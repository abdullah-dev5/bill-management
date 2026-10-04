package extract

import (
	"regexp"
	"strings"
)

// --- SSGC format (Sui Southern Gas Company) ----------------------------------------

const ssgcDate = `\d{1,2}-[A-Za-z]{3}-\d{2,4}`

var (
	ssgcMarker = regexp.MustCompile(`(?i)\bSUI\s+SOUTHERN\s+GAS\b|\bSSGC\b`)
	// The tear-off stub: "1234567890 (4) 2,740 11-Oct-2024 2,810".
	ssgcStub = regexp.MustCompile(`\b(\d{10})\s*(?:\(\d\))?\s+([\d,]+)\s+(` + ssgcDate + `)\s+([\d,]+)\b`)
	// The account summary row under its header: previous balance, current charges, payable,
	// late surcharge, payable after due date, due date.
	ssgcSummary  = regexp.MustCompile(`(?m)^\s*(-?[\d,]+)\s+(-?[\d,]+)\s+([\d,]+)\s+([\d,]+)\s+([\d,]+)\s+(` + ssgcDate + `)\s*$`)
	ssgcCustomer = regexp.MustCompile(`(?i)CUSTOMER\s*(?:NUMBER|NO\.?)\s*:?\s*(\d{10})\b`)
	ssgcMonth    = regexp.MustCompile(`(?i)BILL\s*MONTH\s*:?\s*([A-Za-z]{3,9}[\s\-]\d{2,4})`)
	// Meter row: meter no, current reading + date, previous reading + date.
	ssgcMeter   = regexp.MustCompile(`(?m)^\s*[A-Z0-9\-]{4,15}\s+([\d,]+(?:\.\d+)?)\s+` + ssgcDate + `\s+([\d,]+(?:\.\d+)?)\s+` + ssgcDate + `\b`)
	ssgcHistory = regexp.MustCompile(`(?m)^\s*([A-Za-z]{3})-(\d{4})\s+([\d,]+)(?:\s+` + ssgcDate + `)?\s+([\d,]+)\s*$`)
	ssgcName    = regexp.MustCompile(`(?m)^\s*Name\s*:\s*([A-Za-z][A-Za-z .'\-]{2,60})$`)
)

func looksLikeSSGC(text string) bool { return ssgcMarker.MatchString(text) }

func parseSSGC(r *Result, text, source string) {
	const fromStub, fromLabel = 0.95, 0.88

	if m := ssgcStub.FindStringSubmatch(text); m != nil {
		r.set(FReference, m[1], fromStub, source)
		if v, ok := ParseMoney(m[2]); ok {
			r.setMoney(FPayable, v, fromStub, source)
		}
		if d, ok := ParseDate(m[3]); ok {
			r.set(FDueDate, d, fromStub, source)
		}
		if v, ok := ParseMoney(m[4]); ok {
			r.setMoney(FPayableAfterDue, v, fromStub, source)
		}
	}
	if m := ssgcCustomer.FindStringSubmatch(text); m != nil {
		r.set(FReference, m[1], fromLabel, source)
	}
	if m := ssgcMonth.FindStringSubmatch(text); m != nil {
		if month, ok := ParseMonth(m[1]); ok {
			r.set(FBillingMonth, month, fromLabel, source)
		}
	}
	if d, ok := dateAfter(text, `ISSUE\s*DATE`); ok {
		r.set(FIssueDate, d, fromLabel, source)
	}

	if m := ssgcSummary.FindStringSubmatch(text); m != nil {
		prev, ok1 := ParseMoney(m[1])
		current, ok2 := ParseMoney(m[2])
		payable, ok3 := ParseMoney(m[3])
		after, ok5 := ParseMoney(m[5])
		// The row must add up (within rounding) before we trust its columns.
		if ok1 && ok2 && ok3 && abs(prev+current-payable) <= 100 {
			r.setMoney(FArrears, prev, fromLabel, source)
			r.setMoney(FCurrentCharges, current, fromLabel, source)
			r.setMoney(FPayable, payable, fromLabel, source)
			if ok5 && after >= payable {
				r.setMoney(FPayableAfterDue, after, fromLabel, source)
				r.LateFees = append(r.LateFees, LateFee{AmountMinor: after})
			}
			if d, ok := ParseDate(m[6]); ok {
				r.set(FDueDate, d, fromLabel, source)
			}
		}
	}

	if m := ssgcMeter.FindStringSubmatch(text); m != nil {
		cur, ok1 := ParseNumber(m[1])
		prev, ok2 := ParseNumber(m[2])
		if ok1 && ok2 {
			r.setNumber(FCurrentReading, cur, fromLabel-0.05, source)
			r.setNumber(FPreviousReading, prev, fromLabel-0.05, source)
		}
	}

	for _, m := range ssgcHistory.FindAllStringSubmatch(text, -1) {
		month, ok := ParseMonth(m[1] + " " + m[2])
		if !ok {
			continue
		}
		billed, ok1 := ParseMoney(m[3])
		paid, ok2 := ParseMoney(m[4])
		if !ok1 || !ok2 {
			continue
		}
		r.History = append(r.History, HistoryRow{Month: month, BilledMinor: &billed, PaidMinor: &paid})
	}

	if m := ssgcName.FindStringSubmatch(text); m != nil {
		r.set(FCustomerName, strings.TrimSpace(m[1]), 0.6, source)
	}
}
