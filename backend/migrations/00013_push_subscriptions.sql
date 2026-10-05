-- +goose Up
-- Web Push subscriptions (FR-REM-03): one per browser/device that allowed notifications.
CREATE TABLE push_subscriptions (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    endpoint     text NOT NULL UNIQUE,
    p256dh       text NOT NULL,
    auth         text NOT NULL,
    user_agent   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    failures     integer NOT NULL DEFAULT 0
);
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (user_id, household_id);

-- +goose Down
DROP TABLE IF EXISTS push_subscriptions;
