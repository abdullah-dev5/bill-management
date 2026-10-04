-- +goose Up
-- Recent activity reads the household's latest audit events (FR-DASH-05).
CREATE INDEX audit_events_household_at_idx ON audit_events (household_id, at DESC);

-- +goose Down
DROP INDEX IF EXISTS audit_events_household_at_idx;
