package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
)

// SQSEnqueuer publishes messages to an AWS SQS queue.
type SQSEnqueuer struct {
	client   sqsAPI
	queueURL string
	log      zerolog.Logger
}

// NewSQSEnqueuer creates a new SQSEnqueuer targeting the given queue URL.
func NewSQSEnqueuer(client sqsAPI, queueURL string, log zerolog.Logger) *SQSEnqueuer {
	return &SQSEnqueuer{
		client:   client,
		queueURL: queueURL,
		log:      log,
	}
}

// Enqueue serializes the message to JSON and sends it via SQS SendMessage.
// It returns the SQS message ID.
func (e *SQSEnqueuer) Enqueue(ctx context.Context, msg *Message) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	out, err := e.client.SendMessage(ctx, &sqsSendInput{
		QueueURL:    e.queueURL,
		MessageBody: string(data),
	})
	if err != nil {
		return "", fmt.Errorf("sqs send message: %w", err)
	}

	MessagesEnqueuedTotal.Inc()

	return out.MessageID, nil
}

// EnqueueWithDelay serializes the message and sends it with a delay.
// The delay is capped at 900 seconds (SQS maximum).
func (e *SQSEnqueuer) EnqueueWithDelay(ctx context.Context, msg *Message, delaySeconds int32) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	if delaySeconds > 900 {
		delaySeconds = 900
	}

	out, err := e.client.SendMessage(ctx, &sqsSendInput{
		QueueURL:     e.queueURL,
		MessageBody:  string(data),
		DelaySeconds: delaySeconds,
	})
	if err != nil {
		return "", fmt.Errorf("sqs send message with delay: %w", err)
	}

	MessagesEnqueuedTotal.Inc()

	return out.MessageID, nil
}
