package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// createUserRequest is the JSON body for POST /api/v1/users.
type createUserRequest struct {
	Email            string   `json:"email"`
	Password         string   `json:"password,omitempty"`
	AccountType      string   `json:"account_type"`
	Username         string   `json:"username,omitempty"`
	GroupID          string   `json:"group_id,omitempty"`
	ProviderID       string   `json:"provider_id,omitempty"`
	Role             string   `json:"role,omitempty"`
	AllowedDomains   []string `json:"allowed_domains,omitempty"`
	PasswordDisabled bool     `json:"password_disabled,omitempty"`
	DisplayName      string   `json:"display_name,omitempty"`
	Description      string   `json:"description,omitempty"`
	ApiKeyExpiresIn  string   `json:"api_key_expires_in,omitempty"`
}

// userResponse is the JSON response for a user, excluding sensitive fields.
type userResponse struct {
	ID               uuid.UUID  `json:"id"`
	Email            string     `json:"email"`
	Username         *string    `json:"username,omitempty"`
	AccountType      string     `json:"account_type"`
	Status           string     `json:"status"`
	AllowedDomains   []string   `json:"allowed_domains,omitempty"`
	ApiKey           *string    `json:"api_key,omitempty"`
	ProviderID       *uuid.UUID `json:"provider_id,omitempty"`
	HomeGroupID      *uuid.UUID `json:"home_group_id,omitempty"`
	DisplayName      *string    `json:"display_name,omitempty"`
	Description      *string    `json:"description,omitempty"`
	PasswordDisabled bool       `json:"password_disabled"`
	LastLogin        *time.Time `json:"last_login,omitempty"`
	ApiKeyExpiresAt  *time.Time `json:"api_key_expires_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// toUserResponse converts a storage.User to a userResponse.
func toUserResponse(u storage.User) userResponse {
	resp := userResponse{
		ID:               u.ID,
		Email:            u.Email,
		AccountType:      u.AccountType,
		Status:           u.Status,
		PasswordDisabled: u.PasswordDisabled,
		CreatedAt:        timestampToTime(u.CreatedAt),
		UpdatedAt:        timestampToTime(u.UpdatedAt),
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
	if u.ProviderID.Valid {
		id := uuid.UUID(u.ProviderID.Bytes)
		resp.ProviderID = &id
	}
	if u.HomeGroupID.Valid {
		id := uuid.UUID(u.HomeGroupID.Bytes)
		resp.HomeGroupID = &id
	}
	if u.DisplayName.Valid {
		s := u.DisplayName.String
		resp.DisplayName = &s
	}
	if u.Description.Valid {
		s := u.Description.String
		resp.Description = &s
	}
	if u.DeletedAt.Valid {
		t := u.DeletedAt.Time
		resp.DeletedAt = &t
	}
	return resp
}

// toUserResponseWithAPIKey converts a storage.User to a userResponse.
// The API key must be set separately by the caller since it is no longer stored on the User struct.
func toUserResponseWithAPIKey(u storage.User) userResponse {
	return toUserResponse(u)
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

		// Default account type
		if req.AccountType == "" {
			req.AccountType = "user"
		}
		if req.AccountType != "user" && req.AccountType != "smtp" {
			errs = append(errs, "account_type must be one of: user, smtp")
		}

		// For SMTP accounts: username, group_id, and provider_id are required.
		// Email is optional (defaults to {username}@smtp.internal).
		// For human users: email is required.
		if req.AccountType == "smtp" {
			if req.Username == "" {
				errs = append(errs, "username is required for smtp accounts")
			}
			if req.GroupID == "" {
				errs = append(errs, "group_id is required for smtp accounts")
			}
			if req.Email == "" {
				req.Email = req.Username + "@smtp.internal"
			}
		} else if req.Email == "" {
			errs = append(errs, "email is required")
		}

		// Password is required for human users unless password_disabled is true
		if req.AccountType == "user" && req.Password == "" && !req.PasswordDisabled {
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
			username = sql.NullString{String: strings.ToLower(req.Username), Valid: true}
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

		// Parse and validate provider_id for SMTP accounts
		var providerPgID pgtype.UUID
		if req.ProviderID != "" {
			pid, err := uuid.Parse(req.ProviderID)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid provider_id format")
				return
			}
			// Verify the provider exists and is enabled
			esp, err := queries.GetProviderByID(r.Context(), pid)
			if err != nil {
				respondError(w, http.StatusBadRequest, "provider not found")
				return
			}
			if !esp.Enabled {
				respondError(w, http.StatusBadRequest, "provider is not enabled")
				return
			}
			providerPgID = pgtype.UUID{Bytes: pid, Valid: true}
		}

		// Parse group_id early so we can validate provider belongs to the group
		var groupID uuid.UUID
		if req.GroupID != "" {
			var err error
			groupID, err = uuid.Parse(req.GroupID)
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

			// For SMTP accounts, verify provider is accessible to this group
			if req.ProviderID != "" {
				pid, _ := uuid.Parse(req.ProviderID)
				accessible, accessErr := queries.IsProviderAccessible(r.Context(), storage.IsProviderAccessibleParams{
					ID:      pid,
					GroupID: groupID,
				})
				if accessErr != nil || !accessible {
					respondError(w, http.StatusBadRequest, "provider not accessible to the specified group")
					return
				}
			} else if req.AccountType == "smtp" {
				// Default to the global stdout provider
				stdoutProvider, err := queries.GetGlobalStdoutProvider(r.Context())
				if err != nil {
					respondError(w, http.StatusBadRequest, "no default stdout provider available")
					return
				}
				providerPgID = pgtype.UUID{Bytes: stdoutProvider.ID, Valid: true}
			}
		}

		var homeGroupPgID pgtype.UUID
		if req.AccountType == "smtp" && groupID != uuid.Nil {
			homeGroupPgID = pgtype.UUID{Bytes: groupID, Valid: true}
		}

		apiKeyExpiresAt, err := parseAPIKeyExpiration(req.ApiKeyExpiresIn)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		user, err := queries.CreateUser(r.Context(), storage.CreateUserParams{
			Email:            req.Email,
			PasswordHash:     passwordHash,
			AccountType:      req.AccountType,
			Username:         username,
			AllowedDomains:   domainsJSON,
			PasswordDisabled: req.PasswordDisabled,
			ProviderID:       providerPgID,
			HomeGroupID:      homeGroupPgID,
			DisplayName:      sql.NullString{String: req.DisplayName, Valid: req.DisplayName != ""},
			Description:      pgtype.Text{String: req.Description, Valid: req.Description != ""},
		})
		if err != nil {
			if req.AccountType == "smtp" {
				respondError(w, http.StatusConflict, "username already in use")
			} else {
				respondError(w, http.StatusConflict, "email already in use")
			}
			return
		}

		// Create group membership if group_id is provided
		if req.GroupID != "" {
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

		// For SMTP accounts, create the API key in api_keys table
		if req.AccountType == "smtp" && apiKey.Valid {
			keyPrefix := auth.APIKeyPrefix(apiKey.String)
			keyHash, hashErr := auth.HashAPIKey(apiKey.String)
			if hashErr != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			apiKeyRecord, createErr := queries.CreateAPIKey(r.Context(), storage.CreateAPIKeyParams{
				UserID:    user.ID,
				KeyPrefix: keyPrefix,
				KeyHash:   keyHash,
				Label:     "default",
				ExpiresAt: apiKeyExpiresAt,
				IsActive:  true,
			})
			if createErr != nil {
				respondError(w, http.StatusInternalServerError, "failed to create API key")
				return
			}
			resp := toUserResponse(user)
			resp.ApiKey = &apiKey.String
			if apiKeyRecord.ExpiresAt.Valid {
				t := apiKeyRecord.ExpiresAt.Time
				resp.ApiKeyExpiresAt = &t
			}
			respondJSON(w, http.StatusCreated, resp)
			return
		}

		respondJSON(w, http.StatusCreated, toUserResponse(user))
	}
}

// ListUserMembershipsHandler handles GET /api/v1/users/{id}/memberships.
// Returns all group memberships for a user.
func ListUserMembershipsHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		memberships, err := queries.ListMembershipsByUserID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		type membershipResponse struct {
			ID        uuid.UUID `json:"id"`
			GroupID   uuid.UUID `json:"group_id"`
			GroupName string    `json:"group_name"`
			GroupType string    `json:"group_type"`
			Role      string    `json:"role"`
			CreatedAt time.Time `json:"created_at"`
		}

		resp := make([]membershipResponse, len(memberships))
		for i, m := range memberships {
			resp[i] = membershipResponse{
				ID:        m.ID,
				GroupID:   m.GroupID,
				GroupName: m.GroupName,
				GroupType: m.GroupType,
				Role:      m.Role,
				CreatedAt: timestampToTime(m.CreatedAt),
			}
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// ListUsersHandler handles GET /api/v1/users.
// System admins see all users; group admins/owners see only their group's users.
// Regular members are denied access.
func ListUsersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerRole := auth.RoleFromContext(r.Context())
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		callerGroupID := auth.GroupIDFromContext(r.Context())

		// Only admin+ roles can list users
		if callerRole != "admin" && callerRole != "owner" {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		var users []storage.User
		var err error

		if callerGroupType == "system" {
			users, err = queries.ListUsers(r.Context())
		} else {
			users, err = queries.ListUsersByGroupID(r.Context(), callerGroupID)
		}
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

		if _, err := queries.SoftDeleteUser(r.Context(), id); err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.delete_user", "user", id.String(), nil)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// updatePasswordDisabledRequest is the JSON body for PATCH /api/v1/users/{id}/password-disabled.
type updatePasswordDisabledRequest struct {
	PasswordDisabled bool `json:"password_disabled"`
}

// UpdatePasswordDisabledHandler handles PATCH /api/v1/users/{id}/password-disabled.
// Toggles the password_disabled flag for a user.
func UpdatePasswordDisabledHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		var req updatePasswordDisabledRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		user, err := queries.UpdatePasswordDisabled(r.Context(), storage.UpdatePasswordDisabledParams{
			ID:               id,
			PasswordDisabled: req.PasswordDisabled,
		})
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.update_password_disabled", "user", id.String(), map[string]interface{}{
				"password_disabled": req.PasswordDisabled,
			})
		}

		respondJSON(w, http.StatusOK, toUserResponse(user))
	}
}

// RestoreUserHandler handles POST /api/v1/users/{id}/restore.
// Restores a soft-deleted user by clearing deleted_at and setting status to active.
func RestoreUserHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		user, err := queries.RestoreUser(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.restore_user", "user", id.String(), nil)
		}

		respondJSON(w, http.StatusOK, toUserResponse(user))
	}
}

// ListDeletedUsersHandler handles GET /api/v1/users/deleted.
// Returns all soft-deleted users.
func ListDeletedUsersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := queries.ListDeletedUsers(r.Context())
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

// ResetAPIKeyHandler handles POST /api/v1/users/{id}/reset-api-key.
// Generates a new API key for an SMTP user account.
func ResetAPIKeyHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
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
		if user.AccountType != "smtp" {
			respondError(w, http.StatusBadRequest, "user is not an SMTP account")
			return
		}

		var resetReq resetAPIKeyRequest
		_ = json.NewDecoder(r.Body).Decode(&resetReq)

		apiKeyExpiresAt, err := parseAPIKeyExpiration(resetReq.ApiKeyExpiresIn)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		newKey, err := auth.GenerateAPIKey()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Delete all existing API keys, then create a new one
		if err := queries.DeleteAllAPIKeysByUserID(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to reset API key")
			return
		}

		keyPrefix := auth.APIKeyPrefix(newKey)
		keyHash, err := auth.HashAPIKey(newKey)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		newApiKeyRecord, err := queries.CreateAPIKey(r.Context(), storage.CreateAPIKeyParams{
			UserID:    id,
			KeyPrefix: keyPrefix,
			KeyHash:   keyHash,
			Label:     "default",
			ExpiresAt: apiKeyExpiresAt,
			IsActive:  true,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create API key")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.reset_api_key", "user", id.String(), nil)
		}

		resp := toUserResponse(user)
		resp.ApiKey = &newKey
		if newApiKeyRecord.ExpiresAt.Valid {
			t := newApiKeyRecord.ExpiresAt.Time
			resp.ApiKeyExpiresAt = &t
		}
		respondJSON(w, http.StatusOK, resp)
	}
}
