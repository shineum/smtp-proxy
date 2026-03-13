package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// activityLogResponse is the JSON response for an activity log entry.
type activityLogResponse struct {
	ID           uuid.UUID        `json:"id"`
	GroupID      uuid.UUID        `json:"group_id"`
	ActorID      *uuid.UUID       `json:"actor_id,omitempty"`
	Action       string           `json:"action"`
	ResourceType string           `json:"resource_type"`
	ResourceID   *uuid.UUID       `json:"resource_id,omitempty"`
	Changes      json.RawMessage  `json:"changes,omitempty"`
	Comment      *string          `json:"comment,omitempty"`
	IPAddress    *string          `json:"ip_address,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
}

// toActivityLogResponse converts a storage.ActivityLog to an activityLogResponse.
func toActivityLogResponse(al storage.ActivityLog) activityLogResponse {
	resp := activityLogResponse{
		ID:           al.ID,
		GroupID:      al.GroupID,
		Action:       al.Action,
		ResourceType: al.ResourceType,
		CreatedAt:    timestampToTime(al.CreatedAt),
	}

	if al.ActorID.Valid {
		id := uuid.UUID(al.ActorID.Bytes)
		resp.ActorID = &id
	}

	if al.ResourceID.Valid {
		id := uuid.UUID(al.ResourceID.Bytes)
		resp.ResourceID = &id
	}

	if len(al.Changes) > 0 {
		resp.Changes = json.RawMessage(al.Changes)
	}

	if al.Comment.Valid {
		resp.Comment = &al.Comment.String
	}

	if al.IpAddress != nil {
		s := al.IpAddress.String()
		resp.IPAddress = &s
	}

	return resp
}

// ListActivityLogsHandler handles GET /api/v1/groups/{id}/activity.
// Lists activity logs for a group with pagination.
// Supports query params: limit (default 50, max 100), offset (default 0).
// Requires group admin+ role or system admin access.
func ListActivityLogsHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		groupID, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}

		// Verify access
		callerGroupID := auth.GroupIDFromContext(r.Context())
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if callerGroupType != "system" && callerGroupID != groupID {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		// Parse pagination params
		limit := int32(50)
		offset := int32(0)

		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 {
				limit = int32(v)
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if v, err := strconv.Atoi(o); err == nil && v >= 0 {
				offset = int32(v)
			}
		}

		// Cap limit at 100
		if limit > 100 {
			limit = 100
		}

		logs, err := queries.ListActivityLogsByGroupID(r.Context(), storage.ListActivityLogsByGroupIDParams{
			GroupID: groupID,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		resp := make([]activityLogResponse, len(logs))
		for i, l := range logs {
			resp[i] = toActivityLogResponse(l)
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

