package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestRequireRole_Allowed(t *testing.T) {
	tests := []struct {
		name         string
		userRole     string
		allowedRoles []string
		wantStatus   int
	}{
		{
			name:         "owner allowed when owner required",
			userRole:     "owner",
			allowedRoles: []string{"owner"},
			wantStatus:   http.StatusOK,
		},
		{
			name:         "admin allowed when owner or admin required",
			userRole:     "admin",
			allowedRoles: []string{"owner", "admin"},
			wantStatus:   http.StatusOK,
		},
		{
			name:         "member allowed when all roles required",
			userRole:     "member",
			allowedRoles: []string{"owner", "admin", "member"},
			wantStatus:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireRole(tt.allowedRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			ctx := context.WithValue(req.Context(), userRoleKey, tt.userRole)
			ctx = context.WithValue(ctx, userIDKey, uuid.New())
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	tests := []struct {
		name         string
		userRole     string
		allowedRoles []string
	}{
		{
			name:         "member denied when owner required",
			userRole:     "member",
			allowedRoles: []string{"owner"},
		},
		{
			name:         "member denied when owner or admin required",
			userRole:     "member",
			allowedRoles: []string{"owner", "admin"},
		},
		{
			name:         "admin denied when owner required",
			userRole:     "admin",
			allowedRoles: []string{"owner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireRole(tt.allowedRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("handler should not be called")
			}))

			req := httptest.NewRequest("GET", "/", nil)
			ctx := context.WithValue(req.Context(), userRoleKey, tt.userRole)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
		})
	}
}

func TestRequireRole_NoRole(t *testing.T) {
	handler := RequireRole("owner", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
