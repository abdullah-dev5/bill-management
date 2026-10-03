-- Status and outstanding are derived on read: see bill_status() / bill_outstanding() in 00001_init.sql.

-- name: CreateBill :one
INSERT INTO bills (
    id, household_id, profile_id, category_id, description, bill_number,
    period_key, period_start, period_end, issue_date, due_date, expense_date,
    current_charges_minor, arrears_minor, adjustments_minor, payable_minor, payable_after_due_minor,
    currency, is_estimated, cancelled, verified_at, verified_by, notes, tags, created_by,
    late_step_until, payable_final_minor,
    previous_reading, current_reading, units, multiplying_factor, meter_replaced
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17,
    $18, $19, $20, $21, $22, $23, $24, $25,
    $26, $27,
    $28, $29, $30, $31, $32
)
RETURNING id;

-- name: UpdateBill :execrows
UPDATE bills SET
    profile_id = $3, category_id = $4, description = $5, bill_number = $6,
    period_key = $7, period_start = $8, period_end = $9, issue_date = $10, due_date = $11,
    expense_date = $12, current_charges_minor = $13, arrears_minor = $14, adjustments_minor = $15,
    payable_minor = $16, payable_after_due_minor = $17, is_estimated = $18, cancelled = $19,
    notes = $20, tags = $21, late_step_until = $22, payable_final_minor = $23,
    previous_reading = $24, current_reading = $25, units = $26, multiplying_factor = $27, meter_replaced = $28,
    updated_at = now()
WHERE id = $1 AND household_id = $2;

-- name: GetBill :one
SELECT
    sqlc.embed(bills),
    bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::bigint AS outstanding_minor
FROM bills
WHERE id = sqlc.arg(id) AND household_id = sqlc.arg(household_id);

-- name: FindBillForPeriod :one
SELECT id FROM bills
WHERE profile_id = $1 AND period_key = $2 AND archived_at IS NULL
  AND id <> sqlc.arg(exclude_id);

-- name: ListBills :many
SELECT
    sqlc.embed(bills),
    bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::bigint AS outstanding_minor
FROM bills
WHERE household_id = sqlc.arg(household_id)
  AND (sqlc.narg(profile_id)::uuid IS NULL OR profile_id = sqlc.narg(profile_id)::uuid)
  AND (CASE WHEN sqlc.arg(archived)::boolean THEN archived_at IS NOT NULL ELSE archived_at IS NULL END)
  AND (sqlc.narg(from_date)::date IS NULL OR expense_date >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR expense_date <= sqlc.narg(to_date)::date)
  AND (sqlc.narg(statuses)::text[] IS NULL OR bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
      ) = ANY (sqlc.narg(statuses)::text[]))
  AND (sqlc.narg(cursor_date)::date IS NULL
       OR (expense_date, id) < (sqlc.narg(cursor_date)::date, sqlc.narg(cursor_id)::uuid))
ORDER BY expense_date DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: ArchiveBill :execrows
UPDATE bills SET archived_at = now(), updated_at = now()
WHERE id = $1 AND household_id = $2 AND archived_at IS NULL;

-- name: RestoreBill :execrows
UPDATE bills SET archived_at = NULL, updated_at = now()
WHERE id = $1 AND household_id = $2 AND archived_at IS NOT NULL;

-- name: DeleteArchivedBill :execrows
DELETE FROM bills WHERE id = $1 AND household_id = $2 AND archived_at IS NOT NULL;

-- name: RecalcBillPaid :exec
-- Keeps the denormalised payment totals in step with payments and the due date.
UPDATE bills b SET
    paid_minor = (SELECT coalesce(sum(p.amount_minor), 0) FROM payments p WHERE p.bill_id = b.id),
    paid_on_time_minor = (
        SELECT coalesce(sum(p.amount_minor), 0) FROM payments p
        WHERE p.bill_id = b.id AND (b.due_date IS NULL OR p.paid_on <= b.due_date)
    ),
    paid_by_step_minor = (
        SELECT coalesce(sum(p.amount_minor), 0) FROM payments p
        WHERE p.bill_id = b.id AND b.late_step_until IS NOT NULL AND p.paid_on <= b.late_step_until
    ),
    updated_at = now()
