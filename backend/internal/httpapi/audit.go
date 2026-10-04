package httpapi

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// snapshot is a flat, JSON-friendly view of an entity's auditable fields.
type snapshot map[string]any

// diff returns {field: [old, new]} for every field that changed.
func diff(before, after snapshot) map[string]any {
	out := map[string]any{}
	for k, nv := range after {
		if ov := before[k]; !reflect.DeepEqual(ov, nv) {
			out[k] = []any{ov, nv}
		}
	}
	for k, ov := range before {
		if _, ok := after[k]; !ok {
			out[k] = []any{ov, nil}
		}
	}
	return out
}

// audit appends an audit event (append-only; FR-BILL-05, NFR-05).
func audit(ctx context.Context, q *store.Queries, a authInfo, entityType string, entityID uuid.UUID, action string, changes map[string]any) error {
	if changes == nil {
		changes = map[string]any{}
	}
	body, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	actor := a.UserID
	return q.CreateAuditEvent(ctx, store.CreateAuditEventParams{
		ID:          uuid.Must(uuid.NewV7()),
		HouseholdID: a.HouseholdID,
		ActorUserID: &actor,
		EntityType:  entityType,
		EntityID:    entityID,
		Action:      action,
		Changes:     body,
	})
}

func dateString(t time.Time) string { return t.Format("2006-01-02") }

func billSnapshot(b store.Bill) snapshot {
	s := snapshot{
		"profileId":           b.ProfileID.String(),
		"typeId":              b.CategoryID.String(),
		"description":         deref(b.Description),
		"billNumber":          deref(b.BillNumber),
		"periodKey":           deref(b.PeriodKey),
		"expenseDate":         dateString(b.ExpenseDate),
		"currentChargesMinor": b.CurrentChargesMinor,
		"arrearsMinor":        b.ArrearsMinor,
		"adjustmentsMinor":    b.AdjustmentsMinor,
		"payableMinor":        b.PayableMinor,
		"isEstimated":         b.IsEstimated,
		"cancelled":           b.Cancelled,
		"notes":               deref(b.Notes),
		"tags":                nonNilTags(b.Tags),
	}
	addDate(s, "periodStart", b.PeriodStart)
	addDate(s, "periodEnd", b.PeriodEnd)
	addDate(s, "issueDate", b.IssueDate)
	addDate(s, "dueDate", b.DueDate)
	if b.PayableAfterDueMinor != nil {
		s["payableAfterDueMinor"] = *b.PayableAfterDueMinor
	}
	addDate(s, "lateStepUntil", b.LateStepUntil)
	for k, v := range map[string]*float64{"previousReading": b.PreviousReading, "currentReading": b.CurrentReading, "units": b.Units} {
		if v != nil {
			s[k] = *v
		}
	}
	if b.MeterReplaced {
		s["meterReplaced"] = true
	}
	if b.PayableFinalMinor != nil {
		s["payableFinalMinor"] = *b.PayableFinalMinor
	}
	return normalizeSnapshot(s)
}

// profileSnapshot takes the decrypted reference number; maskReference must run on the diff before it is stored.
func profileSnapshot(p store.BillProfile, reference *string) snapshot {
	s := snapshot{
		"name":            p.Name,
		"typeId":          p.CategoryID.String(),
		"kind":            string(p.Kind),
		"icon":            p.Icon,
		"color":           int(p.Color),
		"referenceNumber": deref(reference),
		"meterNumber":     deref(p.MeterNumber),
		"consumptionUnit": deref(p.ConsumptionUnit),
		"autoCreateBill":  p.AutoCreateBill,
		"pinned":          p.Pinned,
	}
	if p.ProviderID != nil {
		s["providerId"] = p.ProviderID.String()
	}
	if p.MultiplyingFactor != nil {
		s["multiplyingFactor"] = *p.MultiplyingFactor
	}
	if p.BillingCycle != nil {
		s["billingCycle"] = string(*p.BillingCycle)
	}
	if p.ExpectedAmountMinor != nil {
		s["expectedAmountMinor"] = *p.ExpectedAmountMinor
	}
	if p.TypicalIssueDay != nil {
		s["typicalIssueDay"] = int(*p.TypicalIssueDay)
	}
	if p.TypicalDueDay != nil {
		s["typicalDueDay"] = int(*p.TypicalDueDay)
	}
	return normalizeSnapshot(s)
}

func addDate(s snapshot, key string, t *time.Time) {
	if t != nil {
		s[key] = dateString(*t)
	}
}

// normalizeSnapshot drops empty strings so "" and absent compare equal.
func normalizeSnapshot(s snapshot) snapshot {
	for k, v := range s {
		if str, ok := v.(string); ok && str == "" {
			delete(s, k)
		}
	}
	return s
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// maskReference hides all but the last four characters of reference numbers in a diff,
// so audit history never stores them in plain text (docs/08 §4).
func maskReference(changes map[string]any) map[string]any {
	pair, ok := changes["referenceNumber"].([]any)
	if !ok {
		return changes
	}
	masked := make([]any, len(pair))
	for i, v := range pair {
		if str, ok := v.(string); ok && str != "" {
			masked[i] = secure.Mask(str)
		} else {
			masked[i] = v
		}
	}
	changes["referenceNumber"] = masked
	return changes
}
