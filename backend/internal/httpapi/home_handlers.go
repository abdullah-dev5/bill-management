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

	out := apigen.HomeSummary{Month: month, DueSoon: []apigen.Bill{}, Overdue: []apigen.Bill{}, Pinned: []apigen.Profile{}, Recent: []apigen.Bill{}}
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
		if b.Status == apigen.Overdue {
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
	ctx := r.Context()
	sessions, err := s.db.DeleteExpiredSessions(ctx, s.now())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tokens, err := s.db.DeleteExpiredPasswordResetTokens(ctx, s.now())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	due, err := s.db.ListUsersDueForPurge(ctx, ptr(s.now().Add(-accountDeletionTTL)))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, userID := range due {
		err := s.db.InTx(ctx, func(q *store.Queries) error {
			if err := q.DeleteSoleMemberHouseholds(ctx, userID); err != nil {
				return err
			}
			return q.DeleteUser(ctx, userID)
		})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		s.log.InfoContext(ctx, "purged account after deletion grace period", "user_id", userID)
	}
	stale, err := s.db.DeleteStalePendingDocuments(ctx, s.now().Add(-stalePendingAfter))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := s.db.FailAbandonedExtractions(ctx, s.now()); err != nil {
		s.fail(w, r, err)
		return
	}
	filesDeleted, err := s.drainStorageDeletions(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.lastTick.Store(s.now().Unix())
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionsDeleted": sessions, "resetTokensDeleted": tokens, "accountsPurged": len(due),
		"staleUploadsDeleted": stale, "filesDeleted": filesDeleted,
	})
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
