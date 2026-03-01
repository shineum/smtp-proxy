package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// messageResponse is the JSON response for a message.
type messageResponse struct {
	ID          uuid.UUID   `json:"id"`
	GroupID     *uuid.UUID  `json:"group_id,omitempty"`
	UserID      *uuid.UUID  `json:"user_id,omitempty"`
	Sender      string      `json:"sender"`
	Recipients  []string    `json:"recipients"`
	Subject     string      `json:"subject"`
	Status      string      `json:"status"`
	EnqueuedAt  *time.Time  `json:"enqueued_at,omitempty"`
	ProcessedAt *time.Time  `json:"processed_at,omitempty"`
}

func toMessageResponse(m storage.Message) messageResponse {
	resp := messageResponse{
		ID:     m.ID,
		Sender: m.Sender,
		Status: string(m.Status),
	}

	if m.GroupID.Valid {
		gid := uuid.UUID(m.GroupID.Bytes)
		resp.GroupID = &gid
	}
	if m.UserID.Valid {
		uid := uuid.UUID(m.UserID.Bytes)
		resp.UserID = &uid
	}
	if m.Subject.Valid {
		resp.Subject = m.Subject.String
	}
	if m.EnqueuedAt.Valid {
		t := m.EnqueuedAt.Time
		resp.EnqueuedAt = &t
	}
	if m.ProcessedAt.Valid {
		t := m.ProcessedAt.Time
		resp.ProcessedAt = &t
	}

	// Decode recipients from JSON
	if len(m.Recipients) > 0 {
		var recipients []string
		if err := json.Unmarshal(m.Recipients, &recipients); err == nil {
			resp.Recipients = recipients
		}
	}

	return resp
}

// paginatedMessagesResponse wraps a paginated list of messages.
type paginatedMessagesResponse struct {
	Data       []messageResponse `json:"data"`
	Total      int32             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// ListMessagesHandler handles GET /api/v1/messages.
// Returns a paginated list of messages for the current group.
// Supports query params: status, page, page_size
func ListMessagesHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// Parse pagination params
		page := 1
		pageSize := 20
		if v := r.URL.Query().Get("page"); v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				page = p
			}
		}
		if v := r.URL.Query().Get("page_size"); v != "" {
			if ps, err := strconv.Atoi(v); err == nil && ps > 0 && ps <= 100 {
				pageSize = ps
			}
		}

		offset := int32((page - 1) * pageSize)
		limit := int32(pageSize)
		pgGroupID := pgtype.UUID{Bytes: groupID, Valid: true}

		statusFilter := r.URL.Query().Get("status")

		var messages []storage.Message
		var total int32
		var err error

		if statusFilter != "" {
			messages, err = queries.ListMessagesByGroupAndStatusPaginated(r.Context(), storage.ListMessagesByGroupAndStatusPaginatedParams{
				GroupID: pgGroupID,
				Status:  storage.MessageStatus(statusFilter),
				Limit:   limit,
				Offset:  offset,
			})
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			total, err = queries.CountMessagesByGroupAndStatus(r.Context(), storage.CountMessagesByGroupAndStatusParams{
				GroupID: pgGroupID,
				Status:  storage.MessageStatus(statusFilter),
			})
		} else {
			messages, err = queries.ListMessagesByGroupPaginated(r.Context(), storage.ListMessagesByGroupPaginatedParams{
				GroupID: pgGroupID,
				Limit:   limit,
				Offset:  offset,
			})
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			total, err = queries.CountMessagesByGroup(r.Context(), pgGroupID)
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		data := make([]messageResponse, len(messages))
		for i, m := range messages {
			data[i] = toMessageResponse(m)
		}

		totalPages := int(total) / pageSize
		if int(total)%pageSize != 0 {
			totalPages++
		}

		respondJSON(w, http.StatusOK, paginatedMessagesResponse{
			Data:       data,
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		})
	}
}

// deliveryLogResponse is the JSON response for a delivery log entry.
type deliveryLogResponse struct {
	ID                uuid.UUID  `json:"id"`
	MessageID         uuid.UUID  `json:"message_id"`
	ProviderID        *uuid.UUID `json:"provider_id,omitempty"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider,omitempty"`
	ProviderMessageID string     `json:"provider_message_id,omitempty"`
	ResponseCode      *int32     `json:"response_code,omitempty"`
	RetryCount        int32      `json:"retry_count"`
	LastError         string     `json:"last_error,omitempty"`
	DurationMs        *int32     `json:"duration_ms,omitempty"`
	AttemptNumber     int32      `json:"attempt_number"`
	CreatedAt         *time.Time `json:"created_at,omitempty"`
	DeliveredAt       *time.Time `json:"delivered_at,omitempty"`
}

func toDeliveryLogResponse(dl storage.DeliveryLog) deliveryLogResponse {
	resp := deliveryLogResponse{
		ID:            dl.ID,
		MessageID:     dl.MessageID,
		Status:        dl.Status,
		RetryCount:    dl.RetryCount,
		AttemptNumber: dl.AttemptNumber,
	}
	if dl.ProviderID.Valid {
		pid := uuid.UUID(dl.ProviderID.Bytes)
		resp.ProviderID = &pid
	}
	if dl.Provider.Valid {
		resp.Provider = dl.Provider.String
	}
	if dl.ProviderMessageID.Valid {
		resp.ProviderMessageID = dl.ProviderMessageID.String
	}
	if dl.ResponseCode.Valid {
		resp.ResponseCode = &dl.ResponseCode.Int32
	}
	if dl.LastError.Valid {
		resp.LastError = dl.LastError.String
	}
	if dl.DurationMs.Valid {
		resp.DurationMs = &dl.DurationMs.Int32
	}
	if dl.CreatedAt.Valid {
		t := dl.CreatedAt.Time
		resp.CreatedAt = &t
	}
	if dl.DeliveredAt.Valid {
		t := dl.DeliveredAt.Time
		resp.DeliveredAt = &t
	}
	return resp
}

// messageDetailResponse includes the message and its delivery logs.
type messageDetailResponse struct {
	Message      messageResponse       `json:"message"`
	DeliveryLogs []deliveryLogResponse `json:"delivery_logs"`
}

// GetMessageHandler handles GET /api/v1/messages/{id}.
// Returns message detail with delivery logs.
func GetMessageHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid message ID format")
			return
		}

		msg, err := queries.GetMessageByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "message not found")
			return
		}

		// Verify group access
		groupID := auth.GroupIDFromContext(r.Context())
		groupType := auth.GroupTypeFromContext(r.Context())
		if groupType != "system" && msg.GroupID.Valid {
			msgGroupID := uuid.UUID(msg.GroupID.Bytes)
			if msgGroupID != groupID {
				respondError(w, http.StatusForbidden, "access denied")
				return
			}
		}

		logs, err := queries.ListDeliveryLogsByMessageID(r.Context(), id)
		if err != nil {
			logs = nil // non-fatal
		}

		deliveryLogs := make([]deliveryLogResponse, len(logs))
		for i, dl := range logs {
			deliveryLogs[i] = toDeliveryLogResponse(dl)
		}

		respondJSON(w, http.StatusOK, messageDetailResponse{
			Message:      toMessageResponse(msg),
			DeliveryLogs: deliveryLogs,
		})
	}
}
