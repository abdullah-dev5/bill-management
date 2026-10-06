package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// Expected-bill rules (FR-REM-04): a bill is "missing" once its typical issue day (default the
// 10th) plus a grace period has passed with no bill for that cycle.
const (
	defaultIssueDay = 10
	missingGrace    = 5
	reminderHorizon = 14 // days ahead that can be reminded about (max lead days)
)

func cycleMonths(c *store.BillingCycle) int {
	if c == nil {
		return 1
	}
	switch *c {
	case store.BillingCycleBimonthly:
		return 2
	case store.BillingCycleQuarterly:
		return 3
	case store.BillingCycleYearly:
		return 12
	}
	return 1
}

func monthIndex(key string) int {
	t, err := time.Parse("2006-01", key)
	if err != nil {
		return -1
	}
	return t.Year()*12 + int(t.Month()) - 1
}

func monthKeyOf(idx int) string { return fmt.Sprintf("%04d-%02d", idx/12, idx%12+1) }

// dayIn returns the given day of a month (YYYY-MM), clamped to the month's length.
func dayIn(period string, day int) time.Time {
	t, _ := time.Parse("2006-01", period)
	last := t.AddDate(0, 1, -1).Day()
	if day > last {
		day = last
	}
	if day < 1 {
		day = 1
	}
	return time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, time.UTC)
}

// cyclePeriods lists the billing periods, on the profile's cycle, that should have a bill by
// now: those after its latest bill, up to the current month. Without any bill the current
// month counts when allowNoHistory is set.
func cyclePeriods(p store.ListCycleProfilesRow, today time.Time, allowNoHistory bool) []string {
	cur := today.Year()*12 + int(today.Month()) - 1
	if p.LastPeriod == "" {
		if !allowNoHistory {
			return nil
		}
		return []string{monthKeyOf(cur)}
	}
	last := monthIndex(p.LastPeriod)
	if last < 0 {
		return nil
	}
	var out []string
	for next := last + cycleMonths(p.BillingCycle); next <= cur; next += cycleMonths(p.BillingCycle) {
		out = append(out, monthKeyOf(next))
	}
	return out
}

type missingBill struct {
	Profile store.ListCycleProfilesRow
	Period  string
	Since   time.Time // when it became overdue to arrive
}

// expectedMissing lists cycle profiles whose bill for the current cycle hasn't been added (FR-REM-04).
// Profiles that create their own bill (FR-BILL-10) are left out: the tick adds those.
func expectedMissing(profiles []store.ListCycleProfilesRow, today time.Time, createdOn func(time.Time) time.Time) []missingBill {
	var out []missingBill
	for _, p := range profiles {
		if autoCreates(p) {
			continue
		}
		issue := defaultIssueDay
		if p.TypicalIssueDay != nil {
			issue = int(*p.TypicalIssueDay)
		}
		// The latest period whose deadline has passed; not for a profile made after that
		// deadline (a new profile gets a cycle to add its first bill).
		var found *missingBill
		for _, period := range cyclePeriods(p, today, p.TypicalIssueDay != nil) {
			threshold := dayIn(period, issue).AddDate(0, 0, missingGrace)
			if !today.Before(threshold) && createdOn(p.CreatedAt).Before(threshold) {
				found = &missingBill{Profile: p, Period: period, Since: threshold}
			}
		}
		if found != nil {
			out = append(out, *found)
		}
	}
	return out
}

func autoCreates(p store.ListCycleProfilesRow) bool {
	return p.Kind == store.ProfileKindRecurring && p.AutoCreateBill && p.ExpectedAmountMinor != nil
}

// dueToCreate returns the period a recurring profile should create a bill for today, if any.
func dueToCreate(p store.ListCycleProfilesRow, today time.Time) (string, bool) {
	if !autoCreates(p) {
		return "", false
	}
	periods := cyclePeriods(p, today, true)
	if len(periods) == 0 {
		return "", false
	}
	period := periods[len(periods)-1] // the current cycle; missed ones aren't back-filled
	issue := 1
	if p.TypicalIssueDay != nil {
		issue = int(*p.TypicalIssueDay)
	}
	if today.Before(dayIn(period, issue)) {
		return "", false
	}
	return period, true
}

type reminderStats struct {
	BillsCreated  int `json:"billsCreated"`
	Notifications int `json:"notifications"`
}

// runReminders is the reminder part of the tick (FR-REM-01/04, FR-BILL-10). It is idempotent:
// every notification has a dedupe key, and a recurring bill is created at most once per period.
func (s *Server) runReminders(ctx context.Context) (reminderStats, error) {
	var stats reminderStats
	members, err := s.db.ListReminderMembers(ctx)
	if err != nil {
		return stats, err
	}
	byHousehold := map[uuid.UUID][]store.ListReminderMembersRow{}
	var order []uuid.UUID
	for _, m := range members {
		if _, ok := byHousehold[m.HouseholdID]; !ok {
			order = append(order, m.HouseholdID)
		}
		byHousehold[m.HouseholdID] = append(byHousehold[m.HouseholdID], m)
	}
	for _, hhID := range order {
		ms := byHousehold[hhID]
		n, created, err := s.householdReminders(ctx, hhID, ms)
		if err != nil {
			return stats, fmt.Errorf("household %s: %w", hhID, err)
		}
		stats.Notifications += n
		stats.BillsCreated += created
	}
	return stats, nil
}

