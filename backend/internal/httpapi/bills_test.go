package httpapi

import (
	"fmt"
	"math"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

func homeElectricity(cl *client, name string) apigen.Profile {
	return cl.createProfile(map[string]any{
		"name": name, "typeId": typeElectricity, "providerId": providerLESCO,
		"referenceNumber": "04 11132 " + randomDigits(7),
	})
}

var refSeq atomic.Int64

// randomDigits returns a unique n-digit string so reference numbers never collide between tests.
func randomDigits(n int) string {
	return fmt.Sprintf("%0*d", n, refSeq.Add(1)%int64(math.Pow10(n)))
}

func TestProfileRulesPerKind(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Profiles")

	home := homeElectricity(cl, "Home Electricity")
	if home.Kind != apigen.Metered {
		t.Fatalf("kind = %s, want metered (from the type)", home.Kind)
	}
	if home.BillingCycle == nil || *home.BillingCycle != apigen.Monthly {
		t.Fatal("metered profiles default to a monthly cycle")
	}
	if home.MultiplyingFactor == nil || *home.MultiplyingFactor != 1 {
		t.Fatal("multiplying factor defaults to 1")
	}
	if home.ConsumptionUnit == nil || *home.ConsumptionUnit != "kWh" {
		t.Fatal("unit should come from the provider")
	}
	if home.Color != 1 || !home.Pinned {
		t.Fatalf("color %d pinned %v: want first free slot and pinned", home.Color, home.Pinned)
	}

	// Several profiles of the same type are allowed (FR-ACCT-08) and get distinct colours.
	shop := homeElectricity(cl, "Shop Electricity")
	if shop.Color == home.Color {
		t.Fatal("second profile should get a different colour")
	}

	var errBody apigen.Error
	if got := cl.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "home electricity", "typeId": typeElectricity}, &errBody); got != http.StatusConflict || errBody.Code != "profile_name_taken" {
		t.Fatalf("duplicate name: %d %+v", got, errBody)
	}
	if got := cl.do(http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Bad ref", "typeId": typeElectricity, "providerId": providerLESCO, "referenceNumber": "123",
	}, &errBody); got != http.StatusBadRequest || (*errBody.Fields)["referenceNumber"] == "" {
		t.Fatalf("bad LESCO reference: %d %+v", got, errBody)
	}
	if got := cl.do(http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Same ref", "typeId": typeElectricity, "providerId": providerLESCO, "referenceNumber": *home.ReferenceNumber,
	}, &errBody); got != http.StatusConflict || errBody.Code != "reference_taken" {
		t.Fatalf("duplicate reference: %d %+v", got, errBody)
	}

	// Recurring: expected amount + auto-create; expense-log fields are dropped.
	rent := cl.createProfile(map[string]any{
		"name": "Flat Rent", "typeId": typeRent, "expectedAmountMinor": 4000000, "autoCreateBill": true,
		"meterNumber": "ignored for recurring",
	})
	if rent.Kind != apigen.Recurring || rent.ExpectedAmountMinor == nil || *rent.ExpectedAmountMinor != 4000000 || rent.MeterNumber != nil {
		t.Fatalf("rent profile = %+v", rent)
	}

	// The default profile cannot be archived; others can, and restore works.
	def := cl.defaultProfile()
	if got := cl.do(http.MethodPost, "/api/v1/profiles/"+def.Id.String()+"/archive", nil, &errBody); got != http.StatusConflict {
		t.Fatalf("archive default: %d", got)
	}
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/profiles/"+shop.Id.String()+"/archive", nil, nil)
	var active []apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles", nil, &active)
	for _, p := range active {
		if p.Id == shop.Id {
			t.Fatal("archived profile listed")
		}
	}
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/profiles/"+shop.Id.String()+"/restore", nil, nil)
}

