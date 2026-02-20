package delivery

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// SyncService delivers messages synchronously via ESP providers.
// It resolves the provider for the account using ProviderResolver.
type SyncService struct {
	resolver *provider.ProviderResolver
	queries  storage.Querier
	log      zerolog.Logger
}

// NewSyncService creates a SyncService that delivers messages inline.
func NewSyncService(
	resolver *provider.ProviderResolver,
	queries storage.Querier,
	log zerolog.Logger,
) *SyncService {
	return &SyncService{
		resolver: resolver,
		queries:  queries,
		log:      log,
	}
}

// DeliverMessage resolves the ESP provider for the account, sends the message,
// and updates the message status and delivery log in the database.
func (s *SyncService) DeliverMessage(ctx context.Context, req *Request) error {
	// Resolve which provider to use for this account.
	p, err := s.resolver.Resolve(ctx, req.AccountID)
	if err != nil {
		s.log.Error().Err(err).
			Stringer("account_id", req.AccountID).
			Stringer("message_id", req.MessageID).
			Msg("failed to resolve provider")
		s.updateStatus(ctx, req.MessageID, storage.MessageStatusFailed, "", nil, err)
		return fmt.Errorf("resolve provider: %w", err)
	}

	providerName := p.GetName()

	// Build provider message.
	msg := &provider.Message{
		ID:       req.MessageID.String(),
		TenantID: req.TenantID,
		From:     req.Sender,
		To:       req.Recipients,
		Subject:  req.Subject,
		Headers:  req.Headers,
		Body:     req.Body,
	}

	// Send via ESP provider.
	result, sendErr := p.Send(ctx, msg)
	if sendErr != nil {
		s.log.Error().Err(sendErr).
			Str("provider", providerName).
			Stringer("message_id", req.MessageID).
			Msg("provider send failed")
		s.updateStatus(ctx, req.MessageID, storage.MessageStatusFailed, providerName, nil, sendErr)
		return fmt.Errorf("provider send: %w", sendErr)
	}

	s.log.Info().
		Str("provider", providerName).
		Stringer("message_id", req.MessageID).
		Str("provider_message_id", result.ProviderMessageID).
		Msg("message delivered")

	s.updateStatus(ctx, req.MessageID, storage.MessageStatusDelivered, providerName, result, nil)
	return nil
}

// updateStatus updates the message status in the database and creates a delivery log entry.
func (s *SyncService) updateStatus(
	ctx context.Context,
	messageID uuid.UUID,
	status storage.MessageStatus,
	providerName string,
	result *provider.DeliveryResult,
	deliveryErr error,
) {
	// Update message status.
	if err := s.queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
		ID:     messageID,
		Status: status,
	}); err != nil {
		s.log.Error().Err(err).Stringer("message_id", messageID).Msg("failed to update message status")
	}

	// Create delivery log entry.
	logParams := storage.CreateDeliveryLogParams{
		MessageID:  messageID,
		ProviderID: uuid.Nil,
		TenantID:   sql.NullString{String: "", Valid: false},
		Status:     string(status),
		Provider:   sql.NullString{String: providerName, Valid: providerName != ""},
	}

	if result != nil {
		logParams.ProviderMessageID = sql.NullString{String: result.ProviderMessageID, Valid: result.ProviderMessageID != ""}
	}
	if deliveryErr != nil {
		logParams.LastError = pgtype.Text{String: deliveryErr.Error(), Valid: true}
	}

	if _, err := s.queries.CreateDeliveryLog(ctx, logParams); err != nil {
		s.log.Error().Err(err).Stringer("message_id", messageID).Msg("failed to create delivery log")
	}
}
