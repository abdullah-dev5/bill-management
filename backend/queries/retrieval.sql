-- name: SetProfileRetrievalEnabled :exec
UPDATE bill_profiles SET retrieval_enabled = $3 WHERE id = $1 AND household_id = $2;

-- name: SetProfileRetrievalResult :exec
UPDATE bill_profiles SET last_retrieval_at = $2, last_retrieval_outcome = $3, last_retrieval_message = $4 WHERE id = $1;

-- name: ListRetrievalCandidates :many
-- Profiles opted into automatic checks that haven't been checked since cutoff (≤ 1 a day, FR-RETR-03).
SELECT sqlc.embed(p), pr.short_name AS provider_short, h.timezone, h.currency
FROM bill_profiles p
JOIN providers pr ON pr.id = p.provider_id
JOIN households h ON h.id = p.household_id
WHERE p.retrieval_enabled AND p.archived_at IS NULL AND p.reference_enc IS NOT NULL
  AND (p.last_retrieval_at IS NULL OR p.last_retrieval_at < sqlc.arg(cutoff))
ORDER BY p.last_retrieval_at NULLS FIRST
LIMIT sqlc.arg(row_limit);

-- name: GetConnectorHealth :one
SELECT * FROM retrieval_connectors WHERE id = $1;

-- name: ListConnectorHealth :many
SELECT * FROM retrieval_connectors ORDER BY id;

-- name: ConnectorSucceeded :exec
INSERT INTO retrieval_connectors (id, consecutive_failures, last_success_at, updated_at)
VALUES ($1, 0, $2, $2)
ON CONFLICT (id) DO UPDATE SET consecutive_failures = 0, last_success_at = $2, disabled_until = NULL, updated_at = $2;

-- name: ConnectorFailed :one
-- Counts a failure; at the threshold the connector is switched off until disable_until (FR-RETR-05).
INSERT INTO retrieval_connectors (id, consecutive_failures, last_failure_at, last_error, updated_at)
VALUES (sqlc.arg(id), 1, sqlc.arg(now), sqlc.arg(error), sqlc.arg(now))
ON CONFLICT (id) DO UPDATE SET
    consecutive_failures = retrieval_connectors.consecutive_failures + 1,
    last_failure_at = sqlc.arg(now), last_error = sqlc.arg(error), updated_at = sqlc.arg(now),
    disabled_until = CASE WHEN retrieval_connectors.consecutive_failures + 1 >= sqlc.arg(threshold)::int
                          THEN sqlc.arg(disable_until)::timestamptz ELSE retrieval_connectors.disabled_until END
RETURNING *;

-- name: CreateRetrievalRun :exec
INSERT INTO retrieval_runs (id, household_id, profile_id, connector, trigger, outcome, bill_id, message, started_at, finished_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: FindDocumentBySHA :one
SELECT id, bill_id FROM documents WHERE household_id = $1 AND sha256 = $2;

-- name: ListHouseholdMemberIDs :many
SELECT user_id FROM memberships WHERE household_id = $1;