func TestBillStatusLifecycle(t *testing.T) {
	e := newEnv(t) // today = 2026-10-03
	cl, _ := e.signedUp("Status")
	p := homeElectricity(cl, "Home Electricity")

	bill := cl.createBill(map[string]any{
		"profileId": p.Id, "periodKey": "2026-09", "issueDate": "2026-09-24", "dueDate": "2026-10-08",
		"currentChargesMinor": 842000, "payableAfterDueMinor": 926200,
	})
	if bill.Status != apigen.Unpaid || bill.OutstandingMinor != 842000 || bill.PayableMinor != 842000 {
		t.Fatalf("new bill: status %s outstanding %d payable %d", bill.Status, bill.OutstandingMinor, bill.PayableMinor)
	}
	if bill.ExpenseDate.Format("2006-01-02") != "2026-09-24" {
		t.Fatalf("expense date should default to the issue date, got %s", bill.ExpenseDate)
	}

	// One bill per profile per month.
	var errBody apigen.Error
	if got := cl.do(http.MethodPost, "/api/v1/bills", map[string]any{
		"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 1,
	}, &errBody); got != http.StatusConflict || errBody.Code != "duplicate_bill" || errBody.ExistingBillId == nil || *errBody.ExistingBillId != bill.Id {
		t.Fatalf("duplicate bill: %d %+v", got, errBody)
	}

	var after apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-10-03", "amountMinor": 400000, "method": "jazzcash"}, &after)
	if after.Status != apigen.PartiallyPaid || after.OutstandingMinor != 442000 {
		t.Fatalf("partial: %s %d", after.Status, after.OutstandingMinor)
	}

	// After the due date the after-due amount applies to what's still owed.
	e.setDate("2026-10-09")
	b := cl.getBill(bill.Id.String())
	if b.Status != apigen.Overdue || b.OutstandingMinor != 526200 {
		t.Fatalf("overdue: %s %d (want 926200-400000)", b.Status, b.OutstandingMinor)
	}

	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-10-09", "amountMinor": 526200, "method": "bank_transfer"}, &after)
	if after.Status != apigen.Paid || after.OutstandingMinor != 0 || after.PaidMinor != 926200 {
		t.Fatalf("paid late in full: %s %d %d", after.Status, after.OutstandingMinor, after.PaidMinor)
	}

	// Deleting a payment re-derives the status.
	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/payments/"+after.Payments[1].Id.String(), nil, nil)
	if b := cl.getBill(bill.Id.String()); b.Status != apigen.Overdue {
		t.Fatalf("after deleting payment: %s", b.Status)
	}
}

func TestPaidOnTimeStaysPaid(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("OnTime")
	p := homeElectricity(cl, "Home Electricity")
	bill := cl.createBill(map[string]any{
		"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-08",
		"currentChargesMinor": 842000, "payableAfterDueMinor": 926200,
		"paidNow": map[string]any{"paidOn": "2026-10-03", "method": "cash"},
	})
	if bill.Status != apigen.Paid || bill.PaidMinor != 842000 {
		t.Fatalf("paid now: %s %d", bill.Status, bill.PaidMinor)
	}
	e.setDate("2026-10-30")
	if b := cl.getBill(bill.Id.String()); b.Status != apigen.Paid || b.OutstandingMinor != 0 {
		t.Fatalf("a bill paid on time must stay paid after the due date: %s %d", b.Status, b.OutstandingMinor)
	}
}

