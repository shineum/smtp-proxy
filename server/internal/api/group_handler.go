package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// createGroupRequest is the JSON body for POST /api/v1/groups.
type createGroupRequest struct {
	Name         string `json:"name"`
	MonthlyLimit int32  `json:"monthly_limit,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`
	Description  string `json:"description,omitempty"`
}

// addMemberRequest is the JSON body for POST /api/v1/groups/{id}/members.
type addMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// updateMemberRoleRequest is the JSON body for PATCH /api/v1/groups/{id}/members/{uid}.
type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

// groupResponse is the JSON response for a group.
type groupResponse struct {
	ID           int32   `json:"id"`
	Name         string  `json:"name"`
	GroupType    string  `json:"group_type"`
	Status       string  `json:"status"`
	MonthlyLimit int32   `json:"monthly_limit"`
	MonthlySent  int32   `json:"monthly_sent"`
	DisplayName  *string `json:"display_name,omitempty"`
	Description  *string `json:"description,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// groupMemberResponse is the JSON response for a group member.
type groupMemberResponse struct {
	GroupID   int32   `json:"group_id"`
	UserID    int32   `json:"user_id"`
	Email     string  `json:"email,omitempty"`
	Username  *string `json:"username,omitempty"`
	Role      string  `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// toGroupResponse converts a storage.Group to a groupResponse.
func toGroupResponse(g storage.Group) groupResponse {
	resp := groupResponse{
		ID:           g.ID,
		Name:         g.Name,
		GroupType:    g.GroupType,
		Status:       g.Status,
		MonthlyLimit: g.MonthlyLimit,
		MonthlySent:  g.MonthlySent,
		CreatedAt:    timestampToTime(g.CreatedAt),
		UpdatedAt:    timestampToTime(g.UpdatedAt),
	}
	if g.DisplayName.Valid {
		s := g.DisplayName.String
		resp.DisplayName = &s
	}
	if g.Description.Valid {
		s := g.Description.String
		resp.Description = &s
	}
	return resp
}

// toGroupMemberResponse converts a storage.GroupMember to a groupMemberResponse.
func toGroupMemberResponse(gm storage.GroupMember) groupMemberResponse {
	return groupMemberResponse{
		GroupID:   gm.GroupID,
		UserID:    gm.UserID,
		Role:      gm.Role,
		CreatedAt: timestampToTime(gm.CreatedAt),
	}
}

// createServiceAccountRequest is the JSON body for POST /api/v1/groups/{id}/service-accounts.
type createServiceAccountRequest struct {
	Username         string   `json:"username"`
	Email            string   `json:"email,omitempty"`
	AllowedDomains   []string `json:"allowed_domains,omitempty"`
	ProviderID       string   `json:"provider_id"`
	ApiKeyExpiresIn  string   `json:"api_key_expires_in,omitempty"`
}

// resetAPIKeyRequest is the JSON body for reset-api-key endpoints.
type resetAPIKeyRequest struct {
	ApiKeyExpiresIn string `json:"api_key_expires_in,omitempty"`
}

// requireGroupRole checks that the caller has one of the allowed roles in the specified group.
// System admins bypass this check. Returns the caller's membership or an error.
func requireGroupRole(queries storage.Querier, r *http.Request, groupID int32, allowedRoles ...string) (storage.GroupMember, error) {
	callerGroupType := auth.GroupTypeFromContext(r.Context())
	if callerGroupType == "system" {
		return storage.GroupMember{Role: "owner"}, nil
	}

	callerUserID := auth.UserFromContext(r.Context())
	member, err := queries.GetGroupMemberByUserAndGroup(r.Context(), storage.GetGroupMemberByUserAndGroupParams{
		UserID:  callerUserID,
		GroupID: groupID,
	})
	if err != nil {
		return storage.GroupMember{}, fmt.Errorf("not a member of this group")
	}

	for _, role := range allowedRoles {
		if member.Role == role {
			return member, nil
		}
	}

	return storage.GroupMember{}, fmt.Errorf("insufficient role: requires %v", allowedRoles)
}

// CreateGroupHandler handles POST /api/v1/groups.
// Creates a new group with group_type='company' and status='active'.
// Any authenticated user can create a group; the caller becomes the owner.
func CreateGroupHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Name == "" {
			respondError(w, http.StatusBadRequest, "name is required")
			return
		}

		group, err := queries.CreateGroup(r.Context(), storage.CreateGroupParams{
			Name:        req.Name,
			GroupType:   "company",
			DisplayName: sql.NullString{String: req.DisplayName, Valid: req.DisplayName != ""},
			Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		})
		if err != nil {
			respondError(w, http.StatusConflict, "group name already exists")
			return
		}

		// If monthly_limit was specified, update it
		if req.MonthlyLimit > 0 {
			group, err = queries.UpdateGroup(r.Context(), storage.UpdateGroupParams{
				ID:           group.ID,
				Name:         group.Name,
				Status:       group.Status,
				MonthlyLimit: req.MonthlyLimit,
				DisplayName:  group.DisplayName,
				Description:  group.Description,
			})
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}

		// Auto-create owner membership for the caller
		callerUserID := auth.UserFromContext(r.Context())
		if callerUserID != 0 {
			_, _ = queries.CreateGroupMember(r.Context(), storage.CreateGroupMemberParams{
				GroupID: group.ID,
				UserID:  callerUserID,
				Role:    "owner",
			})
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionCreateGroup, "group", fmt.Sprintf("%d", group.ID), map[string]interface{}{
				"name": req.Name,
			})
		}

		respondJSON(w, http.StatusCreated, toGroupResponse(group))
	}
}

