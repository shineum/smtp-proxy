package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Producer enqueues messages into Redis Streams.
type Producer struct {
	client *redis.Client
}

// NewProducer creates a new Producer backed by the given Redis client.
func NewProducer(client *redis.Client) *Producer {
	return &Producer{client: client}
}

// EnqueueMessage adds a message to the tenant's Redis stream using XADD.
// It returns the Redis stream entry ID.
func (p *Producer) EnqueueMessage(ctx context.Context, msg *Message) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	entryID, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(msg.TenantID),
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("xadd to stream %s: %w", streamKey(msg.TenantID), err)
	}

	MessagesEnqueuedTotal.Inc()

	return entryID, nil
}
