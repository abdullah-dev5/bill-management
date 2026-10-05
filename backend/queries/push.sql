-- name: UpsertPushSubscription :exec
INSERT INTO push_subscriptions (id, user_id, household_id, endpoint, p256dh, auth, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (endpoint) DO UPDATE SET
    user_id = EXCLUDED.user_id, household_id = EXCLUDED.household_id, p256dh = EXCLUDED.p256dh,
    auth = EXCLUDED.auth, user_agent = EXCLUDED.user_agent, failures = 0;

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscriptions WHERE endpoint = $1 AND user_id = $2;

-- name: DeletePushSubscriptionByID :exec
DELETE FROM push_subscriptions WHERE id = $1;

-- name: ListPushSubscriptions :many
SELECT * FROM push_subscriptions WHERE user_id = $1 AND household_id = $2 ORDER BY created_at;

-- name: PushSubscriptionUsed :exec
UPDATE push_subscriptions SET last_used_at = $2, failures = 0 WHERE id = $1;

-- name: PushSubscriptionFailed :one
UPDATE push_subscriptions SET failures = failures + 1 WHERE id = $1 RETURNING failures;

-- name: ListUnpushedNotifications :many
-- Reminders not yet pushed (newer than since), with what the message needs and the person's push settings.
SELECT n.id, n.user_id, n.household_id, n.kind, n.profile_id, n.bill_id, n.period_key, n.due_date, n.amount_minor, n.created_at,
       p.name AS profile_name, b.description AS bill_description, h.timezone, h.currency,
       coalesce(s.push, false) AS push, s.quiet_start, s.quiet_end
FROM notifications n
JOIN households h ON h.id = n.household_id
LEFT JOIN bill_profiles p ON p.id = n.profile_id
LEFT JOIN bills b ON b.id = n.bill_id
LEFT JOIN notification_settings s ON s.user_id = n.user_id AND s.household_id = n.household_id
WHERE n.pushed_at IS NULL AND n.created_at > sqlc.arg(since)
ORDER BY n.created_at
LIMIT 500;

-- name: MarkNotificationPushed :exec
UPDATE notifications SET pushed_at = $2 WHERE id = $1;

-- name: SkipOldUnpushedNotifications :exec
-- Too old to be worth a push: never push them.
UPDATE notifications SET pushed_at = sqlc.arg(now) WHERE pushed_at IS NULL AND created_at <= sqlc.arg(since);
