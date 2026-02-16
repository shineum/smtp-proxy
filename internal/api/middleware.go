package api

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/sungwon/smtp-proxy/internal/logger"
)

// LoggingMiddleware logs each HTTP request with method, path, status code, and duration.
func LoggingMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", sw.status).
				Dur("duration", duration).
				Str("correlation_id", logger.CorrelationIDFromContext(r.Context())).
				Msg("request completed")
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader captures the status code before delegating to the wrapped writer.
func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

// CorrelationIDMiddleware generates or extracts a correlation ID from the
// X-Correlation-ID header and stores it in the request context.
func CorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = logger.NewCorrelationID()
		}

		w.Header().Set("X-Correlation-ID", correlationID)

		ctx := logger.WithCorrelationID(r.Context(), correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RecoverMiddleware recovers from panics, logs the error, and returns a 500 response.
func RecoverMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error().
						Interface("panic", err).
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Msg("panic recovered")
					respondError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
