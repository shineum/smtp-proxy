package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuth_ValidKey(t *testing.T) {
	var expectedID int32 = 42
	lookup := func(ctx context.Context, apiKey string) (int32, error) {
		if apiKey == "valid-key" {
			return expectedID, nil
		}
		return 0, errors.New("not found")
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
	lookup := func(ctx context.Context, apiKey string) (int32, error) {
		return 0, errors.New("not found")
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
	lookup := func(ctx context.Context, apiKey string) (int32, error) {
		return 0, errors.New("not found")
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
	lookup := func(ctx context.Context, apiKey string) (int32, error) {
		return 0, errors.New("not found")
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

	if id != 0 {
		t.Errorf("AccountFromContext() = %v, want 0", id)
	}
}
