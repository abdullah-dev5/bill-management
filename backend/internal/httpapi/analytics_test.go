package httpapi

import (
	"net/http"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

func (cl *client) analytics(query string) apigen.Analytics {
	cl.t.Helper()
	var out apigen.Analytics
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/analytics?"+query, nil, &out)
	return out
}

func findSeries(list []apigen.AnalyticsSeries, name string) *apigen.AnalyticsSeries {
	for i := range list {
		if list[i].Name == name {
			return &list[i]
		}
	}
	return nil
}

// FR-ANLT-06 acceptance (docs/02): arrears are never counted twice.
func TestAnalyticsSpendRule(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Spend")
	p := cl.createProfile(map[string]any{"name": "Electricity", "typeId": typeElectricity})
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-08", "dueDate": "2026-09-10", "currentChargesMinor": 700000})
	sep := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-08",
		"currentChargesMinor": 800000, "arrearsMinor": 700000})

	got := cl.analytics("from=2026-08&to=2026-09")
	if got.Total.TotalMinor != 1500000 {
		t.Fatalf("spend Aug–Sep = %d, want 1500000 (not 2200000)", got.Total.TotalMinor)
	}
	if got.Total.Months[0] != 700000 || got.Total.Months[1] != 800000 || len(got.Months) != 2 {
		t.Fatalf("months = %v %v", got.Months, got.Total.Months)
	}

	// Paying September in full (arrears included) doesn't change spending; cash paid shows the payment.
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+sep.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-10-03", "amountMinor": 1500000, "method": "cash"}, nil)
	if got := cl.analytics("from=2026-08&to=2026-09"); got.Total.TotalMinor != 1500000 {
		t.Errorf("spend after paying = %d", got.Total.TotalMinor)
	}
	cash := cl.analytics("from=2026-10&to=2026-10&basis=cash")
	if cash.Total.TotalMinor != 1500000 || cash.Basis != apigen.AnalyticsBasisCash {
		t.Errorf("cash paid in October = %d (%s)", cash.Total.TotalMinor, cash.Basis)
	}
}

func TestAnalyticsLateFeesCount(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("LateFee")
	p := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	e.setDate("2026-10-20")
	b := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-05",
		"currentChargesMinor": 100000, "payableAfterDueMinor": 110000})
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+b.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-10-15", "amountMinor": 110000, "method": "cash"}, nil)
	if got := cl.analytics("from=2026-09&to=2026-09"); got.Total.TotalMinor != 110000 {
		t.Errorf("spend with the late fee paid = %d, want 110000", got.Total.TotalMinor)
	}
}

