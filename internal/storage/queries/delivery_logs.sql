-- name: CreateDeliveryLog :one
INSERT INTO delivery_logs (message_id, provider_id, status, response_code, response_body)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListDeliveryLogsByMessageID :many
SELECT * FROM delivery_logs WHERE message_id = $1 ORDER BY delivered_at DESC;
