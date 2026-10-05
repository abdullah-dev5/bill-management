package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// separateWindow is how long a merge can be undone (FR-RECO-07).
const separateWindow = 30 * 24 * time.Hour

func (s *Server) sourcesAndConflicts(ctx context.Context, a authInfo, billID uuid.UUID) ([]apigen.BillOrigin, []apigen.BillConflict, error) {
	srcRows, err := s.db.ListBillSources(ctx, store.ListBillSourcesParams{BillID: billID, HouseholdID: a.HouseholdID})
	if err != nil {
		return nil, nil, err
	}
	sources := make([]apigen.BillOrigin, 0, len(srcRows))
	for _, r := range srcRows {
		values := map[string]string{}
		_ = json.Unmarshal(r.FieldValues, &values)
		sources = append(sources, apigen.BillOrigin{
			Id: r.ID, Kind: apigen.BillOriginKind(r.Kind), DocumentId: r.DocumentID, DocumentName: r.DocumentName,
			Connector: r.Connector, Values: values, FilledFields: nonNilStrings(r.FilledFields), Merged: r.Merged,
			CanSeparate: ptr(r.Merged && s.now().Sub(r.CreatedAt) < separateWindow), CreatedByName: r.CreatedByName, CreatedAt: r.CreatedAt,
		})
	}
	cRows, err := s.db.ListBillConflicts(ctx, store.ListBillConflictsParams{BillID: billID, HouseholdID: a.HouseholdID})
	if err != nil {
		return nil, nil, err
	}
	conflicts := make([]apigen.BillConflict, 0, len(cRows))
	for _, c := range cRows {
		item := apigen.BillConflict{
			Id: c.ID, Field: c.Field, BillValue: c.BillValue, SourceValue: c.SourceValue, SourceId: c.SourceID,
			SourceKind: apigen.BillOriginKind(c.SourceKind), Status: apigen.BillConflictStatus(c.Status),
			ResolvedValue: c.ResolvedValue, ResolvedByName: c.ResolvedByName, ResolvedAt: c.ResolvedAt, CreatedAt: c.CreatedAt,
		}
		if c.Resolution != nil {
			res := apigen.BillConflictResolution(*c.Resolution)
			item.Resolution = &res
		}
		conflicts = append(conflicts, item)
	}
	return sources, conflicts, nil
}

// ResolveConflict settles one field conflict (FR-RECO-05): keep the bill's value, take the
// source's, type one, or record that the provider revised the bill. Audited.
func (s *Server) ResolveConflict(w http.ResponseWriter, r *http.Request, id apigen.Id, conflictID uuid.UUID) {
	a := mustAuth(r)
	var in apigen.ConflictResolution
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		c, err := q.GetBillConflict(ctx, store.GetBillConflictParams{ID: conflictID, BillID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("conflict")
		} else if err != nil {
			return err
		}
		if c.Status != store.ConflictStatusOpen {
			return errConflict("conflict_resolved", "This difference has already been settled.")
		}
		hh, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		row, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: s.today(hh.Timezone)})
		if err != nil {
			return err
		}
		before := billValues(row.Bill)[c.Field]
		value := before
		switch in.Action {
		case apigen.ConflictResolutionActionKeep:
		case apigen.ConflictResolutionActionSource, apigen.ConflictResolutionActionRevised:
			value = c.SourceValue
		case apigen.ConflictResolutionActionManual:
			if in.Value == nil || *in.Value == "" {
				return errField("value", "Enter the correct value.")
			}
			value = *in.Value
		default:
			return errField("action", "Choose how to settle this difference.")
		}
		if value != before {
			params := updateParams(row.Bill)
			if err := setField(&params, c.Field, value); err != nil {
				return err
			}
			if _, err := q.UpdateBill(ctx, params); err != nil {
				if store.UniqueViolation(err) == "bills_profile_period_uniq" {
					return errConflict("duplicate_bill", "Another bill in this profile already has that billing month.")
				}
				return err
			}
			if err := q.RecalcBillPaid(ctx, id); err != nil {
				return err
			}
		}
		if err := q.ResolveBillConflict(ctx, store.ResolveBillConflictParams{
			ID: c.ID, Resolution: ptr(string(in.Action)), ResolvedValue: &value, ResolvedBy: &a.UserID, ResolvedAt: ptr(s.now()),
		}); err != nil {
			return err
		}
		return audit(ctx, q, a, "bill", id, "conflict_resolved", map[string]any{
			c.Field: []any{before, value}, "resolution": string(in.Action), "source": c.SourceValue,
		})
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeBillDetail(w, r, http.StatusOK, a, id)
}

// SeparateSource undoes a merge (FR-RECO-07, docs/06 §5).
func (s *Server) SeparateSource(w http.ResponseWriter, r *http.Request, id apigen.Id, sourceID uuid.UUID) {
	a := mustAuth(r)
	ctx := r.Context()
	var detached *uuid.UUID
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		src, err := q.GetBillSource(ctx, store.GetBillSourceParams{ID: sourceID, BillID: id, HouseholdID: a.HouseholdID})
		if store.IsNotFound(err) {
			return errNotFound("source")
		} else if err != nil {
			return err
		}
		if !src.Merged {
			return errConflict("not_merged", "This is how the bill was first added; it can't be separated from itself.")
		}
		if s.now().Sub(src.CreatedAt) >= separateWindow {
			return errConflict("too_late", "Merges can be undone for 30 days. Edit the bill instead.")
		}
		hh, err := q.GetHousehold(ctx, a.HouseholdID)
		if err != nil {
			return err
		}
		row, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: a.HouseholdID, Today: s.today(hh.Timezone)})
		if err != nil {
			return err
		}
		// Empty again the fields this source filled, unless someone has changed them since.
		values := map[string]string{}
		_ = json.Unmarshal(src.FieldValues, &values)
		current := billValues(row.Bill)
		params := updateParams(row.Bill)
		var cleared []string
		for _, f := range src.FilledFields {
			if f == "periodKey" || current[f] != values[f] {
				continue
			}
			if err := setField(&params, f, ""); err == nil {
				cleared = append(cleared, f)
			}
		}
		if len(cleared) > 0 {
			if _, err := q.UpdateBill(ctx, params); err != nil {
				return err
			}
		}
		if src.DocumentID != nil {
			doc, err := q.GetDocument(ctx, store.GetDocumentParams{ID: *src.DocumentID, HouseholdID: a.HouseholdID})
			if err == nil && doc.BillID != nil && *doc.BillID == id {
				if _, err := q.UpdateDocument(ctx, store.UpdateDocumentParams{ID: doc.ID, HouseholdID: a.HouseholdID, BillID: nil, Kind: doc.Kind}); err != nil {
					return err
				}
				detached = &doc.ID
			} else if err != nil && !store.IsNotFound(err) {
				return err
			}
		}
		if err := q.DeleteBillSource(ctx, src.ID); err != nil { // its conflicts go with it
			return err
		}
		return audit(ctx, q, a, "bill", id, "merge_undone", map[string]any{"source": string(src.Kind), "cleared": nonNilStrings(cleared)})
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	detail, err := s.billDetail(ctx, a, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Bill       apigen.BillDetail `json:"bill"`
		DocumentID *uuid.UUID        `json:"documentId"`
	}{detail, detached})
}
