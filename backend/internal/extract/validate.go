package extract

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// Confidence thresholds (docs/05 §6).
const (
	HighlightBelow = 0.85
	DropBelow      = 0.5
)

// Context is what validation knows besides the document.
type Context struct {
	ReferencePattern string // provider regex, when the provider is known
}

// Validate applies the deterministic rules of docs/05 §6: failed rules add a warning and
// lower the confidence of the fields involved. Fields that end up below DropBelow are
// removed so the user types them in.
func Validate(r *Result, ctx Context) {
	penalise := func(factor float64, msg string, fields ...string) {
		for _, name := range fields {
			if f, ok := r.Fields[name]; ok {
				f.Confidence = math.Round(f.Confidence*factor*100) / 100
				r.Fields[name] = f
			}
		}
		field := ""
		if len(fields) > 0 {
			field = fields[0]
		}
		r.Warnings = append(r.Warnings, Warning{Field: field, Message: msg})
	}

	payable, hasPayable := r.Money(FPayable)
	current, hasCurrent := r.Money(FCurrentCharges)
	arrears, _ := r.Money(FArrears)
	adjust, _ := r.Money(FAdjustments)
	if hasPayable && hasCurrent && abs(payable-(current+arrears+adjust)) > 100 {
		// The payable is printed prominently and is the more reliable figure: doubt the charges,
		// and offer the charges implied by it instead.
		penalise(0.5, fmt.Sprintf("Current charges + arrears + adjustments don't equal the payable amount (off by %s); the charges were worked out from the payable.", rupees(payable-(current+arrears+adjust))),
			FCurrentCharges)
		src := r.Fields[FPayable].Source
		delete(r.Fields, FCurrentCharges)
		r.setMoney(FCurrentCharges, payable-arrears-adjust, 0.75, src+"+derived")
	}

	// A late amount smaller than the payable is a surcharge printed on its own; turn it into the total.
	if hasPayable {
		for i, lf := range r.LateFees {
			if lf.AmountMinor > 0 && lf.AmountMinor < payable {
				r.LateFees[i].AmountMinor += payable
			}
		}
		if len(r.LateFees) > 0 {
			if after, ok := r.Money(FPayableAfterDue); !ok || after < payable {
				f := r.Fields[FPayableAfterDue]
				conf := f.Confidence
				if conf == 0 {
					conf = 0.75
				}
				delete(r.Fields, FPayableAfterDue)
				r.setMoney(FPayableAfterDue, r.LateFees[0].AmountMinor, conf, firstNonEmpty(f.Source, r.Fields[FPayable].Source))
			}
		}
	}
	if after, ok := r.Money(FPayableAfterDue); ok && hasPayable && after < payable {
		penalise(0.5, "The amount after the due date is lower than the amount within it.", FPayableAfterDue)
	}

	if len(r.LineItems) > 1 && hasCurrent {
		var sum int64
		for _, li := range r.LineItems {
			sum += li.AmountMinor
		}
		if abs(sum-current) > 100 {
			r.Warnings = append(r.Warnings, Warning{Message: "The charge breakdown doesn't add up to the current charges; check it on the bill."})
		}
	}

	prev, hasPrev := r.Number(FPreviousReading)
	cur, hasCur := r.Number(FCurrentReading)
	if hasPrev && hasCur {
		if cur < prev {
			penalise(0.6, "The present reading is lower than the previous one (was the meter replaced?).", FCurrentReading, FPreviousReading)
		} else if units, ok := r.Number(FUnits); ok {
			mf := 1.0
			if v, ok := r.Number(FMultiplyingFactor); ok && v > 0 {
				mf = v
			}
			if math.Abs((cur-prev)*mf-units) > 1 {
				penalise(0.5, fmt.Sprintf("Readings × multiplying factor (%g) don't match the units printed (%g).", (cur-prev)*mf, units),
					FUnits, FCurrentReading, FPreviousReading)
			}
		}
	}

	issue, hasIssue := parseISO(r.Text(FIssueDate))
	due, hasDue := parseISO(r.Text(FDueDate))
	if hasIssue && hasDue {
		days := due.Sub(issue).Hours() / 24
		if days < 0 {
			penalise(0.5, "The due date is before the issue date.", FDueDate, FIssueDate)
		} else if days < 5 || days > 30 {
			penalise(0.8, fmt.Sprintf("The due date is %.0f days after the issue date (usually 5–30).", days), FDueDate)
		}
	}
	if month, ok := parseISO(r.Text(FBillingMonth) + "-01"); ok {
		ref, has := issue, hasIssue
		if !has {
			ref, has = due, hasDue
		}
		if has {
			diff := (ref.Year()-month.Year())*12 + int(ref.Month()) - int(month.Month())
			if diff < -1 || diff > 2 {
				penalise(0.6, "The billing month is far from the issue date.", FBillingMonth)
			}
		}
	}

	if ref := r.Text(FReference); ref != "" && ctx.ReferencePattern != "" {
		if re, err := regexp.Compile(ctx.ReferencePattern); err == nil && !re.MatchString(ref) {
			penalise(0.5, "The reference number doesn't match this provider's format.", FReference)
		}
	}

	for name, f := range r.Fields {
		if f.Confidence < DropBelow {
			delete(r.Fields, name)
			r.Warnings = append(r.Warnings, Warning{Field: name, Message: "Not read reliably; please type it in."})
		}
	}
}

// CrossCheck lowers the confidence of AI money values that don't appear in the document's
// own text (docs/05 §6, "AI value doesn't appear in OCR raw text").
func CrossCheck(r *Result, rawText string) {
	if rawText == "" {
		return
	}
	digits := onlyDigits(rawText)
	for _, name := range []string{FPayable, FPayableAfterDue, FCurrentCharges, FArrears} {
		f, ok := r.Fields[name]
		if !ok || !strings.HasPrefix(f.Source, "gemini") {
			continue
		}
		minor, _ := r.Money(name)
		if minor == 0 {
			continue
		}
		whole := fmt.Sprint(abs(minor) / 100)
		if !strings.Contains(digits, whole) {
			f.Confidence = math.Round(f.Confidence*0.6*100) / 100
			r.Fields[name] = f
			r.Warnings = append(r.Warnings, Warning{Field: name, Message: "This amount doesn't appear in the document text; check it."})
		}
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r != ',' {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

func parseISO(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func rupees(minor int64) string {
	return fmt.Sprintf("Rs %d", abs(minor)/100)
}
