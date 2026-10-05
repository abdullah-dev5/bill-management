package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

type fakePush struct {
	mu     sync.Mutex
	sent   []pushMessage
	status int
}

func (f *fakePush) Send(_ context.Context, _ store.PushSubscription, payload []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var m pushMessage
	_ = json.Unmarshal(payload, &m)
	f.sent = append(f.sent, m)
	if f.status == 0 {
		return http.StatusCreated, nil
	}
	return f.status, nil
}

func (f *fakePush) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func subscribe(cl *client, endpoint string) {
	cl.t.Helper()
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/push/subscriptions", map[string]any{
		"endpoint": endpoint, "keys": map[string]any{"p256dh": "BNcR-key", "auth": "auth-secret"},
	}, nil)
}

func setPush(cl *client, on bool, quiet ...int) {
	cl.t.Helper()
	body := map[string]any{"leadDays": 3, "dueSoon": true, "overdue": true, "missing": true, "push": on}
	if len(quiet) == 2 {
		body["quietStart"], body["quietEnd"] = quiet[0], quiet[1]
	}
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/notification-settings", body, nil)
}

func TestPushKeyIsStable(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("PushKey")
	var k1, k2 apigen.PushKey
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/push/key", nil, &k1)
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/push/key", nil, &k2)
	raw, err := base64.RawURLEncoding.DecodeString(k1.PublicKey)
	if err != nil || len(raw) != 65 || raw[0] != 4 || k1 != k2 {
		t.Errorf("public key %q (%d bytes, err %v)", k1.PublicKey, len(raw), err)
	}
}

func TestPushReminders(t *testing.T) {
	e := newEnv(t) // 3 Oct 2026, 10:00 UTC = 15:00 in Karachi
	fake := &fakePush{}
	e.server.push = fake
	cl, _ := e.signedUp("Pusher")
	subscribe(cl, "https://push.example.com/send/abc")
	p := cl.createProfile(map[string]any{"name": "Home Electricity", "typeId": typeElectricity})
	backdate(t, p.Id)
	bill := cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "dueDate": "2026-10-05", "currentChargesMinor": 326600})

	// Push is off by default: the reminder is in the inbox but not pushed, and never will be.
	e.tick()
	if fake.count() != 0 || len(cl.notifications().Items) != 1 {
		t.Fatalf("pushed %d with push off", fake.count())
	}
	setPush(cl, true)
	e.tick()
	if fake.count() != 0 {
		t.Errorf("an old reminder was pushed after turning push on")
	}

	// Quiet hours 14:00–18:00 Karachi: the due-today reminder waits…
	setPush(cl, true, 14, 18)
	e.setDate("2026-10-05")
	e.tick()
	if fake.count() != 0 {
		t.Errorf("pushed during quiet hours")
	}
	// …and goes out once they end (19:00 Karachi).
	e.clock.Store(time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC).UnixNano())
	e.tick()
	e.tick()
	if fake.count() != 1 {
		t.Fatalf("pushes = %+v", fake.sent)
	}
	m := fake.sent[0]
	if m.Title != "Home Electricity · September 2026" || m.Body != "Due today · Rs 3,266" || m.URL != "/bills/"+bill.Id.String() {
		t.Errorf("message = %+v", m)
	}

	// A device that has gone away (410) is removed.
	fake.status = http.StatusGone
	e.clock.Store(time.Date(2026, 10, 6, 14, 0, 0, 0, time.UTC).UnixNano()) // overdue, after quiet hours
	e.tick()
	subs, err := testDB.ListPushSubscriptions(context.Background(), store.ListPushSubscriptionsParams{UserID: mustMe(cl).User.Id, HouseholdID: mustMe(cl).Household.Id})
	if err != nil || len(subs) != 0 {
		t.Errorf("subscriptions after 410: %d (%v)", len(subs), err)
	}
}

func TestPushTestAndUnsubscribe(t *testing.T) {
	e := newEnv(t)
	fake := &fakePush{}
	e.server.push = fake
	cl, _ := e.signedUp("PushTest")
	subscribe(cl, "https://push.example.com/send/one")
	subscribe(cl, "https://push.example.com/send/two")
	subscribe(cl, "https://push.example.com/send/two") // same device again
	var res apigen.PushTestResult
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/push/test", nil, &res)
	if res.Devices != 2 || res.Delivered != 2 || fake.sent[0].Body != "Notifications are working on this device." {
		t.Errorf("test = %+v, sent %+v", res, fake.sent)
	}
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/push/unsubscribe", map[string]any{"endpoint": "https://push.example.com/send/one"}, nil)
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/push/test", nil, &res)
	if res.Devices != 1 {
		t.Errorf("after unsubscribe: %+v", res)
	}
	cl.must(http.StatusBadRequest, http.MethodPost, "/api/v1/push/subscriptions",
		map[string]any{"endpoint": "http://insecure.example.com", "keys": map[string]any{"p256dh": "x", "auth": "y"}}, nil)
}

func TestPushHelpers(t *testing.T) {
	for minor, want := range map[int64]string{326600: "Rs 3,266", 4000000: "Rs 40,000", 12345678: "Rs 123,456.78", 50: "Rs 0.50", 100000000: "Rs 1,000,000"} {
		if got := formatRupees(minor); got != want {
			t.Errorf("formatRupees(%d) = %q, want %q", minor, got, want)
		}
	}
	h := func(v int16) *int16 { return &v }
	at := func(hour int) time.Time { return time.Date(2026, 10, 5, hour-5, 0, 0, 0, time.UTC) } // Karachi = UTC+5
	cases := []struct {
		start, end *int16
		hour       int
		want       bool
	}{
		{h(22), h(7), 23, true}, {h(22), h(7), 6, true}, {h(22), h(7), 7, false}, {h(22), h(7), 12, false},
		{h(13), h(15), 14, true}, {h(13), h(15), 15, false}, {nil, nil, 3, false},
	}
	for _, c := range cases {
		if got := inQuietHours(at(c.hour), "Asia/Karachi", c.start, c.end); got != c.want {
			t.Errorf("quiet %v–%v at %d:00 = %v", deref(c.start), deref(c.end), c.hour, got)
		}
	}
}

func mustMe(cl *client) apigen.Me {
	cl.t.Helper()
	var me apigen.Me
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/auth/me", nil, &me)
	return me
}
