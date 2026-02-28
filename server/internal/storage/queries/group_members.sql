-- name: CreateGroupMember :one
INSERT INTO group_members (group_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetGroupMemberByID :one
SELECT * FROM group_members WHERE id = $1;

-- name: GetGroupMemberByUserAndGroup :one
SELECT * FROM group_members WHERE user_id = $1 AND group_id = $2;

-- name: ListGroupMembersByGroupID :many
SELECT * FROM group_members WHERE group_id = $1 ORDER BY created_at ASC;

-- name: ListGroupsByUserID :many
SELECT g.* FROM groups g
JOIN group_members gm ON g.id = gm.group_id
WHERE gm.user_id = $1
ORDER BY gm.created_at ASC;

-- name: UpdateGroupMemberRole :one
UPDATE group_members
SET role = $2
WHERE id = $1
RETURNING *;

-- name: DeleteGroupMember :exec
DELETE FROM group_members WHERE id = $1;

-- name: DeleteGroupMembersByUserID :exec
DELETE FROM group_members WHERE user_id = $1;

-- name: CountGroupOwners :one
SELECT count(*) FROM group_members WHERE group_id = $1 AND role = 'owner';
