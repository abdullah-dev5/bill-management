package extract

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

// Gemini reads a bill image or PDF with Google's Gemini API and returns our field schema
// as structured JSON (docs/05 §8).
type Gemini struct {
	APIKey string
	Model  string
	// Fallbacks are tried in order when a model is overloaded or out of quota.
	Fallbacks []string
	BaseURL   string // default https://generativelanguage.googleapis.com
	HTTP      *http.Client
}

func (g *Gemini) Name() string { return "gemini" }

const geminiSystemPrompt = `You read utility and household bills, mostly from Pakistan.
Extract only what is printed on the document. If a field is not visible, return null for it.
Never infer, compute or convert amounts: copy each value exactly as printed (e.g. "3,266", "05 OCT 26").
For every field give a confidence between 0 and 1 for how sure you are you read it correctly.
"payable_within_due_date" is the total to pay by the due date; "payable_after_due_date" is the first amount
payable after the due date. "late_fees" lists every TOTAL amount payable after the due date (the full amount to pay, not the surcharge alone),
with the date until which it applies.
"line_items" lists the individual charges that make up this period's charges (with subsidies or credits as negative
amounts); leave out subtotals, totals, arrears and amounts payable.
"history" is the table of previous months (month, units, bill amount, payment), if the bill prints one.
"current_charges" is the total of this period's charges including taxes and fuel adjustments, excluding arrears
(usually the grand total minus arrears).
"reference_number" is the account identifier: the 14-digit reference number on electricity bills, the customer or
consumer number on gas bills (leave out a check digit printed in brackets after it, e.g. "5051274880 (4)" → "5051274880").
"arrears" includes a "previous balance" carried into this bill.`

var geminiFields = map[string]string{
	"provider_name": FProvider, "reference_number": FReference, "bill_number": FBillNumber,
	"customer_name": FCustomerName, "billing_month": FBillingMonth, "period_start": FPeriodStart,
	"period_end": FPeriodEnd, "issue_date": FIssueDate, "due_date": FDueDate, "reading_date": FReadingDate,
	"previous_reading": FPreviousReading, "current_reading": FCurrentReading, "units": FUnits,
	"multiplying_factor": FMultiplyingFactor, "current_charges": FCurrentCharges, "arrears": FArrears,
	"adjustments": FAdjustments, "payable_within_due_date": FPayable, "payable_after_due_date": FPayableAfterDue,
}

// geminiFieldOrder fixes the order of the response fields (Gemini fills them in this order).
var geminiFieldOrder = []string{
	"provider_name", "reference_number", "bill_number", "customer_name", "billing_month", "period_start", "period_end",
	"issue_date", "due_date", "reading_date", "previous_reading", "current_reading", "units", "multiplying_factor",
	"current_charges", "arrears", "adjustments", "payable_within_due_date", "payable_after_due_date",
}

// geminiSchema lists every field as required but nullable: with optional fields the model
// tends to return only a few of them.
func geminiSchema() map[string]any {
	field := map[string]any{
		"type": "OBJECT", "nullable": true,
		"properties":       map[string]any{"value": map[string]any{"type": "STRING"}, "confidence": map[string]any{"type": "NUMBER"}},
		"required":         []string{"value", "confidence"},
		"propertyOrdering": []string{"value", "confidence"},
	}
	props := map[string]any{}
	for _, k := range geminiFieldOrder {
		props[k] = field
	}
	str := map[string]any{"type": "STRING", "nullable": true}
	props["late_fees"] = map[string]any{"type": "ARRAY", "items": map[string]any{
		"type": "OBJECT", "properties": map[string]any{"until": str, "amount": map[string]any{"type": "STRING"}},
		"required": []string{"until", "amount"}, "propertyOrdering": []string{"until", "amount"},
	}}
	props["line_items"] = map[string]any{"type": "ARRAY", "items": map[string]any{
		"type": "OBJECT", "properties": map[string]any{"label": map[string]any{"type": "STRING"}, "amount": map[string]any{"type": "STRING"}},
		"required": []string{"label", "amount"}, "propertyOrdering": []string{"label", "amount"},
	}}
	props["history"] = map[string]any{"type": "ARRAY", "items": map[string]any{
		"type": "OBJECT", "properties": map[string]any{"month": map[string]any{"type": "STRING"}, "units": str, "bill": str, "payment": str},
		"required": []string{"month", "units", "bill", "payment"}, "propertyOrdering": []string{"month", "units", "bill", "payment"},
	}}
	order := append(append([]string{}, geminiFieldOrder...), "late_fees", "line_items", "history")
	return map[string]any{"type": "OBJECT", "properties": props, "required": order, "propertyOrdering": order}
}

type geminiField struct {
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
}

type geminiOutput struct {
	Fields   map[string]*geminiField
	LateFees []struct {
		Until  *string `json:"until"`
		Amount string  `json:"amount"`
	} `json:"late_fees"`
	LineItems []struct {
		Label  string `json:"label"`
		Amount string `json:"amount"`
	} `json:"line_items"`
	History []struct {
		Month   string  `json:"month"`
		Units   *string `json:"units"`
		Bill    *string `json:"bill"`
		Payment *string `json:"payment"`
	} `json:"history"`
}

