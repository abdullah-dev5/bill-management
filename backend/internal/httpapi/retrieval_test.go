package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
	"github.com/abdullah-dev5/bill-management/backend/internal/retrieval"
)

const providerHESCO = "b0000000-0000-4000-8000-000000000007"

// brokenConnector fails (or panics) every time, for health tests.
type brokenConnector struct{ panics bool }

func (brokenConnector) ID() string          { return "broken" }
func (brokenConnector) Name() string        { return "Broken" }
func (brokenConnector) Providers() []string { return []string{"HESCO"} }
func (b brokenConnector) Fetch(context.Context, retrieval.Request) (retrieval.Document, error) {
	if b.panics {
		panic("boom")
	}
	return retrieval.Document{}, errors.New("site changed its layout")
}

func hescoProfile(cl *client, extra map[string]any) apigen.Profile {
	body := map[string]any{"name": "HESCO Electricity", "typeId": typeElectricity, "providerId": providerHESCO, "referenceNumber": retrieval.SampleReference}
	for k, v := range extra {
		body[k] = v
	}
	return cl.createProfile(body)
}

func fetchNow(cl *client, profileID string, want int) apigen.RetrievalResult {
	cl.t.Helper()
	var out apigen.RetrievalResult
	cl.must(want, http.MethodPost, "/api/v1/profiles/"+profileID+"/retrieve", nil, &out)
	return out
}

func TestFetchNowCreatesOfficialBill(t *testing.T) {
	e := newEnv(t)
	e.server.retrieval = retrieval.NewRegistry(100, retrieval.Sample{})
	cl, _ := e.signedUp("Retrieve")
	p := hescoProfile(cl, nil)
	if p.Retrieval == nil || p.Retrieval.Mode != apigen.Automatic || p.Retrieval.Enabled {
		t.Fatalf("retrieval info = %+v", p.Retrieval)
	}

	res := fetchNow(cl, p.Id.String(), http.StatusOK)
	if res.Outcome != apigen.RetrievalResultOutcomeCreated || res.BillId == nil {
		t.Fatalf("result = %+v", res)
	}
	b := cl.getBill(res.BillId.String())
	if *b.PeriodKey != "2026-09" || b.PayableMinor != 326600 || b.Status != apigen.BillStatusDraft || len(b.Documents) != 1 ||
		len(b.Sources) != 1 || b.Sources[0].Kind != apigen.BillOriginKindOfficial || *b.Sources[0].Connector != "sample" {
		t.Fatalf("bill = period %v payable %d status %s docs %d sources %+v", b.PeriodKey, b.PayableMinor, b.Status, len(b.Documents), b.Sources)
	}
	if k := kinds(cl.notifications()); k[apigen.NotificationKindBillRetrieved] != 1 {
		t.Errorf("notifications = %v", k)
	}

	// Throttled for a while, then the same document is recognised.
	fetchNow(cl, p.Id.String(), http.StatusTooManyRequests)
	e.clock.Store(e.clock.Load() + int64(20*time.Minute))
	if res := fetchNow(cl, p.Id.String(), http.StatusOK); res.Outcome != apigen.RetrievalResultOutcomeUnchanged {
		t.Errorf("second fetch = %+v", res)
	}
	var got apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String(), nil, &got)
	if got.Retrieval.LastOutcome == nil || *got.Retrieval.LastOutcome != apigen.ProfileRetrievalLastOutcomeUnchanged || got.Retrieval.LastAt == nil {
		t.Errorf("profile retrieval = %+v", got.Retrieval)
	}

	// A reference the source doesn't know: not found, not a failure.
	other := cl.createProfile(map[string]any{"name": "Shop meter", "typeId": typeElectricity, "providerId": providerHESCO, "referenceNumber": "09123450099999"})
	if res := fetchNow(cl, other.Id.String(), http.StatusOK); res.Outcome != apigen.RetrievalResultOutcomeNotFound {
		t.Errorf("unknown reference = %+v", res)
	}
}

// M6 exit criterion: a retrieved bill and a scanned bill of the same month become one record.
func TestRetrievedAndScannedBillMerge(t *testing.T) {
	e := newEnv(t)
	e.server.retrieval = retrieval.NewRegistry(100, retrieval.Sample{})
	cl, _ := e.signedUp("RetrieveMerge")
	p := hescoProfile(cl, nil)
	scanned := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-05", "currentChargesMinor": 330000})

	res := fetchNow(cl, p.Id.String(), http.StatusOK)
	if res.Outcome != apigen.RetrievalResultOutcomeMerged || *res.BillId != scanned.Id {
		t.Fatalf("result = %+v", res)
	}
	b := cl.getBill(scanned.Id.String())
	var page apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?profileId="+p.Id.String(), nil, &page)
	if len(page.Items) != 1 || len(b.Sources) != 2 || b.Sources[1].Kind != apigen.BillOriginKindOfficial || b.PayableMinor != 330000 {
		t.Fatalf("bills %d, sources %+v, payable %d", len(page.Items), b.Sources, b.PayableMinor)
	}
	if len(openConflicts(b)) == 0 || len(b.Documents) != 1 {
		t.Errorf("conflicts %+v, documents %d", b.Conflicts, len(b.Documents))
	}
}

