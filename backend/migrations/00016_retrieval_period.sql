-- +goose Up
ALTER TABLE retrieval_runs ADD COLUMN period_key char(7);
CREATE INDEX retrieval_runs_profile_period_idx ON retrieval_runs (profile_id, period_key, trigger);

-- +goose Down
DROP INDEX IF EXISTS retrieval_runs_profile_period_idx;
ALTER TABLE retrieval_runs DROP COLUMN IF EXISTS period_key;
