package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// ListProviderFallbacksHandler handles GET /api/v1/users/{id}/fallbacks.
func ListProviderFallbacksHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID")
			return
		}

		rows, err := queries.ListAllProviderFallbacksByUserID(r.Context(), uid)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusOK, rows)
	}
}

// CreateProviderFallbackHandler handles POST /api/v1/users/{id}/fallbacks.
func CreateProviderFallbackHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user ID")
			return
		}

		var req struct {
			ProviderID string `json:"provider_id"`
			Priority   int32  `json:"priority"`
			Enabled    bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		providerID, err := uuid.Parse(req.ProviderID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider_id")
			return
		}

		fb, err := queries.CreateProviderFallback(r.Context(), storage.CreateProviderFallbackParams{
			UserID:     uid,
			ProviderID: providerID,
			Priority:   req.Priority,
			Enabled:    req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create fallback")
			return
		}

		respondJSON(w, http.StatusCreated, fb)
	}
}

// UpdateProviderFallbackHandler handles PUT /api/v1/users/{id}/fallbacks/{fid}.
func UpdateProviderFallbackHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "fid"))
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid fallback ID")
			return
		}

		var req struct {
			Priority int32 `json:"priority"`
			Enabled  bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		fb, err := queries.UpdateProviderFallback(r.Context(), storage.UpdateProviderFallbackParams{
			ID:       id,
			Priority: req.Priority,
			Enabled:  req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update fallback")
			return
		}

		respondJSON(w, http.StatusOK, fb)
	}
}

// DeleteProviderFallbackHandler handles DELETE /api/v1/users/{id}/fallbacks/{fid}.
func DeleteProviderFallbackHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "fid"))
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid fallback ID")
			return
		}

		if err := queries.DeleteProviderFallback(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to delete fallback")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
