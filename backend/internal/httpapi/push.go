package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// pushSender delivers one Web Push message; tests swap in a fake.
type pushSender interface {
	Send(ctx context.Context, sub store.PushSubscription, payload []byte) (status int, err error)
}

type webPushSender struct {
	publicKey, privateKey, subject string
}

func (w webPushSender) Send(ctx context.Context, sub store.PushSubscription, payload []byte) (int, error) {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber: w.subject, VAPIDPublicKey: w.publicKey, VAPIDPrivateKey: w.privateKey,
		TTL: 24 * 3600, Urgency: webpush.UrgencyNormal,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

type pushMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

const (
	pushMaxAge      = 24 * time.Hour // older reminders are never pushed
	pushMaxFailures = 5              // a device that keeps failing is dropped
)

// sendPushes pushes new reminders to the devices of people who turned push on (FR-REM-03),
// outside their quiet hours. Each reminder is pushed at most once.
func (s *Server) sendPushes(ctx context.Context) (int, error) {
	now := s.now()
	since := now.Add(-pushMaxAge)
	if err := s.db.SkipOldUnpushedNotifications(ctx, store.SkipOldUnpushedNotificationsParams{Now: &now, Since: since}); err != nil {
		return 0, err
	}
	rows, err := s.db.ListUnpushedNotifications(ctx, since)
	if err != nil {
		return 0, err
	}
	subsCache := map[string][]store.PushSubscription{}
	sent := 0
	for _, n := range rows {
		if !n.Push {
			if err := s.db.MarkNotificationPushed(ctx, store.MarkNotificationPushedParams{ID: n.ID, PushedAt: &now}); err != nil {
				return sent, err
			}
			continue
		}
		if inQuietHours(now, n.Timezone, n.QuietStart, n.QuietEnd) {
			continue // try again after quiet hours (while it's still fresh)
		}
		key := n.UserID.String() + n.HouseholdID.String()
		subs, ok := subsCache[key]
		if !ok {
			if subs, err = s.db.ListPushSubscriptions(ctx, store.ListPushSubscriptionsParams{UserID: n.UserID, HouseholdID: n.HouseholdID}); err != nil {
				return sent, err
			}
			subsCache[key] = subs
		}
		payload, _ := json.Marshal(pushText(n, s.today(n.Timezone)))
		for _, sub := range subs {
			if s.deliver(ctx, sub, payload) {
				sent++
			}
		}
		if err := s.db.MarkNotificationPushed(ctx, store.MarkNotificationPushedParams{ID: n.ID, PushedAt: &now}); err != nil {
			return sent, err
		}
	}
	return sent, nil
}

// deliver sends to one device and keeps the subscription list healthy.
func (s *Server) deliver(ctx context.Context, sub store.PushSubscription, payload []byte) bool {
	status, err := s.push.Send(ctx, sub, payload)
	switch {
	case err == nil && status >= 200 && status < 300:
		_ = s.db.PushSubscriptionUsed(ctx, store.PushSubscriptionUsedParams{ID: sub.ID, LastUsedAt: ptr(s.now())})
		return true
	case status == http.StatusNotFound || status == http.StatusGone:
		// The browser unsubscribed or the subscription expired.
		_ = s.db.DeletePushSubscriptionByID(ctx, sub.ID)
	default:
		failures, ferr := s.db.PushSubscriptionFailed(ctx, sub.ID)
		if ferr == nil && failures >= pushMaxFailures {
			_ = s.db.DeletePushSubscriptionByID(ctx, sub.ID)
		}
		s.log.WarnContext(ctx, "push failed", "subscription_id", sub.ID, "status", status, "err", err)
	}
	return false
}

func inQuietHours(now time.Time, tz string, start, end *int16) bool {
	if start == nil || end == nil || *start == *end {
		return false
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	h := int16(now.In(loc).Hour())
	if *start < *end {
		return h >= *start && h < *end
	}
	return h >= *start || h < *end // overnight, e.g. 22:00–07:00
}

// pushText is the same wording as the in-app inbox.
func pushText(n store.ListUnpushedNotificationsRow, today time.Time) pushMessage {
	what := deref(n.BillDescription)
	if n.PeriodKey != nil {
		what = monthName(*n.PeriodKey)
	}
	title := strings.Trim(deref(n.ProfileName)+" · "+what, " ·")
	if title == "" {
		title = "Bill Manager"
	}
	amount := ""
	if n.AmountMinor != nil {
		amount = formatRupees(*n.AmountMinor)
	}
	day := ""
	if n.DueDate != nil {
		day = n.DueDate.Format("02 Jan")
	}
	msg := pushMessage{Title: title, URL: "/notifications", Tag: string(n.Kind) + ":" + n.ID.String()}
	if n.BillID != nil {
		msg.URL = "/bills/" + n.BillID.String()
	}
	switch n.Kind {
	case store.NotificationKindDueSoon:
		days := 1
		if n.DueDate != nil {
			days = max(1, int(n.DueDate.Sub(today).Hours()/24))
		}
		if days == 1 {
			msg.Body = fmt.Sprintf("Due tomorrow (%s) · %s", day, amount)
		} else {
			msg.Body = fmt.Sprintf("Due in %d days (%s) · %s", days, day, amount)
		}
	case store.NotificationKindDueToday:
		msg.Body = "Due today · " + amount
	case store.NotificationKindOverdue:
		msg.Body = fmt.Sprintf("Overdue since %s · %s to pay", day, amount)
	case store.NotificationKindMissingBill:
		msg.Body = fmt.Sprintf("The %s bill hasn’t been added yet. Add it or scan it.", what)
		if n.ProfileID != nil && n.PeriodKey != nil {
			msg.URL = "/bills/new?profileId=" + n.ProfileID.String() + "&period=" + *n.PeriodKey
		}
	case store.NotificationKindBillCreated:
		msg.Body = "Added automatically · " + amount
		if day != "" {
			msg.Body += ", due " + day
		}
	}
	return msg
}

func monthName(period string) string {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return period
	}
	return t.Format("January 2006")
}

// formatRupees writes "Rs 3,266" (paisa only when there are any: "Rs 3,266.50").
func formatRupees(minor int64) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	whole := strconv.FormatInt(minor/100, 10)
	var b bytes.Buffer
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := sign + "Rs " + b.String()
	if p := minor % 100; p != 0 {
		out += fmt.Sprintf(".%02d", p)
	}
	return out
}

