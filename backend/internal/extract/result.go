// Package extract turns bill documents into structured fields (docs/05).
//
// Values are normalised strings: money in integer minor units ("326600"), dates as
// YYYY-MM-DD, billing months as YYYY-MM, readings and units as decimal numbers.
package extract

import (
	"sort"
	"strconv"
)

// Canonical field names.
const (
	FProvider          = "providerName"
	FReference         = "referenceNumber"
	FBillNumber        = "billNumber"
	FCustomerName      = "customerName" // used for redaction only; never stored
	FBillingMonth      = "billingMonth"
	FPeriodStart       = "periodStart"
	FPeriodEnd         = "periodEnd"
	FIssueDate         = "issueDate"
	FDueDate           = "dueDate"
	FReadingDate       = "readingDate"
	FPreviousReading   = "previousReading"
	FCurrentReading    = "currentReading"
	FUnits             = "units"
	FMultiplyingFactor = "multiplyingFactor"
	FCurrentCharges    = "currentChargesMinor"
	FArrears           = "arrearsMinor"
	FAdjustments       = "adjustmentsMinor"
	FPayable           = "payableMinor"
	FPayableAfterDue   = "payableAfterDueMinor"
)

// CriticalFields must be present for a result to count as complete (docs/05 §4.2).
var CriticalFields = []string{FPayable, FDueDate, FReference}

// Field is one extracted value with its confidence (0–1) and where it came from.
type Field struct {
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

type LineItem struct {
	Label       string `json:"label"`
	AmountMinor int64  `json:"amountMinor"`
}

// LateFee is a stepped amount payable after the due date; Until is empty for the last step.
type LateFee struct {
	Until       string `json:"until,omitempty"`
	AmountMinor int64  `json:"amountMinor"`
}

// HistoryRow is one month of the consumption/billing history printed on many bills.
type HistoryRow struct {
	Month       string   `json:"month"`
	Units       *float64 `json:"units,omitempty"`
	BilledMinor *int64   `json:"billedMinor,omitempty"`
	PaidMinor   *int64   `json:"paidMinor,omitempty"`
}

type Warning struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Result is what an extraction produces.
type Result struct {
	Fields    map[string]Field `json:"fields"`
	LineItems []LineItem       `json:"lineItems"`
	LateFees  []LateFee        `json:"lateFees"`
	History   []HistoryRow     `json:"history"`
	Warnings  []Warning        `json:"warnings"`
	// RawText is the document text (from the PDF or OCR), redacted before storage.
	RawText string `json:"-"`
}

func NewResult() Result {
	return Result{Fields: map[string]Field{}, LineItems: []LineItem{}, LateFees: []LateFee{}, History: []HistoryRow{}, Warnings: []Warning{}}
}

func (r *Result) set(name, value string, confidence float64, source string) {
	if value == "" {
		return
	}
	if cur, ok := r.Fields[name]; ok && cur.Confidence >= confidence {
		return
	}
	r.Fields[name] = Field{Value: value, Confidence: confidence, Source: source}
}

func (r *Result) setMoney(name string, minor int64, confidence float64, source string) {
	r.set(name, strconv.FormatInt(minor, 10), confidence, source)
}

func (r *Result) setNumber(name string, v float64, confidence float64, source string) {
	r.set(name, strconv.FormatFloat(v, 'f', -1, 64), confidence, source)
}

// Money returns a money field in minor units.
func (r Result) Money(name string) (int64, bool) {
	f, ok := r.Fields[name]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseInt(f.Value, 10, 64)
	return v, err == nil
}

// Number returns a numeric field.
func (r Result) Number(name string) (float64, bool) {
	f, ok := r.Fields[name]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(f.Value, 64)
	return v, err == nil
}

// Text returns a field's value, or "".
func (r Result) Text(name string) string { return r.Fields[name].Value }

// Score is the mean confidence of the critical fields (missing counts as 0).
func (r Result) Score() float64 {
	sum := 0.0
	for _, f := range CriticalFields {
		sum += r.Fields[f].Confidence
	}
	return sum / float64(len(CriticalFields))
}

// Complete reports whether every critical field is present and the score is at least 0.6.
func (r Result) Complete() bool {
	for _, f := range CriticalFields {
		if _, ok := r.Fields[f]; !ok {
			return false
		}
	}
	return r.Score() >= 0.6
}

// Merge fills r from other field by field: the higher confidence wins (docs/05 §4.2).
func (r *Result) Merge(other Result) {
	for name, f := range other.Fields {
		r.set(name, f.Value, f.Confidence, f.Source)
	}
	if len(r.LineItems) == 0 {
		r.LineItems = other.LineItems
	}
	if len(r.LateFees) == 0 {
		r.LateFees = other.LateFees
	}
	if len(other.History) > len(r.History) {
		r.History = other.History
	}
	if r.RawText == "" {
		r.RawText = other.RawText
	}
}

func sortHistory(h []HistoryRow) []HistoryRow {
	seen := map[string]bool{}
	out := h[:0]
	for _, row := range h {
		if !seen[row.Month] {
			seen[row.Month] = true
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Month < out[j].Month })
	return out
}
