package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// billView is the common shape of every bill query row (bill + derived status/outstanding).
type billView struct {
	Bill             store.Bill
	Status           string
	OutstandingMinor int64
}

func openapiEmail(s string) openapi_types.Email { return openapi_types.Email(s) }

func toDate(t time.Time) openapi_types.Date { return openapi_types.Date{Time: t} }

func toDatePtr(t *time.Time) *openapi_types.Date {
	if t == nil {
		return nil
	}
	d := toDate(*t)
	return &d
}

func fromDatePtr(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func toHousehold(h store.Household, usage apigen.StorageUsage) apigen.Household {
	return apigen.Household{
		Id: h.ID, Name: h.Name, Currency: h.Currency, Timezone: h.Timezone,
		AiExtractionConsent: h.AiExtractionConsent, Storage: usage,
	}
}

func toBillType(c store.Category) apigen.BillType {
	return apigen.BillType{
		Id:          c.ID,
		Name:        c.Name,
		Icon:        c.Icon,
		DefaultKind: apigen.ProfileKind(c.DefaultKind),
		Group:       apigen.TypeGroup(c.TypeGroup),
		IsSystem:    c.HouseholdID == nil,
		ParentId:    c.ParentID,
		Archived:    c.ArchivedAt != nil,
	}
}

func toProvider(p store.Provider) apigen.Provider {
	return apigen.Provider{
		Id:               p.ID,
		Name:             p.Name,
		ShortName:        p.ShortName,
		TypeId:           p.CategoryID,
		Region:           p.Region,
		RefNumberLabel:   p.RefNumberLabel,
		RefNumberPattern: p.RefNumberPattern,
		ConsumptionUnit:  p.ConsumptionUnit,
		IsCustom:         p.HouseholdID != nil,
	}
}

func toProfile(p store.BillProfile, reference *string, summary apigen.ProfileSummary) apigen.Profile {
	out := apigen.Profile{
		Id:                  p.ID,
		Name:                p.Name,
		TypeId:              p.CategoryID,
		Kind:                apigen.ProfileKind(p.Kind),
		Icon:                p.Icon,
		Color:               int(p.Color),
		PropertyId:          p.PropertyID,
		ProviderId:          p.ProviderID,
		ReferenceNumber:     reference,
		MeterNumber:         p.MeterNumber,
		MultiplyingFactor:   p.MultiplyingFactor,
		ConsumptionUnit:     p.ConsumptionUnit,
		ExpectedAmountMinor: p.ExpectedAmountMinor,
		AutoCreateBill:      p.AutoCreateBill,
		IsDefault:           p.IsDefault,
		Pinned:              p.Pinned,
		SortOrder:           int(p.SortOrder),
		Archived:            p.ArchivedAt != nil,
		Summary:             summary,
	}
	if p.BillingCycle != nil {
		c := apigen.BillingCycle(*p.BillingCycle)
		out.BillingCycle = &c
	}
	if p.TypicalIssueDay != nil {
		d := int(*p.TypicalIssueDay)
		out.TypicalIssueDay = &d
	}
	if p.TypicalDueDay != nil {
		d := int(*p.TypicalDueDay)
		out.TypicalDueDay = &d
	}
	return out
}

func toBill(v billView) apigen.Bill {
	b := v.Bill
	return apigen.Bill{
		Id:                   b.ID,
		ProfileId:            b.ProfileID,
		TypeId:               b.CategoryID,
		Description:          b.Description,
		BillNumber:           b.BillNumber,
		PeriodKey:            b.PeriodKey,
		PeriodStart:          toDatePtr(b.PeriodStart),
		PeriodEnd:            toDatePtr(b.PeriodEnd),
		IssueDate:            toDatePtr(b.IssueDate),
		DueDate:              toDatePtr(b.DueDate),
		ExpenseDate:          toDate(b.ExpenseDate),
		CurrentChargesMinor:  b.CurrentChargesMinor,
		ArrearsMinor:         b.ArrearsMinor,
		AdjustmentsMinor:     b.AdjustmentsMinor,
		PayableMinor:         b.PayableMinor,
		PayableAfterDueMinor: b.PayableAfterDueMinor,
		LateStepUntil:        toDatePtr(b.LateStepUntil),
		PayableFinalMinor:    b.PayableFinalMinor,
		SettledByBillId:      b.SettledByBillID,
		PreviousReading:      b.PreviousReading,
		CurrentReading:       b.CurrentReading,
		Units:                b.Units,
		MultiplyingFactor:    b.MultiplyingFactor,
		MeterReplaced:        b.MeterReplaced,
		Currency:             b.Currency,
		Status:               apigen.BillStatus(v.Status),
		PaidMinor:            b.PaidMinor,
		OutstandingMinor:     v.OutstandingMinor,
		IsEstimated:          b.IsEstimated,
		Cancelled:            b.Cancelled,
		Notes:                b.Notes,
		Tags:                 nonNilTags(b.Tags),
		Archived:             b.ArchivedAt != nil,
		CreatedAt:            b.CreatedAt,
		UpdatedAt:            b.UpdatedAt,
	}
}

func toBillDetail(v billView, payments []store.Payment, history []store.ListAuditEventsRow, documents []apigen.Document) apigen.BillDetail {
	b := toBill(v)
	d := apigen.BillDetail{
		Id: b.Id, ProfileId: b.ProfileId, TypeId: b.TypeId, Description: b.Description, BillNumber: b.BillNumber,
		PeriodKey: b.PeriodKey, PeriodStart: b.PeriodStart, PeriodEnd: b.PeriodEnd, IssueDate: b.IssueDate,
		DueDate: b.DueDate, ExpenseDate: b.ExpenseDate, CurrentChargesMinor: b.CurrentChargesMinor,
		ArrearsMinor: b.ArrearsMinor, AdjustmentsMinor: b.AdjustmentsMinor, PayableMinor: b.PayableMinor,
		PayableAfterDueMinor: b.PayableAfterDueMinor, Currency: b.Currency, Status: b.Status,
		LateStepUntil: b.LateStepUntil, PayableFinalMinor: b.PayableFinalMinor, SettledByBillId: b.SettledByBillId,
		PreviousReading: b.PreviousReading, CurrentReading: b.CurrentReading, Units: b.Units,
		MultiplyingFactor: b.MultiplyingFactor, MeterReplaced: b.MeterReplaced,
		PaidMinor: b.PaidMinor, OutstandingMinor: b.OutstandingMinor, IsEstimated: b.IsEstimated,
		Cancelled: b.Cancelled, Notes: b.Notes, Tags: b.Tags, Archived: b.Archived,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
		Payments:  make([]apigen.Payment, 0, len(payments)),
		History:   make([]apigen.AuditEvent, 0, len(history)),
		Documents: documents,
	}
	receipts := map[uuid.UUID]uuid.UUID{}
	for _, doc := range documents {
		if doc.PaymentId != nil {
			receipts[*doc.PaymentId] = doc.Id
		}
	}
	for _, p := range payments {
		pm := toPayment(p)
		if id, ok := receipts[p.ID]; ok {
			pm.ReceiptDocumentId = &id
		}
		d.Payments = append(d.Payments, pm)
	}
	for _, h := range history {
		d.History = append(d.History, toAuditEvent(h))
	}
	return d
}

func toPayment(p store.Payment) apigen.Payment {
	return apigen.Payment{
		Id: p.ID, BillId: p.BillID, PaidOn: toDate(p.PaidOn), AmountMinor: p.AmountMinor,
		Method: apigen.PaymentMethod(p.Method), Reference: p.Reference, Notes: p.Notes, CreatedAt: p.CreatedAt,
	}
}

func toAuditEvent(a store.ListAuditEventsRow) apigen.AuditEvent {
	changes := map[string]any{}
	_ = json.Unmarshal(a.Changes, &changes)
	return apigen.AuditEvent{
		Id: a.ID, At: a.At, ActorName: a.ActorName, EntityType: a.EntityType, Action: a.Action, Changes: changes,
	}
}

func nonNilTags(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}
