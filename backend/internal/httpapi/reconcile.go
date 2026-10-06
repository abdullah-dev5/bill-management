package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// The fields sources are compared on (docs/06 §3). Values are canonical strings: money in
// minor units, dates YYYY-MM-DD, months YYYY-MM, readings as decimals.
var reconcileFields = []string{
	"periodKey", "billNumber", "issueDate", "dueDate", "currentChargesMinor", "arrearsMinor",
	"payableMinor", "payableAfterDueMinor", "previousReading", "currentReading", "units",
}

func fmtFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func putDate(m map[string]string, k string, t *time.Time) {
	if t != nil {
		m[k] = dateString(*t)
	}
}

func putFloat(m map[string]string, k string, v *float64) {
	if v != nil {
		m[k] = fmtFloat(*v)
	}
}

// billValues is what a bill currently holds, for the reconciled fields.
func billValues(b store.Bill) map[string]string {
	m := map[string]string{
		"currentChargesMinor": strconv.FormatInt(b.CurrentChargesMinor, 10),
		"payableMinor":        strconv.FormatInt(b.PayableMinor, 10),
	}
	if b.PeriodKey != nil {
		m["periodKey"] = *b.PeriodKey
	}
	if b.BillNumber != nil && *b.BillNumber != "" {
		m["billNumber"] = *b.BillNumber
	}
	putDate(m, "issueDate", b.IssueDate)
	putDate(m, "dueDate", b.DueDate)
	if b.ArrearsMinor != 0 {
		m["arrearsMinor"] = strconv.FormatInt(b.ArrearsMinor, 10)
	}
	if b.PayableAfterDueMinor != nil {
		m["payableAfterDueMinor"] = strconv.FormatInt(*b.PayableAfterDueMinor, 10)
	}
	putFloat(m, "previousReading", b.PreviousReading)
	putFloat(m, "currentReading", b.CurrentReading)
	putFloat(m, "units", b.Units)
	return m
}

// candidateValues is what a new source says. Zero arrears means "not read", not "no arrears".
func candidateValues(f billFields) map[string]string {
	m := map[string]string{
		"currentChargesMinor": strconv.FormatInt(f.Current, 10),
		"payableMinor":        strconv.FormatInt(f.Payable, 10),
	}
	if f.PeriodKey != nil {
		m["periodKey"] = *f.PeriodKey
	}
	if f.BillNumber != nil && *f.BillNumber != "" {
		m["billNumber"] = *f.BillNumber
	}
	putDate(m, "issueDate", f.IssueDate)
	putDate(m, "dueDate", f.DueDate)
	if f.Arrears != 0 {
		m["arrearsMinor"] = strconv.FormatInt(f.Arrears, 10)
	}
	if f.PayableAfterDue != nil {
		m["payableAfterDueMinor"] = strconv.FormatInt(*f.PayableAfterDue, 10)
	}
	putFloat(m, "previousReading", f.PreviousReading)
	putFloat(m, "currentReading", f.CurrentReading)
	putFloat(m, "units", f.Units)
	return m
}

// sameValue: money and dates exact, readings ±0.5, bill numbers ignoring spaces and punctuation.
func sameValue(field, a, b string) bool {
	switch field {
	case "previousReading", "currentReading", "units":
		x, err1 := strconv.ParseFloat(a, 64)
		y, err2 := strconv.ParseFloat(b, 64)
		return err1 == nil && err2 == nil && math.Abs(x-y) <= 0.5
	case "billNumber":
		return domain.NormalizeReference(a) == domain.NormalizeReference(b)
	}
	return a == b
}

func updateParams(b store.Bill) store.UpdateBillParams {
	return store.UpdateBillParams{
		ID: b.ID, HouseholdID: b.HouseholdID, ProfileID: b.ProfileID, CategoryID: b.CategoryID, Description: b.Description,
		BillNumber: b.BillNumber, PeriodKey: b.PeriodKey, PeriodStart: b.PeriodStart, PeriodEnd: b.PeriodEnd,
		IssueDate: b.IssueDate, DueDate: b.DueDate, ExpenseDate: b.ExpenseDate, CurrentChargesMinor: b.CurrentChargesMinor,
		ArrearsMinor: b.ArrearsMinor, AdjustmentsMinor: b.AdjustmentsMinor, PayableMinor: b.PayableMinor,
		PayableAfterDueMinor: b.PayableAfterDueMinor, IsEstimated: b.IsEstimated, Cancelled: b.Cancelled, Notes: b.Notes,
		Tags: nonNilTags(b.Tags), LateStepUntil: b.LateStepUntil, PayableFinalMinor: b.PayableFinalMinor,
		PreviousReading: b.PreviousReading, CurrentReading: b.CurrentReading, Units: b.Units,
		MultiplyingFactor: b.MultiplyingFactor, MeterReplaced: b.MeterReplaced,
	}
}

