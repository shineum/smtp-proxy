-- name: CreateUser :one
INSERT INTO users (email, password_hash, account_type, username, api_key, allowed_domains, password_disabled, provider_id, home_group_id, display_name, description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- name: GetUserByAPIKey :one
SELECT * FROM users WHERE api_key = $1 AND deleted_at IS NULL;

-- name: GetUserByUsernameAndGroupKey :one
SELECT u.* FROM users u
JOIN groups g ON u.home_group_id = g.id
WHERE u.username = $1 AND g.group_key = $2
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

-- name: ResetUserAPIKey :one
UPDATE users
SET api_key = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;
