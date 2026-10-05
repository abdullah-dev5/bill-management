package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract"
	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// SaveScannedBill turns a checked extraction into a bill with its document attached
// (FR-EXTR-08), deduplicating against the profile's bills first (FR-RECO-01…03).
func (s *Server) SaveScannedBill(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	var in apigen.ScannedBillInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	var billID uuid.UUID
	status := http.StatusCreated
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
		if doc.BillID != nil {
			e := errConflict("already_attached", "This document is already attached to a bill.")
			e.existingBillID = doc.BillID
			return e
		}
		hh, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		today := s.today(hh.Timezone)
		f, err := buildBill(ctx, q, a, today, in.Bill, nil)
		if err != nil {
			return err
		}

		existing, kind, err := findSameBill(ctx, q, f)
		if err != nil {
			return err
		}
		choice := ""
		if in.Duplicate != nil {
			choice = string(*in.Duplicate)
		}
		src := sourceInput{Kind: store.BillSourceKindUpload, DocumentID: &doc.ID, ExtractionID: in.ExtractionId}
		if doc.Source == store.DocumentSourceCamera {
			src.Kind = store.BillSourceKindScan
		}
		switch {
		case existing != uuid.Nil && (choice == "attach"):
			billID, status = existing, http.StatusOK
			// The scan's values are compared with the bill: blanks filled, disagreements become conflicts.
			target, err := q.GetBill(ctx, store.GetBillParams{ID: existing, HouseholdID: a.HouseholdID, Today: today})
			if err != nil {
				return err
			}
			if _, err := mergeCandidate(ctx, q, s.now(), a, target.Bill, candidateValues(f), src); err != nil {
				return err
			}
		case existing != uuid.Nil && kind == "duplicate_bill":
			e := &apiError{status: http.StatusConflict, code: "duplicate_bill",
				message: "This profile already has this bill. Attach the document to it instead?", existingBillID: &existing}
			if f.PeriodKey != nil {
				e.message = "This profile already has a bill for " + monthLabel(*f.PeriodKey) + ". Attach the document to it instead?"
			}
			return e
		case existing != uuid.Nil && choice != "keep_separate":
			return &apiError{status: http.StatusConflict, code: "probable_duplicate",
				message: "This looks like a bill you've already added. Attach the document to it, or keep it as a separate bill?", existingBillID: &existing}
		default:
			billID = uuid.Must(uuid.NewV7())
			now := s.now()
			if _, err := q.CreateBill(ctx, store.CreateBillParams{
				ID: billID, HouseholdID: a.HouseholdID, ProfileID: f.ProfileID, CategoryID: f.CategoryID,
				Description: f.Description, BillNumber: f.BillNumber, PeriodKey: f.PeriodKey,
				PeriodStart: f.PeriodStart, PeriodEnd: f.PeriodEnd, IssueDate: f.IssueDate, DueDate: f.DueDate,
				ExpenseDate: f.ExpenseDate, CurrentChargesMinor: f.Current, ArrearsMinor: f.Arrears,
				AdjustmentsMinor: f.Adjustments, PayableMinor: f.Payable, PayableAfterDueMinor: f.PayableAfterDue,
				Currency: hh.Currency, IsEstimated: f.IsEstimated, Cancelled: f.Cancelled,
				// The user checked every value on the verification screen.
				VerifiedAt: &now, VerifiedBy: &a.UserID, Notes: f.Notes, Tags: f.Tags, CreatedBy: &a.UserID,
				LateStepUntil: f.LateStepUntil, PayableFinalMinor: f.PayableFinal,
				PreviousReading: f.PreviousReading, CurrentReading: f.CurrentReading, Units: f.Units,
				MultiplyingFactor: f.MultiplyingFactor, MeterReplaced: f.MeterReplaced,
			}); err != nil {
				if store.UniqueViolation(err) == "bills_profile_period_uniq" {
					return checkDuplicate(ctx, s.db.Queries, f, uuid.Nil)
				}
				return err
			}
			created, err := q.GetBill(ctx, store.GetBillParams{ID: billID, HouseholdID: a.HouseholdID, Today: today})
			if err != nil {
				return err
			}
			changes := diff(snapshot{}, billSnapshot(created.Bill))
			changes["source"] = []any{nil, string(doc.Source)}
			if err := audit(ctx, q, a, "bill", billID, "create", changes); err != nil {
				return err
			}
			if err := recordCreatedSource(ctx, q, s.now(), a.HouseholdID, &a.UserID, created.Bill, src); err != nil {
				return err
			}
		}

		if _, err := q.UpdateDocument(ctx, store.UpdateDocumentParams{ID: doc.ID, HouseholdID: a.HouseholdID, BillID: &billID, Kind: doc.Kind}); err != nil {
			return err
		}
		if err := audit(ctx, q, a, "bill", billID, "attach_document", map[string]any{"document": []any{nil, doc.FileName}}); err != nil {
			return err
		}
		if in.History != nil {
			if err := saveHistory(ctx, q, a, f.ProfileID, doc.ID, *in.History); err != nil {
				return err
			}
		}
		if err := s.settleArrears(ctx, q, a, billID, today); err != nil {
			return err
		}
		if in.ExtractionId != nil {
			return s.recordCorrections(ctx, q, a, *in.ExtractionId, f)
		}
		return nil
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeBillDetail(w, r, status, a, billID)
}

