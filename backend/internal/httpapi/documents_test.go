package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
	"github.com/abdullah-dev5/bill-management/backend/internal/storage"
)

var (
	fakeJPEG = append([]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"), bytes.Repeat([]byte{7}, 2000)...)
	fakePDF  = extracttest.TextPDF([]string{"Receipt", "Paid Rs 2,500"}, 1)
)

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// startUpload calls POST /documents and returns the decoded response and status.
func (cl *client) startUpload(data []byte, contentType string, extra map[string]any) (apigen.DocumentUpload, int) {
	cl.t.Helper()
	body := map[string]any{
		"fileName": "bill.jpg", "contentType": contentType, "sizeBytes": len(data),
		"sha256": sha(data), "kind": "bill", "source": "camera",
	}
	for k, v := range extra {
		body[k] = v
	}
	var out apigen.DocumentUpload
	status := cl.do(http.MethodPost, "/api/v1/documents", body, &out)
	return out, status
}

// putBytes sends data to a signed upload link.
func (cl *client) putBytes(link, contentType string, data []byte) int {
	cl.t.Helper()
	req, _ := http.NewRequest(http.MethodPut, cl.env.http.URL+link, bytes.NewReader(data))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cl.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// upload runs the whole flow: start, PUT, complete.
func (cl *client) upload(data []byte, contentType string, extra map[string]any) apigen.Document {
	cl.t.Helper()
	up, status := cl.startUpload(data, contentType, extra)
	if status != http.StatusCreated || up.Upload == nil {
		cl.t.Fatalf("start upload: status %d, upload %v", status, up.Upload)
	}
	if got := cl.putBytes(up.Upload.Url, contentType, data); got != http.StatusNoContent {
		cl.t.Fatalf("PUT file: status %d", got)
	}
	var doc apigen.Document
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/documents/"+up.Document.Id.String()+"/complete", nil, &doc)
	return doc
}

func fetch(t *testing.T, e *env, link string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(e.http.URL + link)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func expenseBill(cl *client) apigen.BillDetail {
	cl.t.Helper()
	return cl.createBill(map[string]any{
		"profileId": cl.defaultProfile().Id, "description": "Plumber", "expenseDate": "2026-10-01", "currentChargesMinor": 150000,
	})
}

func TestDocumentUploadAttachView(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Docs")
	bill := expenseBill(cl)

	doc := cl.upload(fakeJPEG, "image/jpeg", map[string]any{"billId": bill.Id, "fileName": `C:\Users\me\IMG 2041.jpg`})
	if doc.Status != apigen.Ready || doc.FileName != "IMG 2041.jpg" || doc.ViewUrl == nil || doc.DownloadUrl == nil {
		t.Fatalf("unexpected document: %+v", doc)
	}

	detail := cl.getBill(bill.Id.String())
	if len(detail.Documents) != 1 || detail.Documents[0].Id != doc.Id {
		t.Fatalf("bill documents = %+v", detail.Documents)
	}
	if latest := detail.History[0]; latest.Action != "attach_document" {
		t.Fatalf("latest history action = %q, want attach_document", latest.Action)
	}

	resp, body := fetch(t, e, *doc.ViewUrl)
	if resp.StatusCode != http.StatusOK || !bytes.Equal(body, fakeJPEG) || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("view: status %d, type %q, %d bytes", resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("images must be served with a sandbox CSP")
	}
	resp, _ = fetch(t, e, *doc.DownloadUrl)
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "IMG 2041.jpg") {
		t.Fatalf("download disposition = %q", cd)
	}

	var hh apigen.Household
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/household", nil, &hh)
	if hh.Storage.UsedBytes != int64(len(fakeJPEG)) || hh.Storage.DocumentCount != 1 || hh.Storage.QuotaBytes != 50<<20 {
		t.Fatalf("storage usage = %+v", hh.Storage)
	}
}

func TestDocumentSignedLinks(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Links")
	up, _ := cl.startUpload(fakePDF, "application/pdf", nil)

	if got := cl.putBytes(up.Upload.Url+"x", "application/pdf", fakePDF); got != http.StatusForbidden {
		t.Fatalf("tampered upload link: %d, want 403", got)
	}
	if got := cl.putBytes(up.Upload.Url, "image/jpeg", fakePDF); got != http.StatusBadRequest {
		t.Fatalf("wrong content type: %d, want 400", got)
	}
	if got := cl.putBytes(up.Upload.Url, "application/pdf", append(fakePDF, 'x')); got != http.StatusBadRequest {
		t.Fatalf("oversized body: %d, want 400", got)
	}
	cl.putBytes(up.Upload.Url, "application/pdf", fakePDF)
	var doc apigen.Document
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/documents/"+up.Document.Id.String()+"/complete", nil, &doc)
	if got := cl.putBytes(up.Upload.Url, "application/pdf", fakePDF); got != http.StatusConflict {
		t.Fatalf("second upload to a finished document: %d, want 409", got)
	}

	resp, _ := fetch(t, e, *doc.ViewUrl)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Security-Policy") != "" {
		t.Fatalf("PDF view: status %d, csp %q", resp.StatusCode, resp.Header.Get("Content-Security-Policy"))
	}
	e.clock.Add(int64(6 * time.Minute))
	if resp, _ := fetch(t, e, *doc.ViewUrl); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expired link: %d, want 403", resp.StatusCode)
	}
	if resp, _ := fetch(t, e, strings.Replace(*doc.ViewUrl, "d=inline", "d=attachment", 1)); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("link reused for another disposition: %d, want 403", resp.StatusCode)
	}
}

