-- name: CreateTenant :one
INSERT INTO tenants (name, status, monthly_limit)
VALUES ($1, 'active', $2)
RETURNING *;

-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantByName :one
SELECT * FROM tenants WHERE name = $1;

-- name: UpdateTenant :one
UPDATE tenants
SET name = $2, status = $3, monthly_limit = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteTenant :exec
DELETE FROM tenants WHERE id = $1;

-- name: ListTenants :many
SELECT * FROM tenants ORDER BY created_at DESC;

-- name: IncrementMonthlySent :exec
UPDATE tenants
SET monthly_sent = monthly_sent + 1, updated_at = NOW()
WHERE id = $1;

-- name: ResetMonthlySent :exec
UPDATE tenants
SET monthly_sent = 0, updated_at = NOW()
WHERE id = $1;
