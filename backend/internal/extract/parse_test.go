package extract

import (
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
)

var catalogue = []Provider{{ShortName: "LESCO"}, {ShortName: "HESCO"}, {ShortName: "SNGPL"}, {ShortName: "PTCL"}, {ShortName: "K-Electric"}}

func wantFields(t *testing.T, r Result, want map[string]string) {
	t.Helper()
	for name, v := range want {
		if got := r.Fields[name].Value; got != v {
			t.Errorf("%s = %q, want %q (confidence %.2f)", name, got, v, r.Fields[name].Confidence)
		}
	}
}

func TestParsePITCHesco(t *testing.T) {
	r := ParseText(extracttest.HescoText, catalogue, "pdf-text")
	Validate(&r, Context{ReferencePattern: `^\d{14}[A-Z]?$`})

	wantFields(t, r, map[string]string{
		FProvider:          "HESCO",
		FReference:         "09123450012345",
		FBillingMonth:      "2026-09",
		FIssueDate:         "2026-09-18",
		FReadingDate:       "2026-09-18",
		FDueDate:           "2026-10-05",
		FPreviousReading:   "18690",
		FCurrentReading:    "18854",
		FUnits:             "164",
		FMultiplyingFactor: "1",
		FArrears:           "0",
		FPayable:           "326600",
		FCurrentCharges:    "326600",
		FPayableAfterDue:   "338900",
	})
	if len(r.Warnings) != 0 {
		t.Errorf("unexpected warnings: %+v", r.Warnings)
	}
	if !r.Complete() || r.Score() < 0.9 {
		t.Errorf("score %.2f, complete %v", r.Score(), r.Complete())
	}
	if len(r.LateFees) != 2 || r.LateFees[0] != (LateFee{Until: "2026-10-08", AmountMinor: 338900}) || r.LateFees[1].AmountMinor != 351200 {
		t.Errorf("late fees = %+v", r.LateFees)
	}
	var sum int64
	for _, li := range r.LineItems {
		sum += li.AmountMinor
	}
	if len(r.LineItems) != 4 || sum != 326600 {
		t.Errorf("line items = %+v (sum %d)", r.LineItems, sum)
	}
	if len(r.History) != 12 || r.History[0].Month != "2025-09" || r.History[11].Month != "2026-08" {
		t.Fatalf("history = %+v", r.History)
	}
	if h := r.History[4]; h.Month != "2026-01" || *h.Units != 0 || *h.BilledMinor != 8900 {
		t.Errorf("Jan 26 row = %+v", h)
	}
	if got := r.Text(FCustomerName); got == "" {
		t.Error("customer name not found (needed for redaction)")
	}
}

func TestParseGenericBill(t *testing.T) {
	text := `NAYATEL PRIVATE LIMITED
Customer ID: NT-448812
Invoice No: 2026-09-55123
Bill Date: 01/09/2026
Billing Period: September 2026
Previous Balance        Rs. 0
Current Charges         Rs. 4,499.00
Total Amount Due        Rs. 4,499.00
Due Date: 15 Sep 2026
Amount after due date   Rs. 4,699.00`
	r := ParseText(text, catalogue, "ocrspace")
	Validate(&r, Context{})
	wantFields(t, r, map[string]string{
		FReference:       "NT448812",
		FBillNumber:      "20260955123",
		FIssueDate:       "2026-09-01",
		FBillingMonth:    "2026-09",
		FArrears:         "0",
		FCurrentCharges:  "449900",
		FPayable:         "449900",
		FDueDate:         "2026-09-15",
		FPayableAfterDue: "469900",
	})
	if r.Fields[FPayable].Confidence != 0.7 {
		t.Errorf("generic confidence = %.2f, want 0.7", r.Fields[FPayable].Confidence)
	}
}

func TestValidateFlagsInconsistencies(t *testing.T) {
	r := NewResult()
	r.set(FPayable, "500000", 0.9, "x")
	r.set(FCurrentCharges, "300000", 0.9, "x")
	r.set(FPreviousReading, "100", 0.9, "x")
	r.set(FCurrentReading, "200", 0.9, "x")
	r.set(FUnits, "150", 0.9, "x")
	r.set(FIssueDate, "2026-09-18", 0.9, "x")
	r.set(FDueDate, "2026-09-10", 0.9, "x")
	r.set(FReference, "123", 0.9, "x")
	Validate(&r, Context{ReferencePattern: `^\d{14}$`})

	// The payable is kept; the inconsistent charges are replaced by the derived value.
	if r.Text(FPayable) != "500000" || r.Text(FCurrentCharges) != "500000" || r.Fields[FCurrentCharges].Confidence != 0.75 {
		t.Errorf("payable %q, charges %+v", r.Text(FPayable), r.Fields[FCurrentCharges])
	}
	if _, ok := r.Fields[FUnits]; ok {
		t.Error("units contradicting the readings should be dropped")
	}
	if len(r.Warnings) < 4 {
		t.Errorf("warnings = %+v", r.Warnings)
	}
}

func TestMergePrefersConfidence(t *testing.T) {
	a := NewResult()
	a.set(FPayable, "100", 0.7, "ocr")
	a.set(FDueDate, "2026-10-05", 0.9, "ocr")
	b := NewResult()
	b.set(FPayable, "200", 0.9, "gemini")
	b.set(FDueDate, "2026-10-06", 0.8, "gemini")
	b.set(FReference, "X1", 0.8, "gemini")
	a.Merge(b)
	wantFields(t, a, map[string]string{FPayable: "200", FDueDate: "2026-10-05", FReference: "X1"})
}

func TestCrossCheck(t *testing.T) {
	r := NewResult()
	r.set(FPayable, "326600", 0.9, "gemini")
	r.set(FArrears, "990000", 0.9, "gemini")
	CrossCheck(&r, "Grand Total 3,266 ... ")
	if r.Fields[FPayable].Confidence != 0.9 {
		t.Error("an amount present in the text was penalised")
	}
	if r.Fields[FArrears].Confidence != 0.54 {
		t.Errorf("missing amount confidence = %.2f, want 0.54", r.Fields[FArrears].Confidence)
	}
}

func TestValidateLateSurchargesBecomeTotals(t *testing.T) {
	r := NewResult()
	r.set(FPayable, "326600", 0.9, "gemini")
	r.LateFees = []LateFee{{Until: "2026-10-08", AmountMinor: 12300}, {AmountMinor: 24600}}
	Validate(&r, Context{})
	if r.LateFees[0].AmountMinor != 338900 || r.LateFees[1].AmountMinor != 351200 || r.Text(FPayableAfterDue) != "338900" {
		t.Errorf("late fees %+v, after due %q", r.LateFees, r.Text(FPayableAfterDue))
	}
}
