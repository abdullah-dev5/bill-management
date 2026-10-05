package search

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	elec        = Named{ID: uuid.MustParse("a0000000-0000-4000-8000-000000000001"), Name: "Electricity"}
	gas         = Named{ID: uuid.MustParse("a0000000-0000-4000-8000-000000000002"), Name: "Gas"}
	internet    = Named{ID: uuid.MustParse("a0000000-0000-4000-8000-000000000004"), Name: "Internet"}
	maintenance = Named{ID: uuid.MustParse("a0000000-0000-4000-8000-000000000012"), Name: "Maintenance"}
	electrician = Named{ID: uuid.MustParse("c0000000-0000-4000-8000-000000000001"), Name: "Electrician", ParentID: &maintenance.ID}
	general     = Named{ID: uuid.MustParse("a0000000-0000-4000-8000-000000000020"), Name: "General expenses"}
	lesco       = Named{ID: uuid.MustParse("b0000000-0000-4000-8000-000000000001"), Name: "Lahore Electric Supply Company", Short: "LESCO"}
	kelectric   = Named{ID: uuid.MustParse("b0000000-0000-4000-8000-000000000011"), Name: "K-Electric", Short: "K-Electric"}

	cat   = Catalog{Types: []Named{elec, gas, internet, maintenance, electrician, general}, Providers: []Named{lesco, kelectric}}
	today = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
)

func TestParse(t *testing.T) {
	cases := []struct {
		text      string
		types     []string
		providers int
		from, to  string
		statuses  []string
		terms     []string
	}{
		{"electricity september 2025", []string{"Electricity"}, 0, "2025-09", "2025-09", nil, nil},
		{"Electricity Sept", []string{"Electricity"}, 0, "2026-09", "2026-09", nil, nil},
		{"gas december", []string{"Gas"}, 0, "2025-12", "2025-12", nil, nil}, // latest December not in the future
		{"bijli bill 2026-08", []string{"Electricity"}, 0, "2026-08", "2026-08", nil, nil},
		{"gas last winter", []string{"Gas"}, 0, "2025-11", "2026-02", nil, nil},
		{"gas winter", []string{"Gas"}, 0, "2025-11", "2026-02", nil, nil},
		{"internet this year", []string{"Internet"}, 0, "2026-01", "2026-12", nil, nil},
		{"unpaid bills", nil, 0, "", "", []string{"unpaid", "overdue", "partially_paid"}, nil},
		{"maintenance 2025", []string{"Maintenance"}, 0, "2025-01", "2025-12", nil, nil},
		{"lesco last month", nil, 1, "2026-09", "2026-09", nil, nil},
		{"k-electric overdue", nil, 1, "", "", []string{"overdue"}, nil},
		{"general expenses plumber", []string{"General expenses"}, 0, "", "", nil, []string{"plumber"}},
		{"kitchen wiring", nil, 0, "", "", nil, []string{"kitchen", "wiring"}},
		{"0912345", nil, 0, "", "", nil, []string{"0912345"}},
	}
	for _, c := range cases {
		q := Parse(c.text, cat, today)
		if !reflect.DeepEqual(q.TypeNames, c.types) || len(q.Providers) != c.providers || q.From != c.from || q.To != c.to ||
			!reflect.DeepEqual(q.Statuses, c.statuses) || !reflect.DeepEqual(q.Terms, c.terms) {
			t.Errorf("%q → types %v providers %d %s–%s statuses %v terms %v", c.text, q.TypeNames, len(q.Providers), q.From, q.To, q.Statuses, q.Terms)
		}
	}
}

func TestLastWinterDuringWinter(t *testing.T) {
	q := Parse("gas last winter", cat, time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC))
	if q.From != "2025-11" || q.To != "2026-02" {
		t.Errorf("in January, last winter = %s–%s", q.From, q.To)
	}
	q = Parse("gas this winter", cat, time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC))
	if q.From != "2026-11" || q.To != "2027-02" {
		t.Errorf("in January, this winter = %s–%s", q.From, q.To)
	}
}

func TestParseIncludesSubCategories(t *testing.T) {
	q := Parse("maintenance", cat, today)
	if len(q.TypeIDs) != 2 || q.TypeIDs[1] != electrician.ID {
		t.Errorf("type ids = %v", q.TypeIDs)
	}
}

func TestParseAliasNeedsType(t *testing.T) {
	// No "Rent" type in this catalogue: the word stays a search term.
	if q := Parse("rent", cat, today); len(q.TypeIDs) != 0 || !reflect.DeepEqual(q.Terms, []string{"rent"}) {
		t.Errorf("%+v", q)
	}
}
