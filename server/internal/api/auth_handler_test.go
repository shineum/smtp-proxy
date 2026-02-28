package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestHashToken(t *testing.T) {
	token := "test-token-value"

	hash1 := hashToken(token)
	hash2 := hashToken(token)

	if hash1 == "" {
		t.Error("hashToken() returned empty string")
	}

	if hash1 != hash2 {
		t.Errorf("hashToken() not deterministic: %q != %q", hash1, hash2)
	}

	// Different tokens should produce different hashes
	hash3 := hashToken("different-token")
	if hash1 == hash3 {
		t.Error("hashToken() returned same hash for different tokens")
	}

	// Hash should be a 64-character hex string (SHA-256)
	if len(hash1) != 64 {
		t.Errorf("hashToken() length = %d, want 64", len(hash1))
	}
}

func TestLoginHandler_Success(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	groupID := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	hash, _ := auth.HashPassword("password123")
	user := storage.User{
		ID:           userID,
		Email:        "test@example.com",
		PasswordHash: hash,
		Status:       "active",
		AccountType:  "user",
	}

	grp := storage.Group{
		ID:        groupID,
		Name:      "test-group",
		GroupType: "organization",
		Status:    "active",
	}

	member := storage.GroupMember{
		ID:      uuid.New(),
		GroupID:  groupID,
		UserID:  userID,
		Role:    "admin",
	}

	mock := &mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (storage.User, error) {
			if email != "test@example.com" {
				t.Errorf("expected email test@example.com, got %s", email)
			}
			return user, nil
		},
		listGroupsByUserIDFn: func(ctx context.Context, uid uuid.UUID) ([]storage.Group, error) {
			return []storage.Group{grp}, nil
		},
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			return member, nil
		},
		createSessionFn: func(ctx context.Context, arg storage.CreateSessionParams) (storage.Session, error) {
			if arg.GroupID != groupID {
				t.Errorf("expected session GroupID %s, got %s", groupID, arg.GroupID)
			}
			return storage.Session{ID: uuid.New()}, nil
		},
	}

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LoginHandler(mock, jwtSvc, nil, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp tokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh_token")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected token_type Bearer, got %s", resp.TokenType)
	}
}

func TestLoginHandler_WithGroupID(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	groupID := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	hash, _ := auth.HashPassword("password123")
	user := storage.User{
		ID:           userID,
		Email:        "test@example.com",
		PasswordHash: hash,
		Status:       "active",
		AccountType:  "user",
	}

	grp := storage.Group{
		ID:        groupID,
		Name:      "test-group",
		GroupType: "organization",
		Status:    "active",
	}

	member := storage.GroupMember{
		ID:      uuid.New(),
		GroupID:  groupID,
		UserID:  userID,
		Role:    "owner",
	}

	mock := &mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (storage.User, error) {
			return user, nil
		},
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			if arg.GroupID != groupID {
				t.Errorf("expected group ID %s, got %s", groupID, arg.GroupID)
			}
			return member, nil
		},
		getGroupByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Group, error) {
			return grp, nil
		},
		createSessionFn: func(ctx context.Context, arg storage.CreateSessionParams) (storage.Session, error) {
			return storage.Session{ID: uuid.New()}, nil
		},
	}

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"email":"test@example.com","password":"password123","group_id":"` + groupID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LoginHandler(mock, jwtSvc, nil, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	mock := &mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (storage.User, error) {
			return storage.User{}, errNotFound
		},
	}

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"email":"wrong@example.com","password":"bad"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LoginHandler(mock, jwtSvc, nil, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestLoginHandler_InactiveUser(t *testing.T) {
	hash, _ := auth.HashPassword("password123")
	user := storage.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: hash,
		Status:       "suspended",
		AccountType:  "user",
	}

	mock := &mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (storage.User, error) {
			return user, nil
		},
	}

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LoginHandler(mock, jwtSvc, nil, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestLoginHandler_NoGroupMembership(t *testing.T) {
	hash, _ := auth.HashPassword("password123")
	user := storage.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: hash,
		Status:       "active",
		AccountType:  "user",
	}

	mock := &mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (storage.User, error) {
			return user, nil
		},
		listGroupsByUserIDFn: func(ctx context.Context, uid uuid.UUID) ([]storage.Group, error) {
			return []storage.Group{}, nil // No groups
		},
	}

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LoginHandler(mock, jwtSvc, nil, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginHandler_MissingFields(t *testing.T) {
	mock := &mockQuerier{}
	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LoginHandler(mock, jwtSvc, nil, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestRefreshHandler_Success(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	groupID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	sessionID := uuid.New()

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})
	refreshToken, _ := jwtSvc.GenerateRefreshToken(userID, groupID, sessionID)
	refreshHash := hashToken(refreshToken)

	session := storage.Session{
		ID:               sessionID,
		UserID:           userID,
		GroupID:          groupID,
		RefreshTokenHash: refreshHash,
		ExpiresAt:        pgtype.Timestamptz{Valid: false}, // No expiry set
	}

	user := storage.User{
		ID:          userID,
		Email:       "test@example.com",
		Status:      "active",
		AccountType: "user",
	}

	grp := storage.Group{
		ID:        groupID,
		GroupType: "organization",
	}

	member := storage.GroupMember{
		ID:      uuid.New(),
		GroupID:  groupID,
		UserID:  userID,
		Role:    "admin",
	}

	mock := &mockQuerier{
		getSessionByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Session, error) {
			return session, nil
		},
		getUserByIDFn: func(ctx context.Context, id uuid.UUID) (storage.User, error) {
			return user, nil
		},
		getGroupByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Group, error) {
			return grp, nil
		},
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			return member, nil
		},
	}

	body := `{"refresh_token":"` + refreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := RefreshHandler(mock, jwtSvc, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp tokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
}