// M6 exit criterion: a broken connector affects nothing else, and switches itself off.
func TestBrokenConnectorIsContained(t *testing.T) {
	e := newEnv(t)
	e.server.retrieval = retrieval.NewRegistry(100, brokenConnector{})
	cl, _ := e.signedUp("Broken")
	p := hescoProfile(cl, nil)
	for i := 0; i < connectorFailureThreshold; i++ {
		if res := fetchNow(cl, p.Id.String(), http.StatusOK); res.Outcome != apigen.RetrievalResultOutcomeFailed {
			t.Fatalf("attempt %d = %+v", i, res)
		}
		e.clock.Store(e.clock.Load() + int64(20*time.Minute))
	}
	// Paused now: no request is made, and the profile says so.
	if res := fetchNow(cl, p.Id.String(), http.StatusOK); res.Outcome != apigen.RetrievalResultOutcomeSkipped {
		t.Errorf("after threshold = %+v", res)
	}
	var got apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String(), nil, &got)
	if got.Retrieval.ConnectorPaused == nil || !*got.Retrieval.ConnectorPaused {
		t.Errorf("profile should show the connector paused: %+v", got.Retrieval)
	}
	var conns []apigen.ConnectorStatus
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/retrieval/connectors", nil, &conns)
	if len(conns) != 1 || conns[0].Healthy || conns[0].DisabledUntil == nil {
		t.Errorf("connectors = %+v", conns)
	}
	// Everything else works: bills, home, the tick.
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 100000})
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home", nil, nil)
	e.tick()

	// A connector that panics fails the attempt, not the server.
	e.server.retrieval = retrieval.NewRegistry(100, brokenConnector{panics: true})
	q := hescoProfile(cl, map[string]any{"name": "Second meter", "referenceNumber": "09123450012399"})
	if res := fetchNow(cl, q.Id.String(), http.StatusOK); res.Outcome != apigen.RetrievalResultOutcomeFailed && res.Outcome != apigen.RetrievalResultOutcomeSkipped {
		t.Errorf("panicking connector = %+v", res)
	}
}

func TestAssistedProvidersAndOfficialUploads(t *testing.T) {
	e := newEnv(t)
	e.server.retrieval = retrieval.NewRegistry(100) // no automatic connectors, like production
	cl, _ := e.signedUp("Assisted")
	p := cl.createProfile(map[string]any{"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO, "referenceNumber": "09 12345 0012345"})
	if p.Retrieval.Mode != apigen.Assisted || p.Retrieval.LookupUrl == nil || *p.Retrieval.LookupUrl != "https://bill.pitc.com.pk/lescobill" {
		t.Fatalf("retrieval = %+v", p.Retrieval)
	}
	fetchNow(cl, p.Id.String(), http.StatusConflict)

	// The person saves the bill from the official page and uploads it as an official copy.
	doc := cl.upload(extracttest.TextPDF([]string{"official bill"}, 1), "application/pdf", nil)
	var b apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/documents/"+doc.Id.String()+"/bill", map[string]any{
		"bill": map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 326600}, "officialCopy": true,
	}, &b)
	if len(b.Sources) != 1 || b.Sources[0].Kind != apigen.BillOriginKindOfficial {
		t.Errorf("sources = %+v", b.Sources)
	}
	plain := cl.createProfile(map[string]any{"name": "Rent", "typeId": typeRent})
	if plain.Retrieval.Mode != apigen.None {
		t.Errorf("no provider = %+v", plain.Retrieval)
	}
}

func TestScheduledRetrieval(t *testing.T) {
	e := newEnv(t) // 3 Oct 2026
	e.server.retrieval = retrieval.NewRegistry(100, retrieval.Sample{})
	cl, _ := e.signedUp("Scheduled")
	hescoProfile(cl, map[string]any{"name": "Not opted in", "referenceNumber": "09123450099999"})
	on := hescoProfile(cl, map[string]any{"name": "Opted in", "retrievalEnabled": true})
	if !on.Retrieval.Enabled {
		t.Fatalf("opt-in not saved: %+v", on.Retrieval)
	}
	e.tick()
	var page apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills", nil, &page)
	if len(page.Items) != 1 || page.Items[0].ProfileId != on.Id {
		t.Fatalf("after tick: %d bills", len(page.Items))
	}
	// Not again the same day.
	var runs int
	count := func() int {
		if err := testDB.Pool.QueryRow(context.Background(), "SELECT count(*) FROM retrieval_runs WHERE profile_id = $1", on.Id).Scan(&runs); err != nil {
			t.Fatal(err)
		}
		return runs
	}
	e.tick()
	if n := count(); n != 1 {
		t.Errorf("runs after two ticks the same day = %d", n)
	}
	e.setDate("2026-10-05") // more than a day later
	e.tick()
	if n := count(); n != 2 {
		t.Errorf("runs the next day = %d", n)
	}
}

func TestToggleRetrieval(t *testing.T) {
	e := newEnv(t)
	e.server.retrieval = retrieval.NewRegistry(100, retrieval.Sample{})
	cl, _ := e.signedUp("Toggle")
	p := hescoProfile(cl, nil)
	var got apigen.Profile
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/profiles/"+p.Id.String()+"/retrieval", map[string]any{"enabled": true}, &got)
	if !got.Retrieval.Enabled {
		t.Errorf("not enabled: %+v", got.Retrieval)
	}
	lesco := cl.createProfile(map[string]any{"name": "LESCO home", "typeId": typeElectricity, "providerId": providerLESCO})
	cl.must(http.StatusConflict, http.MethodPut, "/api/v1/profiles/"+lesco.Id.String()+"/retrieval", map[string]any{"enabled": true}, nil)
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/profiles/"+lesco.Id.String()+"/retrieval", map[string]any{"enabled": false}, nil)
}
