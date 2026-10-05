package httpapi

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

var scanSeq atomic.Int64

// scanInto saves a scanned photo with the given values, attaching it to the bill of the same month.
func scanInto(cl *client, profileID string, bill map[string]any) apigen.BillDetail {
	cl.t.Helper()
	// Each scan is a different photo (identical files are deduplicated).
	photo := append(append([]byte{}, fakeJPEG...), []byte(fmt.Sprintf("scan-%d", scanSeq.Add(1)))...)
	doc := cl.upload(photo, "image/jpeg", nil)
	body := map[string]any{"bill": map[string]any{"profileId": profileID}, "duplicate": "attach"}
	for k, v := range bill {
		body["bill"].(map[string]any)[k] = v
	}
	var out apigen.BillDetail
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/documents/"+doc.Id.String()+"/bill", body, &out)
	return out
}

func openConflicts(b apigen.BillDetail) []apigen.BillConflict {
	var out []apigen.BillConflict
	for _, c := range b.Conflicts {
		if c.Status == apigen.Open {
			out = append(out, c)
		}
	}
	return out
}

func TestScanMergesIntoExistingBill(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Reconcile")
	p := cl.createProfile(map[string]any{"name": "Home Electricity", "typeId": typeElectricity})
	bill := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-05", "currentChargesMinor": 326600})
	if len(bill.Sources) != 1 || bill.Sources[0].Kind != apigen.BillOriginKindManual || bill.Sources[0].Values["payableMinor"] != "326600" {
		t.Fatalf("sources on create = %+v", bill.Sources)
	}

	// The scan agrees on the due date, adds the issue date, and disagrees on the amount.
	merged := scanInto(cl, p.Id.String(), map[string]any{
		"periodKey": "2026-09", "issueDate": "2026-09-18", "dueDate": "2026-10-05", "currentChargesMinor": 330000,
	})
	if merged.Id != bill.Id || merged.PayableMinor != 326600 || merged.IssueDate == nil || merged.IssueDate.String() != "2026-09-18" {
		t.Errorf("bill after merge: payable %d, issue %v", merged.PayableMinor, merged.IssueDate)
	}
	if len(merged.Sources) != 2 || merged.Sources[1].Kind != apigen.BillOriginKindScan || !merged.Sources[1].Merged ||
		!*merged.Sources[1].CanSeparate || len(merged.Sources[1].FilledFields) != 1 || merged.Sources[1].FilledFields[0] != "issueDate" {
		t.Fatalf("sources = %+v", merged.Sources)
	}
	open := openConflicts(merged)
	if len(open) != 2 { // current charges and payable
		t.Fatalf("conflicts = %+v", merged.Conflicts)
	}
	byField := map[string]apigen.BillConflict{}
	for _, c := range open {
		byField[c.Field] = c
	}
	pay := byField["payableMinor"]
	if pay.BillValue != "326600" || pay.SourceValue != "330000" || pay.SourceKind != apigen.BillOriginKindScan {
		t.Errorf("payable conflict = %+v", pay)
	}

	// Needs review: on Home and as a bill filter.
	var home apigen.HomeSummary
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home", nil, &home)
	var page apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?needsReview=true", nil, &page)
	if home.ReviewCount != 1 || len(page.Items) != 1 {
		t.Errorf("review count %d, filtered %d", home.ReviewCount, len(page.Items))
	}

	// Resolve: take the scan's amounts.
	base := "/api/v1/bills/" + bill.Id.String() + "/conflicts/"
	var after apigen.BillDetail
	cl.must(http.StatusOK, http.MethodPost, base+pay.Id.String()+"/resolve", map[string]any{"action": "source"}, &after)
	cl.must(http.StatusOK, http.MethodPost, base+byField["currentChargesMinor"].Id.String()+"/resolve", map[string]any{"action": "revised"}, &after)
	if after.PayableMinor != 330000 || after.CurrentChargesMinor != 330000 || len(openConflicts(after)) != 0 {
		t.Errorf("after resolving: payable %d current %d open %d", after.PayableMinor, after.CurrentChargesMinor, len(openConflicts(after)))
	}
	if h := after.History[0]; h.Action != "conflict_resolved" {
		t.Errorf("latest history = %+v", h)
	}
	cl.must(http.StatusConflict, http.MethodPost, base+pay.Id.String()+"/resolve", map[string]any{"action": "keep"}, nil)
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home", nil, &home)
	if home.ReviewCount != 0 {
		t.Errorf("review count after resolving = %d", home.ReviewCount)
	}

	// Another household can't touch it.
	other, _ := e.signedUp("ReconcileOther")
	other.must(http.StatusNotFound, http.MethodPost, base+pay.Id.String()+"/resolve", map[string]any{"action": "keep"}, nil)
}

