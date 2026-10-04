-- name: UpsertProfileHistory :exec
INSERT INTO profile_history (profile_id, household_id, period_key, units, billed_minor, paid_minor, document_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (profile_id, period_key) DO UPDATE SET
    units = coalesce(EXCLUDED.units, profile_history.units),
    billed_minor = coalesce(EXCLUDED.billed_minor, profile_history.billed_minor),
    paid_minor = coalesce(EXCLUDED.paid_minor, profile_history.paid_minor),
    document_id = EXCLUDED.document_id,
    updated_at = now();

-- name: ListProfileHistory :many
SELECT period_key, units, billed_minor, paid_minor FROM profile_history
WHERE profile_id = $1 AND household_id = $2
ORDER BY period_key;

-- name: ListProfileBillMonths :many
-- One row per billing month of a cycle-based profile, for the history view.
SELECT period_key::text AS period_key, (payable_minor - arrears_minor)::bigint AS billed_minor, paid_minor, id, units,
    bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status
FROM bills
WHERE profile_id = sqlc.arg(profile_id) AND household_id = sqlc.arg(household_id) AND archived_at IS NULL AND period_key IS NOT NULL
ORDER BY period_key;

-- name: CreateExtractionCorrection :exec
INSERT INTO extraction_corrections (id, household_id, extraction_id, field, extracted, confirmed, source, confidence)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetExtraction :one
SELECT * FROM extractions WHERE id = $1 AND household_id = $2;

-- name: ListProfilePayments :many
SELECT p.id, p.bill_id, p.paid_on, p.amount_minor, p.method, p.reference, p.notes, p.created_at,
       b.period_key, b.description, b.expense_date, d.id AS receipt_document_id
FROM payments p
JOIN bills b ON b.id = p.bill_id
LEFT JOIN documents d ON d.payment_id = p.id
WHERE b.profile_id = $1 AND p.household_id = $2
ORDER BY p.paid_on DESC, p.created_at DESC
LIMIT 200;

-- name: ListProfileDocuments :many
SELECT d.* FROM documents d
JOIN bills b ON b.id = d.bill_id
WHERE b.profile_id = $1 AND d.household_id = $2 AND d.status = 'ready'
ORDER BY d.created_at DESC
LIMIT 200;
