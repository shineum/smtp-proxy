-- name: CreateAccount :one
INSERT INTO accounts (name, email, password_hash, allowed_domains, api_key)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetAccountByAPIKey :one
SELECT * FROM accounts WHERE api_key = $1;

-- name: GetAccountByName :one
SELECT * FROM accounts WHERE name = $1;

-- name: UpdateAccount :one
UPDATE accounts
SET name = $2, email = $3, allowed_domains = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteAccount :exec
DELETE FROM accounts WHERE id = $1;

-- name: ListAccounts :many
SELECT * FROM accounts ORDER BY created_at DESC;
