-- name: ListCategories :many
SELECT * FROM categories
WHERE household_id IS NULL OR household_id = $1
ORDER BY sort_order, lower(name);

-- name: GetCategory :one
-- Visible to the household: a system type or one of its own.
SELECT * FROM categories
WHERE id = $1 AND (household_id IS NULL OR household_id = $2);

-- name: CategoryNameTaken :one
SELECT EXISTS (
    SELECT 1 FROM categories
    WHERE (household_id IS NULL OR household_id = $1)
      AND lower(name) = lower(sqlc.arg(name))
      AND archived_at IS NULL
      AND id <> sqlc.arg(exclude_id)
);

-- name: NextCategorySortOrder :one
SELECT (coalesce(max(sort_order), 1000) + 10)::integer FROM categories WHERE household_id = $1;

-- name: CreateCategory :one
INSERT INTO categories (id, household_id, name, icon, default_kind, type_group, parent_id, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories
SET name = $3, icon = $4, default_kind = $5, type_group = $6, parent_id = $7, updated_at = now()
WHERE id = $1 AND household_id = $2
RETURNING *;

-- name: ArchiveCategory :exec
UPDATE categories SET archived_at = now(), updated_at = now()
WHERE id = $1 AND household_id = $2;

-- name: CountActiveProfilesForCategory :one
SELECT count(*) FROM bill_profiles
WHERE category_id = $1 AND household_id = $2 AND archived_at IS NULL;

-- name: ListProviders :many
SELECT * FROM providers
WHERE (household_id IS NULL OR household_id = $1)
  AND archived_at IS NULL
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
ORDER BY household_id NULLS FIRST, short_name;

-- name: GetProvider :one
SELECT * FROM providers
WHERE id = $1 AND (household_id IS NULL OR household_id = $2) AND archived_at IS NULL;

-- name: CreateProvider :one
INSERT INTO providers (id, household_id, name, short_name, category_id, ref_number_label, consumption_unit)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
