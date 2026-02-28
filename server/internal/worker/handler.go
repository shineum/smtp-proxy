package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/mimeparse"
	"github.com/sungwon/smtp-proxy/server/internal/msgstore"
	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// storageRetryBackoff defines the backoff durations for MessageStore read
// retries (REQ-QW-002).
var storageRetryBackoff = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
}

// providerResolver resolves the ESP provider for a given group ID.
type providerResolver interface {
	Resolve(ctx context.Context, groupID uuid.UUID) (provider.Provider, error)
}

// Handler implements queue.MessageHandler. It delivers messages via ESP
// providers and records delivery results in the database.
type Handler struct {
	resolver providerResolver
	queries  storage.Querier
	store    msgstore.MessageStore
	log      zerolog.Logger
}

// NewHandler creates a Handler that delivers queue messages via ESP providers.
// The store parameter may be nil for backward compatibility with inline-body
// queue messages.
func NewHandler(
	resolver providerResolver,
	queries storage.Querier,
	store msgstore.MessageStore,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		resolver: resolver,
		queries:  queries,
		store:    store,
		log:      log,
	}
}

// HandleMessage implements queue.MessageHandler. It resolves the provider,
// sends the message, and updates the database.
func (h *Handler) HandleMessage(ctx context.Context, msg *queue.Message) error {
	messageID, err := uuid.Parse(msg.ID)
	if err != nil {
		return fmt.Errorf("parse message ID %q: %w", msg.ID, err)
	}

	// Update message status to processing.
	if err := h.queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
		ID:     messageID,
		Status: storage.MessageStatusProcessing,
	}); err != nil {
		h.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to set processing status")
	}

	// Look up the message in DB to get the group/user IDs and metadata.
	dbMsg, err := h.queries.GetMessageByID(ctx, messageID)
	if err != nil {
		// REQ-QW-005: Orphaned message_id -- acknowledge without delivery.
		if errors.Is(err, pgx.ErrNoRows) {
			h.log.Warn().Str("message_id", msg.ID).Msg("orphaned message_id not found in database, acknowledging")
			return nil
		}
		h.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to get message from database")
		h.recordFailure(ctx, messageID, pgtype.UUID{}, pgtype.UUID{}, "", fmt.Errorf("get message: %w", err))
		return fmt.Errorf("get message %s: %w", msg.ID, err)
	}

	// Extract group ID as uuid.UUID for provider resolution.
	groupID := uuid.UUID(dbMsg.GroupID.Bytes)

	// Determine message body source.
	var body []byte
	if msg.HasInlineBody() {
		// Backward compatibility: old-format queue message with inline body.
		body = msg.Body
		h.log.Debug().Str("message_id", msg.ID).Msg("using inline body from queue (legacy format)")
	} else {
		// New format: fetch from MessageStore with retry (REQ-QW-002).
		body, err = h.fetchBodyWithRetry(ctx, msg.ID)
		if err != nil {
			// All retries exhausted -- mark as storage_error.
			if statusErr := h.queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
				ID:     messageID,
				Status: storage.MessageStatusStorageError,
			}); statusErr != nil {
				h.log.Error().Err(statusErr).Str("message_id", msg.ID).Msg("failed to set storage_error status")
			}
			h.recordFailure(ctx, messageID, dbMsg.GroupID, dbMsg.UserID, "", fmt.Errorf("storage read: %w", err))
			return fmt.Errorf("fetch body for %s: %w", msg.ID, err)
		}
	}

	// Resolve provider for this group.
	p, err := h.resolver.Resolve(ctx, groupID)
	if err != nil {
		h.log.Error().Err(err).
			Stringer("group_id", groupID).
			Str("message_id", msg.ID).
			Msg("failed to resolve provider")
		h.recordFailure(ctx, messageID, dbMsg.GroupID, dbMsg.UserID, "", err)
		return fmt.Errorf("resolve provider: %w", err)
	}

	providerName := p.GetName()

	// Build provider message from DB metadata + body.
	providerMsg := &provider.Message{
		ID:       msg.ID,
		TenantID: groupID.String(),
		From:     dbMsg.Sender,
		To:       parseRecipients(dbMsg.Recipients),
		Subject:  nullStringValue(dbMsg.Subject),
		Headers:  parseHeaders(dbMsg.Headers),
		Body:     body,
	}

	// Parse MIME structure to extract HTML body and attachments.
	parsed, parseErr := mimeparse.Parse(body)
	if parseErr == nil {
		providerMsg.TextBody = parsed.TextBody
		providerMsg.HTMLBody = parsed.HTMLBody
		if parsed.Subject != "" {
			providerMsg.Subject = parsed.Subject
		}
		for _, att := range parsed.Attachments {
			providerMsg.Attachments = append(providerMsg.Attachments, provider.Attachment{
				Filename:    att.Filename,
				ContentType: att.ContentType,
				Content:     att.Content,
				ContentID:   att.ContentID,
				IsInline:    att.IsInline,
			})
		}
	} else {
		// MIME parse failed -- fall back to raw body as text.
		providerMsg.TextBody = string(body)
		h.log.Debug().Err(parseErr).Str("message_id", msg.ID).Msg("MIME parse failed, using raw body as text")
	}

	// Send via ESP provider.
	sendStart := time.Now()
	result, sendErr := p.Send(ctx, providerMsg)
	sendDuration := time.Since(sendStart)
	if sendErr != nil {
		h.log.Error().Err(sendErr).
			Str("provider", providerName).
			Str("message_id", msg.ID).
			Msg("provider send failed")
		h.recordFailure(ctx, messageID, dbMsg.GroupID, dbMsg.UserID, providerName, sendErr)
		return fmt.Errorf("provider send: %w", sendErr)
	}

	// Record success.
	h.log.Info().
		Str("provider", providerName).
		Str("message_id", msg.ID).
		Str("provider_message_id", result.ProviderMessageID).
		Int64("duration_ms", sendDuration.Milliseconds()).
		Msg("message delivered by worker")

	if err := h.queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
		ID:     messageID,
		Status: storage.MessageStatusDelivered,
	}); err != nil {
		h.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to update delivered status")
	}

	if _, err := h.queries.CreateDeliveryLog(ctx, storage.CreateDeliveryLogParams{
		MessageID:         messageID,
		ProviderID:        pgtype.UUID{},
		Status:            string(storage.MessageStatusDelivered),
		Provider:          sql.NullString{String: providerName, Valid: true},
		ProviderMessageID: sql.NullString{String: result.ProviderMessageID, Valid: result.ProviderMessageID != ""},
		GroupID:           dbMsg.GroupID,
		UserID:            dbMsg.UserID,
		DurationMs:        pgtype.Int4{Int32: int32(sendDuration.Milliseconds()), Valid: true},
		AttemptNumber:     1,
	}); err != nil {
		h.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to create delivery log")
	}

	return nil
}

