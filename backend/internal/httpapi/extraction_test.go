package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/extract"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
)

func hescoPDF() []byte {
	return extracttest.TextPDF(extracttest.LatinLines(extracttest.HescoText), 1)
}

// runExtractions drains the job queue synchronously.
func (e *env) runExtractions() {
	e.t.Helper()
	for {
		ran, err := e.server.runNextExtraction(context.Background())
		if err != nil {
			e.t.Fatal(err)
		}
		if !ran {
			return
		}
	}
}

func (cl *client) extract(docID string) extractionView {
	cl.t.Helper()
	var queued extractionView
	cl.must(http.StatusAccepted, http.MethodPost, "/api/v1/documents/"+docID+"/extraction", nil, &queued)
	if queued.Status != "queued" {
		cl.t.Fatalf("new extraction status = %q", queued.Status)
	}
	cl.env.runExtractions()
	var done extractionView
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/documents/"+docID+"/extraction", nil, &done)
	return done
}

func TestExtractionOfTextPDF(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Extract")
	pdf := hescoPDF()
	doc := cl.upload(pdf, "application/pdf", map[string]any{"fileName": "hesco.pdf", "source": "upload"})
	if doc.PageCount == nil || *doc.PageCount != 1 {
		t.Fatalf("page count = %v", doc.PageCount)
	}

	got := cl.extract(doc.Id.String())
	if got.Status != "succeeded" || got.Error != nil {
		t.Fatalf("status %s, error %v, tiers %+v", got.Status, got.Error, got.Tiers)
	}
	want := map[string]string{
		extract.FProvider: "HESCO", extract.FReference: "09123450012345", extract.FPayable: "326600",
		extract.FDueDate: "2026-10-05", extract.FBillingMonth: "2026-09", extract.FUnits: "164",
	}
	for k, v := range want {
		if got.Fields[k].Value != v {
			t.Errorf("%s = %q, want %q", k, got.Fields[k].Value, v)
		}
	}
	if _, ok := got.Fields[extract.FCustomerName]; ok {
		t.Error("the customer's name must not be stored")
	}
	if got.ProviderID == nil || got.ProviderID.String() != "b0000000-0000-4000-8000-000000000007" && got.ProviderID.String() == "" {
		t.Errorf("provider id = %v", got.ProviderID)
	}
	if len(got.History) != 12 || len(got.LateFees) != 2 || got.Tiers[0].Tier != "pdf-text" || got.Tiers[0].Outcome != "used" {
		t.Errorf("history %d, late fees %d, tiers %+v", len(got.History), len(got.LateFees), got.Tiers)
	}

	var text string
	if err := testDB.Pool.QueryRow(context.Background(), "SELECT coalesce(ocr_text, '') FROM documents WHERE id = $1", doc.Id).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "HESCO") || strings.Contains(text, "Test Customer") {
		t.Errorf("stored text should be kept and redacted: %.200q", text)
	}
}

func TestExtractionProviderIsResolved(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Provider")
	doc := cl.upload(hescoPDF(), "application/pdf", nil)
	got := cl.extract(doc.Id.String())
	var providers []struct {
		ID        string `json:"id"`
		ShortName string `json:"shortName"`
	}
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/providers", nil, &providers)
	for _, p := range providers {
		if p.ShortName == "HESCO" {
			if got.ProviderID == nil || got.ProviderID.String() != p.ID {
				t.Fatalf("provider id = %v, want %s", got.ProviderID, p.ID)
			}
			return
		}
	}
	t.Fatal("HESCO not in the catalogue")
}

