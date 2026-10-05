-- name: SearchBills :many
-- Global search (FR-SRCH-01/02). Each pattern is an ILIKE pattern that must match at least one
-- field: profile, type (or its parent), provider, bill number, notes, description, tags,
-- payment reference/notes, or the text of an attached document.
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
    )::bigint AS outstanding_minor
FROM bills
JOIN bill_profiles p ON p.id = bills.profile_id
JOIN categories c ON c.id = bills.category_id
LEFT JOIN categories pc ON pc.id = c.parent_id
LEFT JOIN providers pr ON pr.id = p.provider_id
WHERE bills.household_id = sqlc.arg(household_id)
  AND bills.archived_at IS NULL
  AND (coalesce(cardinality(sqlc.arg(type_ids)::uuid[]), 0) = 0 OR bills.category_id = ANY (sqlc.arg(type_ids)::uuid[]))
  AND (coalesce(cardinality(sqlc.arg(provider_ids)::uuid[]), 0) = 0 OR p.provider_id = ANY (sqlc.arg(provider_ids)::uuid[]))
  AND (coalesce(cardinality(sqlc.arg(profile_ids)::uuid[]), 0) = 0 OR bills.profile_id = ANY (sqlc.arg(profile_ids)::uuid[]))
  AND (sqlc.narg(from_period)::text IS NULL
       OR coalesce(bills.period_key, to_char(bills.expense_date, 'YYYY-MM')) BETWEEN sqlc.narg(from_period)::text AND sqlc.narg(to_period)::text)
  AND (coalesce(cardinality(sqlc.arg(statuses)::text[]), 0) = 0 OR bill_status(
        bills.cancelled, bills.verified_at IS NOT NULL, bills.due_date, bills.payable_minor,
        bills.payable_after_due_minor, bills.paid_minor, bills.paid_on_time_minor,
        bills.late_step_until, bills.payable_final_minor, bills.paid_by_step_minor, bills.settled_by_bill_id IS NOT NULL, sqlc.arg(today)::date
      ) = ANY (sqlc.arg(statuses)::text[]))
  AND NOT EXISTS (
      SELECT 1 FROM unnest(sqlc.arg(patterns)::text[]) AS t(pat)
      WHERE NOT (
          p.name ILIKE t.pat OR c.name ILIKE t.pat OR coalesce(pc.name, '') ILIKE t.pat
          OR coalesce(pr.name, '') ILIKE t.pat OR coalesce(pr.short_name, '') ILIKE t.pat
          OR coalesce(bills.bill_number, '') ILIKE t.pat OR coalesce(bills.notes, '') ILIKE t.pat
          OR coalesce(bills.description, '') ILIKE t.pat OR array_to_string(bills.tags, ' ') ILIKE t.pat
          OR EXISTS (SELECT 1 FROM payments py WHERE py.bill_id = bills.id
                     AND (coalesce(py.reference, '') ILIKE t.pat OR coalesce(py.notes, '') ILIKE t.pat))
          OR EXISTS (SELECT 1 FROM documents d WHERE d.bill_id = bills.id AND coalesce(d.ocr_text, '') ILIKE t.pat)
      ))
ORDER BY bills.expense_date DESC, bills.id DESC
LIMIT sqlc.arg(row_limit);

-- name: SearchDocuments :many
-- Documents whose text (redacted OCR / PDF text) or file name contains every pattern.
SELECT d.id, d.file_name, d.bill_id, d.kind, d.created_at, d.ocr_text
FROM documents d
WHERE d.household_id = sqlc.arg(household_id)
  AND d.status = 'ready'
  AND NOT EXISTS (
      SELECT 1 FROM unnest(sqlc.arg(patterns)::text[]) AS t(pat)
      WHERE NOT (coalesce(d.ocr_text, '') ILIKE t.pat OR d.file_name ILIKE t.pat))
ORDER BY d.created_at DESC
LIMIT 20;

-- name: ListActivity :many
-- Recent activity for the home screen (FR-DASH-05): bills added, payments, documents, new profiles.
SELECT a.id, a.at, a.action, a.entity_type, a.entity_id, a.changes, u.display_name AS actor_name,
       coalesce(bp.id, ep.id) AS profile_id, coalesce(bp.name, ep.name) AS profile_name,
       b.period_key, b.description, b.payable_minor, b.currency
FROM audit_events a
LEFT JOIN users u ON u.id = a.actor_user_id
LEFT JOIN bills b ON a.entity_type = 'bill' AND b.id = a.entity_id
LEFT JOIN bill_profiles bp ON bp.id = b.profile_id
LEFT JOIN bill_profiles ep ON a.entity_type = 'profile' AND ep.id = a.entity_id
WHERE a.household_id = sqlc.arg(household_id)
  AND ((a.entity_type = 'bill' AND a.action IN ('create', 'payment_added', 'payment_deleted', 'attach_document') AND b.id IS NOT NULL)
       OR (a.entity_type = 'profile' AND a.action = 'create' AND ep.id IS NOT NULL))
ORDER BY a.at DESC
LIMIT sqlc.arg(row_limit);
