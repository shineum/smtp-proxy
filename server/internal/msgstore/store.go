// Package msgstore provides message storage backends for the SMTP proxy.
package msgstore

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
)

// ErrNotFound is returned when a requested message does not exist.
var ErrNotFound = errors.New("msgstore: message not found")

// MessageStore defines the interface for message storage backends.
type MessageStore interface {
	Put(ctx context.Context, messageID string, data []byte) error
	Get(ctx context.Context, messageID string) ([]byte, error)
	Delete(ctx context.Context, messageID string) error
}

// Config holds configuration for creating a MessageStore.
type Config struct {
	Type       string // "local" or "s3"
	Path       string // base directory for local store
	S3Bucket   string
	S3Prefix   string
	S3Endpoint string
	S3Region   string
}

// New creates a MessageStore based on the provided configuration.
// If Type is empty or unsupported, it defaults to local storage and logs a warning.
func New(cfg Config, logger zerolog.Logger) (MessageStore, error) {
	switch cfg.Type {
	case "local":
		return NewLocalFileStore(cfg.Path)
	case "s3":
		return NewS3StoreFromConfig(cfg)
	default:
		logger.Warn().
			Str("type", cfg.Type).
			Msg("unsupported or empty store type, defaulting to local")
		return NewLocalFileStore(cfg.Path)
	}
}
