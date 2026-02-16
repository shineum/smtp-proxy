package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/internal/auth"
	"github.com/sungwon/smtp-proxy/internal/storage"
)

// routingRuleRequest is the JSON body for creating or updating a routing rule.
type routingRuleRequest struct {
	Priority   int32           `json:"priority"`
	Conditions json.RawMessage `json:"conditions"`
	ProviderID string          `json:"provider_id"`
	Enabled    bool            `json:"enabled"`
}

// routingRuleResponse is the JSON response for a routing rule.
type routingRuleResponse struct {
	ID         uuid.UUID       `json:"id"`
	AccountID  uuid.UUID       `json:"account_id"`
	Priority   int32           `json:"priority"`
	Conditions json.RawMessage `json:"conditions"`
	ProviderID uuid.UUID       `json:"provider_id"`
	Enabled    bool            `json:"enabled"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

// toRoutingRuleResponse converts a storage.RoutingRule to a routingRuleResponse.
func toRoutingRuleResponse(rr storage.RoutingRule) routingRuleResponse {
	conditions := json.RawMessage(rr.Conditions)
	if len(conditions) == 0 {
		conditions = json.RawMessage(`{}`)
	}

	return routingRuleResponse{
		ID:         rr.ID,
		AccountID:  rr.AccountID,
		Priority:   rr.Priority,
		Conditions: conditions,
		ProviderID: rr.ProviderID,
		Enabled:    rr.Enabled,
		CreatedAt:  timestampToTime(rr.CreatedAt).Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  timestampToTime(rr.UpdatedAt).Format("2006-01-02T15:04:05Z07:00"),
	}
}

// CreateRoutingRuleHandler handles POST /api/v1/routing-rules.
func CreateRoutingRuleHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := auth.AccountFromContext(r.Context())
		if accountID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req routingRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		providerID, err := uuid.Parse(req.ProviderID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider_id format")
			return
		}

		conditions := []byte("{}")
		if len(req.Conditions) > 0 {
			conditions = req.Conditions
		}

		rule, err := queries.CreateRoutingRule(r.Context(), storage.CreateRoutingRuleParams{
			AccountID:  accountID,
			Priority:   req.Priority,
			Conditions: conditions,
			ProviderID: providerID,
			Enabled:    req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusCreated, toRoutingRuleResponse(rule))
	}
}

// ListRoutingRulesHandler handles GET /api/v1/routing-rules.
// Rules are returned ordered by priority ASC (as defined by the SQL query).
func ListRoutingRulesHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := auth.AccountFromContext(r.Context())
		if accountID == uuid.Nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		rules, err := queries.ListRoutingRulesByAccountID(r.Context(), accountID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		result := make([]routingRuleResponse, len(rules))
		for i, rr := range rules {
			result[i] = toRoutingRuleResponse(rr)
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// GetRoutingRuleHandler handles GET /api/v1/routing-rules/{id}.
func GetRoutingRuleHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid routing rule ID format")
			return
		}

		rule, err := queries.GetRoutingRuleByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "routing rule not found")
			return
		}

		respondJSON(w, http.StatusOK, toRoutingRuleResponse(rule))
	}
}

// UpdateRoutingRuleHandler handles PUT /api/v1/routing-rules/{id}.
func UpdateRoutingRuleHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid routing rule ID format")
			return
		}

		var req routingRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		providerID, err := uuid.Parse(req.ProviderID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider_id format")
			return
		}

		conditions := []byte("{}")
		if len(req.Conditions) > 0 {
			conditions = req.Conditions
		}

		rule, err := queries.UpdateRoutingRule(r.Context(), storage.UpdateRoutingRuleParams{
			ID:         id,
			Priority:   req.Priority,
			Conditions: conditions,
			ProviderID: providerID,
			Enabled:    req.Enabled,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		respondJSON(w, http.StatusOK, toRoutingRuleResponse(rule))
	}
}

// DeleteRoutingRuleHandler handles DELETE /api/v1/routing-rules/{id}.
func DeleteRoutingRuleHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid routing rule ID format")
			return
		}

		if err := queries.DeleteRoutingRule(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
