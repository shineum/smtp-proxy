package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SQSDequeuer manages a pool of worker goroutines that consume and process
// messages from an AWS SQS queue.
type SQSDequeuer struct {
	client          sqsAPI
	queueURL        string
	handler         MessageHandler
	dlq             DeadLetterQueue
	retry           *RetryStrategy
	enqueuer        *SQSEnqueuer
	log             zerolog.Logger
	workerCount     int
	waitTime        int32
	visTimeout      int32
	processTimeout  time.Duration
	shutdownTimeout time.Duration
	wg              sync.WaitGroup
	cancel          context.CancelFunc
}

// NewSQSDequeuer creates an SQSDequeuer configured from the given Config.
func NewSQSDequeuer(
	client sqsAPI,
	queueURL string,
	handler MessageHandler,
	dlq DeadLetterQueue,
	retry *RetryStrategy,
	enqueuer *SQSEnqueuer,
	cfg Config,
	log zerolog.Logger,
) *SQSDequeuer {
	waitTime := cfg.SQSWaitTime
	if waitTime == 0 {
		waitTime = 20
	}
	visTimeout := cfg.SQSVisTimeout
	if visTimeout == 0 {
		visTimeout = 30
	}
	workerCount := cfg.WorkerCount
	if workerCount == 0 {
		workerCount = 10
	}
	processTimeout := cfg.ProcessTimeout
	if processTimeout == 0 {
		processTimeout = 30 * time.Second
	}
	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	return &SQSDequeuer{
		client:          client,
		queueURL:        queueURL,
		handler:         handler,
		dlq:             dlq,
		retry:           retry,
		enqueuer:        enqueuer,
		log:             log,
		workerCount:     workerCount,
		waitTime:        waitTime,
		visTimeout:      visTimeout,
		processTimeout:  processTimeout,
		shutdownTimeout: shutdownTimeout,
	}
}

// Start launches workerCount goroutines that long-poll the SQS queue.
func (d *SQSDequeuer) Start(ctx context.Context) error {
	ctx, d.cancel = context.WithCancel(ctx)

	for i := range d.workerCount {
		d.wg.Add(1)
		go d.runWorker(ctx, fmt.Sprintf("sqs-worker-%d", i))
	}

	d.log.Info().
		Int("worker_count", d.workerCount).
		Str("queue_url", d.queueURL).
		Msg("sqs dequeuer started")

	return nil
}

// Stop cancels the context and waits for workers to finish within the
// shutdown timeout.
func (d *SQSDequeuer) Stop(_ context.Context) error {
	d.cancel()

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.log.Info().Msg("sqs dequeuer stopped gracefully")
		return nil
	case <-time.After(d.shutdownTimeout):
		d.log.Warn().Msg("sqs dequeuer shutdown timed out")
		return fmt.Errorf("shutdown timed out after %s", d.shutdownTimeout)
	}
}

// runWorker is the main loop for a single worker goroutine. It long-polls
// SQS and processes received messages one at a time.
func (d *SQSDequeuer) runWorker(ctx context.Context, workerName string) {
	defer d.wg.Done()

	d.log.Info().Str("worker", workerName).Msg("sqs worker started")

	for {
		select {
		case <-ctx.Done():
			d.log.Info().Str("worker", workerName).Msg("sqs worker stopping")
			return
		default:
		}

		out, err := d.client.ReceiveMessage(ctx, &sqsReceiveInput{
			QueueURL:            d.queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     d.waitTime,
			VisibilityTimeout:   d.visTimeout,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			d.log.Error().Err(err).Str("worker", workerName).Msg("sqs receive error")
			continue
		}

		for _, sqsMsg := range out.Messages {
			d.processMessage(ctx, workerName, sqsMsg)
		}
	}
}

// processMessage deserializes an SQS message body, invokes the handler, and
// either deletes the message (success) or retries/DLQs (failure).
func (d *SQSDequeuer) processMessage(ctx context.Context, workerName string, sqsMsg sqsReceivedMessage) {
	start := time.Now()

	var msg Message
	if err := json.Unmarshal([]byte(sqsMsg.Body), &msg); err != nil {
		d.log.Error().Err(err).
			Str("sqs_message_id", sqsMsg.MessageID).
			Msg("failed to unmarshal sqs message")
		// Delete malformed messages to prevent infinite redelivery.
		_ = d.client.DeleteMessage(ctx, &sqsDeleteInput{
			QueueURL:      d.queueURL,
			ReceiptHandle: sqsMsg.ReceiptHandle,
		})
		return
	}

	processCtx, cancel := context.WithTimeout(ctx, d.processTimeout)
	defer cancel()

	err := d.handler.HandleMessage(processCtx, &msg)

	duration := time.Since(start).Seconds()
	MessageProcessingDuration.Observe(duration)

	if err != nil {
		d.log.Error().
			Err(err).
			Str("message_id", msg.ID).
			Int("retry_count", msg.RetryCount).
			Msg("sqs message processing failed")

		msg.RetryCount++

		if d.retry.ShouldRetry(msg.RetryCount) {
			backoff := d.retry.NextBackoff(msg.RetryCount - 1)
			delaySec := int32(backoff.Seconds())
			if delaySec < 1 {
				delaySec = 1
			}

			d.log.Info().
				Str("message_id", msg.ID).
				Int("retry_count", msg.RetryCount).
				Int32("delay_seconds", delaySec).
				Msg("sqs scheduling retry with delay")

			if _, enqErr := d.enqueuer.EnqueueWithDelay(ctx, &msg, delaySec); enqErr != nil {
				d.log.Error().Err(enqErr).Str("message_id", msg.ID).Msg("failed to re-enqueue for retry")
			}

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

	// Delete the original message regardless of outcome to prevent
	// SQS redelivery. Retries are handled by re-enqueue with delay.
	if delErr := d.client.DeleteMessage(ctx, &sqsDeleteInput{
		QueueURL:      d.queueURL,
		ReceiptHandle: sqsMsg.ReceiptHandle,
	}); delErr != nil {
		d.log.Error().Err(delErr).
			Str("sqs_message_id", sqsMsg.MessageID).
			Msg("failed to delete sqs message")
	}
}
