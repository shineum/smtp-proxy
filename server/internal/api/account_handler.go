package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// accountRequest is the JSON body for creating or updating an account.
type accountRequest struct {
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Password       string   `json:"password"`
	AllowedDomains []string `json:"allowed_domains"`
}

// accountResponse is the JSON response for an account, excluding sensitive fields.
type accountResponse struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	AllowedDomains []string  `json:"allowed_domains"`
	APIKey         string    `json:"api_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// toAccountResponse converts a storage.Account to an accountResponse.
// If includeAPIKey is true, the api_key field is populated.
func toAccountResponse(a storage.Account, includeAPIKey bool) accountResponse {
	domains := decodeDomains(a.AllowedDomains)

	resp := accountResponse{
		ID:             a.ID,
		Name:           a.Name,
		Email:          a.Email,
		AllowedDomains: domains,
		CreatedAt:      timestampToTime(a.CreatedAt),
		UpdatedAt:      timestampToTime(a.UpdatedAt),
	}
	if includeAPIKey {
		resp.APIKey = a.ApiKey
	}
	return resp
}

// timestampToTime converts a pgtype.Timestamptz to time.Time.
// Returns zero time if the timestamp is not valid.
func timestampToTime(ts pgtype.Timestamptz) time.Time {
	if ts.Valid {
		return ts.Time
	}
	return time.Time{}
}

// decodeDomains unmarshals a JSON byte slice into a string slice.
// Returns an empty slice on failure or nil input.
func decodeDomains(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	var domains []string
	if err := json.Unmarshal(data, &domains); err != nil {
		return []string{}
	}
	return domains
}

// CreateAccountHandler handles POST /api/v1/accounts.
// This endpoint does NOT require authentication.
func CreateAccountHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req accountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate required fields
		var errs []string
		if req.Name == "" {
			errs = append(errs, "name is required")
		}
		if req.Email == "" {
			errs = append(errs, "email is required")
		}
		if req.Password == "" {
			errs = append(errs, "password is required")
		}
		if len(errs) > 0 {
			respondValidationErrors(w, errs)
			return
		}

		// Hash password
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Generate API key
		apiKey, err := auth.GenerateAPIKey()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Marshal allowed domains
		domains := req.AllowedDomains
		if domains == nil {
			domains = []string{}
		}
		domainsJSON, err := json.Marshal(domains)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		account, err := queries.CreateAccount(r.Context(), storage.CreateAccountParams{
			Name:           req.Name,
			Email:          req.Email,
			PasswordHash:   hash,
			AllowedDomains: domainsJSON,
			ApiKey:         apiKey,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusCreated, toAccountResponse(account, true))
	}
}

// GetAccountHandler handles GET /api/v1/accounts/{id}.
func GetAccountHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid account ID format")
			return
		}

		account, err := queries.GetAccountByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "account not found")
			return
		}

		respondJSON(w, http.StatusOK, toAccountResponse(account, false))
	}
}

// UpdateAccountHandler handles PUT /api/v1/accounts/{id}.
func UpdateAccountHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid account ID format")
			return
		}

		var req accountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Marshal allowed domains
		domains := req.AllowedDomains
		if domains == nil {
			domains = []string{}
		}
		domainsJSON, err := json.Marshal(domains)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		account, err := queries.UpdateAccount(r.Context(), storage.UpdateAccountParams{
			ID:             id,
			Name:           req.Name,
			Email:          req.Email,
			AllowedDomains: domainsJSON,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusOK, toAccountResponse(account, false))
	}
}

// DeleteAccountHandler handles DELETE /api/v1/accounts/{id}.
func DeleteAccountHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid account ID format")
			return
		}

		if err := queries.DeleteAccount(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