// findSameBill returns an existing bill that is the same as f: exactly (same billing month
// or bill number) or probably (close dates / same amount).
func findSameBill(ctx context.Context, q *store.Queries, f billFields) (uuid.UUID, string, error) {
	if f.PeriodKey != nil {
		id, err := q.FindBillForPeriod(ctx, store.FindBillForPeriodParams{ProfileID: f.ProfileID, PeriodKey: f.PeriodKey, ExcludeID: uuid.Nil})
		if err == nil {
			return id, "duplicate_bill", nil
		} else if !store.IsNotFound(err) {
			return uuid.Nil, "", err
		}
	}
	if f.BillNumber != nil {
		id, err := q.FindBillByNumber(ctx, store.FindBillByNumberParams{ProfileID: f.ProfileID, BillNumber: f.BillNumber})
		if err == nil {
			return id, "duplicate_bill", nil
		} else if !store.IsNotFound(err) {
			return uuid.Nil, "", err
		}
	}
	id, err := q.FindProbableBill(ctx, store.FindProbableBillParams{
		ProfileID: f.ProfileID, IssueDate: f.IssueDate, DueDate: f.DueDate, PayableMinor: f.Payable,
	})
	if err == nil {
		return id, "probable_duplicate", nil
	} else if !store.IsNotFound(err) {
		return uuid.Nil, "", err
	}
	return uuid.Nil, "", nil
}

