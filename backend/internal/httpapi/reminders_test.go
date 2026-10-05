package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

func (e *env) tick() {
	e.t.Helper()
	if _, err := e.server.RunTick(context.Background()); err != nil {
		e.t.Fatal(err)
	}
}

func (cl *client) notifications() apigen.NotificationList {
	cl.t.Helper()
	var out apigen.NotificationList
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/notifications", nil, &out)
	return out
}

func kinds(list apigen.NotificationList) map[apigen.NotificationKind]int {
	out := map[apigen.NotificationKind]int{}
	for _, n := range list.Items {
		out[n.Kind]++
	}
	return out
}

// backdate makes a profile look older than the test clock (created_at comes from the database clock).
func backdate(t *testing.T, profileID uuid.UUID) {
	t.Helper()
	if _, err := testDB.Pool.Exec(context.Background(), "UPDATE bill_profiles SET created_at = '2026-01-01' WHERE id = $1", profileID); err != nil {
		t.Fatal(err)
	}
}

func TestDueReminders(t *testing.T) {
	e := newEnv(t) // 3 Oct 2026
	cl, _ := e.signedUp("Reminders")
	p := cl.createProfile(map[string]any{"name": "Home Electricity", "typeId": typeElectricity})
	backdate(t, p.Id)
	bill := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-05", "currentChargesMinor": 326600})
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-10", "dueDate": "2026-11-05", "currentChargesMinor": 300000}) // too far ahead

	e.tick()
	e.tick() // idempotent
	list := cl.notifications()
	if k := kinds(list); k[apigen.NotificationKindDueSoon] != 1 || len(list.Items) != 1 || list.UnreadCount != 1 {
		t.Fatalf("after first ticks: %+v", list)
	}
	n := list.Items[0]
	if *n.BillId != bill.Id || *n.ProfileName != "Home Electricity" || *n.AmountMinor != 326600 || n.DueDate.String() != "2026-10-05" || *n.PeriodKey != "2026-09" {
		t.Errorf("notification = %+v", n)
	}

	e.setDate("2026-10-05")
	e.tick()
	e.setDate("2026-10-06")
	e.tick()
	e.tick()
	if k := kinds(cl.notifications()); k[apigen.NotificationKindDueToday] != 1 || k[apigen.NotificationKindOverdue] != 1 {
		t.Errorf("kinds = %v", k)
	}

	// Mark one read, then all.
	list = cl.notifications()
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/notifications/read", map[string]any{"ids": []string{list.Items[0].Id.String()}}, nil)
	if got := cl.notifications().UnreadCount; got != 2 {
		t.Errorf("unread after one = %d", got)
	}
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/notifications/read", map[string]any{"all": true}, nil)
	if got := cl.notifications(); got.UnreadCount != 0 || !got.Items[0].Read {
		t.Errorf("after all: %+v", got)
	}
	cl.must(http.StatusBadRequest, http.MethodPost, "/api/v1/notifications/read", map[string]any{}, nil)

	// Paying stops further reminders; another household's member sees nothing.
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+bill.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-10-06", "amountMinor": 338900, "method": "cash"}, nil)
	before := len(cl.notifications().Items)
	e.setDate("2026-10-12")
	e.tick()
	if after := len(cl.notifications().Items); after != before {
		t.Errorf("reminded about a paid bill: %d → %d", before, after)
	}
	other, _ := e.signedUp("RemindersOther")
	if got := other.notifications(); len(got.Items) != 0 {
		t.Errorf("leaked: %+v", got)
	}
}

func TestReminderSettings(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("ReminderPrefs")
	var s apigen.NotificationSettings
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/notification-settings", nil, &s)
	if s.LeadDays != 3 || !s.DueSoon || !s.Overdue || !s.Missing || s.Push {
		t.Errorf("defaults = %+v", s)
	}
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/notification-settings",
		map[string]any{"leadDays": 7, "dueSoon": true, "overdue": false, "missing": true, "push": false, "quietStart": 22, "quietEnd": 7}, &s)
	if s.LeadDays != 7 || s.Overdue || *s.QuietStart != 22 || *s.QuietEnd != 7 {
		t.Errorf("saved = %+v", s)
	}
	cl.must(http.StatusBadRequest, http.MethodPut, "/api/v1/notification-settings",
		map[string]any{"leadDays": 30, "dueSoon": true, "overdue": true, "missing": true, "push": false}, nil)
	cl.must(http.StatusBadRequest, http.MethodPut, "/api/v1/notification-settings",
		map[string]any{"leadDays": 3, "dueSoon": true, "overdue": true, "missing": true, "push": false, "quietStart": 22}, nil)

	// Lead days apply: a bill due in 6 days is "due soon" with 7 lead days; overdue is off.
	p := cl.createProfile(map[string]any{"name": "Gas", "typeId": typeGas})
	backdate(t, p.Id)
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-09", "currentChargesMinor": 90000})
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-08", "dueDate": "2026-09-09", "currentChargesMinor": 90000})
	e.tick()
	if k := kinds(cl.notifications()); k[apigen.NotificationKindDueSoon] != 1 || k[apigen.NotificationKindOverdue] != 0 {
		t.Errorf("kinds = %v", k)
	}
}