// fetchBodyWithRetry retrieves the message body from the MessageStore with
// exponential backoff retries (REQ-QW-002).
func (h *Handler) fetchBodyWithRetry(ctx context.Context, messageID string) ([]byte, error) {
	var lastErr error

	for attempt, delay := range storageRetryBackoff {
		data, err := h.store.Get(ctx, messageID)
		if err == nil {
			return data, nil
		}
		lastErr = err
		h.log.Warn().Err(err).
			Str("message_id", messageID).
			Int("attempt", attempt+1).
			Int("max_attempts", len(storageRetryBackoff)).
			Msg("storage read failed, retrying")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	h.log.Error().Err(lastErr).
		Str("message_id", messageID).
		Msg("storage read failed after all retries")
	return nil, fmt.Errorf("all %d retries exhausted: %w", len(storageRetryBackoff), lastErr)
}

// recordFailure updates the message status to failed and creates a delivery log.
func (h *Handler) recordFailure(ctx context.Context, messageID uuid.UUID, groupID pgtype.UUID, userID pgtype.UUID, providerName string, deliveryErr error) {
	if err := h.queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
		ID:     messageID,
		Status: storage.MessageStatusFailed,
	}); err != nil {
		h.log.Error().Err(err).Stringer("message_id", messageID).Msg("failed to update failed status")
	}

	if _, err := h.queries.CreateDeliveryLog(ctx, storage.CreateDeliveryLogParams{
		MessageID:  messageID,
		ProviderID: pgtype.UUID{},
		Status:     string(storage.MessageStatusFailed),
		Provider:   sql.NullString{String: providerName, Valid: providerName != ""},
		LastError:  pgtype.Text{String: deliveryErr.Error(), Valid: true},
		GroupID:    groupID,
		UserID:     userID,
	}); err != nil {
		h.log.Error().Err(err).Stringer("message_id", messageID).Msg("failed to create failure delivery log")
	}
}

// parseRecipients decodes a JSON-encoded []string from the database recipients
// column. Returns nil on decode failure.
func parseRecipients(data []byte) []string {
	var recipients []string
	_ = json.Unmarshal(data, &recipients)
	return recipients
}

// parseHeaders decodes a JSON-encoded map[string][]string from the database
// headers column and flattens it to map[string]string by taking the first
// value of each key.
func parseHeaders(data []byte) map[string]string {
	if len(data) == 0 {
		return nil
	}
	var multi map[string][]string
	if err := json.Unmarshal(data, &multi); err != nil {
		return nil
	}
	flat := make(map[string]string, len(multi))
	for k, v := range multi {
		if len(v) > 0 {
			flat[k] = v[0]
		}
	}
	return flat
}

// nullStringValue extracts the string from a sql.NullString, returning ""
// when the value is not valid.
func nullStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
