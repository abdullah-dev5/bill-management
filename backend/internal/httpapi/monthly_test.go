package httpapi

import (
	"net/http"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

const typeGas = "a0000000-0000-4000-8000-000000000002"

// FR-DASH-07 acceptance (docs/02): electricity Rs 3,266 unpaid, gas Rs 1,850 paid, rent Rs 45,000 unpaid.
func TestMonthTotal(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Monthly")
	elec := cl.createProfile(map[string]any{"name": "Electricity", "typeId": typeElectricity})
	gas := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	rent := cl.createProfile(map[string]any{"name": "Flat rent", "typeId": typeRent})

	cl.createBill(map[string]any{"profileId": elec.Id, "periodKey": "2026-09", "dueDate": "2026-10-05", "currentChargesMinor": 326600})
	cl.createBill(map[string]any{"profileId": gas.Id, "periodKey": "2026-09", "dueDate": "2026-10-10", "currentChargesMinor": 185000,
		"paidNow": map[string]any{"paidOn": "2026-10-02", "method": "jazzcash"}})
	cl.createBill(map[string]any{"profileId": rent.Id, "periodKey": "2026-09", "dueDate": "2026-10-04", "currentChargesMinor": 4500000})
	// A different month and an archived bill don't count.
	cl.createBill(map[string]any{"profileId": elec.Id, "periodKey": "2026-08", "currentChargesMinor": 290000})
	old := cl.createBill(map[string]any{"profileId": gas.Id, "periodKey": "2026-07", "currentChargesMinor": 100000})
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/bills/"+old.Id.String()+"/archive", nil, nil)

	var home apigen.HomeSummary
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home?month=2026-09", nil, &home)
	m := home.MonthTotal
	if m.TotalMinor != 5011600 || m.PaidMinor != 185000 || m.RemainingMinor != 4826600 || m.BillCount != 3 {
		t.Fatalf("total %d paid %d remaining %d bills %d", m.TotalMinor, m.PaidMinor, m.RemainingMinor, m.BillCount)
	}
	if len(m.Groups) != 2 || m.Groups[0].Group != apigen.TypeGroupUtilities || m.Groups[0].TotalMinor != 511600 ||
		m.Groups[1].Group != apigen.TypeGroupHousehold || m.Groups[1].TotalMinor != 4500000 {
		t.Fatalf("groups = %+v", m.Groups)
	}
	if len(m.Profiles) != 3 {
		t.Fatalf("profiles = %+v", m.Profiles)
	}
	// Rent is due first (4 Oct) among the open bills.
	if m.NextDue == nil || m.NextDue.ProfileId != rent.Id {
		t.Fatalf("next due = %+v", m.NextDue)
	}

	// After the due date the electricity bill's late amount is what's left to pay.
	e.setDate("2026-10-06")
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home?month=2026-09", nil, &home)
	if home.MonthTotal.RemainingMinor != 4826600 {
		t.Errorf("remaining without a late amount should stay the same: %d", home.MonthTotal.RemainingMinor)
	}

	// An expense entry counts in the month of its date.
	general := cl.defaultProfile()
	cl.createBill(map[string]any{"profileId": general.Id, "description": "Plumber", "expenseDate": "2026-09-20", "currentChargesMinor": 250000})
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home?month=2026-09", nil, &home)
	if home.MonthTotal.TotalMinor != 5261600 || home.MonthTotal.BillCount != 4 {
		t.Errorf("with the expense: total %d, bills %d", home.MonthTotal.TotalMinor, home.MonthTotal.BillCount)
	}

	var empty apigen.HomeSummary
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home?month=2025-01", nil, &empty)
	if empty.MonthTotal.TotalMinor != 0 || empty.MonthTotal.NextDue != nil || empty.MonthTotal.Groups == nil {
		t.Errorf("empty month = %+v", empty.MonthTotal)
	}
}

func TestArrearsSettlement(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Arrears")
	p := cl.createProfile(map[string]any{"name": "Electricity", "typeId": typeElectricity})

	e.setDate("2026-09-20")
	aug := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-08", "dueDate": "2026-09-10",
		"currentChargesMinor": 700000, "payableAfterDueMinor": 735000})
	// September's bill carries August's late amount as arrears.
	sep := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-08",
		"currentChargesMinor": 800000, "arrearsMinor": 735000})
	if got := cl.getBill(aug.Id.String()); got.Status != apigen.Overdue {
		t.Fatalf("August before paying September: %s", got.Status)
	}
	// August's late amount is inside September's arrears, so it isn't counted again as "earlier".
	var home apigen.HomeSummary
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home?month=2026-09", nil, &home)
	if home.MonthTotal.EarlierOutstandingMinor != 0 || home.MonthTotal.RemainingMinor != 1535000 {
		t.Fatalf("earlier %d, remaining %d", home.MonthTotal.EarlierOutstandingMinor, home.MonthTotal.RemainingMinor)
	}

	var paid apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+sep.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-09-20", "amountMinor": 1535000, "method": "bank_transfer"}, &paid)
	if paid.Status != apigen.Paid {
		t.Fatalf("September after paying: %s", paid.Status)
	}
	got := cl.getBill(aug.Id.String())
	if got.Status != apigen.Paid || got.OutstandingMinor != 0 || got.SettledByBillId == nil || *got.SettledByBillId != sep.Id {
		t.Fatalf("August should be settled via September: %s %d %v", got.Status, got.OutstandingMinor, got.SettledByBillId)
	}
	if got.History[0].Action != "settled" {
		t.Errorf("August history = %q", got.History[0].Action)
	}

	// Undoing September's payment undoes the settlement.
	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/payments/"+paid.Payments[0].Id.String(), nil, nil)
	got = cl.getBill(aug.Id.String())
	if got.Status != apigen.Overdue || got.SettledByBillId != nil || got.History[0].Action != "unsettled" {
		t.Fatalf("after removing the payment: %s %v %q", got.Status, got.SettledByBillId, got.History[0].Action)
	}
}

