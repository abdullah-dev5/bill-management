package httpapi

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

func TestSignupCreatesHouseholdDefaults(t *testing.T) {
	e := newEnv(t)
	cl, me := e.signedUp("Ayesha")

	if me.Role != apigen.Owner {
		t.Fatalf("role = %s, want owner", me.Role)
	}
	if me.Household.Currency != "PKR" || me.Household.Timezone != "Asia/Karachi" {
		t.Fatalf("household defaults = %+v", me.Household)
	}

	var profiles []apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles", nil, &profiles)
	if len(profiles) != 1 {
		t.Fatalf("want exactly the default profile, got %d", len(profiles))
	}
	p := profiles[0]
	if !p.IsDefault || p.Name != "General expenses" || p.Kind != apigen.ExpenseLog {
		t.Fatalf("default profile = %+v", p)
	}

	var types []apigen.BillType
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/types", nil, &types)
	found := false
	for _, ty := range types {
		if ty.Id.String() == typeElectricity {
			found = ty.Group == apigen.TypeGroupUtilities && ty.DefaultKind == apigen.Metered && ty.IsSystem
		}
	}
	if !found {
		t.Fatal("system Electricity type missing or wrong")
	}
}

func TestSignupValidationAndDuplicateEmail(t *testing.T) {
	e := newEnv(t)
	cl := e.client()

	var errBody apigen.Error
	status := cl.do(http.MethodPost, "/api/v1/auth/signup", map[string]any{
		"email": "not-an-email", "password": "short", "displayName": "X",
	}, &errBody)
	if status != http.StatusBadRequest || errBody.Fields == nil {
		t.Fatalf("status %d body %+v", status, errBody)
	}
	if _, ok := (*errBody.Fields)["password"]; !ok {
		t.Fatalf("expected a password field error, got %v", *errBody.Fields)
	}

	email := uniqueEmail("dup")
	body := map[string]any{"email": email, "password": testPassword, "displayName": "Dup"}
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/signup", body, nil)
	other := e.client()
	if got := other.do(http.MethodPost, "/api/v1/auth/signup", body, &errBody); got != http.StatusConflict || errBody.Code != "email_taken" {
		t.Fatalf("duplicate signup: %d %+v", got, errBody)
	}
}

func TestLoginLogoutAndAuthRequired(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	if got := anon.do(http.MethodGet, "/api/v1/profiles", nil, nil); got != http.StatusUnauthorized {
		t.Fatalf("anonymous profiles: %d", got)
	}

	email := uniqueEmail("login")
	anon.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/signup",
		map[string]any{"email": email, "password": testPassword, "displayName": "Login"}, nil)
	anon.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)
	if got := anon.do(http.MethodGet, "/api/v1/auth/me", nil, nil); got != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", got)
	}

	var errBody apigen.Error
	if got := anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": "wrong password!"}, &errBody); got != http.StatusUnauthorized || errBody.Code != "invalid_credentials" {
		t.Fatalf("wrong password: %d %+v", got, errBody)
	}
	// Email is case-insensitive.
	anon.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "  " + upper(email), "password": testPassword}, nil)
	anon.must(http.StatusOK, http.MethodGet, "/api/v1/auth/me", nil, nil)
}

func TestCSRFHeaderRequired(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Csrf")
	status := cl.doRaw(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "X", "typeId": typeRent}, nil, false)
	if status != http.StatusForbidden {
		t.Fatalf("missing CSRF header: %d, want 403", status)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	e := newEnv(t)
	cl := e.client()
	email := uniqueEmail("reset")
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/signup",
		map[string]any{"email": email, "password": testPassword, "displayName": "Reset"}, nil)

	// Unknown emails get the same response.
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password-reset/request", map[string]any{"email": uniqueEmail("nobody")}, nil)
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password-reset/request", map[string]any{"email": email}, nil)

	msg := e.mailer.last()
	if msg.To != email {
		t.Fatalf("reset email went to %q", msg.To)
	}
	m := regexp.MustCompile(`token=([A-Za-z0-9_-]+)`).FindStringSubmatch(msg.Text)
	if m == nil {
		t.Fatalf("no token in email: %q", msg.Text)
	}
	newPassword := "a brand new passphrase"
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]any{"token": m[1], "password": newPassword}, nil)

	// Existing sessions were revoked.
	if got := cl.do(http.MethodGet, "/api/v1/auth/me", nil, nil); got != http.StatusUnauthorized {
		t.Fatalf("session should be revoked after reset, got %d", got)
	}
	// Tokens are single-use.
	if got := cl.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]any{"token": m[1], "password": newPassword}, nil); got != http.StatusBadRequest {
		t.Fatalf("reused token: %d", got)
	}
	if got := cl.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": testPassword}, nil); got != http.StatusUnauthorized {
		t.Fatalf("old password still works: %d", got)
	}
	cl.must(http.StatusOK, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": newPassword}, nil)
}

func TestViewerCannotWrite(t *testing.T) {
	e := newEnv(t)
	cl, me := e.signedUp("Viewer")
	if _, err := testDB.Pool.Exec(context.Background(),
		"UPDATE memberships SET role = 'viewer' WHERE user_id = $1", me.User.Id); err != nil {
		t.Fatal(err)
	}
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles", nil, nil)
	if got := cl.do(http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Rent", "typeId": typeRent}, nil); got != http.StatusForbidden {
		t.Fatalf("viewer create: %d, want 403", got)
	}
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/auth/logout", nil, nil)
}

func TestAccountDeletionPurgedByTick(t *testing.T) {
	e := newEnv(t)
	cl := e.client()
	email := uniqueEmail("delete")
	var me apigen.Me
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/auth/signup",
		map[string]any{"email": email, "password": testPassword, "displayName": "Delete"}, &me)
	cl.must(http.StatusNoContent, http.MethodDelete, "/api/v1/account", nil, nil)
	if got := cl.do(http.MethodGet, "/api/v1/auth/me", nil, nil); got != http.StatusUnauthorized {
		t.Fatalf("deletion should sign out: %d", got)
	}

	// Pretend the grace period has passed.
	if _, err := testDB.Pool.Exec(context.Background(),
		"UPDATE users SET deletion_requested_at = $2 WHERE id = $1", me.User.Id, e.server.now().AddDate(0, 0, -15)); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, e.http.URL+"/internal/tick", nil)
	req.Header.Set("Authorization", "Bearer "+testTickToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tick: %d", resp.StatusCode)
	}
	if got := cl.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email, "password": testPassword}, nil); got != http.StatusUnauthorized {
		t.Fatalf("purged user can still sign in: %d", got)
	}
	var households int
	if err := testDB.Pool.QueryRow(context.Background(), "SELECT count(*) FROM households WHERE id = $1", me.Household.Id).Scan(&households); err != nil {
		t.Fatal(err)
	}
	if households != 0 {
		t.Fatal("household should be deleted with its only member")
	}
}

func TestTickRejectsBadToken(t *testing.T) {
	e := newEnv(t)
	req, _ := http.NewRequest(http.MethodPost, e.http.URL+"/internal/tick", nil)
	req.Header.Set("Authorization", "Bearer nope")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", resp.StatusCode)
	}
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
