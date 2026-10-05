-- +goose Up
-- In-app reminders (FR-REM-01/04, FR-BILL-10). One row per person per event; dedupe_key stops the
-- 10-minute tick from repeating a reminder (e.g. "due_soon:<bill>:<due date>").
CREATE TYPE notification_kind AS ENUM ('due_soon', 'due_today', 'overdue', 'missing_bill', 'bill_created');

CREATE TABLE notifications (
    id           uuid PRIMARY KEY,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind         notification_kind NOT NULL,
    profile_id   uuid REFERENCES bill_profiles (id) ON DELETE CASCADE,
    bill_id      uuid REFERENCES bills (id) ON DELETE CASCADE,
    period_key   char(7),
    due_date     date,
    amount_minor bigint,
    dedupe_key   text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    read_at      timestamptz,
    pushed_at    timestamptz,
    UNIQUE (user_id, dedupe_key)
);
CREATE INDEX notifications_user_idx ON notifications (user_id, household_id, created_at DESC);
CREATE INDEX notifications_unpushed_idx ON notifications (created_at) WHERE pushed_at IS NULL;

-- Per person, per household (FR-REM-02).
CREATE TABLE notification_settings (
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    lead_days    smallint NOT NULL DEFAULT 3 CHECK (lead_days BETWEEN 0 AND 14),
    due_soon     boolean NOT NULL DEFAULT true,
    overdue      boolean NOT NULL DEFAULT true,
    missing      boolean NOT NULL DEFAULT true,
    push         boolean NOT NULL DEFAULT false,
    quiet_start  smallint CHECK (quiet_start BETWEEN 0 AND 23), -- local hour; no push from quiet_start to quiet_end
    quiet_end    smallint CHECK (quiet_end BETWEEN 0 AND 23),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, household_id)
);

-- +goose Down
DROP TABLE IF EXISTS notification_settings;
DROP TABLE IF EXISTS notifications;
DROP TYPE IF EXISTS notification_kind;
