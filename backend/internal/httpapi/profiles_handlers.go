package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const paletteSize = 8

func (s *Server) ListProfiles(w http.ResponseWriter, r *http.Request, params apigen.ListProfilesParams) {
	a := mustAuth(r)
	ctx := r.Context()
	rows, err := s.db.ListProfiles(ctx, store.ListProfilesParams{HouseholdID: a.HouseholdID, IncludeArchived: deref(params.IncludeArchived)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	summaries, err := s.profileSummaries(ctx, s.db.Queries, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]apigen.Profile, 0, len(rows))
	for _, p := range rows {
		out = append(out, toProfile(p, s.refDisplay(ctx, p), summaries.get(p.ID)))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) GetProfile(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	p, err := s.db.GetProfile(r.Context(), store.GetProfileParams{ID: id, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("profile"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeProfile(w, r, http.StatusOK, s.db.Queries, a, p)
}

func (s *Server) CreateProfile(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.ProfileInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	var out store.BillProfile
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		f, err := s.buildProfile(ctx, q, a, in, nil)
		if err != nil {
			return err
		}
		prop, err := q.GetDefaultProperty(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		order, err := q.NextProfileSortOrder(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		pinned := true // new profiles appear on the home dashboard unless told otherwise
		if in.Pinned != nil {
			pinned = *in.Pinned
		}
		out, err = q.CreateProfile(ctx, store.CreateProfileParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: a.HouseholdID, PropertyID: prop.ID,
			CategoryID: f.CategoryID, Kind: f.Kind, Name: f.Name, Icon: f.Icon, Color: f.Color,
			ProviderID: f.ProviderID, ReferenceHash: f.ReferenceHash, ReferenceEnc: f.ReferenceEnc,
			MeterNumber: f.MeterNumber, MultiplyingFactor: f.MultiplyingFactor, ConsumptionUnit: f.ConsumptionUnit,
			BillingCycle: f.BillingCycle, TypicalIssueDay: f.TypicalIssueDay, TypicalDueDay: f.TypicalDueDay,
			ExpectedAmountMinor: f.ExpectedAmountMinor, AutoCreateBill: f.AutoCreateBill,
			Pinned: pinned, SortOrder: order,
		})
		if err != nil {
			return profileConflict(err)
		}
		return audit(ctx, q, a, "profile", out.ID, "create", maskReference(diff(snapshot{}, profileSnapshot(out, f.ReferenceDisplay))))
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeProfile(w, r, http.StatusCreated, s.db.Queries, a, out)
}

func (s *Server) UpdateProfile(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	var in apigen.ProfileInput
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	var out store.BillProfile
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		existing, err := q.GetProfile(ctx, store.GetProfileParams{ID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("profile")
		} else if err != nil {
			return err
		}
		f, err := s.buildProfile(ctx, q, a, in, &existing)
		if err != nil {
			return err
		}
		if f.Kind != existing.Kind {
			if existing.IsDefault {
				return errField("kind", "The General expenses profile is always an expense log.")
			}
			n, err := q.CountProfileBills(ctx, id)
			if err != nil {
				return err
			}
			if n > 0 {
				return errField("kind", "The kind can't change once a profile has bills. Create a new profile instead.")
			}
		}
		pinned := existing.Pinned
		if in.Pinned != nil {
			pinned = *in.Pinned
		}
		out, err = q.UpdateProfile(ctx, store.UpdateProfileParams{
			ID: id, HouseholdID: a.HouseholdID,
			CategoryID: f.CategoryID, Kind: f.Kind, Name: f.Name, Icon: f.Icon, Color: f.Color,
			ProviderID: f.ProviderID, ReferenceHash: f.ReferenceHash, ReferenceEnc: f.ReferenceEnc,
			MeterNumber: f.MeterNumber, MultiplyingFactor: f.MultiplyingFactor, ConsumptionUnit: f.ConsumptionUnit,
			BillingCycle: f.BillingCycle, TypicalIssueDay: f.TypicalIssueDay, TypicalDueDay: f.TypicalDueDay,
			ExpectedAmountMinor: f.ExpectedAmountMinor, AutoCreateBill: f.AutoCreateBill, Pinned: pinned,
		})
		if err != nil {
			return profileConflict(err)
		}
		return audit(ctx, q, a, "profile", id, "update",
			maskReference(diff(profileSnapshot(existing, s.refDisplay(ctx, existing)), profileSnapshot(out, f.ReferenceDisplay))))
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeProfile(w, r, http.StatusOK, s.db.Queries, a, out)
}

func (s *Server) ArchiveProfile(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		p, err := q.GetProfile(ctx, store.GetProfileParams{ID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("profile")
		} else if err != nil {
			return err
		}
		if p.IsDefault {
			return errConflict("default_profile", "The General expenses profile can't be archived.")
		}
		if p.ArchivedAt != nil {
			return nil
		}
		if _, err := q.ArchiveProfile(ctx, store.ArchiveProfileParams{ID: id, HouseholdID: a.HouseholdID}); err != nil {
			return err
		}
		return audit(ctx, q, a, "profile", id, "archive", nil)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) RestoreProfile(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		n, err := q.RestoreProfile(ctx, store.RestoreProfileParams{ID: id, HouseholdID: a.HouseholdID})
		if err != nil {
			return profileConflict(err)
		}
		if n == 0 {
			if _, err := q.GetProfile(ctx, store.GetProfileParams{ID: id, HouseholdID: a.HouseholdID}); store.IsNotFound(err) {
				return errNotFound("profile")
			}
			return nil // already active
		}
		return audit(ctx, q, a, "profile", id, "restore", nil)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReorderProfiles(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.ProfileOrder
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		seen := map[uuid.UUID]bool{}
		for i, id := range in.Ids {
			if seen[id] {
				return errField("ids", "Each profile may appear only once.")
			}
			seen[id] = true
			n, err := q.SetProfileSortOrder(ctx, store.SetProfileSortOrderParams{ID: id, HouseholdID: a.HouseholdID, SortOrder: int32(i + 1)})
			if err != nil {
				return err
			}
			if n == 0 {
				return errField("ids", "Unknown profile "+id.String()+".")
			}
		}
		return nil
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// profileFields are the validated, normalised values for a profile write.
type profileFields struct {
	CategoryID          uuid.UUID
	Kind                store.ProfileKind
	Name                string
	Icon                string
	Color               int16
	ProviderID          *uuid.UUID
	ReferenceHash       []byte
	ReferenceEnc        []byte
	ReferenceDisplay    *string
	MeterNumber         *string
	MultiplyingFactor   *float64
	ConsumptionUnit     *string
	BillingCycle        *store.BillingCycle
	TypicalIssueDay     *int16
	TypicalDueDay       *int16
	ExpectedAmountMinor *int64
	AutoCreateBill      bool
}

// buildProfile applies the per-kind rules (FR-ACCT-03/04/05/09). existing is nil on create.
func (s *Server) buildProfile(ctx context.Context, q *store.Queries, a authInfo, in apigen.ProfileInput, existing *store.BillProfile) (profileFields, error) {
	fields := domain.FieldErrors{}
	f := profileFields{Name: strings.TrimSpace(in.Name)}
	if f.Name == "" {
		fields.Add("name", "Give the profile a name.")
	}

	typ, err := q.GetCategory(ctx, store.GetCategoryParams{ID: in.TypeId, HouseholdID: &a.HouseholdID})
	if store.IsNotFound(err) {
		return f, errField("typeId", "Choose a bill type.")
	} else if err != nil {
		return f, err
	}
	typeUnchanged := existing != nil && existing.CategoryID == typ.ID
	if typ.ArchivedAt != nil && !typeUnchanged {
		fields.Add("typeId", "This type is archived.")
	}
	if existing != nil && existing.IsDefault && !typeUnchanged {
		fields.Add("typeId", "The General expenses profile's type can't change.")
	}
	f.CategoryID = typ.ID

	f.Kind = typ.DefaultKind
	if in.Kind != nil {
		f.Kind = store.ProfileKind(*in.Kind)
	} else if existing != nil {
		f.Kind = existing.Kind
	}

	f.Icon = typ.Icon
	if icon := strings.TrimSpace(deref(in.Icon)); icon != "" {
		f.Icon = icon
	} else if existing != nil {
		f.Icon = existing.Icon
	}

	switch {
	case in.Color != nil:
		f.Color = int16(*in.Color)
	case existing != nil:
		f.Color = existing.Color
	default:
		used, err := q.ListProfileColors(ctx, a.HouseholdID)
		if err != nil {
			return f, err
		}
		f.Color = nextColor(used)
	}

	var provider *store.Provider
	if in.ProviderId != nil {
		p, err := q.GetProvider(ctx, store.GetProviderParams{ID: *in.ProviderId, HouseholdID: &a.HouseholdID})
		if store.IsNotFound(err) {
			fields.Add("providerId", "Unknown provider.")
		} else if err != nil {
			return f, err
		} else {
			provider = &p
			f.ProviderID = &p.ID
		}
	}

	if display := domain.CleanText(in.ReferenceNumber); display != nil {
		norm := domain.NormalizeReference(*display)
		if norm == "" {
			fields.Add("referenceNumber", "Enter the number printed on your bill.")
		} else if provider != nil && provider.RefNumberPattern != nil {
			ok, err := domain.MatchesPattern(norm, *provider.RefNumberPattern)
			if err != nil {
				return f, err
			}
			if !ok {
				fields.Add("referenceNumber", fmt.Sprintf("This doesn't look like a %s %s. Check the number on your bill.", provider.ShortName, provider.RefNumberLabel))
			}
		}
		f.ReferenceHash, f.ReferenceEnc, f.ReferenceDisplay = s.keys.Hash(norm), s.keys.Seal(*display), display
	}

	cycleBased := f.Kind == store.ProfileKindMetered || f.Kind == store.ProfileKindRecurring
	if cycleBased {
		cycle := store.BillingCycleMonthly
		if in.BillingCycle != nil {
			cycle = store.BillingCycle(*in.BillingCycle)
		}
		f.BillingCycle = &cycle
		f.TypicalIssueDay = dayPtr(in.TypicalIssueDay)
		f.TypicalDueDay = dayPtr(in.TypicalDueDay)
	}

	if f.Kind == store.ProfileKindMetered {
		f.MeterNumber = domain.CleanText(in.MeterNumber)
		mf := 1.0
		if in.MultiplyingFactor != nil {
			mf = *in.MultiplyingFactor
		}
		f.MultiplyingFactor = &mf
		f.ConsumptionUnit = domain.CleanText(in.ConsumptionUnit)
		if f.ConsumptionUnit == nil && provider != nil {
			f.ConsumptionUnit = provider.ConsumptionUnit
		}
	}

	if f.Kind == store.ProfileKindRecurring {
		f.ExpectedAmountMinor = in.ExpectedAmountMinor
		f.AutoCreateBill = deref(in.AutoCreateBill)
		if f.AutoCreateBill && f.ExpectedAmountMinor == nil {
			fields.Add("expectedAmountMinor", "Set the expected amount to create bills automatically.")
		}
	}

	if fields.Any() {
		return f, errValidation(fields)
	}
	return f, nil
}

// nextColor picks the lowest palette slot not used yet, cycling once all eight are taken.
func nextColor(used []int16) int16 {
	count := map[int16]int{}
	for _, c := range used {
		count[c]++
	}
	best, bestN := int16(1), int(^uint(0)>>1)
	for c := int16(1); c <= paletteSize; c++ {
		if count[c] < bestN {
			best, bestN = c, count[c]
		}
	}
	return best
}

func dayPtr(d *int) *int16 {
	if d == nil {
		return nil
	}
	v := int16(*d)
	return &v
}

func profileConflict(err error) error {
	switch store.UniqueViolation(err) {
	case "bill_profiles_name_uniq":
		return &apiError{status: http.StatusConflict, code: "profile_name_taken", message: "You already have a profile with this name.",
			fields: map[string]string{"name": "You already have a profile with this name."}}
	case "bill_profiles_ref_uniq":
		return &apiError{status: http.StatusConflict, code: "reference_taken", message: "Another profile already uses this reference number.",
			fields: map[string]string{"referenceNumber": "Another profile already uses this reference number."}}
	}
	return err
}

func (s *Server) writeProfile(w http.ResponseWriter, r *http.Request, status int, q *store.Queries, a authInfo, p store.BillProfile) {
	summaries, err := s.profileSummaries(r.Context(), q, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, status, toProfile(p, s.refDisplay(r.Context(), p), summaries.get(p.ID)))
}

// refDisplay decrypts a profile's reference number for display.
func (s *Server) refDisplay(ctx context.Context, p store.BillProfile) *string {
	if p.ReferenceEnc == nil {
		return nil
	}
	v, err := s.keys.Open(p.ReferenceEnc)
	if err != nil {
		s.log.ErrorContext(ctx, "decrypt reference number", "profile_id", p.ID, "err", err)
		return nil
	}
	return &v
}

type summaryMap map[uuid.UUID]apigen.ProfileSummary

func (m summaryMap) get(id uuid.UUID) apigen.ProfileSummary {
	if sm, ok := m[id]; ok {
		return sm
	}
	return apigen.ProfileSummary{}
}

// profileSummaries computes totals and the latest bill for every profile of the household.
func (s *Server) profileSummaries(ctx context.Context, q *store.Queries, householdID uuid.UUID) (summaryMap, error) {
	hh, err := q.GetHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	today := s.today(hh.Timezone)
	rows, err := q.ProfileSummaries(ctx, store.ProfileSummariesParams{
		HouseholdID: householdID, Today: today, Year: today.Format("2006"),
	})
	if err != nil {
		return nil, err
	}
	out := summaryMap{}
	for _, row := range rows {
		out[row.ProfileID] = apigen.ProfileSummary{
			BillCount: int(row.BillCount), OutstandingMinor: row.OutstandingMinor, ThisYearMinor: row.ThisYearMinor,
		}
	}
	latest, err := q.LatestBillPerProfile(ctx, store.LatestBillPerProfileParams{HouseholdID: householdID, Today: today})
	if err != nil {
		return nil, err
	}
	for _, row := range latest {
		sm := out[row.Bill.ProfileID]
		b := toBill(billView{Bill: row.Bill, Status: row.Status, OutstandingMinor: row.OutstandingMinor})
		sm.LatestBill = &b
		out[row.Bill.ProfileID] = sm
	}
	return out, nil
}
