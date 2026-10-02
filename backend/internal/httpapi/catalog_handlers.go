package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// --- Household -----------------------------------------------------------------

func (s *Server) GetHousehold(w http.ResponseWriter, r *http.Request) {
	hh, err := s.db.GetHousehold(r.Context(), mustAuth(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toHousehold(hh))
}

func (s *Server) UpdateHousehold(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	if a.Role != store.MemberRoleOwner {
		s.fail(w, r, errForbidden("Only the household owner can change these settings."))
		return
	}
	var in apigen.HouseholdInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		s.fail(w, r, errField("name", "Enter a name."))
		return
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil || in.Timezone == "" {
		s.fail(w, r, errField("timezone", "Unknown timezone."))
		return
	}
	ctx := r.Context()
	var out store.Household
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		before, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		out, err = q.UpdateHousehold(ctx, store.UpdateHouseholdParams{
			ID: a.HouseholdID, Name: name, Currency: in.Currency, Timezone: in.Timezone,
		})
		if err != nil {
			return err
		}
		changes := diff(
			snapshot{"name": before.Name, "currency": before.Currency, "timezone": before.Timezone},
			snapshot{"name": out.Name, "currency": out.Currency, "timezone": out.Timezone},
		)
		return audit(ctx, q, a, "household", a.HouseholdID, "update", changes)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toHousehold(out))
}

// --- Bill types ----------------------------------------------------------------

func (s *Server) ListTypes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.ListCategories(r.Context(), ptr(mustAuth(r).HouseholdID))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]apigen.BillType, 0, len(rows))
	for _, c := range rows {
		out = append(out, toBillType(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CreateType(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.BillTypeInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	var out store.Category
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		name, err := s.checkTypeInput(r, q, a, in, uuid.Nil)
		if err != nil {
			return err
		}
		order, err := q.NextCategorySortOrder(ctx, ptr(a.HouseholdID))
		if err != nil {
			return err
		}
		out, err = q.CreateCategory(ctx, store.CreateCategoryParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: ptr(a.HouseholdID), Name: name,
			Icon: strings.TrimSpace(in.Icon), DefaultKind: store.ProfileKind(in.DefaultKind),
			TypeGroup: store.TypeGroup(in.Group), ParentID: in.ParentId, SortOrder: order,
		})
		if err != nil {
			return err
		}
		return audit(ctx, q, a, "type", out.ID, "create", map[string]any{"name": name})
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toBillType(out))
}

func (s *Server) UpdateType(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	var in apigen.BillTypeInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	var out store.Category
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		existing, err := q.GetCategory(ctx, store.GetCategoryParams{ID: id, HouseholdID: ptr(a.HouseholdID)})
		if store.IsNotFound(err) {
			return errNotFound("bill type")
		} else if err != nil {
			return err
		}
		if existing.HouseholdID == nil {
			return errForbidden("Built-in types can't be changed. Create your own type instead.")
		}
		name, err := s.checkTypeInput(r, q, a, in, id)
		if err != nil {
			return err
		}
		out, err = q.UpdateCategory(ctx, store.UpdateCategoryParams{
			ID: id, HouseholdID: ptr(a.HouseholdID), Name: name, Icon: strings.TrimSpace(in.Icon),
			DefaultKind: store.ProfileKind(in.DefaultKind), TypeGroup: store.TypeGroup(in.Group), ParentID: in.ParentId,
		})
		if err != nil {
			return err
		}
		return audit(ctx, q, a, "type", id, "update", diff(
			snapshot{"name": existing.Name, "icon": existing.Icon, "defaultKind": string(existing.DefaultKind)},
			snapshot{"name": out.Name, "icon": out.Icon, "defaultKind": string(out.DefaultKind)},
		))
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toBillType(out))
}

func (s *Server) ArchiveType(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		existing, err := q.GetCategory(ctx, store.GetCategoryParams{ID: id, HouseholdID: ptr(a.HouseholdID)})
		if store.IsNotFound(err) {
			return errNotFound("bill type")
		} else if err != nil {
			return err
		}
		if existing.HouseholdID == nil {
			return errForbidden("Built-in types can't be archived.")
		}
		inUse, err := q.CountActiveProfilesForCategory(ctx, store.CountActiveProfilesForCategoryParams{CategoryID: id, HouseholdID: a.HouseholdID})
		if err != nil {
			return err
		}
		if inUse > 0 {
			return errConflict("type_in_use", "Archive or move the profiles that use this type first.")
		}
		if err := q.ArchiveCategory(ctx, store.ArchiveCategoryParams{ID: id, HouseholdID: ptr(a.HouseholdID)}); err != nil {
			return err
		}
		return audit(ctx, q, a, "type", id, "archive", nil)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) checkTypeInput(r *http.Request, q *store.Queries, a authInfo, in apigen.BillTypeInput, excludeID uuid.UUID) (string, error) {
	ctx := r.Context()
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errField("name", "Enter a name.")
	}
	if strings.TrimSpace(in.Icon) == "" {
		return "", errField("icon", "Choose an icon.")
	}
	taken, err := q.CategoryNameTaken(ctx, store.CategoryNameTakenParams{HouseholdID: ptr(a.HouseholdID), Name: name, ExcludeID: excludeID})
	if err != nil {
		return "", err
	}
	if taken {
		return "", errField("name", "A type with this name already exists.")
	}
	if in.ParentId != nil {
		if *in.ParentId == excludeID {
			return "", errField("parentId", "A type can't be its own parent.")
		}
		parent, err := q.GetCategory(ctx, store.GetCategoryParams{ID: *in.ParentId, HouseholdID: ptr(a.HouseholdID)})
		if store.IsNotFound(err) {
			return "", errField("parentId", "Unknown parent type.")
		} else if err != nil {
			return "", err
		}
		if parent.ParentID != nil {
			return "", errField("parentId", "Sub-categories can only be one level deep.")
		}
	}
	return name, nil
}

// --- Providers -----------------------------------------------------------------

func (s *Server) ListProviders(w http.ResponseWriter, r *http.Request, params apigen.ListProvidersParams) {
	rows, err := s.db.ListProviders(r.Context(), store.ListProvidersParams{
		HouseholdID: ptr(mustAuth(r).HouseholdID), CategoryID: params.TypeId,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]apigen.Provider, 0, len(rows))
	for _, p := range rows {
		out = append(out, toProvider(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CreateProvider(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.ProviderInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		s.fail(w, r, errField("name", "Enter the provider's name."))
		return
	}
	short := strings.TrimSpace(deref(in.ShortName))
	if short == "" {
		short = name
	}
	label := strings.TrimSpace(deref(in.RefNumberLabel))
	if label == "" {
		label = "Account No."
	}
	ctx := r.Context()
	var out store.Provider
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		if _, err := q.GetCategory(ctx, store.GetCategoryParams{ID: in.TypeId, HouseholdID: ptr(a.HouseholdID)}); store.IsNotFound(err) {
			return errField("typeId", "Unknown bill type.")
		} else if err != nil {
			return err
		}
		var err error
		out, err = q.CreateProvider(ctx, store.CreateProviderParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: ptr(a.HouseholdID), Name: name, ShortName: short,
			CategoryID: in.TypeId, RefNumberLabel: label, ConsumptionUnit: domain.CleanText(in.ConsumptionUnit),
		})
		if err != nil {
			return err
		}
		return audit(ctx, q, a, "provider", out.ID, "create", map[string]any{"name": name})
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProvider(out))
}

func ptr[T any](v T) *T { return &v }
