package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
)

func TestCorrelationIDMiddleware_GeneratesID(t *testing.T) {
	var capturedID string

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = logger.CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := CorrelationIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("expected correlation ID to be generated")
	}

	// Verify header is set in response
	respID := rec.Header().Get("X-Correlation-ID")
	if respID == "" {
		t.Error("expected X-Correlation-ID header in response")
	}
	if respID != capturedID {
		t.Errorf("expected response header %s to match context ID %s", respID, capturedID)
	}
}

func TestCorrelationIDMiddleware_UsesExistingID(t *testing.T) {
	existingID := "test-correlation-123"
	var capturedID string

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = logger.CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := CorrelationIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Correlation-ID", existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("expected correlation ID %s, got %s", existingID, capturedID)
	}
}

func TestLoggingMiddleware_SetsStatus(t *testing.T) {
	log := zerolog.Nop()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := LoggingMiddleware(log)(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestLoggingMiddleware_DefaultStatus(t *testing.T) {
	log := zerolog.Nop()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Writing body without explicit WriteHeader defaults to 200
		w.Write([]byte("ok"))
	})

	handler := LoggingMiddleware(log)(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestRecoverMiddleware_RecoversPanic(t *testing.T) {
	log := zerolog.Nop()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := RecoverMiddleware(log)(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// This should not panic
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "internal server error" {
		t.Errorf("expected error 'internal server error', got %s", resp["error"])
	}
}

func TestRecoverMiddleware_NoRecoverOnNormal(t *testing.T) {
	log := zerolog.Nop()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := RecoverMiddleware(log)(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestStatusWriter_WriteHeaderOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	sw.WriteHeader(http.StatusCreated)
	sw.WriteHeader(http.StatusNotFound) // second call should be ignored

	if sw.status != http.StatusCreated {
		t.Errorf("expected status 201, got %d", sw.status)
	}
}
