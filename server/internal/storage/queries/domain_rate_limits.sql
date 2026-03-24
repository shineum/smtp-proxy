-- name: CreateDomainRateLimit :one
INSERT INTO domain_rate_limits (group_id, domain, max_per_minute, max_per_hour, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListDomainRateLimitsByGroupID :many
SELECT * FROM domain_rate_limits
WHERE group_id = $1
ORDER BY domain ASC;

-- name: GetDomainRateLimit :one
SELECT * FROM domain_rate_limits
WHERE group_id = $1 AND domain = $2;

-- name: UpdateDomainRateLimit :one
UPDATE domain_rate_limits
SET max_per_minute = $2, max_per_hour = $3, enabled = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteDomainRateLimit :exec
DELETE FROM domain_rate_limits WHERE id = $1;
