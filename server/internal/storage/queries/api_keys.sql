-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, key_prefix, key_hash, label, expires_at, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAPIKeysByUserID :many
SELECT id, user_id, key_prefix, label, is_active, expires_at, last_used_at, created_at
FROM api_keys
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetAPIKeyByPrefix :one
SELECT * FROM api_keys
WHERE key_prefix = $1 AND is_active = true;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys WHERE id = $1 AND user_id = $2;

-- name: DeleteAllAPIKeysByUserID :exec
DELETE FROM api_keys WHERE user_id = $1;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;

-- name: CountAPIKeysByUserID :one
SELECT COUNT(*)::integer FROM api_keys WHERE user_id = $1 AND is_active = true;

-- name: UpdateAPIKeyStatus :one
UPDATE api_keys SET is_active = $3 WHERE id = $1 AND user_id = $2
RETURNING *;
