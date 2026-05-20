package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// pgUniqueViolation is the SQLSTATE code Postgres returns when a unique
// constraint is violated. See https://www.postgresql.org/docs/current/errcodes-appendix.html
const pgUniqueViolation = "23505"

// apiKeyCollisionMaxRetries bounds how many times we will regenerate an API
// key when the first 12 hex chars (key_prefix) collide with an existing row.
// 12 hex chars = 48 bits of entropy, so collisions are astronomically rare,
// but the unique index on api_keys.key_prefix makes them possible.
const apiKeyCollisionMaxRetries = 3

// ErrAPIKeyCollision is returned when we exhausted retries without finding
// a unique key prefix. Callers should treat this as an internal error.
var ErrAPIKeyCollision = errors.New("api key prefix collision after retries")

// createUniqueAPIKey generates a new API key, hashes it, and inserts a row
// into api_keys. If the database rejects the row because key_prefix is not
// unique (SQLSTATE 23505), a fresh key is generated and the insert is retried
// up to apiKeyCollisionMaxRetries times. Any other CreateAPIKey error is
// returned unchanged.
//
// Returns the raw plaintext key (to be displayed once to the caller) and the
// stored ApiKey row.
//
// REQ-AUTH-026: prevents user-visible 500s on the rare unique_violation path.
func createUniqueAPIKey(ctx context.Context, q storage.Querier, base storage.CreateAPIKeyParams) (string, storage.ApiKey, error) {
	for attempt := 0; attempt < apiKeyCollisionMaxRetries; attempt++ {
		rawKey, err := auth.GenerateAPIKey()
		if err != nil {
			return "", storage.ApiKey{}, fmt.Errorf("generate api key: %w", err)
		}
		keyHash, err := auth.HashAPIKey(rawKey)
		if err != nil {
			return "", storage.ApiKey{}, fmt.Errorf("hash api key: %w", err)
		}

		params := base
		params.KeyPrefix = auth.APIKeyPrefix(rawKey)
		params.KeyHash = keyHash

		record, err := q.CreateAPIKey(ctx, params)
		if err == nil {
			return rawKey, record, nil
		}
		if !isPgUniqueViolation(err) {
			return "", storage.ApiKey{}, err
		}
		// Collision — loop and regenerate.
	}
	return "", storage.ApiKey{}, ErrAPIKeyCollision
}

// isPgUniqueViolation reports whether err is a Postgres unique_violation
// (SQLSTATE 23505), regardless of how deeply it is wrapped.
func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}
	return false
}