func saveHistory(ctx context.Context, q *store.Queries, a authInfo, profileID, documentID uuid.UUID, rows []apigen.HistoryEntry) error {
	for _, h := range rows {
		if !domain.ValidPeriodKey(h.Month) {
			return errField("history", "Invalid month "+h.Month+".")
		}
		units := h.Units
		if err := q.UpsertProfileHistory(ctx, store.UpsertProfileHistoryParams{
			ProfileID: profileID, HouseholdID: a.HouseholdID, PeriodKey: h.Month,
			Units: units, BilledMinor: h.BilledMinor, PaidMinor: h.PaidMinor, DocumentID: &documentID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// recordCorrections stores what extraction read next to what the user saved (FR-EXTR-14).
// Reference numbers are stored masked.
func (s *Server) recordCorrections(ctx context.Context, q *store.Queries, a authInfo, extractionID uuid.UUID, f billFields) error {
	e, err := q.GetExtraction(ctx, store.GetExtractionParams{ID: extractionID, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		return errField("extractionId", "Unknown extraction.")
	} else if err != nil {
		return err
	}
	var res storedResult
	if len(e.Result) == 0 || json.Unmarshal(e.Result, &res) != nil {
		return nil
	}
	money := func(v int64) *string { s := strconv.FormatInt(v, 10); return &s }
	moneyPtr := func(v *int64) *string {
		if v == nil {
			return nil
		}
		return money(*v)
	}
	num := func(v *float64) *string {
		if v == nil {
			return nil
		}
		s := strconv.FormatFloat(*v, 'f', -1, 64)
		return &s
	}
	date := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := t.Format("2006-01-02")
		return &s
	}
	confirmed := map[string]*string{
		extract.FBillingMonth:    f.PeriodKey,
		extract.FIssueDate:       date(f.IssueDate),
		extract.FDueDate:         date(f.DueDate),
		extract.FPeriodStart:     date(f.PeriodStart),
		extract.FPeriodEnd:       date(f.PeriodEnd),
		extract.FBillNumber:      f.BillNumber,
		extract.FCurrentCharges:  money(f.Current),
		extract.FArrears:         money(f.Arrears),
		extract.FAdjustments:     money(f.Adjustments),
		extract.FPayable:         money(f.Payable),
		extract.FPayableAfterDue: moneyPtr(f.PayableAfterDue),
		extract.FPreviousReading: num(f.PreviousReading),
		extract.FCurrentReading:  num(f.CurrentReading),
		extract.FUnits:           num(f.Units),
	}
	if f.BillNumber != nil {
		n := domain.NormalizeReference(*f.BillNumber)
		confirmed[extract.FBillNumber] = &n
	}
	names := make([]string, 0, len(confirmed))
	for name := range confirmed {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		got, ok := res.Fields[name]
		want := confirmed[name]
		if !ok && want == nil {
			continue
		}
		var extracted *string
		var conf *float64
		if ok {
			v := got.Value
			extracted = &v
			c := got.Confidence
			conf = &c
		}
		if err := q.CreateExtractionCorrection(ctx, store.CreateExtractionCorrectionParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: a.HouseholdID, ExtractionID: extractionID,
			Field: name, Extracted: extracted, Confirmed: want, Source: strPtr(got.Source), Confidence: conf,
		}); err != nil {
			return err
		}
	}
	if ref, ok := res.Fields[extract.FReference]; ok {
		masked := secure.Mask(ref.Value)
		c := ref.Confidence
		if err := q.CreateExtractionCorrection(ctx, store.CreateExtractionCorrectionParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: a.HouseholdID, ExtractionID: extractionID,
			Field: extract.FReference, Extracted: &masked, Source: strPtr(ref.Source), Confidence: &c,
		}); err != nil {
			return err
		}
	}
	return nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetProfileHistory merges the profile's bills with history printed on its documents.
func (s *Server) GetProfileHistory(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	if _, err := s.db.GetProfile(ctx, store.GetProfileParams{ID: id, HouseholdID: a.HouseholdID}); store.IsNotFound(err) {
		s.fail(w, r, errNotFound("profile"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	printed, err := s.db.ListProfileHistory(ctx, store.ListProfileHistoryParams{ProfileID: id, HouseholdID: a.HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	bills, err := s.db.ListProfileBillMonths(ctx, store.ListProfileBillMonthsParams{ProfileID: id, HouseholdID: a.HouseholdID, Today: s.today(hh.Timezone)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	months := map[string]*apigen.HistoryMonth{}
	for _, p := range printed {
		m := &apigen.HistoryMonth{Month: p.PeriodKey, Source: apigen.HistoryMonthSourcePrinted, BilledMinor: p.BilledMinor, PaidMinor: p.PaidMinor}
		m.Units = p.Units
		months[p.PeriodKey] = m
	}
	for _, b := range bills {
		billed, paid, billID := b.BilledMinor, b.PaidMinor, b.ID
		m, ok := months[b.PeriodKey]
		if !ok {
			m = &apigen.HistoryMonth{Month: b.PeriodKey}
			months[b.PeriodKey] = m
		}
		if b.Units != nil {
			m.Units = b.Units
		}
		// The bill itself wins over printed history; printed units are kept until readings exist (M3).
		status := apigen.BillStatus(b.Status)
		m.Source, m.BilledMinor, m.PaidMinor, m.BillId, m.Status = apigen.HistoryMonthSourceBill, &billed, &paid, &billID, &status
	}
	out := make([]apigen.HistoryMonth, 0, len(months))
	for _, m := range months {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Month < out[j].Month })
	writeJSON(w, http.StatusOK, out)
}

// ensureProfile checks the profile belongs to the household.
func (s *Server) ensureProfile(w http.ResponseWriter, r *http.Request, id uuid.UUID) bool {
	_, err := s.db.GetProfile(r.Context(), store.GetProfileParams{ID: id, HouseholdID: mustAuth(r).HouseholdID})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("profile"))
		return false
	} else if err != nil {
		s.fail(w, r, err)
		return false
	}
	return true
}

// ListProfilePayments lists the payments made towards a profile's bills (profile dashboard).
func (s *Server) ListProfilePayments(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	if !s.ensureProfile(w, r, id) {
		return
	}
	rows, err := s.db.ListProfilePayments(r.Context(), store.ListProfilePaymentsParams{ProfileID: id, HouseholdID: mustAuth(r).HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]apigen.ProfilePayment, 0, len(rows))
	for _, p := range rows {
		out = append(out, apigen.ProfilePayment{
			Id: p.ID, BillId: p.BillID, PaidOn: toDate(p.PaidOn), AmountMinor: p.AmountMinor,
			Method: apigen.PaymentMethod(p.Method), Reference: p.Reference, Notes: p.Notes, CreatedAt: p.CreatedAt,
			ReceiptDocumentId: p.ReceiptDocumentID, BillPeriodKey: p.PeriodKey, BillDescription: p.Description,
			BillExpenseDate: toDate(p.ExpenseDate),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ListProfileDocuments lists the documents of a profile's bills (profile dashboard).
func (s *Server) ListProfileDocuments(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	if !s.ensureProfile(w, r, id) {
		return
	}
	ctx := r.Context()
	rows, err := s.db.ListProfileDocuments(ctx, store.ListProfileDocumentsParams{ProfileID: id, HouseholdID: mustAuth(r).HouseholdID})
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
