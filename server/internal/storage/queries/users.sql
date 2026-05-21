-- name: CreateUser :one
INSERT INTO users (email, password_hash, account_type, username, allowed_domains, password_disabled, provider_id, home_group_id, display_name, description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- name: GetUserByUsernameAndGroupID :one
SELECT u.* FROM users u
JOIN groups g ON u.home_group_id = g.id
WHERE u.username = $1 AND g.id = $2
AND u.account_type = 'smtp'
AND u.deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users WHERE deleted_at IS NULL ORDER BY created_at DESC;

-- name: UpdateUser :one
UPDATE users
SET email = $2, status = $3, allowed_domains = $4, display_name = $5, description = $6, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserStatus :one
UPDATE users
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET last_login = NOW(), failed_attempts = 0, updated_at = NOW()
WHERE id = $1;

-- name: IncrementFailedAttempts :exec
UPDATE users
SET failed_attempts = failed_attempts + 1, updated_at = NOW()
WHERE id = $1;

-- name: ResetFailedAttempts :exec
UPDATE users
SET failed_attempts = 0, updated_at = NOW()
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1;

-- name: ListUsersByGroupID :many
SELECT u.* FROM users u
JOIN group_members gm ON u.id = gm.user_id
WHERE gm.group_id = $1 AND u.deleted_at IS NULL
ORDER BY u.created_at DESC;

-- name: UpdatePasswordDisabled :one
UPDATE users
SET password_disabled = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListUsersByProviderID :many
SELECT u.id, u.email, u.account_type, gm.role, g.id AS group_id, g.name AS group_name
FROM users u
JOIN group_members gm ON u.id = gm.user_id
JOIN groups g ON gm.group_id = g.id
WHERE u.provider_id = $1 AND u.deleted_at IS NULL
ORDER BY g.name, u.email;

-- name: UpdateUserProvider :one
UPDATE users
SET provider_id = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteUser :one
UPDATE users
SET deleted_at = NOW(), status = 'deleted', updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RestoreUser :one
UPDATE users
SET deleted_at = NULL, status = 'active', updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListDeletedUsers :many
SELECT * FROM users WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC;

-- name: PurgeDeletedUsers :exec
DELETE FROM users WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - INTERVAL '30 days';

-- name: ResolveAnonymousSMTPByIP :many
-- Returns active SMTP service accounts whose anonymous_allowed_cidrs contain
-- the given source IP. Used when an unauthenticated client submits MAIL FROM.
-- Multiple matches indicate operator misconfiguration (overlapping CIDRs);
-- callers should reject the session rather than picking arbitrarily.
SELECT u.*
FROM users u
JOIN groups g ON u.home_group_id = g.id
WHERE u.account_type = 'smtp'
  AND u.status = 'active'
  AND u.anonymous_allowed = true
  AND u.deleted_at IS NULL
  AND g.status = 'active'
  AND EXISTS (
      SELECT 1
      FROM jsonb_array_elements_text(u.anonymous_allowed_cidrs) AS c(cidr)
      WHERE $1::inet <<= c.cidr::inet
  );

-- name: UpdateUserAnonymous :one
UPDATE users
SET anonymous_allowed = $2,
    anonymous_allowed_cidrs = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

