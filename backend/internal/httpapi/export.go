package httpapi

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// ExportBillsCsv is FR-EXP-01: bills in a date range (by bill date) as CSV.
func (s *Server) ExportBillsCsv(w http.ResponseWriter, r *http.Request, params apigen.ExportBillsCsvParams) {
	a := mustAuth(r)
	ctx := r.Context()
	if err := checkRange(params.From, params.To); err != nil {
		s.fail(w, r, err)
		return
	}
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rows, err := s.db.ExportBills(ctx, store.ExportBillsParams{
		Today: s.today(hh.Timezone), HouseholdID: a.HouseholdID, FromDate: fromDatePtr(params.From), ToDate: fromDatePtr(params.To),
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	cw := startCSV(w, "bills", params.From, params.To)
	_ = cw.Write([]string{
		"Profile", "Bill type", "Provider", "Billing month", "Bill date", "Description", "Bill number", "Issue date", "Due date",
		"Current charges", "Arrears", "Adjustments", "Payable within due date", "Payable after due date", "Paid", "Outstanding",
		"Status", "Units", "Currency", "Archived", "Notes",
	})
	for _, row := range rows {
		b := row.Bill
		_ = cw.Write([]string{
			cell(row.ProfileName), cell(row.TypeName), cell(deref(row.ProviderName)), deref(b.PeriodKey), dateString(b.ExpenseDate),
			cell(deref(b.Description)), cell(deref(b.BillNumber)), optDate(b.IssueDate), optDate(b.DueDate),
			rupeesCSV(b.CurrentChargesMinor), rupeesCSV(b.ArrearsMinor), rupeesCSV(b.AdjustmentsMinor), rupeesCSV(b.PayableMinor),
			optRupees(b.PayableAfterDueMinor), rupeesCSV(b.PaidMinor), rupeesCSV(row.OutstandingMinor),
			statusWord(row.Status), optNumber(b.Units), b.Currency, yesNo(b.ArchivedAt != nil), cell(deref(b.Notes)),
		})
	}
	cw.Flush()
}

// ExportPaymentsCsv is FR-EXP-01: payments in a date range (by payment date) as CSV.
func (s *Server) ExportPaymentsCsv(w http.ResponseWriter, r *http.Request, params apigen.ExportPaymentsCsvParams) {
	a := mustAuth(r)
	if err := checkRange(params.From, params.To); err != nil {
		s.fail(w, r, err)
		return
	}
	rows, err := s.db.ExportPayments(r.Context(), store.ExportPaymentsParams{
		HouseholdID: a.HouseholdID, FromDate: fromDatePtr(params.From), ToDate: fromDatePtr(params.To),
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	cw := startCSV(w, "payments", params.From, params.To)
	_ = cw.Write([]string{"Paid on", "Profile", "Bill", "Amount", "Method", "Reference", "Notes"})
	for _, p := range rows {
		bill := deref(p.PeriodKey)
		if bill == "" {
			bill = firstNonBlank(deref(p.BillDescription), dateString(p.BillDate))
		}
		_ = cw.Write([]string{
			dateString(p.PaidOn), cell(p.ProfileName), cell(bill), rupeesCSV(p.AmountMinor), methodWord(string(p.Method)),
			cell(deref(p.Reference)), cell(deref(p.Notes)),
		})
	}
	cw.Flush()
}

// backupFormat versions the backup layout so a future import (FR-EXP-03) can read old files.
const backupFormat = 1

// ExportBackup is FR-EXP-02: a ZIP of the household's data as JSON plus every document.
func (s *Server) ExportBackup(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	ctx := r.Context()
	if a.Role != store.MemberRoleOwner {
		s.fail(w, r, errForbidden("Only the household owner can download a full backup."))
		return
	}
	if !s.backupsByHH.Allow(a.HouseholdID.String()) {
		s.fail(w, r, errTooManyRequests)
		return
	}
	data, docs, err := s.backupData(ctx, a)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="bills-backup-%s.zip"`, dateString(s.now())))
	w.Header().Set("Cache-Control", "no-store")
	zw := zip.NewWriter(w)
	defer zw.Close()

	writeJSONFile := func(name string, v any) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	for _, part := range []struct {
		name string
		v    any
	}{
		{"household.json", data.household},
		{"profiles.json", data.profiles},
		{"bills.json", data.bills},
		{"payments.json", data.payments},
		{"documents.json", data.documents},
		{"types.json", data.types},
		{"providers.json", data.providers},
		{"history.json", data.history},
	} {
		if err := writeJSONFile(part.name, part.v); err != nil {
			s.log.ErrorContext(ctx, "backup: write json", "file", part.name, "err", err)
			return
		}
	}
	if f, err := zw.Create("README.txt"); err == nil {
		_, _ = io.WriteString(f, backupReadme(data.household.Name, s.now()))
	}

	// Documents are streamed one by one; a missing object is noted rather than failing the backup.
	var missing []string
	for _, d := range docs {
		name := path.Join("documents", d.ID.String()+"-"+safeFileName(d.FileName))
		if err := s.copyDocument(ctx, zw, name, d); err != nil {
			s.log.WarnContext(ctx, "backup: document", "document_id", d.ID, "err", err)
			missing = append(missing, d.FileName)
		}
	}
	if len(missing) > 0 {
		if f, err := zw.Create("documents/MISSING.txt"); err == nil {
			_, _ = io.WriteString(f, "These documents could not be read from storage:\n"+strings.Join(missing, "\n")+"\n")
		}
	}
}

type backupJSON struct {
	household backupHousehold
	profiles  []apigen.Profile
	bills     []apigen.Bill
	payments  []backupPayment
	documents []backupDocument
	types     []apigen.BillType
	providers []apigen.Provider
	history   []backupHistory
}

type backupHousehold struct {
	Format     int       `json:"format"`
	ExportedAt time.Time `json:"exportedAt"`
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Currency   string    `json:"currency"`
	Timezone   string    `json:"timezone"`
}

type backupPayment struct {
	apigen.Payment
	BillID string `json:"billId"`
}

type backupDocument struct {
	ID          string  `json:"id"`
	BillID      *string `json:"billId,omitempty"`
	PaymentID   *string `json:"paymentId,omitempty"`
	Kind        string  `json:"kind"`
	Source      string  `json:"source"`
	FileName    string  `json:"fileName"`
	ContentType string  `json:"contentType"`
	SizeBytes   int64   `json:"sizeBytes"`
	Path        string  `json:"path"`
	CreatedAt   string  `json:"createdAt"`
}

type backupHistory struct {
	ProfileID   string   `json:"profileId"`
	Month       string   `json:"month"`
	Units       *float64 `json:"units,omitempty"`
	BilledMinor *int64   `json:"billedMinor,omitempty"`
	PaidMinor   *int64   `json:"paidMinor,omitempty"`
}

func (s *Server) backupData(ctx context.Context, a authInfo) (backupJSON, []store.Document, error) {
	var out backupJSON
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		return out, nil, err
	}
	out.household = backupHousehold{
		Format: backupFormat, ExportedAt: s.now(), ID: hh.ID.String(), Name: hh.Name, Currency: hh.Currency, Timezone: hh.Timezone,
	}

	profiles, err := s.db.ListProfiles(ctx, store.ListProfilesParams{HouseholdID: a.HouseholdID, IncludeArchived: true})
	if err != nil {
		return out, nil, err
	}
	out.profiles = make([]apigen.Profile, 0, len(profiles))
	for _, p := range profiles {
		out.profiles = append(out.profiles, toProfile(p, s.refDisplay(ctx, p), apigen.ProfileSummary{}))
	}

	bills, err := s.db.ExportBills(ctx, store.ExportBillsParams{Today: s.today(hh.Timezone), HouseholdID: a.HouseholdID})
	if err != nil {
		return out, nil, err
	}
	out.bills = make([]apigen.Bill, 0, len(bills))
	for _, row := range bills {
		out.bills = append(out.bills, toBill(billView{Bill: row.Bill, Status: row.Status, OutstandingMinor: row.OutstandingMinor}))
	}

	payments, err := s.db.ExportPayments(ctx, store.ExportPaymentsParams{HouseholdID: a.HouseholdID})
	if err != nil {
		return out, nil, err
	}
	out.payments = make([]backupPayment, 0, len(payments))
	for _, p := range payments {
		out.payments = append(out.payments, backupPayment{
			Payment: toPayment(store.Payment{
				ID: p.ID, HouseholdID: p.HouseholdID, BillID: p.BillID, PaidOn: p.PaidOn, AmountMinor: p.AmountMinor, Method: p.Method,
				Reference: p.Reference, Notes: p.Notes, CreatedBy: p.CreatedBy, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
			}),
			BillID: p.BillID.String(),
		})
	}

	docs, err := s.db.ExportDocuments(ctx, a.HouseholdID)
	if err != nil {
		return out, nil, err
	}
	out.documents = make([]backupDocument, 0, len(docs))
	for _, d := range docs {
		bd := backupDocument{
			ID: d.ID.String(), Kind: string(d.Kind), Source: string(d.Source), FileName: d.FileName, ContentType: d.ContentType,
			SizeBytes: d.SizeBytes, Path: path.Join("documents", d.ID.String()+"-"+safeFileName(d.FileName)),
			CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
		}
		if d.BillID != nil {
			bd.BillID = ptr(d.BillID.String())
		}
		if d.PaymentID != nil {
			bd.PaymentID = ptr(d.PaymentID.String())
		}
		out.documents = append(out.documents, bd)
	}

	types, err := s.db.ExportCustomTypes(ctx, &a.HouseholdID)
	if err != nil {
		return out, nil, err
	}
	out.types = make([]apigen.BillType, 0, len(types))
	for _, t := range types {
		out.types = append(out.types, toBillType(t))
	}
	providers, err := s.db.ExportCustomProviders(ctx, &a.HouseholdID)
	if err != nil {
		return out, nil, err
	}
	out.providers = make([]apigen.Provider, 0, len(providers))
	for _, p := range providers {
		out.providers = append(out.providers, toProvider(p))
	}
	history, err := s.db.ExportProfileHistory(ctx, a.HouseholdID)
	if err != nil {
		return out, nil, err
	}
	out.history = make([]backupHistory, 0, len(history))
	for _, h := range history {
		out.history = append(out.history, backupHistory{
			ProfileID: h.ProfileID.String(), Month: h.PeriodKey, Units: h.Units, BilledMinor: h.BilledMinor, PaidMinor: h.PaidMinor,
		})
	}
	return out, docs, nil
}

func (s *Server) copyDocument(ctx context.Context, zw *zip.Writer, name string, d store.Document) error {
	rc, err := s.files.Open(ctx, d.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()
	// Photos and PDFs are already compressed; store them as they are.
	f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: d.CreatedAt})
	if err != nil {
		return err
	}
	_, err = io.Copy(f, rc)
	return err
}

func backupReadme(household string, at time.Time) string {
	return fmt.Sprintf(`Bill Manager backup — %s
Made on %s.

household.json  household settings
profiles.json   bill profiles (includes reference numbers: keep this file private)
bills.json      every bill, with status and amounts in paisa (divide by 100 for rupees)
payments.json   every payment
documents.json  the list of documents; the files are in documents/
types.json      bill types you created
providers.json  providers you added
history.json    month-by-month history read from your bills
`, household, at.Format("2 January 2006 15:04 MST"))
}

// --- CSV helpers ----------------------------------------------------------------

func startCSV(w http.ResponseWriter, what string, from, to *openapi_types.Date) *csv.Writer {
	name := what
	if from != nil {
		name += "-from-" + from.String()
	}
	if to != nil {
		name += "-to-" + to.String()
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, name))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, "\xef\xbb\xbf") // BOM, so Excel opens it as UTF-8
	return csv.NewWriter(w)
}

func checkRange(from, to *openapi_types.Date) error {
	if from != nil && to != nil && to.Time.Before(from.Time) {
		return errField("to", "The end date is before the start date.")
	}
	return nil
}

// cell neutralises text that a spreadsheet would run as a formula (CSV injection).
func cell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func rupeesCSV(minor int64) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}

func optRupees(v *int64) string {
	if v == nil {
		return ""
	}
	return rupeesCSV(*v)
}

func optDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return dateString(*t)
}

func optNumber(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func statusWord(st string) string {
	switch st {
	case "partially_paid":
		return "Partly paid"
	case "draft":
		return "Needs review"
	}
	return strings.ToUpper(st[:1]) + st[1:]
}

func methodWord(m string) string {
	switch m {
	case "bank_transfer":
		return "Bank transfer"
	case "jazzcash":
		return "JazzCash"
	case "easypaisa":
		return "Easypaisa"
	}
	return strings.ToUpper(m[:1]) + m[1:]
}

func firstNonBlank(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._\- ]+`)

func safeFileName(name string) string {
	n := strings.TrimSpace(unsafeName.ReplaceAllString(path.Base(strings.ReplaceAll(name, `\`, "/")), "_"))
	if n == "" || n == "." || n == ".." {
		return "file"
	}
	return n
}
