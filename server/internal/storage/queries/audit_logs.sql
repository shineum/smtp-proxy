-- name: CreateAuditLog :one
INSERT INTO audit_logs (tenant_id, user_id, action, resource_type, resource_id, result, metadata, ip_address)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListAuditLogsByTenantID :many
SELECT * FROM audit_logs
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