// --- Endpoints ------------------------------------------------------------------------

func (s *Server) GetPushKey(w http.ResponseWriter, r *http.Request) {
	pub, _ := s.keys.VAPID()
	writeJSON(w, http.StatusOK, apigen.PushKey{PublicKey: pub})
}

func (s *Server) SubscribePush(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.PushSubscriptionInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	if !strings.HasPrefix(in.Endpoint, "https://") || len(in.Endpoint) > 2000 || in.Keys.P256dh == "" || in.Keys.Auth == "" {
		s.fail(w, r, errField("endpoint", "This doesn't look like a push subscription."))
		return
	}
	ua := r.UserAgent()
	if len(ua) > 300 {
		ua = ua[:300]
	}
	if err := s.db.UpsertPushSubscription(r.Context(), store.UpsertPushSubscriptionParams{
		ID: uuid.Must(uuid.NewV7()), UserID: a.UserID, HouseholdID: a.HouseholdID, Endpoint: in.Endpoint,
		P256dh: in.Keys.P256dh, Auth: in.Keys.Auth, UserAgent: &ua,
	}); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) UnsubscribePush(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.UnsubscribePushJSONRequestBody
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := s.db.DeletePushSubscription(r.Context(), store.DeletePushSubscriptionParams{Endpoint: in.Endpoint, UserID: a.UserID}); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestPush sends a sample notification to the caller's devices.
func (s *Server) TestPush(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	ctx := r.Context()
	subs, err := s.db.ListPushSubscriptions(ctx, store.ListPushSubscriptionsParams{UserID: a.UserID, HouseholdID: a.HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	payload, _ := json.Marshal(pushMessage{
		Title: "Bill Manager", Body: "Notifications are working on this device.", URL: "/notifications", Tag: "test",
	})
	sent := 0
	for _, sub := range subs {
		if s.deliver(ctx, sub, payload) {
			sent++
		}
	}
	writeJSON(w, http.StatusOK, apigen.PushTestResult{Devices: len(subs), Delivered: sent})
}
