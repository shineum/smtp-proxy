package worker

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	"github.com/sungwon/smtp-proxy/server/internal/routing"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// Handler implements queue.MessageHandler. It delivers messages via ESP
// providers and records delivery results in the database.
type Handler struct {
	registry *provider.Registry
	router   *routing.Engine
	queries  storage.Querier
	log      zerolog.Logger
}

// NewHandler creates a Handler that delivers queue messages via ESP providers.
func NewHandler(
	registry *provider.Registry,
	router *routing.Engine,
	queries storage.Querier,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		registry: registry,
		router:   router,
		queries:  queries,
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

	// Resolve provider for this tenant.
	providerName, err := h.router.ResolveProvider(ctx, msg.TenantID)
	if err != nil {
		h.log.Error().Err(err).
			Str("tenant_id", msg.TenantID).
			Str("message_id", msg.ID).
			Msg("failed to resolve provider")
		h.recordFailure(ctx, messageID, "", err)
		return fmt.Errorf("resolve provider: %w", err)
	}

	p, err := h.registry.Get(providerName)
	if err != nil {
		h.log.Error().Err(err).
			Str("provider", providerName).
			Str("message_id", msg.ID).
			Msg("provider not found in registry")
		h.recordFailure(ctx, messageID, providerName, err)
		return fmt.Errorf("get provider %s: %w", providerName, err)
	}

	// Build provider message.
	providerMsg := &provider.Message{
		ID:       msg.ID,
		TenantID: msg.TenantID,
		From:     msg.From,
		To:       msg.To,
		Subject:  msg.Subject,
		Headers:  msg.Headers,
		Body:     msg.Body,
	}

	// Send via ESP provider.
	result, sendErr := p.Send(ctx, providerMsg)
	if sendErr != nil {
		h.log.Error().Err(sendErr).
			Str("provider", providerName).
			Str("message_id", msg.ID).
			Msg("provider send failed")
		h.recordFailure(ctx, messageID, providerName, sendErr)
		return fmt.Errorf("provider send: %w", sendErr)
	}

	// Record success.
	h.log.Info().
		Str("provider", providerName).
		Str("message_id", msg.ID).
		Str("provider_message_id", result.ProviderMessageID).
		Msg("message delivered by worker")

	if err := h.queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
		ID:     messageID,
		Status: storage.MessageStatusDelivered,
	}); err != nil {
		h.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to update delivered status")
	}

	if _, err := h.queries.CreateDeliveryLog(ctx, storage.CreateDeliveryLogParams{
		MessageID:         messageID,
		ProviderID:        uuid.Nil,
		Status:            string(storage.MessageStatusDelivered),
		Provider:          sql.NullString{String: providerName, Valid: true},
		ProviderMessageID: sql.NullString{String: result.ProviderMessageID, Valid: result.ProviderMessageID != ""},
	}); err != nil {
		h.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to create delivery log")
	}

	return nil
}

// recordFailure updates the message status to failed and creates a delivery log.
func (h *Handler) recordFailure(ctx context.Context, messageID uuid.UUID, providerName string, deliveryErr error) {
	if err := h.queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
		ID:     messageID,
		Status: storage.MessageStatusFailed,
	}); err != nil {
		h.log.Error().Err(err).Stringer("message_id", messageID).Msg("failed to update failed status")
	}

	if _, err := h.queries.CreateDeliveryLog(ctx, storage.CreateDeliveryLogParams{
		MessageID:  messageID,
		ProviderID: uuid.Nil,
		Status:     string(storage.MessageStatusFailed),
		Provider:   sql.NullString{String: providerName, Valid: providerName != ""},
		LastError:  pgtype.Text{String: deliveryErr.Error(), Valid: true},
	}); err != nil {
		h.log.Error().Err(err).Stringer("message_id", messageID).Msg("failed to create failure delivery log")
	}
}
