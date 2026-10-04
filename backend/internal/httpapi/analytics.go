package httpapi

import (
	"context"
	"net/http"
	"sort"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const maxAnalyticsMonths = 36

// analyticsRow is one bill (spend basis) or one payment (cash basis).
type analyticsRow struct {
	ProfileID    uuid.UUID
	ProfileName  string
	ProfileIcon  string
	ProfileColor int16
	ProfileKind  store.ProfileKind
	Unit         *string
	TypeID       uuid.UUID
	TypeName     string
	TypeIcon     string
	Group        store.TypeGroup
	ProviderID   *uuid.UUID
	ProviderName *string
	Month        string
	AmountMinor  int64
	Units        *float64
	BillID       *uuid.UUID
}

type period int

const (
	current period = iota
	previous
	lastYear
)

// monthRange lists the YYYY-MM keys from..to inclusive.
func monthRange(from, to string) ([]string, error) {
	start, err := domain.PeriodStart(from)
	if err != nil {
		return nil, err
	}
	end, err := domain.PeriodStart(to)
	if err != nil {
		return nil, err
	}
	var out []string
	for m := start; !m.After(end); m = m.AddDate(0, 1, 0) {
		out = append(out, domain.MonthKey(m))
		if len(out) > maxAnalyticsMonths {
			break
		}
	}
	return out, nil
}

func shiftMonth(key string, delta int) string {
	t, _ := domain.PeriodStart(key)
	return domain.MonthKey(t.AddDate(0, delta, 0))
}

func (s *Server) GetAnalytics(w http.ResponseWriter, r *http.Request, params apigen.GetAnalyticsParams) {
	a := mustAuth(r)
	ctx := r.Context()
	months, err := monthRange(params.From, params.To)
	switch {
	case err != nil:
		s.fail(w, r, errField("from", "Use the format YYYY-MM."))
		return
	case len(months) == 0:
		s.fail(w, r, errField("to", "The end month must not be before the start month."))
		return
	case len(months) > maxAnalyticsMonths:
		s.fail(w, r, errField("to", "Choose a range of 36 months or less."))
		return
	}
	basis := "spend"
	if params.Basis != nil {
		basis = string(*params.Basis)
	}
	n := len(months)
	ranges := map[period][2]string{
		current:  {params.From, params.To},
		previous: {shiftMonth(params.From, -n), shiftMonth(params.To, -n)},
		lastYear: {shiftMonth(params.From, -12), shiftMonth(params.To, -12)},
	}

	b := newSeriesBuilder(months, basis == "spend")
	for p, rg := range ranges {
		rows, err := s.analyticsRows(ctx, a.HouseholdID, rg[0], rg[1], basis)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		printed, err := s.db.AnalyticsPrintedUnits(ctx, store.AnalyticsPrintedUnitsParams{HouseholdID: a.HouseholdID, FromMonth: rg[0], ToMonth: rg[1]})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		b.add(p, rows, printed)
	}
	writeJSON(w, http.StatusOK, b.result(params.From, params.To, basis))
}

func (s *Server) analyticsRows(ctx context.Context, householdID uuid.UUID, from, to, basis string) ([]analyticsRow, error) {
	var out []analyticsRow
	if basis == "cash" {
		rows, err := s.db.AnalyticsPayments(ctx, store.AnalyticsPaymentsParams{HouseholdID: householdID, FromMonth: from, ToMonth: to})
		for _, r := range rows {
			out = append(out, analyticsRow{
				ProfileID: r.ProfileID, ProfileName: r.ProfileName, ProfileIcon: r.ProfileIcon, ProfileColor: r.ProfileColor,
				ProfileKind: r.ProfileKind, Unit: r.ConsumptionUnit, TypeID: r.TypeID, TypeName: r.TypeName, TypeIcon: r.TypeIcon,
				Group: r.TypeGroup, ProviderID: r.ProviderID, ProviderName: r.ProviderName, Month: r.Month, AmountMinor: r.AmountMinor,
			})
		}
		return out, err
	}
	rows, err := s.db.AnalyticsBills(ctx, store.AnalyticsBillsParams{HouseholdID: householdID, FromMonth: from, ToMonth: to})
	for _, r := range rows {
		id := r.ID
		out = append(out, analyticsRow{
			ProfileID: r.ProfileID, ProfileName: r.ProfileName, ProfileIcon: r.ProfileIcon, ProfileColor: r.ProfileColor,
			ProfileKind: r.ProfileKind, Unit: r.ConsumptionUnit, TypeID: r.TypeID, TypeName: r.TypeName, TypeIcon: r.TypeIcon,
			Group: r.TypeGroup, ProviderID: r.ProviderID, ProviderName: r.ProviderName, Month: r.Month, AmountMinor: r.SpendMinor,
			Units: r.Units, BillID: &id,
		})
	}
	return out, err
}

// seriesAcc accumulates one breakdown line.
type seriesAcc struct {
	s          apigen.AnalyticsSeries
	bills      []int64
	maxBill    *uuid.UUID
	units      map[string]float64 // month → units from bills (current period)
	printed    map[string]float64 // month → printed units (current period)
	unitsPrev  float64
	unitsLY    float64
	hasUnits   bool
	billMonths map[period]map[string]bool
}

type seriesBuilder struct {
	months    []string
	index     map[string]int
	spend     bool
	total     *seriesAcc
	groups    map[string]*seriesAcc
	types     map[string]*seriesAcc
	providers map[string]*seriesAcc
	profiles  map[string]*seriesAcc
}

func newSeriesBuilder(months []string, spend bool) *seriesBuilder {
	b := &seriesBuilder{months: months, index: map[string]int{}, spend: spend,
		groups: map[string]*seriesAcc{}, types: map[string]*seriesAcc{}, providers: map[string]*seriesAcc{}, profiles: map[string]*seriesAcc{}}
	for i, m := range months {
		b.index[m] = i
	}
	b.total = b.newAcc("total", "Total")
	return b
}

func (b *seriesBuilder) newAcc(key, name string) *seriesAcc {
	return &seriesAcc{
		s:     apigen.AnalyticsSeries{Key: key, Name: name, Months: make([]int64, len(b.months))},
		units: map[string]float64{}, printed: map[string]float64{},
		billMonths: map[period]map[string]bool{current: {}, previous: {}, lastYear: {}},
	}
}

func (b *seriesBuilder) get(m map[string]*seriesAcc, key, name string, init func(*apigen.AnalyticsSeries)) *seriesAcc {
	acc, ok := m[key]
	if !ok {
		acc = b.newAcc(key, name)
		if init != nil {
			init(&acc.s)
		}
		m[key] = acc
	}
	return acc
}

// add folds one period's rows in; only the current period is broken down by month.
func (b *seriesBuilder) add(p period, rows []analyticsRow, printed []store.AnalyticsPrintedUnitsRow) {
	for _, r := range rows {
		group := string(r.Group)
		accs := []*seriesAcc{
			b.total,
			b.get(b.groups, group, group, func(s *apigen.AnalyticsSeries) { g := apigen.TypeGroup(group); s.Group = &g }),
			b.get(b.types, r.TypeID.String(), r.TypeName, func(s *apigen.AnalyticsSeries) {
				icon, g := r.TypeIcon, apigen.TypeGroup(group)
				s.Icon, s.Group = &icon, &g
			}),
		}
		providerKey, providerName := "none", "No provider"
		if r.ProviderID != nil {
			providerKey, providerName = r.ProviderID.String(), deref(r.ProviderName)
		}
		accs = append(accs, b.get(b.providers, providerKey, providerName, nil))
		prof := b.get(b.profiles, r.ProfileID.String(), r.ProfileName, func(s *apigen.AnalyticsSeries) {
			icon, color, kind, g := r.ProfileIcon, int(r.ProfileColor), apigen.ProfileKind(r.ProfileKind), apigen.TypeGroup(group)
			s.Icon, s.Color, s.Kind, s.Group, s.Unit = &icon, &color, &kind, &g, r.Unit
		})
		accs = append(accs, prof)

		for _, acc := range accs {
			switch p {
			case current:
				acc.s.TotalMinor += r.AmountMinor
				if i, ok := b.index[r.Month]; ok {
					acc.s.Months[i] += r.AmountMinor
				}
				acc.s.BillCount++
			case previous:
				acc.s.PreviousMinor += r.AmountMinor
			case lastYear:
				acc.s.LastYearMinor += r.AmountMinor
			}
		}

		if p == current && b.spend {
			prof.bills = append(prof.bills, r.AmountMinor)
			if prof.maxBill == nil || r.AmountMinor >= maxInt(prof.bills[:len(prof.bills)-1]) {
				prof.maxBill = r.BillID
			}
		}
		if r.Units != nil {
			prof.hasUnits = true
			prof.billMonths[p][r.Month] = true
			switch p {
			case current:
				prof.units[r.Month] += *r.Units
			case previous:
				prof.unitsPrev += *r.Units
			case lastYear:
				prof.unitsLY += *r.Units
			}
		}
	}

	// Printed history fills months the profile has no bill units for.
	for _, h := range printed {
		prof, ok := b.profiles[h.ProfileID.String()]
		if !ok || h.Units == nil || prof.billMonths[p][h.Month] {
			continue
		}
		prof.hasUnits = true
		switch p {
		case current:
			prof.printed[h.Month] = *h.Units
		case previous:
			prof.unitsPrev += *h.Units
		case lastYear:
			prof.unitsLY += *h.Units
		}
	}
}

func maxInt(v []int64) int64 {
	var m int64 = -1 << 62
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

func (b *seriesBuilder) result(from, to, basis string) apigen.Analytics {
	out := apigen.Analytics{
		From: from, To: to, Basis: apigen.AnalyticsBasis(basis), Months: b.months,
		Total: b.total.s, Groups: []apigen.AnalyticsSeries{}, Types: []apigen.AnalyticsSeries{},
		Providers: []apigen.AnalyticsSeries{}, Profiles: []apigen.AnalyticsSeries{},
	}
	for _, g := range groupOrder {
		if acc, ok := b.groups[string(g)]; ok {
			out.Groups = append(out.Groups, acc.s)
		}
	}
	out.Types = sortedSeries(b.types)
	out.Providers = sortedSeries(b.providers)
	for _, acc := range b.profiles {
		if len(acc.bills) > 0 {
			var sum, hi, lo int64 = 0, acc.bills[0], acc.bills[0]
			for _, v := range acc.bills {
				sum += v
				hi = max(hi, v)
				lo = min(lo, v)
			}
			avg := sum / int64(len(acc.bills))
			acc.s.AvgMinor, acc.s.MaxMinor, acc.s.MinMinor, acc.s.MaxBillId = &avg, &hi, &lo, acc.maxBill
		}
		if acc.hasUnits {
			monthly := make([]*float64, len(b.months))
			var total float64
			for i, m := range b.months {
				if u, ok := acc.units[m]; ok {
					v := u
					monthly[i] = &v
					total += u
				} else if u, ok := acc.printed[m]; ok {
					v := u
					monthly[i] = &v
					total += u
				}
			}
			prev, ly := acc.unitsPrev, acc.unitsLY
			acc.s.UnitsMonths, acc.s.UnitsTotal, acc.s.UnitsPrevious, acc.s.UnitsLastYear = &monthly, &total, &prev, &ly
		}
	}
	out.Profiles = sortedSeries(b.profiles)
	return out
}

// sortedSeries orders lines by this period's total, then by name; lines with no amounts at all
// in any of the three periods are left out.
func sortedSeries(m map[string]*seriesAcc) []apigen.AnalyticsSeries {
	out := make([]apigen.AnalyticsSeries, 0, len(m))
	for _, acc := range m {
		if acc.s.TotalMinor == 0 && acc.s.PreviousMinor == 0 && acc.s.LastYearMinor == 0 && acc.s.UnitsTotal == nil {
			continue
		}
		out = append(out, acc.s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalMinor != out[j].TotalMinor {
			return out[i].TotalMinor > out[j].TotalMinor
		}
		return out[i].Name < out[j].Name
	})
	return out
}