func TestArrearsSettlementNeedsMatchingAmount(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("NoMatch")
	p := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	e.setDate("2026-09-20")
	aug := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-08", "dueDate": "2026-09-10", "currentChargesMinor": 300000})
	// Arrears that don't match August's amount (e.g. an older balance) settle nothing.
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 200000, "arrearsMinor": 50000,
		"paidNow": map[string]any{"paidOn": "2026-09-20", "method": "cash"}})
	if got := cl.getBill(aug.Id.String()); got.SettledByBillId != nil || got.Status != apigen.Overdue {
		t.Fatalf("August must stay open: %s %v", got.Status, got.SettledByBillId)
	}

	// Two unpaid months carried together are both settled ("Already paid" on create).
	q := cl.createProfile(map[string]any{"name": "Water", "typeId": "a0000000-0000-4000-8000-000000000003"})
	jul := cl.createBill(map[string]any{"profileId": q.Id, "periodKey": "2026-07", "currentChargesMinor": 100000})
	aug2 := cl.createBill(map[string]any{"profileId": q.Id, "periodKey": "2026-08", "currentChargesMinor": 120000})
	sep := cl.createBill(map[string]any{"profileId": q.Id, "periodKey": "2026-09", "currentChargesMinor": 110000, "arrearsMinor": 220000,
		"paidNow": map[string]any{"paidOn": "2026-09-20", "method": "cash"}})
	for _, id := range []string{jul.Id.String(), aug2.Id.String()} {
		if got := cl.getBill(id); got.SettledByBillId == nil || *got.SettledByBillId != sep.Id {
			t.Errorf("bill %s not settled: %v", id, got.SettledByBillId)
		}
	}
}

func TestPaymentReceipt(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Receipt")
	bill := expenseBill(cl)
	other := cl.createBill(map[string]any{"profileId": cl.defaultProfile().Id, "description": "Other", "expenseDate": "2026-10-01", "currentChargesMinor": 1000})
	receipt := cl.upload(fakeJPEG, "image/jpeg", map[string]any{"billId": bill.Id})
	foreign := cl.upload(fakePDF, "application/pdf", map[string]any{"billId": other.Id})

	if got := cl.do(http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/payments", map[string]any{
		"paidOn": "2026-10-02", "amountMinor": 150000, "method": "cash", "receiptDocumentId": foreign.Id,
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("receipt from another bill: %d, want 400", got)
	}

	var detail apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/payments", map[string]any{
		"paidOn": "2026-10-02", "amountMinor": 150000, "method": "cash", "receiptDocumentId": receipt.Id,
	}, &detail)
	if len(detail.Payments) != 1 || detail.Payments[0].ReceiptDocumentId == nil || *detail.Payments[0].ReceiptDocumentId != receipt.Id {
		t.Fatalf("payments = %+v", detail.Payments)
	}
	for _, d := range detail.Documents {
		if d.Id == receipt.Id && (d.Kind != apigen.DocumentKindReceipt || d.PaymentId == nil) {
			t.Errorf("receipt document = %+v", d)
		}
	}

	// Deleting the payment keeps the document on the bill.
	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/payments/"+detail.Payments[0].Id.String(), nil, nil)
	if docs := cl.getBill(bill.Id.String()).Documents; len(docs) != 1 || docs[0].PaymentId != nil {
		t.Errorf("documents after deleting the payment = %+v", docs)
	}
}
