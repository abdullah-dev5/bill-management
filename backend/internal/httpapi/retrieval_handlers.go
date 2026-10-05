package httpapi

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// retrievalInfo returns a function describing each profile's official source: automatic (a
// connector), assisted (the provider's lookup page), or none. Providers and connector health are
// loaded once per request.
func (s *Server) retrievalInfo(ctx context.Context, householdID uuid.UUID) (func(store.BillProfile) apigen.ProfileRetrieval, error) {
	providers, err := s.db.ListProviders(ctx, store.ListProvidersParams{HouseholdID: &householdID})
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]store.Provider{}
	for _, p := range providers {
		byID[p.ID] = p
	}
	paused := map[string]bool{}
	health, err := s.db.ListConnectorHealth(ctx)
	if err != nil {
		return nil, err
	}
	for _, h := range health {
		paused[h.ID] = h.DisabledUntil != nil && h.DisabledUntil.After(s.now())
	}
	return func(p store.BillProfile) apigen.ProfileRetrieval {
		out := apigen.ProfileRetrieval{Mode: apigen.None, Enabled: false, LastAt: p.LastRetrievalAt, LastMessage: p.LastRetrievalMessage}
		if p.LastRetrievalOutcome != nil {
			o := apigen.ProfileRetrievalLastOutcome(*p.LastRetrievalOutcome)
			out.LastOutcome = &o
		}
		if p.ProviderID == nil {
			return out
		}
		prov, ok := byID[*p.ProviderID]
		if !ok {
			return out
		}
		out.LookupUrl = prov.LookupUrl
		if c := s.retrieval.For(prov.ShortName); c != nil {
			out.Mode, out.Enabled, out.ConnectorPaused = apigen.Automatic, p.RetrievalEnabled, ptr(paused[c.ID()])
		} else if prov.LookupUrl != nil {
			out.Mode = apigen.Assisted
		}
		return out
	}, nil
}

// RetrieveProfileBill is "Fetch now" (FR-RETR-06): user-initiated and throttled.
func (s *Server) RetrieveProfileBill(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	ctx := r.Context()
	p, err := s.db.GetProfile(ctx, store.GetProfileParams{ID: id, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("profile"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	if p.ProviderID == nil {
		s.fail(w, r, errConflict("no_connector", "Choose this profile's provider first."))
		return
	}
	prov, err := s.db.GetProvider(ctx, store.GetProviderParams{ID: *p.ProviderID, HouseholdID: &a.HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if s.retrieval.For(prov.ShortName) == nil {
		s.fail(w, r, errConflict("no_connector", "Bills from "+prov.ShortName+" can't be fetched automatically. Open the official bill page and upload the bill instead."))
		return
	}
	if p.LastRetrievalAt != nil && s.now().Sub(*p.LastRetrievalAt) < manualRetrievalGap {
		s.fail(w, r, errTooManyRequests)
		return
	}
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	res := s.retrieveProfile(ctx, p, prov.ShortName, hh.Timezone, hh.Currency, "manual")
	writeJSON(w, http.StatusOK, apigen.RetrievalResult{Outcome: apigen.RetrievalResultOutcome(res.Outcome), Message: res.Message, BillId: res.BillID})
}

// ListConnectors shows each automatic connector's health (FR-RETR-05).
func (s *Server) ListConnectors(w http.ResponseWriter, r *http.Request) {
	health, err := s.db.ListConnectorHealth(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	byID := map[string]store.RetrievalConnector{}
	for _, h := range health {
		byID[h.ID] = h
	}
	out := []apigen.ConnectorStatus{}
	for _, c := range s.retrieval.All() {
		h := byID[c.ID()]
		st := apigen.ConnectorStatus{
			Id: c.ID(), Name: c.Name(), Providers: c.Providers(), Healthy: h.DisabledUntil == nil || !h.DisabledUntil.After(s.now()),
			DisabledUntil: h.DisabledUntil, ConsecutiveFailures: ptr(int(h.ConsecutiveFailures)), LastSuccessAt: h.LastSuccessAt, LastFailureAt: h.LastFailureAt,
		}
		out = append(out, st)
	}
	writeJSON(w, http.StatusOK, out)
}

// SetProfileRetrieval turns automatic checks on or off (FR-RETR-01). Audited.
func (s *Server) SetProfileRetrieval(w http.ResponseWriter, r *http.Request, id apigen.Id) {
	a := mustAuth(r)
	var in apigen.SetProfileRetrievalJSONRequestBody
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	p, err := s.db.GetProfile(ctx, store.GetProfileParams{ID: id, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		s.fail(w, r, errNotFound("profile"))
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	if in.Enabled {
		info, err := s.retrievalInfo(ctx, a.HouseholdID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if info(p).Mode != apigen.Automatic {
			s.fail(w, r, errConflict("no_connector", "This provider's bills can't be fetched automatically."))
			return
		}
	}
	if err := s.db.InTx(ctx, func(q *store.Queries) error {
		if err := q.SetProfileRetrievalEnabled(ctx, store.SetProfileRetrievalEnabledParams{ID: id, HouseholdID: a.HouseholdID, RetrievalEnabled: in.Enabled}); err != nil {
			return err
		}
		return audit(ctx, q, a, "profile", id, "update", map[string]any{"retrievalEnabled": []any{p.RetrievalEnabled, in.Enabled}})
	}); err != nil {
		s.fail(w, r, err)
		return
	}
	p.RetrievalEnabled = in.Enabled
	s.writeProfile(w, r, http.StatusOK, s.db.Queries, a, p)
}
