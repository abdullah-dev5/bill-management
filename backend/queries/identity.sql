-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, display_name, locale)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: SetUserDeletionRequested :exec
UPDATE users SET deletion_requested_at = $2, updated_at = now() WHERE id = $1;

-- name: ListUsersDueForPurge :many
SELECT id FROM users
WHERE deletion_requested_at IS NOT NULL AND deletion_requested_at < $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, user_agent, ip, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetSessionByTokenHash :one
SELECT id, user_id, expires_at, last_seen_at FROM sessions
WHERE token_hash = $1 AND expires_at > now();

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now(), expires_at = $2 WHERE id = $1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= now();

-- name: CreatePasswordResetToken :exec
INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetValidPasswordResetToken :one
SELECT id, user_id FROM password_reset_tokens
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now();

-- name: MarkPasswordResetTokenUsed :exec
UPDATE password_reset_tokens SET used_at = now() WHERE id = $1;

-- name: DeleteUserPasswordResetTokens :exec
DELETE FROM password_reset_tokens WHERE user_id = $1;

-- name: DeleteExpiredPasswordResetTokens :execrows
DELETE FROM password_reset_tokens WHERE expires_at <= now() OR used_at IS NOT NULL;

-- name: CreateHousehold :one
INSERT INTO households (id, name, currency, timezone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetHousehold :one
SELECT * FROM households WHERE id = $1;

-- name: UpdateHousehold :one
UPDATE households SET name = $2, currency = $3, timezone = $4,
    ai_extraction_consent = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateMembership :exec
INSERT INTO memberships (household_id, user_id, role) VALUES ($1, $2, $3);

-- name: GetPrimaryMembership :one
-- The household a user works in. One per user until invitations ship.
SELECT household_id, role FROM memberships
WHERE user_id = $1
ORDER BY created_at
LIMIT 1;

-- name: DeleteSoleMemberHouseholds :exec
-- Households whose only member is this user are deleted with the user.
DELETE FROM households h
WHERE h.id IN (SELECT m.household_id FROM memberships m WHERE m.user_id = $1)
  AND NOT EXISTS (
      SELECT 1 FROM memberships o WHERE o.household_id = h.id AND o.user_id <> $1
  );

-- name: CreateProperty :one
INSERT INTO properties (id, household_id, name, is_default)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDefaultProperty :one
SELECT * FROM properties WHERE household_id = $1 AND is_default;
