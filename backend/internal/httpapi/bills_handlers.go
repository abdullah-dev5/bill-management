package httpapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const (
	defaultPageSize = 50
	dueSoonWindow   = 14 * 24 * time.Hour
)

func (s *Server) ListBills(w http.ResponseWriter, r *http.Request, params apigen.ListBillsParams) {
	a := mustAuth(r)
	ctx := r.Context()
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	limit := defaultPageSize
	if params.Limit != nil {
		limit = *params.Limit
	}
	q := store.ListBillsParams{
		Today:       s.today(hh.Timezone),
		HouseholdID: a.HouseholdID,
		ProfileID:   params.ProfileId,
		Archived:    deref(params.Archived),
		FromDate:    fromDatePtr(params.From),
		ToDate:      fromDatePtr(params.To),
		RowLimit:    int32(limit + 1),
	}
	if params.Status != nil {
		for _, st := range *params.Status {
			q.Statuses = append(q.Statuses, string(st))
		}
	}
	q.ProviderID, q.MinAmount, q.MaxAmount = params.ProviderId, params.MinAmount, params.MaxAmount
	if q.MinAmount != nil && q.MaxAmount != nil && *q.MinAmount > *q.MaxAmount {
		s.fail(w, r, errField("maxAmount", "The highest amount is below the lowest."))
		return
	}
	if params.Source != nil {
		for _, src := range *params.Source {
			q.Sources = append(q.Sources, string(src))
		}
	}
	if params.TypeId != nil && len(*params.TypeId) > 0 {
		// A type includes its sub-categories (e.g. Maintenance → Electrician, Plumber).
		types, err := s.db.ListCategories(ctx, &a.HouseholdID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		for _, id := range *params.TypeId {
			q.TypeIds = append(q.TypeIds, id)
			for _, t := range types {
				if t.ParentID != nil && *t.ParentID == id {
					q.TypeIds = append(q.TypeIds, t.ID)
				}
			}
		}
	}
	if params.Cursor != nil {
		d, id, ok := decodeCursor(*params.Cursor)
		if !ok {
			s.fail(w, r, errField("cursor", "Invalid cursor."))
			return
		}
		q.CursorDate, q.CursorID = &d, &id
	}
	rows, err := s.db.ListBills(ctx, q)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page := apigen.BillPage{Items: make([]apigen.Bill, 0, len(rows))}
	for i, row := range rows {
		if i == limit {
			c := encodeCursor(rows[i-1].Bill.ExpenseDate, rows[i-1].Bill.ID)
			page.NextCursor = &c
			break
		}
		page.Items = append(page.Items, toBill(billView(row)))
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) GetBill(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	s.writeBillDetail(w, r, http.StatusOK, mustAuth(r), id)
}

func (s *Server) CreateBill(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.BillCreate
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	id := uuid.Must(uuid.NewV7())
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		hh, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		today := s.today(hh.Timezone)
		f, err := buildBill(ctx, q, a, today, createToInput(in), nil)
		if err != nil {
			return err
		}
		if err := checkDuplicate(ctx, q, f, uuid.Nil); err != nil {
			return err
		}
		now := s.now()
		if _, err := q.CreateBill(ctx, store.CreateBillParams{
			ID: id, HouseholdID: a.HouseholdID, ProfileID: f.ProfileID, CategoryID: f.CategoryID,
			Description: f.Description, BillNumber: f.BillNumber, PeriodKey: f.PeriodKey,
			PeriodStart: f.PeriodStart, PeriodEnd: f.PeriodEnd, IssueDate: f.IssueDate, DueDate: f.DueDate,
			ExpenseDate: f.ExpenseDate, CurrentChargesMinor: f.Current, ArrearsMinor: f.Arrears,
			AdjustmentsMinor: f.Adjustments, PayableMinor: f.Payable, PayableAfterDueMinor: f.PayableAfterDue,
			Currency: hh.Currency, IsEstimated: f.IsEstimated, Cancelled: f.Cancelled,
			// Manual entries are verified by the person typing them (FR-EXTR-08 applies to extracted bills).
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
		created, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: today})
		if err != nil {
			return err
		}
		if err := audit(ctx, q, a, "bill", id, "create", diff(snapshot{}, billSnapshot(created.Bill))); err != nil {
			return err
		}
		if in.PaidNow != nil && !f.Cancelled {
			if err := recordFullPayment(ctx, q, a, created.Bill, *in.PaidNow, today); err != nil {
				return err
			}
		}
		return s.settleArrears(ctx, q, a, id, today)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeBillDetail(w, r, http.StatusCreated, a, id)
}

func (s *Server) UpdateBill(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	var in apigen.BillInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		hh, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		today := s.today(hh.Timezone)
		before, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: today})
		if store.IsNotFound(err) {
			return errNotFound("bill")
		} else if err != nil {
			return err
		}
		f, err := buildBill(ctx, q, a, today, in, &before.Bill)
		if err != nil {
			return err
		}
		if before.Bill.ArchivedAt == nil {
			if err := checkDuplicate(ctx, q, f, id); err != nil {
				return err
			}
		}
		if _, err := q.UpdateBill(ctx, store.UpdateBillParams{
			ID: id, HouseholdID: a.HouseholdID, ProfileID: f.ProfileID, CategoryID: f.CategoryID,
			Description: f.Description, BillNumber: f.BillNumber, PeriodKey: f.PeriodKey,
			PeriodStart: f.PeriodStart, PeriodEnd: f.PeriodEnd, IssueDate: f.IssueDate, DueDate: f.DueDate,
			ExpenseDate: f.ExpenseDate, CurrentChargesMinor: f.Current, ArrearsMinor: f.Arrears,
			AdjustmentsMinor: f.Adjustments, PayableMinor: f.Payable, PayableAfterDueMinor: f.PayableAfterDue,
			IsEstimated: f.IsEstimated, Cancelled: f.Cancelled, Notes: f.Notes, Tags: f.Tags,
			LateStepUntil: f.LateStepUntil, PayableFinalMinor: f.PayableFinal,
			PreviousReading: f.PreviousReading, CurrentReading: f.CurrentReading, Units: f.Units,
			MultiplyingFactor: f.MultiplyingFactor, MeterReplaced: f.MeterReplaced,
		}); err != nil {
			if store.UniqueViolation(err) == "bills_profile_period_uniq" {
				return checkDuplicate(ctx, s.db.Queries, f, id)
			}
			return err
		}
		// The due date decides which payments were on time.
		if err := q.RecalcBillPaid(ctx, id); err != nil {
			return err
		}
		if err := s.settleArrears(ctx, q, a, id, today); err != nil {
			return err
		}
		after, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: today})
		if err != nil {
			return err
		}
		action := "update"
		if f.ProfileID != before.Bill.ProfileID {
			action = "move" // FR-ACCT-14
		}
		return audit(ctx, q, a, "bill", id, action, diff(billSnapshot(before.Bill), billSnapshot(after.Bill)))
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeBillDetail(w, r, http.StatusOK, a, id)
}

