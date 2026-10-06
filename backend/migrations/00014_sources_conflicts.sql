-- +goose Up
-- One canonical bill, many sources (docs/06, FR-RECO-04…07): every channel that produced or
-- confirmed a bill is recorded with the values it gave; disagreements become conflicts that a
-- person resolves. A bill's own columns are never overwritten silently.
CREATE TYPE bill_source_kind AS ENUM ('manual', 'scan', 'upload', 'official', 'auto');

CREATE TABLE bill_sources (
    id            uuid PRIMARY KEY,
    household_id  uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    bill_id       uuid NOT NULL REFERENCES bills (id) ON DELETE CASCADE,
    kind          bill_source_kind NOT NULL,
    document_id   uuid REFERENCES documents (id) ON DELETE SET NULL,
    extraction_id uuid REFERENCES extractions (id) ON DELETE SET NULL,
    connector     text,                       -- official sources: which connector
    field_values  jsonb NOT NULL DEFAULT '{}', -- {field: value} as given by this source
    filled_fields text[] NOT NULL DEFAULT '{}', -- empty bill fields this source filled (undone on "separate")
    merged        boolean NOT NULL DEFAULT false, -- attached to an existing bill (vs. created it)
    created_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bill_sources_bill_idx ON bill_sources (bill_id, created_at);

CREATE TYPE conflict_status AS ENUM ('open', 'resolved');

CREATE TABLE bill_conflicts (
    id             uuid PRIMARY KEY,
    household_id   uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    bill_id        uuid NOT NULL REFERENCES bills (id) ON DELETE CASCADE,
    source_id      uuid NOT NULL REFERENCES bill_sources (id) ON DELETE CASCADE,
    field          text NOT NULL,
    bill_value     text NOT NULL,  -- the bill's value when the conflict arose
    source_value   text NOT NULL,  -- what the new source says
    status         conflict_status NOT NULL DEFAULT 'open',
    resolution     text,           -- keep | source | manual | revised
    resolved_value text,
    resolved_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    resolved_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bill_conflicts_bill_idx ON bill_conflicts (bill_id, created_at);
CREATE INDEX bill_conflicts_open_idx ON bill_conflicts (household_id) WHERE status = 'open';

-- Existing bills: record how each one arrived (a bill document's camera/upload, else typed in).
INSERT INTO bill_sources (id, household_id, bill_id, kind, document_id, field_values, created_by, created_at)
SELECT gen_random_uuid(), b.household_id, b.id,
       CASE d.source WHEN 'camera' THEN 'scan'::bill_source_kind WHEN 'upload' THEN 'upload'::bill_source_kind ELSE 'manual'::bill_source_kind END,
       d.id,
       jsonb_strip_nulls(jsonb_build_object(
           'periodKey', b.period_key, 'billNumber', b.bill_number,
           'issueDate', to_char(b.issue_date, 'YYYY-MM-DD'), 'dueDate', to_char(b.due_date, 'YYYY-MM-DD'),
           'currentChargesMinor', b.current_charges_minor::text, 'arrearsMinor', NULLIF(b.arrears_minor, 0)::text,
           'payableMinor', b.payable_minor::text, 'payableAfterDueMinor', b.payable_after_due_minor::text,
           'previousReading', b.previous_reading::text, 'currentReading', b.current_reading::text, 'units', b.units::text)),
       b.created_by, b.created_at
FROM bills b
LEFT JOIN LATERAL (
    SELECT id, source FROM documents WHERE bill_id = b.id AND kind = 'bill' ORDER BY created_at LIMIT 1
) d ON true;

-- +goose Down
DROP TABLE IF EXISTS bill_conflicts;
DROP TYPE IF EXISTS conflict_status;
DROP TABLE IF EXISTS bill_sources;
DROP TYPE IF EXISTS bill_source_kind;
