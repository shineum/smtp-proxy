package queue

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// NewEnqueuer creates an Enqueuer based on the given configuration.
// Use this when only message publishing is needed (e.g., SMTP server).
func NewEnqueuer(cfg Config, log zerolog.Logger) (Enqueuer, error) {
	switch cfg.Type {
	case "redis", "":
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		return NewRedisEnqueuer(client), nil

	case "sqs":
		sqsClient, err := newAWSSQSClient(cfg.SQSRegion)
		if err != nil {
			return nil, fmt.Errorf("create sqs client: %w", err)
		}
		return NewSQSEnqueuer(sqsClient, cfg.SQSQueueURL, log), nil

	default:
		return nil, fmt.Errorf("unknown queue type: %s", cfg.Type)
	}
}

// NewQueue creates an Enqueuer, Dequeuer, and DeadLetterQueue based on the
// given configuration. The handler defines the message processing logic used
// by the Dequeuer. tenantID and groupName identify the stream and consumer
// group for dequeuing.
func NewQueue(
	cfg Config,
	handler MessageHandler,
	log zerolog.Logger,
	tenantID string,
	groupName string,
) (Enqueuer, Dequeuer, DeadLetterQueue, error) {
	switch cfg.Type {
	case "redis", "":
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})

		enqueuer := NewRedisEnqueuer(client)
		retry := NewRetryStrategy(cfg.MaxRetries)
		dlq := NewRedisDLQ(client, enqueuer)
		dequeuer := NewRedisDequeuer(client, enqueuer, dlq, handler, retry, cfg, log, tenantID, groupName)

		return enqueuer, dequeuer, dlq, nil

	case "sqs":
		sqsClient, err := newAWSSQSClient(cfg.SQSRegion)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("create sqs client: %w", err)
		}
		retry := NewRetryStrategy(cfg.MaxRetries)
		enqueuer := NewSQSEnqueuer(sqsClient, cfg.SQSQueueURL, log)
		dlq := NewSQSDLQ(sqsClient, cfg.SQSDLQueueURL, cfg.SQSQueueURL, enqueuer, log)
		dequeuer := NewSQSDequeuer(sqsClient, cfg.SQSQueueURL, handler, dlq, retry, enqueuer, cfg, log)

		return enqueuer, dequeuer, dlq, nil

	default:
		return nil, nil, nil, fmt.Errorf("unknown queue type: %s", cfg.Type)
	}
}
