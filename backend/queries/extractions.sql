-- name: CreateExtraction :one
INSERT INTO extractions (id, household_id, document_id, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLatestExtraction :one
SELECT * FROM extractions
WHERE document_id = $1 AND household_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: CountExtractionsSince :one
SELECT count(*) FROM extractions WHERE household_id = $1 AND created_at >= $2;

-- name: ClaimExtraction :one
-- Takes the oldest queued job, or a running one whose worker died (lock expired).
UPDATE extractions SET
    status = 'running', attempts = attempts + 1, started_at = now(), locked_until = sqlc.arg(locked_until)
WHERE id = (
    SELECT id FROM extractions
    WHERE attempts < 3
      AND (status = 'queued' OR (status = 'running' AND locked_until < now()))
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;

-- name: FinishExtraction :exec
UPDATE extractions SET
    status = $2, result = $3, trace = $4, error = $5, finished_at = now(), locked_until = NULL
WHERE id = $1;

-- name: FailAbandonedExtractions :execrows
UPDATE extractions SET
    status = 'failed', error = 'Reading this document kept failing. Please enter the details yourself.',
    finished_at = now(), locked_until = NULL
WHERE status IN ('queued', 'running') AND attempts >= 3 AND (locked_until IS NULL OR locked_until < now());

-- name: SetDocumentText :exec
UPDATE documents SET ocr_text = $2, updated_at = now() WHERE id = $1;

-- name: ListProviderHints :many
SELECT id, short_name, name, ref_number_pattern FROM providers
WHERE (household_id IS NULL OR household_id = $1) AND archived_at IS NULL
ORDER BY household_id NULLS LAST, short_name;
