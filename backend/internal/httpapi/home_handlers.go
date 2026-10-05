package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

func (s *Server) GetHome(w http.ResponseWriter, r *http.Request, params apigen.GetHomeParams) {
	a := mustAuth(r)
	ctx := r.Context()
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	today := s.today(hh.Timezone)
	month := domain.MonthKey(today)
	if params.Month != nil {
		month = *params.Month
	}
	start, end, err := domain.MonthBounds(month)
	if err != nil {
		s.fail(w, r, errField("month", "Use the format YYYY-MM."))
		return
	}

	out := apigen.HomeSummary{Month: month, DueSoon: []apigen.Bill{}, Overdue: []apigen.Bill{}, Pinned: []apigen.Profile{}, Recent: []apigen.Bill{},
		ExpectedMissing: []apigen.ExpectedBill{}}

	// FR-DASH-04: computed live, so Home is right even between ticks.
	cycleProfiles, err := s.db.ListCycleProfiles(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	loc, lerr := time.LoadLocation(hh.Timezone)
	if lerr != nil {
		loc = time.UTC
	}
	for _, mb := range expectedMissing(cycleProfiles, today, func(t time.Time) time.Time {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}) {
		out.ExpectedMissing = append(out.ExpectedMissing, apigen.ExpectedBill{
			ProfileId: mb.Profile.ID, ProfileName: mb.Profile.Name, Icon: mb.Profile.Icon, Color: int(mb.Profile.Color), PeriodKey: mb.Period,
		})
	}
	if out.BilledMinor, err = s.db.MonthBilled(ctx, store.MonthBilledParams{HouseholdID: a.HouseholdID, Month: month}); err != nil {
		s.fail(w, r, err)
		return
	}
	if out.PaidMinor, err = s.db.MonthPaid(ctx, store.MonthPaidParams{HouseholdID: a.HouseholdID, MonthStart: start, MonthEnd: end}); err != nil {
		s.fail(w, r, err)
		return
	}
	if out.OutstandingMinor, err = s.db.TotalOutstanding(ctx, store.TotalOutstandingParams{HouseholdID: a.HouseholdID, Today: today}); err != nil {
		s.fail(w, r, err)
		return
	}

	due, err := s.db.ListOpenBillsDueBy(ctx, store.ListOpenBillsDueByParams{
		HouseholdID: a.HouseholdID, Today: today, Until: today.Add(dueSoonWindow),
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, row := range due {
		b := toBill(billView(row))
		if b.Status == apigen.BillStatusOverdue {
			out.Overdue = append(out.Overdue, b)
		} else {
			out.DueSoon = append(out.DueSoon, b)
		}
	}

	profiles, err := s.db.ListProfiles(ctx, store.ListProfilesParams{HouseholdID: a.HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	summaries, err := s.profileSummaries(ctx, s.db.Queries, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, p := range profiles {
		if p.Pinned {
			out.Pinned = append(out.Pinned, toProfile(p, s.refDisplay(ctx, p), summaries.get(p.ID)))
		}
	}

	if out.MonthTotal, err = s.monthTotal(ctx, a.HouseholdID, month, today); err != nil {
		s.fail(w, r, err)
		return
	}

	recent, err := s.db.ListRecentBills(ctx, store.ListRecentBillsParams{HouseholdID: a.HouseholdID, Today: today})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, row := range recent {
		out.Recent = append(out.Recent, toBill(billView(row)))
	}
	writeJSON(w, http.StatusOK, out)
}

// --- Operations ------------------------------------------------------------------

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{"status": "ok"}
	if err := s.db.Pool.Ping(r.Context()); err != nil {
		body["status"] = "degraded"
		body["db"] = "unreachable"
		writeJSON(w, http.StatusServiceUnavailable, body)
		return
	}
	if t := s.lastTick.Load(); t > 0 {
		body["lastTickAt"] = time.Unix(t, 0).UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, body)
}

// tick is called every 10 minutes by the external scheduler (cron-job.org). It runs housekeeping
// now; reminders and expected-bill checks join it in M5.
func (s *Server) tick(w http.ResponseWriter, r *http.Request) {
	if s.cfg.TickToken == "" {
		writeError(w, &apiError{status: http.StatusServiceUnavailable, code: "tick_disabled", message: "TICK_TOKEN is not configured."})
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.TickToken)) != 1 {
		writeError(w, errUnauthorized("bad_token", "Invalid tick token."))
		return
	}
	out, err := s.RunTick(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// RunTick does the periodic housekeeping and reminders. Production calls it every 10 minutes
// through POST /internal/tick (cron-job.org); development runs it in-process (StartTicker).
func (s *Server) RunTick(ctx context.Context) (map[string]any, error) {
	sessions, err := s.db.DeleteExpiredSessions(ctx, s.now())
	if err != nil {
		return nil, err
	}
	tokens, err := s.db.DeleteExpiredPasswordResetTokens(ctx, s.now())
	if err != nil {
		return nil, err
	}
	due, err := s.db.ListUsersDueForPurge(ctx, ptr(s.now().Add(-accountDeletionTTL)))
	if err != nil {
		return nil, err
	}
	for _, userID := range due {
		err := s.db.InTx(ctx, func(q *store.Queries) error {
			if err := q.DeleteSoleMemberHouseholds(ctx, userID); err != nil {
				return err
			}
			return q.DeleteUser(ctx, userID)
		})
		if err != nil {
			return nil, err
		}
		s.log.InfoContext(ctx, "purged account after deletion grace period", "user_id", userID)
	}
	stale, err := s.db.DeleteStalePendingDocuments(ctx, s.now().Add(-stalePendingAfter))
	if err != nil {
		return nil, err
	}
	if _, err := s.db.FailAbandonedExtractions(ctx, s.now()); err != nil {
		return nil, err
	}
	filesDeleted, err := s.drainStorageDeletions(ctx)
	if err != nil {
		return nil, err
	}
	reminders, err := s.runReminders(ctx)
	if err != nil {
		return nil, err
	}
	s.lastTick.Store(s.now().Unix())
	return map[string]any{
		"sessionsDeleted": sessions, "resetTokensDeleted": tokens, "accountsPurged": len(due),
		"staleUploadsDeleted": stale, "filesDeleted": filesDeleted,
		"billsCreated": reminders.BillsCreated, "notifications": reminders.Notifications,
	}, nil
}

// StartTicker runs RunTick every interval in-process (development, where nothing calls /internal/tick).
func (s *Server) StartTicker(ctx context.Context, every time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			if out, err := s.RunTick(ctx); err != nil && ctx.Err() == nil {
				s.log.ErrorContext(ctx, "tick failed", "err", err)
			} else if err == nil {
				s.log.InfoContext(ctx, "tick", "result", out)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// drainStorageDeletions removes objects queued by deleted documents (including cascades).
// A failed delete stays queued for the next tick.
func (s *Server) drainStorageDeletions(ctx context.Context) (int, error) {
	keys, err := s.db.ListStorageDeletions(ctx, 200)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, key := range keys {
		if err := s.files.Delete(ctx, key); err != nil {
			s.log.WarnContext(ctx, "storage delete failed; will retry", "err", err)
			continue
		}
		if err := s.db.RemoveStorageDeletion(ctx, key); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}
