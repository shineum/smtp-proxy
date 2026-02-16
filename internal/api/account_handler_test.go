package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/internal/storage"
)

func TestCreateAccountHandler_Valid(t *testing.T) {
	mock := &mockQuerier{
		createAccountFn: func(ctx context.Context, arg storage.CreateAccountParams) (storage.Account, error) {
			if arg.Name != "testuser" {
				t.Errorf("expected name testuser, got %s", arg.Name)
			}
			if arg.Email != "test@example.com" {
				t.Errorf("expected email test@example.com, got %s", arg.Email)
			}
			if arg.PasswordHash == "" {
				t.Error("expected non-empty password hash")
			}
			if arg.ApiKey == "" {
				t.Error("expected non-empty API key")
			}
			return testAccount(), nil
		},
	}

	body := `{"name":"testuser","email":"test@example.com","password":"secret123","allowed_domains":["example.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := CreateAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var resp accountResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "test-account" {
		t.Errorf("expected name test-account, got %s", resp.Name)
	}
	// API key should be included in creation response
	if resp.APIKey == "" {
		t.Error("expected api_key in creation response")
	}
}

func TestCreateAccountHandler_MissingFields(t *testing.T) {
	mock := &mockQuerier{}

	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"email":"a@b.com","password":"pass1234"}`},
		{"missing email", `{"name":"test","password":"pass1234"}`},
		{"missing password", `{"name":"test","email":"a@b.com"}`},
		{"all missing", `{}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler := CreateAccountHandler(mock)
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", rec.Code)
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp["error"] != "validation_failed" {
				t.Errorf("expected error validation_failed, got %v", resp["error"])
			}
		})
	}
}

func TestCreateAccountHandler_DuplicateName(t *testing.T) {
	mock := &mockQuerier{
		createAccountFn: func(ctx context.Context, arg storage.CreateAccountParams) (storage.Account, error) {
			return storage.Account{}, errors.New("duplicate key value")
		},
	}

	body := `{"name":"existing","email":"a@b.com","password":"pass1234"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := CreateAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Must NOT expose internal error details
	if resp["error"] != "internal server error" {
		t.Errorf("expected generic error, got %s", resp["error"])
	}
}

func TestGetAccountHandler_Found(t *testing.T) {
	acct := testAccount()
	mock := &mockQuerier{
		getAccountByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Account, error) {
			if id != acct.ID {
				t.Errorf("expected ID %s, got %s", acct.ID, id)
			}
			return acct, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acct.ID.String(), nil)
	rec := httptest.NewRecorder()

	// Set up chi URL parameter
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", acct.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp accountResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Must NOT expose api_key in get response
	if resp.APIKey != "" {
		t.Error("api_key should not be included in get response")
	}
	if resp.Name != acct.Name {
		t.Errorf("expected name %s, got %s", acct.Name, resp.Name)
	}
}

func TestGetAccountHandler_NotFound(t *testing.T) {
	mock := &mockQuerier{
		getAccountByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Account, error) {
			return storage.Account{}, errNotFound
		},
	}

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestGetAccountHandler_InvalidUUID(t *testing.T) {
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestUpdateAccountHandler(t *testing.T) {
	acct := testAccount()
	mock := &mockQuerier{
		updateAccountFn: func(ctx context.Context, arg storage.UpdateAccountParams) (storage.Account, error) {
			if arg.ID != acct.ID {
				t.Errorf("expected ID %s, got %s", acct.ID, arg.ID)
			}
			updated := acct
			updated.Name = arg.Name
			updated.Email = arg.Email
			return updated, nil
		},
	}

	body := `{"name":"updated-name","email":"updated@example.com","allowed_domains":["new.com"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/accounts/"+acct.ID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", acct.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := UpdateAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp accountResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Name != "updated-name" {
		t.Errorf("expected name updated-name, got %s", resp.Name)
	}
}

func TestDeleteAccountHandler(t *testing.T) {
	id := uuid.New()
	deleteCalled := false
	mock := &mockQuerier{
		deleteAccountFn: func(ctx context.Context, delID uuid.UUID) error {
			deleteCalled = true
			if delID != id {
				t.Errorf("expected ID %s, got %s", id, delID)
			}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := DeleteAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if !deleteCalled {
		t.Error("expected delete to be called")
	}
}

func TestCreateAccountHandler_InvalidJSON(t *testing.T) {
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := CreateAccountHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
