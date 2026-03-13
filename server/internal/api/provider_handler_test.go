package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// adminMemberFn returns a mock function that resolves the caller as an admin.
func adminMemberFn() func(context.Context, storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
	return func(_ context.Context, _ storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
		return storage.GroupMember{Role: "admin"}, nil
	}
}

func TestCreateProviderHandler_Valid(t *testing.T) {
	prov := testProvider()
	groupID := testGroup().ID

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: adminMemberFn(),
		createProviderFn: func(ctx context.Context, arg storage.CreateProviderParams) (storage.EspProvider, error) {
			if arg.GroupID != groupID {
				t.Errorf("expected group ID %d, got %d", groupID, arg.GroupID)
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
		t.Errorf("expected group_id %d, got %d", groupID, resp.GroupID)
	}
}

func TestCreateProviderHandler_InvalidType(t *testing.T) {
	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: adminMemberFn(),
	}
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
		listAccessibleProvidersFn: func(ctx context.Context, gID int32) ([]storage.EspProvider, error) {
			if gID != groupID {
				t.Errorf("expected group ID %d, got %d", groupID, gID)
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
	groupID := testGroup().ID

	mock := &mockQuerier{
		getProviderByIDFn: func(ctx context.Context, id int32) (storage.EspProvider, error) {
			return prov, nil
		},
		isProviderAccessibleFn: func(arg storage.IsProviderAccessibleParams) (bool, error) {
			return true, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+int32ToStr(prov.ID), nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(prov.ID))
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := GetProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestGetProviderHandler_NotFound(t *testing.T) {
	groupID := testGroup().ID
	mock := &mockQuerier{
		getProviderByIDFn: func(ctx context.Context, id int32) (storage.EspProvider, error) {
			return storage.EspProvider{}, errNotFound
		},
	}

	var id int32 = 999
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+int32ToStr(id), nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(id))
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := GetProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestUpdateProviderHandler(t *testing.T) {
	prov := testProvider()
	groupID := testGroup().ID

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: adminMemberFn(),
		getProviderByIDFn: func(ctx context.Context, id int32) (storage.EspProvider, error) {
			return prov, nil
		},
		updateProviderFn: func(ctx context.Context, arg storage.UpdateProviderParams) (storage.EspProvider, error) {
			updated := prov
			updated.Name = arg.Name
			return updated, nil
		},
	}

	body := `{"name":"updated-provider","provider_type":"mailgun","enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/providers/"+int32ToStr(prov.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(prov.ID))
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := UpdateProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestProviderUsageHandler_Success(t *testing.T) {
	prov := testProvider()
	groupID := testGroup().ID

	mock := &mockQuerier{
		getProviderByIDFn: func(_ context.Context, id int32) (storage.EspProvider, error) {
			return prov, nil
		},
		listUsersByProviderIDFn: func(_ context.Context, _ pgtype.Int4) ([]storage.ListUsersByProviderIDRow, error) {
			return []storage.ListUsersByProviderIDRow{
				{
					ID:          99,
					Email:       "user@example.com",
					AccountType: "user",
					Role:        "member",
					GroupID:     groupID,
					GroupName:   "test-group",
				},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+int32ToStr(prov.ID)+"/usage", nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(prov.ID))
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := ProviderUsageHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp []struct {
		UserID    int32  `json:"user_id"`
		Email     string `json:"email"`
		GroupName string `json:"group_name"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 usage row, got %d", len(resp))
	}
	if resp[0].Email != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", resp[0].Email)
	}
}

func TestProviderUsageHandler_InvalidID(t *testing.T) {
	groupID := testGroup().ID

	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/not-a-number/usage", nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-number")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := ProviderUsageHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestProviderUsageHandler_EmptyResult(t *testing.T) {
	prov := testProvider()
	groupID := testGroup().ID

	mock := &mockQuerier{
		getProviderByIDFn: func(_ context.Context, _ int32) (storage.EspProvider, error) {
			return prov, nil
		},
		listUsersByProviderIDFn: func(_ context.Context, _ pgtype.Int4) ([]storage.ListUsersByProviderIDRow, error) {
			return []storage.ListUsersByProviderIDRow{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+int32ToStr(prov.ID)+"/usage", nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(prov.ID))
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := ProviderUsageHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected 0 usage rows, got %d", len(resp))
	}
}

func TestProviderUsageHandler_Unauthorized(t *testing.T) {
	mock := &mockQuerier{}

	var id int32 = 999
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+int32ToStr(id)+"/usage", nil)
	// No JWT context set

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(id))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := ProviderUsageHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestProviderUsageHandler_Forbidden(t *testing.T) {
	prov := testProvider()
	var otherGroupID int32 = 99

	mock := &mockQuerier{
		getProviderByIDFn: func(_ context.Context, _ int32) (storage.EspProvider, error) {
			return prov, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+int32ToStr(prov.ID)+"/usage", nil)
	ctx := setJWTContext(req.Context(), testUser().ID, otherGroupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(prov.ID))
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := ProviderUsageHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestDeleteProviderHandler(t *testing.T) {
	prov := testProvider()
	groupID := testGroup().ID
	deleteCalled := false

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: adminMemberFn(),
		getProviderByIDFn: func(ctx context.Context, id int32) (storage.EspProvider, error) {
			return prov, nil
		},
		deleteProviderFn: func(ctx context.Context, delID int32) error {
			deleteCalled = true
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/providers/"+int32ToStr(prov.ID), nil)
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", int32ToStr(prov.ID))
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := DeleteProviderHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if !deleteCalled {
		t.Error("expected delete to be called")
	}
}
