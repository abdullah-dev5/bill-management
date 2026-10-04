package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
)

func TestSteppedLateFees(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Steps")
	p := electricityProfile(cl)
	step := map[string]any{
		"profileId": p.Id, "periodKey": "2026-09", "issueDate": "2026-09-18", "dueDate": "2026-10-05",
		"currentChargesMinor": 326600, "payableAfterDueMinor": 338900,
		"lateStepUntil": "2026-10-08", "payableFinalMinor": 351200,
	}
	bill := cl.createBill(step)
	id := bill.Id.String()
	if bill.LateStepUntil == nil || bill.PayableFinalMinor == nil || *bill.PayableFinalMinor != 351200 {
		t.Fatalf("late step not stored: %+v", bill)
	}

	for _, tc := range []struct {
		date        string
		status      apigen.BillStatus
		outstanding int64
	}{
		{"2026-10-05", apigen.Unpaid, 326600},
		{"2026-10-07", apigen.Overdue, 338900},
		{"2026-10-09", apigen.Overdue, 351200},
	} {
		e.setDate(tc.date)
		got := cl.getBill(id)
		if got.Status != tc.status || got.OutstandingMinor != tc.outstanding {
			t.Errorf("%s: status %s outstanding %d, want %s %d", tc.date, got.Status, got.OutstandingMinor, tc.status, tc.outstanding)
		}
	}

	// Paying the first late amount before the step date settles the bill for good.
	e.setDate("2026-10-07")
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+id+"/payments", map[string]any{"paidOn": "2026-10-07", "amountMinor": 338900, "method": "cash"}, nil)
	e.setDate("2026-10-20")
	if got := cl.getBill(id); got.Status != apigen.Paid || got.OutstandingMinor != 0 {
		t.Errorf("paid within the step: %s %d", got.Status, got.OutstandingMinor)
	}

	// Paying the same amount after the step date leaves the increase outstanding.
	step["periodKey"] = "2026-08"
	late := cl.createBill(step)
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+late.Id.String()+"/payments", map[string]any{"paidOn": "2026-10-10", "amountMinor": 338900, "method": "cash"}, nil)
	if got := cl.getBill(late.Id.String()); got.Status != apigen.Overdue || got.OutstandingMinor != 12300 {
		t.Errorf("paid after the step: %s %d", got.Status, got.OutstandingMinor)
	}

	// "Already paid" after the step pays the final amount.
	step["periodKey"] = "2026-07"
	step["paidNow"] = map[string]any{"paidOn": "2026-10-12", "method": "cash"}
	if got := cl.createBill(step); got.PaidMinor != 351200 || got.Status != apigen.Paid {
		t.Errorf("already paid after the step: paid %d status %s", got.PaidMinor, got.Status)
	}
}

func TestSteppedLateFeeValidation(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("StepRules")
	p := electricityProfile(cl)
	base := func(extra map[string]any) map[string]any {
		b := map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-05", "currentChargesMinor": 100000, "payableAfterDueMinor": 110000}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	for name, body := range map[string]map[string]any{
		"date without amount": base(map[string]any{"lateStepUntil": "2026-10-08"}),
		"lower final amount":  base(map[string]any{"lateStepUntil": "2026-10-08", "payableFinalMinor": 105000}),
		"step before due":     base(map[string]any{"lateStepUntil": "2026-10-01", "payableFinalMinor": 120000}),
	} {
		if got := cl.do(http.MethodPost, "/api/v1/bills", body, nil); got != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", name, got)
		}
	}
}

// electricityProfile creates a HESCO profile whose reference matches the masked sample bill.
func electricityProfile(cl *client) apigen.Profile {
	cl.t.Helper()
	return cl.createProfile(map[string]any{
		"name": "Home Electricity " + randomDigits(4), "typeId": typeElectricity,
		"providerId": "b0000000-0000-4000-8000-000000000007", "referenceNumber": "09123450012345",
	})
}

func scannedBill(profileID any, extra map[string]any) map[string]any {
	bill := map[string]any{
		"profileId": profileID, "periodKey": "2026-09", "issueDate": "2026-09-18", "dueDate": "2026-10-05",
		"currentChargesMinor": 326600, "payableAfterDueMinor": 338900, "lateStepUntil": "2026-10-08", "payableFinalMinor": 351200,
	}
	for k, v := range extra {
		bill[k] = v
	}
	return bill
}

