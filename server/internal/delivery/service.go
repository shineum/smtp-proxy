package delivery

import (
	"context"

	"github.com/google/uuid"
)

// Service delivers email messages after they have been persisted to the database.
// The sole implementation is AsyncService, which enqueues ID-only references to
// Redis Streams for background worker delivery.
type Service interface {
	DeliverMessage(ctx context.Context, req *Request) error
}

// Request contains the minimal data needed to enqueue a message for delivery.
// The worker process fetches the full message body from the message store using
// the MessageID.
type Request struct {
	MessageID uuid.UUID
	AccountID uuid.UUID
	TenantID  string
}
