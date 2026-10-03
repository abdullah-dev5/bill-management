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
SELECT period_key::text AS period_key, (payable_minor - arrears_minor)::bigint AS billed_minor, paid_minor, id
FROM bills
WHERE profile_id = $1 AND household_id = $2 AND archived_at IS NULL AND period_key IS NOT NULL
ORDER BY period_key;

-- name: CreateExtractionCorrection :exec
INSERT INTO extraction_corrections (id, household_id, extraction_id, field, extracted, confirmed, source, confidence)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetExtraction :one
SELECT * FROM extractions WHERE id = $1 AND household_id = $2;