func TestExpenseLogEntries(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Expenses")
	maint := cl.createProfile(map[string]any{"name": "Home Maintenance", "typeId": typeMaintenance})
	if maint.Kind != apigen.ExpenseLog || maint.BillingCycle != nil {
		t.Fatalf("maintenance profile = %+v", maint)
	}

	// Many entries in the same month, with a sub-category.
	for _, amount := range []int{350000, 180000} {
		b := cl.createBill(map[string]any{
			"profileId": maint.Id, "typeId": typeElectrician, "description": "Kitchen wiring",
			"expenseDate": "2026-09-28", "currentChargesMinor": amount,
			"paidNow": map[string]any{"paidOn": "2026-09-28", "method": "cash"},
		})
		if b.Status != apigen.Paid || b.PeriodKey != nil || b.TypeId.String() != typeElectrician {
			t.Fatalf("entry = %+v", b)
		}
	}

	var errBody apigen.Error
	if got := cl.do(http.MethodPost, "/api/v1/bills", map[string]any{"profileId": maint.Id, "currentChargesMinor": 100}, &errBody); got != http.StatusBadRequest || (*errBody.Fields)["description"] == "" {
		t.Fatalf("missing description: %d %+v", got, errBody)
	}
	// A sub-category of another type is rejected.
	if got := cl.do(http.MethodPost, "/api/v1/bills", map[string]any{
		"profileId": cl.defaultProfile().Id, "typeId": typeElectrician, "description": "x", "currentChargesMinor": 100,
	}, &errBody); got != http.StatusBadRequest {
		t.Fatalf("foreign sub-category: %d", got)
	}

	var page apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?profileId="+maint.Id.String(), nil, &page)
	if len(page.Items) != 2 {
		t.Fatalf("entries listed: %d", len(page.Items))
	}
}

func TestAuditHistoryRecordsChanges(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Audit")
	p := homeElectricity(cl, "Home Electricity")
	bill := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-08", "currentChargesMinor": 100000})

	var updated apigen.BillDetail
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/bills/"+bill.Id.String(), map[string]any{
		"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-10", "currentChargesMinor": 100000,
	}, &updated)
	if len(updated.History) < 2 || updated.History[0].Action != "update" {
		t.Fatalf("history = %+v", updated.History)
	}
	change, ok := updated.History[0].Changes["dueDate"].([]any)
	if !ok || change[0] != "2026-10-08" || change[1] != "2026-10-10" {
		t.Fatalf("dueDate change = %#v", updated.History[0].Changes)
	}
	if updated.History[0].ActorName == nil || *updated.History[0].ActorName != "Audit" {
		t.Fatal("actor name missing")
	}
}

func TestHouseholdIsolation(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.signedUp("Alice")
	bob, _ := e.signedUp("Bob")

	p := homeElectricity(alice, "Home Electricity")
	bill := alice.createBill(map[string]any{
		"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 5000,
		"paidNow": map[string]any{"paidOn": "2026-10-01", "method": "cash"},
	})

	if got := bob.do(http.MethodGet, "/api/v1/bills/"+bill.Id.String(), nil, nil); got != http.StatusNotFound {
		t.Fatalf("bob reads alice's bill: %d", got)
	}
	if got := bob.do(http.MethodGet, "/api/v1/profiles/"+p.Id.String(), nil, nil); got != http.StatusNotFound {
		t.Fatalf("bob reads alice's profile: %d", got)
	}
	var page apigen.BillPage
	bob.must(http.StatusOK, http.MethodGet, "/api/v1/bills?profileId="+p.Id.String(), nil, &page)
	if len(page.Items) != 0 {
		t.Fatal("bob sees alice's bills")
	}
	if got := bob.do(http.MethodPost, "/api/v1/bills", map[string]any{"profileId": p.Id, "periodKey": "2026-08", "currentChargesMinor": 1}, nil); got != http.StatusBadRequest {
		t.Fatalf("bob writes into alice's profile: %d", got)
	}
	if got := bob.do(http.MethodDelete, "/api/v1/payments/"+bill.Payments[0].Id.String(), nil, nil); got != http.StatusNotFound {
		t.Fatalf("bob deletes alice's payment: %d", got)
	}
	if got := bob.do(http.MethodPut, "/api/v1/profiles/"+p.Id.String(), map[string]any{"name": "Mine", "typeId": typeElectricity}, nil); got != http.StatusNotFound {
		t.Fatalf("bob edits alice's profile: %d", got)
	}
}

