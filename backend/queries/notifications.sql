-- name: ListReminderMembers :many
-- Everyone who should get reminders, with their settings (defaults when they never saved any).
SELECT m.household_id, m.user_id, h.timezone, h.currency,
       coalesce(s.lead_days, 3)::smallint AS lead_days,
       coalesce(s.due_soon, true) AS due_soon,
       coalesce(s.overdue, true) AS overdue,
       coalesce(s.missing, true) AS missing
FROM memberships m
JOIN households h ON h.id = m.household_id
JOIN users u ON u.id = m.user_id
LEFT JOIN notification_settings s ON s.user_id = m.user_id AND s.household_id = m.household_id
WHERE u.deletion_requested_at IS NULL
ORDER BY m.household_id;

-- name: ListReminderBills :many
-- Open bills of a household with a due date up to until (overdue ones included).
SELECT b.id, b.profile_id, b.period_key, b.description, b.due_date::date AS due_date, b.currency,
       bill_outstanding(
           b.cancelled, b.verified_at IS NOT NULL, b.due_date, b.payable_minor,
           b.payable_after_due_minor, b.paid_minor, b.paid_on_time_minor,
           b.late_step_until, b.payable_final_minor, b.paid_by_step_minor, b.settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
       )::bigint AS outstanding_minor
FROM bills b
JOIN bill_profiles p ON p.id = b.profile_id
WHERE b.household_id = sqlc.arg(household_id)
  AND b.archived_at IS NULL AND NOT b.cancelled AND b.verified_at IS NOT NULL
  AND b.due_date IS NOT NULL AND b.due_date <= sqlc.arg(until)::date
  AND b.settled_by_bill_id IS NULL
  AND p.archived_at IS NULL
  AND bill_outstanding(
           b.cancelled, b.verified_at IS NOT NULL, b.due_date, b.payable_minor,
           b.payable_after_due_minor, b.paid_minor, b.paid_on_time_minor,
           b.late_step_until, b.payable_final_minor, b.paid_by_step_minor, b.settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
       ) > 0;

-- name: ListCycleProfiles :many
-- Monthly-type profiles (metered / recurring) and their latest bill, for expected-bill
-- detection (FR-REM-04) and auto-created recurring bills (FR-BILL-10).
SELECT p.id, p.name, p.icon, p.color, p.kind, p.category_id, p.billing_cycle, p.typical_issue_day, p.typical_due_day,
       p.expected_amount_minor, p.auto_create_bill, p.created_at,
       coalesce((SELECT max(b.period_key) FROM bills b WHERE b.profile_id = p.id AND b.archived_at IS NULL), '')::text AS last_period
FROM bill_profiles p
WHERE p.household_id = $1 AND p.archived_at IS NULL AND p.kind IN ('metered', 'recurring')
ORDER BY p.sort_order, p.created_at;

-- name: InsertNotification :execrows
INSERT INTO notifications (id, household_id, user_id, kind, profile_id, bill_id, period_key, due_date, amount_minor, dedupe_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id, dedupe_key) DO NOTHING;

-- name: ListNotifications :many
SELECT n.*, p.name AS profile_name, p.icon AS profile_icon, p.color AS profile_color, b.description AS bill_description
FROM notifications n
LEFT JOIN bill_profiles p ON p.id = n.profile_id
LEFT JOIN bills b ON b.id = n.bill_id
WHERE n.user_id = $1 AND n.household_id = $2
ORDER BY n.created_at DESC, n.id DESC
LIMIT $3;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE user_id = $1 AND household_id = $2 AND read_at IS NULL;

-- name: MarkNotificationsRead :execrows
UPDATE notifications SET read_at = sqlc.arg(now)
WHERE user_id = sqlc.arg(user_id) AND household_id = sqlc.arg(household_id) AND read_at IS NULL
  AND (sqlc.arg(all_unread)::boolean OR id = ANY (sqlc.arg(ids)::uuid[]));

-- name: GetNotificationSettings :one
SELECT * FROM notification_settings WHERE user_id = $1 AND household_id = $2;

-- name: UpsertNotificationSettings :one
INSERT INTO notification_settings (user_id, household_id, lead_days, due_soon, overdue, missing, push, quiet_start, quiet_end, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
ON CONFLICT (user_id, household_id) DO UPDATE SET
    lead_days = EXCLUDED.lead_days, due_soon = EXCLUDED.due_soon, overdue = EXCLUDED.overdue, missing = EXCLUDED.missing,
    push = EXCLUDED.push, quiet_start = EXCLUDED.quiet_start, quiet_end = EXCLUDED.quiet_end, updated_at = now()
RETURNING *;

-- name: CreateSystemAuditEvent :exec
-- An audit event with no person behind it (the tick created the bill).
INSERT INTO audit_events (id, household_id, actor_user_id, entity_type, entity_id, action, changes)
VALUES ($1, $2, NULL, $3, $4, $5, $6);
