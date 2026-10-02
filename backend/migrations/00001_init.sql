-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

-- Identity ------------------------------------------------------------------

CREATE TABLE users (
    id                    uuid PRIMARY KEY,
    email                 citext NOT NULL UNIQUE,
    password_hash         text NOT NULL,
    display_name          text NOT NULL,
    locale                text NOT NULL DEFAULT 'en-PK',
    deletion_requested_at timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    user_agent   text,
    ip           text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE password_reset_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Household -----------------------------------------------------------------

CREATE TABLE households (
    id                    uuid PRIMARY KEY,
    name                  text NOT NULL,
    currency              char(3) NOT NULL DEFAULT 'PKR',
    timezone              text NOT NULL DEFAULT 'Asia/Karachi',
    ai_extraction_consent boolean NOT NULL DEFAULT false,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE member_role AS ENUM ('owner', 'editor', 'viewer');

CREATE TABLE memberships (
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role         member_role NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, user_id)
);
CREATE INDEX memberships_user_idx ON memberships (user_id);

CREATE TABLE properties (
    id           uuid PRIMARY KEY,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         text NOT NULL,
    address      text,
    is_default   boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    archived_at  timestamptz
);
CREATE UNIQUE INDEX properties_one_default ON properties (household_id) WHERE is_default;

-- Bill types & providers ----------------------------------------------------

CREATE TYPE profile_kind AS ENUM ('metered', 'recurring', 'expense_log');
CREATE TYPE type_group AS ENUM ('utilities', 'household', 'other');

-- "categories" are the bill types. household_id NULL = system type.
CREATE TABLE categories (
    id           uuid PRIMARY KEY,
    household_id uuid REFERENCES households (id) ON DELETE CASCADE,
    name         text NOT NULL,
    icon         text NOT NULL,
    default_kind profile_kind NOT NULL,
    type_group   type_group NOT NULL DEFAULT 'other',
    parent_id    uuid REFERENCES categories (id),
    sort_order   integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    archived_at  timestamptz
);
CREATE UNIQUE INDEX categories_name_uniq
    ON categories (coalesce(household_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name))
    WHERE archived_at IS NULL;

CREATE TABLE providers (
    id                 uuid PRIMARY KEY,
    household_id       uuid REFERENCES households (id) ON DELETE CASCADE,
    name               text NOT NULL,
    short_name         text NOT NULL,
    category_id        uuid NOT NULL REFERENCES categories (id),
    region             text,
    ref_number_label   text NOT NULL DEFAULT 'Reference No.',
    ref_number_pattern text,
    consumption_unit   text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    archived_at        timestamptz
);
CREATE INDEX providers_category_idx ON providers (category_id);

-- Bill profiles ---------------------------------------------------------------

CREATE TYPE billing_cycle AS ENUM ('monthly', 'bimonthly', 'quarterly', 'yearly');

CREATE TABLE bill_profiles (
    id                    uuid PRIMARY KEY,
    household_id          uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    property_id           uuid NOT NULL REFERENCES properties (id),
    category_id           uuid NOT NULL REFERENCES categories (id),
    kind                  profile_kind NOT NULL,
    name                  text NOT NULL,
    icon                  text NOT NULL,
    color                 smallint NOT NULL CHECK (color BETWEEN 1 AND 8),
    provider_id           uuid REFERENCES providers (id),
    reference_number      text,
    reference_display     text,
    meter_number          text,
    multiplying_factor    numeric(10, 4),
    consumption_unit      text,
    billing_cycle         billing_cycle,
    typical_issue_day     smallint CHECK (typical_issue_day BETWEEN 1 AND 31),
    typical_due_day       smallint CHECK (typical_due_day BETWEEN 1 AND 31),
    expected_amount_minor bigint CHECK (expected_amount_minor >= 0),
    auto_create_bill      boolean NOT NULL DEFAULT false,
    is_default            boolean NOT NULL DEFAULT false,
    pinned                boolean NOT NULL DEFAULT false,
    sort_order            integer NOT NULL DEFAULT 0,
    retrieval_enabled     boolean NOT NULL DEFAULT false,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    archived_at           timestamptz
);
CREATE UNIQUE INDEX bill_profiles_name_uniq
    ON bill_profiles (household_id, lower(name)) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX bill_profiles_ref_uniq
    ON bill_profiles (household_id, provider_id, reference_number)
    WHERE archived_at IS NULL AND reference_number IS NOT NULL;
CREATE UNIQUE INDEX bill_profiles_one_default ON bill_profiles (property_id) WHERE is_default;
CREATE INDEX bill_profiles_household_idx ON bill_profiles (household_id, sort_order);

-- Bills -----------------------------------------------------------------------

CREATE TABLE bills (
    id                      uuid PRIMARY KEY,
    household_id            uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    profile_id              uuid NOT NULL REFERENCES bill_profiles (id),
    category_id             uuid NOT NULL REFERENCES categories (id),
    description             text,
    bill_number             text,
    period_key              char(7) CHECK (period_key ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    period_start            date,
    period_end              date,
    issue_date              date,
    due_date                date,
    expense_date            date NOT NULL,
    current_charges_minor   bigint NOT NULL CHECK (current_charges_minor >= 0),
    arrears_minor           bigint NOT NULL DEFAULT 0 CHECK (arrears_minor >= 0),
    adjustments_minor       bigint NOT NULL DEFAULT 0,
    payable_minor           bigint NOT NULL CHECK (payable_minor >= 0),
    payable_after_due_minor bigint CHECK (payable_after_due_minor >= 0),
    currency                char(3) NOT NULL,
    paid_minor              bigint NOT NULL DEFAULT 0,
    paid_on_time_minor      bigint NOT NULL DEFAULT 0,
    is_estimated            boolean NOT NULL DEFAULT false,
    cancelled               boolean NOT NULL DEFAULT false,
    verified_at             timestamptz,
    verified_by             uuid REFERENCES users (id) ON DELETE SET NULL,
    field_meta              jsonb NOT NULL DEFAULT '{}',
    notes                   text,
    tags                    text[] NOT NULL DEFAULT '{}',
    created_by              uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    archived_at             timestamptz
);
-- One bill per profile per billing month (cycle-based profiles; expense logs have no period_key).
CREATE UNIQUE INDEX bills_profile_period_uniq
    ON bills (profile_id, period_key) WHERE archived_at IS NULL AND period_key IS NOT NULL;
CREATE INDEX bills_household_date_idx ON bills (household_id, expense_date DESC, id DESC);
CREATE INDEX bills_profile_date_idx ON bills (profile_id, expense_date DESC, id DESC);
CREATE INDEX bills_open_due_idx ON bills (household_id, due_date)
    WHERE archived_at IS NULL AND NOT cancelled;

-- Payments --------------------------------------------------------------------

CREATE TYPE payment_method AS ENUM
    ('cash', 'bank_transfer', 'card', 'jazzcash', 'easypaisa', 'cheque', 'other');

CREATE TABLE payments (
    id           uuid PRIMARY KEY,
    household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    bill_id      uuid NOT NULL REFERENCES bills (id) ON DELETE CASCADE,
    paid_on      date NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    method       payment_method NOT NULL,
    reference    text,
    notes        text,
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_bill_idx ON payments (bill_id);
CREATE INDEX payments_household_paid_on_idx ON payments (household_id, paid_on);

-- Audit -----------------------------------------------------------------------

CREATE TABLE audit_events (
    id            uuid PRIMARY KEY,
    household_id  uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    entity_type   text NOT NULL,
    entity_id     uuid NOT NULL,
    action        text NOT NULL,
    changes       jsonb NOT NULL DEFAULT '{}',
    at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_entity_idx ON audit_events (entity_type, entity_id, at);

-- Derived bill state ------------------------------------------------------------
-- Status and outstanding are derived on read from stored facts, so they are never stale.
-- See docs/04-domain-model.md §3.

-- +goose StatementBegin
CREATE FUNCTION bill_status(
    cancelled boolean, verified boolean, due_date date,
    payable bigint, payable_after_due bigint,
    paid bigint, paid_on_time bigint, today date
) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled THEN 'cancelled'
        WHEN NOT verified THEN 'draft'
        WHEN paid_on_time >= payable THEN 'paid'
        WHEN paid >= coalesce(payable_after_due, payable) THEN 'paid'
        WHEN due_date IS NOT NULL AND today > due_date THEN 'overdue'
        WHEN paid > 0 THEN 'partially_paid'
        ELSE 'unpaid'
    END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION bill_outstanding(
    cancelled boolean, verified boolean, due_date date,
    payable bigint, payable_after_due bigint,
    paid bigint, paid_on_time bigint, today date
) RETURNS bigint LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN cancelled OR NOT verified THEN 0
        WHEN paid_on_time >= payable THEN 0
        WHEN due_date IS NOT NULL AND today > due_date AND payable_after_due IS NOT NULL
            THEN greatest(payable_after_due - paid, 0)
        ELSE greatest(payable - paid, 0)
    END::bigint
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS bill_outstanding;
DROP FUNCTION IF EXISTS bill_status;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS payments;
DROP TYPE IF EXISTS payment_method;
DROP TABLE IF EXISTS bills;
DROP TABLE IF EXISTS bill_profiles;
DROP TYPE IF EXISTS billing_cycle;
DROP TABLE IF EXISTS providers;
DROP TABLE IF EXISTS categories;
DROP TYPE IF EXISTS type_group;
DROP TYPE IF EXISTS profile_kind;
DROP TABLE IF EXISTS properties;
DROP TABLE IF EXISTS memberships;
DROP TYPE IF EXISTS member_role;
DROP TABLE IF EXISTS households;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
