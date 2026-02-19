package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestToUserResponse(t *testing.T) {
	now := time.Now()
	user := storage.User{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		Email:     "user@example.com",
		Role:      "admin",
		Status:    "active",
		LastLogin: pgtype.Timestamptz{Time: now, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := toUserResponse(user)

	if resp.ID != user.ID {
		t.Errorf("ID = %v, want %v", resp.ID, user.ID)
	}
	if resp.TenantID != user.TenantID {
		t.Errorf("TenantID = %v, want %v", resp.TenantID, user.TenantID)
	}
	if resp.Email != user.Email {
		t.Errorf("Email = %q, want %q", resp.Email, user.Email)
	}
	if resp.Role != user.Role {
		t.Errorf("Role = %q, want %q", resp.Role, user.Role)
	}
	if resp.Status != user.Status {
		t.Errorf("Status = %q, want %q", resp.Status, user.Status)
	}
	if resp.LastLogin == nil {
		t.Error("LastLogin should not be nil")
	}
}

func TestToUserResponse_NoLastLogin(t *testing.T) {
	now := time.Now()
	user := storage.User{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		Email:     "user@example.com",
		Role:      "member",
		Status:    "active",
		LastLogin: pgtype.Timestamptz{Valid: false},
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := toUserResponse(user)

	if resp.LastLogin != nil {
		t.Error("LastLogin should be nil when not set")
	}
}

func TestValidRoles(t *testing.T) {
	tests := []struct {
		role  string
		valid bool
	}{
		{"owner", true},
		{"admin", true},
		{"member", true},
		{"superadmin", false},
		{"", false},
		{"OWNER", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			_, ok := validRoles[tt.role]
			if ok != tt.valid {
				t.Errorf("validRoles[%q] = %v, want %v", tt.role, ok, tt.valid)
			}
		})
	}
}
