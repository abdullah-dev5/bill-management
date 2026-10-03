package extract

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var months = map[string]time.Month{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

func monthOf(s string) (time.Month, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) < 3 {
		return 0, false
	}
	m, ok := months[s[:3]]
	return m, ok
}

var amountRe = regexp.MustCompile(`-?\d[\d,]*(?:\.\d{1,2})?`)

// ParseMoney turns printed amounts like "Rs. 3,266", "3266.50" or "(1,200)" into minor units.
func ParseMoney(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") || strings.HasSuffix(s, "-") || strings.HasSuffix(strings.ToUpper(s), "CR")
	m := amountRe.FindString(s)
	if m == "" {
		return 0, false
	}
	if strings.HasPrefix(m, "-") {
		neg = true
		m = m[1:]
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	if err != nil || f > 1e12 {
		return 0, false
	}
	minor := int64(math.Round(f * 100))
	if neg {
		minor = -minor
	}
	return minor, true
}

// ParseNumber parses readings, units and multiplying factors ("18,854", "1.5").
func ParseNumber(s string) (float64, bool) {
	m := amountRe.FindString(strings.TrimSpace(s))
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	return f, err == nil
}

var (
	isoDateRe   = regexp.MustCompile(`\b(\d{4})-(\d{1,2})-(\d{1,2})\b`)
	dmyNameRe   = regexp.MustCompile(`(?i)\b(\d{1,2})[\s\-./]*([a-z]{3,9})[\s\-./,]*(\d{2,4})\b`)
	mdyNameRe   = regexp.MustCompile(`(?i)\b([a-z]{3,9})[\s\-./]*(\d{1,2})(?:st|nd|rd|th)?[\s,\-./]+(\d{4})\b`)
	dmyNumRe    = regexp.MustCompile(`\b(\d{1,2})[/\-.](\d{1,2})[/\-.](\d{2,4})\b`)
	monthYearRe = regexp.MustCompile(`(?i)\b([a-z]{3,9})[\s\-./']*(\d{2}|\d{4})\b`)
	numMonthRe  = regexp.MustCompile(`\b(\d{4})-(\d{2})\b|\b(\d{1,2})[/\-](\d{4})\b`)
)

func fullYear(y int) int {
	if y < 100 {
		return 2000 + y
	}
	return y
}

func validDate(y int, m time.Month, d int) (string, bool) {
	if y < 2000 || y > 2100 || m < 1 || m > 12 || d < 1 || d > 31 {
		return "", false
	}
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

// ParseDate understands "05 OCT 26", "05-OCT-2026", "Oct 5, 2026", "2026-10-05" and "05/10/2026"
// (day first, as printed on Pakistani bills). It returns YYYY-MM-DD.
func ParseDate(s string) (string, bool) {
	if m := isoDateRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return validDate(y, time.Month(mo), d)
	}
	if m := dmyNameRe.FindStringSubmatch(s); m != nil {
		if mo, ok := monthOf(m[2]); ok {
			d, _ := strconv.Atoi(m[1])
			y, _ := strconv.Atoi(m[3])
			return validDate(fullYear(y), mo, d)
		}
	}
	if m := mdyNameRe.FindStringSubmatch(s); m != nil {
		if mo, ok := monthOf(m[1]); ok {
			d, _ := strconv.Atoi(m[2])
			y, _ := strconv.Atoi(m[3])
			return validDate(y, mo, d)
		}
	}
	if m := dmyNumRe.FindStringSubmatch(s); m != nil {
		d, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		return validDate(fullYear(y), time.Month(mo), d)
	}
	return "", false
}

// ParseMonth understands "SEP 26", "Sep-2026", "September 2026", "2026-09" and "09/2026". It returns YYYY-MM.
func ParseMonth(s string) (string, bool) {
	if m := monthYearRe.FindStringSubmatch(s); m != nil {
		if mo, ok := monthOf(m[1]); ok {
			y, _ := strconv.Atoi(m[2])
			if d, ok := validDate(fullYear(y), mo, 1); ok {
				return d[:7], true
			}
		}
	}
	if m := numMonthRe.FindStringSubmatch(s); m != nil {
		y, mo := m[1], m[2]
		if y == "" {
			y, mo = m[4], m[3]
		}
		yi, _ := strconv.Atoi(y)
		mi, _ := strconv.Atoi(mo)
		if d, ok := validDate(yi, time.Month(mi), 1); ok {
			return d[:7], true
		}
	}
	return "", false
}

// NormalizeReference keeps letters and digits only, upper-cased.
func NormalizeReference(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
		}
	}
	return b.String()
}

var cnicRe = regexp.MustCompile(`\b\d{5}-?\d{7}-?\d\b`)

// Redact removes CNIC-like numbers and the given personal values (e.g. the customer's name)
// before text is stored (docs/08 §4).
func Redact(text string, personal ...string) string {
	text = cnicRe.ReplaceAllString(text, "[redacted]")
	for _, p := range personal {
		p = strings.TrimSpace(p)
		if len(p) >= 4 {
			text = strings.ReplaceAll(text, p, "[redacted]")
		}
	}
	return text
}
