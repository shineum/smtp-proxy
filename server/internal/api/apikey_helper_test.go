package api

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// TestCreateUniqueAPIKey_FirstAttemptSucceeds is the happy path — no
// collision, helper returns the generated key after a single CreateAPIKey
// call.
func TestCreateUniqueAPIKey_FirstAttemptSucceeds(t *testing.T) {
	userID := uuid.New()
	var calls int
	mock := &mockQuerier{
		createAPIKeyFn: func(_ context.Context, arg storage.CreateAPIKeyParams) (storage.ApiKey, error) {
			calls++
			return storage.ApiKey{
				ID:        uuid.New(),
				UserID:    arg.UserID,
				KeyPrefix: arg.KeyPrefix,
				KeyHash:   arg.KeyHash,
				Label:     arg.Label,
				IsActive:  true,
			}, nil
		},
	}

	raw, record, err := createUniqueAPIKey(context.Background(), mock, storage.CreateAPIKeyParams{
		UserID:   userID,
		Label:    "default",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("createUniqueAPIKey() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 CreateAPIKey call, got %d", calls)
	}
	if len(raw) != 64 {
		t.Errorf("expected 64-char raw key, got %d", len(raw))
	}
	if record.UserID != userID {
		t.Errorf("record.UserID = %s, want %s", record.UserID, userID)
	}
	if record.KeyPrefix != raw[:12] {
		t.Errorf("record.KeyPrefix = %q, want first 12 chars of raw %q", record.KeyPrefix, raw[:12])
	}
}

// TestCreateUniqueAPIKey_RetriesOnUniqueViolation simulates two consecutive
// unique_violation errors followed by success. Verifies the helper regenerates
// the key (different prefix each time) and returns the third attempt's row.
func TestCreateUniqueAPIKey_RetriesOnUniqueViolation(t *testing.T) {
	var calls int
	var seenPrefixes []string
	mock := &mockQuerier{
		createAPIKeyFn: func(_ context.Context, arg storage.CreateAPIKeyParams) (storage.ApiKey, error) {
			calls++
			seenPrefixes = append(seenPrefixes, arg.KeyPrefix)
			if calls < 3 {
				return storage.ApiKey{}, &pgconn.PgError{
					Code:    pgUniqueViolation,
					Message: "duplicate key value violates unique constraint",
				}
			}
			return storage.ApiKey{ID: uuid.New(), KeyPrefix: arg.KeyPrefix, KeyHash: arg.KeyHash}, nil
		},
	}

	raw, record, err := createUniqueAPIKey(context.Background(), mock, storage.CreateAPIKeyParams{
		UserID:   uuid.New(),
		Label:    "default",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("createUniqueAPIKey() error = %v after 2 collisions", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 CreateAPIKey calls (2 collisions + success), got %d", calls)
	}
	if record.KeyPrefix != raw[:12] {
		t.Errorf("returned record prefix %q does not match final raw key prefix %q", record.KeyPrefix, raw[:12])
	}
	// Each retry must use a freshly generated key (different prefix).
	if len(seenPrefixes) == 3 && (seenPrefixes[0] == seenPrefixes[1] || seenPrefixes[1] == seenPrefixes[2]) {
		t.Errorf("retries did not regenerate key prefix: %v", seenPrefixes)
	}
}

// TestCreateUniqueAPIKey_ExhaustsRetries returns ErrAPIKeyCollision when the
// max retry budget is consumed without a successful insert.
func TestCreateUniqueAPIKey_ExhaustsRetries(t *testing.T) {
	mock := &mockQuerier{
		createAPIKeyFn: func(_ context.Context, _ storage.CreateAPIKeyParams) (storage.ApiKey, error) {
			return storage.ApiKey{}, &pgconn.PgError{
				Code:    pgUniqueViolation,
				Message: "duplicate key value violates unique constraint",
			}
		},
	}

	_, _, err := createUniqueAPIKey(context.Background(), mock, storage.CreateAPIKeyParams{
		UserID:   uuid.New(),
		Label:    "default",
		IsActive: true,
	})
	if !errors.Is(err, ErrAPIKeyCollision) {
		t.Errorf("expected ErrAPIKeyCollision after %d collisions, got %v", apiKeyCollisionMaxRetries, err)
	}
}

// TestCreateUniqueAPIKey_NonCollisionErrorPropagates verifies that non-unique
// constraint errors (e.g. FK violation, connection error) are returned
// without further retries.
func TestCreateUniqueAPIKey_NonCollisionErrorPropagates(t *testing.T) {
	var calls int
	expected := errors.New("connection refused")
	mock := &mockQuerier{
		createAPIKeyFn: func(_ context.Context, _ storage.CreateAPIKeyParams) (storage.ApiKey, error) {
			calls++
			return storage.ApiKey{}, expected
		},
	}

	_, _, err := createUniqueAPIKey(context.Background(), mock, storage.CreateAPIKeyParams{
		UserID:   uuid.New(),
		Label:    "default",
		IsActive: true,
	})
	if !errors.Is(err, expected) {
		t.Errorf("expected error to wrap %q, got %v", expected, err)
	}
	if calls != 1 {
		t.Errorf("expected single call (no retry on non-collision), got %d", calls)
	}
}

// TestIsPgUniqueViolation_WrappedError verifies the helper handles wrapped
// errors (errors.As semantics).
func TestIsPgUniqueViolation_WrappedError(t *testing.T) {
	pgErr := &pgconn.PgError{Code: pgUniqueViolation}
	wrapped := fmt.Errorf("insert failed: %w", pgErr)
	if !isPgUniqueViolation(wrapped) {
		t.Error("isPgUniqueViolation should detect 23505 inside wrapped error")
	}
	otherPg := &pgconn.PgError{Code: "23503"} // foreign_key_violation
	if isPgUniqueViolation(otherPg) {
		t.Error("isPgUniqueViolation should not match 23503 (foreign_key_violation)")
	}
	if isPgUniqueViolation(errors.New("plain string")) {
		t.Error("isPgUniqueViolation should not match a non-pg error")
	}
}
