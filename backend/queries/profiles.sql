-- name: ListProfiles :many
SELECT * FROM bill_profiles
WHERE household_id = $1 AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY archived_at NULLS FIRST, sort_order, created_at;

-- name: GetProfile :one
SELECT * FROM bill_profiles WHERE id = $1 AND household_id = $2;

-- name: ListProfileColors :many
SELECT color FROM bill_profiles WHERE household_id = $1 AND archived_at IS NULL;

-- name: NextProfileSortOrder :one
SELECT (coalesce(max(sort_order), 0) + 1)::integer FROM bill_profiles WHERE household_id = $1;

-- name: CreateProfile :one
INSERT INTO bill_profiles (
    id, household_id, property_id, category_id, kind, name, icon, color,
    provider_id, reference_number, reference_display, meter_number, multiplying_factor,
    consumption_unit, billing_cycle, typical_issue_day, typical_due_day,
    expected_amount_minor, auto_create_bill, is_default, pinned, sort_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19, $20, $21, $22
)
RETURNING *;

-- name: UpdateProfile :one
UPDATE bill_profiles SET
    category_id = $3, kind = $4, name = $5, icon = $6, color = $7,
    provider_id = $8, reference_number = $9, reference_display = $10, meter_number = $11,
    multiplying_factor = $12, consumption_unit = $13, billing_cycle = $14,
    typical_issue_day = $15, typical_due_day = $16, expected_amount_minor = $17,
    auto_create_bill = $18, pinned = $19, updated_at = now()
WHERE id = $1 AND household_id = $2
RETURNING *;

-- name: ArchiveProfile :execrows
UPDATE bill_profiles SET archived_at = now(), pinned = false, updated_at = now()
WHERE id = $1 AND household_id = $2 AND archived_at IS NULL AND NOT is_default;

-- name: RestoreProfile :execrows
UPDATE bill_profiles SET archived_at = NULL, updated_at = now()
WHERE id = $1 AND household_id = $2 AND archived_at IS NOT NULL;

-- name: SetProfileSortOrder :execrows
UPDATE bill_profiles SET sort_order = $3, updated_at = now()
WHERE id = $1 AND household_id = $2;

-- name: ProfileSummaries :many
-- Per-profile totals. Spend excludes arrears (docs/04-domain-model.md §4).
SELECT
    p.id AS profile_id,
    count(b.id)::bigint AS bill_count,
    coalesce(sum(bill_outstanding(
        b.cancelled, b.verified_at IS NOT NULL, b.due_date, b.payable_minor,
        b.payable_after_due_minor, b.paid_minor, b.paid_on_time_minor, sqlc.arg(today)::date
    )), 0)::bigint AS outstanding_minor,
    coalesce(sum(b.current_charges_minor + b.adjustments_minor) FILTER (
        WHERE NOT b.cancelled AND b.verified_at IS NOT NULL
          AND left(coalesce(b.period_key, to_char(b.expense_date, 'YYYY-MM')), 4) = sqlc.arg(year)::text
    ), 0)::bigint AS this_year_minor
FROM bill_profiles p
LEFT JOIN bills b ON b.profile_id = p.id AND b.archived_at IS NULL
WHERE p.household_id = sqlc.arg(household_id)
GROUP BY p.id;

-- name: LatestBillPerProfile :many
SELECT DISTINCT ON (bills.profile_id)
    sqlc.embed(bills),
    bill_status(
        bills.cancelled, bills.verified_at IS NOT NULL, bills.due_date, bills.payable_minor,
        bills.payable_after_due_minor, bills.paid_minor, bills.paid_on_time_minor, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        bills.cancelled, bills.verified_at IS NOT NULL, bills.due_date, bills.payable_minor,
        bills.payable_after_due_minor, bills.paid_minor, bills.paid_on_time_minor, sqlc.arg(today)::date
    )::bigint AS outstanding_minor
FROM bills
WHERE bills.household_id = sqlc.arg(household_id) AND bills.archived_at IS NULL
ORDER BY bills.profile_id,
         coalesce(bills.period_key, to_char(bills.expense_date, 'YYYY-MM')) DESC,
         bills.expense_date DESC,
         bills.created_at DESC;

-- name: CountProfileBills :one
SELECT count(*) FROM bills WHERE profile_id = $1;