WHERE b.id = $1;

-- name: CreatePayment :one
INSERT INTO payments (id, household_id, bill_id, paid_on, amount_minor, method, reference, notes, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListPaymentsForBill :many
SELECT * FROM payments WHERE bill_id = $1 ORDER BY paid_on, created_at;

-- name: GetPayment :one
SELECT * FROM payments WHERE id = $1 AND household_id = $2;

-- name: DeletePayment :exec
DELETE FROM payments WHERE id = $1 AND household_id = $2;

-- name: CreateAuditEvent :exec
INSERT INTO audit_events (id, household_id, actor_user_id, entity_type, entity_id, action, changes)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAuditEvents :many
SELECT a.id, a.at, a.action, a.entity_type, a.changes, u.display_name AS actor_name
FROM audit_events a
LEFT JOIN users u ON u.id = a.actor_user_id
WHERE a.household_id = $1 AND a.entity_type = $2 AND a.entity_id = $3
ORDER BY a.at DESC
LIMIT 100;

-- name: MonthBilled :one
SELECT coalesce(sum(current_charges_minor + adjustments_minor), 0)::bigint
FROM bills
WHERE household_id = $1 AND archived_at IS NULL AND NOT cancelled AND verified_at IS NOT NULL
  AND coalesce(period_key, to_char(expense_date, 'YYYY-MM')) = sqlc.arg(month)::text;

-- name: MonthPaid :one
SELECT coalesce(sum(p.amount_minor), 0)::bigint
FROM payments p
JOIN bills b ON b.id = p.bill_id
WHERE p.household_id = $1 AND b.archived_at IS NULL
  AND p.paid_on >= sqlc.arg(month_start)::date AND p.paid_on < sqlc.arg(month_end)::date;

-- name: TotalOutstanding :one
SELECT coalesce(sum(bill_outstanding(
    cancelled, verified_at IS NOT NULL, due_date, payable_minor,
    payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
)), 0)::bigint
FROM bills
WHERE household_id = sqlc.arg(household_id) AND archived_at IS NULL;

-- name: ListOpenBillsDueBy :many
SELECT
    sqlc.embed(bills),
    bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::bigint AS outstanding_minor
FROM bills
WHERE household_id = sqlc.arg(household_id) AND archived_at IS NULL AND NOT cancelled
  AND verified_at IS NOT NULL AND due_date IS NOT NULL AND due_date <= sqlc.arg(until)::date
  AND bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
      ) > 0
ORDER BY due_date, id
LIMIT 20;

-- name: ListRecentBills :many
SELECT
    sqlc.embed(bills),
    bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::bigint AS outstanding_minor
FROM bills
WHERE household_id = sqlc.arg(household_id) AND archived_at IS NULL
ORDER BY created_at DESC
LIMIT 5;

-- name: FindProbableBill :one
-- A bill of the same profile that looks like the same bill (FR-RECO-03): issue dates within
-- 10 days, or the same payable with due dates within 3 days.
SELECT id FROM bills
WHERE profile_id = sqlc.arg(profile_id) AND archived_at IS NULL
  AND ((sqlc.narg(issue_date)::date IS NOT NULL AND issue_date IS NOT NULL
        AND abs(issue_date - sqlc.narg(issue_date)::date) <= 10)
    OR (sqlc.narg(due_date)::date IS NOT NULL AND due_date IS NOT NULL
        AND payable_minor = sqlc.arg(payable_minor) AND abs(due_date - sqlc.narg(due_date)::date) <= 3))
ORDER BY created_at DESC
LIMIT 1;

-- name: FindBillByNumber :one
SELECT id FROM bills
WHERE profile_id = $1 AND bill_number = $2 AND archived_at IS NULL
LIMIT 1;

