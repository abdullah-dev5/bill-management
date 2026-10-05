package extract

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrTransient     = errors.New("temporary failure")
	ErrRateLimited   = errors.New("rate limited")
	ErrInvalidOutput = errors.New("invalid output")
	// ErrNothingExtracted means no tier could read the document; the user enters it manually.
	ErrNothingExtracted = errors.New("nothing could be read from the document")
)

// Input is one document to read.
type Input struct {
	Data        []byte
	ContentType string
	FileName    string
	Providers   []Provider
	AIConsent   bool
}

// TierRun records what each tier did, for the user and for debugging.
type TierRun struct {
	Tier    string `json:"tier"`
	Outcome string `json:"outcome"` // used, skipped, failed, not_needed
	Message string `json:"message,omitempty"`
	Millis  int64  `json:"ms"`
}

// Config sets up the chain. Empty keys disable a tier.
type Config struct {
	Gemini      *Gemini
	OCRSpace    *OCRSpace
	DailyLimits map[string]int // per tier name; 0 = unlimited
	Retry       time.Duration  // backoff before the one retry of a temporary failure
	Timeout     map[string]time.Duration
	Now         func() time.Time
}

// Chain runs the extraction tiers in order (docs/05 §4): PDF text → Gemini (with consent) →
// OCR.space; later tiers only run when the result is still incomplete.
type Chain struct {
	cfg Config

	mu       sync.Mutex
	failures map[string]int       // consecutive temporary failures
	pausedTo map[string]time.Time // circuit open / rate limited until
	usage    map[string]int       // calls today
	usageDay string
}

const (
	breakerThreshold = 3
	breakerPause     = 15 * time.Minute
	rateLimitPause   = 15 * time.Minute
	minPDFTextChars  = 200
)

func NewChain(cfg Config) *Chain {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Retry == 0 {
		cfg.Retry = time.Second
	}
	return &Chain{cfg: cfg, failures: map[string]int{}, pausedTo: map[string]time.Time{}, usage: map[string]int{}}
}

// Available lists the tiers that are configured (for health and error messages).
func (c *Chain) Available() []string {
	out := []string{"pdf-text"}
	if c.cfg.Gemini != nil {
		out = append(out, "gemini")
	}
	if c.cfg.OCRSpace != nil {
		out = append(out, "ocrspace")
	}
	return out
}

// Run extracts fields from in. The result is validated; RawText holds the best text found.
func (c *Chain) Run(ctx context.Context, in Input) (Result, []TierRun, error) {
	result := NewResult()
	var trace []TierRun
	record := func(tier, outcome, msg string, start time.Time) {
		trace = append(trace, TierRun{Tier: tier, Outcome: outcome, Message: msg, Millis: time.Since(start).Milliseconds()})
	}
	gotAnything := false

	// Tier 0: the PDF's own text layer — free, private and exact.
	pdfHadText := false
	if in.ContentType == "application/pdf" {
		start := time.Now()
		text, _, err := PDFText(in.Data)
		switch {
		case err != nil:
			record("pdf-text", "failed", err.Error(), start)
		case len(strings.TrimSpace(text)) < minPDFTextChars:
			record("pdf-text", "skipped", "This PDF is a scan without a text layer.", start)
		default:
			pdfHadText = true
			gotAnything = true
			result.Merge(ParseText(text, in.Providers, "pdf-text"))
			record("pdf-text", "used", "", start)
		}
	}

	// Tier 1: Gemini, only with the household's AI consent.
	switch {
	case result.Complete():
		trace = append(trace, TierRun{Tier: "gemini", Outcome: "not_needed"})
	case c.cfg.Gemini == nil:
		trace = append(trace, TierRun{Tier: "gemini", Outcome: "skipped", Message: "No Gemini API key is configured."})
	case !in.AIConsent:
		trace = append(trace, TierRun{Tier: "gemini", Outcome: "skipped", Message: "AI-assisted extraction is off for this household."})
	default:
		start := time.Now()
		r, err := c.attempt(ctx, "gemini", func(ctx context.Context) (Result, error) { return c.cfg.Gemini.Extract(ctx, in) })
		if err != nil {
			record("gemini", outcomeOf(err), err.Error(), start)
		} else {
			gotAnything = true
			CrossCheck(&r, result.RawText)
			result.Merge(r)
			record("gemini", "used", "", start)
		}
	}

	// Tier 2: OCR.space text + our parsers.
	switch {
	case result.Complete():
		trace = append(trace, TierRun{Tier: "ocrspace", Outcome: "not_needed"})
	case c.cfg.OCRSpace == nil:
		trace = append(trace, TierRun{Tier: "ocrspace", Outcome: "skipped", Message: "No OCR.space API key is configured."})
	case pdfHadText:
		trace = append(trace, TierRun{Tier: "ocrspace", Outcome: "skipped", Message: "The PDF's own text was already read."})
	case len(in.Data) > OCRSpaceMaxBytes:
		trace = append(trace, TierRun{Tier: "ocrspace", Outcome: "skipped", Message: "The file is larger than OCR.space's 1 MB limit."})
	default:
		start := time.Now()
		var text string
		_, err := c.attempt(ctx, "ocrspace", func(ctx context.Context) (Result, error) {
			t, err := c.cfg.OCRSpace.Text(ctx, in)
			text = t
			return Result{}, err
		})
		switch {
		case err != nil:
			record("ocrspace", outcomeOf(err), err.Error(), start)
		case strings.TrimSpace(text) == "":
			record("ocrspace", "failed", "No text was found in the image.", start)
		default:
			gotAnything = true
			parsed := ParseText(text, in.Providers, "ocrspace")
			if result.RawText == "" {
				result.RawText = text
			}
			result.Merge(parsed)
			record("ocrspace", "used", "", start)
		}
	}

	if !gotAnything {
		return result, trace, ErrNothingExtracted
	}
	Validate(&result, Context{ReferencePattern: referencePattern(result.Text(FProvider), in.Providers)})
	return result, trace, nil
}