// ListGroupsHandler handles GET /api/v1/groups.
// System admins see all groups; other users see only their own groups.
func ListGroupsHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		callerUserID := auth.UserFromContext(r.Context())

		var groups []storage.Group
		var err error

		if callerGroupType == "system" {
			groups, err = queries.ListGroups(r.Context())
		} else {
			groups, err = queries.ListGroupsByUserID(r.Context(), callerUserID)
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		resp := make([]groupResponse, len(groups))
		for i, g := range groups {
			resp[i] = toGroupResponse(g)
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// GetGroupHandler handles GET /api/v1/groups/{id}.
// Returns group details. Requires membership in the group or system admin.
func GetGroupHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		n, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		id := int32(n)

		// Verify the requesting user has access to this group
		if _, err := requireGroupRole(queries, r, id, "owner", "admin", "member"); err != nil {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		group, err := queries.GetGroupByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "group not found")
			return
		}

		respondJSON(w, http.StatusOK, toGroupResponse(group))
	}
}

// DeleteGroupHandler handles DELETE /api/v1/groups/{id}.
// Soft-deletes a group by setting status='deleted'.
// Auto-suspends SMTP accounts in the group.
// Returns 403 if attempting to delete a system group.
// Requires system admin or group owner.
func DeleteGroupHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		n, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		id := int32(n)

		// Get group to check type
		group, err := queries.GetGroupByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "group not found")
			return
		}

		// Cannot delete system group
		if group.GroupType == "system" {
			respondError(w, http.StatusForbidden, "cannot delete system group")
			return
		}

		// Require system admin or group owner
		if _, err := requireGroupRole(queries, r, id, "owner"); err != nil {
			respondError(w, http.StatusForbidden, "only group owner or system admin can delete a group")
			return
		}

		// Soft-delete: set status to 'deleted'
		_, err = queries.UpdateGroupStatus(r.Context(), storage.UpdateGroupStatusParams{
			ID:     id,
			Status: "deleted",
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Auto-suspend SMTP accounts in this group
		members, err := queries.ListGroupMembersByGroupID(r.Context(), id)
		if err == nil {
			for _, member := range members {
				user, uerr := queries.GetUserByID(r.Context(), member.UserID)
				if uerr == nil && user.AccountType == "smtp" {
					_, _ = queries.UpdateUserStatus(r.Context(), storage.UpdateUserStatusParams{
						ID:     user.ID,
						Status: "suspended",
					})
				}
			}
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionDeleteGroup, "group", fmt.Sprintf("%d", id), nil)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ListGroupMembersHandler handles GET /api/v1/groups/{id}/members.
// Lists all members of a group. Requires membership in the group or system admin.
func ListGroupMembersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		n, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		id := int32(n)

		if _, err := requireGroupRole(queries, r, id, "owner", "admin", "member"); err != nil {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		members, err := queries.ListGroupMembersByGroupID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		resp := make([]groupMemberResponse, len(members))
		for i, m := range members {
			resp[i] = toGroupMemberResponse(m)
			if user, err := queries.GetUserByID(r.Context(), m.UserID); err == nil {
				resp[i].Email = user.Email
				if user.Username.Valid {
					resp[i].Username = &user.Username.String
				}
			}
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// AddGroupMemberHandler handles POST /api/v1/groups/{id}/members.
// Adds a member to a group. Requires owner or admin role in the group.
// Returns 409 if SMTP account is already in another group.
func AddGroupMemberHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		gn, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		groupID := int32(gn)

		if _, err := requireGroupRole(queries, r, groupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		var req addMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		un, err := strconv.ParseInt(req.UserID, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user_id format")
			return
		}
		userID := int32(un)

		if req.Role == "" {
			req.Role = "member"
		}
		if _, ok := validRoles[req.Role]; !ok {
			respondError(w, http.StatusBadRequest, "role must be one of: owner, admin, member")
			return
		}

		// Check if user is an SMTP account already in another group
		user, err := queries.GetUserByID(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if user.AccountType == "smtp" {
			groups, _ := queries.ListGroupsByUserID(r.Context(), userID)
			if len(groups) > 0 {
				respondError(w, http.StatusConflict, "smtp account already belongs to another group")
				return
			}
		}

		member, err := queries.CreateGroupMember(r.Context(), storage.CreateGroupMemberParams{
			GroupID: groupID,
			UserID:  userID,
			Role:    req.Role,
		})
		if err != nil {
			respondError(w, http.StatusConflict, "user is already a member of this group")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.add_member", "group_member", fmt.Sprintf("%d/%d", groupID, userID), map[string]interface{}{
				"group_id": fmt.Sprintf("%d", groupID),
				"user_id":  fmt.Sprintf("%d", userID),
				"role":     req.Role,
			})
		}

		respondJSON(w, http.StatusCreated, toGroupMemberResponse(member))
	}
}

// UpdateGroupMemberRoleHandler handles PATCH /api/v1/groups/{id}/members/{uid}.
// Updates a member's role. Owner required for promoting to owner/admin.
// Admin can manage member roles. Returns 409 if last owner.
func UpdateGroupMemberRoleHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupIDStr := chi.URLParam(r, "id")
		gn, err := strconv.ParseInt(groupIDStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		groupID := int32(gn)

		uidStr := chi.URLParam(r, "uid")
		un, err := strconv.ParseInt(uidStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}
		userID := int32(un)

		var req updateMemberRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if _, ok := validRoles[req.Role]; !ok {
			respondError(w, http.StatusBadRequest, "role must be one of: owner, admin, member")
			return
		}

		// Owner required for promoting to owner/admin; admin can manage members
		if req.Role == "owner" || req.Role == "admin" {
			if _, err := requireGroupRole(queries, r, groupID, "owner"); err != nil {
				respondError(w, http.StatusForbidden, "only owner can promote to owner or admin")
				return
			}
		} else {
			if _, err := requireGroupRole(queries, r, groupID, "owner", "admin"); err != nil {
				respondError(w, http.StatusForbidden, "owner or admin role required")
				return
			}
		}

		// Find the membership
		member, err := queries.GetGroupMemberByUserAndGroup(r.Context(), storage.GetGroupMemberByUserAndGroupParams{
			UserID:  userID,
			GroupID: groupID,
		})
		if err != nil {
			respondError(w, http.StatusNotFound, "member not found")
			return
		}

		// If demoting from owner, check if last owner
		if member.Role == "owner" && req.Role != "owner" {
			count, err := queries.CountGroupOwners(r.Context(), groupID)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			if count <= 1 {
				respondError(w, http.StatusConflict, "cannot demote the last owner")
				return
			}
		}

		oldRole := member.Role
		updated, err := queries.UpdateGroupMemberRole(r.Context(), storage.UpdateGroupMemberRoleParams{
			GroupID: groupID,
			UserID:  userID,
			Role:    req.Role,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.update_member_role", "group_member", fmt.Sprintf("%d/%d", groupID, userID), map[string]interface{}{
				"old_role": oldRole,
				"new_role": req.Role,
			})
		}

		respondJSON(w, http.StatusOK, toGroupMemberResponse(updated))
	}
}

