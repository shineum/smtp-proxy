-- name: CreateProvider :one
INSERT INTO esp_providers (account_id, name, provider_type, api_key, smtp_config, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProviderByID :one
SELECT * FROM esp_providers WHERE id = $1;

-- name: ListProvidersByAccountID :many
SELECT * FROM esp_providers WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateProvider :one
UPDATE esp_providers
SET name = $2, provider_type = $3, api_key = $4, smtp_config = $5, enabled = $6, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteProvider :exec
DELETE FROM esp_providers WHERE id = $1;
