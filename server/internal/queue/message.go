package queue

import (
	"time"

	"github.com/google/uuid"
)

// Message represents an email message in the queue.
type Message struct {
	ID         string            `json:"id"`
	TenantID   string            `json:"tenant_id"`
	From       string            `json:"from"`
	To         []string          `json:"to"`
	Subject    string            `json:"subject"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body"`
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

// streamKey returns the Redis stream key for this message's tenant.
func streamKey(tenantID string) string {
	return "queue:" + tenantID
}

// dlqStreamKey returns the Redis DLQ stream key for a tenant.
func dlqStreamKey(tenantID string) string {
	return "dlq:" + tenantID
}
