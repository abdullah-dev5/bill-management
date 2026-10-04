package extract

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
)

func TestPDFTextAndPages(t *testing.T) {
	pdf := extracttest.TextPDF([]string{"HESCO", "Due Date 05 OCT 26"}, 3)
	text, pages, err := PDFText(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 3 || !strings.Contains(text, "Due Date 05 OCT 26") {
		t.Fatalf("pages %d, text %q", pages, text)
	}
	if n, err := PDFPageCount(pdf); err != nil || n != 3 {
		t.Fatalf("PDFPageCount = %d, %v", n, err)
	}
	if _, _, err := PDFText([]byte("%PDF-1.4 garbage")); err == nil {
		t.Fatal("garbage PDF accepted")
	}
}

func TestChainTextPDFNeedsNoExternalService(t *testing.T) {
	gemini := fakeGemini(t, http.StatusOK, geminiReply(map[string]any{}))
	c := NewChain(Config{Gemini: &Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}})
	r, trace, err := c.Run(context.Background(), Input{
		Data: extracttest.TextPDF(extracttest.LatinLines(extracttest.HescoText), 1), ContentType: "application/pdf", Providers: catalogue, AIConsent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantFields(t, r, map[string]string{FPayable: "326600", FDueDate: "2026-10-05", FReference: "09123450012345", FBillingMonth: "2026-09"})
	if trace[0].Outcome != "used" || trace[1].Outcome != "not_needed" {
		t.Fatalf("trace = %+v", trace)
	}
	if gemini.calls.Load() != 0 {
		t.Fatal("Gemini was called for a complete text PDF")
	}
	if len(r.History) != 12 {
		t.Errorf("history rows = %d", len(r.History))
	}
}

type fakeServer struct {
	*httptest.Server
	calls atomic.Int32
}

func fakeGemini(t *testing.T, status int, body string) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if r.Header.Get("x-goog-api-key") != "k" || !strings.HasSuffix(r.URL.Path, "/models/m:generateContent") {
			http.Error(w, "bad request", 400)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(f.Close)
	return f
}

func geminiReply(fields map[string]any) string {
	text, _ := json.Marshal(fields)
	out, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(text)}}}}}})
	return string(out)
}

var photoInput = Input{Data: []byte("\xff\xd8\xff jpeg"), ContentType: "image/jpeg", FileName: "bill.jpg", Providers: catalogue, AIConsent: true}

func TestChainGeminiReadsPhoto(t *testing.T) {
	f := func(v string, c float64) map[string]any { return map[string]any{"value": v, "confidence": c} }
	gemini := fakeGemini(t, http.StatusOK, geminiReply(map[string]any{
		"provider_name":           f("LESCO", 0.95),
		"reference_number":        f("04 11234 5678901", 0.97),
		"billing_month":           f("SEP 26", 0.9),
		"issue_date":              f("18 SEP 26", 0.9),
		"due_date":                f("05 OCT 26", 0.99),
		"payable_within_due_date": f("3,266", 0.96),
		"payable_after_due_date":  f("3,389", 0.9),
		"current_charges":         f("3,266", 0.9),
		"arrears":                 f("0", 0.9),
		"customer_name":           f("Test Customer", 0.8),
		"units":                   nil,
		"history":                 []any{map[string]any{"month": "Aug26", "units": "161", "bill": "2,727", "payment": "2,727"}},
		"late_fees":               []any{map[string]any{"until": "08-OCT-26", "amount": "3,389"}, map[string]any{"amount": "3,512"}},
	}))
	ocr := fakeOCR(t, http.StatusOK, `{"ParsedResults":[{"ParsedText":"x"}]}`)
	c := NewChain(Config{Gemini: &Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}, OCRSpace: &OCRSpace{APIKey: "o", Endpoint: ocr.URL}})
	r, trace, err := c.Run(context.Background(), photoInput)
	if err != nil {
		t.Fatal(err)
	}
	wantFields(t, r, map[string]string{
		FProvider: "LESCO", FReference: "04112345678901", FBillingMonth: "2026-09", FDueDate: "2026-10-05",
		FPayable: "326600", FPayableAfterDue: "338900", FCustomerName: "Test Customer",
	})
	if c := r.Fields[FDueDate].Confidence; c != 0.92 {
		t.Errorf("model confidence should be capped at 0.92, got %.2f", c)
	}
	if len(r.History) != 1 || *r.History[0].BilledMinor != 272700 || len(r.LateFees) != 2 {
		t.Errorf("history %+v, late fees %+v", r.History, r.LateFees)
	}
	if trace[0].Tier != "gemini" || trace[0].Outcome != "used" || trace[1].Outcome != "not_needed" || ocr.calls.Load() != 0 {
		t.Errorf("trace = %+v, ocr calls %d", trace, ocr.calls.Load())
	}
}

