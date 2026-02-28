-- name: EnqueueMessage :one
INSERT INTO messages (user_id, group_id, sender, recipients, subject, headers, body, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'queued')
RETURNING *;

-- name: EnqueueMessageMetadata :one
INSERT INTO messages (user_id, group_id, sender, recipients, subject, headers, storage_ref, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'queued')
RETURNING *;

-- name: GetMessageByID :one
SELECT * FROM messages WHERE id = $1;

-- name: ListMessagesByGroupID :many
SELECT * FROM messages WHERE group_id = $1 ORDER BY enqueued_at DESC LIMIT $2;

-- name: UpdateMessageStatus :exec
UPDATE messages SET status = $2, processed_at = NOW() WHERE id = $1;

-- name: GetQueuedMessages :many
SELECT * FROM messages WHERE status = 'queued' ORDER BY enqueued_at ASC LIMIT $1;
