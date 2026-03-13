package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestCreateRoutingRuleHandler_Valid(t *testing.T) {
	rule := testRoutingRule()
	groupID := testGroup().ID

	mock := &mockQuerier{
		createRoutingRuleFn: func(ctx context.Context, arg storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
			if arg.GroupID != groupID {
				t.Errorf("expected group ID %s, got %s", groupID, arg.GroupID)
			}
			if arg.Priority != 10 {
				t.Errorf("expected priority 10, got %d", arg.Priority)
			}
			return rule, nil
		},
	}

	providerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	body := `{"priority":10,"conditions":{"from":"*@test.com"},"provider_id":"` + providerID.String() + `","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing-rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateRoutingRuleHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp routingRuleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Priority != 10 {
		t.Errorf("expected priority 10, got %d", resp.Priority)
	}
	if resp.GroupID != groupID {
		t.Errorf("expected group_id %s, got %s", groupID, resp.GroupID)
	}
}

func TestCreateRoutingRuleHandler_InvalidProviderID(t *testing.T) {
	mock := &mockQuerier{}
	groupID := testGroup().ID

	body := `{"priority":10,"conditions":{},"provider_id":"not-a-uuid","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing-rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateRoutingRuleHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestListRoutingRulesHandler_OrderedByPriority(t *testing.T) {
	groupID := testGroup().ID

	rule1 := testRoutingRule()
	rule1.Priority = 1

	rule2 := testRoutingRule()
	rule2.ID = uuid.MustParse("00000000-0000-0000-0000-000000000004")
	rule2.Priority = 5

	rule3 := testRoutingRule()
	rule3.ID = uuid.MustParse("00000000-0000-0000-0000-000000000005")
	rule3.Priority = 10

	mock := &mockQuerier{
		listRoutingRulesByGroupFn: func(ctx context.Context, gID uuid.UUID) ([]storage.RoutingRule, error) {
			if gID != groupID {
				t.Errorf("expected group ID %s, got %s", groupID, gID)
			}
			// Return in priority order (as the SQL query does)
			return []storage.RoutingRule{rule1, rule2, rule3}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routing-rules", nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := ListRoutingRulesHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []routingRuleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(resp))
	}
	if resp[0].Priority != 1 {
		t.Errorf("expected first rule priority 1, got %d", resp[0].Priority)
	}
	if resp[1].Priority != 5 {
		t.Errorf("expected second rule priority 5, got %d", resp[1].Priority)
	}
	if resp[2].Priority != 10 {
		t.Errorf("expected third rule priority 10, got %d", resp[2].Priority)
	}
}

func TestGetRoutingRuleHandler_Found(t *testing.T) {
	rule := testRoutingRule()
	mock := &mockQuerier{
		getRoutingRuleByIDFn: func(ctx context.Context, id uuid.UUID) (storage.RoutingRule, error) {
			return rule, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routing-rules/"+rule.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", rule.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetRoutingRuleHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp routingRuleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID != rule.ID {
		t.Errorf("expected ID %s, got %s", rule.ID, resp.ID)
	}
}

func TestGetRoutingRuleHandler_NotFound(t *testing.T) {
	mock := &mockQuerier{
		getRoutingRuleByIDFn: func(ctx context.Context, id uuid.UUID) (storage.RoutingRule, error) {
			return storage.RoutingRule{}, errNotFound
		},
	}

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/routing-rules/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetRoutingRuleHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestUpdateRoutingRuleHandler(t *testing.T) {
	rule := testRoutingRule()
	mock := &mockQuerier{
		updateRoutingRuleFn: func(ctx context.Context, arg storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
			updated := rule
			updated.Priority = arg.Priority
			return updated, nil
		},
	}

	providerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	body := `{"priority":20,"conditions":{},"provider_id":"` + providerID.String() + `","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/routing-rules/"+rule.ID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", rule.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := UpdateRoutingRuleHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRoutingRuleHandler(t *testing.T) {
	id := uuid.New()
	deleteCalled := false
	mock := &mockQuerier{
		deleteRoutingRuleFn: func(ctx context.Context, delID uuid.UUID) error {
			deleteCalled = true
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/routing-rules/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := DeleteRoutingRuleHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if !deleteCalled {
		t.Error("expected delete to be called")
	}
}

func TestGetRoutingRuleHandler_InvalidUUID(t *testing.T) {
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routing-rules/bad-id", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetRoutingRuleHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
