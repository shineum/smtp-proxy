-- name: CreateUser :one
INSERT INTO users (email, password_hash, account_type, username, api_key, allowed_domains)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByAPIKey :one
SELECT * FROM users WHERE api_key = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at DESC;

-- name: UpdateUser :one
UPDATE users
SET email = $2, status = $3, allowed_domains = $4, updated_at = NOW()
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
