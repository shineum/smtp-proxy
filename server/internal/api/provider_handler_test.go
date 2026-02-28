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

func TestCreateProviderHandler_Valid(t *testing.T) {
	prov := testProvider()
	groupID := testGroup().ID

	mock := &mockQuerier{
		createProviderFn: func(ctx context.Context, arg storage.CreateProviderParams) (storage.EspProvider, error) {
			if arg.GroupID != groupID {
				t.Errorf("expected group ID %s, got %s", groupID, arg.GroupID)
			}
			if arg.Name != "my-sendgrid" {
				t.Errorf("expected name my-sendgrid, got %s", arg.Name)
			}
			return prov, nil
		},
	}

	body := `{"name":"my-sendgrid","provider_type":"sendgrid","api_key":"sg-key","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp providerResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Name != prov.Name {
		t.Errorf("expected name %s, got %s", prov.Name, resp.Name)
	}
	if resp.GroupID != groupID {
		t.Errorf("expected group_id %s, got %s", groupID, resp.GroupID)
	}
}

func TestCreateProviderHandler_InvalidType(t *testing.T) {
	mock := &mockQuerier{}
	groupID := testGroup().ID

	body := `{"name":"bad","provider_type":"invalid_type","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestListProvidersHandler_FilteredByGroup(t *testing.T) {
	groupID := testGroup().ID
	prov := testProvider()

	mock := &mockQuerier{
		listProvidersByGroupFn: func(ctx context.Context, gID uuid.UUID) ([]storage.EspProvider, error) {
			if gID != groupID {
				t.Errorf("expected group ID %s, got %s", groupID, gID)
			}
			return []storage.EspProvider{prov}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := ListProvidersHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []providerResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(resp))
	}
	if resp[0].Name != prov.Name {
		t.Errorf("expected name %s, got %s", prov.Name, resp[0].Name)
	}
}

func TestGetProviderHandler_Found(t *testing.T) {
	prov := testProvider()
	mock := &mockQuerier{
		getProviderByIDFn: func(ctx context.Context, id uuid.UUID) (storage.EspProvider, error) {
			return prov, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+prov.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", prov.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestGetProviderHandler_NotFound(t *testing.T) {
	mock := &mockQuerier{
		getProviderByIDFn: func(ctx context.Context, id uuid.UUID) (storage.EspProvider, error) {
			return storage.EspProvider{}, errNotFound
		},
	}

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestUpdateProviderHandler(t *testing.T) {
	prov := testProvider()
	mock := &mockQuerier{
		updateProviderFn: func(ctx context.Context, arg storage.UpdateProviderParams) (storage.EspProvider, error) {
			updated := prov
			updated.Name = arg.Name
			return updated, nil
		},
	}

	body := `{"name":"updated-provider","provider_type":"mailgun","enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/providers/"+prov.ID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", prov.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := UpdateProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteProviderHandler(t *testing.T) {
	id := uuid.New()
	deleteCalled := false
	mock := &mockQuerier{
		deleteProviderFn: func(ctx context.Context, delID uuid.UUID) error {
			deleteCalled = true
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/providers/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := DeleteProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if !deleteCalled {
		t.Error("expected delete to be called")
	}
}
