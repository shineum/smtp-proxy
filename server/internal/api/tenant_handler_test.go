package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestToTenantResponse(t *testing.T) {
	now := time.Now()
	tenant := storage.Tenant{
		ID:           uuid.New(),
		Name:         "test-tenant",
		Status:       "active",
		MonthlyLimit: 10000,
		MonthlySent:  500,
		CreatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := toTenantResponse(tenant)

	if resp.ID != tenant.ID {
		t.Errorf("ID = %v, want %v", resp.ID, tenant.ID)
	}
	if resp.Name != tenant.Name {
		t.Errorf("Name = %q, want %q", resp.Name, tenant.Name)
	}
	if resp.Status != tenant.Status {
		t.Errorf("Status = %q, want %q", resp.Status, tenant.Status)
	}
	if resp.MonthlyLimit != tenant.MonthlyLimit {
		t.Errorf("MonthlyLimit = %d, want %d", resp.MonthlyLimit, tenant.MonthlyLimit)
	}
	if resp.MonthlySent != tenant.MonthlySent {
		t.Errorf("MonthlySent = %d, want %d", resp.MonthlySent, tenant.MonthlySent)
	}
}
