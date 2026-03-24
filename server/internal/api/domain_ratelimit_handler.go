package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// ListDomainRateLimitsHandler handles GET /api/v1/domain-rate-limits.
func ListDomainRateLimitsHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		rows, err := queries.ListDomainRateLimitsByGroupID(r.Context(), groupID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if rows == nil {
			rows = []storage.DomainRateLimit{}
		}

		respondJSON(w, http.StatusOK, rows)
	}
}

// CreateDomainRateLimitHandler handles POST /api/v1/domain-rate-limits.
func CreateDomainRateLimitHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req struct {
			Domain       string `json:"domain"`
			MaxPerMinute int32  `json:"max_per_minute"`
			MaxPerHour   int32  `json:"max_per_hour"`
			Enabled      bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Domain == "" {
			respondError(w, http.StatusBadRequest, "domain is required")
			return
		}

		limit, err := queries.CreateDomainRateLimit(r.Context(), storage.CreateDomainRateLimitParams{
			GroupID:      groupID,
			Domain:       req.Domain,
			MaxPerMinute: req.MaxPerMinute,
			MaxPerHour:   req.MaxPerHour,
			Enabled:      req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create domain rate limit")
			return
		}

		respondJSON(w, http.StatusCreated, limit)
	}
}

// UpdateDomainRateLimitHandler handles PUT /api/v1/domain-rate-limits/{id}.
func UpdateDomainRateLimitHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid rate limit ID")
			return
		}

		var req struct {
			MaxPerMinute int32 `json:"max_per_minute"`
			MaxPerHour   int32 `json:"max_per_hour"`
			Enabled      bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		limit, err := queries.UpdateDomainRateLimit(r.Context(), storage.UpdateDomainRateLimitParams{
			ID:           id,
			MaxPerMinute: req.MaxPerMinute,
			MaxPerHour:   req.MaxPerHour,
			Enabled:      req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update domain rate limit")
			return
		}

		respondJSON(w, http.StatusOK, limit)
	}
}

// DeleteDomainRateLimitHandler handles DELETE /api/v1/domain-rate-limits/{id}.
func DeleteDomainRateLimitHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid rate limit ID")
			return
		}

		if err := queries.DeleteDomainRateLimit(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to delete domain rate limit")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
