package extract

import (
	"regexp"
	"sort"
	"strings"
)

// --- Label rows (any bill) ---------------------------------------------------------
//
// Many bills print a row of labels with the values on the line below, e.g.
//
//	Previous Balance (Rs.) Current Charges (Rs.) Payable Within Due Date (Rs.) Due Date
//	3 2,738 2,740 11-Oct-24
//
// and a history table under a "Month … Bill … Paid" header. parseTables reads those
// layouts for any provider: it matches the header against a vocabulary of bill labels,
// then lines the value tokens up with the columns, checking each token fits its column.

type colKind int

const (
	colMoney colKind = iota
	colNumber
	colDate
	colMonth
	colRef
	colCode // meter numbers, tariff codes: anything but a date
)

// Pseudo-fields for columns that matter only for alignment or the history table.
const (
	colSkip      = "-"
	colSurcharge = "~surcharge"
	histMonth    = "~month"
	histUnits    = "~units"
	histBilled   = "~billed"
	histPaid     = "~paid"
)

type colLabel struct {
	field  string
	kind   colKind
	labels []string
}

// Longer labels win over shorter ones at the same position ("payment after due date" over "payment").
var colLabels = []colLabel{
	{FArrears, colMoney, []string{"previous balance", "previous arrears", "arrears", "balance b/f", "previous dues", "outstanding balance", "opening balance"}},
	{FCurrentCharges, colMoney, []string{"current charges", "current bill", "current month charges", "charges for the month", "current amount", "current month bill"}},
	{FPayable, colMoney, []string{"payable within due date", "amount payable within due date", "total amount due", "amount due", "total payable", "net payable", "payable amount", "amount payable", "total amount payable", "within due date"}},
	{FPayableAfterDue, colMoney, []string{"payment after due date", "payable after due date", "amount after due date", "amount payable after due date", "after due date", "late payment amount"}},
	{colSurcharge, colMoney, []string{"late payment surcharge", "late surcharge", "lp surcharge", "late payment charges", "late fee", "surcharge"}},
	{FAdjustments, colMoney, []string{"adjustments", "adjustment"}},
	{FDueDate, colDate, []string{"due date", "last date", "pay by", "last date of payment"}},
	{FIssueDate, colDate, []string{"issue date", "bill date", "billing date", "date of issue"}},
	{FReadingDate, colDate, []string{"reading date", "meter reading date"}},
	{FBillingMonth, colMonth, []string{"bill month", "billing month", "billing period"}},
	{FReference, colRef, []string{"customer number", "customer no", "consumer number", "consumer no", "consumer id", "customer id", "reference no", "reference number", "ref no", "account no", "account number", "account id"}},
	{FBillNumber, colRef, []string{"bill no", "bill number", "invoice no", "invoice number"}},
	{FPreviousReading, colNumber, []string{"previous reading", "prev reading", "last reading", "previous meter reading"}},
	{FCurrentReading, colNumber, []string{"current reading", "present reading", "pres reading", "current meter reading"}},
	{FMultiplyingFactor, colNumber, []string{"multiplying factor", "mf"}},
	{colSkip, colCode, []string{"meter no", "meter number", "tariff", "category", "sanctioned load", "connection type"}},
	{colSkip, colNumber, []string{"measuring qty", "mmbtu", "gcv", "pressure factor"}},
	{colSkip, colDate, []string{"payment date", "paid on", "date paid"}},
	// History-table columns; outside a history table, units are the period's units.
	{histMonth, colMonth, []string{"month", "months"}},
	{histUnits, colNumber, []string{"units consumed", "units billed", "units", "consumption"}},
	{histBilled, colMoney, []string{"bill amount", "billed amount", "amount billed", "billed", "bill"}},
	{histPaid, colMoney, []string{"amount paid", "paid amount", "payment", "paid"}},
}

type headerCol struct {
	field string
	kind  colKind
}

