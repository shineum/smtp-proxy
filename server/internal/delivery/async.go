package delivery

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/queue"
)

// AsyncService enqueues messages to Redis Streams for background delivery
// by the queue-worker process.
type AsyncService struct {
	producer *queue.Producer
	log      zerolog.Logger
}

// NewAsyncService creates an AsyncService backed by the given Redis producer.
func NewAsyncService(producer *queue.Producer, log zerolog.Logger) *AsyncService {
	return &AsyncService{
		producer: producer,
		log:      log,
	}
}

// DeliverMessage enqueues the message to Redis Streams. The actual ESP delivery
// is handled asynchronously by the queue-worker process.
func (a *AsyncService) DeliverMessage(ctx context.Context, req *Request) error {
	msg := queue.NewMessage(
		req.TenantID,
		req.Sender,
		req.Recipients,
		req.Subject,
		req.Body,
	)
	msg.ID = req.MessageID.String()
	msg.Headers = req.Headers

	entryID, err := a.producer.EnqueueMessage(ctx, msg)
	if err != nil {
		a.log.Error().Err(err).
			Stringer("message_id", req.MessageID).
			Msg("failed to enqueue message to Redis")
		return fmt.Errorf("enqueue to redis: %w", err)
	}

	a.log.Info().
		Stringer("message_id", req.MessageID).
		Str("entry_id", entryID).
		Msg("message enqueued for async delivery")

	return nil
}