// RemoveGroupMemberHandler handles DELETE /api/v1/groups/{id}/members/{uid}.
// Removes a member from a group. Requires owner or admin role. Returns 409 if last owner.
func RemoveGroupMemberHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupIDStr := chi.URLParam(r, "id")
		gn, err := strconv.ParseInt(groupIDStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		groupID := int32(gn)

		uidStr := chi.URLParam(r, "uid")
		un, err := strconv.ParseInt(uidStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}
		userID := int32(un)

		if _, err := requireGroupRole(queries, r, groupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		// Find the membership
		member, err := queries.GetGroupMemberByUserAndGroup(r.Context(), storage.GetGroupMemberByUserAndGroupParams{
			UserID:  userID,
			GroupID: groupID,
		})
		if err != nil {
			respondError(w, http.StatusNotFound, "member not found")
			return
		}

		// If removing an owner, check if last owner
		if member.Role == "owner" {
			count, err := queries.CountGroupOwners(r.Context(), groupID)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			if count <= 1 {
				respondError(w, http.StatusConflict, "cannot remove the last owner")
				return
			}
		}

		if err := queries.DeleteGroupMember(r.Context(), storage.DeleteGroupMemberParams{
			GroupID: groupID,
			UserID:  userID,
		}); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.remove_member", "group_member", fmt.Sprintf("%d/%d", groupID, userID), map[string]interface{}{
				"group_id": fmt.Sprintf("%d", groupID),
				"user_id":  fmt.Sprintf("%d", userID),
			})
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// updateServiceAccountRequest is the JSON body for PATCH /api/v1/groups/{id}/service-accounts/{uid}.
type updateServiceAccountRequest struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	ProviderID     string   `json:"provider_id,omitempty"`
}

