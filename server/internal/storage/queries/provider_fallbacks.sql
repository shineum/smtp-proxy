-- name: CreateProviderFallback :one
INSERT INTO provider_fallbacks (user_id, provider_id, priority, enabled)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListProviderFallbacksByUserID :many
SELECT pf.*, ep.name AS provider_name, ep.provider_type
FROM provider_fallbacks pf
JOIN esp_providers ep ON ep.id = pf.provider_id
WHERE pf.user_id = $1 AND pf.enabled = true
ORDER BY pf.priority ASC;

-- name: ListAllProviderFallbacksByUserID :many
SELECT pf.*, ep.name AS provider_name, ep.provider_type
FROM provider_fallbacks pf
JOIN esp_providers ep ON ep.id = pf.provider_id
WHERE pf.user_id = $1
ORDER BY pf.priority ASC;

-- name: DeleteProviderFallback :exec
DELETE FROM provider_fallbacks WHERE id = $1;

-- name: DeleteProviderFallbacksByUserID :exec
DELETE FROM provider_fallbacks WHERE user_id = $1;

-- name: UpdateProviderFallback :one
UPDATE provider_fallbacks
SET priority = $2, enabled = $3
WHERE id = $1
RETURNING *;
