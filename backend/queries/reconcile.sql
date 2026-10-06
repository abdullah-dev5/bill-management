-- name: CreateBillSource :one
INSERT INTO bill_sources (id, household_id, bill_id, kind, document_id, extraction_id, connector, field_values, filled_fields, merged, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: ListBillSources :many
SELECT s.*, d.file_name AS document_name, u.display_name AS created_by_name
FROM bill_sources s
LEFT JOIN documents d ON d.id = s.document_id
LEFT JOIN users u ON u.id = s.created_by
WHERE s.bill_id = $1 AND s.household_id = $2
ORDER BY s.created_at, s.id;

-- name: GetBillSource :one
SELECT * FROM bill_sources WHERE id = $1 AND bill_id = $2 AND household_id = $3;

-- name: DeleteBillSource :exec
DELETE FROM bill_sources WHERE id = $1;

-- name: CreateBillConflict :one
INSERT INTO bill_conflicts (id, household_id, bill_id, source_id, field, bill_value, source_value, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListBillConflicts :many
SELECT c.*, s.kind AS source_kind, u.display_name AS resolved_by_name
FROM bill_conflicts c
JOIN bill_sources s ON s.id = c.source_id
LEFT JOIN users u ON u.id = c.resolved_by
WHERE c.bill_id = $1 AND c.household_id = $2
ORDER BY c.status, c.created_at;

-- name: GetBillConflict :one
SELECT * FROM bill_conflicts WHERE id = $1 AND bill_id = $2 AND household_id = $3;

-- name: OpenConflictForField :one
SELECT * FROM bill_conflicts WHERE bill_id = $1 AND field = $2 AND status = 'open' LIMIT 1;

-- name: ResolveBillConflict :exec
UPDATE bill_conflicts SET status = 'resolved', resolution = $2, resolved_value = $3, resolved_by = $4, resolved_at = $5
WHERE id = $1;

-- name: CountOpenConflicts :one
SELECT count(DISTINCT bill_id) FROM bill_conflicts c
JOIN bills b ON b.id = c.bill_id
WHERE c.household_id = $1 AND c.status = 'open' AND b.archived_at IS NULL;

-- name: OpenConflictCounts :many
-- Open conflicts per bill, for the bills in a list.
SELECT bill_id, count(*)::int AS open FROM bill_conflicts
WHERE bill_id = ANY (sqlc.arg(bill_ids)::uuid[]) AND status = 'open'
GROUP BY bill_id;