func TestAnalyticsBreakdowns(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Breakdown")
	home := cl.createProfile(map[string]any{"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO})
	shop := cl.createProfile(map[string]any{"name": "Shop Electricity", "typeId": typeElectricity})
	maint := cl.createProfile(map[string]any{"name": "Home Maintenance", "typeId": typeMaintenance})
	rent := cl.createProfile(map[string]any{"name": "Flat Rent", "typeId": typeRent})

	// Last year's July, this year's June (previous period) and July–August (the range).
	cl.createBill(map[string]any{"profileId": home.Id, "periodKey": "2025-07", "currentChargesMinor": 500000, "previousReading": 1000, "currentReading": 1300})
	cl.createBill(map[string]any{"profileId": home.Id, "periodKey": "2026-06", "currentChargesMinor": 400000})
	jul := cl.createBill(map[string]any{"profileId": home.Id, "periodKey": "2026-07", "currentChargesMinor": 600000, "previousReading": 2000, "currentReading": 2350})
	cl.createBill(map[string]any{"profileId": home.Id, "periodKey": "2026-08", "currentChargesMinor": 300000, "previousReading": 2350, "currentReading": 2500})
	cl.createBill(map[string]any{"profileId": shop.Id, "periodKey": "2026-08", "currentChargesMinor": 200000})
	cl.createBill(map[string]any{"profileId": rent.Id, "periodKey": "2026-07", "currentChargesMinor": 4500000})
	// A maintenance sub-category rolls up into Maintenance.
	cl.createBill(map[string]any{"profileId": maint.Id, "typeId": typeElectrician, "description": "Wiring", "expenseDate": "2026-08-12", "currentChargesMinor": 350000})

	got := cl.analytics("from=2026-07&to=2026-08")
	if got.Total.TotalMinor != 5950000 || got.Total.PreviousMinor != 400000 || got.Total.LastYearMinor != 500000 {
		t.Fatalf("total %d, previous %d, last year %d", got.Total.TotalMinor, got.Total.PreviousMinor, got.Total.LastYearMinor)
	}

	if len(got.Groups) != 2 || got.Groups[0].Key != "utilities" || got.Groups[0].TotalMinor != 1100000 || got.Groups[1].Key != "household" {
		t.Errorf("groups = %+v", got.Groups)
	}
	if m := findSeries(got.Types, "Maintenance"); m == nil || m.TotalMinor != 350000 {
		t.Errorf("maintenance type = %+v", m)
	}
	if el := findSeries(got.Types, "Electricity"); el == nil || el.TotalMinor != 1100000 || el.Months[0] != 600000 || el.Months[1] != 500000 {
		t.Errorf("electricity type = %+v", el)
	}
	if l := findSeries(got.Providers, "LESCO"); l == nil || l.TotalMinor != 900000 {
		t.Errorf("LESCO = %+v", l)
	}
	if none := findSeries(got.Providers, "No provider"); none == nil || none.TotalMinor != 5050000 {
		t.Errorf("no provider = %+v", none)
	}
	if got.Profiles[0].Name != "Flat Rent" {
		t.Errorf("profiles should be ordered by total: first %q", got.Profiles[0].Name)
	}

	h := findSeries(got.Profiles, "Home Electricity")
	if h == nil || h.BillCount != 2 || *h.AvgMinor != 450000 || *h.MaxMinor != 600000 || *h.MinMinor != 300000 || *h.MaxBillId != jul.Id {
		t.Fatalf("home electricity stats = %+v", h)
	}
	if h.PreviousMinor != 400000 || h.LastYearMinor != 500000 {
		t.Errorf("home electricity comparisons: previous %d, last year %d", h.PreviousMinor, h.LastYearMinor)
	}
	if h.UnitsTotal == nil || *h.UnitsTotal != 500 || *h.UnitsLastYear != 300 || (*h.UnitsMonths)[0] == nil || *(*h.UnitsMonths)[0] != 350 {
		t.Errorf("home electricity units = total %v, last year %v", h.UnitsTotal, h.UnitsLastYear)
	}
}

func TestAnalyticsUsesPrintedUnitsAndValidates(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Printed")
	p := electricityProfile(cl)
	doc := cl.upload(hescoPDF(), "application/pdf", nil)
	ex := cl.extract(doc.Id.String())
	history := []map[string]any{}
	for _, h := range ex.History {
		history = append(history, map[string]any{"month": h.Month, "units": h.Units, "billedMinor": h.BilledMinor, "paidMinor": h.PaidMinor})
	}
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/documents/"+doc.Id.String()+"/bill",
		map[string]any{"bill": scannedBill(p.Id, map[string]any{"previousReading": 18690, "currentReading": 18854}), "history": history}, nil)

	// Winter 2025–26: no bills, but the history printed on the September bill has the units.
	got := cl.analytics("from=2025-11&to=2026-02")
	prof := findSeries(got.Profiles, p.Name)
	if prof != nil {
		t.Fatalf("a profile with no bills in the range shouldn't appear: %+v", prof)
	}
	got = cl.analytics("from=2026-08&to=2026-09")
	prof = findSeries(got.Profiles, p.Name)
	if prof == nil || prof.UnitsTotal == nil || *prof.UnitsTotal != 164+161 {
		t.Fatalf("units with printed August = %+v", prof)
	}

	for name, q := range map[string]string{
		"end before start": "from=2026-09&to=2026-08",
		"too long":         "from=2020-01&to=2026-09",
		"bad month":        "from=2026-13&to=2026-14",
	} {
		if got := cl.do(http.MethodGet, "/api/v1/analytics?"+q, nil, nil); got != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", name, got)
		}
	}

	other, _ := e.signedUp("OtherHousehold")
	if got := other.analytics("from=2026-08&to=2026-09"); got.Total.TotalMinor != 0 || len(got.Profiles) != 0 {
		t.Errorf("another household sees %d", got.Total.TotalMinor)
	}
}
