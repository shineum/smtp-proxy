-- name: CreateProvider :one
INSERT INTO esp_providers (group_id, name, provider_type, api_key, smtp_config, enabled, visibility)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetProviderByID :one
SELECT * FROM esp_providers WHERE id = $1;

-- name: ListProvidersByGroupID :many
SELECT * FROM esp_providers WHERE group_id = $1 ORDER BY created_at DESC;

-- name: ListAccessibleProviders :many
SELECT ep.* FROM esp_providers ep
WHERE ep.enabled = true
AND (
    ep.visibility = 'global'
    OR ep.group_id = $1
    OR (ep.visibility = 'shared' AND EXISTS (
        SELECT 1 FROM provider_group_access pga
        WHERE pga.provider_id = ep.id AND pga.group_id = $1
    ))
)
ORDER BY ep.created_at DESC;

-- name: UpdateProvider :one
UPDATE esp_providers
SET name = $2, provider_type = $3, api_key = $4, smtp_config = $5, enabled = $6, visibility = $7, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetStdoutProviderByGroupID :one
SELECT * FROM esp_providers WHERE group_id = $1 AND provider_type = 'stdout' LIMIT 1;

-- name: GetGlobalStdoutProvider :one
SELECT * FROM esp_providers WHERE provider_type = 'stdout' AND visibility = 'global' AND enabled = true LIMIT 1;

-- name: DeleteProvider :exec
DELETE FROM esp_providers WHERE id = $1;

-- name: GrantProviderAccess :exec
INSERT INTO provider_group_access (provider_id, group_id, granted_by)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RevokeProviderAccess :exec
DELETE FROM provider_group_access WHERE provider_id = $1 AND group_id = $2;

-- name: ListProviderAccess :many
SELECT * FROM provider_group_access WHERE provider_id = $1 ORDER BY granted_at DESC;

-- name: IsProviderAccessible :one
SELECT EXISTS(
    SELECT 1 FROM esp_providers ep
    WHERE ep.id = $1
    AND ep.enabled = true
    AND (
        ep.visibility = 'global'
        OR ep.group_id = $2
        OR (ep.visibility = 'shared' AND EXISTS (
            SELECT 1 FROM provider_group_access pga
            WHERE pga.provider_id = ep.id AND pga.group_id = $2
        ))
    )
) AS accessible;
