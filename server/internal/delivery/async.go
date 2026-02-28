package delivery

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/queue"
)

// AsyncService enqueues messages for background delivery
// by the queue-worker process.
type AsyncService struct {
	enqueuer queue.Enqueuer
	log      zerolog.Logger
}

// NewAsyncService creates an AsyncService backed by the given Enqueuer.
func NewAsyncService(enqueuer queue.Enqueuer, log zerolog.Logger) *AsyncService {
	return &AsyncService{
		enqueuer: enqueuer,
		log:      log,
	}
}

// DeliverMessage enqueues an ID-only message reference to Redis Streams.
// The actual ESP delivery is handled asynchronously by the queue-worker process,
// which fetches the full message body from the message store.
func (a *AsyncService) DeliverMessage(ctx context.Context, req *Request) error {
	msg := queue.NewIDOnlyMessage(req.MessageID.String(), req.GroupID.String(), req.GroupID.String())

	entryID, err := a.enqueuer.Enqueue(ctx, msg)
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
