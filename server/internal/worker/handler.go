package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/auth"
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

// providerResolver resolves the ESP provider for message delivery.
type providerResolver interface {
	Resolve(ctx context.Context, groupID uuid.UUID) (provider.Provider, error)
	ResolveByUserID(ctx context.Context, userID uuid.UUID) (provider.Provider, error)
	ResolveWithFallbacks(ctx context.Context, userID uuid.UUID) ([]provider.Provider, error)
}

// Handler implements queue.MessageHandler. It delivers messages via ESP
// providers and records delivery results in the database.
type Handler struct {
	resolver           providerResolver
	queries            storage.Querier
	store              msgstore.MessageStore
	domainRateLimiter  *auth.DomainRateLimiter
	log                zerolog.Logger
}

// NewHandler creates a Handler that delivers queue messages via ESP providers.
// The store parameter may be nil for backward compatibility with inline-body
// queue messages. The domainRateLimiter may be nil to disable domain throttling.
func NewHandler(
	resolver providerResolver,
	queries storage.Querier,
	store msgstore.MessageStore,
	domainRateLimiter *auth.DomainRateLimiter,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		resolver:          resolver,
		queries:           queries,
		store:             store,
		domainRateLimiter: domainRateLimiter,
		log:               log,
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

	// Extract IDs for provider resolution and logging.
	groupID := uuid.UUID(dbMsg.GroupID.Bytes)
	userID := uuid.UUID(dbMsg.UserID.Bytes)

	// Determine message body source.
	var body []byte
	if msg.HasInlineBody() {
		// Backward compatibility: old-format queue message with inline body.
		body = msg.Body
		h.log.Debug().Str("message_id", msg.ID).Msg("using inline body from queue (legacy format)")
	} else {
		// New format: fetch from MessageStore using the storage reference.
		storageKey := msg.ID
		if dbMsg.StorageRef.Valid && dbMsg.StorageRef.String != "" {
			storageKey = dbMsg.StorageRef.String
		}
		body, err = h.fetchBodyWithRetry(ctx, storageKey)
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

	// Resolve providers: prefer user's direct provider with fallbacks, fall back to group-level.
	var providers []provider.Provider
	if dbMsg.UserID.Valid {
		providers, err = h.resolver.ResolveWithFallbacks(ctx, userID)
	} else {
		var p provider.Provider
		p, err = h.resolver.Resolve(ctx, groupID)
		if err == nil {
			providers = []provider.Provider{p}
		}
	}
	if err != nil {
		h.log.Error().Err(err).
			Stringer("group_id", groupID).
			Stringer("user_id", userID).
			Str("message_id", msg.ID).
			Msg("failed to resolve provider")
		h.recordFailure(ctx, messageID, dbMsg.GroupID, dbMsg.UserID, "", err)
		return fmt.Errorf("resolve provider: %w", err)
	}

	p := providers[0]
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

	// Parse MIME structure to extract HTML body, attachments, CC, and BCC.
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

		// Extract To and CC from MIME headers; derive BCC from envelope recipients.
		headerTo := extractAddresses(parsed.Headers.Get("To"))
		headerCC := extractAddresses(parsed.Headers.Get("Cc"))
		if len(headerTo) > 0 {
			providerMsg.To = headerTo
		}
		providerMsg.CC = headerCC

		// BCC = envelope recipients - (To + CC)
		knownAddrs := make(map[string]struct{}, len(headerTo)+len(headerCC))
		for _, a := range headerTo {
			knownAddrs[a] = struct{}{}
		}
		for _, a := range headerCC {
			knownAddrs[a] = struct{}{}
		}
		allRecipients := parseRecipients(dbMsg.Recipients)
		for _, r := range allRecipients {
			if _, known := knownAddrs[r]; !known {
				providerMsg.BCC = append(providerMsg.BCC, r)
			}
		}
	} else {
		// MIME parse failed -- fall back to raw body as text.
		providerMsg.TextBody = string(body)
		h.log.Debug().Err(parseErr).Str("message_id", msg.ID).Msg("MIME parse failed, using raw body as text")
	}

	// Check destination domain rate limits before sending.
	if h.domainRateLimiter != nil && dbMsg.GroupID.Valid {
		domains := uniqueRecipientDomains(providerMsg.To, providerMsg.CC, providerMsg.BCC)
		for _, domain := range domains {
			limit, limErr := h.queries.GetDomainRateLimit(ctx, storage.GetDomainRateLimitParams{
				GroupID: groupID,
				Domain:  domain,
			})
			if limErr != nil {
				// No limit configured for this domain -- skip.
				continue
			}
			if !limit.Enabled {
				continue
			}
			result := h.domainRateLimiter.CheckDomainRateLimit(ctx, groupID, domain, limit.MaxPerMinute, limit.MaxPerHour)
			if !result.Allowed {
				h.log.Info().
					Str("message_id", msg.ID).
					Str("domain", domain).
					Str("reason", result.Reason).
					Msg("domain rate limit hit, deferring delivery")
				return &queue.RateLimitedError{
					RetryAfter: result.RetryAfter,
					Reason:     result.Reason,
				}
			}
		}
	}

	// Send via ESP provider with failover on transient errors.
	var lastSendErr error
	for i, p := range providers {
		providerName = p.GetName()

		// REQ-MIME-006: pre-flight check that the rendered message fits the
		// provider's documented size cap. This avoids a wasted round-trip and
		// gives a clearer error than the ESP's generic "request too large".
		// If a fallback provider has a larger limit we'll try it; otherwise
		// the error is permanent for the message.
		if limit := provider.MaxMessageBytes(providerName); limit > 0 {
			size := provider.EstimateMessageBytes(providerMsg)
			if size > limit {
				sizeErr := &provider.ProviderError{
					Provider:  providerName,
					Message:   fmt.Sprintf("message size %d bytes exceeds %s limit of %d bytes", size, providerName, limit),
					Permanent: true,
				}
				lastSendErr = sizeErr
				h.log.Warn().
					Str("provider", providerName).
					Str("message_id", msg.ID).
					Int64("size_bytes", size).
					Int64("limit_bytes", limit).
					Msg("message exceeds provider size limit, skipping")
				if i == len(providers)-1 {
					h.recordFailure(ctx, messageID, dbMsg.GroupID, dbMsg.UserID, providerName, sizeErr)
					return fmt.Errorf("provider send: %w", sizeErr)
				}
				// A fallback with a larger limit may still succeed.
				continue
			}
		}

		sendStart := time.Now()
		result, sendErr := p.Send(ctx, providerMsg)
		sendDuration := time.Since(sendStart)

		if sendErr != nil {
			lastSendErr = sendErr
			h.log.Warn().Err(sendErr).
				Str("provider", providerName).
				Str("message_id", msg.ID).
				Int("attempt", i+1).
				Int("total_providers", len(providers)).
				Msg("provider send failed")

			// If permanent error or last provider, stop trying.
			if provider.IsPermanent(sendErr) || i == len(providers)-1 {
				h.recordFailure(ctx, messageID, dbMsg.GroupID, dbMsg.UserID, providerName, sendErr)
				return fmt.Errorf("provider send: %w", sendErr)
			}

			// Transient error with more fallbacks available -- try next.
			h.log.Info().
				Str("failed_provider", providerName).
				Str("next_provider", providers[i+1].GetName()).
				Str("message_id", msg.ID).
				Msg("transient error, trying fallback provider")
			continue
		}

		// Record success.
		h.log.Info().
			Str("provider", providerName).
			Str("message_id", msg.ID).
			Str("provider_message_id", result.ProviderMessageID).
			Int64("duration_ms", sendDuration.Milliseconds()).
			Bool("is_fallback", i > 0).
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
			AttemptNumber:     int32(i + 1),
		}); err != nil {
			h.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to create delivery log")
		}

		// Increment domain send counters for rate limiting.
		if h.domainRateLimiter != nil {
			for _, domain := range uniqueRecipientDomains(providerMsg.To, providerMsg.CC, providerMsg.BCC) {
				if incErr := h.domainRateLimiter.IncrementDomainCount(ctx, groupID, domain); incErr != nil {
					h.log.Warn().Err(incErr).Str("domain", domain).Msg("failed to increment domain rate counter")
				}
			}
		}

		return nil
	}

	// Should not reach here, but handle gracefully.
	h.recordFailure(ctx, messageID, dbMsg.GroupID, dbMsg.UserID, providerName, lastSendErr)
	return fmt.Errorf("all providers failed: %w", lastSendErr)
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

// extractAddresses parses an RFC 5322 address header value and returns
// a slice of plain email addresses. Returns nil on parse failure or empty input.
func extractAddresses(header string) []string {
	if header == "" {
		return nil
	}
	addrs, err := mail.ParseAddressList(header)
	if err != nil {
		return nil
	}
	result := make([]string, len(addrs))
	for i, a := range addrs {
		result[i] = a.Address
	}
	return result
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

// uniqueRecipientDomains extracts unique domain parts from all recipient
// email addresses (To, CC, BCC).
func uniqueRecipientDomains(lists ...[]string) []string {
	seen := make(map[string]struct{})
	for _, list := range lists {
		for _, addr := range list {
			if idx := strings.LastIndex(addr, "@"); idx >= 0 {
				domain := strings.ToLower(addr[idx+1:])
				seen[domain] = struct{}{}
			}
		}
	}
	domains := make([]string, 0, len(seen))
	for d := range seen {
		domains = append(domains, d)
	}
	return domains
}
