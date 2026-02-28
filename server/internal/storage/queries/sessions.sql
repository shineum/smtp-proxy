-- name: CreateSession :one
INSERT INTO sessions (user_id, group_id, refresh_token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();

-- name: ListSessionsByUserID :many
SELECT * FROM sessions WHERE user_id = $1 ORDER BY created_at DESC;