// UpdateServiceAccountHandler handles PATCH /api/v1/groups/{id}/service-accounts/{uid}.
// Updates allowed_domains and/or provider_id for an SMTP service account.
// Requires owner or admin role in the group.
func UpdateServiceAccountHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		gn, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		groupID := int32(gn)

		uidStr := chi.URLParam(r, "uid")
		un, err := strconv.ParseInt(uidStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}
		userID := int32(un)

		if _, err := requireGroupRole(queries, r, groupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		// Verify user belongs to this group
		_, err = queries.GetGroupMemberByUserAndGroup(r.Context(), storage.GetGroupMemberByUserAndGroupParams{
			UserID:  userID,
			GroupID: groupID,
		})
		if err != nil {
			respondError(w, http.StatusNotFound, "user is not a member of this group")
			return
		}

		// Verify user is an SMTP service account
		user, err := queries.GetUserByID(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		if user.AccountType != "smtp" {
			respondError(w, http.StatusBadRequest, "user is not a service account")
			return
		}

		var req updateServiceAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Update allowed_domains if provided
		if req.AllowedDomains != nil {
			var domainsJSON []byte
			if len(req.AllowedDomains) > 0 {
				domainsJSON, err = json.Marshal(req.AllowedDomains)
				if err != nil {
					respondError(w, http.StatusInternalServerError, "internal server error")
					return
				}
			}
			user, err = queries.UpdateUser(r.Context(), storage.UpdateUserParams{
				ID:             user.ID,
				Email:          user.Email,
				Status:         user.Status,
				AllowedDomains: domainsJSON,
				DisplayName:    user.DisplayName,
				Description:    user.Description,
			})
			if err != nil {
				respondError(w, http.StatusInternalServerError, "failed to update allowed domains")
				return
			}
		}

		// Update provider_id if provided
		if req.ProviderID != "" {
			pn, err := strconv.ParseInt(req.ProviderID, 10, 32)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid provider_id format")
				return
			}
			providerID := int32(pn)

			accessible, err := queries.IsProviderAccessible(r.Context(), storage.IsProviderAccessibleParams{
				ID:      providerID,
				GroupID: groupID,
			})
			if err != nil || !accessible {
				respondError(w, http.StatusBadRequest, "provider not found or not accessible to this group")
				return
			}

			user, err = queries.UpdateUserProvider(r.Context(), storage.UpdateUserProviderParams{
				ID:         user.ID,
				ProviderID: pgtype.Int4{Int32: providerID, Valid: true},
			})
			if err != nil {
				respondError(w, http.StatusInternalServerError, "failed to update provider")
				return
			}
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.update_service_account", "user", fmt.Sprintf("%d", user.ID), map[string]interface{}{
				"group_id": fmt.Sprintf("%d", groupID),
			})
		}

		respondJSON(w, http.StatusOK, toUserResponse(user))
	}
}

