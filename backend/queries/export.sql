-- name: ExportBills :many
-- Bills for CSV export and backup (FR-EXP-01/02), oldest first, by bill date.
SELECT
    sqlc.embed(bills),
    bill_status(
        bills.cancelled, bills.verified_at IS NOT NULL, bills.due_date, bills.payable_minor,
        bills.payable_after_due_minor, bills.paid_minor, bills.paid_on_time_minor,
        bills.late_step_until, bills.payable_final_minor, bills.paid_by_step_minor, bills.settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        bills.cancelled, bills.verified_at IS NOT NULL, bills.due_date, bills.payable_minor,
        bills.payable_after_due_minor, bills.paid_minor, bills.paid_on_time_minor,
        bills.late_step_until, bills.payable_final_minor, bills.paid_by_step_minor, bills.settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::bigint AS outstanding_minor,
    p.name AS profile_name, c.name AS type_name, pr.short_name AS provider_name
FROM bills
JOIN bill_profiles p ON p.id = bills.profile_id
JOIN categories c ON c.id = bills.category_id
LEFT JOIN providers pr ON pr.id = p.provider_id
WHERE bills.household_id = sqlc.arg(household_id)
  AND (sqlc.narg(from_date)::date IS NULL OR bills.expense_date >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR bills.expense_date <= sqlc.narg(to_date)::date)
ORDER BY bills.expense_date, p.name, bills.id;

-- name: ExportPayments :many
SELECT py.*, p.name AS profile_name, b.period_key, b.description AS bill_description, b.expense_date AS bill_date
FROM payments py
JOIN bills b ON b.id = py.bill_id
JOIN bill_profiles p ON p.id = b.profile_id
WHERE py.household_id = sqlc.arg(household_id)
  AND (sqlc.narg(from_date)::date IS NULL OR py.paid_on >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR py.paid_on <= sqlc.narg(to_date)::date)
ORDER BY py.paid_on, py.created_at;

-- name: ExportDocuments :many
SELECT * FROM documents
WHERE household_id = $1 AND status = 'ready'
ORDER BY created_at;

-- name: ExportCustomTypes :many
SELECT * FROM categories WHERE household_id = $1 ORDER BY created_at;

-- name: ExportCustomProviders :many
SELECT * FROM providers WHERE household_id = $1 ORDER BY created_at;

-- name: ExportProfileHistory :many
SELECT * FROM profile_history
WHERE household_id = $1
ORDER BY profile_id, period_key;