func TestDocumentUploadRejected(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Reject")

	// Completing before uploading.
	up, _ := cl.startUpload(fakeJPEG, "image/jpeg", nil)
	var apiErr apigen.Error
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+up.Document.Id.String()+"/complete", nil, &apiErr); got != http.StatusBadRequest || apiErr.Code != "upload_rejected" {
		t.Fatalf("complete without upload: %d %s", got, apiErr.Code)
	}
	if got := cl.do(http.MethodGet, "/api/v1/documents/"+up.Document.Id.String(), nil, nil); got != http.StatusNotFound {
		t.Fatalf("rejected document still exists: %d", got)
	}

	// HTML disguised as a PDF.
	html := []byte("<html><script>alert(1)</script></html>")
	up, _ = cl.startUpload(html, "application/pdf", nil)
	cl.putBytes(up.Upload.Url, "application/pdf", html)
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+up.Document.Id.String()+"/complete", nil, &apiErr); got != http.StatusBadRequest {
		t.Fatalf("disguised HTML accepted: %d", got)
	}

	// Declared hash doesn't match the bytes.
	up, _ = cl.startUpload(fakeJPEG, "image/jpeg", nil)
	other := bytes.Clone(fakeJPEG)
	other[len(other)-1] = 9
	cl.putBytes(up.Upload.Url, "image/jpeg", other)
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+up.Document.Id.String()+"/complete", nil, &apiErr); got != http.StatusBadRequest {
		t.Fatalf("hash mismatch accepted: %d", got)
	}

	if got := cl.do(http.MethodPost, "/api/v1/documents", map[string]any{
		"fileName": "big.pdf", "contentType": "application/pdf", "sizeBytes": 11 << 20,
		"sha256": sha(fakePDF), "kind": "bill", "source": "upload",
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("11 MB file accepted: %d", got)
	}
	if got := cl.do(http.MethodPost, "/api/v1/documents", map[string]any{
		"fileName": "x.gif", "contentType": "image/gif", "sizeBytes": 10,
		"sha256": sha(fakePDF), "kind": "bill", "source": "upload",
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("GIF accepted: %d", got)
	}
}

func TestDocumentDeduplication(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Dedup")
	first := cl.upload(fakeJPEG, "image/jpeg", nil)

	up, status := cl.startUpload(fakeJPEG, "image/jpeg", nil)
	if status != http.StatusOK || !up.Duplicate || up.Upload != nil || up.Document.Id != first.Id {
		t.Fatalf("re-upload: status %d, %+v", status, up)
	}

	// The same file sent for a bill attaches the existing unattached copy.
	bill := expenseBill(cl)
	up, status = cl.startUpload(fakeJPEG, "image/jpeg", map[string]any{"billId": bill.Id})
	if status != http.StatusOK || !up.Duplicate || up.Document.BillId == nil || *up.Document.BillId != bill.Id {
		t.Fatalf("re-upload for a bill: status %d, %+v", status, up.Document)
	}
	if docs := cl.getBill(bill.Id.String()).Documents; len(docs) != 1 {
		t.Fatalf("bill has %d documents, want 1", len(docs))
	}

	// An abandoned upload of the same file can be restarted.
	up, _ = cl.startUpload(fakePDF, "application/pdf", nil)
	again, status := cl.startUpload(fakePDF, "application/pdf", nil)
	if status != http.StatusCreated || again.Document.Id != up.Document.Id || again.Upload == nil {
		t.Fatalf("restart pending upload: status %d, %+v", status, again)
	}
}

func TestDocumentHouseholdIsolation(t *testing.T) {
	e := newEnv(t)
	owner, _ := e.signedUp("Owner")
	intruder, _ := e.signedUp("Intruder")
	doc := owner.upload(fakeJPEG, "image/jpeg", nil)
	ownerBill := expenseBill(owner)
	id := doc.Id.String()

	for _, tc := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/v1/documents/" + id, nil},
		{http.MethodPut, "/api/v1/documents/" + id, map[string]any{"kind": "receipt"}},
		{http.MethodDelete, "/api/v1/documents/" + id, nil},
		{http.MethodPost, "/api/v1/documents/" + id + "/complete", nil},
	} {
		if got := intruder.do(tc.method, tc.path, tc.body, nil); got != http.StatusNotFound {
			t.Errorf("%s %s by another household: %d, want 404", tc.method, tc.path, got)
		}
	}
	if _, got := intruder.startUpload(fakePDF, "application/pdf", map[string]any{"billId": ownerBill.Id}); got != http.StatusNotFound {
		t.Errorf("upload to another household's bill: %d, want 404", got)
	}
	// The same file in another household is a separate document.
	if up, got := intruder.startUpload(fakeJPEG, "image/jpeg", nil); got != http.StatusCreated || up.Duplicate {
		t.Errorf("same file in another household: %d duplicate=%v", got, up.Duplicate)
	}
	var list []apigen.Document
	intruder.must(http.StatusOK, http.MethodGet, "/api/v1/documents?unattached=true", nil, &list)
	if len(list) != 0 {
		t.Errorf("intruder sees %d documents", len(list))
	}
}