func (s *Server) ArchiveBill(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		n, err := q.ArchiveBill(ctx, store.ArchiveBillParams{ID: id, HouseholdID: a.HouseholdID})
		if err != nil {
			return err
		}
		if n == 0 {
			return s.ensureBillExists(ctx, q, a, id)
		}
		return audit(ctx, q, a, "bill", id, "archive", nil)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) RestoreBill(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		n, err := q.RestoreBill(ctx, store.RestoreBillParams{ID: id, HouseholdID: a.HouseholdID})
		if err != nil {
			if store.UniqueViolation(err) == "bills_profile_period_uniq" {
				return errConflict("duplicate_bill", "Another bill for the same month already exists in this profile.")
			}
			return err
		}
		if n == 0 {
			return s.ensureBillExists(ctx, q, a, id)
		}
		return audit(ctx, q, a, "bill", id, "restore", nil)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) DeleteBill(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		n, err := q.DeleteArchivedBill(ctx, store.DeleteArchivedBillParams{ID: id, HouseholdID: a.HouseholdID})
		if err != nil {
			return err
		}
		if n == 0 {
			if err := s.ensureBillExists(ctx, q, a, id); err != nil {
				return err
			}
			return errConflict("not_archived", "Archive the bill before deleting it permanently.")
		}
		return audit(ctx, q, a, "bill", id, "delete", nil)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Payments --------------------------------------------------------------------

func (s *Server) CreatePayment(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	var in apigen.PaymentInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		hh, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		today := s.today(hh.Timezone)
		bill, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: today})
		if store.IsNotFound(err) {
			return errNotFound("bill")
		} else if err != nil {
			return err
		}
		if bill.Bill.ArchivedAt != nil || bill.Bill.Cancelled {
			return errConflict("bill_closed", "Payments can't be added to an archived or cancelled bill.")
		}
		if in.PaidOn.Time.After(today.AddDate(0, 0, 1)) {
			return errField("paidOn", "The payment date can't be in the future.")
		}
		paymentID, err := addPayment(ctx, q, a, bill.Bill.ID, in.PaidOn.Time, in.AmountMinor, store.PaymentMethod(in.Method),
			domain.CleanText(in.Reference), domain.CleanText(in.Notes))
		if err != nil {
			return err
		}
		if in.ReceiptDocumentId != nil {
			n, err := q.SetDocumentPayment(ctx, store.SetDocumentPaymentParams{
				ID: *in.ReceiptDocumentId, HouseholdID: a.HouseholdID, BillID: &bill.Bill.ID, PaymentID: &paymentID,
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return errField("receiptDocumentId", "The receipt must be a document of this bill.")
			}
		}
		return s.settleArrears(ctx, q, a, bill.Bill.ID, today)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeBillDetail(w, r, http.StatusCreated, a, id)
}

