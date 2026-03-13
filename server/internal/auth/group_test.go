package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestGroupContext_NoGroupID(t *testing.T) {
	// Pass nil pool since we expect early return before DB access
	handler := GroupContext(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGroupIDFromContext_Valid(t *testing.T) {
	groupID := uuid.New()
	ctx := context.WithValue(context.Background(), groupIDKey, groupID)

	got := GroupIDFromContext(ctx)
	if got != groupID {
		t.Errorf("GroupIDFromContext() = %v, want %v", got, groupID)
	}
}

func TestGroupIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	got := GroupIDFromContext(ctx)
	if got != uuid.Nil {
		t.Errorf("GroupIDFromContext() = %v, want uuid.Nil", got)
	}
}

func TestGroupTypeFromContext_Valid(t *testing.T) {
	ctx := context.WithValue(context.Background(), groupTypeKey, "organization")

	got := GroupTypeFromContext(ctx)
	if got != "organization" {
		t.Errorf("GroupTypeFromContext() = %q, want %q", got, "organization")
	}
}

func TestGroupTypeFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	got := GroupTypeFromContext(ctx)
	if got != "" {
		t.Errorf("GroupTypeFromContext() = %q, want empty string", got)
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
