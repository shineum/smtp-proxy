package api

import (
	"encoding/json"
	"net/http"
)

// respondJSON writes a JSON response with the given status code and data.
// If data is nil, only the status code and Content-Type header are written.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// respondError writes a JSON error response with the given status code and message.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// respondValidationErrors writes a 400 response with a list of validation error details.
func respondValidationErrors(w http.ResponseWriter, errors []string) {
	respondJSON(w, http.StatusBadRequest, map[string]interface{}{
		"error":   "validation_failed",
		"details": errors,
	})
}
