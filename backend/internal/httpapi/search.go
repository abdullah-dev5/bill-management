package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/search"
	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const searchBillLimit = 50

// Search answers the global search box (FR-SRCH-01/02).
func (s *Server) Search(w http.ResponseWriter, r *http.Request, params apigen.SearchParams) {
	a := mustAuth(r)
	ctx := r.Context()
	text := strings.TrimSpace(params.Q)
	if text == "" || utf8.RuneCountInString(text) > 200 {
		s.fail(w, r, errField("q", "Type something to search for."))
		return
	}
	hh, err := s.db.GetHousehold(ctx, a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	types, err := s.db.ListCategories(ctx, &a.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	providers, err := s.db.ListProviders(ctx, store.ListProvidersParams{HouseholdID: &a.HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	profiles, err := s.db.ListProfiles(ctx, store.ListProfilesParams{HouseholdID: a.HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}

	cat := search.Catalog{}
	typeName := map[uuid.UUID]string{}
	for _, t := range types {
		typeName[t.ID] = t.Name
		if t.ArchivedAt == nil {
			cat.Types = append(cat.Types, search.Named{ID: t.ID, Name: t.Name, ParentID: t.ParentID})
		}
	}
	providerByID := map[uuid.UUID]store.Provider{}
	for _, p := range providers {
		providerByID[p.ID] = p
		cat.Providers = append(cat.Providers, search.Named{ID: p.ID, Name: p.Name, Short: p.ShortName})
	}

	today := s.today(hh.Timezone)
	q := search.Parse(text, cat, today)

	// A word that is a profile's reference number finds that profile (matched by keyed hash;
	// the numbers themselves are stored encrypted).
	// People type them with spaces ("09 12345 0012345"), so runs of adjacent words are tried
	// together, longest first.
	var refProfiles []uuid.UUID
	var terms []string
	for i := 0; i < len(q.Terms); {
		matchedTo := -1
		for j := min(len(q.Terms), i+5); j > i; j-- {
			if matched := s.referenceMatches(strings.Join(q.Terms[i:j], ""), profiles); len(matched) > 0 {
				refProfiles = append(refProfiles, matched...)
				matchedTo = j
				break
			}
		}
		if matchedTo > 0 {
			i = matchedTo
			continue
		}
		terms = append(terms, q.Terms[i])
		i++
	}
	q.Terms = terms

	out := apigen.SearchResult{
		Query:     text,
		Profiles:  []apigen.SearchProfile{},
		Bills:     []apigen.Bill{},
		Documents: []apigen.SearchDocument{},
		Interpretation: apigen.SearchInterpretation{
			TypeNames: nonNil(q.TypeNames), ProviderNames: []string{}, Statuses: []apigen.BillStatus{}, Terms: nonNil(q.Terms),
		},
	}
	providerIDs := make([]uuid.UUID, 0, len(q.Providers))
	for _, p := range q.Providers {
		providerIDs = append(providerIDs, p.ID)
		out.Interpretation.ProviderNames = append(out.Interpretation.ProviderNames, p.Short)
	}
	for _, st := range q.Statuses {
		out.Interpretation.Statuses = append(out.Interpretation.Statuses, apigen.BillStatus(st))
	}
	if q.From != "" {
		out.Interpretation.From, out.Interpretation.To = &q.From, &q.To
	}
	if len(refProfiles) > 0 {
		out.Interpretation.ReferenceMatch = ptr(true)
	}

	// Profiles: by reference number, or matching every word by name / type / provider.
	refSet := map[uuid.UUID]bool{}
	for _, id := range refProfiles {
		refSet[id] = true
	}
	for _, p := range profiles {
		if p.ArchivedAt != nil {
			continue
		}
		var prov *store.Provider
		if p.ProviderID != nil {
			if v, ok := providerByID[*p.ProviderID]; ok {
				prov = &v
			}
		}
		if !refSet[p.ID] && !profileMatches(p, prov, typeName[p.CategoryID], q) {
			continue
		}
		sp := apigen.SearchProfile{Id: p.ID, Name: p.Name, Icon: p.Icon, Color: int(p.Color), TypeName: typeName[p.CategoryID]}
		if prov != nil {
			sp.ProviderName = &prov.ShortName
		}
		if ref := s.refDisplay(ctx, p); ref != nil {
			sp.ReferenceDisplay = ptr(secure.Mask(*ref))
		}
		out.Profiles = append(out.Profiles, sp)
	}

	if !q.Empty() || len(refProfiles) > 0 {
		patterns := likePatterns(q.Terms)
		rows, err := s.db.SearchBills(ctx, store.SearchBillsParams{
			Today: today, HouseholdID: a.HouseholdID, TypeIds: q.TypeIDs, ProviderIds: providerIDs, ProfileIds: refProfiles,
			FromPeriod: strPtr(q.From), ToPeriod: strPtr(q.To), Statuses: q.Statuses, Patterns: patterns, RowLimit: searchBillLimit,
		})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		for _, row := range rows {
			out.Bills = append(out.Bills, toBill(billView(row)))
		}
		if len(patterns) > 0 {
			docs, err := s.db.SearchDocuments(ctx, store.SearchDocumentsParams{HouseholdID: a.HouseholdID, Patterns: patterns})
			if err != nil {
				s.fail(w, r, err)
				return
			}
			for _, d := range docs {
				out.Documents = append(out.Documents, apigen.SearchDocument{
					Id: d.ID, FileName: d.FileName, Kind: apigen.SearchDocumentKind(d.Kind), BillId: d.BillID,
					Snippet: snippet(deref(d.OcrText), q.Terms), CreatedAt: d.CreatedAt,
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// referenceMatches returns the profiles whose reference number is term (ignoring spaces and dashes).
func (s *Server) referenceMatches(term string, profiles []store.BillProfile) []uuid.UUID {
	norm := domain.NormalizeReference(term)
	digits := 0
	for _, c := range norm {
		if c >= '0' && c <= '9' {
			digits++
		}
	}
	if len(norm) < 5 || digits < 3 {
		return nil
	}
	h := s.keys.Hash(norm)
	var out []uuid.UUID
	for _, p := range profiles {
		if p.ReferenceHash != nil && bytes.Equal(p.ReferenceHash, h) {
			out = append(out, p.ID)
		}
	}
	return out
}

func profileMatches(p store.BillProfile, prov *store.Provider, typeName string, q search.Query) bool {
	if len(q.TypeIDs) > 0 && !containsUUID(q.TypeIDs, p.CategoryID) {
		return false
	}
	if len(q.Providers) > 0 {
		ok := false
		for _, n := range q.Providers {
			ok = ok || (p.ProviderID != nil && *p.ProviderID == n.ID)
		}
		if !ok {
			return false
		}
	}
	if len(q.Terms) == 0 {
		// Only type / provider words: list those profiles; a bare month or status lists none.
		return len(q.TypeIDs) > 0 || len(q.Providers) > 0
	}
	hay := strings.ToLower(p.Name + " " + typeName)
	if prov != nil {
		hay += " " + strings.ToLower(prov.Name+" "+prov.ShortName)
	}
	for _, t := range q.Terms {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

// likePatterns turns words into ILIKE patterns, escaping the wildcards.
func likePatterns(terms []string) []string {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		out = append(out, "%"+esc.Replace(t)+"%")
	}
	return out
}

// snippet returns ~120 characters of text around the first term found.
func snippet(text string, terms []string) *string {
	if text == "" {
		return nil
	}
	flat := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(flat)
	at := -1
	for _, t := range terms {
		if i := strings.Index(lower, t); i >= 0 && (at < 0 || i < at) {
			at = i
		}
	}
	if at < 0 {
		return nil
	}
	start, end := at-60, at+60
	prefix, suffix := "…", "…"
	if start <= 0 {
		start, prefix = 0, ""
	}
	if end >= len(flat) {
		end, suffix = len(flat), ""
	}
	// Don't cut a multi-byte character in half.
	for start > 0 && !utf8.RuneStart(flat[start]) {
		start--
	}
	for end < len(flat) && !utf8.RuneStart(flat[end]) {
		end++
	}
	v := prefix + strings.TrimSpace(flat[start:end]) + suffix
	return &v
}

func containsUUID(list []uuid.UUID, id uuid.UUID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// ListActivity is the home screen's recent activity (FR-DASH-05).
func (s *Server) ListActivity(w http.ResponseWriter, r *http.Request, params apigen.ListActivityParams) {
	a := mustAuth(r)
	limit := 10
	if params.Limit != nil {
		limit = *params.Limit
	}
	rows, err := s.db.ListActivity(r.Context(), store.ListActivityParams{HouseholdID: a.HouseholdID, RowLimit: int32(limit)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	actions := map[string]apigen.ActivityItemAction{
		"create": apigen.BillAdded, "payment_added": apigen.PaymentAdded, "payment_deleted": apigen.PaymentRemoved,
		"attach_document": apigen.DocumentAdded,
	}
	out := make([]apigen.ActivityItem, 0, len(rows))
	for _, row := range rows {
		item := apigen.ActivityItem{
			Id: row.ID, At: row.At, ActorName: row.ActorName, ProfileId: row.ProfileID, ProfileName: row.ProfileName,
			PeriodKey: row.PeriodKey, Description: row.Description, Currency: row.Currency,
		}
		var changes map[string]any
		_ = json.Unmarshal(row.Changes, &changes)
		if row.EntityType == "profile" {
			item.Action = apigen.ProfileAdded
		} else {
			item.Action = actions[row.Action]
			item.BillId = ptr(row.EntityID)
			switch row.Action {
			case "create":
				item.AmountMinor = row.PayableMinor
			case "payment_added", "payment_deleted":
				if v, ok := changes["amountMinor"].(float64); ok {
					item.AmountMinor = ptr(int64(v))
				}
				if m, ok := changes["method"].(string); ok {
					item.Detail = &m
				}
			case "attach_document":
				if pair, ok := changes["document"].([]any); ok && len(pair) == 2 {
					if name, ok := pair[1].(string); ok {
						item.Detail = &name
					}
				}
			}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}