var (
	headerRe = func() *regexp.Regexp {
		var all []string
		for _, c := range colLabels {
			all = append(all, c.labels...)
		}
		sort.Slice(all, func(i, j int) bool { return len(all[i]) > len(all[j]) })
		for i, l := range all {
			all[i] = strings.ReplaceAll(regexp.QuoteMeta(l), " ", `\s+`)
		}
		return regexp.MustCompile(`(?i)\b(?:` + strings.Join(all, "|") + `)\b`)
	}()
	labelOf = func() map[string]headerCol {
		m := map[string]headerCol{}
		for _, c := range colLabels {
			for _, l := range c.labels {
				m[l] = headerCol{c.field, c.kind}
			}
		}
		return m
	}()
	// What may sit between labels in a header: currency marks, punctuation, short words.
	unitMark     = regexp.MustCompile(`(?i)\((?:hm3|hm³|m3|mmbtu|kwh|units?|rs\.?|pkr)\)`)
	headerFiller = regexp.MustCompile(`(?i)\(?\b(?:rs|pkr|no|of|the|in|to|be|rupees)\b\.?\)?|[\s:/|()\-.#&*,]+`)
	spaces       = regexp.MustCompile(`\s+`)
)

// headerColumns reads a line made only of known labels (at least two).
func headerColumns(line string) ([]headerCol, bool) {
	locs := headerRe.FindAllStringIndex(line, -1)
	if len(locs) < 2 {
		return nil, false
	}
	var cols []headerCol
	var rest strings.Builder
	prev := 0
	for _, l := range locs {
		rest.WriteString(line[prev:l[0]])
		key := strings.ToLower(spaces.ReplaceAllString(line[l[0]:l[1]], " "))
		cols = append(cols, labelOf[key])
		prev = l[1]
	}
	rest.WriteString(line[prev:])
	if leftover := headerFiller.ReplaceAllString(unitMark.ReplaceAllString(rest.String(), ""), ""); leftover != "" {
		return nil, false // other words or numbers: not a pure header, columns can't be trusted
	}
	return cols, true
}

type tokKind int

const (
	tokDate tokKind = iota
	tokMonth
	tokRef // long digit run, possibly with a bracketed check digit
	tokNumber
	tokCode
)

type token struct {
	text string
	kind tokKind
}

var tokenRe = regexp.MustCompile(`(?i)` +
	`(\d{1,2}[\-/ ][a-z]{3,9}[\-/ ]\d{2,4}|\d{1,2}[/\-.]\d{1,2}[/\-.]\d{2,4}|\d{4}-\d{2}-\d{2})` + // date
	`|([a-z]{3,9}[\-\s']\d{2,4})\b` + // month
	`|(\d{6,}\s*\(\d\)|\d{6,}\b)` + // reference
	`|(\(?-?[\d,]*\d(?:\.\d+)?\)?)` + // number
	`|(\S+)`) // anything else

var currencyWord = regexp.MustCompile(`(?i)^(?:rs\.?|pkr|rs:)$`)

func lineTokens(line string) []token {
	var out []token
	for _, m := range tokenRe.FindAllStringSubmatch(line, -1) {
		switch {
		case m[1] != "":
			if _, ok := ParseDate(m[1]); ok {
				out = append(out, token{m[1], tokDate})
				continue
			}
			out = append(out, token{m[1], tokCode})
		case m[2] != "":
			if _, ok := ParseMonth(m[2]); ok {
				out = append(out, token{m[2], tokMonth})
				continue
			}
			out = append(out, token{m[2], tokCode})
		case m[3] != "":
			out = append(out, token{m[3], tokRef})
		case m[4] != "":
			out = append(out, token{m[4], tokNumber})
		case currencyWord.MatchString(m[5]):
			// "Rs." before an amount
		default:
			out = append(out, token{m[5], tokCode})
		}
	}
	return out
}

func fits(k colKind, t token) bool {
	switch k {
	case colMoney:
		return t.kind == tokNumber || t.kind == tokRef && !strings.Contains(t.text, "(") && len(t.text) <= 8
	case colNumber:
		return t.kind == tokNumber || t.kind == tokRef && !strings.Contains(t.text, "(")
	case colDate:
		return t.kind == tokDate
	case colMonth:
		return t.kind == tokMonth || t.kind == tokDate
	case colRef:
		return t.kind == tokRef || (t.kind == tokNumber || t.kind == tokCode) && countDigits(t.text) >= 5
	case colCode:
		return t.kind != tokDate
	}
	return false
}

