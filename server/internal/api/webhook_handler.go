package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// webhookStatusMap maps normalized event types to delivery log status strings.
var webhookStatusMap = map[string]string{
	"delivered":  "sent",
	"bounced":    "bounced",
	"bounce":     "bounced",
	"dropped":    "failed",
	"complained": "complained",
	"complaint":  "complained",
	"failed":     "failed",
}

// SendGridWebhookHandler handles POST /api/v1/webhooks/sendgrid.
// SendGrid sends an array of event objects.
func SendGridWebhookHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		var events []sendGridEvent
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			log.Warn().Err(err).Msg("sendgrid webhook: invalid payload")
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		for _, event := range events {
			status := normalizeSendGridStatus(event.Event)
			if status == "" {
				continue
			}

			msgID, err := lookupMessageIDByProvider(r, queries, event.SGMessageID)
			if err != nil {
				log.Warn().Str("sg_message_id", event.SGMessageID).Msg("sendgrid webhook: delivery log not found")
				continue
			}

			if err := queries.UpdateDeliveryLogStatus(r.Context(), storage.UpdateDeliveryLogStatusParams{
				MessageID:         msgID,
				Status:            status,
				Provider:          sql.NullString{String: "sendgrid", Valid: true},
				ProviderMessageID: sql.NullString{String: event.SGMessageID, Valid: true},
				RetryCount:        0,
				LastError:         pgtype.Text{String: event.Reason, Valid: event.Reason != ""},
				Metadata:          marshalMetadata(map[string]string{"event": event.Event, "email": event.Email}),
			}); err != nil {
				log.Error().Err(err).Str("message_id", msgID.String()).Msg("sendgrid webhook: update delivery log failed")
			}
		}

		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// SESWebhookHandler handles POST /api/v1/webhooks/ses.
// AWS SES sends SNS notification messages containing SES-specific event data.
func SESWebhookHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		var notification sesNotification
		if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
			log.Warn().Err(err).Msg("ses webhook: invalid payload")
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		status := normalizeSESStatus(notification.NotificationType)
		if status == "" {
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}

		providerMsgID := ""
		var lastError string
		switch notification.NotificationType {
		case "Bounce":
			if notification.Bounce != nil {
				providerMsgID = notification.Bounce.FeedbackID
				lastError = notification.Bounce.BounceType + ": " + notification.Bounce.BounceSubType
			}
		case "Complaint":
			if notification.Complaint != nil {
				providerMsgID = notification.Complaint.FeedbackID
			}
		case "Delivery":
			if notification.Delivery != nil {
				providerMsgID = notification.Mail.MessageID
			}
		}

		msgID, err := lookupMessageIDByProvider(r, queries, providerMsgID)
		if err != nil {
			log.Warn().Str("provider_message_id", providerMsgID).Msg("ses webhook: delivery log not found")
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}

		if err := queries.UpdateDeliveryLogStatus(r.Context(), storage.UpdateDeliveryLogStatusParams{
			MessageID:         msgID,
			Status:            status,
			Provider:          sql.NullString{String: "ses", Valid: true},
			ProviderMessageID: sql.NullString{String: providerMsgID, Valid: providerMsgID != ""},
			RetryCount:        0,
			LastError:         pgtype.Text{String: lastError, Valid: lastError != ""},
			Metadata:          marshalMetadata(map[string]string{"notification_type": notification.NotificationType}),
		}); err != nil {
			log.Error().Err(err).Str("message_id", msgID.String()).Msg("ses webhook: update delivery log failed")
		}

		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// MailgunWebhookHandler handles POST /api/v1/webhooks/mailgun.
