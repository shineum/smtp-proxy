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

// createGroupRequest is the JSON body for POST /api/v1/groups.
type createGroupRequest struct {
	Name         string `json:"name"`
	MonthlyLimit int32  `json:"monthly_limit,omitempty"`
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
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	GroupType    string    `json:"group_type"`
	Status       string    `json:"status"`
	MonthlyLimit int32     `json:"monthly_limit"`
	MonthlySent  int32     `json:"monthly_sent"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// groupMemberResponse is the JSON response for a group member.
type groupMemberResponse struct {
	ID        uuid.UUID `json:"id"`
	GroupID   uuid.UUID `json:"group_id"`
	UserID    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// toGroupResponse converts a storage.Group to a groupResponse.
func toGroupResponse(g storage.Group) groupResponse {
	return groupResponse{
		ID:           g.ID,
		Name:         g.Name,
		GroupType:    g.GroupType,
		Status:       g.Status,
		MonthlyLimit: g.MonthlyLimit,
		MonthlySent:  g.MonthlySent,
		CreatedAt:    timestampToTime(g.CreatedAt),
		UpdatedAt:    timestampToTime(g.UpdatedAt),
	}
}

// toGroupMemberResponse converts a storage.GroupMember to a groupMemberResponse.
func toGroupMemberResponse(gm storage.GroupMember) groupMemberResponse {
	return groupMemberResponse{
		ID:        gm.ID,
		GroupID:   gm.GroupID,
		UserID:    gm.UserID,
		Role:      gm.Role,
		CreatedAt: timestampToTime(gm.CreatedAt),
	}
}

// CreateGroupHandler handles POST /api/v1/groups.
// Creates a new group with group_type='company' and status='active'.
// Requires system admin access (group_type == "system").
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
			Name:      req.Name,
			GroupType: "company",
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
			})
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionCreateGroup, "group", group.ID.String(), map[string]interface{}{
				"name": req.Name,
			})
		}

		respondJSON(w, http.StatusCreated, toGroupResponse(group))
	}
}

// ListGroupsHandler handles GET /api/v1/groups.
// Lists all groups. Requires system admin access.
func ListGroupsHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups, err := queries.ListGroups(r.Context())
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
// Returns group details. Requires group admin+ role.
func GetGroupHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}

		// Verify the requesting user has access to this group
		callerGroupID := auth.GroupIDFromContext(r.Context())
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if callerGroupType != "system" && callerGroupID != id {
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
// Requires system admin access.
func DeleteGroupHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}

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
			auditLogger.LogAdminAction(r.Context(), r, auth.AuditActionDeleteGroup, "group", id.String(), nil)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ListGroupMembersHandler handles GET /api/v1/groups/{id}/members.
// Lists all members of a group. Requires group admin+ role.
func ListGroupMembersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}

		// Verify access
		callerGroupID := auth.GroupIDFromContext(r.Context())
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if callerGroupType != "system" && callerGroupID != id {
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
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// AddGroupMemberHandler handles POST /api/v1/groups/{id}/members.
// Adds a member to a group. Returns 409 if SMTP account is already in another group.
func AddGroupMemberHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		groupID, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}

		var req addMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user_id format")
			return
		}

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
			auditLogger.LogAdminAction(r.Context(), r, "admin.add_member", "group_member", member.ID.String(), map[string]interface{}{
				"group_id": groupID.String(),
				"user_id":  userID.String(),
				"role":     req.Role,
			})
		}

		respondJSON(w, http.StatusCreated, toGroupMemberResponse(member))
	}
}

// UpdateGroupMemberRoleHandler handles PATCH /api/v1/groups/{id}/members/{uid}.
// Updates a member's role. Returns 409 if last owner.
func UpdateGroupMemberRoleHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupIDStr := chi.URLParam(r, "id")
		groupID, err := uuid.Parse(groupIDStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}

		uidStr := chi.URLParam(r, "uid")
		userID, err := uuid.Parse(uidStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}

		var req updateMemberRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if _, ok := validRoles[req.Role]; !ok {
			respondError(w, http.StatusBadRequest, "role must be one of: owner, admin, member")
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
			ID:   member.ID,
			Role: req.Role,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.update_member_role", "group_member", member.ID.String(), map[string]interface{}{
				"old_role": oldRole,
				"new_role": req.Role,
			})
		}

		respondJSON(w, http.StatusOK, toGroupMemberResponse(updated))
	}
}

// RemoveGroupMemberHandler handles DELETE /api/v1/groups/{id}/members/{uid}.
// Removes a member from a group. Returns 409 if last owner.
func RemoveGroupMemberHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupIDStr := chi.URLParam(r, "id")
		groupID, err := uuid.Parse(groupIDStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}

		uidStr := chi.URLParam(r, "uid")
		userID, err := uuid.Parse(uidStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
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

		if err := queries.DeleteGroupMember(r.Context(), member.ID); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.remove_member", "group_member", member.ID.String(), map[string]interface{}{
				"group_id": groupID.String(),
				"user_id":  userID.String(),
			})
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
