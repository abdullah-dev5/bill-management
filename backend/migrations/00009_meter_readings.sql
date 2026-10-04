-- +goose Up
-- Meter readings for metered profiles (FR-CONS-01…04). units is the consumption for the period:
-- (current − previous) × multiplying factor, or what the bill prints when the meter was replaced.
ALTER TABLE bills
    ADD COLUMN previous_reading   numeric(14, 2) CHECK (previous_reading >= 0),
    ADD COLUMN current_reading    numeric(14, 2) CHECK (current_reading >= 0),
    ADD COLUMN units              numeric(14, 2) CHECK (units >= 0),
    ADD COLUMN multiplying_factor numeric(10, 4) CHECK (multiplying_factor > 0),
    ADD COLUMN meter_replaced     boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE bills
    DROP COLUMN IF EXISTS meter_replaced,
    DROP COLUMN IF EXISTS multiplying_factor,
    DROP COLUMN IF EXISTS units,
    DROP COLUMN IF EXISTS current_reading,
    DROP COLUMN IF EXISTS previous_reading;
