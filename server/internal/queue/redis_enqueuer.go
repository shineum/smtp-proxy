package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisEnqueuer publishes messages to Redis Streams.
type RedisEnqueuer struct {
	client *redis.Client
}

// NewRedisEnqueuer creates a new RedisEnqueuer backed by the given Redis client.
func NewRedisEnqueuer(client *redis.Client) *RedisEnqueuer {
	return &RedisEnqueuer{client: client}
}

// Enqueue adds a message to the tenant's Redis stream using XADD.
// It returns the Redis stream entry ID.
func (e *RedisEnqueuer) Enqueue(ctx context.Context, msg *Message) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	entryID, err := e.client.XAdd(ctx, &redis.XAddArgs{
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
