package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DLQMessage wraps a failed message with failure metadata.
type DLQMessage struct {
	OriginalMessage *Message  `json:"original_message"`
	FailureReason   string    `json:"failure_reason"`
	RetryHistory    []string  `json:"retry_history,omitempty"`
	FinalError      string    `json:"final_error"`
	MovedAt         time.Time `json:"moved_at"`
}

// RedisDLQ manages dead letter queue operations backed by Redis Streams.
type RedisDLQ struct {
	client   *redis.Client
	enqueuer Enqueuer
}

// NewRedisDLQ creates a new RedisDLQ backed by the given Redis client and enqueuer.
func NewRedisDLQ(client *redis.Client, enqueuer Enqueuer) *RedisDLQ {
	return &RedisDLQ{client: client, enqueuer: enqueuer}
}

// MoveToDLQ moves a failed message to the tenant's dead letter queue stream.
func (d *RedisDLQ) MoveToDLQ(ctx context.Context, msg *Message, reason string) error {
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

	err = d.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqStreamKey(msg.TenantID),
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Err()
	if err != nil {
		return fmt.Errorf("xadd to dlq stream %s: %w", dlqStreamKey(msg.TenantID), err)
	}

	DLQMessagesTotal.WithLabelValues(reason).Inc()
	MessagesProcessedTotal.WithLabelValues("dlq").Inc()

	return nil
}

// Reprocess removes messages from the DLQ, resets their retry count,
// and re-enqueues them to the primary queue. It returns the number of
// messages successfully reprocessed.
func (d *RedisDLQ) Reprocess(ctx context.Context, tenantID string, messageIDs []string) (int, error) {
	reprocessed := 0

	for _, msgID := range messageIDs {
		// Read the message from DLQ.
		msgs, err := d.client.XRange(ctx, dlqStreamKey(tenantID), msgID, msgID).Result()
		if err != nil {
			return reprocessed, fmt.Errorf("xrange dlq message %s: %w", msgID, err)
		}
		if len(msgs) == 0 {
			continue
		}

		data, ok := msgs[0].Values["data"].(string)
		if !ok {
			continue
		}

		var dlqMsg DLQMessage
		if err := json.Unmarshal([]byte(data), &dlqMsg); err != nil {
			continue
		}

		// Reset retry count and re-enqueue.
		dlqMsg.OriginalMessage.RetryCount = 0
		if _, err := d.enqueuer.Enqueue(ctx, dlqMsg.OriginalMessage); err != nil {
			return reprocessed, fmt.Errorf("re-enqueue message %s: %w", dlqMsg.OriginalMessage.ID, err)
		}

		// Remove from DLQ.
		if err := d.client.XDel(ctx, dlqStreamKey(tenantID), msgID).Err(); err != nil {
			return reprocessed, fmt.Errorf("xdel dlq message %s: %w", msgID, err)
		}

		reprocessed++
	}

	return reprocessed, nil
}
