package delivery

import (
	"context"

	"github.com/google/uuid"
)

// Service delivers email messages after they have been persisted to the database.
// Two implementations exist: SyncService (direct ESP delivery) and AsyncService
// (enqueue to Redis Streams for background worker delivery).
type Service interface {
	DeliverMessage(ctx context.Context, req *Request) error
}

// Request contains the data needed to deliver a message.
type Request struct {
	MessageID  uuid.UUID
	AccountID  uuid.UUID
	TenantID   string
	Sender     string
	Recipients []string
	Subject    string
	Headers    map[string]string
	Body       []byte
}
