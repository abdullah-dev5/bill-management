package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/config"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const (
	extractionLock    = 3 * time.Minute
	extractionTimeout = 2 * time.Minute
	workerPoll        = 5 * time.Second
)

// NewExtractionChain builds the extractor chain from configuration (docs/05 §4).
func NewExtractionChain(cfg config.Config) *extract.Chain {
	c := extract.Config{
		DailyLimits: map[string]int{"gemini": cfg.GeminiDailyLimit, "ocrspace": cfg.OCRSpaceDailyLimit},
		Timeout:     map[string]time.Duration{"gemini": 45 * time.Second, "ocrspace": 30 * time.Second},
	}
	if cfg.GeminiAPIKey != "" {
		models := strings.Split(cfg.GeminiModel, ",")
		for i := range models {
			models[i] = strings.TrimSpace(models[i])
		}
		c.Gemini = &extract.Gemini{APIKey: cfg.GeminiAPIKey, Model: models[0], Fallbacks: models[1:]}
	}
	if cfg.OCRSpaceAPIKey != "" {
		c.OCRSpace = &extract.OCRSpace{APIKey: cfg.OCRSpaceAPIKey}
	}
	return extract.NewChain(c)
}

// storedResult is what an extraction keeps in extractions.result.
type storedResult struct {
	extract.Result
	ProviderID *uuid.UUID `json:"providerId,omitempty"`
}

// extractionView is the API shape of an extraction (schema Extraction).
type extractionView struct {
	ID         uuid.UUID                `json:"id"`
	DocumentID uuid.UUID                `json:"documentId"`
	Status     string                   `json:"status"`
	Tiers      []extract.TierRun        `json:"tiers"`
	Fields     map[string]extract.Field `json:"fields"`
	LineItems  []extract.LineItem       `json:"lineItems"`
	LateFees   []extract.LateFee        `json:"lateFees"`
	History    []extract.HistoryRow     `json:"history"`
	Warnings   []extract.Warning        `json:"warnings"`
	ProviderID *uuid.UUID               `json:"providerId"`
	Error      *string                  `json:"error"`
	CreatedAt  time.Time                `json:"createdAt"`
	FinishedAt *time.Time               `json:"finishedAt"`
}

func toExtractionView(e store.Extraction) extractionView {
	v := extractionView{
		ID: e.ID, DocumentID: e.DocumentID, Status: string(e.Status), Error: e.Error,
		CreatedAt: e.CreatedAt, FinishedAt: e.FinishedAt, Tiers: []extract.TierRun{},
	}
	_ = json.Unmarshal(e.Trace, &v.Tiers)
	res := storedResult{Result: extract.NewResult()}
	if len(e.Result) > 0 {
		_ = json.Unmarshal(e.Result, &res)
	}
	v.Fields, v.LineItems, v.LateFees, v.History, v.Warnings = res.Fields, res.LineItems, res.LateFees, res.History, res.Warnings
	v.ProviderID = res.ProviderID
	if v.Tiers == nil {
		v.Tiers = []extract.TierRun{}
	}
	return v
}

