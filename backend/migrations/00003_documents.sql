-- +goose Up

-- Encrypted reference numbers (docs/08 §4): a keyed hash for matching and an
-- AES-GCM ciphertext for display. Filled from the plain columns by 00004.
ALTER TABLE bill_profiles
    ADD COLUMN reference_hash bytea,
    ADD COLUMN reference_enc  bytea;

-- Documents ---------------------------------------------------------------------

CREATE TYPE document_kind AS ENUM ('bill', 'receipt', 'other');
CREATE TYPE document_source AS ENUM ('camera', 'upload');
CREATE TYPE document_status AS ENUM ('pending', 'ready');

CREATE TABLE documents (
    id           uuid PRIMARY KEY,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    bill_id      uuid REFERENCES bills (id) ON DELETE CASCADE,
    kind         document_kind NOT NULL,
    source       document_source NOT NULL,
    status       document_status NOT NULL DEFAULT 'pending',
    storage_key  text NOT NULL UNIQUE,
    file_name    text NOT NULL,
    content_type text NOT NULL,
    size_bytes   bigint NOT NULL CHECK (size_bytes > 0),
    sha256       bytea NOT NULL CHECK (length(sha256) = 32),
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    uploaded_at  timestamptz
);
-- The same file is stored once per household (FR-RECO dedup by sha256).
CREATE UNIQUE INDEX documents_sha256_uniq ON documents (household_id, sha256);
CREATE INDEX documents_bill_idx ON documents (bill_id, created_at) WHERE bill_id IS NOT NULL;
CREATE INDEX documents_pending_idx ON documents (created_at) WHERE status = 'pending';

-- Objects to remove from storage. Filled by a trigger, so deletes that cascade from
-- bills or households also clean up storage; drained by the tick.
CREATE TABLE storage_deletions (
    storage_key text PRIMARY KEY,
    enqueued_at timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementBegin
CREATE FUNCTION enqueue_storage_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO storage_deletions (storage_key) VALUES (OLD.storage_key) ON CONFLICT DO NOTHING;
    RETURN OLD;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER documents_enqueue_storage_deletion
    AFTER DELETE ON documents
    FOR EACH ROW EXECUTE FUNCTION enqueue_storage_deletion();

-- +goose Down
DROP TRIGGER IF EXISTS documents_enqueue_storage_deletion ON documents;
DROP FUNCTION IF EXISTS enqueue_storage_deletion;
DROP TABLE IF EXISTS storage_deletions;
DROP TABLE IF EXISTS documents;
DROP TYPE IF EXISTS document_status;
DROP TYPE IF EXISTS document_source;
DROP TYPE IF EXISTS document_kind;
ALTER TABLE bill_profiles DROP COLUMN IF EXISTS reference_enc, DROP COLUMN IF EXISTS reference_hash;
