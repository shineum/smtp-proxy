-- name: CreateGroup :one
INSERT INTO groups (name, group_type)
VALUES ($1, $2)
RETURNING *;

-- name: GetGroupByID :one
SELECT * FROM groups WHERE id = $1;

-- name: GetGroupByName :one
SELECT * FROM groups WHERE name = $1;

-- name: ListGroups :many
SELECT * FROM groups ORDER BY created_at DESC;

-- name: UpdateGroup :one
UPDATE groups
SET name = $2, status = $3, monthly_limit = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateGroupStatus :one
UPDATE groups
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = $1;

-- name: IncrementMonthlySent :exec
UPDATE groups
SET monthly_sent = monthly_sent + 1, updated_at = NOW()
WHERE id = $1;

-- name: ResetMonthlySent :exec
UPDATE groups
SET monthly_sent = 0, updated_at = NOW()
WHERE id = $1;