-- name: MonthTotalsByProfile :many
-- FR-DASH-07: the month's bills per profile. Settled bills count as paid.
SELECT p.id AS profile_id, p.name, p.icon, p.color, c.type_group,
       count(b.id)::int AS bill_count,
       coalesce(sum(CASE WHEN b.settled_by_bill_id IS NOT NULL THEN greatest(b.paid_minor, b.payable_minor) ELSE b.paid_minor END), 0)::bigint AS paid_minor,
       coalesce(sum(bill_outstanding(
           b.cancelled, b.verified_at IS NOT NULL, b.due_date, b.payable_minor,
           b.payable_after_due_minor, b.paid_minor, b.paid_on_time_minor,
           b.late_step_until, b.payable_final_minor, b.paid_by_step_minor, b.settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
       )), 0)::bigint AS remaining_minor,
       coalesce(sum(b.arrears_minor) FILTER (WHERE bill_outstanding(
           b.cancelled, b.verified_at IS NOT NULL, b.due_date, b.payable_minor,
           b.payable_after_due_minor, b.paid_minor, b.paid_on_time_minor,
           b.late_step_until, b.payable_final_minor, b.paid_by_step_minor, b.settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
       ) > 0), 0)::bigint AS open_arrears_minor
FROM bills b
JOIN bill_profiles p ON p.id = b.profile_id
JOIN categories c ON c.id = p.category_id
WHERE b.household_id = sqlc.arg(household_id) AND b.archived_at IS NULL AND NOT b.cancelled AND b.verified_at IS NOT NULL
  AND coalesce(b.period_key, to_char(b.expense_date, 'YYYY-MM')) = sqlc.arg(month)::text
GROUP BY p.id, p.name, p.icon, p.color, c.type_group, p.sort_order
ORDER BY p.sort_order, p.name;

-- name: MonthNextDue :one
-- The month's open bill that is due first.
SELECT
    sqlc.embed(bills),
    bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::bigint AS outstanding_minor
FROM bills
WHERE household_id = sqlc.arg(household_id) AND archived_at IS NULL AND NOT cancelled AND verified_at IS NOT NULL
  AND coalesce(period_key, to_char(expense_date, 'YYYY-MM')) = sqlc.arg(month)::text
  AND bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
      ) > 0
ORDER BY due_date NULLS LAST, created_at
LIMIT 1;

-- name: ClearSettlementsBy :many
UPDATE bills SET settled_by_bill_id = NULL, updated_at = now()
WHERE settled_by_bill_id = $1
RETURNING id;

-- name: ListEarlierOpenBills :many
-- Unsettled earlier bills of the same profile, newest first, with what they still owe.
SELECT
    sqlc.embed(bills),
    bill_status(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::text AS status,
    bill_outstanding(
        cancelled, verified_at IS NOT NULL, due_date, payable_minor,
        payable_after_due_minor, paid_minor, paid_on_time_minor,
        late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
    )::bigint AS outstanding_minor
FROM bills
WHERE profile_id = sqlc.arg(profile_id) AND id <> sqlc.arg(bill_id) AND archived_at IS NULL AND NOT cancelled
  AND verified_at IS NOT NULL AND settled_by_bill_id IS NULL AND expense_date < sqlc.arg(before)::date
ORDER BY expense_date DESC, created_at DESC
LIMIT 12;

-- name: SettleBill :exec
UPDATE bills SET settled_by_bill_id = sqlc.arg(settled_by), updated_at = now()
WHERE id = sqlc.arg(id) AND household_id = sqlc.arg(household_id);

-- name: EarlierOutstandingByProfile :many
-- What bills from months before the given one still owe, per profile.
SELECT profile_id, coalesce(sum(bill_outstanding(
           cancelled, verified_at IS NOT NULL, due_date, payable_minor,
           payable_after_due_minor, paid_minor, paid_on_time_minor,
           late_step_until, payable_final_minor, paid_by_step_minor, settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
       )), 0)::bigint AS outstanding_minor
FROM bills
WHERE household_id = sqlc.arg(household_id) AND archived_at IS NULL AND NOT cancelled AND verified_at IS NOT NULL
  AND coalesce(period_key, to_char(expense_date, 'YYYY-MM')) < sqlc.arg(month)::text
GROUP BY profile_id;
