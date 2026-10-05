// Package search interprets what people type into the search box (FR-SRCH-01/02):
// "electricity september 2025" becomes the Electricity type and the month 2025-09, and
// whatever isn't understood is searched as text.
package search

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Named is a bill type or provider the household can see.
type Named struct {
	ID       uuid.UUID
	Name     string
	Short    string // providers: "LESCO"
	ParentID *uuid.UUID
}

type Catalog struct {
	Types     []Named
	Providers []Named
}

// Query is the interpreted search.
type Query struct {
	Text      string
	TypeIDs   []uuid.UUID // includes sub-categories of the types named
	TypeNames []string
	Providers []Named
	From, To  string // YYYY-MM, both set or both empty
	Statuses  []string
	// Terms are the words left over, each matched as text (all must match).
	Terms []string
}

func (q Query) Empty() bool {
	return len(q.TypeIDs) == 0 && len(q.Providers) == 0 && q.From == "" && len(q.Statuses) == 0 && len(q.Terms) == 0
}

var monthNames = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6, "july": 7,
	"august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "sept": 9,
	"oct": 10, "nov": 11, "dec": 12,
}

// Everyday words for types, mapped to the catalogue name they mean (used only if that type exists).
var typeAliases = map[string]string{
	"electric": "Electricity", "electricity": "Electricity", "bijli": "Electricity", "light": "Electricity", "power": "Electricity",
	"gas": "Gas", "sui": "Gas",
	"water": "Water", "pani": "Water",
	"internet": "Internet", "wifi": "Internet", "broadband": "Internet", "fiber": "Internet", "fibre": "Internet",
	"phone": "Phone", "mobile": "Phone", "landline": "Phone",
	"rent": "Rent", "kiraya": "Rent",
	"maintenance": "Maintenance", "repair": "Maintenance", "repairs": "Maintenance",
}

var statusWords = map[string][]string{
	"unpaid": {"unpaid", "overdue", "partially_paid"}, "due": {"unpaid", "overdue", "partially_paid"},
	"pending": {"unpaid", "overdue", "partially_paid"}, "outstanding": {"unpaid", "overdue", "partially_paid"},
	"overdue": {"overdue"}, "late": {"overdue"},
	"paid": {"paid"}, "partial": {"partially_paid"},
	"draft": {"draft"}, "review": {"draft"}, "cancelled": {"cancelled"}, "canceled": {"cancelled"},
}

var stopWords = map[string]bool{
	"bill": true, "bills": true, "for": true, "in": true, "of": true, "the": true, "my": true, "a": true,
	"show": true, "all": true, "from": true, "and": true, "what": true, "did": true, "i": true, "pay": true,
	"spend": true, "spent": true, "on": true, "how": true, "much": true, "was": true, "is": true, "where": true,
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}\-/.']*`)

