package logger

import (
	"context"
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type contextKey string

const (
	loggerKey        contextKey = "logger"
	correlationIDKey contextKey = "correlation_id"
)

// New creates a zerolog.Logger with the specified level and JSON output.
// If the level string is invalid, it defaults to info.
func New(level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	return zerolog.New(os.Stdout).
		Level(lvl).
		With().
		Timestamp().
		Logger()
}

// WithLogger stores a logger in the context.
func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// WithCorrelationID stores a correlation ID in the context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// CorrelationIDFromContext retrieves the correlation ID from the context.
// Returns an empty string if not set.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// FromContext retrieves the logger from the context. If a correlation ID is
// present, it is attached to the returned logger. If no logger is found
// in the context, a default info-level logger is returned.
func FromContext(ctx context.Context) zerolog.Logger {
	var log zerolog.Logger

	if l, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
		log = l
	} else {
		log = New("info")
	}

	if id := CorrelationIDFromContext(ctx); id != "" {
		log = log.With().Str("correlation_id", id).Logger()
	}

	return log
}

// NewCorrelationID generates a new UUID-based correlation ID.
func NewCorrelationID() string {
	return uuid.New().String()
}
