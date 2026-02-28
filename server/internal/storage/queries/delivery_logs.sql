-- name: CreateDeliveryLog :one
INSERT INTO delivery_logs (
    message_id, provider_id, group_id, user_id, status, provider,
    provider_message_id, response_code, response_body,
    retry_count, last_error, metadata,
    duration_ms, attempt_number
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetDeliveryLogByMessageID :one
SELECT * FROM delivery_logs WHERE message_id = $1;

-- name: GetDeliveryLogByProviderMessageID :one
SELECT * FROM delivery_logs WHERE provider_message_id = $1;

-- name: ListDeliveryLogsByMessageID :many
SELECT * FROM delivery_logs WHERE message_id = $1 ORDER BY delivered_at DESC;

-- name: ListDeliveryLogsByGroupAndStatus :many
SELECT * FROM delivery_logs
WHERE group_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdateDeliveryLogStatus :exec
UPDATE delivery_logs
SET status = $2,
    provider = $3,
    provider_message_id = $4,
    retry_count = $5,
    last_error = $6,
    metadata = $7,
    updated_at = NOW()
WHERE message_id = $1;

-- name: IncrementRetryCount :exec
UPDATE delivery_logs
SET retry_count = retry_count + 1,
    last_error = $2,
    updated_at = NOW()
WHERE message_id = $1;

-- name: CountDeliveryLogsByStatus :many
SELECT status, COUNT(*) as count FROM delivery_logs
WHERE created_at >= $1 AND created_at <= $2
GROUP BY status;

-- name: CountDeliveryLogsByProvider :many
SELECT provider, status, COUNT(*) as count FROM delivery_logs
WHERE created_at >= $1 AND created_at <= $2
GROUP BY provider, status;

-- name: CountDeliveryLogsByGroup :many
SELECT group_id, status, COUNT(*) as count FROM delivery_logs
WHERE group_id IS NOT NULL AND created_at >= $1 AND created_at <= $2
GROUP BY group_id, status;

-- name: AverageDeliveryDuration :many
SELECT provider, AVG(duration_ms)::integer as avg_duration_ms, COUNT(*) as count
FROM delivery_logs
WHERE duration_ms IS NOT NULL AND created_at >= $1 AND created_at <= $2
GROUP BY provider;
