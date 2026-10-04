package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/pressly/goose/v3"

	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
)

func init() {
	goose.AddMigrationContext(encryptReferencesUp, encryptReferencesDown)
}

var errNoKeys = errors.New("migration 00004 needs DATA_ENCRYPTION_KEY to encrypt existing reference numbers")

// encryptReferencesUp moves plain reference numbers into reference_hash / reference_enc
// and masks reference numbers recorded in profile audit history.
func encryptReferencesUp(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, reference_number, coalesce(reference_display, reference_number)
		 FROM bill_profiles WHERE reference_number IS NOT NULL`)
	if err != nil {
		return err
	}
	type ref struct{ id, norm, display string }
	var refs []ref
	for rows.Next() {
		var r ref
		if err := rows.Scan(&r.id, &r.norm, &r.display); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, r)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(refs) > 0 && runKeys == nil {
		return errNoKeys
	}
	for _, r := range refs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE bill_profiles SET reference_hash = $2, reference_enc = $3 WHERE id = $1`,
			r.id, runKeys.Hash(r.norm), runKeys.Seal(r.display)); err != nil {
			return err
		}
	}
	return maskAuditedReferences(ctx, tx)
}

func maskAuditedReferences(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, changes FROM audit_events WHERE entity_type = 'profile' AND changes ? 'referenceNumber'`)
	if err != nil {
		return err
	}
	type event struct {
		id      string
		changes map[string]any
	}
	var events []event
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		var changes map[string]any
		if err := json.Unmarshal(raw, &changes); err != nil {
			rows.Close()
			return err
		}
		events = append(events, event{id, changes})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, e := range events {
		if pair, ok := e.changes["referenceNumber"].([]any); ok {
			for i, v := range pair {
				if s, ok := v.(string); ok && s != "" {
					pair[i] = secure.Mask(s)
				}
			}
		}
		body, err := json.Marshal(e.changes)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE audit_events SET changes = $2 WHERE id = $1`, e.id, body); err != nil {
			return err
		}
	}
	return nil
}

// encryptReferencesDown decrypts back into the plain columns restored by 00005's down step.
// Masked audit history stays masked.
func encryptReferencesDown(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, reference_enc FROM bill_profiles WHERE reference_enc IS NOT NULL`)
	if err != nil {
		return err
	}
	type enc struct {
		id     string
		sealed []byte
	}
	var encs []enc
	for rows.Next() {
		var e enc
		if err := rows.Scan(&e.id, &e.sealed); err != nil {
			rows.Close()
			return err
		}
		encs = append(encs, e)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(encs) > 0 && runKeys == nil {
		return errNoKeys
	}
	for _, e := range encs {
		display, err := runKeys.Open(e.sealed)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE bill_profiles SET reference_display = $2, reference_number = $3 WHERE id = $1`,
			e.id, display, domain.NormalizeReference(display)); err != nil {
			return err
		}
	}
	return nil
}
