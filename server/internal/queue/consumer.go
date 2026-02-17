package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Consumer reads messages from Redis Streams using consumer groups.
type Consumer struct {
	client *redis.Client
}

// NewConsumer creates a new Consumer backed by the given Redis client.
func NewConsumer(client *redis.Client) *Consumer {
	return &Consumer{client: client}
}

// CreateConsumerGroup creates a consumer group for a tenant's stream.
// If the stream or group already exists, the error is ignored.
func (c *Consumer) CreateConsumerGroup(ctx context.Context, tenantID, groupName string) error {
	err := c.client.XGroupCreateMkStream(ctx, streamKey(tenantID), groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group %s on stream %s: %w", groupName, streamKey(tenantID), err)
	}
	return nil
}

// ReadMessages reads up to count messages from a consumer group using XREADGROUP.
// It blocks for up to the specified duration waiting for new messages.
// Returns deserialized Message objects.
func (c *Consumer) ReadMessages(ctx context.Context, tenantID, groupName, consumerName string, count int64, block time.Duration) ([]Message, error) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamKey(tenantID), ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("xreadgroup: %w", err)
	}

	var messages []Message
	for _, stream := range streams {
		for _, xMsg := range stream.Messages {
			data, ok := xMsg.Values["data"].(string)
			if !ok {
				continue
			}

			var msg Message
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue
			}

			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// AcknowledgeMessage acknowledges a message in the consumer group using XACK.
func (c *Consumer) AcknowledgeMessage(ctx context.Context, tenantID, groupName, entryID string) error {
	err := c.client.XAck(ctx, streamKey(tenantID), groupName, entryID).Err()
	if err != nil {
		return fmt.Errorf("xack message %s on stream %s: %w", entryID, streamKey(tenantID), err)
	}
	return nil
}
