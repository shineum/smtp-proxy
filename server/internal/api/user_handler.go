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

// createUserRequest is the JSON body for POST /api/v1/users.
type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// updateUserRoleRequest is the JSON body for PATCH /api/v1/users/{id}/role.
type updateUserRoleRequest struct {
	Role string `json:"role"`
}

// userResponse is the JSON response for a user, excluding sensitive fields.
type userResponse struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	LastLogin *time.Time `json:"last_login,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// toUserResponse converts a storage.User to a userResponse.
func toUserResponse(u storage.User) userResponse {
	resp := userResponse{
		ID:        u.ID,
		TenantID:  u.TenantID,
		Email:     u.Email,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: timestampToTime(u.CreatedAt),
		UpdatedAt: timestampToTime(u.UpdatedAt),
	}
	if u.LastLogin.Valid {
		t := u.LastLogin.Time
		resp.LastLogin = &t
	}
	return resp
}

// validRoles is the set of valid user roles.
var validRoles = map[string]struct{}{
	"owner":  {},
	"admin":  {},
	"member": {},
}

// CreateUserHandler handles POST /api/v1/users.
// Creates a new user within the authenticated user's tenant.
// Requires owner or admin role.
func CreateUserHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate required fields
		var errs []string
		if req.Email == "" {
			errs = append(errs, "email is required")
		}
		if req.Password == "" {
			errs = append(errs, "password is required")
		}
		if req.Role == "" {
			req.Role = "member" // Default role
		}
		if _, ok := validRoles[req.Role]; !ok {
			errs = append(errs, "role must be one of: owner, admin, member")
		}
		if len(errs) > 0 {
			respondValidationErrors(w, errs)
			return
		}

		// Only owners can create other owners
		callerRole := auth.RoleFromContext(r.Context())
		if req.Role == "owner" && callerRole != "owner" {
			respondError(w, http.StatusForbidden, "only owners can create owner accounts")
			return
		}

		tenantID := auth.TenantFromContext(r.Context())

		// Hash password
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		user, err := queries.CreateUser(r.Context(), storage.CreateUserParams{
			TenantID:     tenantID,
			Email:        req.Email,
			PasswordHash: hash,
			Role:         req.Role,
		})
		if err != nil {
			respondError(w, http.StatusConflict, "email already in use")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionCreateUser, "user", user.ID.String(), map[string]interface{}{
				"email": req.Email,
				"role":  req.Role,
			})
		}

		respondJSON(w, http.StatusCreated, toUserResponse(user))
	}
}

// ListUsersHandler handles GET /api/v1/users.
// Lists all users in the authenticated user's tenant.
func ListUsersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantFromContext(r.Context())

		users, err := queries.ListUsersByTenantID(r.Context(), tenantID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		resp := make([]userResponse, len(users))
		for i, u := range users {
			resp[i] = toUserResponse(u)
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// UpdateUserRoleHandler handles PATCH /api/v1/users/{id}/role.
// Updates a user's role. Requires owner or admin role.
func UpdateUserRoleHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		var req updateUserRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if _, ok := validRoles[req.Role]; !ok {
			respondError(w, http.StatusBadRequest, "role must be one of: owner, admin, member")
			return
		}

		// Only owners can assign the owner role
		callerRole := auth.RoleFromContext(r.Context())
		if req.Role == "owner" && callerRole != "owner" {
			respondError(w, http.StatusForbidden, "only owners can assign the owner role")
			return
		}

		// Verify the target user belongs to the same tenant
		targetUser, err := queries.GetUserByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		tenantID := auth.TenantFromContext(r.Context())
		if targetUser.TenantID != tenantID {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		// Prevent demoting the last owner
		if targetUser.Role == "owner" && req.Role != "owner" {
			// Check if there are other owners
			users, err := queries.ListUsersByTenantID(r.Context(), tenantID)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			ownerCount := 0
			for _, u := range users {
				if u.Role == "owner" {
					ownerCount++
				}
			}
			if ownerCount <= 1 {
				respondError(w, http.StatusConflict, "cannot demote the last owner")
				return
			}
		}

		user, err := queries.UpdateUserRole(r.Context(), storage.UpdateUserRoleParams{
			ID:   id,
			Role: req.Role,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionUpdateRole, "user", id.String(), map[string]interface{}{
				"old_role": targetUser.Role,
				"new_role": req.Role,
			})
		}

		respondJSON(w, http.StatusOK, toUserResponse(user))
	}
}
