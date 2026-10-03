package httpapi

import (
	"net/http"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

func TestMeterReadings(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Readings")
	p := cl.createProfile(map[string]any{"name": "Electricity", "typeId": typeElectricity})

	// Units are worked out from the readings (MF 1).
	b := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 326600,
		"previousReading": 18690, "currentReading": 18854})
	if b.Units == nil || *b.Units != 164 || b.MultiplyingFactor == nil || *b.MultiplyingFactor != 1 {
		t.Fatalf("units %v, MF %v", b.Units, b.MultiplyingFactor)
	}

	// The next bill can start from this bill's present reading.
	var prof apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String(), nil, &prof)
	if prof.Summary.LastReading == nil || *prof.Summary.LastReading != 18854 {
		t.Fatalf("last reading = %v", prof.Summary.LastReading)
	}

	// The history shows the bill's units.
	var months []apigen.HistoryMonth
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String()+"/history", nil, &months)
	if len(months) != 1 || months[0].Units == nil || *months[0].Units != 164 {
		t.Fatalf("history = %+v", months)
	}

	bad := func(name string, body map[string]any, field string) {
		t.Helper()
		body["profileId"], body["periodKey"], body["currentChargesMinor"] = p.Id, "2026-10", 100000
		var apiErr apigen.Error
		if got := cl.do(http.MethodPost, "/api/v1/bills", body, &apiErr); got != http.StatusBadRequest || apiErr.Fields == nil || (*apiErr.Fields)[field] == "" {
			t.Errorf("%s: %d %+v", name, got, apiErr)
		}
	}
	bad("lower reading", map[string]any{"previousReading": 18854, "currentReading": 100}, "currentReading")
	bad("one reading", map[string]any{"previousReading": 18854}, "currentReading")
	bad("units contradict readings", map[string]any{"previousReading": 18854, "currentReading": 19000, "units": 200}, "units")
	bad("replaced without units", map[string]any{"previousReading": 18854, "currentReading": 50, "meterReplaced": true}, "units")

	// A replaced meter restarts: the printed units are used.
	r := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-10", "currentChargesMinor": 100000,
		"previousReading": 18854, "currentReading": 60, "meterReplaced": true, "units": 155})
	if !r.MeterReplaced || *r.Units != 155 {
		t.Errorf("replaced meter: %v %v", r.MeterReplaced, r.Units)
	}
	// Printed units within 1 of the readings are accepted (rounding on the bill).
	ok := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-11", "currentChargesMinor": 100000,
		"previousReading": 60, "currentReading": 210.4, "units": 150})
	if *ok.Units != 150 {
		t.Errorf("printed units kept: %v", ok.Units)
	}
}

func TestMultiplyingFactorAndOtherKinds(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Factor")
	shop := cl.createProfile(map[string]any{"name": "Shop meter", "typeId": typeElectricity, "multiplyingFactor": 2.5})
	b := cl.createBill(map[string]any{"profileId": shop.Id, "periodKey": "2026-09", "currentChargesMinor": 100000,
		"previousReading": 100, "currentReading": 140})
	if *b.Units != 100 || *b.MultiplyingFactor != 2.5 {
		t.Errorf("MF 2.5: units %v, MF %v", *b.Units, *b.MultiplyingFactor)
	}

	// Readings don't apply to recurring bills such as rent.
	rent := cl.createProfile(map[string]any{"name": "Rent", "typeId": typeRent})
	rb := cl.createBill(map[string]any{"profileId": rent.Id, "periodKey": "2026-09", "currentChargesMinor": 4500000,
		"previousReading": 1, "currentReading": 2})
	if rb.PreviousReading != nil || rb.Units != nil {
		t.Errorf("rent readings should be ignored: %v %v", rb.PreviousReading, rb.Units)
	}
}
