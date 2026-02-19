package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestTenantContext_NoTenantID(t *testing.T) {
	// Pass nil pool since we expect early return before DB access
	handler := TenantContext(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTenantFromContext_Valid(t *testing.T) {
	tenantID := uuid.New()
	ctx := context.WithValue(context.Background(), tenantIDKey, tenantID)

	got := TenantFromContext(ctx)
	if got != tenantID {
		t.Errorf("TenantFromContext() = %v, want %v", got, tenantID)
	}
}

func TestTenantFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	got := TenantFromContext(ctx)
	if got != uuid.Nil {
		t.Errorf("TenantFromContext() = %v, want uuid.Nil", got)
	}
}

func TestUserFromContext_Valid(t *testing.T) {
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), userIDKey, userID)

	got := UserFromContext(ctx)
	if got != userID {
		t.Errorf("UserFromContext() = %v, want %v", got, userID)
	}
}

func TestUserFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	got := UserFromContext(ctx)
	if got != uuid.Nil {
		t.Errorf("UserFromContext() = %v, want uuid.Nil", got)
	}
}

func TestRoleFromContext_Valid(t *testing.T) {
	ctx := context.WithValue(context.Background(), userRoleKey, "admin")

	got := RoleFromContext(ctx)
	if got != "admin" {
		t.Errorf("RoleFromContext() = %q, want %q", got, "admin")
	}
}

func TestRoleFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	got := RoleFromContext(ctx)
	if got != "" {
		t.Errorf("RoleFromContext() = %q, want empty string", got)
	}
}

func TestUserEmailFromContext_Valid(t *testing.T) {
	ctx := context.WithValue(context.Background(), userEmailKey, "user@example.com")

	got := UserEmailFromContext(ctx)
	if got != "user@example.com" {
		t.Errorf("UserEmailFromContext() = %q, want %q", got, "user@example.com")
	}
}

func TestUserEmailFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	got := UserEmailFromContext(ctx)
	if got != "" {
		t.Errorf("UserEmailFromContext() = %q, want empty string", got)
	}
}
