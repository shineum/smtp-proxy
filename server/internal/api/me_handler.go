package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// membershipResponse represents a group membership with group info.
type membershipResponse struct {
	GroupID   int32     `json:"group_id"`
	GroupName string    `json:"group_name"`
	GroupType string    `json:"group_type"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// meResponse is the response for GET /api/v1/auth/me.
type meResponse struct {
	User         userResponse         `json:"user"`
	CurrentGroup currentGroupResponse `json:"current_group"`
	Memberships  []membershipResponse `json:"memberships"`
}

type currentGroupResponse struct {
	GroupID   int32  `json:"group_id"`
	GroupType string `json:"group_type"`
	Role      string `json:"role"`
}

// MeHandler handles GET /api/v1/auth/me.
// Returns the current user's profile and group memberships.
func MeHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.UserFromContext(r.Context())
		if userID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := queries.GetUserByID(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		memberships, err := queries.ListMembershipsByUserID(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		membershipList := make([]membershipResponse, len(memberships))
		for i, m := range memberships {
			membershipList[i] = membershipResponse{
				GroupID:   m.GroupID,
				GroupName: m.GroupName,
				GroupType: m.GroupType,
				Role:      m.Role,
				CreatedAt: timestampToTime(m.CreatedAt),
			}
		}

		respondJSON(w, http.StatusOK, meResponse{
			User: toUserResponse(user),
			CurrentGroup: currentGroupResponse{
				GroupID:   auth.GroupIDFromContext(r.Context()),
				GroupType: auth.GroupTypeFromContext(r.Context()),
				Role:      auth.RoleFromContext(r.Context()),
			},
			Memberships: membershipList,
		})
	}
}

// changePasswordRequest is the JSON body for PATCH /api/v1/users/me/password.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePasswordHandler handles PATCH /api/v1/users/me/password.
// Allows authenticated users to change their own password.
func ChangePasswordHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.UserFromContext(r.Context())
		if userID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.CurrentPassword == "" || req.NewPassword == "" {
			respondError(w, http.StatusBadRequest, "current_password and new_password are required")
			return
		}

		if len(req.NewPassword) < 8 {
			respondError(w, http.StatusBadRequest, "new password must be at least 8 characters")
			return
		}

		user, err := queries.GetUserByID(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if user.PasswordDisabled {
			respondError(w, http.StatusForbidden, "password management is disabled for this account")
			return
		}

		if err := auth.VerifyPassword(user.PasswordHash, req.CurrentPassword); err != nil {
			respondError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}

		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := queries.UpdateUserPassword(r.Context(), storage.UpdateUserPasswordParams{
			ID:           userID,
			PasswordHash: hash,
		}); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "password changed successfully"})
	}
}

// resetPasswordRequest is the JSON body for POST /api/v1/users/{id}/reset-password.
type resetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// ResetPasswordHandler handles POST /api/v1/users/{id}/reset-password.
// Allows admins to reset another user's password.
func ResetPasswordHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		targetUserID64, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}
		targetUserID := int32(targetUserID64)

		var req resetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.NewPassword == "" {
			respondError(w, http.StatusBadRequest, "new_password is required")
			return
		}

		if len(req.NewPassword) < 8 {
			respondError(w, http.StatusBadRequest, "new password must be at least 8 characters")
			return
		}

		// Verify target user exists
		targetUser, err := queries.GetUserByID(r.Context(), targetUserID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		if targetUser.PasswordDisabled {
			respondError(w, http.StatusForbidden, "password management is disabled for this account")
			return
		}

		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := queries.UpdateUserPassword(r.Context(), storage.UpdateUserPasswordParams{
			ID:           targetUserID,
			PasswordHash: hash,
		}); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.reset_password", "user", fmt.Sprintf("%d", targetUserID), nil)
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
	}
}

// updateGroupRequest is the JSON body for PUT /api/v1/groups/{id}.
type updateGroupRequest struct {
	Name         string `json:"name"`
	MonthlyLimit int32  `json:"monthly_limit"`
}

// UpdateGroupHandler handles PUT /api/v1/groups/{id}.
// Updates a group's name and monthly limit. Requires owner/admin or system admin.
func UpdateGroupHandler(queries storage.Querier, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		id := int32(id64)

		// Verify access: owner/admin in the group or system admin
		if _, err := requireGroupRole(queries, r, id, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		var req updateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		group, err := queries.GetGroupByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "group not found")
			return
		}

		name := group.Name
		if req.Name != "" {
			name = req.Name
		}
		monthlyLimit := group.MonthlyLimit
		if req.MonthlyLimit > 0 {
			monthlyLimit = req.MonthlyLimit
		}

		updated, err := queries.UpdateGroup(r.Context(), storage.UpdateGroupParams{
			ID:           id,
			Name:         name,
			Status:       group.Status,
			MonthlyLimit: monthlyLimit,
			DisplayName:  group.DisplayName,
			Description:  group.Description,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAdminAction(r.Context(), r, "admin.update_group", "group", fmt.Sprintf("%d", id), map[string]interface{}{
				"name":          name,
				"monthly_limit": monthlyLimit,
			})
		}

		respondJSON(w, http.StatusOK, toGroupResponse(updated))
	}
}
