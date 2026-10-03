package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/config"
	"github.com/abdullah-dev5/bill-management/backend/internal/mail"
	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
	"github.com/abdullah-dev5/bill-management/backend/internal/storage"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
	"github.com/abdullah-dev5/bill-management/backend/migrations"
)

var testDB *store.DB

const testTickToken = "test-tick-token-0123456789abcdef0123"

var testKeys = func() *secure.Keys {
	k, err := secure.New("test-data-key-0123456789abcdef0123456789")
	if err != nil {
		panic(err)
	}
	return k
}()

// TestMain resets the test database schema and applies all migrations once.
func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/bill_test?sslmode=disable"
	}
	ctx := context.Background()
	if err := resetSchema(ctx, url); err != nil {
		fmt.Fprintf(os.Stderr, "test database unavailable (%s): %v\nRun `go run ./cmd/devdb` or set TEST_DATABASE_URL.\n", url, err)
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	testDB = store.NewDB(pool)
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func resetSchema(ctx context.Context, url string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;"); err != nil {
		conn.Close(ctx)
		return err
	}
	conn.Close(ctx)
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	return migrations.Run(ctx, db, "up", testKeys)
}

// fakeMailer records sent messages.
type fakeMailer struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (f *fakeMailer) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeMailer) last() mail.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return mail.Message{}
	}
	return f.sent[len(f.sent)-1]
}

// env is one API server with a controllable clock.
type env struct {
	t      *testing.T
	server *Server
	http   *httptest.Server
	mailer *fakeMailer
	files  *storage.Local
	clock  atomic.Int64
}

// 3 Oct 2026, 10:00 UTC (15:00 in Asia/Karachi).
var defaultNow = time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, mailer: &fakeMailer{}}
	e.clock.Store(defaultNow.UnixNano())
	cfg := config.Config{Env: "test", AppBaseURL: "http://app.test", TickToken: testTickToken, StorageQuotaBytes: 50 << 20}
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e.files = files
	e.server = New(testDB, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), e.mailer, testKeys, files)
	e.server.now = func() time.Time { return time.Unix(0, e.clock.Load()).UTC() }
	h, err := e.server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.http = httptest.NewServer(h)
	t.Cleanup(e.http.Close)
	return e
}

// setDate moves the server clock to 10:00 UTC on the given date.
func (e *env) setDate(date string) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		e.t.Fatal(err)
	}
	e.clock.Store(d.Add(10 * time.Hour).UnixNano())
}

// client is a browser-like API client: cookie jar plus the CSRF header.
type client struct {
	t   *testing.T
	env *env
	c   *http.Client
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	cl := &client{t: e.t, env: e, c: &http.Client{Jar: jar}}
	cl.do(http.MethodGet, "/api/v1/auth/me", nil, nil) // obtains the CSRF cookie
	return cl
}

func (cl *client) csrfToken() string {
	u, _ := url.Parse(cl.env.http.URL)
	for _, c := range cl.c.Jar.Cookies(u) {
		if c.Name == csrfCookie {
			return c.Value
		}
	}
	return ""
}

// do sends a JSON request and decodes a JSON response into out (if non-nil). It returns the status.
func (cl *client) do(method, path string, body any, out any) int {
	cl.t.Helper()
	return cl.doRaw(method, path, body, out, true)
}

func (cl *client) doRaw(method, path string, body any, out any, withCSRF bool) int {
	cl.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			cl.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, cl.env.http.URL+path, rd)
	if err != nil {
		cl.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withCSRF {
		req.Header.Set(csrfHeader, cl.csrfToken())
	}
	resp, err := cl.c.Do(req)
	if err != nil {
		cl.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			cl.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode
}

// must asserts the status code, printing the error body on mismatch.
func (cl *client) must(want int, method, path string, body any, out any) {
	cl.t.Helper()
	var raw json.RawMessage
	got := cl.do(method, path, body, &raw)
	if got != want {
		cl.t.Fatalf("%s %s: status %d, want %d: %s", method, path, got, want, raw)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			cl.t.Fatal(err)
		}
	}
}

var emailSeq atomic.Int64

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d-%d@example.com", prefix, time.Now().UnixNano(), emailSeq.Add(1))
}

const testPassword = "correct horse battery staple"

// signedUp returns a client signed in as a brand-new user.
func (e *env) signedUp(name string) (*client, apigen.Me) {
	e.t.Helper()
	cl := e.client()
	var me apigen.Me
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/signup", map[string]any{
		"email": uniqueEmail(strings.ToLower(name)), "password": testPassword, "displayName": name,
	}, &me)
	return cl, me
}

// Fixed IDs from the seed migration.
const (
	typeElectricity = "a0000000-0000-4000-8000-000000000001"
	typeMaintenance = "a0000000-0000-4000-8000-000000000012"
	typeElectrician = "a0000000-0000-4000-8000-000000000031"
	typeRent        = "a0000000-0000-4000-8000-000000000011"
	providerLESCO   = "b0000000-0000-4000-8000-000000000001"
)

func (cl *client) createProfile(body map[string]any) apigen.Profile {
	cl.t.Helper()
	var p apigen.Profile
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/profiles", body, &p)
	return p
}

func (cl *client) createBill(body map[string]any) apigen.BillDetail {
	cl.t.Helper()
	var b apigen.BillDetail
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/bills", body, &b)
	return b
}

func (cl *client) getBill(id string) apigen.BillDetail {
	cl.t.Helper()
	var b apigen.BillDetail
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/bills/"+id, nil, &b)
	return b
}

func (cl *client) defaultProfile() apigen.Profile {
	cl.t.Helper()
	var list []apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles", nil, &list)
	for _, p := range list {
		if p.IsDefault {
			return p
		}
	}
	cl.t.Fatal("no default profile")
	return apigen.Profile{}
}
