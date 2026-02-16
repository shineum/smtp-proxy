package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestBearerAuth_ValidKey(t *testing.T) {
	expectedID := uuid.New()
	lookup := func(ctx context.Context, apiKey string) (uuid.UUID, error) {
		if apiKey == "valid-key" {
			return expectedID, nil
		}
		return uuid.Nil, errors.New("not found")
	}

	handler := BearerAuth(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := AccountFromContext(r.Context())
		if id != expectedID {
			t.Errorf("AccountFromContext() = %v, want %v", id, expectedID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	lookup := func(ctx context.Context, apiKey string) (uuid.UUID, error) {
		return uuid.Nil, errors.New("not found")
	}

	handler := BearerAuth(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerAuth_InvalidFormat(t *testing.T) {
	lookup := func(ctx context.Context, apiKey string) (uuid.UUID, error) {
		return uuid.Nil, errors.New("not found")
	}

	handler := BearerAuth(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic some-credentials")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerAuth_InvalidKey(t *testing.T) {
	lookup := func(ctx context.Context, apiKey string) (uuid.UUID, error) {
		return uuid.Nil, errors.New("not found")
	}

	handler := BearerAuth(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAccountFromContext_NoAccount(t *testing.T) {
	ctx := context.Background()
	id := AccountFromContext(ctx)

	if id != uuid.Nil {
		t.Errorf("AccountFromContext() = %v, want uuid.Nil", id)
	}
}