func fakeOCR(t *testing.T, status int, body string) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if r.Header.Get("apikey") != "o" {
			http.Error(w, "bad key", 400)
			return
		}
		if err := r.ParseMultipartForm(2 << 20); err != nil || r.FormValue("OCREngine") != "2" {
			http.Error(w, "bad form", 400)
			return
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(f.Close)
	return f
}

const ocrText = `SUI NORTHERN GAS PIPELINES LIMITED\r\nConsumer No: 12345678901\r\nBill Month: SEP-2026\r\nIssue Date: 20-09-2026\r\nDue Date: 10-10-2026\r\nCurrent Charges 1,850\r\nArrears 0\r\nAmount Payable 1,850\r\nPayable after due date 1,950`

func TestChainWithoutConsentUsesOCR(t *testing.T) {
	gemini := fakeGemini(t, http.StatusOK, geminiReply(map[string]any{}))
	ocr := fakeOCR(t, http.StatusOK, `{"ParsedResults":[{"ParsedText":"`+ocrText+`"}],"IsErroredOnProcessing":false}`)
	c := NewChain(Config{Gemini: &Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}, OCRSpace: &OCRSpace{APIKey: "o", Endpoint: ocr.URL}})
	in := photoInput
	in.AIConsent = false
	r, trace, err := c.Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if gemini.calls.Load() != 0 {
		t.Fatal("Gemini called without consent")
	}
	wantFields(t, r, map[string]string{FReference: "12345678901", FBillingMonth: "2026-09", FDueDate: "2026-10-10", FPayable: "185000", FPayableAfterDue: "195000"})
	if trace[0].Outcome != "skipped" || !strings.Contains(trace[0].Message, "off") || trace[1].Outcome != "used" {
		t.Errorf("trace = %+v", trace)
	}
	if !strings.Contains(r.RawText, "SUI NORTHERN") {
		t.Error("OCR text not kept as raw text")
	}
}

func TestChainFallsBackAndPausesOnRateLimit(t *testing.T) {
	gemini := fakeGemini(t, http.StatusTooManyRequests, `{"error":{"message":"Resource has been exhausted"}}`)
	ocr := fakeOCR(t, http.StatusOK, `{"ParsedResults":[{"ParsedText":"`+ocrText+`"}]}`)
	now := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	c := NewChain(Config{
		Gemini: &Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}, OCRSpace: &OCRSpace{APIKey: "o", Endpoint: ocr.URL},
		Retry: time.Millisecond, Now: func() time.Time { return now },
	})
	_, trace, err := c.Run(context.Background(), photoInput)
	if err != nil || trace[0].Outcome != "skipped" || trace[1].Outcome != "used" {
		t.Fatalf("first run: %v %+v", err, trace)
	}
	if gemini.calls.Load() != 1 {
		t.Errorf("429 must not be retried; calls = %d", gemini.calls.Load())
	}
	_, trace, _ = c.Run(context.Background(), photoInput)
	if gemini.calls.Load() != 1 || !strings.Contains(trace[0].Message, "paused") {
		t.Errorf("Gemini should be paused after a rate limit: calls %d, %+v", gemini.calls.Load(), trace[0])
	}
	now = now.Add(16 * time.Minute)
	c.Run(context.Background(), photoInput)
	if gemini.calls.Load() != 2 {
		t.Errorf("Gemini should be tried again after the pause; calls = %d", gemini.calls.Load())
	}
}