func (s *Server) householdReminders(ctx context.Context, hhID uuid.UUID, members []store.ListReminderMembersRow) (int, int, error) {
	tz, currency := members[0].Timezone, members[0].Currency
	today := s.today(tz)
	now := s.now()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	createdOn := func(t time.Time) time.Time {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}

	profiles, err := s.db.ListCycleProfiles(ctx, hhID)
	if err != nil {
		return 0, 0, err
	}

	notified := 0
	notify := func(q *store.Queries, userID uuid.UUID, kind store.NotificationKind, profileID, billID *uuid.UUID, period *string,
		due *time.Time, amount *int64, key string) error {
		n, err := q.InsertNotification(ctx, store.InsertNotificationParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: hhID, UserID: userID, Kind: kind, ProfileID: profileID, BillID: billID,
			PeriodKey: period, DueDate: due, AmountMinor: amount, DedupeKey: key, CreatedAt: now,
		})
		notified += int(n)
		return err
	}

	// FR-BILL-10: recurring bills create themselves on their issue day.
	created := 0
	for _, p := range profiles {
		period, ok := dueToCreate(p, today)
		if !ok {
			continue
		}
		err := s.db.InTx(ctx, func(q *store.Queries) error {
			billID := uuid.Must(uuid.NewV7())
			var due *time.Time
			if p.TypicalDueDay != nil {
				d := dayIn(period, int(*p.TypicalDueDay))
				issue := 1
				if p.TypicalIssueDay != nil {
					issue = int(*p.TypicalIssueDay)
				}
				if d.Before(dayIn(period, issue)) { // e.g. issued on the 25th, due on the 5th: next month
					d = dayIn(monthKeyOf(monthIndex(period)+1), int(*p.TypicalDueDay))
				}
				due = &d
			}
			amount := *p.ExpectedAmountMinor
			if _, err := q.CreateBill(ctx, store.CreateBillParams{
				ID: billID, HouseholdID: hhID, ProfileID: p.ID, CategoryID: p.CategoryID, PeriodKey: ptr(period),
				DueDate: due, ExpenseDate: dayIn(period, 1), CurrentChargesMinor: amount, PayableMinor: amount,
				Currency: currency, VerifiedAt: &now, Tags: []string{},
			}); err != nil {
				return err
			}
			created, err := q.GetBill(ctx, store.GetBillParams{ID: billID, HouseholdID: hhID, Today: today})
			if err != nil {
				return err
			}
			if err := recordCreatedSource(ctx, q, now, hhID, nil, created.Bill, sourceInput{Kind: store.BillSourceKindAuto}); err != nil {
				return err
			}
			changes, _ := json.Marshal(map[string]any{"periodKey": []any{nil, period}, "payableMinor": []any{nil, amount}, "auto": true})
			if err := q.CreateSystemAuditEvent(ctx, store.CreateSystemAuditEventParams{
				ID: uuid.Must(uuid.NewV7()), HouseholdID: hhID, EntityType: "bill", EntityID: billID, Action: "create", Changes: changes,
			}); err != nil {
				return err
			}
			for _, m := range members {
				if err := notify(q, m.UserID, store.NotificationKindBillCreated, &p.ID, &billID, ptr(period), due, &amount,
					"bill_created:"+p.ID.String()+":"+period); err != nil {
					return err
				}
			}
			return nil
		})
		if store.UniqueViolation(err) == "bills_profile_period_uniq" {
			continue // a bill for that period appeared meanwhile
		} else if err != nil {
			return notified, created, err
		}
		created++
	}

	// FR-REM-01: due soon, due today, overdue.
	bills, err := s.db.ListReminderBills(ctx, store.ListReminderBillsParams{Today: today, HouseholdID: hhID, Until: today.AddDate(0, 0, reminderHorizon)})
	if err != nil {
		return notified, created, err
	}
	missing := expectedMissing(profiles, today, createdOn)
	q := s.db.Queries
	for _, m := range members {
		for _, b := range bills {
			days := int(b.DueDate.Sub(today).Hours() / 24)
			due, amount := b.DueDate, b.OutstandingMinor
			suffix := b.ID.String() + ":" + dateString(due)
			var err error
			switch {
			case days < 0 && m.Overdue:
				err = notify(q, m.UserID, store.NotificationKindOverdue, &b.ProfileID, &b.ID, b.PeriodKey, &due, &amount, "overdue:"+suffix)
			case days == 0 && m.DueSoon:
				err = notify(q, m.UserID, store.NotificationKindDueToday, &b.ProfileID, &b.ID, b.PeriodKey, &due, &amount, "due_today:"+suffix)
			case days > 0 && days <= int(m.LeadDays) && m.DueSoon:
				err = notify(q, m.UserID, store.NotificationKindDueSoon, &b.ProfileID, &b.ID, b.PeriodKey, &due, &amount, "due_soon:"+suffix)
			}
			if err != nil {
				return notified, created, err
			}
		}
		// FR-REM-04: expected but missing.
		if m.Missing {
			for _, mb := range missing {
				if err := notify(q, m.UserID, store.NotificationKindMissingBill, &mb.Profile.ID, nil, ptr(mb.Period), nil, nil,
					"missing:"+mb.Profile.ID.String()+":"+mb.Period); err != nil {
					return notified, created, err
				}
			}
		}
	}
	return notified, created, nil
}
