package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestToUserResponse(t *testing.T) {
	now := time.Now()
	user := storage.User{
		ID:          uuid.New(),
		Email:       "user@example.com",
		AccountType: "user",
		Status:      "active",
		Username:    sql.NullString{String: "testuser", Valid: true},
		LastLogin:   pgtype.Timestamptz{Time: now, Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := toUserResponse(user)

	if resp.ID != user.ID {
		t.Errorf("ID = %v, want %v", resp.ID, user.ID)
	}
	if resp.Email != user.Email {
		t.Errorf("Email = %q, want %q", resp.Email, user.Email)
	}
	if resp.AccountType != user.AccountType {
		t.Errorf("AccountType = %q, want %q", resp.AccountType, user.AccountType)
	}
	if resp.Status != user.Status {
		t.Errorf("Status = %q, want %q", resp.Status, user.Status)
	}
	if resp.Username == nil || *resp.Username != "testuser" {
		t.Errorf("Username = %v, want testuser", resp.Username)
	}
	if resp.LastLogin == nil {
		t.Error("LastLogin should not be nil")
	}
}

func TestToUserResponse_NoLastLogin(t *testing.T) {
	now := time.Now()
	user := storage.User{
		ID:          uuid.New(),
		Email:       "user@example.com",
		AccountType: "user",
		Status:      "active",
		LastLogin:   pgtype.Timestamptz{Valid: false},
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}

	resp := toUserResponse(user)

	if resp.LastLogin != nil {
		t.Error("LastLogin should be nil when not set")
	}
}

func TestToUserResponseWithAPIKey(t *testing.T) {
	user := storage.User{
		ID:          uuid.New(),
		Email:       "smtp@example.com",
		AccountType: "smtp",
		Status:      "active",
		ApiKey:      sql.NullString{String: "test-api-key-123", Valid: true},
		CreatedAt:   pgtype.Timestamptz{Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Valid: true},
	}

	resp := toUserResponseWithAPIKey(user)

	if resp.ApiKey == nil || *resp.ApiKey != "test-api-key-123" {
		t.Errorf("ApiKey = %v, want test-api-key-123", resp.ApiKey)
	}
}

func TestValidRoles(t *testing.T) {
	tests := []struct {
		role  string
		valid bool
	}{
		{"owner", true},
		{"admin", true},
		{"member", true},
		{"superadmin", false},
		{"", false},
		{"OWNER", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			_, ok := validRoles[tt.role]
			if ok != tt.valid {
				t.Errorf("validRoles[%q] = %v, want %v", tt.role, ok, tt.valid)
			}
		})
	}
}

func TestCreateUserHandler_HumanUser(t *testing.T) {
	usr := testUser()
	groupID := testGroup().ID

	mock := &mockQuerier{
		createUserFn: func(ctx context.Context, arg storage.CreateUserParams) (storage.User, error) {
			if arg.Email != "new@example.com" {
				t.Errorf("expected email new@example.com, got %s", arg.Email)
			}
			if arg.AccountType != "user" {
				t.Errorf("expected account_type user, got %s", arg.AccountType)
			}
			return usr, nil
		},
		createGroupMemberFn: func(ctx context.Context, arg storage.CreateGroupMemberParams) (storage.GroupMember, error) {
			if arg.GroupID != groupID {
				t.Errorf("expected group ID %s, got %s", groupID, arg.GroupID)
			}
			if arg.Role != "member" {
				t.Errorf("expected role member, got %s", arg.Role)
			}
			return testGroupMember(), nil
		},
	}

	body := `{"email":"new@example.com","password":"pass123","account_type":"user","group_id":"` + groupID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateUserHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUserHandler_SMTPAccount(t *testing.T) {
	smtpUser := testUser()
	smtpUser.AccountType = "smtp"
	smtpUser.ApiKey = sql.NullString{String: "generated-api-key", Valid: true}

	mock := &mockQuerier{
		createUserFn: func(ctx context.Context, arg storage.CreateUserParams) (storage.User, error) {
			if arg.AccountType != "smtp" {
				t.Errorf("expected account_type smtp, got %s", arg.AccountType)
			}
			if !arg.ApiKey.Valid {
				t.Error("expected ApiKey to be set for smtp account")
			}
			return smtpUser, nil
		},
	}

	body := `{"email":"smtp@example.com","account_type":"smtp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	groupID := testGroup().ID
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateUserHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp userResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ApiKey == nil {
		t.Error("expected api_key in response for smtp account")
	}
}

func TestCreateUserHandler_MissingEmail(t *testing.T) {
	mock := &mockQuerier{}

	body := `{"password":"pass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	groupID := testGroup().ID
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateUserHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreateUserHandler_UserMissingPassword(t *testing.T) {
	mock := &mockQuerier{}

	body := `{"email":"test@example.com","account_type":"user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	groupID := testGroup().ID
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateUserHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreateUserHandler_OwnerCannotBeCreatedByAdmin(t *testing.T) {
	mock := &mockQuerier{}

	body := `{"email":"owner@example.com","password":"pass123","role":"owner"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	groupID := testGroup().ID
	// Caller is admin, not owner
	ctx := setJWTContext(req.Context(), testUser().ID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := CreateUserHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestListUsersHandler(t *testing.T) {
	usr := testUser()
	mock := &mockQuerier{
		listUsersFn: func(ctx context.Context) ([]storage.User, error) {
			return []storage.User{usr}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec := httptest.NewRecorder()

	handler := ListUsersHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []userResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp))
	}
}

func TestGetUserHandler_Found(t *testing.T) {
	usr := testUser()
	mock := &mockQuerier{
		getUserByIDFn: func(ctx context.Context, id uuid.UUID) (storage.User, error) {
			return usr, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+usr.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", usr.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetUserHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestGetUserHandler_NotFound(t *testing.T) {
	mock := &mockQuerier{
		getUserByIDFn: func(ctx context.Context, id uuid.UUID) (storage.User, error) {
			return storage.User{}, errNotFound
		},
	}

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := GetUserHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestUpdateUserStatusHandler(t *testing.T) {
	usr := testUser()
	mock := &mockQuerier{
		updateUserStatusFn: func(ctx context.Context, arg storage.UpdateUserStatusParams) (storage.User, error) {
			if arg.Status != "suspended" {
				t.Errorf("expected status suspended, got %s", arg.Status)
			}
			usr.Status = arg.Status
			return usr, nil
		},
	}

	body := `{"status":"suspended"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+usr.ID.String()+"/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", usr.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := UpdateUserStatusHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserStatusHandler_InvalidStatus(t *testing.T) {
	mock := &mockQuerier{}

	body := `{"status":"deleted"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+uuid.New().String()+"/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := UpdateUserStatusHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestDeleteUserHandler_Success(t *testing.T) {
	deleteCalled := false
	mock := &mockQuerier{
		deleteUserFn: func(ctx context.Context, id uuid.UUID) error {
			deleteCalled = true
			return nil
		},
	}

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+id.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := DeleteUserHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if !deleteCalled {
		t.Error("expected DeleteUser to be called")
	}
}
