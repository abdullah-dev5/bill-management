-- +goose Up
-- Official retrieval (M6, ADR-002, FR-RETR-01…06). Two ways in:
--  * assisted: the provider's official public lookup page opens in the person's own browser; they
--    solve any CAPTCHA, save the bill, and upload it as an official copy. Used where providers don't
--    allow automated access (PITC and SSGC pages have CAPTCHAs).
--  * automatic: a connector fetches the bill on a schedule. Only for providers that permit it.

-- The official public lookup page (verified links only).
ALTER TABLE providers ADD COLUMN lookup_url text;
UPDATE providers SET lookup_url = 'https://bill.pitc.com.pk/' || lower(short_name) || 'bill'
WHERE household_id IS NULL AND short_name IN ('LESCO','IESCO','GEPCO','FESCO','MEPCO','PESCO','HESCO','QESCO','SEPCO','TESCO');
UPDATE providers SET lookup_url = 'https://www.ke.com.pk/know-your-bill/' WHERE id = 'b0000000-0000-4000-8000-000000000011';
UPDATE providers SET lookup_url = 'https://viewbill.ssgc.com.pk/web' WHERE id = 'b0000000-0000-4000-8000-000000000022';

-- A profile opts into automatic checks with retrieval_enabled (FR-RETR-01, column from 00001) and
-- remembers its last attempt (FR-RETR-03).
ALTER TABLE bill_profiles
    ADD COLUMN last_retrieval_at      timestamptz,
    ADD COLUMN last_retrieval_outcome text,
    ADD COLUMN last_retrieval_message text;

-- Connector health (FR-RETR-05): repeated failures disable a connector until disabled_until.
CREATE TABLE retrieval_connectors (
    id                   text PRIMARY KEY,
    consecutive_failures integer NOT NULL DEFAULT 0,
    last_success_at      timestamptz,
    last_failure_at      timestamptz,
    last_error           text,
    disabled_until       timestamptz,
    updated_at           timestamptz NOT NULL DEFAULT now()
);

-- Every attempt, for health and support.
CREATE TABLE retrieval_runs (
    id           uuid PRIMARY KEY,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    profile_id   uuid NOT NULL REFERENCES bill_profiles (id) ON DELETE CASCADE,
    connector    text NOT NULL,
    trigger      text NOT NULL,            -- schedule | manual
    outcome      text NOT NULL,            -- created | merged | unchanged | not_found | failed | skipped
    bill_id      uuid REFERENCES bills (id) ON DELETE SET NULL,
    message      text,
    started_at   timestamptz NOT NULL,
    finished_at  timestamptz NOT NULL
);
CREATE INDEX retrieval_runs_profile_idx ON retrieval_runs (profile_id, started_at DESC);

-- "New bill retrieved" (FR-REM-06).
ALTER TYPE notification_kind ADD VALUE IF NOT EXISTS 'bill_retrieved';

-- +goose Down
DROP TABLE IF EXISTS retrieval_runs;
DROP TABLE IF EXISTS retrieval_connectors;
ALTER TABLE bill_profiles
    DROP COLUMN IF EXISTS last_retrieval_message,
    DROP COLUMN IF EXISTS last_retrieval_outcome,
    DROP COLUMN IF EXISTS last_retrieval_at;
ALTER TABLE providers DROP COLUMN IF EXISTS lookup_url;