func TestDocumentAttachDetachDelete(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Attach")
	bill := expenseBill(cl)
	doc := cl.upload(fakePDF, "application/pdf", nil)

	var list []apigen.Document
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/documents?unattached=true", nil, &list)
	if len(list) != 1 {
		t.Fatalf("unattached = %d, want 1", len(list))
	}
	var updated apigen.Document
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/documents/"+doc.Id.String(), map[string]any{"billId": bill.Id, "kind": "receipt"}, &updated)
	if updated.BillId == nil || *updated.BillId != bill.Id || updated.Kind != apigen.DocumentKindReceipt {
		t.Fatalf("attach: %+v", updated)
	}
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/documents?billId="+bill.Id.String(), nil, &list)
	if len(list) != 1 {
		t.Fatalf("bill documents = %d, want 1", len(list))
	}

	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/documents/"+doc.Id.String(), nil, nil)
	history := cl.getBill(bill.Id.String()).History
	if latest := history[0]; latest.Action != "remove_document" {
		t.Fatalf("latest action = %q, want remove_document", latest.Action)
	}
}

func TestDocumentStorageCleanup(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Cleanup")
	ctx := context.Background()
	storageKey := func(id string) string {
		var key string
		if err := testDB.Pool.QueryRow(ctx, "SELECT storage_key FROM documents WHERE id = $1", id).Scan(&key); err != nil {
			t.Fatal(err)
		}
		return key
	}
	gone := func(key string) bool {
		rc, err := e.files.Open(ctx, key)
		if err == nil {
			rc.Close()
		}
		return errors.Is(err, storage.ErrNotFound)
	}
	tick := func() {
		req, _ := http.NewRequest(http.MethodPost, e.http.URL+"/internal/tick", nil)
		req.Header.Set("Authorization", "Bearer "+testTickToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("tick: %v %v", err, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Deleting a document removes the file at the next tick.
	doc := cl.upload(fakeJPEG, "image/jpeg", nil)
	key := storageKey(doc.Id.String())
	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/documents/"+doc.Id.String(), nil, nil)
	if gone(key) {
		t.Fatal("file removed before the tick")
	}
	tick()
	if !gone(key) {
		t.Fatal("file still stored after the tick")
	}

	// Permanently deleting a bill cascades to its documents and their files.
	bill := expenseBill(cl)
	doc = cl.upload(fakePDF, "application/pdf", map[string]any{"billId": bill.Id})
	key = storageKey(doc.Id.String())
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/archive", nil, nil)
	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/bills/"+bill.Id.String(), nil, nil)
	tick()
	if !gone(key) {
		t.Fatal("file of a deleted bill still stored")
	}

	// Abandoned uploads are cleared after a day.
	other := append(bytes.Clone(fakeJPEG), 1)
	up, _ := cl.startUpload(other, "image/jpeg", nil)
	cl.putBytes(up.Upload.Url, "image/jpeg", other)
	key = storageKey(up.Document.Id.String())
	if _, err := testDB.Pool.Exec(ctx, "UPDATE documents SET created_at = now() - interval '25 hours' WHERE id = $1", up.Document.Id); err != nil {
		t.Fatal(err)
	}
	tick()
	if !gone(key) {
		t.Fatal("abandoned upload still stored")
	}
}

func TestDocumentQuotaAndViewer(t *testing.T) {
	e := newEnv(t)
	cl, me := e.signedUp("Quota")
	e.server.cfg.StorageQuotaBytes = int64(len(fakeJPEG)) + 10
	cl.upload(fakeJPEG, "image/jpeg", nil)
	var apiErr apigen.Error
	_, got := cl.startUpload(fakePDF, "application/pdf", nil)
	if got != http.StatusConflict {
		t.Fatalf("over quota: %d, want 409", got)
	}
	cl.do(http.MethodPost, "/api/v1/documents", map[string]any{
		"fileName": "a.pdf", "contentType": "application/pdf", "sizeBytes": len(fakePDF),
		"sha256": sha(fakePDF), "kind": "bill", "source": "upload",
	}, &apiErr)
	if apiErr.Code != "storage_quota_exceeded" {
		t.Fatalf("code = %q", apiErr.Code)
	}

	if _, err := testDB.Pool.Exec(context.Background(), "UPDATE memberships SET role = 'viewer' WHERE user_id = $1", me.User.Id); err != nil {
		t.Fatal(err)
	}
	e.server.cfg.StorageQuotaBytes = 50 << 20
	if _, got := cl.startUpload(fakePDF, "application/pdf", nil); got != http.StatusForbidden {
		t.Fatalf("viewer upload: %d, want 403", got)
	}
}

func TestAIConsentSetting(t *testing.T) {
	e := newEnv(t)
	cl, me := e.signedUp("Consent")
	if me.Household.AiExtractionConsent {
		t.Fatal("AI consent must default to off")
	}
	var hh apigen.Household
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/household", map[string]any{
		"name": "Home", "currency": "PKR", "timezone": "Asia/Karachi", "aiExtractionConsent": true,
	}, &hh)
	if !hh.AiExtractionConsent {
		t.Fatal("consent not saved")
	}
	// Omitting the field keeps the current value.
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/household", map[string]any{
		"name": "Home 2", "currency": "PKR", "timezone": "Asia/Karachi",
	}, &hh)
	if !hh.AiExtractionConsent {
		t.Fatal("consent reset when the field was omitted")
	}
}

