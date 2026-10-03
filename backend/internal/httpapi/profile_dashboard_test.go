package httpapi

import (
	"net/http"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

func TestProfileDashboardLists(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Dashboard")
	p := cl.createProfile(map[string]any{"name": "Electricity", "typeId": typeElectricity})
	other := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})

	aug := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-08", "dueDate": "2026-09-10", "currentChargesMinor": 290000,
		"paidNow": map[string]any{"paidOn": "2026-09-05", "method": "jazzcash"}})
	sep := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-08", "currentChargesMinor": 326600})
	cl.createBill(map[string]any{"profileId": other.Id, "periodKey": "2026-09", "currentChargesMinor": 185000,
		"paidNow": map[string]any{"paidOn": "2026-10-01", "method": "cash"}})

	receipt := cl.upload(fakeJPEG, "image/jpeg", map[string]any{"billId": sep.Id})
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+sep.Id.String()+"/payments", map[string]any{
		"paidOn": "2026-10-02", "amountMinor": 100000, "method": "card", "receiptDocumentId": receipt.Id,
	}, nil)
	cl.upload(fakePDF, "application/pdf", map[string]any{"billId": aug.Id})

	var payments []apigen.ProfilePayment
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String()+"/payments", nil, &payments)
	if len(payments) != 2 {
		t.Fatalf("payments = %d, want 2 (only this profile's)", len(payments))
	}
	if first := payments[0]; first.AmountMinor != 100000 || first.ReceiptDocumentId == nil || *first.ReceiptDocumentId != receipt.Id ||
		first.BillPeriodKey == nil || *first.BillPeriodKey != "2026-09" {
		t.Errorf("newest payment = %+v", first)
	}
	if payments[1].ReceiptDocumentId != nil || *payments[1].BillPeriodKey != "2026-08" {
		t.Errorf("older payment = %+v", payments[1])
	}

	var docs []apigen.Document
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String()+"/documents", nil, &docs)
	if len(docs) != 2 || docs[0].ViewUrl == nil {
		t.Fatalf("documents = %+v", docs)
	}

	var months []apigen.HistoryMonth
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String()+"/history", nil, &months)
	if len(months) != 2 || months[0].Status == nil || *months[0].Status != apigen.Paid || *months[1].Status != apigen.PartiallyPaid {
		t.Fatalf("history statuses = %+v", months)
	}

	intruder, _ := e.signedUp("Snoop")
	for _, path := range []string{"/payments", "/documents", "/history"} {
		if got := intruder.do(http.MethodGet, "/api/v1/profiles/"+p.Id.String()+path, nil, nil); got != http.StatusNotFound {
			t.Errorf("%s by another household: %d, want 404", path, got)
		}
	}
}

// The Bills page filters on several statuses at once ("Unpaid" = unpaid, partially paid, overdue).
func TestBillsFilterByManyStatuses(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Filters")
	p := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-08", "currentChargesMinor": 100000,
		"paidNow": map[string]any{"paidOn": "2026-09-01", "method": "cash"}})
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-09-20", "currentChargesMinor": 100000})
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-10", "dueDate": "2026-10-20", "currentChargesMinor": 100000})

	var page apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?status=unpaid&status=partially_paid&status=overdue", nil, &page)
	if len(page.Items) != 2 {
		t.Fatalf("open bills = %d, want 2", len(page.Items))
	}
	for _, b := range page.Items {
		if b.Status == apigen.Paid {
			t.Errorf("a paid bill was returned: %s", *b.PeriodKey)
		}
	}
}