// setField writes one reconciled field; an empty value clears an optional one.
func setField(p *store.UpdateBillParams, field, value string) error {
	bad := func() error { return errField("value", fmt.Sprintf("%q isn't a valid value for this field.", value)) }
	optDate := func() (*time.Time, error) {
		if value == "" {
			return nil, nil
		}
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, bad()
		}
		return &t, nil
	}
	money := func() (int64, error) {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 0 {
			return 0, bad()
		}
		return n, nil
	}
	optFloat := func() (*float64, error) {
		if value == "" {
			return nil, nil
		}
		f, err := strconv.ParseFloat(value, 64)
		if err != nil || f < 0 {
			return nil, bad()
		}
		return &f, nil
	}
	var err error
	switch field {
	case "periodKey":
		if !domain.ValidPeriodKey(value) {
			return bad()
		}
		p.PeriodKey = &value
		p.ExpenseDate, _ = time.Parse("2006-01", value)
	case "billNumber":
		p.BillNumber = domain.CleanText(&value)
	case "issueDate":
		p.IssueDate, err = optDate()
	case "dueDate":
		p.DueDate, err = optDate()
	case "currentChargesMinor":
		p.CurrentChargesMinor, err = money()
	case "arrearsMinor":
		if value == "" {
			p.ArrearsMinor = 0
		} else {
			p.ArrearsMinor, err = money()
		}
	case "payableMinor":
		p.PayableMinor, err = money()
	case "payableAfterDueMinor":
		if value == "" {
			p.PayableAfterDueMinor = nil
		} else {
			var n int64
			n, err = money()
			p.PayableAfterDueMinor = &n
		}
	case "previousReading":
		p.PreviousReading, err = optFloat()
	case "currentReading":
		p.CurrentReading, err = optFloat()
	case "units":
		p.Units, err = optFloat()
	default:
		return errField("field", "This field can't be changed here.")
	}
	return err
}

type sourceInput struct {
	Kind         store.BillSourceKind
	DocumentID   *uuid.UUID
	ExtractionID *uuid.UUID
	Connector    *string
}

// recordCreatedSource notes the channel that created a bill.
func recordCreatedSource(ctx context.Context, q *store.Queries, now time.Time, hhID uuid.UUID, createdBy *uuid.UUID, b store.Bill, src sourceInput) error {
	values, _ := json.Marshal(billValues(b))
	_, err := q.CreateBillSource(ctx, store.CreateBillSourceParams{
		ID: uuid.Must(uuid.NewV7()), HouseholdID: hhID, BillID: b.ID, Kind: src.Kind, DocumentID: src.DocumentID,
		ExtractionID: src.ExtractionID, Connector: src.Connector, FieldValues: values, FilledFields: []string{}, CreatedBy: createdBy,
		CreatedAt: now,
	})
	return err
}

type mergeOutcome struct {
	Filled    []string
	Conflicts []string
	Confirmed []string
}

// mergeCandidate attaches a new source to an existing bill (docs/06 §3): empty bill fields are
// filled from it, equal values confirm, and differing values become conflicts. The bill's own
// values never change here. Audited as "merge".
func mergeCandidate(ctx context.Context, q *store.Queries, now time.Time, a authInfo, b store.Bill, cand map[string]string, src sourceInput) (mergeOutcome, error) {
	var out mergeOutcome
	current := billValues(b)
	params := updateParams(b)
	sourceID := uuid.Must(uuid.NewV7())
	type pending struct{ field, billValue, sourceValue string }
	var conflicts []pending
	for _, f := range reconcileFields {
		cv, ok := cand[f]
		if !ok || cv == "" {
			continue
		}
		bv, has := current[f]
		switch {
		case !has:
			if err := setField(&params, f, cv); err != nil {
				return out, err
			}
			out.Filled = append(out.Filled, f)
		case sameValue(f, bv, cv):
			out.Confirmed = append(out.Confirmed, f)
		default:
			conflicts = append(conflicts, pending{f, bv, cv})
			out.Conflicts = append(out.Conflicts, f)
		}
	}
	values, _ := json.Marshal(cand)
	if _, err := q.CreateBillSource(ctx, store.CreateBillSourceParams{
		ID: sourceID, HouseholdID: a.HouseholdID, BillID: b.ID, Kind: src.Kind, DocumentID: src.DocumentID,
		ExtractionID: src.ExtractionID, Connector: src.Connector, FieldValues: values, FilledFields: nonNilStrings(out.Filled),
		Merged: true, CreatedBy: userPtr(a), CreatedAt: now,
	}); err != nil {
		return out, err
	}
	if len(out.Filled) > 0 {
		if _, err := q.UpdateBill(ctx, params); err != nil {
			return out, err
		}
	}
	for _, c := range conflicts {
		if _, err := q.CreateBillConflict(ctx, store.CreateBillConflictParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: a.HouseholdID, BillID: b.ID, SourceID: sourceID,
			Field: c.field, BillValue: c.billValue, SourceValue: c.sourceValue, CreatedAt: now,
		}); err != nil {
			return out, err
		}
	}
	// A source agreeing with the bill settles an open conflict on that field (docs/06 §4).
	for _, f := range out.Confirmed {
		open, err := q.OpenConflictForField(ctx, store.OpenConflictForFieldParams{BillID: b.ID, Field: f})
		if store.IsNotFound(err) {
			continue
		} else if err != nil {
			return out, err
		}
		if err := q.ResolveBillConflict(ctx, store.ResolveBillConflictParams{
			ID: open.ID, Resolution: ptr("confirmed"), ResolvedValue: ptr(current[f]), ResolvedAt: &now,
		}); err != nil {
			return out, err
		}
	}
	changes := map[string]any{"source": string(src.Kind), "filled": nonNilStrings(out.Filled), "conflicts": nonNilStrings(out.Conflicts)}
	if a.UserID == uuid.Nil {
		raw, _ := json.Marshal(changes)
		return out, q.CreateSystemAuditEvent(ctx, store.CreateSystemAuditEventParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: a.HouseholdID, EntityType: "bill", EntityID: b.ID, Action: "merge", Changes: raw,
		})
	}
	return out, audit(ctx, q, a, "bill", b.ID, "merge", changes)
}

func userPtr(a authInfo) *uuid.UUID {
	if a.UserID == uuid.Nil {
		return nil
	}
	return &a.UserID
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
