-- +goose Up

-- Arrears settlement (docs/04 §3): when a later bill carries this bill's unpaid amount as
-- arrears and the later bill is paid in full, this bill is settled through it.
ALTER TABLE bills ADD COLUMN settled_by_bill_id uuid REFERENCES bills (id) ON DELETE SET NULL;
CREATE INDEX bills_settled_by_idx ON bills (settled_by_bill_id) WHERE settled_by_bill_id IS NOT NULL;

-- Receipts: a document can belong to a payment of its bill (FR-PAY-03).
ALTER TABLE documents ADD COLUMN payment_id uuid REFERENCES payments (id) ON DELETE SET NULL;

DROP FUNCTION bill_status(boolean, boolean, date, bigint, bigint, bigint, bigint, date, bigint, bigint, date);
DROP FUNCTION bill_outstanding(boolean, boolean, date, bigint, bigint, bigint, bigint, date, bigint, bigint, date);

-- +goose StatementBegin
CREATE FUNCTION bill_status(
    cancelled boolean, verified boolean, due_date date,
    payable bigint, payable_after_due bigint,
    paid bigint, paid_on_time bigint,
    late_step_until date, payable_final bigint, paid_by_step bigint,
    settled boolean, today date
) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled THEN 'cancelled'
        WHEN NOT verified THEN 'draft'
        WHEN settled THEN 'paid'
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
    settled boolean, today date
) RETURNS bigint LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled OR NOT verified OR settled THEN 0
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

-- +goose Down
DROP FUNCTION IF EXISTS bill_status(boolean, boolean, date, bigint, bigint, bigint, bigint, date, bigint, bigint, boolean, date);
DROP FUNCTION IF EXISTS bill_outstanding(boolean, boolean, date, bigint, bigint, bigint, bigint, date, bigint, bigint, boolean, date);

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

ALTER TABLE documents DROP COLUMN IF EXISTS payment_id;
DROP INDEX IF EXISTS bills_settled_by_idx;
ALTER TABLE bills DROP COLUMN IF EXISTS settled_by_bill_id;
