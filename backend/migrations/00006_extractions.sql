-- +goose Up
ALTER TABLE documents
    ADD COLUMN page_count integer,
    ADD COLUMN ocr_text   text; -- redacted document text, for search (FR-DOC-03)

CREATE TYPE extraction_status AS ENUM ('queued', 'running', 'succeeded', 'failed');

-- One row per extraction run; also the job queue (claimed with FOR UPDATE SKIP LOCKED).
CREATE TABLE extractions (
    id           uuid PRIMARY KEY,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    document_id  uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    status       extraction_status NOT NULL DEFAULT 'queued',
    attempts     integer NOT NULL DEFAULT 0,
    locked_until timestamptz,
    result       jsonb,
    trace        jsonb NOT NULL DEFAULT '[]',
    error        text,
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    started_at   timestamptz,
    finished_at  timestamptz
);
CREATE INDEX extractions_document_idx ON extractions (document_id, created_at DESC);
CREATE INDEX extractions_queue_idx ON extractions (created_at) WHERE status IN ('queued', 'running');
CREATE INDEX extractions_household_idx ON extractions (household_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS extractions;
DROP TYPE IF EXISTS extraction_status;
ALTER TABLE documents DROP COLUMN IF EXISTS ocr_text, DROP COLUMN IF EXISTS page_count;
