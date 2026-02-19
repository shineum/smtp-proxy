package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// createTenantRequest is the JSON body for POST /api/v1/tenants.
type createTenantRequest struct {
	Name         string `json:"name"`
	MonthlyLimit int32  `json:"monthly_limit,omitempty"`
	// OwnerEmail and OwnerPassword create the initial owner user.
	OwnerEmail    string `json:"owner_email"`
	OwnerPassword string `json:"owner_password"`
}

// tenantResponse is the JSON response for a tenant.
type tenantResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	MonthlyLimit int32     `json:"monthly_limit"`
	MonthlySent  int32     `json:"monthly_sent"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// toTenantResponse converts a storage.Tenant to a tenantResponse.
func toTenantResponse(t storage.Tenant) tenantResponse {
	return tenantResponse{
		ID:           t.ID,
		Name:         t.Name,
		Status:       t.Status,
		MonthlyLimit: t.MonthlyLimit,
		MonthlySent:  t.MonthlySent,
		CreatedAt:    timestampToTime(t.CreatedAt),
		UpdatedAt:    timestampToTime(t.UpdatedAt),
	}
}

// CreateTenantHandler handles POST /api/v1/tenants.
// Creates a new tenant and its initial owner user. No auth required for initial setup.
func CreateTenantHandler(queries storage.Querier, jwtService *auth.JWTService, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate required fields
		var errs []string
		if req.Name == "" {
			errs = append(errs, "name is required")
		}
		if req.OwnerEmail == "" {
			errs = append(errs, "owner_email is required")
		}
		if req.OwnerPassword == "" {
			errs = append(errs, "owner_password is required")
		}
		if len(errs) > 0 {
			respondValidationErrors(w, errs)
			return
		}

		// Default monthly limit
		monthlyLimit := req.MonthlyLimit
		if monthlyLimit <= 0 {
			monthlyLimit = 10000
		}

		// Create tenant
		tenant, err := queries.CreateTenant(r.Context(), storage.CreateTenantParams{
			Name:         req.Name,
			MonthlyLimit: monthlyLimit,
		})
		if err != nil {
			respondError(w, http.StatusConflict, "tenant name already exists")
			return
		}

		// Hash password for the owner user
		hash, err := auth.HashPassword(req.OwnerPassword)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Create owner user
		_, err = queries.CreateUser(r.Context(), storage.CreateUserParams{
			TenantID:     tenant.ID,
			Email:        req.OwnerEmail,
			PasswordHash: hash,
			Role:         "owner",
		})
		if err != nil {
			// Rollback: delete the tenant if user creation fails
			_ = queries.DeleteTenant(r.Context(), tenant.ID)
			respondError(w, http.StatusConflict, "email already in use")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionCreateTenant, "tenant", tenant.ID.String(), map[string]interface{}{
				"tenant_name": req.Name,
			})
		}

		respondJSON(w, http.StatusCreated, toTenantResponse(tenant))
	}
}

// GetTenantHandler handles GET /api/v1/tenants/{id}.
// Returns tenant details. Requires owner or admin role.
func GetTenantHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid tenant ID format")
			return
		}

		// Verify the requesting user belongs to this tenant
		tenantID := auth.TenantFromContext(r.Context())
		if tenantID != id {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		tenant, err := queries.GetTenantByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "tenant not found")
			return
		}

		respondJSON(w, http.StatusOK, toTenantResponse(tenant))
	}
}

// DeleteTenantHandler handles DELETE /api/v1/tenants/{id}.
// Deletes a tenant and all associated data. Requires owner role.
func DeleteTenantHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid tenant ID format")
			return
		}

		// Verify the requesting user belongs to this tenant
		tenantID := auth.TenantFromContext(r.Context())
		if tenantID != id {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		if err := queries.DeleteTenant(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionDeleteTenant, "tenant", id.String(), nil)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
