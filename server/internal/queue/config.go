package queue

import "time"

// Config holds configuration for the queue system.
type Config struct {
	// Type selects the queue backend: "redis" (default) or "sqs".
	Type            string        `mapstructure:"type"`
	RedisAddr       string        `mapstructure:"redis_addr"`
	RedisPassword   string        `mapstructure:"redis_password"`
	RedisDB         int           `mapstructure:"redis_db"`
	WorkerCount     int           `mapstructure:"worker_count"`
	BlockTimeout    time.Duration `mapstructure:"block_timeout"`
	ProcessTimeout  time.Duration `mapstructure:"process_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	MaxRetries      int           `mapstructure:"max_retries"`

	// SQS-specific config
	SQSQueueURL   string `mapstructure:"sqs_queue_url"`
	SQSDLQueueURL string `mapstructure:"sqs_dlq_url"`
	SQSRegion     string `mapstructure:"sqs_region"`
	SQSWaitTime   int32  `mapstructure:"sqs_wait_time"`          // long poll seconds, default 20
	SQSVisTimeout int32  `mapstructure:"sqs_visibility_timeout"` // seconds, default 30
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		RedisAddr:       "localhost:6379",
		RedisDB:         0,
		WorkerCount:     10,
		BlockTimeout:    5 * time.Second,
		ProcessTimeout:  30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		MaxRetries:      5,
	}
}
