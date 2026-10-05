package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

// get fetches a non-JSON response.
func (cl *client) get(path string) (int, http.Header, []byte) {
	cl.t.Helper()
	resp, err := cl.c.Get(cl.env.http.URL + path)
	if err != nil {
		cl.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, body
}

func readCSV(t *testing.T, body []byte) [][]string {
	t.Helper()
	if !bytes.HasPrefix(body, []byte("\xef\xbb\xbf")) {
		t.Error("CSV should start with a UTF-8 BOM for Excel")
	}
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestExportBillsAndPaymentsCSV(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Export")
	elec := cl.createProfile(map[string]any{"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO})
	cl.createBill(map[string]any{"profileId": elec.Id, "periodKey": "2025-12", "currentChargesMinor": 300050})
	sep := cl.createBill(map[string]any{
		"profileId": elec.Id, "periodKey": "2026-09", "dueDate": "2026-10-05", "currentChargesMinor": 420000, "arrearsMinor": 10000,
		"notes": "=HYPERLINK(\"http://evil\")",
	})
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills/"+sep.Id.String()+"/payments",
		map[string]any{"paidOn": "2026-10-01", "amountMinor": 200000, "method": "bank_transfer", "reference": "TRX-991"}, nil)

	status, hdr, body := cl.get("/api/v1/export/bills.csv?from=2026-01-01&to=2026-12-31")
	if status != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") ||
		!strings.Contains(hdr.Get("Content-Disposition"), `filename="bills-from-2026-01-01-to-2026-12-31.csv"`) {
		t.Fatalf("status %d, headers %v", status, hdr)
	}
	rows := readCSV(t, body)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	head, row := rows[0], rows[1]
	col := func(name string) string {
		for i, h := range head {
			if h == name {
				return row[i]
			}
		}
		t.Fatalf("no column %q", name)
		return ""
	}
	for name, want := range map[string]string{
		"Profile": "Home Electricity", "Bill type": "Electricity", "Provider": "LESCO", "Billing month": "2026-09",
		"Due date": "2026-10-05", "Current charges": "4200.00", "Arrears": "100.00", "Payable within due date": "4300.00",
		"Paid": "2000.00", "Outstanding": "2300.00", "Status": "Partly paid",
	} {
		if got := col(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if got := col("Notes"); !strings.HasPrefix(got, "'=") {
		t.Errorf("a formula in notes must be neutralised, got %q", got)
	}

	// No range: every bill, oldest first.
	_, _, body = cl.get("/api/v1/export/bills.csv")
	if rows := readCSV(t, body); len(rows) != 3 || rows[1][3] != "2025-12" || rows[1][9] != "3000.50" {
		t.Errorf("all bills = %v", rows)
	}

	_, _, body = cl.get("/api/v1/export/payments.csv?from=2026-10-01")
	pays := readCSV(t, body)
	if len(pays) != 2 || pays[1][0] != "2026-10-01" || pays[1][2] != "2026-09" || pays[1][3] != "2000.00" || pays[1][4] != "Bank transfer" || pays[1][5] != "TRX-991" {
		t.Errorf("payments = %v", pays)
	}

	if status, _, _ := cl.get("/api/v1/export/bills.csv?from=2026-12-01&to=2026-01-01"); status != http.StatusBadRequest {
		t.Errorf("reversed range: %d", status)
	}

	// Another household sees none of it.
	other, _ := e.signedUp("ExportOther")
	if _, _, body := other.get("/api/v1/export/bills.csv"); len(readCSV(t, body)) != 1 {
		t.Error("export leaked across households")
	}
}

func TestBackupZIP(t *testing.T) {
	e := newEnv(t)
	cl, me := e.signedUp("Backup")
	elec := cl.createProfile(map[string]any{
		"name": "Home Electricity", "typeId": typeElectricity, "providerId": providerLESCO, "referenceNumber": "09 12345 0012345",
	})
	bill := cl.createBill(map[string]any{"profileId": elec.Id, "periodKey": "2026-09", "currentChargesMinor": 420000,
		"paidNow": map[string]any{"paidOn": "2026-09-20", "method": "cash"}})
	doc := cl.upload(fakeJPEG, "image/jpeg", map[string]any{"billId": bill.Id, "fileName": `C:\Users\me\..\bill sep.jpg`})

	status, hdr, body := cl.get("/api/v1/export/backup.zip")
	if status != http.StatusOK || hdr.Get("Content-Type") != "application/zip" {
		t.Fatalf("status %d %s: %s", status, hdr.Get("Content-Type"), body)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	var names []string
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name], _ = io.ReadAll(rc)
		rc.Close()
		names = append(names, f.Name)
	}
	sort.Strings(names)
	for _, want := range []string{"README.txt", "household.json", "profiles.json", "bills.json", "payments.json", "documents.json", "types.json", "providers.json", "history.json"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s (have %v)", want, names)
		}
	}
	docPath := "documents/" + doc.Id.String() + "-bill sep.jpg"
	if got := files[docPath]; !bytes.Equal(got, fakeJPEG) {
		t.Errorf("document %s: %d bytes (have %v)", docPath, len(got), names)
	}

	var profiles []apigen.Profile
	if err := json.Unmarshal(files["profiles.json"], &profiles); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range profiles {
		if p.Id == elec.Id {
			found = p.ReferenceNumber != nil && *p.ReferenceNumber == "09 12345 0012345"
		}
	}
	if !found {
		t.Errorf("the backup should carry the reference number: %s", files["profiles.json"])
	}
	var bills []apigen.Bill
	var payments []map[string]any
	_ = json.Unmarshal(files["bills.json"], &bills)
	_ = json.Unmarshal(files["payments.json"], &payments)
	if len(bills) != 1 || bills[0].Status != apigen.Paid || len(payments) != 1 || payments[0]["billId"] != bill.Id.String() {
		t.Errorf("bills %+v payments %+v", bills, payments)
	}
	var hh map[string]any
	_ = json.Unmarshal(files["household.json"], &hh)
	if hh["format"] != float64(1) {
		t.Errorf("household.json = %s", files["household.json"])
	}

	// Owners only.
	if _, err := testDB.Pool.Exec(context.Background(), "UPDATE memberships SET role = 'editor' WHERE user_id = $1", me.User.Id); err != nil {
		t.Fatal(err)
	}
	if status, _, _ := cl.get("/api/v1/export/backup.zip"); status != http.StatusForbidden {
		t.Errorf("editor backup: %d", status)
	}
	// CSV export stays open to every member.
	if status, _, _ := cl.get("/api/v1/export/bills.csv"); status != http.StatusOK {
		t.Errorf("editor CSV: %d", status)
	}
}

// NFR-01: the dashboard and lists answer in < 300 ms server-side for a household with years of bills.
func TestDashboardSpeed(t *testing.T) {
	if testing.Short() {
		t.Skip("seeds several hundred bills")
	}
	e := newEnv(t)
	cl, me := e.signedUp("Speed")
	seedYears(t, cl, me, 5)

	paths := []string{
		"/api/v1/home",
		"/api/v1/bills",
		"/api/v1/bills?status=unpaid&status=overdue",
		"/api/v1/analytics?from=2025-10&to=2026-09",
		"/api/v1/analytics?from=2023-10&to=2026-09",
		"/api/v1/search?q=" + "electricity%20september%202025",
		"/api/v1/search?q=kitchen",
		"/api/v1/activity",
		"/api/v1/profiles",
	}
	for _, p := range paths {
		var took []time.Duration
		for i := 0; i < 12; i++ {
			start := time.Now()
			if status := cl.do(http.MethodGet, p, nil, nil); status != http.StatusOK {
				t.Fatalf("%s: %d", p, status)
			}
			took = append(took, time.Since(start))
		}
		sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
		p95 := took[len(took)*95/100]
		t.Logf("%-50s median %6.1f ms  p95 %6.1f ms", p, ms(took[len(took)/2]), ms(p95))
		if p95 > 300*time.Millisecond {
			t.Errorf("%s: p95 %v over the 300 ms budget", p, p95)
		}
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// seedYears inserts years of monthly bills for six profiles (plus payments), straight into the database.
func seedYears(t *testing.T, cl *client, me apigen.Me, years int) {
	t.Helper()
	types := []string{typeElectricity, typeGas, typeRent, "a0000000-0000-4000-8000-000000000003", "a0000000-0000-4000-8000-000000000004", typeMaintenance}
	names := []string{"Home Electricity", "Kitchen Gas", "Flat Rent", "Water", "Internet", "Repairs"}
	var profiles []apigen.Profile
	for i, ty := range types {
		profiles = append(profiles, cl.createProfile(map[string]any{"name": names[i], "typeId": ty}))
	}
	ctx := context.Background()
	start := time.Date(2026-years, 10, 1, 0, 0, 0, 0, time.UTC)
	n := 0
	for m := start; m.Before(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); m = m.AddDate(0, 1, 0) {
		for i, p := range profiles {
			amount := int64(100000 + 25000*i + int(m.Month())*1000)
			period := m.Format("2006-01")
			var periodArg any = period
			if types[i] == typeMaintenance {
				periodArg = nil
			}
			paid := int64(0)
			if m.Before(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
				paid = amount
			}
			_, err := testDB.Pool.Exec(ctx, `
				INSERT INTO bills (id, household_id, profile_id, category_id, description, period_key, due_date, expense_date,
				                   current_charges_minor, payable_minor, currency, paid_minor, paid_on_time_minor, units, notes)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $8, 'PKR', $9, $9, $10, $11)`,
				me.Household.Id, p.Id, types[i], ifThen(periodArg == nil, "Kitchen tap", ""), periodArg, m.AddDate(0, 0, 20), m,
				amount, paid, float64(150+i*10), "seeded")
			if err != nil {
				t.Fatal(err)
			}
			n++
		}
	}
	if _, err := testDB.Pool.Exec(ctx, `
		INSERT INTO payments (id, household_id, bill_id, paid_on, amount_minor, method)
		SELECT gen_random_uuid(), household_id, id, expense_date + 15, paid_minor, 'cash' FROM bills
		WHERE household_id = $1 AND paid_minor > 0`, me.Household.Id); err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d bills", n)
}

func ifThen(cond bool, a, b string) any {
	if cond {
		return a
	}
	if b == "" {
		return nil
	}
	return b
}