func TestArchiveRestoreDelete(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Archive")
	p := homeElectricity(cl, "Home Electricity")
	bill := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 1000})
	id := bill.Id.String()

	if got := cl.do(http.MethodDelete, "/api/v1/bills/"+id, nil, nil); got != http.StatusConflict {
		t.Fatalf("delete before archive: %d", got)
	}
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/bills/"+id+"/archive", nil, nil)

	var page apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills", nil, &page)
	if len(page.Items) != 0 {
		t.Fatal("archived bill listed")
	}
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?archived=true", nil, &page)
	if len(page.Items) != 1 {
		t.Fatal("archived bill missing from the archive")
	}

	// Archiving frees the month; restoring then conflicts with the replacement.
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 2000})
	if got := cl.do(http.MethodPost, "/api/v1/bills/"+id+"/restore", nil, nil); got != http.StatusConflict {
		t.Fatalf("restore over replacement: %d", got)
	}

	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/bills/"+id, nil, nil)
	if got := cl.do(http.MethodGet, "/api/v1/bills/"+id, nil, nil); got != http.StatusNotFound {
		t.Fatalf("deleted bill: %d", got)
	}
}

func TestHomeSpendExcludesArrears(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Home")
	p := homeElectricity(cl, "Home Electricity")

	// FR-ANLT-06: August Rs 7,000 unpaid; September Rs 8,000 + Rs 7,000 arrears = Rs 15,000 payable.
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-08", "dueDate": "2026-09-08", "currentChargesMinor": 700000})
	sep := cl.createBill(map[string]any{
		"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-08",
		"currentChargesMinor": 800000, "arrearsMinor": 700000,
	})
	if sep.PayableMinor != 1500000 {
		t.Fatalf("payable defaults to charges + arrears: %d", sep.PayableMinor)
	}

	var aug, sepHome apigen.HomeSummary
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home?month=2026-08", nil, &aug)
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home?month=2026-09", nil, &sepHome)
	if aug.BilledMinor+sepHome.BilledMinor != 1500000 {
		t.Fatalf("Aug+Sep spend = %d, want 1500000 (arrears not double counted)", aug.BilledMinor+sepHome.BilledMinor)
	}
	if len(sepHome.DueSoon) != 1 || len(sepHome.Overdue) != 1 {
		t.Fatalf("due soon %d overdue %d, want 1 and 1", len(sepHome.DueSoon), len(sepHome.Overdue))
	}
	if len(sepHome.Pinned) != 1 || sepHome.Pinned[0].Summary.LatestBill == nil || sepHome.Pinned[0].Summary.LatestBill.PeriodKey == nil || *sepHome.Pinned[0].Summary.LatestBill.PeriodKey != "2026-09" {
		t.Fatalf("pinned profile summary = %+v", sepHome.Pinned)
	}
}

func TestListBillsPagination(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Pages")
	def := cl.defaultProfile()
	for _, d := range []string{"2026-09-01", "2026-09-02", "2026-09-03"} {
		cl.createBill(map[string]any{"profileId": def.Id, "description": "Item " + d, "expenseDate": d, "currentChargesMinor": 100})
	}
	var first, second apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?limit=2", nil, &first)
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first page: %d items, cursor %v", len(first.Items), first.NextCursor)
	}
	if first.Items[0].ExpenseDate.Format("2006-01-02") != "2026-09-03" {
		t.Fatal("newest first")
	}
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?limit=2&cursor="+*first.NextCursor, nil, &second)
	if len(second.Items) != 1 || second.NextCursor != nil {
		t.Fatalf("second page: %d items", len(second.Items))
	}

	var errBody apigen.Error
	if got := cl.do(http.MethodGet, "/api/v1/bills?status=bogus", nil, &errBody); got != http.StatusBadRequest {
		t.Fatalf("invalid status filter: %d", got)
	}
}
