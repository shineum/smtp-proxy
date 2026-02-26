package logger

import (
	"context"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// LoggingConfig mirrors config.LoggingConfig to avoid a circular import.
// Callers should populate this from config.LoggingConfig fields.
type LoggingConfig struct {
	Level     string
	Output    string // stdout (default), file, cloudwatch
	FilePath  string
	MaxSizeMB int
	MaxFiles  int
	CWGroup   string
	CWStream  string
	CWRegion  string
}

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

// NewFromConfig creates a zerolog.Logger from a LoggingConfig, selecting the
// appropriate output writer based on cfg.Output:
//   - "file": rotating file via lumberjack
//   - "cloudwatch": CloudWatch Logs (currently a stub writing to stdout)
//   - "stdout" or any other value: os.Stdout (default)
//
// The existing New() function is preserved for backward compatibility.
func NewFromConfig(cfg LoggingConfig) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var writer io.Writer
	switch cfg.Output {
	case "file":
		writer = NewFileWriter(FileConfig{
			Path:      cfg.FilePath,
			MaxSizeMB: cfg.MaxSizeMB,
			MaxFiles:  cfg.MaxFiles,
		})
	case "cloudwatch":
		writer = NewCloudWatchWriter(CloudWatchConfig{
			Group:  cfg.CWGroup,
			Stream: cfg.CWStream,
			Region: cfg.CWRegion,
		})
	default:
		writer = os.Stdout
	}

	return zerolog.New(writer).
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
