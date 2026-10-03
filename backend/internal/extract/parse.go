package extract

import (
	"regexp"
	"strings"
)

// Provider is what the parsers know about a catalogue provider.
type Provider struct {
	ShortName  string // e.g. "LESCO"
	Name       string
	RefPattern string // regex for the normalised reference number, if known
}

// ParseText extracts fields from document text (a PDF text layer or OCR output).
// Provider-specific parsers run first; the generic label parser fills the gaps.
func ParseText(text string, providers []Provider, source string) Result {
	r := NewResult()
	r.RawText = text
	if text == "" {
		return r
	}
	if p := detectProvider(text, providers); p != "" {
		r.set(FProvider, p, 0.9, source)
	}
	if looksLikePITC(text) {
		parsePITC(&r, text, source+"+pitc")
	}
	parseGeneric(&r, text, source+"+generic")
	r.History = sortHistory(r.History)
	return r
}

// detectProvider finds the first catalogue provider whose short name appears as a word.
func detectProvider(text string, providers []Provider) string {
	upper := strings.ToUpper(text)
	best, bestAt := "", len(upper)+1
	for _, p := range providers {
		re, err := regexp.Compile(`\b` + regexp.QuoteMeta(strings.ToUpper(p.ShortName)) + `\b`)
		if err != nil {
			continue
		}
		if loc := re.FindStringIndex(upper); loc != nil && loc[0] < bestAt {
			best, bestAt = p.ShortName, loc[0]
		}
	}
	return best
}

// --- PITC format (LESCO, HESCO and the other DISCOs) -------------------------------

var (
	pitcDiscos = regexp.MustCompile(`\b(LESCO|IESCO|GEPCO|FESCO|MEPCO|PESCO|HESCO|QESCO|SEPCO|TESCO|HAZECO)\b`)
	// The line printed under the barcode, e.g.
	// "SEP 26 - 09 37333 0408021 - 000003266 - 05 OCT 26 - 000003389 - 081026 - 6"
	pitcBarcode = regexp.MustCompile(`(?i)\b([A-Z]{3})\s?(\d{2})\s*-\s*(\d{2}\s?\d{5}\s?\d{7})\s*-\s*(\d{6,12})\s*-\s*(\d{1,2}\s?[A-Z]{3}\s?\d{2})\s*-\s*(\d{6,12})\s*-\s*(\d{6})\s*-\s*\d\b`)
	pitcRef     = regexp.MustCompile(`(?i)REFERENCE\s*NO[^\d]{0,40}(\d{2}\s?\d{5}\s?\d{7}(?:\s?[A-Z]\b)?)`)
	pitcHistory = regexp.MustCompile(`\b(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)(\d{2})\s+(?:[A-Z]{1,3}\s+)?(\d+)\s+(\d+)\s+(\d+)\b`)
	pitcTill    = regexp.MustCompile(`(?i)Till\s+(\d{1,2}-[A-Z]{3}-\d{2,4})[^\d]{0,10}([\d,]+)`)
	pitcAfter   = regexp.MustCompile(`(?i)After\s+(\d{1,2}-[A-Z]{3}-\d{2,4})[^\d]{0,10}([\d,]+)`)
	pitcName    = regexp.MustCompile(`(?is)NAME\s*&\s*ADDRESS(.*?)(?:TRANSFORMER|FEEDER|SUB\s*DIVISION|CATEGORY|TARIFF)`)
	latinWords  = regexp.MustCompile(`[A-Za-z][A-Za-z.,'\- ]{2,}`)
)

// maxMultiplyingFactor bounds plausible meter constants (homes use 1; large connections up to a few hundred).
const maxMultiplyingFactor = 500

func looksLikePITC(text string) bool {
	return pitcBarcode.MatchString(text) || pitcDiscos.MatchString(strings.ToUpper(text))
}

// after finds the first number that follows label, skipping non-digit text between them.
func after(text, label string, gap int) (string, bool) {
	re := regexp.MustCompile(`(?i)` + label + `[^\d]{0,` + itoa(gap) + `}(-?\d[\d,]*(?:\.\d+)?)`)
	m := re.FindStringSubmatchIndex(text)
	if m == nil {
		return "", false
	}
	// "SEP 26" on a following line is a month, not the value we're looking for.
	if partOfMonth(text, m[2]) {
		return "", false
	}
	return text[m[2]:m[3]], true
}

var monthBefore = regexp.MustCompile(`(?i)\b(jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*[\s\-./']*$`)

// partOfMonth reports whether the number starting at i is the year of a month like "SEP 26".
func partOfMonth(text string, i int) bool {
	start := i - 12
	if start < 0 {
		start = 0
	}
	return monthBefore.MatchString(text[start:i])
}

