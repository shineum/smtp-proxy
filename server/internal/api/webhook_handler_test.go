package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// --- SendGrid Webhook Tests ---

func TestSendGridWebhookHandler_Delivered(t *testing.T) {
	msgID := uuid.New()
	var capturedStatus string

	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			if providerMsgID.String != "abc123" {
				t.Errorf("expected provider message ID abc123, got %s", providerMsgID.String)
			}
			return storage.DeliveryLog{MessageID: msgID}, nil
		},
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			capturedStatus = arg.Status
			if arg.MessageID != msgID {
				t.Errorf("expected message ID %s, got %s", msgID, arg.MessageID)
			}
			if arg.Provider.String != "sendgrid" {
				t.Errorf("expected provider sendgrid, got %s", arg.Provider.String)
			}
			return nil
		},
	}

	body := `[{"email":"test@example.com","event":"delivered","sg_message_id":"abc123","reason":""}]`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sendgrid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SendGridWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if capturedStatus != "sent" {
		t.Errorf("expected status 'sent', got %q", capturedStatus)
	}
}

func TestSendGridWebhookHandler_Bounce(t *testing.T) {
	msgID := uuid.New()
	var capturedStatus string

	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			return storage.DeliveryLog{MessageID: msgID}, nil
		},
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			capturedStatus = arg.Status
			return nil
		},
	}

	body := `[{"email":"test@example.com","event":"bounce","sg_message_id":"bounce123","reason":"550 User unknown"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sendgrid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SendGridWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if capturedStatus != "bounced" {
		t.Errorf("expected status 'bounced', got %q", capturedStatus)
	}
}

func TestSendGridWebhookHandler_InvalidJSON(t *testing.T) {
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sendgrid", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SendGridWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid request body" {
		t.Errorf("expected error 'invalid request body', got %q", resp["error"])
	}
}

func TestSendGridWebhookHandler_UnknownEvent(t *testing.T) {
	updateCalled := false
	mock := &mockQuerier{
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			updateCalled = true
			return nil
		},
	}

	body := `[{"email":"test@example.com","event":"open","sg_message_id":"open123","reason":""}]`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sendgrid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SendGridWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for unknown event, got %d", rec.Code)
	}
	if updateCalled {
		t.Error("expected no update call for unknown event type")
	}
}

func TestSendGridWebhookHandler_ProviderMessageIDNotFound(t *testing.T) {
	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			return storage.DeliveryLog{}, errors.New("not found")
		},
	}

	body := `[{"email":"test@example.com","event":"delivered","sg_message_id":"unknown","reason":""}]`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/sendgrid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SendGridWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	// Should still return 200 OK even when message ID is not found
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 when provider message ID not found, got %d", rec.Code)
	}
}

// --- SES Webhook Tests ---

func TestSESWebhookHandler_Delivered(t *testing.T) {
	msgID := uuid.New()
	var capturedStatus string

	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			if providerMsgID.String != "abc123" {
				t.Errorf("expected provider message ID abc123, got %s", providerMsgID.String)
			}
			return storage.DeliveryLog{MessageID: msgID}, nil
		},
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			capturedStatus = arg.Status
			if arg.MessageID != msgID {
				t.Errorf("expected message ID %s, got %s", msgID, arg.MessageID)
			}
			if arg.Provider.String != "ses" {
				t.Errorf("expected provider ses, got %s", arg.Provider.String)
			}
			return nil
		},
	}

	body := `{"notificationType":"Delivery","mail":{"messageId":"abc123"},"delivery":{"timestamp":"2024-01-01T00:00:00Z"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SESWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if capturedStatus != "sent" {
		t.Errorf("expected status 'sent', got %q", capturedStatus)
	}
}

func TestSESWebhookHandler_Bounce(t *testing.T) {
	msgID := uuid.New()
	var capturedStatus string
	var capturedLastError string

	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			// For bounces, the provider message ID is the bounce feedback ID
			if providerMsgID.String != "bounce-123" {
				t.Errorf("expected provider message ID bounce-123, got %s", providerMsgID.String)
			}
			return storage.DeliveryLog{MessageID: msgID}, nil
		},
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			capturedStatus = arg.Status
			capturedLastError = arg.LastError.String
			return nil
		},
	}

	body := `{"notificationType":"Bounce","mail":{"messageId":"abc123"},"bounce":{"bounceType":"Permanent","bounceSubType":"General","feedbackId":"bounce-123"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SESWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if capturedStatus != "bounced" {
		t.Errorf("expected status 'bounced', got %q", capturedStatus)
	}
	expectedError := "Permanent: General"
	if capturedLastError != expectedError {
		t.Errorf("expected last error %q, got %q", expectedError, capturedLastError)
	}
}

func TestSESWebhookHandler_InvalidJSON(t *testing.T) {
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SESWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestSESWebhookHandler_UnknownNotificationType(t *testing.T) {
	updateCalled := false
	mock := &mockQuerier{
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			updateCalled = true
			return nil
		},
	}

	body := `{"notificationType":"Unknown","mail":{"messageId":"abc123"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SESWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for unknown notification type, got %d", rec.Code)
	}
	if updateCalled {
		t.Error("expected no update call for unknown notification type")
	}
}