// CreateServiceAccountHandler handles POST /api/v1/groups/{id}/service-accounts.
// Creates an SMTP service account and adds it to the group.
// Requires owner or admin role in the group.
func CreateServiceAccountHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		gn, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		groupID := int32(gn)

		if _, err := requireGroupRole(queries, r, groupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		var req createServiceAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Username == "" {
			respondError(w, http.StatusBadRequest, "username is required")
			return
		}
		req.Username = strings.ToLower(req.Username)

		var providerID int32
		if req.ProviderID != "" {
			pn, err := strconv.ParseInt(req.ProviderID, 10, 32)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid provider_id format")
				return
			}
			providerID = int32(pn)

			// Verify provider is accessible to this group (owner, shared, or global)
			accessible, err := queries.IsProviderAccessible(r.Context(), storage.IsProviderAccessibleParams{
				ID:      providerID,
				GroupID: groupID,
			})
			if err != nil || !accessible {
				respondError(w, http.StatusBadRequest, "provider not found or not accessible to this group")
				return
			}
		} else {
			// Default to the global stdout provider
			stdoutProvider, err := queries.GetGlobalStdoutProvider(r.Context())
			if err != nil {
				respondError(w, http.StatusBadRequest, "no default stdout provider available")
				return
			}
			providerID = stdoutProvider.ID
		}

		email := req.Email
		if email == "" {
			email = req.Username + "@smtp.internal"
		}

		// Generate random password hash (SMTP accounts don't log in)
		randomPwd, err := auth.GenerateAPIKey()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		passwordHash, err := auth.HashPassword(randomPwd)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Auto-generate API key
		apiKey, err := auth.GenerateAPIKey()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Marshal allowed domains
		var domainsJSON []byte
		if len(req.AllowedDomains) > 0 {
			domainsJSON, err = json.Marshal(req.AllowedDomains)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}

		// Parse API key expiration
		apiKeyExpiresAt, err := parseAPIKeyExpiration(req.ApiKeyExpiresIn)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		user, err := queries.CreateUser(r.Context(), storage.CreateUserParams{
			Email:           email,
			PasswordHash:    passwordHash,
			AccountType:     "smtp",
			Username:        sql.NullString{String: req.Username, Valid: true},
			ApiKey:          sql.NullString{String: apiKey, Valid: true},
			AllowedDomains:  domainsJSON,
			ProviderID:      pgtype.Int4{Int32: providerID, Valid: true},
			HomeGroupID:     pgtype.Int4{Int32: groupID, Valid: true},
			ApiKeyExpiresAt: apiKeyExpiresAt,
		})
		if err != nil {
			if strings.Contains(err.Error(), "users_username_key") {
				respondError(w, http.StatusConflict, "username already in use")
			} else {
				respondError(w, http.StatusInternalServerError, "failed to create service account")
			}
			return
		}

		// Add to the group as member
		_, err = queries.CreateGroupMember(r.Context(), storage.CreateGroupMemberParams{
			GroupID: groupID,
			UserID:  user.ID,
			Role:    "member",
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to add service account to group")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.create_service_account", "user", fmt.Sprintf("%d", user.ID), map[string]interface{}{
				"username": req.Username,
				"group_id": fmt.Sprintf("%d", groupID),
			})
		}

		respondJSON(w, http.StatusCreated, toUserResponseWithAPIKey(user))
	}
}

// ResetServiceAccountAPIKeyHandler handles POST /api/v1/groups/{id}/service-accounts/{uid}/reset-api-key.
// Generates a new API key for an SMTP service account in the group.
// Requires owner or admin role.
func ResetServiceAccountAPIKeyHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		gn, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		groupID := int32(gn)

		uidStr := chi.URLParam(r, "uid")
		un, err := strconv.ParseInt(uidStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}
		userID := int32(un)

		if _, err := requireGroupRole(queries, r, groupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		// Verify user belongs to this group
		_, err = queries.GetGroupMemberByUserAndGroup(r.Context(), storage.GetGroupMemberByUserAndGroupParams{
			UserID:  userID,
			GroupID: groupID,
		})
		if err != nil {
			respondError(w, http.StatusNotFound, "user is not a member of this group")
			return
		}

		// Verify user is an SMTP service account
		user, err := queries.GetUserByID(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		if user.AccountType != "smtp" {
			respondError(w, http.StatusBadRequest, "user is not a service account")
			return
		}

		var resetReq resetAPIKeyRequest
		// Body is optional for this endpoint
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

		updated, err := queries.ResetUserAPIKey(r.Context(), storage.ResetUserAPIKeyParams{
			ID:              userID,
			ApiKey:          sql.NullString{String: newKey, Valid: true},
			ApiKeyExpiresAt: apiKeyExpiresAt,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to reset API key")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.reset_api_key", "user", fmt.Sprintf("%d", userID), map[string]interface{}{
				"group_id": fmt.Sprintf("%d", groupID),
			})
		}

		respondJSON(w, http.StatusOK, toUserResponseWithAPIKey(updated))
	}
}
