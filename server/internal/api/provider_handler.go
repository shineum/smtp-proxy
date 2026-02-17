package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// providerRequest is the JSON body for creating or updating a provider.
type providerRequest struct {
	Name         string          `json:"name"`
	ProviderType string          `json:"provider_type"`
	APIKey       *string         `json:"api_key"`
	SMTPConfig   json.RawMessage `json:"smtp_config"`
	Enabled      bool            `json:"enabled"`
}

// providerResponse is the JSON response for a provider.
type providerResponse struct {
	ID           uuid.UUID       `json:"id"`
	AccountID    uuid.UUID       `json:"account_id"`
	Name         string          `json:"name"`
	ProviderType string          `json:"provider_type"`
	SMTPConfig   json.RawMessage `json:"smtp_config"`
	Enabled      bool            `json:"enabled"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

// toProviderResponse converts a storage.EspProvider to a providerResponse.
// The api_key field is intentionally excluded for security.
func toProviderResponse(p storage.EspProvider) providerResponse {
	smtpConfig := json.RawMessage(p.SmtpConfig)
	if len(smtpConfig) == 0 {
		smtpConfig = json.RawMessage(`{}`)
	}

	return providerResponse{
		ID:           p.ID,
		AccountID:    p.AccountID,
		Name:         p.Name,
		ProviderType: string(p.ProviderType),
		SMTPConfig:   smtpConfig,
		Enabled:      p.Enabled,
		CreatedAt:    timestampToTime(p.CreatedAt).Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    timestampToTime(p.UpdatedAt).Format("2006-01-02T15:04:05Z07:00"),
	}
}

// validProviderTypes contains the set of allowed provider type values.
var validProviderTypes = map[string]storage.ProviderType{
	"sendgrid": storage.ProviderTypeSendgrid,
	"mailgun":  storage.ProviderTypeMailgun,
	"ses":      storage.ProviderTypeSes,
	"smtp":     storage.ProviderTypeSmtp,
	"msgraph":  storage.ProviderTypeMsgraph,
}

// CreateProviderHandler handles POST /api/v1/providers.
func CreateProviderHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := auth.AccountFromContext(r.Context())
		if accountID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req providerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate provider_type
		pt, ok := validProviderTypes[req.ProviderType]
		if !ok {
			respondError(w, http.StatusBadRequest, "invalid provider_type")
			return
		}

		// Build api_key as sql.NullString
		var apiKey sql.NullString
		if req.APIKey != nil {
			apiKey = sql.NullString{String: *req.APIKey, Valid: true}
		}

		// Marshal smtp_config
		smtpConfig := []byte("{}")
		if len(req.SMTPConfig) > 0 {
			smtpConfig = req.SMTPConfig
		}

		provider, err := queries.CreateProvider(r.Context(), storage.CreateProviderParams{
			AccountID:    accountID,
			Name:         req.Name,
			ProviderType: pt,
			ApiKey:       apiKey,
			SmtpConfig:   smtpConfig,
			Enabled:      req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusCreated, toProviderResponse(provider))
	}
}

// ListProvidersHandler handles GET /api/v1/providers.
func ListProvidersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := auth.AccountFromContext(r.Context())
		if accountID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		providers, err := queries.ListProvidersByAccountID(r.Context(), accountID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		result := make([]providerResponse, len(providers))
		for i, p := range providers {
			result[i] = toProviderResponse(p)
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// GetProviderHandler handles GET /api/v1/providers/{id}.
func GetProviderHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}

		provider, err := queries.GetProviderByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}

		respondJSON(w, http.StatusOK, toProviderResponse(provider))
	}
}

// UpdateProviderHandler handles PUT /api/v1/providers/{id}.
func UpdateProviderHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}

		var req providerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate provider_type
		pt, ok := validProviderTypes[req.ProviderType]
		if !ok {
			respondError(w, http.StatusBadRequest, "invalid provider_type")
			return
		}

		// Build api_key as sql.NullString
		var apiKey sql.NullString
		if req.APIKey != nil {
			apiKey = sql.NullString{String: *req.APIKey, Valid: true}
		}

		// Marshal smtp_config
		smtpConfig := []byte("{}")
		if len(req.SMTPConfig) > 0 {
			smtpConfig = req.SMTPConfig
		}

		provider, err := queries.UpdateProvider(r.Context(), storage.UpdateProviderParams{
			ID:           id,
			Name:         req.Name,
			ProviderType: pt,
			ApiKey:       apiKey,
			SmtpConfig:   smtpConfig,
			Enabled:      req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusOK, toProviderResponse(provider))
	}
}

// DeleteProviderHandler handles DELETE /api/v1/providers/{id}.
func DeleteProviderHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}

		if err := queries.DeleteProvider(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