// Parse interprets text against the household's catalogue. today anchors "last month",
// "this year" and a month named without a year (the latest one not in the future).
func Parse(text string, cat Catalog, today time.Time) Query {
	q := Query{Text: strings.TrimSpace(text)}
	words := wordRe.FindAllString(strings.ToLower(q.Text), -1)
	used := make([]bool, len(words))
	month, year := 0, 0
	thisMonth := today.Year()*12 + int(today.Month()) - 1

	setRange := func(from, to int) { // absolute month numbers (year*12 + month-1)
		q.From, q.To = monthKey(from), monthKey(to)
	}

	// Multi-word names first (types and providers like "general expenses", "k-electric").
	matchNames := func(list []Named, add func(Named)) {
		for _, n := range list {
			for _, name := range []string{n.Name, n.Short} {
				parts := wordRe.FindAllString(strings.ToLower(name), -1)
				if len(parts) == 0 {
					continue
				}
				for i := 0; i+len(parts) <= len(words); i++ {
					if anyUsed(used[i:i+len(parts)]) || !equalWords(words[i:i+len(parts)], parts) {
						continue
					}
					markUsed(used[i : i+len(parts)])
					add(n)
				}
			}
		}
	}
	addType := func(n Named) {
		if containsID(q.TypeIDs, n.ID) {
			return
		}
		q.TypeNames = append(q.TypeNames, n.Name)
		q.TypeIDs = append(q.TypeIDs, n.ID)
		for _, child := range cat.Types {
			if child.ParentID != nil && *child.ParentID == n.ID && !containsID(q.TypeIDs, child.ID) {
				q.TypeIDs = append(q.TypeIDs, child.ID)
			}
		}
	}
	matchNames(cat.Providers, func(n Named) {
		for _, p := range q.Providers {
			if p.ID == n.ID {
				return
			}
		}
		q.Providers = append(q.Providers, n)
	})
	matchNames(cat.Types, addType)

	for i := 0; i < len(words); i++ {
		if used[i] {
			continue
		}
		w := words[i]
		next := ""
		if i+1 < len(words) {
			next = words[i+1]
		}
		switch {
		case (w == "this" || w == "last" || w == "previous") && (next == "month" || next == "year" || next == "winter" || next == "summer"):
			used[i], used[i+1] = true, true
			back := 0
			if w != "this" {
				back = 1
			}
			switch next {
			case "month":
				setRange(thisMonth-back, thisMonth-back)
			case "year":
				y := today.Year() - back
				setRange(y*12, y*12+11)
			case "winter":
				setRange(season(thisMonth, 10, 4, back == 1)) // Nov–Feb
			case "summer":
				setRange(season(thisMonth, 4, 4, back == 1)) // May–Aug
			}
		case w == "winter" || w == "summer":
			used[i] = true
			start := 10
			if w == "summer" {
				start = 4
			}
			setRange(season(thisMonth, start, 4, false))
		case monthNames[w] != 0:
			used[i] = true
			month = monthNames[w]
		case isYear(w):
			used[i] = true
			year, _ = strconv.Atoi(w)
		case len(w) == 7 && w[4] == '-' && isYear(w[:4]): // 2025-09
			if m, err := strconv.Atoi(w[5:]); err == nil && m >= 1 && m <= 12 {
				used[i] = true
				year, month = atoi(w[:4]), m
			}
		case statusWords[w] != nil:
			used[i] = true
			for _, s := range statusWords[w] {
				if !contains(q.Statuses, s) {
					q.Statuses = append(q.Statuses, s)
				}
			}
		case typeAliases[w] != "":
			if n, ok := findByName(cat.Types, typeAliases[w]); ok {
				used[i] = true
				addType(n)
			}
		case stopWords[w]:
			used[i] = true
		}
	}

	switch {
	case month != 0 && year != 0:
		m := year*12 + month - 1
		setRange(m, m)
	case month != 0: // the latest such month that isn't in the future
		m := today.Year()*12 + month - 1
		if m > thisMonth {
			m -= 12
		}
		setRange(m, m)
	case year != 0:
		setRange(year*12, year*12+11)
	}

	for i, w := range words {
		if !used[i] {
			q.Terms = append(q.Terms, w)
		}
	}
	return q
}

// season returns the [from, to] months of the latest season starting in startMonth0
// (0-based, n months long) that has begun by now. ended asks for the latest one that is
// already over: "last winter" in October is the winter just gone, in January the one before.
func season(now, startMonth0, n int, ended bool) (int, int) {
	start := now/12*12 + startMonth0
	if start > now {
		start -= 12
	}
	if ended && start+n-1 >= now {
		start -= 12
	}
	return start, start + n - 1
}

func monthKey(m int) string { return fmt.Sprintf("%04d-%02d", m/12, m%12+1) }

func isYear(w string) bool {
	if len(w) != 4 {
		return false
	}
	y, err := strconv.Atoi(w)
	return err == nil && y >= 2000 && y <= 2100
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func findByName(list []Named, name string) (Named, bool) {
	for _, n := range list {
		if strings.EqualFold(n.Name, name) {
			return n, true
		}
	}
	return Named{}, false
}

func equalWords(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func anyUsed(u []bool) bool {
	for _, x := range u {
		if x {
			return true
		}
	}
	return false
}

func markUsed(u []bool) {
	for i := range u {
		u[i] = true
	}
}

func containsID(list []uuid.UUID, id uuid.UUID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
