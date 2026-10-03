-- +goose Up
-- Plain reference numbers were moved to reference_hash / reference_enc by 00004.
ALTER TABLE bill_profiles DROP COLUMN reference_number, DROP COLUMN reference_display;
CREATE UNIQUE INDEX bill_profiles_ref_uniq
    ON bill_profiles (household_id, provider_id, reference_hash)
    WHERE archived_at IS NULL AND reference_hash IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS bill_profiles_ref_uniq;
ALTER TABLE bill_profiles ADD COLUMN reference_number text, ADD COLUMN reference_display text;
CREATE UNIQUE INDEX bill_profiles_ref_uniq
    ON bill_profiles (household_id, provider_id, reference_number)
    WHERE archived_at IS NULL AND reference_number IS NOT NULL;
