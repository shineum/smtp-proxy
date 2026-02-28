package api

import (
	"database/sql"
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
	Email          string   `json:"email"`
	Password       string   `json:"password,omitempty"`
	AccountType    string   `json:"account_type"`
	Username       string   `json:"username,omitempty"`
	GroupID        string   `json:"group_id,omitempty"`
	Role           string   `json:"role,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

// userResponse is the JSON response for a user, excluding sensitive fields.
type userResponse struct {
	ID             uuid.UUID  `json:"id"`
	Email          string     `json:"email"`
	Username       *string    `json:"username,omitempty"`
	AccountType    string     `json:"account_type"`
	Status         string     `json:"status"`
	AllowedDomains []string   `json:"allowed_domains,omitempty"`
	ApiKey         *string    `json:"api_key,omitempty"`
	LastLogin      *time.Time `json:"last_login,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// toUserResponse converts a storage.User to a userResponse.
func toUserResponse(u storage.User) userResponse {
	resp := userResponse{
		ID:          u.ID,
		Email:       u.Email,
		AccountType: u.AccountType,
		Status:      u.Status,
		CreatedAt:   timestampToTime(u.CreatedAt),
		UpdatedAt:   timestampToTime(u.UpdatedAt),
	}
	if u.Username.Valid {
		resp.Username = &u.Username.String
	}
	if u.LastLogin.Valid {
		t := u.LastLogin.Time
		resp.LastLogin = &t
	}
	if len(u.AllowedDomains) > 0 {
		resp.AllowedDomains = decodeDomains(u.AllowedDomains)
	}
	return resp
}

// toUserResponseWithAPIKey converts a storage.User to a userResponse, including the API key.
func toUserResponseWithAPIKey(u storage.User) userResponse {
	resp := toUserResponse(u)
	if u.ApiKey.Valid {
		resp.ApiKey = &u.ApiKey.String
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
// Creates a new user (human or smtp account).
// For account_type="smtp", auto-generates an API key and password is optional.
// If group_id is provided, creates a group membership. For system admins,
// any group can be specified. For non-system users, the group must match their own.
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

		// Default account type
		if req.AccountType == "" {
			req.AccountType = "user"
		}
		if req.AccountType != "user" && req.AccountType != "smtp" {
			errs = append(errs, "account_type must be one of: user, smtp")
		}

		// Password is required for human users, optional for SMTP
		if req.AccountType == "user" && req.Password == "" {
			errs = append(errs, "password is required for user accounts")
		}

		if req.Role == "" {
			req.Role = "member"
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

		// Hash password
		var passwordHash string
		if req.Password != "" {
			hash, err := auth.HashPassword(req.Password)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			passwordHash = hash
		} else if req.AccountType == "smtp" {
			// Generate a random password hash for SMTP accounts that don't log in
			hash, err := auth.HashPassword(uuid.New().String())
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			passwordHash = hash
		}

		// Auto-generate API key for SMTP accounts
		var apiKey sql.NullString
		if req.AccountType == "smtp" {
			key, err := auth.GenerateAPIKey()
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			apiKey = sql.NullString{String: key, Valid: true}
		}

		// Build username
		var username sql.NullString
		if req.Username != "" {
			username = sql.NullString{String: req.Username, Valid: true}
		}

		// Marshal allowed domains
		var domainsJSON []byte
		if len(req.AllowedDomains) > 0 {
			var err error
			domainsJSON, err = json.Marshal(req.AllowedDomains)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}

		user, err := queries.CreateUser(r.Context(), storage.CreateUserParams{
			Email:          req.Email,
			PasswordHash:   passwordHash,
			AccountType:    req.AccountType,
			Username:       username,
			ApiKey:         apiKey,
			AllowedDomains: domainsJSON,
		})
		if err != nil {
			respondError(w, http.StatusConflict, "email already in use")
			return
		}

		// Create group membership if group_id is provided
		if req.GroupID != "" {
			groupID, err := uuid.Parse(req.GroupID)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid group_id format")
				return
			}

			// Verify the caller has access to this group
			callerGroupID := auth.GroupIDFromContext(r.Context())
			callerGroupType := auth.GroupTypeFromContext(r.Context())
			if callerGroupType != "system" && callerGroupID != groupID {
				respondError(w, http.StatusForbidden, "access denied to the specified group")
				return
			}

			_, err = queries.CreateGroupMember(r.Context(), storage.CreateGroupMemberParams{
				GroupID: groupID,
				UserID:  user.ID,
				Role:    req.Role,
			})
			if err != nil {
				respondError(w, http.StatusConflict, "failed to add user to group")
				return
			}
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionCreateUser, "user", user.ID.String(), map[string]interface{}{
				"email":        req.Email,
				"account_type": req.AccountType,
			})
		}

		// Return API key in response for SMTP accounts
		if req.AccountType == "smtp" {
			respondJSON(w, http.StatusCreated, toUserResponseWithAPIKey(user))
			return
		}

		respondJSON(w, http.StatusCreated, toUserResponse(user))
	}
}

// ListUsersHandler handles GET /api/v1/users.
// Lists all users. Requires system admin access.
func ListUsersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := queries.ListUsers(r.Context())
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

// GetUserHandler handles GET /api/v1/users/{id}.
// Returns user details.
func GetUserHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		user, err := queries.GetUserByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		respondJSON(w, http.StatusOK, toUserResponse(user))
	}
}

// UpdateUserStatusHandler handles PATCH /api/v1/users/{id}/status.
// Updates a user's status. Requires owner or admin role.
type updateUserStatusRequest struct {
	Status string `json:"status"`
}

// UpdateUserStatusHandler handles PATCH /api/v1/users/{id}/status.
func UpdateUserStatusHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		var req updateUserStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		validStatuses := map[string]bool{
			"active":    true,
			"suspended": true,
		}
		if !validStatuses[req.Status] {
			respondError(w, http.StatusBadRequest, "status must be one of: active, suspended")
			return
		}

		user, err := queries.UpdateUserStatus(r.Context(), storage.UpdateUserStatusParams{
			ID:     id,
			Status: req.Status,
		})
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.update_user_status", "user", id.String(), map[string]interface{}{
				"status": req.Status,
			})
		}

		respondJSON(w, http.StatusOK, toUserResponse(user))
	}
}

// DeleteUserHandler handles DELETE /api/v1/users/{id}.
// Deletes a user and all their group memberships.
func DeleteUserHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		// Remove all group memberships first
		_ = queries.DeleteGroupMembersByUserID(r.Context(), id)

		if err := queries.DeleteUser(r.Context(), id); err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.delete_user", "user", id.String(), nil)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
