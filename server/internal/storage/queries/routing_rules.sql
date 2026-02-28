-- name: CreateRoutingRule :one
INSERT INTO routing_rules (group_id, priority, conditions, provider_id, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRoutingRuleByID :one
SELECT * FROM routing_rules WHERE id = $1;

-- name: ListRoutingRulesByGroupID :many
SELECT * FROM routing_rules WHERE group_id = $1 ORDER BY priority ASC;

-- name: UpdateRoutingRule :one
UPDATE routing_rules
SET priority = $2, conditions = $3, provider_id = $4, enabled = $5, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteRoutingRule :exec
DELETE FROM routing_rules WHERE id = $1;
