package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// newTestJWTService creates a JWTService with test configuration.
func newTestJWTService() *auth.JWTService {
	return auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})
}

// --- Test 1: Auth Lifecycle ---

func TestAuthLifecycle_LoginSwitchRefreshLogout(t *testing.T) {
	t.Parallel()

	var userID int32 = 1
	var systemGroupID int32 = 10
	var companyGroupID int32 = 20

	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	user := storage.User{
		ID:           userID,
		Email:        "lifecycle@example.com",
		PasswordHash: hash,
		Status:       "active",
		AccountType:  "user",
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	systemGroup := storage.Group{
		ID:        systemGroupID,
		Name:      "system-group",
		GroupType: "system",
		Status:    "active",
	}

	companyGroup := storage.Group{
		ID:        companyGroupID,
		Name:      "company-group",
		GroupType: "company",
		Status:    "active",
	}

	systemMember := storage.GroupMember{
		GroupID:  systemGroupID,
		UserID:  userID,
		Role:    "admin",
	}

	companyMember := storage.GroupMember{
		GroupID:  companyGroupID,
		UserID:  userID,
		Role:    "member",
	}

	var lastCreatedSession storage.Session
	var deletedSessionIDs []int32

	var sessionCounter int32

	mock := &mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (storage.User, error) {
			if email == user.Email {
				return user, nil
			}
			return storage.User{}, errNotFound
		},
		getUserByIDFn: func(ctx context.Context, id int32) (storage.User, error) {
			if id == userID {
				return user, nil
			}
			return storage.User{}, errNotFound
		},
		listGroupsByUserIDFn: func(ctx context.Context, uid int32) ([]storage.Group, error) {
			return []storage.Group{systemGroup, companyGroup}, nil
		},
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			switch arg.GroupID {
			case systemGroupID:
				return systemMember, nil
			case companyGroupID:
				return companyMember, nil
			}
			return storage.GroupMember{}, errNotFound
		},
		getGroupByIDFn: func(ctx context.Context, id int32) (storage.Group, error) {
			switch id {
			case systemGroupID:
				return systemGroup, nil
			case companyGroupID:
				return companyGroup, nil
			}
			return storage.Group{}, errNotFound
		},
		createSessionFn: func(ctx context.Context, arg storage.CreateSessionParams) (storage.Session, error) {
			sessionCounter++
			lastCreatedSession = storage.Session{
				ID:               sessionCounter,
				UserID:           arg.UserID,
				GroupID:          arg.GroupID,
				RefreshTokenHash: arg.RefreshTokenHash,
				ExpiresAt:        arg.ExpiresAt,
			}
			return lastCreatedSession, nil
		},
		getSessionByIDFn: func(ctx context.Context, id int32) (storage.Session, error) {
			s := lastCreatedSession
			s.ID = id
			return s, nil
		},
		updateSessionRefreshTokenFn: func(ctx context.Context, arg storage.UpdateSessionRefreshTokenParams) error {
			lastCreatedSession.RefreshTokenHash = arg.RefreshTokenHash
			return nil
		},
		deleteSessionFn: func(ctx context.Context, id int32) error {
			deletedSessionIDs = append(deletedSessionIDs, id)
			return nil
		},
	}

	jwtSvc := newTestJWTService()

	// Step 1: Login with email/password
	loginBody := `{"email":"lifecycle@example.com","password":"password123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()

	LoginHandler(mock, jwtSvc, nil, nil).ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("Step 1 (Login): expected 200, got %d; body: %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp tokenResponse
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("Step 1 (Login): failed to decode response: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatal("Step 1 (Login): access_token is empty")
	}
	if loginResp.RefreshToken == "" {
		t.Fatal("Step 1 (Login): refresh_token is empty")
	}

	// Verify access token claims (should have system group context since it's the first group)
	claims, err := jwtSvc.ValidateAccessToken(loginResp.AccessToken)
	if err != nil {
		t.Fatalf("Step 1 (Login): failed to validate access token: %v", err)
	}
	if claims.Subject != strconv.FormatInt(int64(userID), 10) {
		t.Errorf("Step 1 (Login): expected subject %d, got %s", userID, claims.Subject)
	}
	if claims.GroupID != strconv.FormatInt(int64(systemGroupID), 10) {
		t.Errorf("Step 1 (Login): expected group_id %d, got %s", systemGroupID, claims.GroupID)
	}
	if claims.Role != "admin" {
		t.Errorf("Step 1 (Login): expected role admin, got %s", claims.Role)
	}
	if claims.GroupType != "system" {
		t.Errorf("Step 1 (Login): expected group_type system, got %s", claims.GroupType)
	}

	// Step 2: Switch group to company
	switchBody := `{"group_id":"` + int32ToStr(companyGroupID) + `"}`
	switchReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/switch-group", strings.NewReader(switchBody))
	switchReq.Header.Set("Content-Type", "application/json")

	// Set JWT context from the login token
	switchCtx := setJWTContext(switchReq.Context(), userID, systemGroupID, "admin", "system")
	switchReq = switchReq.WithContext(switchCtx)

	switchRec := httptest.NewRecorder()
	SwitchGroupHandler(mock, jwtSvc, nil).ServeHTTP(switchRec, switchReq)

	if switchRec.Code != http.StatusOK {
		t.Fatalf("Step 2 (Switch): expected 200, got %d; body: %s", switchRec.Code, switchRec.Body.String())
	}

	var switchResp tokenResponse
	if err := json.NewDecoder(switchRec.Body).Decode(&switchResp); err != nil {
		t.Fatalf("Step 2 (Switch): failed to decode response: %v", err)
	}

	// Verify switched token has company group context
	switchClaims, err := jwtSvc.ValidateAccessToken(switchResp.AccessToken)
	if err != nil {
		t.Fatalf("Step 2 (Switch): failed to validate access token: %v", err)
	}
	if switchClaims.GroupID != strconv.FormatInt(int64(companyGroupID), 10) {
		t.Errorf("Step 2 (Switch): expected group_id %d, got %s", companyGroupID, switchClaims.GroupID)
	}
	if switchClaims.GroupType != "company" {
		t.Errorf("Step 2 (Switch): expected group_type company, got %s", switchClaims.GroupType)
	}
	if switchClaims.Role != "member" {
		t.Errorf("Step 2 (Switch): expected role member, got %s", switchClaims.Role)
	}

	// Step 3: Refresh token
	refreshBody := `{"refresh_token":"` + switchResp.RefreshToken + `"}`
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()

	RefreshHandler(mock, jwtSvc, nil).ServeHTTP(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Fatalf("Step 3 (Refresh): expected 200, got %d; body: %s", refreshRec.Code, refreshRec.Body.String())
	}

	var refreshResp tokenResponse
	if err := json.NewDecoder(refreshRec.Body).Decode(&refreshResp); err != nil {
		t.Fatalf("Step 3 (Refresh): failed to decode response: %v", err)
	}
	if refreshResp.AccessToken == "" {
		t.Fatal("Step 3 (Refresh): access_token is empty")
	}

	// Verify refreshed token still has company group context
	refreshClaims, err := jwtSvc.ValidateAccessToken(refreshResp.AccessToken)
	if err != nil {
		t.Fatalf("Step 3 (Refresh): failed to validate access token: %v", err)
	}
	if refreshClaims.GroupID != strconv.FormatInt(int64(companyGroupID), 10) {
		t.Errorf("Step 3 (Refresh): expected group_id %d, got %s", companyGroupID, refreshClaims.GroupID)
	}

	// Step 4: Logout
	logoutBody := `{"refresh_token":"` + switchResp.RefreshToken + `"}`
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(logoutBody))
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutRec := httptest.NewRecorder()

	LogoutHandler(mock, jwtSvc, nil).ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("Step 4 (Logout): expected 204, got %d", logoutRec.Code)
	}
	if len(deletedSessionIDs) == 0 {
		t.Fatal("Step 4 (Logout): expected at least one session deleted")
	}
}

// --- Test 2: Group Management Access Control ---

func TestGroupManagement_SystemAdminOnly(t *testing.T) {
	t.Parallel()

	var userID int32 = 1
	var systemGroupID int32 = 10
	var companyGroupID int32 = 20
	var companyGroupToDelete int32 = 30

	systemGroup := storage.Group{
		ID:        systemGroupID,
		Name:      "system-group",
		GroupType: "system",
		Status:    "active",
	}

	companyGroup := storage.Group{
		ID:        companyGroupToDelete,
		Name:      "deletable-company",
		GroupType: "company",
		Status:    "active",
	}

	createdGroup := storage.Group{
		ID:        40,
		Name:      "new-group",
		GroupType: "company",
		Status:    "active",
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	mock := &mockQuerier{
		createGroupFn: func(ctx context.Context, arg storage.CreateGroupParams) (storage.Group, error) {
			return createdGroup, nil
		},
		getGroupByIDFn: func(ctx context.Context, id int32) (storage.Group, error) {
			switch id {
			case systemGroupID:
				return systemGroup, nil
			case companyGroupToDelete:
				return companyGroup, nil
			}
			return storage.Group{}, errNotFound
		},
		updateGroupStatusFn: func(ctx context.Context, arg storage.UpdateGroupStatusParams) (storage.Group, error) {
			return companyGroup, nil
		},
		listGroupMembersByGroupIDFn: func(ctx context.Context, groupID int32) ([]storage.GroupMember, error) {
			return nil, nil
		},
	}

	t.Run("AnyUser_CanCreateGroup", func(t *testing.T) {
		t.Parallel()

		body := `{"name":"new-group"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Company group user (not system admin) can now create groups
		ctx := setJWTContext(req.Context(), userID, companyGroupID, "admin", "company")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()

		handler := CreateGroupHandler(mock, nil)
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("SystemAdmin_CanCreateGroup", func(t *testing.T) {
		t.Parallel()

		body := `{"name":"new-group"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// System admin user
		ctx := setJWTContext(req.Context(), userID, systemGroupID, "admin", "system")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()

		handler := CreateGroupHandler(mock, nil)
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DeleteSystemGroup_Forbidden", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+int32ToStr(systemGroupID), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", int32ToStr(systemGroupID))
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = setJWTContext(ctx, userID, systemGroupID, "admin", "system")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		DeleteGroupHandler(mock, nil).ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DeleteCompanyGroup_Success", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+int32ToStr(companyGroupToDelete), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", int32ToStr(companyGroupToDelete))
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = setJWTContext(ctx, userID, systemGroupID, "admin", "system")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		DeleteGroupHandler(mock, nil).ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})
}

// --- Test 3: Unified Auth Middleware ---

func TestUnifiedAuth_JWTAndAPIKey(t *testing.T) {
	t.Parallel()

	jwtSvc := newTestJWTService()

	var userID int32 = 1
	var groupID int32 = 10

	smtpUser := storage.User{
		ID:          userID,
		Email:       "smtp@example.com",
		Status:      "active",
		AccountType: "smtp",
		ApiKey:      sql.NullString{String: "testapikey123456", Valid: true},
	}

	group := storage.Group{
		ID:        groupID,
		Name:      "test-group",
		GroupType: "company",
		Status:    "active",
	}

	member := storage.GroupMember{
		GroupID:  groupID,
		UserID:  userID,
		Role:    "member",
	}

	mock := &mockQuerier{
		getUserByAPIKeyFn: func(ctx context.Context, apiKey sql.NullString) (storage.User, error) {
			if apiKey.String == smtpUser.ApiKey.String {
				return smtpUser, nil
			}
			return storage.User{}, errNotFound
		},
		listGroupsByUserIDFn: func(ctx context.Context, uid int32) ([]storage.Group, error) {
			if uid == userID {
				return []storage.Group{group}, nil
			}
			return nil, nil
		},
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			if arg.UserID == userID && arg.GroupID == groupID {
				return member, nil
			}
			return storage.GroupMember{}, errNotFound
		},
	}

	// Handler that captures context values set by middleware
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserFromContext(r.Context())
		gid := auth.GroupIDFromContext(r.Context())
		gt := auth.GroupTypeFromContext(r.Context())
		role := auth.RoleFromContext(r.Context())

		respondJSON(w, http.StatusOK, map[string]string{
			"user_id":    strconv.FormatInt(int64(uid), 10),
			"group_id":   strconv.FormatInt(int64(gid), 10),
			"group_type": gt,
			"role":       role,
		})
	})

	middleware := auth.UnifiedAuth(jwtSvc, mock)
	handler := middleware(captureHandler)

	t.Run("ValidJWT_Succeeds", func(t *testing.T) {
		t.Parallel()

		token, err := jwtSvc.GenerateAccessToken(userID, groupID, "test@example.com", "admin", "system")
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["user_id"] != strconv.FormatInt(int64(userID), 10) {
			t.Errorf("expected user_id %d, got %s", userID, resp["user_id"])
		}
		if resp["group_type"] != "system" {
			t.Errorf("expected group_type system, got %s", resp["group_type"])
		}
	})

	t.Run("ValidAPIKey_Succeeds", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
		req.Header.Set("Authorization", "Bearer "+smtpUser.ApiKey.String)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["user_id"] != strconv.FormatInt(int64(userID), 10) {
			t.Errorf("expected user_id %d, got %s", userID, resp["user_id"])
		}
		if resp["group_id"] != strconv.FormatInt(int64(groupID), 10) {
			t.Errorf("expected group_id %d, got %s", groupID, resp["group_id"])
		}
		if resp["group_type"] != "company" {
			t.Errorf("expected group_type company, got %s", resp["group_type"])
		}
		if resp["role"] != "member" {
			t.Errorf("expected role member, got %s", resp["role"])
		}
	})

	t.Run("InvalidToken_Returns401", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("NoAuth_Returns401", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})
}

// --- Test 4: SMTP Account Creation Flow ---

func TestSMTPAccountCreation_WithGroupMembership(t *testing.T) {
	t.Parallel()

	var callerUserID int32 = 1
	var groupID int32 = 10
	var providerID int32 = 99

	var createdUser storage.User
	var createdMembership bool

	mock := &mockQuerier{
		createUserFn: func(ctx context.Context, arg storage.CreateUserParams) (storage.User, error) {
			if arg.AccountType != "smtp" {
				t.Errorf("expected account_type smtp, got %s", arg.AccountType)
			}
			if !arg.ApiKey.Valid {
				t.Error("expected API key to be generated for SMTP account")
			}
			if arg.ApiKey.String == "" {
				t.Error("expected non-empty API key")
			}

			createdUser = storage.User{
				ID:          5,
				Email:       arg.Email,
				AccountType: arg.AccountType,
				Status:      "active",
				ApiKey:      arg.ApiKey,
				ProviderID:  arg.ProviderID,
				CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			return createdUser, nil
		},
		createGroupMemberFn: func(ctx context.Context, arg storage.CreateGroupMemberParams) (storage.GroupMember, error) {
			if arg.GroupID != groupID {
				t.Errorf("expected group_id %d, got %d", groupID, arg.GroupID)
			}
			createdMembership = true
			return storage.GroupMember{
				GroupID:  arg.GroupID,
				UserID:  arg.UserID,
				Role:    arg.Role,
			}, nil
		},
		getUserByAPIKeyFn: func(ctx context.Context, apiKey sql.NullString) (storage.User, error) {
			if apiKey.Valid && apiKey.String == createdUser.ApiKey.String {
				return createdUser, nil
			}
			return storage.User{}, errNotFound
		},
		getProviderByIDFn: func(ctx context.Context, id int32) (storage.EspProvider, error) {
			if id == providerID {
				return storage.EspProvider{
					ID:      providerID,
					GroupID: groupID,
					Enabled: true,
				}, nil
			}
			return storage.EspProvider{}, errNotFound
		},
	}

	// Step 1: Create SMTP user with group_id and provider_id
	body := `{"email":"smtp-bot@example.com","account_type":"smtp","username":"smtp-bot","group_id":"` + int32ToStr(groupID) + `","provider_id":"` + int32ToStr(providerID) + `","role":"member"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Set caller context as system admin (can create in any group)
	ctx := setJWTContext(req.Context(), callerUserID, groupID, "admin", "system")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	CreateUserHandler(mock, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Step 1 (Create): expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp userResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Step 1 (Create): failed to decode response: %v", err)
	}

	// Step 2: Verify response includes api_key
	if resp.ApiKey == nil {
		t.Fatal("Step 2 (Verify): expected api_key in response for SMTP account")
	}
	if *resp.ApiKey == "" {
		t.Fatal("Step 2 (Verify): expected non-empty api_key")
	}
	if resp.AccountType != "smtp" {
		t.Errorf("Step 2 (Verify): expected account_type smtp, got %s", resp.AccountType)
	}

	// Step 3: Verify the SMTP user can be looked up by API key
	lookedUpUser, err := mock.GetUserByAPIKey(context.Background(), sql.NullString{String: *resp.ApiKey, Valid: true})
	if err != nil {
		t.Fatalf("Step 3 (Lookup): failed to look up user by API key: %v", err)
	}
	if lookedUpUser.Email != "smtp-bot@example.com" {
		t.Errorf("Step 3 (Lookup): expected email smtp-bot@example.com, got %s", lookedUpUser.Email)
	}

	// Step 4: Verify group membership was created
	if !createdMembership {
		t.Fatal("Step 4 (Membership): expected group membership to be created")
	}
}

// --- Test 5: Member Role Protection (Last Owner) ---

func TestMemberRoleProtection_LastOwner(t *testing.T) {
	t.Parallel()

	var groupID int32 = 10
	var ownerUserID int32 = 1
	var secondOwnerID int32 = 2

	ownerMember := storage.GroupMember{
		GroupID:  groupID,
		UserID:  ownerUserID,
		Role:    "owner",
	}

	ownerCount := int64(1) // Start with single owner

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			if arg.UserID == ownerUserID && arg.GroupID == groupID {
				return ownerMember, nil
			}
			if arg.UserID == secondOwnerID && arg.GroupID == groupID {
				return storage.GroupMember{
					GroupID:  groupID,
					UserID:  secondOwnerID,
					Role:    "owner",
				}, nil
			}
			return storage.GroupMember{}, errNotFound
		},
		countGroupOwnersFn: func(ctx context.Context, gid int32) (int64, error) {
			return ownerCount, nil
		},
		updateGroupMemberRoleFn: func(ctx context.Context, arg storage.UpdateGroupMemberRoleParams) (storage.GroupMember, error) {
			m := ownerMember
			m.Role = arg.Role
			return m, nil
		},
		deleteGroupMemberFn: func(ctx context.Context, arg storage.DeleteGroupMemberParams) error {
			return nil
		},
	}

	var systemGroupID int32 = 99

	t.Run("DemoteLastOwner_Conflict", func(t *testing.T) {
		t.Parallel()

		body := `{"role":"member"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/groups/"+int32ToStr(groupID)+"/members/"+int32ToStr(ownerUserID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", int32ToStr(groupID))
		rctx.URLParams.Add("uid", int32ToStr(ownerUserID))
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = setJWTContext(ctx, ownerUserID, systemGroupID, "admin", "system")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		UpdateGroupMemberRoleHandler(mock, nil).ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("RemoveLastOwner_Conflict", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+int32ToStr(groupID)+"/members/"+int32ToStr(ownerUserID), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", int32ToStr(groupID))
		rctx.URLParams.Add("uid", int32ToStr(ownerUserID))
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = setJWTContext(ctx, ownerUserID, systemGroupID, "admin", "system")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		RemoveGroupMemberHandler(mock, nil).ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DemoteOwner_WithSecondOwner_Success", func(t *testing.T) {
		// Use a separate mock with 2 owners
		twoOwnerMock := &mockQuerier{
			getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
				if arg.UserID == ownerUserID && arg.GroupID == groupID {
					return ownerMember, nil
				}
				return storage.GroupMember{}, errNotFound
			},
			countGroupOwnersFn: func(ctx context.Context, gid int32) (int64, error) {
				return 2, nil // Two owners now
			},
			updateGroupMemberRoleFn: func(ctx context.Context, arg storage.UpdateGroupMemberRoleParams) (storage.GroupMember, error) {
				m := ownerMember
				m.Role = arg.Role
				return m, nil
			},
		}

		body := `{"role":"member"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/groups/"+int32ToStr(groupID)+"/members/"+int32ToStr(ownerUserID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", int32ToStr(groupID))
		rctx.URLParams.Add("uid", int32ToStr(ownerUserID))
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = setJWTContext(ctx, ownerUserID, systemGroupID, "admin", "system")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		UpdateGroupMemberRoleHandler(twoOwnerMock, nil).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}

		var resp groupMemberResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Role != "member" {
			t.Errorf("expected role member, got %s", resp.Role)
		}
	})
}