// before finds a number printed just before label on the same line.
func before(text, label string) (string, bool) {
	re := regexp.MustCompile(`(?i)(-?\d[\d,]*(?:\.\d+)?)[^\d\n]{0,15}` + label)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func dateAfter(text, label string) (string, bool) {
	re := regexp.MustCompile(`(?i)` + label + `[^\d]{0,40}(\d{1,2}[\s\-]?[A-Z]{3}[\s\-]?\d{2,4})`)
	if m := re.FindStringSubmatch(text); m != nil {
		return ParseDate(m[1])
	}
	return "", false
}

func parsePITC(r *Result, text, source string) {
	const fromBarcode, fromLabel = 0.97, 0.88

	if m := pitcBarcode.FindStringSubmatch(text); m != nil {
		if month, ok := ParseMonth(m[1] + " " + m[2]); ok {
			r.set(FBillingMonth, month, fromBarcode, source)
		}
		r.set(FReference, NormalizeReference(m[3]), fromBarcode, source)
		if v, ok := ParseMoney(m[4]); ok {
			r.setMoney(FPayable, v, fromBarcode, source)
		}
		if d, ok := ParseDate(m[5]); ok {
			r.set(FDueDate, d, fromBarcode, source)
		}
		if v, ok := ParseMoney(m[6]); ok {
			r.setMoney(FPayableAfterDue, v, fromBarcode, source)
		}
	}

	if m := pitcRef.FindStringSubmatch(text); m != nil {
		r.set(FReference, NormalizeReference(m[1]), fromLabel, source)
	}
	if d, ok := dateAfter(text, `ISSUE\s*DATE`); ok {
		r.set(FIssueDate, d, fromLabel, source)
	}
	if d, ok := dateAfter(text, `READING\s*DATE`); ok {
		r.set(FReadingDate, d, fromLabel, source)
	}
	if d, ok := dateAfter(text, `DUE\s*DATE`); ok {
		r.set(FDueDate, d, fromLabel, source)
	}
	if v, ok := after(text, `PREVIOUS\s*READING`, 60); ok {
		if n, ok := ParseNumber(v); ok {
			r.setNumber(FPreviousReading, n, fromLabel, source)
		}
	}
	if v, ok := after(text, `PRESENT\s*READING`, 60); ok {
		if n, ok := ParseNumber(v); ok {
			r.setNumber(FCurrentReading, n, fromLabel, source)
		}
	}
	if v, ok := after(text, `\bMF\b`, 20); ok {
		// Multiplying factors are small; a large number here is a meter reading on the next line.
		if n, ok := ParseNumber(v); ok && n > 0 && n <= maxMultiplyingFactor {
			r.setNumber(FMultiplyingFactor, n, fromLabel, source)
		}
	}
	// Uppercase only: "Outstanding Units" and the history header also contain the word.
	if m := regexp.MustCompile(`\bUNITS\b[^\dA-Za-z]{0,30}(\d+)\b`).FindStringSubmatch(text); m != nil {
		if n, ok := ParseNumber(m[1]); ok {
			r.setNumber(FUnits, n, fromLabel-0.08, source)
		}
	}
	if v, ok := after(text, `\bArrears\b`, 20); ok {
		if n, ok := ParseMoney(v); ok {
			r.setMoney(FArrears, n, fromLabel, source)
		}
	}
	if v, ok := after(text, `PAYABLE\s*WITHIN\s*DUE\s*DATE`, 80); ok {
		if n, ok := ParseMoney(v); ok {
			r.setMoney(FPayable, n, fromLabel, source)
		}
	}

	// Stepped late-payment amounts.
	if m := pitcTill.FindStringSubmatch(text); m != nil {
		if d, ok := ParseDate(m[1]); ok {
			if v, ok := ParseMoney(m[2]); ok {
				r.LateFees = append(r.LateFees, LateFee{Until: d, AmountMinor: v})
				r.setMoney(FPayableAfterDue, v, fromLabel, source)
			}
		}
	}
	if m := pitcAfter.FindStringSubmatch(text); m != nil {
		if v, ok := ParseMoney(m[2]); ok {
			r.LateFees = append(r.LateFees, LateFee{AmountMinor: v})
		}
	}

	// Charge breakdown; the grand total is the payable, so current charges exclude arrears.
	items := []struct {
		label, name string
		sign        int64
		useBefore   bool
	}{
		{`Total\s*Electricity\s*Charges`, "Electricity charges", 1, false},
		{`Subsidies`, "Subsidies", -1, true},
		{`Taxes`, "Taxes", 1, false},
		{`Total\s*FPA`, "Fuel price adjustment (FPA)", 1, false},
	}
	for _, it := range items {
		var v string
		var ok bool
		if it.useBefore {
			v, ok = before(text, it.label)
		} else {
			v, ok = after(text, it.label, 15)
		}
		if n, parsed := ParseMoney(v); ok && parsed {
			r.LineItems = append(r.LineItems, LineItem{Label: it.name, AmountMinor: it.sign * abs(n)})
		}
	}
	if payable, ok := r.Money(FPayable); ok {
		arrears, _ := r.Money(FArrears)
		adjust, _ := r.Money(FAdjustments)
		r.setMoney(FCurrentCharges, payable-arrears-adjust, r.Fields[FPayable].Confidence-0.05, source)
	}

	for _, m := range pitcHistory.FindAllStringSubmatch(text, -1) {
		month, ok := ParseMonth(m[1] + " " + m[2])
		if !ok {
			continue
		}
		units, _ := ParseNumber(m[3])
		billed, _ := ParseMoney(m[4])
		paid, _ := ParseMoney(m[5])
		r.History = append(r.History, HistoryRow{Month: month, Units: &units, BilledMinor: &billed, PaidMinor: &paid})
	}

	if m := pitcName.FindStringSubmatch(text); m != nil {
		var parts []string
		for _, w := range latinWords.FindAllString(m[1], -1) {
			parts = append(parts, strings.TrimSpace(w))
		}
		if name := strings.Join(parts, " "); name != "" {
			r.set(FCustomerName, name, 0.6, source)
		}
	}
}

// --- Generic label parser (any bill) -----------------------------------------------

type valueKind int

const (
	kindMoney valueKind = iota
	kindDate
	kindMonth
	kindNumber
	kindReference
)

var genericLabels = []struct {
	field  string
	kind   valueKind
	labels []string
}{
	{FPayableAfterDue, kindMoney, []string{"payable after due date", "amount after due date", "after due date", "late payment amount"}},
	{FPayable, kindMoney, []string{"payable within due date", "amount payable", "total amount due", "total payable", "net payable", "amount due", "total due", "grand total", "balance due", "pay this amount"}},
	{FDueDate, kindDate, []string{"due date", "last date", "pay by", "payment due"}},
	{FIssueDate, kindDate, []string{"issue date", "bill date", "date of issue", "invoice date", "billing date"}},
	{FBillingMonth, kindMonth, []string{"bill month", "billing month", "billing period", "bill period", "month of"}},
	{FReference, kindReference, []string{"reference no", "reference number", "consumer no", "consumer number", "account no", "account number", "customer id", "customer no", "consumer id", "ref no"}},
	{FBillNumber, kindReference, []string{"bill no", "bill number", "invoice no", "invoice number"}},
	{FArrears, kindMoney, []string{"arrears", "previous balance", "outstanding balance", "balance b/f", "previous dues"}},
	{FCurrentCharges, kindMoney, []string{"current charges", "current bill", "charges for the month", "current amount"}},
	{FPreviousReading, kindNumber, []string{"previous reading", "prev reading", "last reading"}},
	{FCurrentReading, kindNumber, []string{"present reading", "current reading", "pres reading"}},
	{FUnits, kindNumber, []string{"units consumed", "units billed", "consumption", "units"}},
}

var referenceToken = regexp.MustCompile(`[A-Z0-9][A-Z0-9\- ]{4,24}[A-Z0-9]`)

func parseValue(s string, kind valueKind) (string, bool) {
	switch kind {
	case kindMoney:
		// A line holding a month or a date ("SEP 26", "05-10-2026") isn't an amount.
		if _, isMonth := ParseMonth(s); isMonth {
			return "", false
		}
		if _, isDate := ParseDate(s); isDate {
			return "", false
		}
		if v, ok := ParseMoney(s); ok {
			return itoa64(v), true
		}
	case kindDate:
		return ParseDate(s)
	case kindMonth:
		if m, ok := ParseMonth(s); ok {
			return m, true
		}
		if d, ok := ParseDate(s); ok {
			return d[:7], true
		}
	case kindNumber:
		if v, ok := ParseNumber(s); ok {
			return formatNumber(v), true
		}
	case kindReference:
		for _, tok := range referenceToken.FindAllString(strings.ToUpper(s), -1) {
			if norm := NormalizeReference(tok); countDigits(norm) >= 5 {
				return norm, true
			}
		}
	}
	return "", false
}

func parseGeneric(r *Result, text, source string) {
	const conf = 0.7
	lines := strings.Split(text, "\n")
	lower := make([]string, len(lines))
	for i, l := range lines {
		lower[i] = strings.ToLower(l)
	}
	for _, spec := range genericLabels {
		if _, done := r.Fields[spec.field]; done {
			continue
		}
	search:
		for _, label := range spec.labels {
			for i, l := range lower {
				at := strings.Index(l, label)
				if at < 0 {
					continue
				}
				// A shorter label inside a longer, different one ("due date" in "after due date").
				if spec.field == FDueDate && strings.Contains(l, "after due") {
					continue
				}
				candidates := []string{lines[i][at+len(label):]}
				for j := i + 1; j <= i+2 && j < len(lines); j++ {
					candidates = append(candidates, lines[j])
				}
				for _, c := range candidates {
					if v, ok := parseValue(c, spec.kind); ok {
						r.set(spec.field, v, conf, source)
						break search
					}
				}
			}
		}
	}
}

func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}
