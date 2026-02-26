package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// SQSDLQ manages dead letter queue operations backed by an AWS SQS queue.
type SQSDLQ struct {
	client     sqsAPI
	dlqURL     string
	primaryURL string
	enqueuer   Enqueuer
	log        zerolog.Logger
}

// NewSQSDLQ creates a new SQSDLQ targeting the given DLQ URL. The enqueuer
// is used by Reprocess to re-enqueue messages back to the primary queue.
func NewSQSDLQ(client sqsAPI, dlqURL string, primaryURL string, enqueuer Enqueuer, log zerolog.Logger) *SQSDLQ {
	return &SQSDLQ{
		client:     client,
		dlqURL:     dlqURL,
		primaryURL: primaryURL,
		enqueuer:   enqueuer,
		log:        log,
	}
}

// MoveToDLQ wraps the failed message in a DLQMessage envelope and sends it
// to the dead letter queue.
func (d *SQSDLQ) MoveToDLQ(ctx context.Context, msg *Message, reason string) error {
	dlqMsg := DLQMessage{
		OriginalMessage: msg,
		FailureReason:   reason,
		FinalError:      reason,
		MovedAt:         time.Now(),
	}

	data, err := json.Marshal(dlqMsg)
	if err != nil {
		return fmt.Errorf("marshal dlq message: %w", err)
	}

	_, err = d.client.SendMessage(ctx, &sqsSendInput{
		QueueURL:    d.dlqURL,
		MessageBody: string(data),
	})
	if err != nil {
		return fmt.Errorf("sqs send to dlq: %w", err)
	}

	DLQMessagesTotal.WithLabelValues(reason).Inc()
	MessagesProcessedTotal.WithLabelValues("dlq").Inc()

	return nil
}

// Reprocess reads messages from the DLQ, resets their retry count, and
// re-enqueues them to the primary queue via the Enqueuer. It returns
// the number of messages successfully reprocessed.
func (d *SQSDLQ) Reprocess(ctx context.Context, _ string, messageIDs []string) (int, error) {
	// SQS does not support reading by message ID. We poll the DLQ and
	// re-enqueue any messages we receive, up to len(messageIDs) count.
	// This is a best-effort approach; in production the SQS redrive
	// policy is the primary mechanism.

	batchSize := len(messageIDs)
	if batchSize == 0 {
		return 0, nil
	}
	if batchSize > 10 {
		batchSize = 10
	}

	out, err := d.client.ReceiveMessage(ctx, &sqsReceiveInput{
		QueueURL:            d.dlqURL,
		MaxNumberOfMessages: int32(batchSize),
		WaitTimeSeconds:     0, // no long-poll for reprocessing
		VisibilityTimeout:   30,
	})
	if err != nil {
		return 0, fmt.Errorf("sqs receive from dlq: %w", err)
	}

	reprocessed := 0
	for _, sqsMsg := range out.Messages {
		var dlqMsg DLQMessage
		if err := json.Unmarshal([]byte(sqsMsg.Body), &dlqMsg); err != nil {
			d.log.Warn().Err(err).Msg("skipping malformed dlq message")
			continue
		}

		// Reset retry count and re-enqueue to primary queue.
		dlqMsg.OriginalMessage.RetryCount = 0
		if _, err := d.enqueuer.Enqueue(ctx, dlqMsg.OriginalMessage); err != nil {
			return reprocessed, fmt.Errorf("re-enqueue message %s: %w", dlqMsg.OriginalMessage.ID, err)
		}

		// Delete from DLQ after successful re-enqueue.
		if err := d.client.DeleteMessage(ctx, &sqsDeleteInput{
			QueueURL:      d.dlqURL,
			ReceiptHandle: sqsMsg.ReceiptHandle,
		}); err != nil {
			return reprocessed, fmt.Errorf("delete dlq message: %w", err)
		}

		reprocessed++
	}

	return reprocessed, nil
}
