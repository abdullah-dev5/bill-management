package httpapi

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

var groupOrder = []store.TypeGroup{store.TypeGroupUtilities, store.TypeGroupHousehold, store.TypeGroupOther}

// monthTotal is FR-DASH-07: what the household has to pay for the month's bills, by group
// and by profile. Total = paid + still to pay at today's amounts.
func (s *Server) monthTotal(ctx context.Context, householdID uuid.UUID, month string, today time.Time) (apigen.MonthTotal, error) {
	out := apigen.MonthTotal{Groups: []apigen.MonthTotalGroup{}, Profiles: []apigen.MonthTotalProfile{}}
	rows, err := s.db.MonthTotalsByProfile(ctx, store.MonthTotalsByProfileParams{HouseholdID: householdID, Month: month, Today: today})
	if err != nil {
		return out, err
	}
	groups := map[store.TypeGroup]*apigen.MonthTotalGroup{}
	carried := map[uuid.UUID]int64{} // arrears on this month's open bills, per profile
	for _, r := range rows {
		carried[r.ProfileID] += r.OpenArrearsMinor
		total := r.PaidMinor + r.RemainingMinor
		out.Profiles = append(out.Profiles, apigen.MonthTotalProfile{
			ProfileId: r.ProfileID, Name: r.Name, Icon: r.Icon, Color: int(r.Color), Group: apigen.TypeGroup(r.TypeGroup),
			TotalMinor: total, PaidMinor: r.PaidMinor, RemainingMinor: r.RemainingMinor, BillCount: int(r.BillCount),
		})
		g, ok := groups[r.TypeGroup]
		if !ok {
			g = &apigen.MonthTotalGroup{Group: apigen.TypeGroup(r.TypeGroup)}
			groups[r.TypeGroup] = g
		}
		g.TotalMinor += total
		g.PaidMinor += r.PaidMinor
		g.RemainingMinor += r.RemainingMinor
		g.BillCount += int(r.BillCount)
		out.TotalMinor += total
		out.PaidMinor += r.PaidMinor
		out.RemainingMinor += r.RemainingMinor
		out.BillCount += int(r.BillCount)
	}
	for _, key := range groupOrder {
		if g, ok := groups[key]; ok {
			out.Groups = append(out.Groups, *g)
		}
	}
	earlier, err := s.db.EarlierOutstandingByProfile(ctx, store.EarlierOutstandingByProfileParams{HouseholdID: householdID, Month: month, Today: today})
	if err != nil {
		return out, err
	}
	// An earlier bill's unpaid amount that this month's bill carries as arrears is already in the total.
	for _, e := range earlier {
		out.EarlierOutstandingMinor += max(e.OutstandingMinor-carried[e.ProfileID], 0)
	}

	next, err := s.db.MonthNextDue(ctx, store.MonthNextDueParams{HouseholdID: householdID, Month: month, Today: today})
	if err == nil {
		b := toBill(billView(next))
		out.NextDue = &b
	} else if !store.IsNotFound(err) {
		return out, err
	}
	return out, nil
}

// arrearsTolerance absorbs rounding on printed bills (Rs 1).
const arrearsTolerance = 100

// settleArrears applies the arrears rule (docs/04 §3): when a bill that carries arrears is paid
// in full, the earlier unpaid bills whose amounts make up those arrears are settled through it.
// It first undoes settlements this bill made before, so it can be called after any change.
func (s *Server) settleArrears(ctx context.Context, q *store.Queries, a authInfo, billID uuid.UUID, today time.Time) error {
	cleared, err := q.ClearSettlementsBy(ctx, &billID)
	if err != nil {
		return err
	}
	row, err := q.GetBill(ctx, store.GetBillParams{ID: billID, HouseholdID: a.HouseholdID, Today: today})
	if err != nil {
		return err
	}
	b := row.Bill
	settled := map[uuid.UUID]bool{}
	if row.Status == "paid" && b.SettledByBillID == nil && b.ArrearsMinor > 0 && b.ArchivedAt == nil && !b.Cancelled {
		earlier, err := q.ListEarlierOpenBills(ctx, store.ListEarlierOpenBillsParams{
			ProfileID: b.ProfileID, BillID: b.ID, Before: b.ExpenseDate, Today: today,
		})
		if err != nil {
			return err
		}
		// Walk back through consecutive unpaid bills until their amounts add up to the arrears,
		// counting each either at today's amount (late fees included) or at its original amount.
		var owedNow, owedOriginal int64
		var chosen []uuid.UUID
		for _, e := range earlier {
			if e.OutstandingMinor <= 0 {
				break
			}
			owedNow += e.OutstandingMinor
			owedOriginal += max(e.Bill.PayableMinor-e.Bill.PaidMinor, 0)
			chosen = append(chosen, e.Bill.ID)
			if abs64(owedNow-b.ArrearsMinor) <= arrearsTolerance || abs64(owedOriginal-b.ArrearsMinor) <= arrearsTolerance {
				for _, id := range chosen {
					settled[id] = true
				}
				break
			}
			if owedOriginal > b.ArrearsMinor+arrearsTolerance {
				break
			}
		}
	}

	label := billTitle(b)
	for id := range settled {
		if err := q.SettleBill(ctx, store.SettleBillParams{ID: id, HouseholdID: a.HouseholdID, SettledBy: &billID}); err != nil {
			return err
		}
	}
	wasSettled := map[uuid.UUID]bool{}
	for _, id := range cleared {
		wasSettled[id] = true
		if !settled[id] {
			if err := audit(ctx, q, a, "bill", id, "unsettled", map[string]any{"settledBy": []any{label, nil}}); err != nil {
				return err
			}
		}
	}
	for id := range settled {
		if !wasSettled[id] {
			if err := audit(ctx, q, a, "bill", id, "settled", map[string]any{"settledBy": []any{nil, label}}); err != nil {
				return err
			}
		}
	}
	return nil
}

// billTitle names a bill for history entries: its billing month, or its description.
func billTitle(b store.Bill) string {
	if b.PeriodKey != nil {
		return monthLabel(*b.PeriodKey)
	}
	if b.Description != nil {
		return *b.Description
	}
	return b.ExpenseDate.Format("02 Jan 2006")
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