// Mailgun sends event data wrapped in an "event-data" field.
func MailgunWebhookHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		var payload mailgunWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Warn().Err(err).Msg("mailgun webhook: invalid payload")
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		event := payload.EventData
		status := normalizeMailgunStatus(event.Event)
		if status == "" {
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}

		providerMsgID := event.Message.Headers.MessageID
		msgID, err := lookupMessageIDByProvider(r, queries, providerMsgID)
		if err != nil {
			log.Warn().Str("provider_message_id", providerMsgID).Msg("mailgun webhook: delivery log not found")
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}

		reason := ""
		if event.DeliveryStatus.Message != "" {
			reason = event.DeliveryStatus.Message
		}

		if err := queries.UpdateDeliveryLogStatus(r.Context(), storage.UpdateDeliveryLogStatusParams{
			MessageID:         msgID,
			Status:            status,
			Provider:          sql.NullString{String: "mailgun", Valid: true},
			ProviderMessageID: sql.NullString{String: providerMsgID, Valid: providerMsgID != ""},
			RetryCount:        0,
			LastError:         pgtype.Text{String: reason, Valid: reason != ""},
			Metadata:          marshalMetadata(map[string]string{"event": event.Event, "recipient": event.Recipient}),
		}); err != nil {
			log.Error().Err(err).Str("message_id", msgID.String()).Msg("mailgun webhook: update delivery log failed")
		}

		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// --- SendGrid event types ---

type sendGridEvent struct {
	Email       string `json:"email"`
	Event       string `json:"event"`
	SGMessageID string `json:"sg_message_id"`
	Reason      string `json:"reason"`
}

func normalizeSendGridStatus(event string) string {
	switch event {
	case "delivered":
		return "sent"
	case "bounce", "blocked":
		return "bounced"
	case "dropped":
		return "failed"
	case "spamreport":
		return "complained"
	default:
		return ""
	}
}

// --- SES event types ---

type sesNotification struct {
	NotificationType string        `json:"notificationType"`
	Mail             sesMail       `json:"mail"`
	Bounce           *sesBounce    `json:"bounce,omitempty"`
	Complaint        *sesComplaint `json:"complaint,omitempty"`
	Delivery         *sesDelivery  `json:"delivery,omitempty"`
}

type sesMail struct {
	MessageID string `json:"messageId"`
}

type sesBounce struct {
	BounceType    string `json:"bounceType"`
	BounceSubType string `json:"bounceSubType"`
	FeedbackID    string `json:"feedbackId"`
}

type sesComplaint struct {
	FeedbackID string `json:"feedbackId"`
}

type sesDelivery struct {
	Timestamp string `json:"timestamp"`
}

func normalizeSESStatus(notificationType string) string {
	switch notificationType {
	case "Delivery":
		return "sent"
	case "Bounce":
		return "bounced"
	case "Complaint":
		return "complained"
	default:
		return ""
	}
}

// --- Mailgun event types ---

type mailgunWebhookPayload struct {
	EventData mailgunEventData `json:"event-data"`
}

type mailgunEventData struct {
	Event          string                `json:"event"`
	Recipient      string                `json:"recipient"`
	Message        mailgunMessage        `json:"message"`
	DeliveryStatus mailgunDeliveryStatus `json:"delivery-status"`
}

type mailgunMessage struct {
	Headers mailgunHeaders `json:"headers"`
}

type mailgunHeaders struct {
	MessageID string `json:"message-id"`
}

type mailgunDeliveryStatus struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func normalizeMailgunStatus(event string) string {
	switch event {
	case "delivered":
		return "sent"
	case "failed", "rejected":
		return "failed"
	case "complained":
		return "complained"
	default:
		return ""
	}
}

// --- Helpers ---

// lookupMessageIDByProvider finds the internal message ID from a provider message ID.
func lookupMessageIDByProvider(r *http.Request, queries storage.Querier, providerMessageID string) (uuid.UUID, error) {
	log, err := queries.GetDeliveryLogByProviderMessageID(r.Context(), sql.NullString{
		String: providerMessageID,
		Valid:  providerMessageID != "",
	})
	if err != nil {
		return uuid.Nil, err
	}
	return log.MessageID, nil
}

// marshalMetadata marshals a string map to JSON bytes for storage.
func marshalMetadata(m map[string]string) []byte {
	data, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return data
}