func TestConflictKeepManualAndConfirm(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("ReconcileKeep")
	p := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	bill := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-11", "currentChargesMinor": 273800})

	b := scanInto(cl, p.Id.String(), map[string]any{"periodKey": "2026-09", "dueDate": "2026-10-12", "currentChargesMinor": 273800})
	due := openConflicts(b)
	if len(due) != 1 || due[0].Field != "dueDate" {
		t.Fatalf("conflicts = %+v", b.Conflicts)
	}
	path := "/api/v1/bills/" + bill.Id.String() + "/conflicts/" + due[0].Id.String() + "/resolve"
	cl.must(http.StatusBadRequest, http.MethodPost, path, map[string]any{"action": "manual", "value": "next tuesday"}, nil)
	cl.must(http.StatusBadRequest, http.MethodPost, path, map[string]any{"action": "manual"}, nil)
	var after apigen.BillDetail
	cl.must(http.StatusOK, http.MethodPost, path, map[string]any{"action": "manual", "value": "2026-10-13"}, &after)
	if after.DueDate.String() != "2026-10-13" {
		t.Errorf("due date = %v", after.DueDate)
	}

	// A new disagreement, then a third source that agrees with the bill settles it on its own.
	b = scanInto(cl, p.Id.String(), map[string]any{"periodKey": "2026-09", "dueDate": "2026-10-20", "currentChargesMinor": 273800})
	if len(openConflicts(b)) != 1 {
		t.Fatalf("open = %+v", b.Conflicts)
	}
	b = scanInto(cl, p.Id.String(), map[string]any{"periodKey": "2026-09", "dueDate": "2026-10-13", "currentChargesMinor": 273800})
	if len(openConflicts(b)) != 0 {
		t.Errorf("an agreeing source should settle the open conflict: %+v", b.Conflicts)
	}
	confirmed := false
	for _, c := range b.Conflicts {
		confirmed = confirmed || (c.Resolution != nil && *c.Resolution == apigen.BillConflictResolutionConfirmed)
	}
	if !confirmed {
		t.Errorf("no confirmed resolution: %+v", b.Conflicts)
	}
}

func TestSeparateUndoesMerge(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Separate")
	p := cl.createProfile(map[string]any{"name": "Water", "typeId": "a0000000-0000-4000-8000-000000000003"})
	bill := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 120000})
	merged := scanInto(cl, p.Id.String(), map[string]any{"periodKey": "2026-09", "issueDate": "2026-09-20", "dueDate": "2026-10-10", "currentChargesMinor": 150000})
	src := merged.Sources[1]
	if len(merged.Documents) != 1 || len(openConflicts(merged)) != 2 {
		t.Fatalf("merged = %d docs, %d conflicts", len(merged.Documents), len(openConflicts(merged)))
	}

	var res struct {
		Bill       apigen.BillDetail `json:"bill"`
		DocumentID *string           `json:"documentId"`
	}
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/sources/"+src.Id.String()+"/separate", nil, &res)
	b := res.Bill
	if b.IssueDate != nil || b.DueDate != nil || len(b.Sources) != 1 || len(b.Conflicts) != 0 || len(b.Documents) != 0 || res.DocumentID == nil {
		t.Errorf("after separating: issue %v due %v sources %d conflicts %d docs %d doc %v",
			b.IssueDate, b.DueDate, len(b.Sources), len(b.Conflicts), len(b.Documents), res.DocumentID)
	}
	if b.History[0].Action != "merge_undone" {
		t.Errorf("history = %+v", b.History[0])
	}
	// The original source can't be separated from its own bill.
	cl.must(http.StatusConflict, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/sources/"+b.Sources[0].Id.String()+"/separate", nil, nil)

	// After 30 days a merge stays.
	again := scanInto(cl, p.Id.String(), map[string]any{"periodKey": "2026-09", "currentChargesMinor": 120000})
	e.setDate("2026-10-25")
	cl.getBill(bill.Id.String()) // using the session keeps it alive
	e.setDate("2026-11-03")      // the merge is now 31 days old
	cl.must(http.StatusConflict, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/sources/"+again.Sources[1].Id.String()+"/separate", nil, nil)
}
