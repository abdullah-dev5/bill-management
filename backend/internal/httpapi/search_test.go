package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

func (cl *client) search(q string) apigen.SearchResult {
	cl.t.Helper()
	var out apigen.SearchResult
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/search?q="+url.QueryEscape(q), nil, &out)
	return out
}

func periods(bills []apigen.Bill) []string {
	var out []string
	for _, b := range bills {
		if b.PeriodKey != nil {
			out = append(out, *b.PeriodKey)
		} else {
			out = append(out, b.ExpenseDate.String())
		}
	}
	return out
}

func TestSearchPhrasesReferencesAndText(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Search")
	elec := cl.createProfile(map[string]any{
		"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO, "referenceNumber": "09 12345 0012345",
	})
	gas := cl.createProfile(map[string]any{"name": "Kitchen Gas", "typeId": typeGas})
	for _, m := range []string{"2025-09", "2025-10", "2026-09"} {
		cl.createBill(map[string]any{"profileId": elec.Id, "periodKey": m, "currentChargesMinor": 300000})
	}
	cl.createBill(map[string]any{"profileId": gas.Id, "periodKey": "2025-12", "currentChargesMinor": 90000, "billNumber": "SG-7781"})
	cl.createBill(map[string]any{"profileId": gas.Id, "periodKey": "2026-01", "currentChargesMinor": 120000})
	plumber := cl.createBill(map[string]any{
		"profileId": cl.defaultProfile().Id, "description": "Plumber", "expenseDate": "2026-08-14", "currentChargesMinor": 250000,
		"notes": "Kitchen sink leak",
	})

	// FR-SRCH-02: a natural phrase becomes type + month.
	res := cl.search("electricity september 2025")
	if got := periods(res.Bills); len(got) != 1 || got[0] != "2025-09" {
		t.Errorf("electricity september 2025 → %v", got)
	}
	if res.Interpretation.From == nil || *res.Interpretation.From != "2025-09" || len(res.Interpretation.TypeNames) != 1 {
		t.Errorf("interpretation = %+v", res.Interpretation)
	}

	// Last winter (Nov 2025 – Feb 2026 on 3 Oct 2026).
	if got := periods(cl.search("gas last winter").Bills); len(got) != 2 {
		t.Errorf("gas last winter → %v", got)
	}

	// Provider short name.
	if got := cl.search("lesco"); len(got.Bills) != 3 || len(got.Profiles) != 1 {
		t.Errorf("lesco → %d bills, %d profiles", len(got.Bills), len(got.Profiles))
	}

	// The reference number, typed any way, finds the profile and its bills (matched by keyed hash).
	res = cl.search("0912345-0012345")
	if len(res.Profiles) != 1 || res.Profiles[0].Id != elec.Id || len(res.Bills) != 3 {
		t.Errorf("reference → profiles %+v, %d bills", res.Profiles, len(res.Bills))
	}
	if res.Profiles[0].ReferenceDisplay == nil || *res.Profiles[0].ReferenceDisplay != "•••• 2345" {
		t.Errorf("reference shown as %v", res.Profiles[0].ReferenceDisplay)
	}

	// Typed with spaces, as printed on the bill.
	if res := cl.search("09 12345 0012345 september 2025"); len(res.Profiles) != 1 || len(res.Bills) != 1 {
		t.Errorf("spaced reference + month → %d profiles, %v", len(res.Profiles), periods(res.Bills))
	}

	// Bill number, notes and description.
	if got := cl.search("sg-7781").Bills; len(got) != 1 {
		t.Errorf("bill number → %d", len(got))
	}
	if got := cl.search("sink leak").Bills; len(got) != 1 || got[0].Id != plumber.Id {
		t.Errorf("notes → %v", periods(got))
	}
	// Words must all match; "kitchen" alone matches the gas profile name and the plumber's notes.
	if got := cl.search("kitchen").Bills; len(got) != 3 {
		t.Errorf("kitchen → %v", periods(got))
	}

	// Status words.
	if got := cl.search("unpaid electricity").Bills; len(got) != 3 {
		t.Errorf("unpaid electricity → %v", periods(got))
	}

	// Wildcards are literal.
	if got := cl.search("100%").Bills; len(got) != 0 {
		t.Errorf("100%% → %v", periods(got))
	}
}