func TestChainCircuitBreaker(t *testing.T) {
	gemini := fakeGemini(t, http.StatusServiceUnavailable, `{"error":{"message":"overloaded"}}`)
	c := NewChain(Config{Gemini: &Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}, Retry: time.Millisecond})
	for i := 0; i < 3; i++ {
		if _, _, err := c.Run(context.Background(), photoInput); !errors.Is(err, ErrNothingExtracted) {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if got := gemini.calls.Load(); got != 6 {
		t.Fatalf("each failing run retries once: calls = %d, want 6", got)
	}
	_, trace, _ := c.Run(context.Background(), photoInput)
	if gemini.calls.Load() != 6 || !strings.Contains(trace[0].Message, "paused") {
		t.Errorf("breaker should be open: calls %d, %+v", gemini.calls.Load(), trace[0])
	}
}

func TestChainDailyLimitAndNothingConfigured(t *testing.T) {
	gemini := fakeGemini(t, http.StatusOK, geminiReply(map[string]any{"due_date": map[string]any{"value": "05 OCT 26", "confidence": 0.9}}))
	c := NewChain(Config{Gemini: &Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}, DailyLimits: map[string]int{"gemini": 1}})
	c.Run(context.Background(), photoInput)
	_, trace, err := c.Run(context.Background(), photoInput)
	if !errors.Is(err, ErrNothingExtracted) || !strings.Contains(trace[0].Message, "limit") || gemini.calls.Load() != 1 {
		t.Errorf("daily limit: %v %+v calls %d", err, trace, gemini.calls.Load())
	}

	bare := NewChain(Config{})
	_, trace, err = bare.Run(context.Background(), photoInput)
	if !errors.Is(err, ErrNothingExtracted) || !strings.Contains(trace[0].Message, "No Gemini API key") {
		t.Errorf("unconfigured: %v %+v", err, trace)
	}
}

func TestGeminiInvalidJSONIsRetriedOnce(t *testing.T) {
	bad, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": "not json"}}}}}})
	gemini := fakeGemini(t, http.StatusOK, string(bad))
	g := &Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}
	if _, err := g.Extract(context.Background(), photoInput); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("err = %v", err)
	}
	if gemini.calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 (one repair attempt)", gemini.calls.Load())
	}
}

func TestGeminiFallsBackToTheNextModel(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		switch {
		case strings.Contains(r.URL.Path, "/models/busy:"):
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"error":{"message":"high demand"}}`)
		case strings.Contains(r.URL.Path, "/models/cut:"):
			io.WriteString(w, `{"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"text":"{}"}]}}]}`)
		default:
			io.WriteString(w, geminiReply(map[string]any{"due_date": map[string]any{"value": "05 OCT 26", "confidence": 0.9}}))
		}
	}))
	defer srv.Close()

	g := &Gemini{APIKey: "k", Model: "busy", Fallbacks: []string{"ok"}, BaseURL: srv.URL}
	r, err := g.Extract(context.Background(), photoInput)
	if err != nil || r.Text(FDueDate) != "2026-10-05" || len(seen) != 2 {
		t.Fatalf("fallback: %v, %+v, calls %v", err, r.Fields, seen)
	}

	g = &Gemini{APIKey: "k", Model: "cut", Fallbacks: []string{"ok"}, BaseURL: srv.URL}
	if _, err := g.Extract(context.Background(), photoInput); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("a cut-off response must be rejected, not passed to the next model: %v", err)
	}
}
