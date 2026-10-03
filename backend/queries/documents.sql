-- name: CreateDocument :one
INSERT INTO documents (
    id, household_id, bill_id, kind, source, storage_key, file_name,
    content_type, size_bytes, sha256, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetDocument :one
SELECT * FROM documents WHERE id = $1 AND household_id = $2;

-- name: GetDocumentByID :one
-- Unscoped: only for signed file links, where the signature is the authorisation.
SELECT * FROM documents WHERE id = $1;

-- name: GetDocumentBySHA256 :one
SELECT * FROM documents WHERE household_id = $1 AND sha256 = $2;

-- name: ListBillDocuments :many
SELECT * FROM documents
WHERE household_id = $1 AND bill_id = $2 AND status = 'ready'
ORDER BY created_at;

-- name: ListUnattachedDocuments :many
SELECT * FROM documents
WHERE household_id = $1 AND bill_id IS NULL AND status = 'ready'
ORDER BY created_at DESC
LIMIT 50;

-- name: RefreshPendingDocument :one
-- Re-issuing an upload for an abandoned pending row with the same file.
UPDATE documents SET bill_id = $3, kind = $4, source = $5, file_name = $6,
    content_type = $7, size_bytes = $8, created_by = $9, created_at = now(), updated_at = now()
WHERE id = $1 AND household_id = $2 AND status = 'pending'
RETURNING *;

-- name: MarkDocumentReady :one
UPDATE documents SET status = 'ready', uploaded_at = now(), updated_at = now()
WHERE id = $1 AND household_id = $2
RETURNING *;

-- name: UpdateDocument :one
UPDATE documents SET bill_id = $3, kind = $4, updated_at = now()
WHERE id = $1 AND household_id = $2
RETURNING *;

-- name: DeleteDocument :execrows
DELETE FROM documents WHERE id = $1 AND household_id = $2;

-- name: HouseholdStorageUsage :one
SELECT coalesce(sum(size_bytes), 0)::bigint AS used_bytes,
       count(*) FILTER (WHERE status = 'ready')::bigint AS document_count
FROM documents WHERE household_id = $1;

-- name: DeleteStalePendingDocuments :execrows
DELETE FROM documents WHERE status = 'pending' AND created_at < $1;

-- name: ListStorageDeletions :many
SELECT storage_key FROM storage_deletions ORDER BY enqueued_at LIMIT $1;

-- name: RemoveStorageDeletion :exec
DELETE FROM storage_deletions WHERE storage_key = $1;
