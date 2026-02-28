-- name: CreateActivityLog :one
INSERT INTO activity_logs (group_id, actor_id, action, resource_type, resource_id, changes, comment, ip_address)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetActivityLogByID :one
SELECT * FROM activity_logs WHERE id = $1;

-- name: ListActivityLogsByGroupID :many
SELECT * FROM activity_logs
WHERE group_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActivityLogsByActorID :many
SELECT * FROM activity_logs
WHERE actor_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActivityLogsByResource :many
SELECT * FROM activity_logs
WHERE resource_type = $1 AND resource_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