func TestExpectedMissingBills(t *testing.T) {
	e := newEnv(t) // 3 Oct 2026
	cl, _ := e.signedUp("Missing")
	gas := cl.createProfile(map[string]any{"name": "Kitchen Gas", "typeId": typeGas})
	backdate(t, gas.Id)
	cl.createBill(map[string]any{"profileId": gas.Id, "periodKey": "2026-08", "currentChargesMinor": 90000,
		"paidNow": map[string]any{"paidOn": "2026-09-01", "method": "cash"}})
	fresh := cl.createProfile(map[string]any{"name": "New Water", "typeId": "a0000000-0000-4000-8000-000000000003", "typicalIssueDay": 1})
	// Made on 3 Oct, after September's deadline: not missing until October's (6 Oct) has passed.
	if _, err := testDB.Pool.Exec(context.Background(), "UPDATE bill_profiles SET created_at = '2026-10-03 09:00Z' WHERE id = $1", fresh.Id); err != nil {
		t.Fatal(err)
	}

	// September's gas bill was expected by 15 Sep (10th + 5 days' grace).
	var home apigen.HomeSummary
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home", nil, &home)
	if len(home.ExpectedMissing) != 1 || home.ExpectedMissing[0].ProfileName != "Kitchen Gas" || home.ExpectedMissing[0].PeriodKey != "2026-09" {
		t.Fatalf("expected missing = %+v", home.ExpectedMissing)
	}
	e.tick()
	e.tick()
	if k := kinds(cl.notifications()); k[apigen.NotificationKindMissingBill] != 1 {
		t.Errorf("kinds = %v", k)
	}

	// Adding September's bill clears it; on 20 Oct, October's is missing.
	cl.createBill(map[string]any{"profileId": gas.Id, "periodKey": "2026-09", "currentChargesMinor": 90000})
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home", nil, &home)
	if len(home.ExpectedMissing) != 0 {
		t.Errorf("still missing: %+v", home.ExpectedMissing)
	}
	e.setDate("2026-10-20")
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home", nil, &home)
	if len(home.ExpectedMissing) != 2 || home.ExpectedMissing[0].PeriodKey != "2026-10" || home.ExpectedMissing[1].ProfileName != "New Water" {
		t.Errorf("on 20 Oct: %+v", home.ExpectedMissing)
	}
}

func TestRecurringBillsCreateThemselves(t *testing.T) {
	e := newEnv(t)
	e.setDate("2026-10-01")
	cl, _ := e.signedUp("Recurring")
	rent := cl.createProfile(map[string]any{
		"name": "Flat Rent", "typeId": typeRent, "kind": "recurring", "expectedAmountMinor": 4000000, "autoCreateBill": true,
		"typicalIssueDay": 1, "typicalDueDay": 5,
	})
	e.tick()
	e.tick() // once per period
	var page apigen.BillPage
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?profileId="+rent.Id.String(), nil, &page)
	if len(page.Items) != 1 {
		t.Fatalf("bills = %+v", page.Items)
	}
	b := page.Items[0]
	if *b.PeriodKey != "2026-10" || b.PayableMinor != 4000000 || b.DueDate.String() != "2026-10-05" || b.Status != apigen.BillStatusUnpaid {
		t.Errorf("bill = %+v", b)
	}
	detail := cl.getBill(b.Id.String())
	if len(detail.History) == 0 || detail.History[len(detail.History)-1].Action != "create" {
		t.Errorf("history = %+v", detail.History)
	}
	if k := kinds(cl.notifications()); k[apigen.NotificationKindBillCreated] != 1 {
		t.Errorf("kinds = %v", k)
	}
	// It is never reported as missing.
	var home apigen.HomeSummary
	e.setDate("2026-10-25")
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/home", nil, &home)
	if len(home.ExpectedMissing) != 0 {
		t.Errorf("auto-created profile reported missing: %+v", home.ExpectedMissing)
	}
	// November's arrives on its own.
	e.setDate("2026-11-01")
	e.tick()
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills?profileId="+rent.Id.String(), nil, &page)
	if len(page.Items) != 2 || *page.Items[0].PeriodKey != "2026-11" {
		t.Errorf("after November tick: %d bills", len(page.Items))
	}
}

func TestCyclePeriods(t *testing.T) {
	q := store.BillingCycleQuarterly
	today := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		last  string
		cycle *store.BillingCycle
		want  []string
	}{
		{"2026-08", nil, []string{"2026-09", "2026-10"}},
		{"2026-10", nil, nil},
		{"2026-07", &q, []string{"2026-10"}},
		{"2026-08", &q, nil},
	}
	for _, c := range cases {
		got := cyclePeriods(store.ListCycleProfilesRow{LastPeriod: c.last, BillingCycle: c.cycle}, today, false)
		if len(got) != len(c.want) || (len(got) > 0 && got[len(got)-1] != c.want[len(c.want)-1]) {
			t.Errorf("last %s cycle %v → %v, want %v", c.last, c.cycle, got, c.want)
		}
	}
}
