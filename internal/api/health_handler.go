package api

import (
	"net/http"

	"github.com/sungwon/smtp-proxy/internal/storage"
)

// HealthzHandler handles GET /healthz.
// Always returns 200 OK with {"status":"ok"}.
func HealthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ReadyzHandler handles GET /readyz.
// Checks database connectivity via ping.
// Returns 200 if healthy, 503 with Retry-After header if unhealthy.
func ReadyzHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			w.Header().Set("Retry-After", "30")
			respondError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
