package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	Visibility   string          `json:"visibility"`
}

// providerResponse is the JSON response for a provider.
type providerResponse struct {
	ID           int32           `json:"id"`
	GroupID      int32           `json:"group_id"`
	Name         string          `json:"name"`
	ProviderType string          `json:"provider_type"`
	SMTPConfig   json.RawMessage `json:"smtp_config"`
	Enabled      bool            `json:"enabled"`
	Visibility   string          `json:"visibility"`
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
		GroupID:      p.GroupID,
		Name:         p.Name,
		ProviderType: string(p.ProviderType),
		SMTPConfig:   smtpConfig,
		Enabled:      p.Enabled,
		Visibility:   string(p.Visibility),
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
	"stdout":   storage.ProviderTypeStdout,
}

var validVisibilities = map[string]storage.ProviderVisibility{
	"private": storage.ProviderVisibilityPrivate,
	"shared":  storage.ProviderVisibilityShared,
	"global":  storage.ProviderVisibilityGlobal,
}

// CreateProviderHandler handles POST /api/v1/providers.
// Creates a new ESP provider for the authenticated user's group.
func CreateProviderHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// Require owner or admin role for provider management
		if _, err := requireGroupRole(queries, r, groupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
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

		// Determine visibility: default to private, only system admin can set global
		visibility := storage.ProviderVisibilityPrivate
		if req.Visibility != "" {
			v, ok := validVisibilities[req.Visibility]
			if !ok {
				respondError(w, http.StatusBadRequest, "invalid visibility: must be private, shared, or global")
				return
			}
			if v == storage.ProviderVisibilityGlobal {
				callerGroupType := auth.GroupTypeFromContext(r.Context())
				if callerGroupType != "system" {
					respondError(w, http.StatusForbidden, "only system admins can create global providers")
					return
				}
			}
			visibility = v
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
			GroupID:      groupID,
			Name:         req.Name,
			ProviderType: pt,
			ApiKey:       apiKey,
			SmtpConfig:   smtpConfig,
			Enabled:      req.Enabled,
			Visibility:   visibility,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusCreated, toProviderResponse(provider))
	}
}

// ListProvidersHandler handles GET /api/v1/providers.
// Lists all providers accessible to a group. When ?group_id= is provided,
// lists providers accessible to that group (admin use case); otherwise
// defaults to the authenticated user's own group.
func ListProvidersHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// Allow overriding group_id via query param (for admin editing service accounts).
		if qg := r.URL.Query().Get("group_id"); qg != "" {
			parsed, err := strconv.ParseInt(qg, 10, 32)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid group_id format")
				return
			}
			groupID = int32(parsed)
		}

		providers, err := queries.ListAccessibleProviders(r.Context(), groupID)
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
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		idStr := chi.URLParam(r, "id")
		parsed, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		id := int32(parsed)

		provider, err := queries.GetProviderByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}

		// Check access: owner group or accessible via visibility
		accessible, err := queries.IsProviderAccessible(r.Context(), storage.IsProviderAccessibleParams{
			ID:      id,
			GroupID: groupID,
		})
		if err != nil || !accessible {
			respondError(w, http.StatusForbidden, "access denied to this provider")
			return
		}

		respondJSON(w, http.StatusOK, toProviderResponse(provider))
	}
}

