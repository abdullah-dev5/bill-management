package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract"
	"github.com/abdullah-dev5/bill-management/backend/internal/storage"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const (
	maxDocumentBytes  = 10 << 20
	maxPDFPages       = 5
	linkTTL           = 5 * time.Minute
	stalePendingAfter = 24 * time.Hour
	filesPath         = "/api/files/"
)

var documentExt = map[string]string{
	"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "application/pdf": ".pdf",
}

func (s *Server) ListDocuments(w http.ResponseWriter, r *http.Request, params apigen.ListDocumentsParams) {
	a := mustAuth(r)
	ctx := r.Context()
	var rows []store.Document
	var err error
	switch {
	case params.BillId != nil:
		rows, err = s.db.ListBillDocuments(ctx, store.ListBillDocumentsParams{HouseholdID: a.HouseholdID, BillID: params.BillId})
	case deref(params.Unattached):
		rows, err = s.db.ListUnattachedDocuments(ctx, a.HouseholdID)
	default:
		err = errField("billId", "Pass billId or unattached=true.")
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out, err := s.toDocuments(ctx, rows)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	d, err := s.db.GetDocument(r.Context(), store.GetDocumentParams{ID: id, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("document"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeDocument(w, r, http.StatusOK, d)
}

func (s *Server) CreateDocument(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.DocumentUploadInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	sum, err := hex.DecodeString(in.Sha256)
	if err != nil || len(sum) != sha256.Size {
		s.fail(w, r, errField("sha256", "Invalid SHA-256."))
		return
	}
	contentType := string(in.ContentType)
	fileName := cleanFileName(in.FileName, documentExt[contentType])
	if !s.uploadsByHH.Allow(a.HouseholdID.String()) {
		s.fail(w, r, errTooManyRequests)
		return
	}

	ctx := r.Context()
	var doc store.Document
	duplicate := false
	err = s.db.InTx(ctx, func(q *store.Queries) error {
		if in.BillId != nil {
			if err := s.ensureBillExists(ctx, q, a, *in.BillId); err != nil {
				return err
			}
		}
		existing, err := q.GetDocumentBySHA256(ctx, store.GetDocumentBySHA256Params{HouseholdID: a.HouseholdID, Sha256: sum})
		switch {
		case err == nil && existing.Status == store.DocumentStatusReady:
			duplicate = true
			doc = existing
			// The same file uploaded to a bill while an unattached copy exists: attach that copy.
			if in.BillId != nil && existing.BillID == nil {
				doc, err = q.UpdateDocument(ctx, store.UpdateDocumentParams{
					ID: existing.ID, HouseholdID: a.HouseholdID, BillID: in.BillId, Kind: existing.Kind,
				})
				if err != nil {
					return err
				}
				return audit(ctx, q, a, "bill", *in.BillId, "attach_document", map[string]any{"document": []any{nil, doc.FileName}})
			}
			return nil
		case err == nil:
			// An abandoned upload of the same file: start it again.
			if err := s.checkQuota(ctx, q, a.HouseholdID, in.SizeBytes-existing.SizeBytes); err != nil {
				return err
			}
			doc, err = q.RefreshPendingDocument(ctx, store.RefreshPendingDocumentParams{
				ID: existing.ID, HouseholdID: a.HouseholdID, BillID: in.BillId,
				Kind: store.DocumentKind(in.Kind), Source: store.DocumentSource(in.Source), FileName: fileName,
				ContentType: contentType, SizeBytes: in.SizeBytes, CreatedBy: &a.UserID,
			})
			return err
		case !store.IsNotFound(err):
			return err
		}
		if err := s.checkQuota(ctx, q, a.HouseholdID, in.SizeBytes); err != nil {
			return err
		}
		doc, err = q.CreateDocument(ctx, store.CreateDocumentParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: a.HouseholdID, BillID: in.BillId,
			Kind: store.DocumentKind(in.Kind), Source: store.DocumentSource(in.Source),
			StorageKey: "households/" + a.HouseholdID.String() + "/" + uuid.NewString(),
			FileName:   fileName, ContentType: contentType, SizeBytes: in.SizeBytes, Sha256: sum, CreatedBy: &a.UserID,
		})
		return err
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}

	out := apigen.DocumentUpload{Duplicate: duplicate}
	out.Document, err = s.toDocument(ctx, doc)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if !duplicate {
		status = http.StatusCreated
		u, err := s.uploadURL(ctx, doc)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		out.Upload = &struct {
			Headers map[string]string                 `json:"headers"`
			Method  apigen.DocumentUploadUploadMethod `json:"method"`
			Url     string                            `json:"url"`
		}{Headers: map[string]string{"Content-Type": doc.ContentType}, Method: apigen.PUT, Url: u}
	}
	writeJSON(w, status, out)
}

// CompleteDocument checks the stored bytes against what the client declared before
// the document becomes visible: size, SHA-256 and the real (sniffed) file type.
func (s *Server) CompleteDocument(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	doc, err := s.db.GetDocument(ctx, store.GetDocumentParams{ID: id, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("document"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	if doc.Status == store.DocumentStatusReady {
		s.writeDocument(w, r, http.StatusOK, doc)
		return
	}

	pages, problem, err := s.verifyStoredFile(ctx, doc)
	if err != nil {
		s.fail(w, r, err)
		return
	} else if problem != "" {
		if _, err := s.db.DeleteDocument(ctx, store.DeleteDocumentParams{ID: doc.ID, HouseholdID: a.HouseholdID}); err != nil {
			s.fail(w, r, err)
			return
		}
		s.fail(w, r, &apiError{status: http.StatusBadRequest, code: "upload_rejected", message: problem})
		return
	}

	err = s.db.InTx(ctx, func(q *store.Queries) error {
		doc, err = q.MarkDocumentReady(ctx, store.MarkDocumentReadyParams{ID: doc.ID, HouseholdID: a.HouseholdID, PageCount: pages})
		if err != nil || doc.BillID == nil {
			return err
		}
		return audit(ctx, q, a, "bill", *doc.BillID, "attach_document", map[string]any{"document": []any{nil, doc.FileName}})
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeDocument(w, r, http.StatusOK, doc)
}

// verifyStoredFile returns a user-facing problem when the upload is missing or doesn't match,
// and the page count for PDFs.
func (s *Server) verifyStoredFile(ctx context.Context, doc store.Document) (*int32, string, error) {
	rc, err := s.files.Open(ctx, doc.StorageKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, "The file didn't finish uploading. Please try again.", nil
	} else if err != nil {
		return nil, "", err
	}
	data, err := io.ReadAll(io.LimitReader(rc, maxDocumentBytes+1))
	rc.Close()
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	switch {
	case int64(len(data)) != doc.SizeBytes:
		return nil, "The uploaded file is incomplete. Please try again.", nil
	case !bytes.Equal(sum[:], doc.Sha256):
		return nil, "The uploaded file doesn't match. Please try again.", nil
	case http.DetectContentType(data) != doc.ContentType:
		return nil, "This file isn't a valid JPG, PNG, WEBP or PDF.", nil
	}
	if doc.ContentType != "application/pdf" {
		return nil, "", nil
	}
	pages, err := extract.PDFPageCount(data)
	switch {
	case err != nil:
		return nil, "This PDF can't be opened. It may be damaged or password-protected.", nil
	case pages > maxPDFPages:
		return nil, fmt.Sprintf("This PDF has %d pages; the limit is %d. Upload only the bill's pages.", pages, maxPDFPages), nil
	}
	n := int32(pages)
	return &n, "", nil
}

func (s *Server) UpdateDocument(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	var in apigen.DocumentUpdate
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	var doc store.Document
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		before, err := q.GetDocument(ctx, store.GetDocumentParams{ID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("document")
		} else if err != nil {
			return err
		}
		if in.BillId != nil {
			if err := s.ensureBillExists(ctx, q, a, *in.BillId); err != nil {
				return err
			}
		}
		doc, err = q.UpdateDocument(ctx, store.UpdateDocumentParams{
			ID: id, HouseholdID: a.HouseholdID, BillID: in.BillId, Kind: store.DocumentKind(in.Kind),
		})
		if err != nil || before.Status != store.DocumentStatusReady || sameBill(before.BillID, doc.BillID) {
			return err
		}
		if before.BillID != nil {
			if err := audit(ctx, q, a, "bill", *before.BillID, "remove_document", map[string]any{"document": []any{before.FileName, nil}}); err != nil {
				return err
			}
		}
		if doc.BillID != nil {
			return audit(ctx, q, a, "bill", *doc.BillID, "attach_document", map[string]any{"document": []any{nil, doc.FileName}})
		}
		return nil
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeDocument(w, r, http.StatusOK, doc)
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		doc, err := q.GetDocument(ctx, store.GetDocumentParams{ID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("document")
		} else if err != nil {
			return err
		}
		if _, err := q.DeleteDocument(ctx, store.DeleteDocumentParams{ID: id, HouseholdID: a.HouseholdID}); err != nil {
			return err
		}
		if doc.BillID != nil && doc.Status == store.DocumentStatusReady {
			return audit(ctx, q, a, "bill", *doc.BillID, "remove_document", map[string]any{"document": []any{doc.FileName, nil}})
		}
		return nil
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) checkQuota(ctx context.Context, q *store.Queries, householdID uuid.UUID, extra int64) error {
	u, err := q.HouseholdStorageUsage(ctx, householdID)
	if err != nil {
		return err
	}
	if u.UsedBytes+extra > s.cfg.StorageQuotaBytes {
		return errConflict("storage_quota_exceeded", "Your household's document storage is full. Delete some documents to upload more.")
	}
	return nil
}

func (s *Server) writeDocument(w http.ResponseWriter, r *http.Request, status int, d store.Document) {
	out, err := s.toDocument(r.Context(), d)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, status, out)
}

func (s *Server) toDocuments(ctx context.Context, rows []store.Document) ([]apigen.Document, error) {
	out := make([]apigen.Document, 0, len(rows))
	for _, d := range rows {
		doc, err := s.toDocument(ctx, d)
		if err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, nil
}

// toDocument converts a row; ready documents get fresh signed links.
func (s *Server) toDocument(ctx context.Context, d store.Document) (apigen.Document, error) {
	out := apigen.Document{
		Id: d.ID, BillId: d.BillID, Kind: apigen.DocumentKind(d.Kind), Source: apigen.DocumentSource(d.Source),
		Status: apigen.DocumentStatus(d.Status), FileName: d.FileName, ContentType: d.ContentType,
		SizeBytes: d.SizeBytes, CreatedAt: d.CreatedAt, PageCount: int32ToIntPtr(d.PageCount),
	}
	if d.Status != store.DocumentStatusReady {
		return out, nil
	}
	view, err := s.downloadURL(ctx, d, "inline")
	if err != nil {
		return out, err
	}
	dl, err := s.downloadURL(ctx, d, "attachment")
	if err != nil {
		return out, err
	}
	exp := s.now().Add(linkTTL)
	out.ViewUrl, out.DownloadUrl, out.UrlsExpireAt = &view, &dl, &exp
	return out, nil
}

func (s *Server) presigner() (storage.Presigner, bool) {
	if !s.cfg.DirectUploads {
		return nil, false
	}
	p, ok := s.files.(storage.Presigner)
	return p, ok
}

func (s *Server) uploadURL(ctx context.Context, d store.Document) (string, error) {
	if p, ok := s.presigner(); ok {
		return p.PresignPut(ctx, d.StorageKey, d.ContentType, d.SizeBytes, linkTTL)
	}
	tok := s.keys.Sign(putPayload(d), s.now().Add(linkTTL))
	return filesPath + d.ID.String() + "?" + url.Values{"t": {tok}}.Encode(), nil
}

func (s *Server) downloadURL(ctx context.Context, d store.Document, disposition string) (string, error) {
	if p, ok := s.presigner(); ok {
		return p.PresignGet(ctx, d.StorageKey, d.ContentType, contentDisposition(disposition, d.FileName), linkTTL)
	}
	tok := s.keys.Sign(getPayload(d.ID, disposition), s.now().Add(linkTTL))
	return filesPath + d.ID.String() + "?" + url.Values{"d": {disposition}, "t": {tok}}.Encode(), nil
}

func putPayload(d store.Document) string {
	return "PUT|" + d.ID.String() + "|" + d.ContentType + "|" + strconv.FormatInt(d.SizeBytes, 10)
}

func getPayload(id uuid.UUID, disposition string) string {
	return "GET|" + id.String() + "|" + disposition
}

// putFile receives an upload through a signed link (local storage, or direct uploads off).
func (s *Server) putFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, errNotFound("file"))
		return
	}
	doc, err := s.db.GetDocumentByID(ctx, id)
	if store.IsNotFound(err) || (err == nil && !s.keys.Verify(r.URL.Query().Get("t"), putPayload(doc), s.now())) {
		writeError(w, errForbidden("This upload link is invalid or has expired."))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	if doc.Status != store.DocumentStatusPending {
		writeError(w, errConflict("already_uploaded", "This file was already uploaded."))
		return
	}
	if r.Header.Get("Content-Type") != doc.ContentType {
		writeError(w, errField("Content-Type", "Must be "+doc.ContentType+"."))
		return
	}
	_ = http.NewResponseController(w).SetReadDeadline(s.now().Add(5 * time.Minute))
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, doc.SizeBytes))
	if err != nil || int64(len(body)) != doc.SizeBytes {
		writeError(w, errField("body", "The file size doesn't match."))
		return
	}
	if err := s.files.Put(ctx, doc.StorageKey, doc.ContentType, bytes.NewReader(body), doc.SizeBytes); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getFile serves a document through a signed link.
func (s *Server) getFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	disposition := r.URL.Query().Get("d")
	if err != nil || (disposition != "inline" && disposition != "attachment") ||
		!s.keys.Verify(r.URL.Query().Get("t"), getPayload(id, disposition), s.now()) {
		writeError(w, errForbidden("This link is invalid or has expired."))
		return
	}
	doc, err := s.db.GetDocumentByID(ctx, id)
	if store.IsNotFound(err) || (err == nil && doc.Status != store.DocumentStatusReady) {
		writeError(w, errNotFound("file"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	rc, err := s.files.Open(ctx, doc.StorageKey)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, errNotFound("file"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	defer rc.Close()

	h := w.Header()
	h.Set("Content-Type", doc.ContentType)
	h.Set("Content-Length", strconv.FormatInt(doc.SizeBytes, 10))
	h.Set("Content-Disposition", contentDisposition(disposition, doc.FileName))
	h.Set("Cache-Control", "private, max-age=300")
	if doc.ContentType != "application/pdf" {
		// Browsers' PDF viewers don't run in a sandbox; images never need scripts.
		h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	}
	_ = http.NewResponseController(w).SetWriteDeadline(s.now().Add(5 * time.Minute))
	_, _ = io.Copy(w, rc)
}

func contentDisposition(disposition, fileName string) string {
	v := mime.FormatMediaType(disposition, map[string]string{"filename": fileName})
	if v == "" {
		return disposition
	}
	return v
}

// cleanFileName keeps a readable base name without paths or control characters.
func cleanFileName(name, ext string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 120 {
		name = string(r[:120])
	}
	if name == "" || name == "." || name == "/" {
		name = "document" + ext
	}
	return name
}

func sameBill(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func int32ToIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}
