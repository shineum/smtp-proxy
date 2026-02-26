package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNew_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	log := New("info")
	log = log.Output(&buf)

	log.Info().Msg("test message")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v, output: %s", err, buf.String())
	}

	if entry["message"] != "test message" {
		t.Errorf("expected message 'test message', got %v", entry["message"])
	}
	if _, ok := entry["time"]; !ok {
		t.Error("expected 'time' field in JSON output")
	}
}

func TestNew_LevelFiltering(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		logLevel  string // level to log at
		shouldLog bool
	}{
		{"info logger logs info", "info", "info", true},
		{"info logger logs warn", "info", "warn", true},
		{"info logger skips debug", "info", "debug", false},
		{"debug logger logs debug", "debug", "debug", true},
		{"warn logger skips info", "warn", "info", false},
		{"error logger skips warn", "error", "warn", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := New(tt.level).Output(&buf)

			switch tt.logLevel {
			case "debug":
				log.Debug().Msg("test")
			case "info":
				log.Info().Msg("test")
			case "warn":
				log.Warn().Msg("test")
			case "error":
				log.Error().Msg("test")
			}

			hasOutput := buf.Len() > 0
			if hasOutput != tt.shouldLog {
				t.Errorf("level=%s, logAt=%s: expected shouldLog=%v, got output=%v (%s)",
					tt.level, tt.logLevel, tt.shouldLog, hasOutput, buf.String())
			}
		})
	}
}

func TestNew_InvalidLevel_DefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	log := New("invalid_level").Output(&buf)

	// Should default to info, so debug should not appear
	log.Debug().Msg("debug message")
	if buf.Len() > 0 {
		t.Error("expected debug message to be filtered at info level")
	}

	log.Info().Msg("info message")
	if buf.Len() == 0 {
		t.Error("expected info message to appear at info level")
	}
}

func TestWithCorrelationID(t *testing.T) {
	ctx := context.Background()
	correlationID := "test-correlation-123"

	ctx = WithCorrelationID(ctx, correlationID)

	got := CorrelationIDFromContext(ctx)
	if got != correlationID {
		t.Errorf("expected correlation ID %s, got %s", correlationID, got)
	}
}

func TestFromContext_WithCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	log := New("info").Output(&buf)

	ctx := context.Background()
	ctx = WithLogger(ctx, log)
	ctx = WithCorrelationID(ctx, "req-abc-123")

	ctxLogger := FromContext(ctx)
	ctxLogger.Info().Msg("request handled")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %s", err, buf.String())
	}

	if entry["correlation_id"] != "req-abc-123" {
		t.Errorf("expected correlation_id 'req-abc-123', got %v", entry["correlation_id"])
	}
}

func TestFromContext_WithoutLogger(t *testing.T) {
	ctx := context.Background()

	// Should return a default logger without panic
	log := FromContext(ctx)
	var buf bytes.Buffer
	log = log.Output(&buf)
	log.Info().Msg("fallback")

	if buf.Len() == 0 {
		t.Error("expected fallback logger to produce output")
	}
}

func TestNewFromConfig_StdoutDefault(t *testing.T) {
	var buf bytes.Buffer
	log := NewFromConfig(LoggingConfig{
		Level:  "info",
		Output: "stdout",
	})
	log = log.Output(&buf)

	log.Info().Msg("stdout message")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v, output: %s", err, buf.String())
	}
	if entry["message"] != "stdout message" {
		t.Errorf("expected message 'stdout message', got %v", entry["message"])
	}
}

func TestNewFromConfig_EmptyOutputDefaultsToStdout(t *testing.T) {
	var buf bytes.Buffer
	log := NewFromConfig(LoggingConfig{
		Level:  "debug",
		Output: "",
	})
	log = log.Output(&buf)

	log.Debug().Msg("default output")
	if buf.Len() == 0 {
		t.Error("expected output from default stdout writer")
	}
}

func TestNewFromConfig_UnknownOutputDefaultsToStdout(t *testing.T) {
	var buf bytes.Buffer
	log := NewFromConfig(LoggingConfig{
		Level:  "info",
		Output: "unknown_output",
	})
	log = log.Output(&buf)

	log.Info().Msg("fallback")
	if buf.Len() == 0 {
		t.Error("expected output from stdout fallback for unknown output type")
	}
}

func TestNewFromConfig_FileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/test.log"

	log := NewFromConfig(LoggingConfig{
		Level:     "info",
		Output:    "file",
		FilePath:  logPath,
		MaxSizeMB: 10,
		MaxFiles:  3,
	})

	log.Info().Msg("file message")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("expected valid JSON in log file, got error: %v, output: %s", err, data)
	}
	if entry["message"] != "file message" {
		t.Errorf("expected message 'file message', got %v", entry["message"])
	}
}

func TestNewFromConfig_InvalidLevel(t *testing.T) {
	var buf bytes.Buffer
	log := NewFromConfig(LoggingConfig{
		Level:  "bad_level",
		Output: "stdout",
	})
	log = log.Output(&buf)

	// Should default to info; debug should be filtered
	log.Debug().Msg("debug")
	if buf.Len() > 0 {
		t.Error("expected debug to be filtered at default info level")
	}

	log.Info().Msg("info")
	if buf.Len() == 0 {
		t.Error("expected info message to appear")
	}
}

func TestNewCorrelationID(t *testing.T) {
	id1 := NewCorrelationID()
	id2 := NewCorrelationID()

	if id1 == "" {
		t.Error("expected non-empty correlation ID")
	}
	if id1 == id2 {
		t.Error("expected unique correlation IDs")
	}
	// UUID format: 8-4-4-4-12
	if len(strings.Split(id1, "-")) != 5 {
		t.Errorf("expected UUID format (5 groups), got %s", id1)
	}
}