func TestSearchDocumentText(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("SearchDocs")
	bill := expenseBill(cl)
	doc := cl.upload(fakeJPEG, "image/jpeg", map[string]any{"billId": bill.Id, "fileName": "receipt.jpg"})
	if _, err := testDB.Pool.Exec(context.Background(),
		"UPDATE documents SET ocr_text = $2 WHERE id = $1", doc.Id, "Invoice from Ali Electric Works\nReplaced kitchen DB breaker 32A\nTotal 1,500"); err != nil {
		t.Fatal(err)
	}
	res := cl.search("breaker")
	if len(res.Documents) != 1 || res.Documents[0].Snippet == nil || len(res.Bills) != 1 || res.Bills[0].Id != bill.Id {
		t.Fatalf("documents %+v, bills %d", res.Documents, len(res.Bills))
	}
	if s := *res.Documents[0].Snippet; s == "" || len(s) > 140 {
		t.Errorf("snippet %q", s)
	}

	// Another household can't find it.
	other, _ := e.signedUp("Other")
	if res := other.search("breaker"); len(res.Documents) != 0 || len(res.Bills) != 0 {
		t.Errorf("leaked across households: %+v", res)
	}
}

func TestBillFilters(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("BillFilters")
	elec := cl.createProfile(map[string]any{"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO})
	gas := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	cl.createBill(map[string]any{"profileId": elec.Id, "periodKey": "2026-08", "currentChargesMinor": 500000})
	scanned := cl.createBill(map[string]any{"profileId": elec.Id, "periodKey": "2026-09", "currentChargesMinor": 300000})
	cl.upload(fakeJPEG, "image/jpeg", map[string]any{"billId": scanned.Id}) // a camera photo
	cl.createBill(map[string]any{"profileId": gas.Id, "periodKey": "2026-09", "currentChargesMinor": 80000})
	upkeep := cl.createProfile(map[string]any{"name": "Repairs", "typeId": typeMaintenance})
	electrician := cl.createBill(map[string]any{
		"profileId": upkeep.Id, "typeId": typeElectrician, "description": "Wiring", "expenseDate": "2026-09-10", "currentChargesMinor": 200000,
	})

	list := func(query string) []apigen.Bill {
		t.Helper()
		var page apigen.BillPage
		cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?"+query, nil, &page)
		return page.Items
	}
	if got := list("typeId=" + typeElectricity); len(got) != 2 {
		t.Errorf("electricity → %d", len(got))
	}
	// A type includes its sub-categories.
	if got := list("typeId=" + typeMaintenance); len(got) != 1 || got[0].Id != electrician.Id {
		t.Errorf("maintenance → %d", len(got))
	}
	if got := list("typeId=" + typeElectricity + "&typeId=" + typeGas); len(got) != 3 {
		t.Errorf("electricity+gas → %d", len(got))
	}
	if got := list("providerId=" + providerLESCO); len(got) != 2 {
		t.Errorf("LESCO → %d", len(got))
	}
	if got := list("minAmount=100000&maxAmount=300000"); len(got) != 2 {
		t.Errorf("Rs 1,000–3,000 → %d", len(got))
	}
	if got := list("source=camera"); len(got) != 1 || got[0].Id != scanned.Id {
		t.Errorf("camera → %d", len(got))
	}
	if got := list("source=manual"); len(got) != 3 {
		t.Errorf("manual → %d", len(got))
	}
	cl.must(http.StatusBadRequest, http.MethodGet, "/api/v1/bills?minAmount=5&maxAmount=1", nil, nil)
}

func TestActivity(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Activity")
	p := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	b := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 80000})
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+b.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-10-01", "amountMinor": 80000, "method": "jazzcash"}, nil)

	var items []apigen.ActivityItem
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/activity", nil, &items)
	if len(items) < 3 {
		t.Fatalf("activity = %+v", items)
	}
	pay, add := items[0], items[1]
	if pay.Action != apigen.PaymentAdded || *pay.AmountMinor != 80000 || *pay.Detail != "jazzcash" || pay.ProfileName != "Gas" {
		t.Errorf("payment item = %+v", pay)
	}
	if add.Action != apigen.BillAdded || *add.AmountMinor != 80000 || *add.PeriodKey != "2026-09" || add.BillId == nil {
		t.Errorf("bill item = %+v", add)
	}
	if items[2].Action != apigen.ProfileAdded || items[2].ActorName == nil {
		t.Errorf("profile item = %+v", items[2])
	}
}
