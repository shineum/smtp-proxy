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

// MessageHandler processes a single queue message. Implementations define
// the actual delivery logic (e.g., sending via ESP provider).
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *Message) error
}

// WorkerPool manages a pool of worker goroutines that consume and process
// messages from Redis Streams.
type WorkerPool struct {
	client       *redis.Client
	consumer     *Consumer
	dlq          *DLQ
	handler      MessageHandler
	retry        *RetryStrategy
	config       Config
	log          zerolog.Logger
	tenantID     string
	groupName    string
	wg           sync.WaitGroup
	cancel       context.CancelFunc
}

// NewWorkerPool creates a WorkerPool for processing messages from a tenant's
// queue. The handler defines message processing logic.
func NewWorkerPool(
	client *redis.Client,
	consumer *Consumer,
	dlq *DLQ,
	handler MessageHandler,
	retry *RetryStrategy,
	cfg Config,
	log zerolog.Logger,
	tenantID string,
	groupName string,
) *WorkerPool {
	return &WorkerPool{
		client:    client,
		consumer:  consumer,
		dlq:       dlq,
		handler:   handler,
		retry:     retry,
		config:    cfg,
		log:       log,
		tenantID:  tenantID,
		groupName: groupName,
	}
}

// Start launches the configured number of worker goroutines.
func (wp *WorkerPool) Start(ctx context.Context) {
	ctx, wp.cancel = context.WithCancel(ctx)

	for i := range wp.config.WorkerCount {
		wp.wg.Add(1)
		go wp.runWorker(ctx, fmt.Sprintf("worker-%d", i))
	}

	wp.log.Info().
		Int("worker_count", wp.config.WorkerCount).
		Str("tenant_id", wp.tenantID).
		Msg("worker pool started")
}

// Stop signals all workers to stop and waits up to the configured shutdown
// timeout for them to finish processing.
func (wp *WorkerPool) Stop(ctx context.Context) {
	wp.cancel()

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wp.log.Info().Msg("worker pool stopped gracefully")
	case <-time.After(wp.config.ShutdownTimeout):
		wp.log.Warn().Msg("worker pool shutdown timed out")
	}
}

// runWorker is the main loop for a single worker goroutine.
func (wp *WorkerPool) runWorker(ctx context.Context, consumerName string) {
	defer wp.wg.Done()

	wp.log.Info().Str("consumer", consumerName).Msg("worker started")

	for {
		select {
		case <-ctx.Done():
			wp.log.Info().Str("consumer", consumerName).Msg("worker stopping")
			return
		default:
		}

		xMsgs, err := wp.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    wp.groupName,
			Consumer: consumerName,
			Streams:  []string{streamKey(wp.tenantID), ">"},
			Count:    1,
			Block:    wp.config.BlockTimeout,
		}).Result()
		if err != nil {
			if err == redis.Nil || ctx.Err() != nil {
				continue
			}
			wp.log.Error().Err(err).Str("consumer", consumerName).Msg("xreadgroup error")
			continue
		}

		for _, stream := range xMsgs {
			for _, xMsg := range stream.Messages {
				wp.processMessage(ctx, consumerName, xMsg)
			}
		}
	}
}

// processMessage handles a single Redis stream message: deserializes it,
// invokes the handler, and either acknowledges or retries/DLQs on failure.
func (wp *WorkerPool) processMessage(ctx context.Context, consumerName string, xMsg redis.XMessage) {
	start := time.Now()

	data, ok := xMsg.Values["data"].(string)
	if !ok {
		wp.log.Error().Str("entry_id", xMsg.ID).Msg("invalid message data type")
		_ = wp.consumer.AcknowledgeMessage(ctx, wp.tenantID, wp.groupName, xMsg.ID)
		return
	}

	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		wp.log.Error().Err(err).Str("entry_id", xMsg.ID).Msg("failed to unmarshal message")
		_ = wp.consumer.AcknowledgeMessage(ctx, wp.tenantID, wp.groupName, xMsg.ID)
		return
	}

	processCtx, cancel := context.WithTimeout(ctx, wp.config.ProcessTimeout)
	defer cancel()

	err := wp.handler.HandleMessage(processCtx, &msg)

	duration := time.Since(start).Seconds()
	MessageProcessingDuration.Observe(duration)

	if err != nil {
		wp.log.Error().
			Err(err).
			Str("message_id", msg.ID).
			Int("retry_count", msg.RetryCount).
			Msg("message processing failed")

		msg.RetryCount++

		if wp.retry.ShouldRetry(msg.RetryCount) {
			backoff := wp.retry.NextBackoff(msg.RetryCount - 1)
			wp.log.Info().
				Str("message_id", msg.ID).
				Int("retry_count", msg.RetryCount).
				Dur("backoff", backoff).
				Msg("scheduling retry")

			// Re-enqueue after backoff by sleeping then re-adding.
			go wp.retryAfterBackoff(context.WithoutCancel(ctx), &msg, backoff)

			MessagesProcessedTotal.WithLabelValues("failed").Inc()
		} else {
			wp.log.Warn().
				Str("message_id", msg.ID).
				Int("retry_count", msg.RetryCount).
				Msg("max retries exhausted, moving to DLQ")

			if dlqErr := wp.dlq.MoveToDLQ(ctx, &msg, err.Error()); dlqErr != nil {
				wp.log.Error().Err(dlqErr).Str("message_id", msg.ID).Msg("failed to move to DLQ")
			}
		}
	} else {
		MessagesProcessedTotal.WithLabelValues("sent").Inc()
	}

	// Acknowledge regardless of outcome to prevent redelivery of the original.
	if ackErr := wp.consumer.AcknowledgeMessage(ctx, wp.tenantID, wp.groupName, xMsg.ID); ackErr != nil {
		wp.log.Error().Err(ackErr).Str("entry_id", xMsg.ID).Msg("failed to acknowledge message")
	}
}

// retryAfterBackoff waits for the backoff duration then re-enqueues the message.
func (wp *WorkerPool) retryAfterBackoff(ctx context.Context, msg *Message, backoff time.Duration) {
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	producer := NewProducer(wp.client)
	if _, err := producer.EnqueueMessage(ctx, msg); err != nil {
		wp.log.Error().Err(err).Str("message_id", msg.ID).Msg("failed to re-enqueue message for retry")
	}
}
