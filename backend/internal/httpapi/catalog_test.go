package httpapi

import (
	"net/http"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
)

func TestCustomTypesLifecycle(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Types")

	var custom apigen.BillType
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/types", map[string]any{
		"name": "Maid salary", "icon": "user", "defaultKind": "recurring", "group": "household",
	}, &custom)
	if custom.IsSystem || custom.DefaultKind != apigen.Recurring {
		t.Fatalf("custom type = %+v", custom)
	}

	var errBody apigen.Error
	// Names are unique against system types too, case-insensitively.
	if got := cl.do(http.MethodPost, "/api/v1/types", map[string]any{
		"name": "electricity", "icon": "zap", "defaultKind": "metered", "group": "utilities",
	}, &errBody); got != http.StatusBadRequest || (*errBody.Fields)["name"] == "" {
		t.Fatalf("duplicate type name: %d %+v", got, errBody)
	}
	// Sub-categories are one level deep.
	if got := cl.do(http.MethodPost, "/api/v1/types", map[string]any{
		"name": "Deep", "icon": "x", "defaultKind": "expense_log", "group": "household", "parentId": typeElectrician,
	}, &errBody); got != http.StatusBadRequest {
		t.Fatalf("two-level sub-category: %d", got)
	}
	// Built-in types are read-only.
	if got := cl.do(http.MethodPut, "/api/v1/types/"+typeRent, map[string]any{
		"name": "Rent!", "icon": "house", "defaultKind": "recurring", "group": "household",
	}, nil); got != http.StatusForbidden {
		t.Fatalf("edit system type: %d", got)
	}

	var renamed apigen.BillType
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/types/"+custom.Id.String(), map[string]any{
		"name": "Household help", "icon": "user", "defaultKind": "recurring", "group": "household",
	}, &renamed)
	if renamed.Name != "Household help" {
		t.Fatalf("rename: %+v", renamed)
	}

	// A type in use can't be archived until its profiles are.
	p := cl.createProfile(map[string]any{"name": "Maid", "typeId": custom.Id})
	if got := cl.do(http.MethodPost, "/api/v1/types/"+custom.Id.String()+"/archive", nil, &errBody); got != http.StatusConflict || errBody.Code != "type_in_use" {
		t.Fatalf("archive in-use type: %d %+v", got, errBody)
	}
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/profiles/"+p.Id.String()+"/archive", nil, nil)
	cl.must(http.StatusNoContent, http.MethodPost, "/api/v1/types/"+custom.Id.String()+"/archive", nil, nil)

	// Other households never see it.
	other, _ := e.signedUp("OtherTypes")
	var theirs []apigen.BillType
	other.must(http.StatusOK, http.MethodGet, "/api/v1/types", nil, &theirs)
	for _, ty := range theirs {
		if ty.Id == custom.Id {
			t.Fatal("custom type leaked to another household")
		}
	}
}

func TestProvidersCatalogueAndCustom(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Providers")

	var electric []apigen.Provider
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/providers?typeId="+typeElectricity, nil, &electric)
	if len(electric) < 11 {
		t.Fatalf("expected the DISCOs + K-Electric, got %d", len(electric))
	}
	for _, p := range electric {
		if p.TypeId.String() != typeElectricity {
			t.Fatalf("filter leaked %s", p.ShortName)
		}
	}

	var custom apigen.Provider
	cl.must(http.StatusCreated, http.MethodPost, "/api/v1/providers", map[string]any{
		"name": "Society Maintenance Office", "typeId": typeMaintenance,
	}, &custom)
	if !custom.IsCustom || custom.ShortName != "Society Maintenance Office" || custom.RefNumberLabel != "Account No." {
		t.Fatalf("custom provider defaults = %+v", custom)
	}
}

func TestHouseholdSettings(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Settings")

	var hh apigen.Household
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/household", map[string]any{
		"name": "Khan family", "currency": "PKR", "timezone": "Asia/Karachi",
	}, &hh)
	if hh.Name != "Khan family" {
		t.Fatalf("household = %+v", hh)
	}
	if got := cl.do(http.MethodPut, "/api/v1/household", map[string]any{
		"name": "x", "currency": "PKR", "timezone": "Mars/Olympus",
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("bad timezone: %d", got)
	}
	if got := cl.do(http.MethodPut, "/api/v1/household", map[string]any{
		"name": "x", "currency": "rupees", "timezone": "Asia/Karachi",
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("bad currency (spec pattern): %d", got)
	}
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/household", nil, &hh)
	if hh.Name != "Khan family" {
		t.Fatal("household update not persisted")
	}
}

func TestProfileUpdateKindLockAndReorder(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("Update")
	p := homeElectricity(cl, "Home Electricity")

	var updated apigen.Profile
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/profiles/"+p.Id.String(), map[string]any{
		"name": "Main house electricity", "typeId": typeElectricity, "providerId": providerLESCO,
		"referenceNumber": *p.ReferenceNumber, "multiplyingFactor": 2.5, "pinned": false,
	}, &updated)
	if updated.Name != "Main house electricity" || updated.Pinned || updated.MultiplyingFactor == nil || *updated.MultiplyingFactor != 2.5 {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.Color != p.Color {
		t.Fatal("colour must be kept when not sent")
	}

	// Kind can change while empty, but not once bills exist.
	cl.must(http.StatusOK, http.MethodPut, "/api/v1/profiles/"+p.Id.String(), map[string]any{
		"name": "Main house electricity", "typeId": typeElectricity, "kind": "recurring",
	}, &updated)
	if updated.Kind != apigen.Recurring {
		t.Fatalf("kind = %s", updated.Kind)
	}
	cl.createBill(map[string]any{"profileId": p.Id, "periodKey": "2026-09", "currentChargesMinor": 100})
	var errBody apigen.Error
	if got := cl.do(http.MethodPut, "/api/v1/profiles/"+p.Id.String(), map[string]any{
		"name": "Main house electricity", "typeId": typeElectricity, "kind": "expense_log",
	}, &errBody); got != http.StatusBadRequest || (*errBody.Fields)["kind"] == "" {
		t.Fatalf("kind change with bills: %d %+v", got, errBody)
	}

	second := cl.createProfile(map[string]any{"name": "Flat Rent", "typeId": typeRent})
	cl.must(http.StatusNoContent, http.MethodPut, "/api/v1/profiles/order", map[string]any{
		"ids": []string{second.Id.String(), p.Id.String()},
	}, nil)
	var list []apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles", nil, &list)
	if list[0].Id != second.Id {
		t.Fatalf("reorder not applied: first is %s", list[0].Name)
	}
	if got := cl.do(http.MethodPut, "/api/v1/profiles/order", map[string]any{
		"ids": []string{second.Id.String(), second.Id.String()},
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("duplicate ids in reorder: %d", got)
	}

	var got apigen.Profile
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/profiles/"+p.Id.String(), nil, &got)
	if got.Summary.BillCount != 1 || got.Summary.ThisYearMinor != 100 {
		t.Fatalf("summary = %+v", got.Summary)
	}
}

func TestUnknownRouteIsJSON404(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("NotFound")
	var errBody apigen.Error
	if got := cl.do(http.MethodGet, "/api/v1/nope", nil, &errBody); got != http.StatusNotFound || errBody.Code != "not_found" {
		t.Fatalf("unknown route: %d %+v", got, errBody)
	}
}