// attempt runs one tier with the daily limit, circuit breaker and one retry of temporary failures.
func (c *Chain) attempt(ctx context.Context, tier string, call func(context.Context) (Result, error)) (Result, error) {
	if err := c.reserve(tier); err != nil {
		return Result{}, err
	}
	var r Result
	var err error
	for try := 0; try < 2; try++ {
		if try > 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(c.cfg.Retry):
			}
		}
		timeout := c.cfg.Timeout[tier]
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		tctx, cancel := context.WithTimeout(ctx, timeout)
		r, err = call(tctx)
		cancel()
		if err == nil || !errors.Is(err, ErrTransient) && !errors.Is(tctx.Err(), context.DeadlineExceeded) {
			break
		}
		if errors.Is(tctx.Err(), context.DeadlineExceeded) && !errors.Is(err, ErrTransient) {
			err = fmt.Errorf("%w: timed out after %s", ErrTransient, timeout)
		}
	}
	c.report(tier, err)
	return r, err
}

func (c *Chain) reserve(tier string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.cfg.Now()
	if until := c.pausedTo[tier]; now.Before(until) {
		return fmt.Errorf("%w: paused until %s after repeated failures or limits", ErrRateLimited, until.Format("15:04"))
	}
	day := now.UTC().Format("2006-01-02")
	if c.usageDay != day {
		c.usageDay, c.usage = day, map[string]int{}
	}
	if limit := c.cfg.DailyLimits[tier]; limit > 0 && c.usage[tier] >= limit {
		return fmt.Errorf("%w: today's limit of %d calls is used up", ErrRateLimited, limit)
	}
	c.usage[tier]++
	return nil
}

func (c *Chain) report(tier string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.cfg.Now()
	switch {
	case err == nil:
		c.failures[tier] = 0
	case errors.Is(err, ErrRateLimited):
		c.pausedTo[tier] = now.Add(rateLimitPause)
	case errors.Is(err, ErrTransient):
		c.failures[tier]++
		if c.failures[tier] >= breakerThreshold {
			c.pausedTo[tier] = now.Add(breakerPause)
			c.failures[tier] = 0
		}
	}
}

func outcomeOf(err error) string {
	if errors.Is(err, ErrRateLimited) {
		return "skipped"
	}
	return "failed"
}

func referencePattern(shortName string, providers []Provider) string {
	for _, p := range providers {
		if strings.EqualFold(p.ShortName, shortName) {
			return p.RefPattern
		}
	}
	return ""
}

// classifyStatus maps HTTP failures to the chain's error kinds.
func classifyStatus(status int, msg string) error {
	if len(msg) > 300 {
		msg = msg[:300]
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, msg)
	case status >= 500 || status == http.StatusRequestTimeout:
		return fmt.Errorf("%w: HTTP %d: %s", ErrTransient, status, msg)
	default:
		return fmt.Errorf("HTTP %d: %s", status, msg)
	}
}

func httpClient(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return http.DefaultClient
}

// FromDeviceText is tier 3 (docs/05): text recognised on the user's phone (Tesseract.js),
// parsed and validated exactly like OCR text. Low recognition confidence lowers every field.
func FromDeviceText(text string, providers []Provider, ocrConfidence float64) (Result, error) {
	if strings.TrimSpace(text) == "" {
		return NewResult(), ErrNothingExtracted
	}
	r := ParseText(text, providers, "device")
	if len(r.Fields) == 0 {
		return r, ErrNothingExtracted
	}
	if ocrConfidence > 0 && ocrConfidence < 70 {
		factor := 0.6 + 0.4*ocrConfidence/70
		for name, f := range r.Fields {
			f.Confidence = math.Round(f.Confidence*factor*100) / 100
			r.Fields[name] = f
		}
	}
	Validate(&r, Context{ReferencePattern: referencePattern(r.Text(FProvider), providers)})
	return r, nil
}
