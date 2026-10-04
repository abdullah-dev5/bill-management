-- Analytics (FR-ANLT-02…06). A bill belongs to its billing month (expense entries: the month
-- of their date). Sub-categories roll up into their parent type.

-- name: AnalyticsBills :many
-- One row per bill in the month range with its spend: current charges + adjustments + the late
-- fee actually paid. Arrears are never counted (they are an earlier bill's spend).
SELECT b.id, b.profile_id, p.name AS profile_name, p.icon AS profile_icon, p.color AS profile_color,
       p.kind AS profile_kind, p.consumption_unit,
       coalesce(pc.id, c.id) AS type_id, coalesce(pc.name, c.name) AS type_name,
       coalesce(pc.icon, c.icon) AS type_icon, coalesce(pc.type_group, c.type_group) AS type_group,
       pv.id AS provider_id, pv.short_name AS provider_name,
       coalesce(b.period_key, to_char(b.expense_date, 'YYYY-MM'))::text AS month,
       (b.current_charges_minor + b.adjustments_minor + least(
           greatest(b.paid_minor - b.payable_minor, 0),
           greatest(coalesce(b.payable_final_minor, b.payable_after_due_minor, b.payable_minor) - b.payable_minor, 0)
       ))::bigint AS spend_minor,
       b.units
FROM bills b
JOIN bill_profiles p ON p.id = b.profile_id
JOIN categories c ON c.id = b.category_id
LEFT JOIN categories pc ON pc.id = c.parent_id
LEFT JOIN providers pv ON pv.id = p.provider_id
WHERE b.household_id = sqlc.arg(household_id) AND b.archived_at IS NULL AND NOT b.cancelled
  AND b.verified_at IS NOT NULL
  AND coalesce(b.period_key, to_char(b.expense_date, 'YYYY-MM')) BETWEEN sqlc.arg(from_month)::text AND sqlc.arg(to_month)::text;

-- name: AnalyticsPayments :many
-- "Cash paid" basis: payments in the month they were made.
SELECT pay.id, b.profile_id, p.name AS profile_name, p.icon AS profile_icon, p.color AS profile_color,
       p.kind AS profile_kind, p.consumption_unit,
       coalesce(pc.id, c.id) AS type_id, coalesce(pc.name, c.name) AS type_name,
       coalesce(pc.icon, c.icon) AS type_icon, coalesce(pc.type_group, c.type_group) AS type_group,
       pv.id AS provider_id, pv.short_name AS provider_name,
       to_char(pay.paid_on, 'YYYY-MM')::text AS month,
       pay.amount_minor
FROM payments pay
JOIN bills b ON b.id = pay.bill_id
JOIN bill_profiles p ON p.id = b.profile_id
JOIN categories c ON c.id = b.category_id
LEFT JOIN categories pc ON pc.id = c.parent_id
LEFT JOIN providers pv ON pv.id = p.provider_id
WHERE pay.household_id = sqlc.arg(household_id) AND b.archived_at IS NULL
  AND to_char(pay.paid_on, 'YYYY-MM') BETWEEN sqlc.arg(from_month)::text AND sqlc.arg(to_month)::text;

-- name: AnalyticsPrintedUnits :many
-- Units printed on bills' history, used for months without a bill of their own.
SELECT profile_id, period_key::text AS month, units
FROM profile_history
WHERE household_id = sqlc.arg(household_id) AND units IS NOT NULL
  AND period_key BETWEEN sqlc.arg(from_month)::text AND sqlc.arg(to_month)::text;