func TestSESWebhookHandler_ProviderMessageIDNotFound(t *testing.T) {
	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			return storage.DeliveryLog{}, errors.New("not found")
		},
	}

	body := `{"notificationType":"Delivery","mail":{"messageId":"unknown"},"delivery":{"timestamp":"2024-01-01T00:00:00Z"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := SESWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	// Should return 200 OK even when message not found
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

// --- Mailgun Webhook Tests ---

func TestMailgunWebhookHandler_Delivered(t *testing.T) {
	msgID := uuid.New()
	var capturedStatus string

	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			if providerMsgID.String != "abc123" {
				t.Errorf("expected provider message ID abc123, got %s", providerMsgID.String)
			}
			return storage.DeliveryLog{MessageID: msgID}, nil
		},
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			capturedStatus = arg.Status
			if arg.MessageID != msgID {
				t.Errorf("expected message ID %s, got %s", msgID, arg.MessageID)
			}
			if arg.Provider.String != "mailgun" {
				t.Errorf("expected provider mailgun, got %s", arg.Provider.String)
			}
			return nil
		},
	}

	body := `{"event-data":{"event":"delivered","recipient":"test@example.com","message":{"headers":{"message-id":"abc123"}},"delivery-status":{"message":"OK","code":250}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/mailgun", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := MailgunWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if capturedStatus != "sent" {
		t.Errorf("expected status 'sent', got %q", capturedStatus)
	}
}

func TestMailgunWebhookHandler_Failed(t *testing.T) {
	msgID := uuid.New()
	var capturedStatus string

	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			return storage.DeliveryLog{MessageID: msgID}, nil
		},
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			capturedStatus = arg.Status
			return nil
		},
	}

	body := `{"event-data":{"event":"failed","recipient":"test@example.com","message":{"headers":{"message-id":"fail123"}},"delivery-status":{"message":"550 Mailbox not found","code":550}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/mailgun", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := MailgunWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if capturedStatus != "failed" {
		t.Errorf("expected status 'failed', got %q", capturedStatus)
	}
}

func TestMailgunWebhookHandler_InvalidJSON(t *testing.T) {
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/mailgun", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := MailgunWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMailgunWebhookHandler_UnknownEvent(t *testing.T) {
	updateCalled := false
	mock := &mockQuerier{
		updateDeliveryLogStatusFn: func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
			updateCalled = true
			return nil
		},
	}

	body := `{"event-data":{"event":"opened","recipient":"test@example.com","message":{"headers":{"message-id":"open123"}},"delivery-status":{"message":"","code":0}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/mailgun", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := MailgunWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for unknown event, got %d", rec.Code)
	}
	if updateCalled {
		t.Error("expected no update call for unknown event type")
	}
}

func TestMailgunWebhookHandler_ProviderMessageIDNotFound(t *testing.T) {
	mock := &mockQuerier{
		getDeliveryLogByProviderMessageIDFn: func(ctx context.Context, providerMsgID sql.NullString) (storage.DeliveryLog, error) {
			return storage.DeliveryLog{}, errors.New("not found")
		},
	}

	body := `{"event-data":{"event":"delivered","recipient":"test@example.com","message":{"headers":{"message-id":"unknown"}},"delivery-status":{"message":"OK","code":250}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/mailgun", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := MailgunWebhookHandler(mock)
	handler.ServeHTTP(rec, req)

	// Should return 200 OK even when message not found
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

// --- Helper function tests ---

func TestMarshalMetadata(t *testing.T) {
	m := map[string]string{"key": "value"}
	data := marshalMetadata(m)

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected value 'value', got %q", result["key"])
	}
}

func TestNormalizeSendGridStatus(t *testing.T) {
	tests := []struct {
		event string
		want  string
	}{
		{"delivered", "sent"},
		{"bounce", "bounced"},
		{"blocked", "bounced"},
		{"dropped", "failed"},
		{"spamreport", "complained"},
		{"open", ""},
		{"click", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.event, func(t *testing.T) {
			got := normalizeSendGridStatus(tc.event)
			if got != tc.want {
				t.Errorf("normalizeSendGridStatus(%q) = %q, want %q", tc.event, got, tc.want)
			}
		})
	}
}

func TestNormalizeSESStatus(t *testing.T) {
	tests := []struct {
		notificationType string
		want             string
	}{
		{"Delivery", "sent"},
		{"Bounce", "bounced"},
		{"Complaint", "complained"},
		{"Unknown", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.notificationType, func(t *testing.T) {
			got := normalizeSESStatus(tc.notificationType)
			if got != tc.want {
				t.Errorf("normalizeSESStatus(%q) = %q, want %q", tc.notificationType, got, tc.want)
			}
		})
	}
}

func TestNormalizeMailgunStatus(t *testing.T) {
	tests := []struct {
		event string
		want  string
	}{
		{"delivered", "sent"},
		{"failed", "failed"},
		{"rejected", "failed"},
		{"complained", "complained"},
		{"opened", ""},
		{"clicked", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.event, func(t *testing.T) {
			got := normalizeMailgunStatus(tc.event)
			if got != tc.want {
				t.Errorf("normalizeMailgunStatus(%q) = %q, want %q", tc.event, got, tc.want)
			}
		})
	}
}