func TestReferenceNumbersEncrypted(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Encrypt")
	const ref = "04 11234 5678901"
	p := cl.createProfile(map[string]any{
		"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO, "referenceNumber": ref,
	})
	if p.ReferenceNumber == nil || *p.ReferenceNumber != ref {
		t.Fatalf("API reference = %v, want %q", p.ReferenceNumber, ref)
	}

	var raw []byte
	if err := testDB.Pool.QueryRow(context.Background(),
		"SELECT row_to_json(p)::text::bytea FROM bill_profiles p WHERE id = $1", p.Id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("5678901")) {
		t.Fatal("reference number stored in plain text")
	}

	var history []byte
	if err := testDB.Pool.QueryRow(context.Background(),
		"SELECT coalesce(string_agg(changes::text, ' '), '') FROM audit_events WHERE entity_id = $1", p.Id).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(history, []byte("5678901")) || !bytes.Contains(history, []byte("8901")) {
		t.Fatalf("audit history should hold only the masked number: %s", history)
	}

	// Changing to a new number is still recorded (masked).
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/profiles/"+p.Id.String(), map[string]any{
		"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO, "referenceNumber": "04 11234 5670000",
	}, nil)
	if err := testDB.Pool.QueryRow(context.Background(),
		"SELECT coalesce(string_agg(changes::text, ' '), '') FROM audit_events WHERE entity_id = $1 AND action = 'update'", p.Id).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(history, []byte("0000")) || bytes.Contains(history, []byte("5670000")) {
		t.Fatalf("update audit = %s", history)
	}
}
