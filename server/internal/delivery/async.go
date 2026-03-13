package delivery

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/queue"
)

// AsyncService enqueues messages for background delivery
// by the queue-worker process.
type AsyncService struct {
	enqueuer   queue.Enqueuer
	streamName string
	log        zerolog.Logger
}

// NewAsyncService creates an AsyncService backed by the given Enqueuer.
// streamName must match the queue worker's configured stream name so both
// sides use the same Redis stream key.
func NewAsyncService(enqueuer queue.Enqueuer, streamName string, log zerolog.Logger) *AsyncService {
	return &AsyncService{
		enqueuer:   enqueuer,
		streamName: streamName,
		log:        log,
	}
}

// DeliverMessage enqueues an ID-only message reference to Redis Streams.
// The actual ESP delivery is handled asynchronously by the queue-worker process,
// which fetches the full message body from the message store.
func (a *AsyncService) DeliverMessage(ctx context.Context, req *Request) error {
	messageIDStr := strconv.FormatInt(req.MessageID, 10)
	groupIDStr := strconv.FormatInt(int64(req.GroupID), 10)
	msg := queue.NewIDOnlyMessage(messageIDStr, groupIDStr, a.streamName)

	entryID, err := a.enqueuer.Enqueue(ctx, msg)
	if err != nil {
		a.log.Error().Err(err).
			Int64("message_id", req.MessageID).
			Msg("failed to enqueue message to Redis")
		return fmt.Errorf("enqueue to redis: %w", err)
	}

	a.log.Info().
		Int64("message_id", req.MessageID).
		Str("entry_id", entryID).
		Msg("message enqueued for async delivery")

	return nil
}
