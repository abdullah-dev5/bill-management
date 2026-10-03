-- +goose Up

-- Stepped late fees: many Pakistani bills charge one amount until a date after the due date
-- and a higher one after it (e.g. "Till 08-OCT-26 3,389 / After 08-OCT-26 3,512").
ALTER TABLE bills
    ADD COLUMN late_step_until     date,
    ADD COLUMN payable_final_minor bigint CHECK (payable_final_minor >= 0),
    ADD COLUMN paid_by_step_minor  bigint NOT NULL DEFAULT 0;

DROP FUNCTION bill_status(boolean, boolean, date, bigint, bigint, bigint, bigint, date);
DROP FUNCTION bill_outstanding(boolean, boolean, date, bigint, bigint, bigint, bigint, date);

-- +goose StatementBegin
CREATE FUNCTION bill_status(
    cancelled boolean, verified boolean, due_date date,
    payable bigint, payable_after_due bigint,
    paid bigint, paid_on_time bigint,
    late_step_until date, payable_final bigint, paid_by_step bigint,
    today date
) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled THEN 'cancelled'
        WHEN NOT verified THEN 'draft'
        WHEN paid_on_time >= payable THEN 'paid'
        WHEN late_step_until IS NOT NULL AND payable_final IS NOT NULL
             AND paid_by_step >= coalesce(payable_after_due, payable) THEN 'paid'
        WHEN paid >= coalesce(payable_final, payable_after_due, payable) THEN 'paid'
        WHEN due_date IS NOT NULL AND today > due_date THEN 'overdue'
        WHEN paid > 0 THEN 'partially_paid'
        ELSE 'unpaid'
    END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION bill_outstanding(
    cancelled boolean, verified boolean, due_date date,
    payable bigint, payable_after_due bigint,
    paid bigint, paid_on_time bigint,
    late_step_until date, payable_final bigint, paid_by_step bigint,
    today date
) RETURNS bigint LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled OR NOT verified THEN 0
        WHEN paid_on_time >= payable THEN 0
        WHEN late_step_until IS NOT NULL AND payable_final IS NOT NULL
             AND paid_by_step >= coalesce(payable_after_due, payable) THEN 0
        WHEN due_date IS NOT NULL AND today > due_date AND late_step_until IS NOT NULL
             AND payable_final IS NOT NULL AND today > late_step_until
            THEN greatest(payable_final - paid, 0)
        WHEN due_date IS NOT NULL AND today > due_date AND payable_after_due IS NOT NULL
            THEN greatest(payable_after_due - paid, 0)
        ELSE greatest(payable - paid, 0)
    END::bigint
$$;
-- +goose StatementEnd

-- Bill history printed on bills (e.g. the last 12 months), kept per profile and shown on its
-- dashboard next to the bills themselves.
CREATE TABLE profile_history (
    profile_id   uuid NOT NULL REFERENCES bill_profiles (id) ON DELETE CASCADE,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    period_key   char(7) NOT NULL CHECK (period_key ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    units        numeric(12, 2),
    billed_minor bigint,
    paid_minor   bigint,
    document_id  uuid REFERENCES documents (id) ON DELETE SET NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, period_key)
);

-- What extraction read vs what the user confirmed, to measure accuracy (FR-EXTR-14).
CREATE TABLE extraction_corrections (
    id            uuid PRIMARY KEY,
    household_id  uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    extraction_id uuid NOT NULL REFERENCES extractions (id) ON DELETE CASCADE,
    field         text NOT NULL,
    extracted     text,
    confirmed     text,
    source        text,
    confidence    numeric(4, 3),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX extraction_corrections_field_idx ON extraction_corrections (field, created_at);

-- +goose Down
DROP TABLE IF EXISTS extraction_corrections;
DROP TABLE IF EXISTS profile_history;
DROP FUNCTION IF EXISTS bill_status(boolean, boolean, date, bigint, bigint, bigint, bigint, date, bigint, bigint, date);
DROP FUNCTION IF EXISTS bill_outstanding(boolean, boolean, date, bigint, bigint, bigint, bigint, date, bigint, bigint, date);

-- +goose StatementBegin
CREATE FUNCTION bill_status(
    cancelled boolean, verified boolean, due_date date,
    payable bigint, payable_after_due bigint,
    paid bigint, paid_on_time bigint, today date
) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled THEN 'cancelled'
        WHEN NOT verified THEN 'draft'
        WHEN paid_on_time >= payable THEN 'paid'
        WHEN paid >= coalesce(payable_after_due, payable) THEN 'paid'
        WHEN due_date IS NOT NULL AND today > due_date THEN 'overdue'
        WHEN paid > 0 THEN 'partially_paid'
        ELSE 'unpaid'
    END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION bill_outstanding(
    cancelled boolean, verified boolean, due_date date,
    payable bigint, payable_after_due bigint,
    paid bigint, paid_on_time bigint, today date
) RETURNS bigint LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled OR NOT verified THEN 0
        WHEN paid_on_time >= payable THEN 0
        WHEN due_date IS NOT NULL AND today > due_date AND payable_after_due IS NOT NULL
            THEN greatest(payable_after_due - paid, 0)
        ELSE greatest(payable - paid, 0)
    END::bigint
$$;
-- +goose StatementEnd

ALTER TABLE bills DROP COLUMN IF EXISTS paid_by_step_minor,
    DROP COLUMN IF EXISTS payable_final_minor, DROP COLUMN IF EXISTS late_step_until;
