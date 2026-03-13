package queue

import (
	"time"

	"github.com/google/uuid"
)

// Message represents an email message in the queue.
//
// Two formats are supported for backward compatibility:
//   - Full payload: all fields populated (legacy format)
//   - ID-only: only ID, AccountID, and TenantID populated; the worker
//     fetches the body from the message store using the ID.
type Message struct {
	ID         string            `json:"id"`
	AccountID  string            `json:"account_id,omitempty"`
	TenantID   string            `json:"tenant_id"`
	From       string            `json:"from,omitempty"`
	To         []string          `json:"to,omitempty"`
	Subject    string            `json:"subject,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
	RetryCount int               `json:"retry_count"`
	CreatedAt  time.Time         `json:"created_at"`
}

// NewMessage creates a new Message with a generated UUID and current timestamp.
func NewMessage(tenantID, from string, to []string, subject string, body []byte) *Message {
	return &Message{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		From:      from,
		To:        to,
		Subject:   subject,
		Body:      body,
		CreatedAt: time.Now(),
	}
}

// NewIDOnlyMessage creates a lightweight queue message that contains only an
// ID reference. The worker is expected to fetch the full body from the message
// store using this ID.
func NewIDOnlyMessage(id, accountID, tenantID string) *Message {
	return &Message{
		ID:        id,
		AccountID: accountID,
		TenantID:  tenantID,
		CreatedAt: time.Now(),
	}
}

// HasInlineBody reports whether the message carries its body inline (legacy
// full-payload format) as opposed to being an ID-only reference.
func (m *Message) HasInlineBody() bool {
	return len(m.Body) > 0
}

// streamKey returns the Redis stream key for this message's tenant.
func streamKey(tenantID string) string {
	return "queue:" + tenantID
}

// dlqStreamKey returns the Redis DLQ stream key for a tenant.
func dlqStreamKey(tenantID string) string {
	return "dlq:" + tenantID
}
