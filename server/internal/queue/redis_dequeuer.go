package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// RedisDequeuer manages a pool of worker goroutines that consume and process
// messages from Redis Streams using consumer groups.
type RedisDequeuer struct {
	client    *redis.Client
	enqueuer  Enqueuer
	dlq       DeadLetterQueue
	handler   MessageHandler
	retry     *RetryStrategy
	config    Config
	log       zerolog.Logger
	tenantID  string
	groupName string
	wg        sync.WaitGroup
	cancel    context.CancelFunc
}

// NewRedisDequeuer creates a RedisDequeuer for processing messages from a
// tenant's queue. The handler defines message processing logic.
func NewRedisDequeuer(
	client *redis.Client,
	enqueuer Enqueuer,
	dlq DeadLetterQueue,
	handler MessageHandler,
	retry *RetryStrategy,
	cfg Config,
	log zerolog.Logger,
	tenantID string,
	groupName string,
) *RedisDequeuer {
	return &RedisDequeuer{
		client:    client,
		enqueuer:  enqueuer,
		dlq:       dlq,
		handler:   handler,
		retry:     retry,
		config:    cfg,
		log:       log,
		tenantID:  tenantID,
		groupName: groupName,
	}
}

// Start creates the consumer group (if it does not already exist) and
// launches the configured number of worker goroutines.
func (d *RedisDequeuer) Start(ctx context.Context) error {
	if err := d.createConsumerGroup(ctx); err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}

	ctx, d.cancel = context.WithCancel(ctx)

	for i := range d.config.WorkerCount {
		d.wg.Add(1)
		go d.runWorker(ctx, fmt.Sprintf("worker-%d", i))
	}

	d.log.Info().
		Int("worker_count", d.config.WorkerCount).
		Str("tenant_id", d.tenantID).
		Msg("redis dequeuer started")

	return nil
}

// Stop signals all workers to stop and waits up to the configured shutdown
// timeout for them to finish processing.
func (d *RedisDequeuer) Stop(ctx context.Context) error {
	d.cancel()

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.log.Info().Msg("redis dequeuer stopped gracefully")
		return nil
	case <-time.After(d.config.ShutdownTimeout):
		d.log.Warn().Msg("redis dequeuer shutdown timed out")
		return fmt.Errorf("shutdown timed out after %s", d.config.ShutdownTimeout)
	}
}

// createConsumerGroup creates a consumer group for the tenant's stream.
// If the stream or group already exists, the error is ignored.
func (d *RedisDequeuer) createConsumerGroup(ctx context.Context) error {
	err := d.client.XGroupCreateMkStream(ctx, streamKey(d.tenantID), d.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group %s on stream %s: %w", d.groupName, streamKey(d.tenantID), err)
	}
	return nil
}

// runWorker is the main loop for a single worker goroutine.
func (d *RedisDequeuer) runWorker(ctx context.Context, consumerName string) {
	defer d.wg.Done()

	d.log.Info().Str("consumer", consumerName).Msg("worker started")

	for {
		select {
		case <-ctx.Done():
			d.log.Info().Str("consumer", consumerName).Msg("worker stopping")
			return
		default:
		}

		xMsgs, err := d.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    d.groupName,
			Consumer: consumerName,
			Streams:  []string{streamKey(d.tenantID), ">"},
			Count:    1,
			Block:    d.config.BlockTimeout,
		}).Result()
		if err != nil {
			if err == redis.Nil || ctx.Err() != nil {
				continue
			}
			d.log.Error().Err(err).Str("consumer", consumerName).Msg("xreadgroup error")
			continue
		}

		for _, stream := range xMsgs {
			for _, xMsg := range stream.Messages {
				d.processMessage(ctx, consumerName, xMsg)
			}
		}
	}
}

// processMessage handles a single Redis stream message: deserializes it,
// invokes the handler, and either acknowledges or retries/DLQs on failure.
func (d *RedisDequeuer) processMessage(ctx context.Context, consumerName string, xMsg redis.XMessage) {
	start := time.Now()

	data, ok := xMsg.Values["data"].(string)
	if !ok {
		d.log.Error().Str("entry_id", xMsg.ID).Msg("invalid message data type")
		_ = d.acknowledgeMessage(ctx, xMsg.ID)
		return
	}

	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		d.log.Error().Err(err).Str("entry_id", xMsg.ID).Msg("failed to unmarshal message")
		_ = d.acknowledgeMessage(ctx, xMsg.ID)
		return
	}

	processCtx, cancel := context.WithTimeout(ctx, d.config.ProcessTimeout)
	defer cancel()

	err := d.handler.HandleMessage(processCtx, &msg)

	duration := time.Since(start).Seconds()
	MessageProcessingDuration.Observe(duration)

	if err != nil {
		d.log.Error().
			Err(err).
			Str("message_id", msg.ID).
			Int("retry_count", msg.RetryCount).
			Msg("message processing failed")

		msg.RetryCount++

		if d.retry.ShouldRetry(msg.RetryCount) {
			backoff := d.retry.NextBackoff(msg.RetryCount - 1)
			d.log.Info().
				Str("message_id", msg.ID).
				Int("retry_count", msg.RetryCount).
				Dur("backoff", backoff).
				Msg("scheduling retry")

			// Re-enqueue after backoff by sleeping then re-adding.
			go d.retryAfterBackoff(context.WithoutCancel(ctx), &msg, backoff)

			MessagesProcessedTotal.WithLabelValues("failed").Inc()
		} else {
			d.log.Warn().
				Str("message_id", msg.ID).
				Int("retry_count", msg.RetryCount).
				Msg("max retries exhausted, moving to DLQ")

			if dlqErr := d.dlq.MoveToDLQ(ctx, &msg, err.Error()); dlqErr != nil {
				d.log.Error().Err(dlqErr).Str("message_id", msg.ID).Msg("failed to move to DLQ")
			}
		}
	} else {
		MessagesProcessedTotal.WithLabelValues("sent").Inc()
	}

	// Acknowledge regardless of outcome to prevent redelivery of the original.
	if ackErr := d.acknowledgeMessage(ctx, xMsg.ID); ackErr != nil {
		d.log.Error().Err(ackErr).Str("entry_id", xMsg.ID).Msg("failed to acknowledge message")
	}
}

// acknowledgeMessage acknowledges a message in the consumer group using XACK.
func (d *RedisDequeuer) acknowledgeMessage(ctx context.Context, entryID string) error {
	err := d.client.XAck(ctx, streamKey(d.tenantID), d.groupName, entryID).Err()
	if err != nil {
		return fmt.Errorf("xack message %s on stream %s: %w", entryID, streamKey(d.tenantID), err)
	}
	return nil
}

// retryAfterBackoff waits for the backoff duration then re-enqueues the message
// using the injected Enqueuer.
func (d *RedisDequeuer) retryAfterBackoff(ctx context.Context, msg *Message, backoff time.Duration) {
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	if _, err := d.enqueuer.Enqueue(ctx, msg); err != nil {
		d.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to re-enqueue message for retry")
	}
}