func TestSaveScannedBill(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Scan")
	p := electricityProfile(cl)
	doc := cl.upload(hescoPDF(), "application/pdf", map[string]any{"fileName": "hesco.pdf", "source": "upload"})
	ex := cl.extract(doc.Id.String())

	history := []map[string]any{}
	for _, h := range ex.History {
		history = append(history, map[string]any{"month": h.Month, "units": h.Units, "billedMinor": h.BilledMinor, "paidMinor": h.PaidMinor})
	}
	var bill apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/documents/"+doc.Id.String()+"/bill", map[string]any{
		// The user corrected the billing month on the verification screen.
		"bill": scannedBill(p.Id, nil), "extractionId": ex.ID, "history": history,
	}, &bill)
	if bill.Status != apigen.Unpaid || len(bill.Documents) != 1 || bill.Documents[0].Id != doc.Id {
		t.Fatalf("saved bill: status %s, documents %+v", bill.Status, bill.Documents)
	}

	var months []apigen.HistoryMonth
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String()+"/history", nil, &months)
	if len(months) != 13 || months[0].Month != "2025-09" || months[12].Month != "2026-09" {
		t.Fatalf("history months = %d (%v … %v)", len(months), months[0].Month, months[len(months)-1].Month)
	}
	if last := months[12]; last.Source != apigen.HistoryMonthSourceBill || last.BillId == nil || *last.BillId != bill.Id {
		t.Errorf("current month should come from the bill: %+v", last)
	}
	if aug := months[11]; aug.Source != apigen.HistoryMonthSourcePrinted || *aug.BilledMinor != 272700 || *aug.Units != 161 {
		t.Errorf("Aug 26 printed row = %+v", aug)
	}

	var n int
	var refExtracted string
	if err := testDB.Pool.QueryRow(context.Background(),
		"SELECT count(*), coalesce(max(extracted) FILTER (WHERE field = 'referenceNumber'), '') FROM extraction_corrections WHERE extraction_id = $1", ex.ID).Scan(&n, &refExtracted); err != nil {
		t.Fatal(err)
	}
	if n < 5 || refExtracted != "•••• 2345" {
		t.Errorf("corrections: %d rows, reference %q", n, refExtracted)
	}

	if got := cl.do(http.MethodPost, "/api/v1/documents/"+doc.Id.String()+"/bill", map[string]any{"bill": scannedBill(p.Id, nil)}, nil); got != http.StatusConflict {
		t.Errorf("saving an attached document again: %d, want 409", got)
	}
}

func TestSaveScannedBillDuplicates(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Dupes")
	p := electricityProfile(cl)
	first := cl.upload(hescoPDF(), "application/pdf", nil)
	var original apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/documents/"+first.Id.String()+"/bill", map[string]any{"bill": scannedBill(p.Id, nil)}, &original)

	// The same bill photographed later: exact duplicate (same month).
	photo := cl.upload(fakeJPEG, "image/jpeg", nil)
	var apiErr apigen.Error
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+photo.Id.String()+"/bill", map[string]any{"bill": scannedBill(p.Id, nil)}, &apiErr); got != http.StatusConflict || apiErr.Code != "duplicate_bill" || apiErr.ExistingBillId == nil || *apiErr.ExistingBillId != original.Id {
		t.Fatalf("exact duplicate: %d %+v", got, apiErr)
	}
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+photo.Id.String()+"/bill", map[string]any{"bill": scannedBill(p.Id, nil), "duplicate": "keep_separate"}, nil); got != http.StatusConflict {
		t.Errorf("an exact duplicate can't be kept separate: %d", got)
	}
	var attached apigen.BillDetail
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/documents/"+photo.Id.String()+"/bill", map[string]any{"bill": scannedBill(p.Id, nil), "duplicate": "attach"}, &attached)
	if attached.Id != original.Id || len(attached.Documents) != 2 {
		t.Fatalf("attach: bill %s with %d documents", attached.Id, len(attached.Documents))
	}

	// A different month but issued within days of the existing bill: probably the same bill.
	other := cl.upload(extracttest.TextPDF([]string{"another scan"}, 1), "application/pdf", nil)
	probable := scannedBill(p.Id, map[string]any{"periodKey": "2026-10", "issueDate": "2026-09-20", "dueDate": "2026-10-12", "lateStepUntil": "2026-10-15"})
	if got := cl.do(http.MethodPost, "/api/v1/documents/"+other.Id.String()+"/bill", map[string]any{"bill": probable}, &apiErr); got != http.StatusConflict || apiErr.Code != "probable_duplicate" {
		t.Fatalf("probable duplicate: %d %+v", got, apiErr)
	}
	var separate apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/documents/"+other.Id.String()+"/bill", map[string]any{"bill": probable, "duplicate": "keep_separate"}, &separate)
	if separate.Id == original.Id || derefStr(separate.PeriodKey) != "2026-10" {
		t.Fatalf("keep separate: %+v", separate.Id)
	}

	intruder, _ := e.signedUp("Intruder")
	if got := intruder.do(http.MethodPost, "/api/v1/documents/"+other.Id.String()+"/bill", map[string]any{"bill": scannedBill(p.Id, nil)}, nil); got != http.StatusNotFound {
		t.Errorf("another household: %d, want 404", got)
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