// UpdateProviderHandler handles PUT /api/v1/providers/{id}.
func UpdateProviderHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		idStr := chi.URLParam(r, "id")
		parsed, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		id := int32(parsed)

		// Only the owning group can update a provider
		existing, err := queries.GetProviderByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if existing.GroupID != groupID && callerGroupType != "system" {
			respondError(w, http.StatusForbidden, "only the owning group or system admin can update a provider")
			return
		}

		// Require owner or admin role
		if _, err := requireGroupRole(queries, r, existing.GroupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
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

		// Determine visibility
		visibility := existing.Visibility
		if req.Visibility != "" {
			v, ok := validVisibilities[req.Visibility]
			if !ok {
				respondError(w, http.StatusBadRequest, "invalid visibility: must be private, shared, or global")
				return
			}
			if v == storage.ProviderVisibilityGlobal && callerGroupType != "system" {
				respondError(w, http.StatusForbidden, "only system admins can set global visibility")
				return
			}
			visibility = v
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
			Visibility:   visibility,
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
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		idStr := chi.URLParam(r, "id")
		parsed, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		id := int32(parsed)

		// Only the owning group can delete a provider
		existing, err := queries.GetProviderByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if existing.GroupID != groupID && callerGroupType != "system" {
			respondError(w, http.StatusForbidden, "only the owning group or system admin can delete a provider")
			return
		}

		// Require owner or admin role
		if _, err := requireGroupRole(queries, r, existing.GroupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		if err := queries.DeleteProvider(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ListProviderAccessHandler handles GET /api/v1/providers/{id}/access.
// Lists groups that have been granted access to a shared provider.
func ListProviderAccessHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		idStr := chi.URLParam(r, "id")
		parsed, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		id := int32(parsed)

		// Only the owning group or system admin can view access list
		provider, err := queries.GetProviderByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if provider.GroupID != groupID && callerGroupType != "system" {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		accessList, err := queries.ListProviderAccess(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusOK, accessList)
	}
}

type grantAccessRequest struct {
	GroupID string `json:"group_id"`
}

// GrantProviderAccessHandler handles POST /api/v1/providers/{id}/access.
// Grants a group access to a shared provider.
func GrantProviderAccessHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		idStr := chi.URLParam(r, "id")
		parsedID, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		providerID := int32(parsedID)

		// Only the owning group's admin/owner or system admin can grant access
		provider, err := queries.GetProviderByID(r.Context(), providerID)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if provider.GroupID != groupID && callerGroupType != "system" {
			respondError(w, http.StatusForbidden, "only the owning group or system admin can grant access")
			return
		}
		if _, err := requireGroupRole(queries, r, provider.GroupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		var req grantAccessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		parsedGID, err := strconv.ParseInt(req.GroupID, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group_id format")
			return
		}
		targetGroupID := int32(parsedGID)

		callerUserID := auth.UserFromContext(r.Context())
		if err := queries.GrantProviderAccess(r.Context(), storage.GrantProviderAccessParams{
			ProviderID: providerID,
			GroupID:    targetGroupID,
			GrantedBy:  pgtype.Int4{Int32: callerUserID, Valid: callerUserID != 0},
		}); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ProviderUsageHandler handles GET /api/v1/providers/{id}/usage.
// Returns the list of group-user combinations using this provider.
func ProviderUsageHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		idStr := chi.URLParam(r, "id")
		parsed, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		id := int32(parsed)

		// Only the owning group or system admin can view usage
		provider, err := queries.GetProviderByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if provider.GroupID != groupID && callerGroupType != "system" {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}

		rows, err := queries.ListUsersByProviderID(r.Context(), pgtype.Int4{Int32: id, Valid: true})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		type usageRow struct {
			UserID      int32  `json:"user_id"`
			Email       string `json:"email"`
			AccountType string `json:"account_type"`
			Role        string `json:"role"`
			GroupID     int32  `json:"group_id"`
			GroupName   string `json:"group_name"`
		}

		result := make([]usageRow, len(rows))
		for i, row := range rows {
			result[i] = usageRow{
				UserID:      row.ID,
				Email:       row.Email,
				AccountType: row.AccountType,
				Role:        row.Role,
				GroupID:     row.GroupID,
				GroupName:   row.GroupName,
			}
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// RevokeProviderAccessHandler handles DELETE /api/v1/providers/{id}/access/{groupId}.
// Revokes a group's access to a shared provider.
func RevokeProviderAccessHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		parsedPID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		providerID := int32(parsedPID)

		parsedGID, err := strconv.ParseInt(chi.URLParam(r, "groupId"), 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid group ID format")
			return
		}
		targetGroupID := int32(parsedGID)

		// Only the owning group's admin/owner or system admin can revoke access
		provider, err := queries.GetProviderByID(r.Context(), providerID)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		callerGroupType := auth.GroupTypeFromContext(r.Context())
		if provider.GroupID != groupID && callerGroupType != "system" {
			respondError(w, http.StatusForbidden, "only the owning group or system admin can revoke access")
			return
		}
		if _, err := requireGroupRole(queries, r, provider.GroupID, "owner", "admin"); err != nil {
			respondError(w, http.StatusForbidden, "owner or admin role required")
			return
		}

		if err := queries.RevokeProviderAccess(r.Context(), storage.RevokeProviderAccessParams{
			ProviderID: providerID,
			GroupID:    targetGroupID,
		}); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