func (s *Server) GetExtraction(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	e, err := s.db.GetLatestExtraction(r.Context(), store.GetLatestExtractionParams{DocumentID: id, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("extraction"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toExtractionView(e))
}

func (s *Server) StartExtraction(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	var out store.Extraction
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		doc, err := q.GetDocument(ctx, store.GetDocumentParams{ID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("document")
		} else if err != nil {
			return err
		}
		if doc.Status != store.DocumentStatusReady {
			return errConflict("document_not_ready", "The document hasn't finished uploading.")
		}
		latest, err := q.GetLatestExtraction(ctx, store.GetLatestExtractionParams{DocumentID: id, HouseholdID: a.HouseholdID})
		if err == nil && (latest.Status == store.ExtractionStatusQueued || latest.Status == store.ExtractionStatusRunning) {
			out = latest
			return nil
		} else if err != nil && !store.IsNotFound(err) {
			return err
		}
		n, err := q.CountExtractionsSince(ctx, store.CountExtractionsSinceParams{HouseholdID: a.HouseholdID, CreatedAt: s.now().Add(-24 * time.Hour)})
		if err != nil {
			return err
		}
		if limit := s.cfg.ExtractionDailyLimit; limit > 0 && n >= int64(limit) {
			return &apiError{status: http.StatusTooManyRequests, code: "extraction_limit",
				message: "You've reached today's limit for reading bills automatically. Enter this one yourself or try again tomorrow."}
		}
		out, err = q.CreateExtraction(ctx, store.CreateExtractionParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: a.HouseholdID, DocumentID: id, CreatedBy: &a.UserID,
		})
		return err
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.wakeWorkers()
	writeJSON(w, http.StatusAccepted, toExtractionView(out))
}

func (s *Server) wakeWorkers() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// StartWorkers runs n background extraction workers until ctx ends.
func (s *Server) StartWorkers(ctx context.Context, n int) {
	for i := 0; i < n; i++ {
		go func() {
			t := time.NewTicker(workerPoll)
			defer t.Stop()
			for {
				ran, err := s.runNextExtraction(ctx)
				if err != nil && ctx.Err() == nil {
					s.log.ErrorContext(ctx, "extraction worker", "err", err)
				}
				if ran {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case <-s.wake:
				case <-t.C:
				}
			}
		}()
	}
}

// runNextExtraction claims and runs one queued extraction. It reports whether a job ran.
func (s *Server) runNextExtraction(ctx context.Context) (bool, error) {
	lock := s.now().Add(extractionLock)
	job, err := s.db.ClaimExtraction(ctx, store.ClaimExtractionParams{LockedUntil: &lock, Now: s.now()})
	if store.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	result, trace, runErr := s.extractDocument(ctx, job)
	traceJSON, _ := json.Marshal(trace)
	finish := store.FinishExtractionParams{ID: job.ID, Trace: traceJSON}
	switch {
	case runErr == nil:
		finish.Status = store.ExtractionStatusSucceeded
		finish.Result, err = json.Marshal(result)
		if err != nil {
			return true, err
		}
	case errors.Is(runErr, extract.ErrNothingExtracted):
		finish.Status = store.ExtractionStatusFailed
		finish.Error = ptr(nothingReadMessage(trace))
	default:
		s.log.ErrorContext(ctx, "extraction failed", "extraction_id", job.ID, "err", runErr)
		finish.Status = store.ExtractionStatusFailed
		finish.Error = ptr("Something went wrong while reading this document. Please try again or enter the details yourself.")
	}
	// Finish even if the request context was cancelled mid-run (shutdown).
	return true, s.db.FinishExtraction(context.WithoutCancel(ctx), finish)
}

func nothingReadMessage(trace []extract.TierRun) string {
	var gemini, ocr string
	for _, t := range trace {
		switch t.Tier {
		case "gemini":
			gemini = t.Message
		case "ocrspace":
			ocr = t.Message
		}
	}
	noOCR := strings.Contains(ocr, "No OCR.space")
	switch {
	case strings.Contains(gemini, "No Gemini") && noOCR:
		return "This document couldn't be read automatically because no reading service (Gemini or OCR.space) is set up yet. Enter the details yourself."
	case strings.Contains(gemini, "is off") && noOCR:
		return "This document couldn't be read automatically: AI-assisted extraction is off and no OCR service is set up. Enter the details yourself."
	}
	return "We couldn't read this document automatically. Enter the details yourself; the document stays attached."
}

func (s *Server) extractDocument(ctx context.Context, job store.Extraction) (storedResult, []extract.TierRun, error) {
	doc, err := s.db.GetDocument(ctx, store.GetDocumentParams{ID: job.DocumentID, HouseholdID: job.HouseholdID})
	if err != nil {
		return storedResult{}, nil, err
	}
	hh, err := s.db.GetHousehold(ctx, job.HouseholdID)
	if err != nil {
		return storedResult{}, nil, err
	}
	hints, err := s.db.ListProviderHints(ctx, &job.HouseholdID)
	if err != nil {
		return storedResult{}, nil, err
	}
	rc, err := s.files.Open(ctx, doc.StorageKey)
	if err != nil {
		return storedResult{}, nil, err
	}
	data, err := io.ReadAll(io.LimitReader(rc, maxDocumentBytes+1))
	rc.Close()
	if err != nil {
		return storedResult{}, nil, err
	}

	providers := make([]extract.Provider, 0, len(hints))
	for _, h := range hints {
		providers = append(providers, extract.Provider{ShortName: h.ShortName, Name: h.Name, RefPattern: deref(h.RefNumberPattern)})
	}
	runCtx, cancel := context.WithTimeout(ctx, extractionTimeout)
	defer cancel()
	res, trace, err := s.extractor.Run(runCtx, extract.Input{
		Data: data, ContentType: doc.ContentType, FileName: doc.FileName, Providers: providers, AIConsent: hh.AiExtractionConsent,
	})
	if err != nil {
		return storedResult{}, trace, err
	}

	// The customer's name is only used to redact the stored text; it is never kept (docs/08 §4).
	name := res.Text(extract.FCustomerName)
	delete(res.Fields, extract.FCustomerName)
	if res.RawText != "" {
		if err := s.db.SetDocumentText(ctx, store.SetDocumentTextParams{ID: doc.ID, OcrText: ptr(extract.Redact(res.RawText, name))}); err != nil {
			return storedResult{}, trace, err
		}
	}
	out := storedResult{Result: res}
	if name := res.Text(extract.FProvider); name != "" {
		if h, ok := matchProvider(hints, name); ok {
			id := h.ID
			out.ProviderID = &id
			if f, ok := out.Fields[extract.FProvider]; ok {
				f.Value = h.ShortName
				out.Fields[extract.FProvider] = f
			}
		}
	}
	return out, trace, nil
}

// matchProvider finds the catalogue provider for an extracted name: its short name ("HESCO"),
// its full name ("Hyderabad Electric Supply Company"), or a name containing the short name.
func matchProvider(hints []store.ListProviderHintsRow, name string) (store.ListProviderHintsRow, bool) {
	clean := func(s string) string { return strings.Join(strings.Fields(strings.ToUpper(s)), " ") }
	n := clean(name)
	for _, h := range hints {
		if n == clean(h.ShortName) || n == clean(h.Name) {
			return h, true
		}
	}
	for _, h := range hints {
		short, full := clean(h.ShortName), clean(h.Name)
		if len(short) >= 3 && strings.Contains(" "+n+" ", " "+short+" ") || len(n) >= 8 && strings.Contains(full, n) {
			return h, true
		}
	}
	return store.ListProviderHintsRow{}, false
}
