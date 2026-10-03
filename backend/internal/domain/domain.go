// Package domain holds small, pure business helpers shared by the API handlers.
package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// FieldErrors collects validation problems keyed by API field name.
type FieldErrors map[string]string

func (f FieldErrors) Add(field, msg string) {
	if _, exists := f[field]; !exists {
		f[field] = msg
	}
}

func (f FieldErrors) Any() bool { return len(f) > 0 }

var periodKeyRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// ValidPeriodKey reports whether s is a YYYY-MM billing month.
func ValidPeriodKey(s string) bool { return periodKeyRe.MatchString(s) }

// PeriodStart returns the first day of a YYYY-MM billing month.
func PeriodStart(key string) (time.Time, error) {
	return time.Parse("2006-01", key)
}

// MonthKey formats a date as its YYYY-MM billing month.
func MonthKey(t time.Time) string { return t.Format("2006-01") }

// MonthBounds returns [start, end) for a YYYY-MM month.
func MonthBounds(key string) (start, end time.Time, err error) {
	start, err = PeriodStart(key)
	if err != nil {
		return
	}
	return start, start.AddDate(0, 1, 0), nil
}

// Today returns the current calendar date in loc, as a UTC midnight time (how pgx dates compare).
func Today(now time.Time, loc *time.Location) time.Time {
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// NormalizeReference strips spaces, dashes and other separators from a provider reference number,
// keeping letters and digits, upper-cased. "04 11132-1234567" → "04111321234567".
func NormalizeReference(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// MatchesPattern checks a normalised reference against a provider's regex (empty pattern = no check).
func MatchesPattern(normalised, pattern string) (bool, error) {
	if pattern == "" {
		return true, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("provider pattern: %w", err)
	}
	return re.MatchString(normalised), nil
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// NormalizeEmail trims and lower-cases; ok is false if it doesn't look like an email.
func NormalizeEmail(s string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(s))
	return e, len(e) <= 254 && emailRe.MatchString(e)
}

// CleanText trims a string and returns nil when it is empty.
func CleanText(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// CleanTags trims, drops empties and de-duplicates (case-insensitive), keeping first spelling.
func CleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		k := strings.ToLower(t)
		if t == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, t)
	}
	return out
}