func (s *Server) DeletePayment(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		p, err := q.GetPayment(ctx, store.GetPaymentParams{ID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("payment")
		} else if err != nil {
			return err
		}
		if err := q.DeletePayment(ctx, store.DeletePaymentParams{ID: id, HouseholdID: a.HouseholdID}); err != nil {
			return err
		}
		if err := q.RecalcBillPaid(ctx, p.BillID); err != nil {
			return err
		}
		if err := audit(ctx, q, a, "bill", p.BillID, "payment_deleted", map[string]any{
			"amountMinor": p.AmountMinor, "paidOn": dateString(p.PaidOn), "method": string(p.Method),
		}); err != nil {
			return err
		}
		hh, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		return s.settleArrears(ctx, q, a, p.BillID, s.today(hh.Timezone))
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func addPayment(ctx context.Context, q *store.Queries, a authInfo, billID uuid.UUID, paidOn time.Time, amount int64,
	method store.PaymentMethod, reference, notes *string) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	if _, err := q.CreatePayment(ctx, store.CreatePaymentParams{
		ID: id, HouseholdID: a.HouseholdID, BillID: billID, PaidOn: paidOn,
		AmountMinor: amount, Method: method, Reference: reference, Notes: notes, CreatedBy: &a.UserID,
	}); err != nil {
		return uuid.Nil, err
	}
	if err := q.RecalcBillPaid(ctx, billID); err != nil {
		return uuid.Nil, err
	}
	return id, audit(ctx, q, a, "bill", billID, "payment_added", map[string]any{
		"amountMinor": amount, "paidOn": dateString(paidOn), "method": string(method),
	})
}

// recordFullPayment handles "Already paid?" on create: pays the amount that applied on the payment date.
func recordFullPayment(ctx context.Context, q *store.Queries, a authInfo, b store.Bill, paid apigen.PaidNow, today time.Time) error {
	if paid.PaidOn.Time.After(today.AddDate(0, 0, 1)) {
		return errField("paidNow.paidOn", "The payment date can't be in the future.")
	}
	amount := amountDueOn(b, paid.PaidOn.Time)
	if amount <= 0 {
		return nil
	}
	_, err := addPayment(ctx, q, a, b.ID, paid.PaidOn.Time, amount, store.PaymentMethod(paid.Method), nil, nil)
	return err
}

// amountDueOn is the full amount payable on a given date: within the due date, after it,
// or after the late-fee step date.
func amountDueOn(b store.Bill, on time.Time) int64 {
	if b.DueDate == nil || !on.After(*b.DueDate) {
		return b.PayableMinor
	}
	if b.LateStepUntil != nil && b.PayableFinalMinor != nil && on.After(*b.LateStepUntil) {
		return *b.PayableFinalMinor
	}
	if b.PayableAfterDueMinor != nil {
		return *b.PayableAfterDueMinor
	}
	return b.PayableMinor
}

// --- Building & validating bills -------------------------------------------------

type billFields struct {
	ProfileID       uuid.UUID
	CategoryID      uuid.UUID
	Description     *string
	BillNumber      *string
	PeriodKey       *string
	PeriodStart     *time.Time
	PeriodEnd       *time.Time
	IssueDate       *time.Time
	DueDate         *time.Time
	ExpenseDate     time.Time
	Current         int64
	Arrears         int64
	Adjustments     int64
	Payable         int64
	PayableAfterDue *int64
	LateStepUntil   *time.Time
	PayableFinal    *int64

	PreviousReading   *float64
	CurrentReading    *float64
	Units             *float64
	MultiplyingFactor *float64
	MeterReplaced     bool
	IsEstimated       bool
	Cancelled         bool
	Notes             *string
	Tags              []string
}

// buildBill validates a bill against its profile's kind (FR-BILL-01/02/04). existing is nil on create.
func buildBill(ctx context.Context, q *store.Queries, a authInfo, today time.Time, in apigen.BillInput, existing *store.Bill) (billFields, error) {
	fields := domain.FieldErrors{}
	f := billFields{
		ProfileID:   in.ProfileId,
		Description: domain.CleanText(in.Description),
		BillNumber:  domain.CleanText(in.BillNumber),
		PeriodStart: fromDatePtr(in.PeriodStart),
		PeriodEnd:   fromDatePtr(in.PeriodEnd),
		IssueDate:   fromDatePtr(in.IssueDate),
		DueDate:     fromDatePtr(in.DueDate),
		Current:     in.CurrentChargesMinor,
		Arrears:     deref(in.ArrearsMinor),
		Adjustments: deref(in.AdjustmentsMinor),
		IsEstimated: deref(in.IsEstimated),
		Cancelled:   deref(in.Cancelled),
		Notes:       domain.CleanText(in.Notes),
	}
	if in.Tags != nil {
		f.Tags = domain.CleanTags(*in.Tags)
	} else {
		f.Tags = []string{}
	}

	profile, err := q.GetProfile(ctx, store.GetProfileParams{ID: in.ProfileId, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		return f, errField("profileId", "Choose a profile.")
	} else if err != nil {
		return f, err
	}
	sameProfile := existing != nil && existing.ProfileID == profile.ID
	if profile.ArchivedAt != nil && !sameProfile {
		fields.Add("profileId", "This profile is archived.")
	}

	// Type: the profile's own type, or one of its sub-categories (expense logs).
	f.CategoryID = profile.CategoryID
	if in.TypeId != nil && *in.TypeId != profile.CategoryID {
		sub, err := q.GetCategory(ctx, store.GetCategoryParams{ID: *in.TypeId, HouseholdID: &a.HouseholdID})
		switch {
		case store.IsNotFound(err):
			fields.Add("typeId", "Unknown sub-category.")
		case err != nil:
			return f, err
		case profile.Kind != store.ProfileKindExpenseLog:
			fields.Add("typeId", "Only expense-log profiles use sub-categories.")
		case sub.ParentID == nil || *sub.ParentID != profile.CategoryID:
			fields.Add("typeId", "This sub-category doesn't belong to the profile's type.")
		default:
			f.CategoryID = sub.ID
		}
	}

	if profile.Kind == store.ProfileKindExpenseLog {
		// Expense logs hold any number of entries; they have no billing month.
		if f.Description == nil {
			fields.Add("description", "Say what this expense was for.")
		}
	} else {
		key := strings.TrimSpace(deref(in.PeriodKey))
		switch {
		case key == "":
			fields.Add("periodKey", "Choose the billing month.")
		case !domain.ValidPeriodKey(key):
			fields.Add("periodKey", "Use the format YYYY-MM.")
		default:
			f.PeriodKey = &key
		}
	}

	if f.PeriodStart != nil && f.PeriodEnd != nil && f.PeriodEnd.Before(*f.PeriodStart) {
		fields.Add("periodEnd", "The period can't end before it starts.")
	}
	if f.IssueDate != nil && f.DueDate != nil && f.DueDate.Before(*f.IssueDate) {
		fields.Add("dueDate", "The due date can't be before the issue date.")
	}

	switch {
	case in.ExpenseDate != nil:
		f.ExpenseDate = in.ExpenseDate.Time
	case f.IssueDate != nil:
		f.ExpenseDate = *f.IssueDate
	case f.DueDate != nil:
		f.ExpenseDate = *f.DueDate
	case f.PeriodKey != nil:
		f.ExpenseDate, _ = domain.PeriodStart(*f.PeriodKey)
	case existing != nil:
		f.ExpenseDate = existing.ExpenseDate
	default:
		f.ExpenseDate = today
	}

	if in.PayableMinor != nil {
		f.Payable = *in.PayableMinor
	} else {
		f.Payable = f.Current + f.Arrears + f.Adjustments
		if f.Payable < 0 {
			fields.Add("adjustmentsMinor", "Adjustments can't make the payable amount negative.")
		}
	}
	if in.PayableAfterDueMinor != nil {
		if *in.PayableAfterDueMinor < f.Payable {
			fields.Add("payableAfterDueMinor", "Should be at least the amount payable by the due date.")
		}
		f.PayableAfterDue = in.PayableAfterDueMinor
	}
	f.LateStepUntil = fromDatePtr(in.LateStepUntil)
	f.PayableFinal = in.PayableFinalMinor
	switch {
	case f.LateStepUntil == nil && f.PayableFinal == nil:
	case f.LateStepUntil == nil || f.PayableFinal == nil:
		fields.Add("lateStepUntil", "Give both the date and the higher amount after it.")
	case f.PayableAfterDue == nil:
		fields.Add("payableAfterDueMinor", "Enter the amount payable after the due date first.")
	case f.DueDate == nil || f.LateStepUntil.Before(*f.DueDate):
		fields.Add("lateStepUntil", "Must be on or after the due date.")
	case *f.PayableFinal < *f.PayableAfterDue:
		fields.Add("payableFinalMinor", "Should be at least the amount payable after the due date.")
	}

	if profile.Kind == store.ProfileKindMetered {
		applyReadings(&f, in, profile, fields)
	}

	if fields.Any() {
		return f, errValidation(fields)
	}
	return f, nil
}

// applyReadings validates meter readings and works out the consumption (FR-CONS-01…04):
// units = (current − previous) × multiplying factor, unless the meter was replaced, when the
// printed units are used.
func applyReadings(f *billFields, in apigen.BillInput, profile store.BillProfile, fields domain.FieldErrors) {
	f.PreviousReading, f.CurrentReading, f.Units = in.PreviousReading, in.CurrentReading, in.Units
	f.MeterReplaced = deref(in.MeterReplaced)
	if f.PreviousReading == nil && f.CurrentReading == nil && f.Units == nil {
		return
	}
	mf := 1.0
	switch {
	case in.MultiplyingFactor != nil:
		mf = *in.MultiplyingFactor
	case profile.MultiplyingFactor != nil:
		mf = *profile.MultiplyingFactor
	}
	f.MultiplyingFactor = &mf

	if (f.PreviousReading == nil) != (f.CurrentReading == nil) {
		missing := "previousReading"
		if f.CurrentReading == nil {
			missing = "currentReading"
		}
		fields.Add(missing, "Enter both readings, or neither.")
		return
	}
	if f.PreviousReading == nil {
		return // units only
	}
	if f.MeterReplaced {
		if f.Units == nil {
			fields.Add("units", "The meter was replaced: enter the units printed on the bill.")
		}
		return
	}
	if *f.CurrentReading < *f.PreviousReading {
		fields.Add("currentReading", "Lower than the previous reading. If the meter was replaced, tick “Meter replaced”.")
		return
	}
	computed := math.Round((*f.CurrentReading-*f.PreviousReading)*mf*100) / 100
	if f.Units == nil {
		f.Units = &computed
	} else if math.Abs(*f.Units-computed) > 1 {
		fields.Add("units", fmt.Sprintf("The readings give %s units. Check the readings or the units.", strconv.FormatFloat(computed, 'f', -1, 64)))
	}
}

// checkDuplicate enforces one bill per profile per billing month (FR-RECO-01 for manual entry).
func checkDuplicate(ctx context.Context, q *store.Queries, f billFields, excludeID uuid.UUID) error {
	if f.PeriodKey == nil {
		return nil
	}
	existingID, err := q.FindBillForPeriod(ctx, store.FindBillForPeriodParams{ProfileID: f.ProfileID, PeriodKey: f.PeriodKey, ExcludeID: excludeID})
	if store.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	return &apiError{
		status: http.StatusConflict, code: "duplicate_bill",
		message:        "This profile already has a bill for " + monthLabel(*f.PeriodKey) + ".",
		fields:         map[string]string{"periodKey": "Already added for this month."},
		existingBillID: &existingID,
	}
}

func monthLabel(key string) string {
	t, err := domain.PeriodStart(key)
	if err != nil {
		return key
	}
	return t.Format("January 2006")
}

func createToInput(c apigen.BillCreate) apigen.BillInput {
	return apigen.BillInput{
		AdjustmentsMinor: c.AdjustmentsMinor, ArrearsMinor: c.ArrearsMinor, BillNumber: c.BillNumber,
		Cancelled: c.Cancelled, CurrentChargesMinor: c.CurrentChargesMinor, Description: c.Description,
		DueDate: c.DueDate, ExpenseDate: c.ExpenseDate, IsEstimated: c.IsEstimated, IssueDate: c.IssueDate,
		Notes: c.Notes, PayableAfterDueMinor: c.PayableAfterDueMinor, PayableMinor: c.PayableMinor,
		LateStepUntil: c.LateStepUntil, PayableFinalMinor: c.PayableFinalMinor,
		PreviousReading: c.PreviousReading, CurrentReading: c.CurrentReading, Units: c.Units,
		MultiplyingFactor: c.MultiplyingFactor, MeterReplaced: c.MeterReplaced,
		PeriodEnd: c.PeriodEnd, PeriodKey: c.PeriodKey, PeriodStart: c.PeriodStart, ProfileId: c.ProfileId,
		Tags: c.Tags, TypeId: c.TypeId,
	}
}

func (s *Server) ensureBillExists(ctx context.Context, q *store.Queries, a authInfo, id uuid.UUID) error {
	_, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: time.Now().UTC()})
	if store.IsNotFound(err) {
		return errNotFound("bill")
	}
	return err
}

func (s *Server) writeBillDetail(w http.ResponseWriter, r *http.Request, status int, a authInfo, id uuid.UUID) {
	ctx := r.Context()
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	row, err := s.db.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: s.today(hh.Timezone)})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("bill"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	payments, err := s.db.ListPaymentsForBill(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	history, err := s.db.ListAuditEvents(ctx, store.ListAuditEventsParams{HouseholdID: a.HouseholdID, EntityType: "bill", EntityID: id})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	docs, err := s.db.ListBillDocuments(ctx, store.ListBillDocumentsParams{HouseholdID: a.HouseholdID, BillID: &id})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	documents, err := s.toDocuments(ctx, docs)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, status, toBillDetail(billView(row), payments, history, documents))
}

func encodeCursor(d time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(d.Format("2006-01-02") + "|" + id.String()))
}

func decodeCursor(c string) (time.Time, uuid.UUID, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	datePart, idPart, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, uuid.Nil, false
	}
	d, err := time.Parse("2006-01-02", datePart)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(idPart)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return d, id, true
}