func TestExtractionImageWithoutServices(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("NoKeys")
	doc := cl.upload(fakeJPEG, "image/jpeg", nil)

	var first, again extractionView
	cl.must(http.StatusAccepted, http.MethodPost, "/api/v1/documents/"+doc.Id.String()+"/extraction", nil, &first)
	cl.must(http.StatusAccepted, http.MethodPost, "/api/v1/documents/"+doc.Id.String()+"/extraction", nil, &again)
	if first.ID != again.ID {
		t.Fatal("a second request while queued should return the same extraction")
	}
	e.runExtractions()
	var got extractionView
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/documents/"+doc.Id.String()+"/extraction", nil, &got)
	if got.Status != "failed" || got.Error == nil || !strings.Contains(*got.Error, "no reading service") {
		t.Fatalf("status %s, error %v", got.Status, got.Error)
	}
}

func TestExtractionWithGeminiNeedsConsent(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Gemini")
	calls := 0
	gemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		f := func(v string) map[string]any { return map[string]any{"value": v, "confidence": 0.95} }
		inner, _ := json.Marshal(map[string]any{
			"provider_name": f("LESCO"), "reference_number": f("04 11234 5678901"), "due_date": f("05 OCT 26"),
			"payable_within_due_date": f("3,266"), "current_charges": f("3,266"), "arrears": f("0"), "billing_month": f("SEP 26"),
		})
		out, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(inner)}}}}}})
		io.WriteString(w, string(out))
	}))
	defer gemini.Close()
	e.server.extractor = extract.NewChain(extract.Config{Gemini: &extract.Gemini{APIKey: "k", Model: "m", BaseURL: gemini.URL}})

	doc := cl.upload(fakeJPEG, "image/jpeg", nil)
	if got := cl.extract(doc.Id.String()); got.Status != "failed" || calls != 0 {
		t.Fatalf("without consent: status %s, gemini calls %d", got.Status, calls)
	}

	cl.must(http.StatusOK, http.MethodPut, "/api/v1/household", map[string]any{
		"name": "Home", "currency": "PKR", "timezone": "Asia/Karachi", "aiExtractionConsent": true,
	}, nil)
	got := cl.extract(doc.Id.String())
	if got.Status != "succeeded" || calls != 1 {
		t.Fatalf("with consent: status %s, error %v, calls %d", got.Status, got.Error, calls)
	}
	if got.Fields[extract.FReference].Value != "04112345678901" || got.Fields[extract.FPayable].Source != "gemini" {
		t.Errorf("fields = %+v", got.Fields)
	}
}

func TestExtractionLimitsAndIsolation(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Limits")
	other, _ := e.signedUp("Other")
	doc := cl.upload(hescoPDF(), "application/pdf", nil)
	id := doc.Id.String()

	if got := other.do(http.MethodPost, "/api/v1/documents/"+id+"/extraction", nil, nil); got != http.StatusNotFound {
		t.Errorf("other household start: %d", got)
	}
	cl.extract(id)
	if got := other.do(http.MethodGet, "/api/v1/documents/"+id+"/extraction", nil, nil); got != http.StatusNotFound {
		t.Errorf("other household read: %d", got)
	}

	e.server.cfg.ExtractionDailyLimit = 1
	var apiErr struct{ Code string }
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+id+"/extraction", nil, &apiErr); got != http.StatusTooManyRequests || apiErr.Code != "extraction_limit" {
		t.Errorf("over the daily limit: %d %s", got, apiErr.Code)
	}

	pending, _ := cl.startUpload(fakePDF, "application/pdf", nil)
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+pending.Document.Id.String()+"/extraction", nil, nil); got != http.StatusConflict {
		t.Errorf("extraction of an unfinished upload: %d, want 409", got)
	}
}

func TestPDFPageLimit(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Pages")
	pdf := extracttest.TextPDF([]string{"page"}, 6)
	up, _ := cl.startUpload(pdf, "application/pdf", nil)
	cl.putBytes(up.Upload.Url, "application/pdf", pdf)
	var apiErr struct{ Code, Message string }
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+up.Document.Id.String()+"/complete", nil, &apiErr); got != http.StatusBadRequest || !strings.Contains(apiErr.Message, "6 pages") {
		t.Fatalf("6-page PDF: %d %+v", got, apiErr)
	}
}
