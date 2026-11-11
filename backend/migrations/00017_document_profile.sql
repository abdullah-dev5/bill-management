-- +goose Up
ALTER TABLE documents
    ADD COLUMN profile_id uuid REFERENCES bill_profiles (id) ON DELETE CASCADE;

CREATE INDEX documents_profile_idx ON documents (profile_id, created_at)
    WHERE profile_id IS NOT NULL;

-- Backfill documents already attached to bills.
UPDATE documents d
SET profile_id = b.profile_id
FROM bills b
WHERE d.bill_id = b.id AND d.profile_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS documents_profile_idx;
ALTER TABLE documents DROP COLUMN IF EXISTS profile_id;