func TestRefreshHandler_InvalidToken(t *testing.T) {
	mock := &mockQuerier{}
	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"refresh_token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := RefreshHandler(mock, jwtSvc, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestLogoutHandler_Success(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	groupID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	sessionID := uuid.New()

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})
	refreshToken, _ := jwtSvc.GenerateRefreshToken(userID, groupID, sessionID)

	deleteCalled := false
	mock := &mockQuerier{
		deleteSessionFn: func(ctx context.Context, id uuid.UUID) error {
			deleteCalled = true
			if id != sessionID {
				t.Errorf("expected session ID %s, got %s", sessionID, id)
			}
			return nil
		},
	}

	body := `{"refresh_token":"` + refreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LogoutHandler(mock, jwtSvc, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if !deleteCalled {
		t.Error("expected DeleteSession to be called")
	}
}

func TestLogoutHandler_InvalidToken(t *testing.T) {
	mock := &mockQuerier{}
	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"refresh_token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := LogoutHandler(mock, jwtSvc, nil)
	handler.ServeHTTP(rec, req)

	// Even with invalid token, logout should succeed (204)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}

func TestSwitchGroupHandler_Success(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	currentGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	targetGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000020")

	user := storage.User{
		ID:          userID,
		Email:       "test@example.com",
		Status:      "active",
		AccountType: "user",
	}

	targetGroup := storage.Group{
		ID:        targetGroupID,
		Name:      "target-group",
		GroupType: "organization",
		Status:    "active",
	}

	member := storage.GroupMember{
		ID:      uuid.New(),
		GroupID:  targetGroupID,
		UserID:  userID,
		Role:    "member",
	}

	mock := &mockQuerier{
		getUserByIDFn: func(ctx context.Context, id uuid.UUID) (storage.User, error) {
			return user, nil
		},
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			if arg.GroupID != targetGroupID {
				t.Errorf("expected target group ID %s, got %s", targetGroupID, arg.GroupID)
			}
			return member, nil
		},
		getGroupByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Group, error) {
			return targetGroup, nil
		},
		createSessionFn: func(ctx context.Context, arg storage.CreateSessionParams) (storage.Session, error) {
			if arg.GroupID != targetGroupID {
				t.Errorf("expected session GroupID %s, got %s", targetGroupID, arg.GroupID)
			}
			return storage.Session{ID: uuid.New()}, nil
		},
	}

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"group_id":"` + targetGroupID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/switch-group", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setJWTContext(req.Context(), userID, currentGroupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler := SwitchGroupHandler(mock, jwtSvc, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp tokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
}

func TestSwitchGroupHandler_NotMember(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	currentGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	targetGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000020")

	user := storage.User{
		ID:          userID,
		Email:       "test@example.com",
		Status:      "active",
		AccountType: "user",
	}

	mock := &mockQuerier{
		getUserByIDFn: func(ctx context.Context, id uuid.UUID) (storage.User, error) {
			return user, nil
		},
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			return storage.GroupMember{}, errNotFound
		},
	}

	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{"group_id":"` + targetGroupID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/switch-group", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setJWTContext(req.Context(), userID, currentGroupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler := SwitchGroupHandler(mock, jwtSvc, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestSwitchGroupHandler_MissingGroupID(t *testing.T) {
	mock := &mockQuerier{}
	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/switch-group", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	groupID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	ctx := setJWTContext(req.Context(), userID, groupID, "admin", "organization")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler := SwitchGroupHandler(mock, jwtSvc, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