func (g *Gemini) Extract(ctx context.Context, in Input) (Result, error) {
	prompt := "Extract the bill fields."
	if len(in.Providers) > 0 {
		names := make([]string, 0, len(in.Providers))
		for _, p := range in.Providers {
			names = append(names, p.ShortName)
		}
		prompt += " Known providers: " + strings.Join(names, ", ") + ". Use the provider's short name if it matches."
	}
	body := map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": geminiSystemPrompt}}},
		"contents": []any{map[string]any{"role": "user", "parts": []any{
			map[string]any{"inlineData": map[string]any{"mimeType": in.ContentType, "data": base64.StdEncoding.EncodeToString(in.Data)}},
			map[string]any{"text": prompt},
		}}},
		"generationConfig": map[string]any{
			"temperature": 0, "responseMimeType": "application/json", "responseSchema": geminiSchema(),
			"maxOutputTokens": 16384,
		},
	}
	raw, err := g.call(ctx, body)
	if err != nil {
		return Result{}, err
	}
	out, err := parseGeminiOutput(raw)
	if err != nil {
		// One repair attempt (docs/05 §4.2), then give up on this tier.
		if raw, err = g.call(ctx, body); err != nil {
			return Result{}, err
		}
		if out, err = parseGeminiOutput(raw); err != nil {
			return Result{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
		}
	}
	return out.toResult(), nil
}

// call tries the model, then each fallback while the failure is an overload or a quota limit.
func (g *Gemini) call(ctx context.Context, body map[string]any) (string, error) {
	var err error
	for _, model := range append([]string{g.Model}, g.Fallbacks...) {
		var text string
		text, err = g.callModel(ctx, model, body)
		if err == nil {
			return text, nil
		}
		if !errors.Is(err, ErrTransient) && !errors.Is(err, ErrRateLimited) || ctx.Err() != nil {
			return "", err
		}
	}
	return "", err
}

func (g *Gemini) callModel(ctx context.Context, model string, body map[string]any) (string, error) {
	base := g.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1beta/models/"+model+":generateContent", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.APIKey)
	resp, err := httpClient(g.HTTP).Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTransient, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err := classifyStatus(resp.StatusCode, geminiErrorMessage(data)); err != nil {
		return "", err
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("%w: empty response", ErrInvalidOutput)
	}
	c := parsed.Candidates[0]
	if c.FinishReason != "" && c.FinishReason != "STOP" {
		return "", fmt.Errorf("%w: the response was cut off (%s)", ErrInvalidOutput, c.FinishReason)
	}
	var text strings.Builder
	for _, p := range c.Content.Parts {
		text.WriteString(p.Text)
	}
	return text.String(), nil
}

func geminiErrorMessage(data []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return strings.TrimSpace(string(data))
}

func parseGeminiOutput(text string) (geminiOutput, error) {
	var out geminiOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return out, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &all); err != nil {
		return out, err
	}
	out.Fields = map[string]*geminiField{}
	for key := range geminiFields {
		raw, ok := all[key]
		if !ok || string(raw) == "null" {
			continue
		}
		var f geminiField
		if err := json.Unmarshal(raw, &f); err != nil {
			return out, fmt.Errorf("field %s: %w", key, err)
		}
		out.Fields[key] = &f
	}
	return out, nil
}

func (o geminiOutput) toResult() Result {
	r := NewResult()
	const source = "gemini"
	for key, f := range o.Fields {
		if f == nil || strings.TrimSpace(f.Value) == "" {
			continue
		}
		name := geminiFields[key]
		conf := math.Max(0.3, math.Min(f.Confidence, 0.92))
		if v, ok := normalizeField(name, f.Value); ok {
			r.set(name, v, conf, source)
		}
	}
	for _, lf := range o.LateFees {
		amount, ok := ParseMoney(lf.Amount)
		if !ok {
			continue
		}
		fee := LateFee{AmountMinor: amount}
		if lf.Until != nil {
			fee.Until, _ = ParseDate(*lf.Until)
		}
		r.LateFees = append(r.LateFees, fee)
	}
	for _, li := range o.LineItems {
		if amount, ok := ParseMoney(li.Amount); ok && strings.TrimSpace(li.Label) != "" {
			r.LineItems = append(r.LineItems, LineItem{Label: strings.TrimSpace(li.Label), AmountMinor: amount})
		}
	}
	for _, h := range o.History {
		month, ok := ParseMonth(h.Month)
		if !ok {
			continue
		}
		row := HistoryRow{Month: month}
		if h.Units != nil {
			if v, ok := ParseNumber(*h.Units); ok {
				row.Units = &v
			}
		}
		if h.Bill != nil {
			if v, ok := ParseMoney(*h.Bill); ok {
				row.BilledMinor = &v
			}
		}
		if h.Payment != nil {
			if v, ok := ParseMoney(*h.Payment); ok {
				row.PaidMinor = &v
			}
		}
		r.History = append(r.History, row)
	}
	r.History = sortHistory(r.History)
	return r
}

// normalizeField converts a printed value to the canonical format for its field.
func normalizeField(name, value string) (string, bool) {
	switch name {
	case FCurrentCharges, FArrears, FAdjustments, FPayable, FPayableAfterDue:
		v, ok := ParseMoney(value)
		return itoa64(v), ok
	case FIssueDate, FDueDate, FReadingDate, FPeriodStart, FPeriodEnd:
		return ParseDate(value)
	case FBillingMonth:
		if m, ok := ParseMonth(value); ok {
			return m, true
		}
		if d, ok := ParseDate(value); ok {
			return d[:7], true
		}
		return "", false
	case FPreviousReading, FCurrentReading, FUnits, FMultiplyingFactor:
		v, ok := ParseNumber(value)
		return formatNumber(v), ok
	case FReference, FBillNumber:
		v := NormalizeReference(value)
		return v, v != ""
	default:
		v := strings.TrimSpace(value)
		return v, v != ""
	}
}