// align lines tokens up with columns. A date printed beside a value (a reading and its
// date) may be skipped; anything else that doesn't fit means the row isn't this table's.
func align(cols []headerCol, toks []token) ([]token, bool) {
	out := make([]token, len(cols))
	i := 0
	for c, col := range cols {
		for i < len(toks) && !fits(col.kind, toks[i]) && toks[i].kind == tokDate {
			i++
		}
		if i >= len(toks) || !fits(col.kind, toks[i]) {
			return nil, false
		}
		out[c] = toks[i]
		i++
	}
	for ; i < len(toks); i++ {
		if toks[i].kind != tokDate {
			return nil, false
		}
	}
	return out, true
}

func isHistoryHeader(cols []headerCol) bool {
	month, other := false, false
	for _, c := range cols {
		switch c.field {
		case histMonth:
			month = true
		case histBilled, histPaid, histUnits:
			other = true
		}
	}
	return month && other
}

func parseTables(r *Result, text, source string) {
	const conf = 0.85
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		cols, ok := headerColumns(lines[i])
		if !ok {
			continue
		}
		if isHistoryHeader(cols) {
			rows := readHistory(cols, lines[i+1:])
			if len(r.History) == 0 && len(rows) >= 2 {
				r.History = rows
			}
			continue
		}
		// The values are on the next non-empty line (or the one after, past a stray line).
		for j, seen := i+1, 0; j < len(lines) && seen < 2; j++ {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			seen++
			vals, ok := align(cols, lineTokens(lines[j]))
			if !ok {
				continue
			}
			applyRow(r, cols, vals, conf, source)
			break
		}
	}
}

func applyRow(r *Result, cols []headerCol, vals []token, conf float64, source string) {
	var surcharge, after int64
	hasAfter := false
	for c, col := range cols {
		v := vals[c].text
		switch col.field {
		case colSkip, histMonth, histBilled, histPaid:
		case colSurcharge:
			surcharge, _ = ParseMoney(v)
		case histUnits:
			if n, ok := ParseNumber(v); ok {
				r.setNumber(FUnits, n, conf-0.05, source)
			}
		case FPayableAfterDue:
			if n, ok := ParseMoney(v); ok {
				r.setMoney(col.field, n, conf, source)
				after, hasAfter = n, true
			}
		default:
			if s, ok := parseValue(v, kindFor(col.kind)); ok {
				r.set(col.field, s, conf, source)
			}
		}
	}
	switch {
	case len(r.LateFees) > 0:
	case hasAfter:
		r.LateFees = append(r.LateFees, LateFee{AmountMinor: after})
	case surcharge > 0:
		r.LateFees = append(r.LateFees, LateFee{AmountMinor: surcharge}) // Validate turns it into the total
	}
}

func kindFor(k colKind) valueKind {
	switch k {
	case colMoney:
		return kindMoney
	case colDate:
		return kindDate
	case colMonth:
		return kindMonth
	case colRef:
		return kindReference
	}
	return kindNumber
}

func readHistory(cols []headerCol, lines []string) []HistoryRow {
	var rows []HistoryRow
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		vals, ok := align(cols, lineTokens(line))
		if !ok {
			break
		}
		var row HistoryRow
		for c, col := range cols {
			v := vals[c].text
			switch col.field {
			case histMonth:
				row.Month, _ = parseValue(v, kindMonth)
			case histUnits:
				if n, ok := ParseNumber(v); ok {
					row.Units = &n
				}
			case histBilled:
				if n, ok := ParseMoney(v); ok {
					row.BilledMinor = &n
				}
			case histPaid:
				if n, ok := ParseMoney(v); ok {
					row.PaidMinor = &n
				}
			}
		}
		if row.Month == "" {
			break
		}
		rows = append(rows, row)
	}
	return rows
}
