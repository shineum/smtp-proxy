package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
)

// dlqReprocessRequest is the JSON body for POST /api/v1/dlq/reprocess.
type dlqReprocessRequest struct {
	MessageIDs     []string `json:"message_ids"`
	ResetRetryCount bool    `json:"reset_retry_count"`
}

// dlqReprocessResponse is the JSON response for a DLQ reprocess operation.
type dlqReprocessResponse struct {
	Reprocessed int `json:"reprocessed"`
	Total       int `json:"total"`
}

// DLQReprocessHandler handles POST /api/v1/dlq/reprocess.
// It re-enqueues messages from the dead letter queue back to the primary queue.
func DLQReprocessHandler(dlq queue.DeadLetterQueue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		accountID := auth.AccountFromContext(r.Context())
		if accountID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req dlqReprocessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if len(req.MessageIDs) == 0 {
			respondError(w, http.StatusBadRequest, "message_ids is required and must not be empty")
			return
		}

		// Use the account ID as tenant ID for DLQ lookup.
		tenantID := accountID.String()

		reprocessed, err := dlq.Reprocess(r.Context(), tenantID, req.MessageIDs)
		if err != nil {
			log.Error().Err(err).
				Str("tenant_id", tenantID).
				Int("requested", len(req.MessageIDs)).
				Int("reprocessed", reprocessed).
				Msg("dlq reprocess failed")
			respondError(w, http.StatusInternalServerError, "reprocess failed")
			return
		}

		log.Info().
			Str("tenant_id", tenantID).
			Int("reprocessed", reprocessed).
			Int("total", len(req.MessageIDs)).
			Msg("dlq reprocess completed")

		respondJSON(w, http.StatusOK, dlqReprocessResponse{
			Reprocessed: reprocessed,
			Total:       len(req.MessageIDs),
		})
	}
}
