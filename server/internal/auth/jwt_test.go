package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newTestJWTService() *JWTService {
	return NewJWTService(JWTConfig{
		SigningKey:         "test-secret-key-at-least-32-chars!",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "smtp-proxy-test",
		Audience:           "smtp-proxy-api",
	})
}

func TestGenerateAccessToken(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	groupID := uuid.New()

	token, err := svc.GenerateAccessToken(userID, groupID, "user@example.com", "admin", "organization")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateAccessToken() returned empty token")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	groupID := uuid.New()
	sessionID := uuid.New()

	token, err := svc.GenerateRefreshToken(userID, groupID, sessionID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateRefreshToken() returned empty token")
	}
}

func TestValidateAccessToken_Valid(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	groupID := uuid.New()
	email := "user@example.com"
	role := "admin"

	token, err := svc.GenerateAccessToken(userID, groupID, email, role, "organization")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
	if claims.GroupID != groupID.String() {
		t.Errorf("GroupID = %q, want %q", claims.GroupID, groupID.String())
	}
	if claims.GroupType != "organization" {
		t.Errorf("GroupType = %q, want %q", claims.GroupType, "organization")
	}
	if claims.Email != email {
		t.Errorf("Email = %q, want %q", claims.Email, email)
	}
	if claims.Role != role {
		t.Errorf("Role = %q, want %q", claims.Role, role)
	}
	if claims.Issuer != "smtp-proxy-test" {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, "smtp-proxy-test")
	}
}

func TestValidateAccessToken_Expired(t *testing.T) {
	svc := NewJWTService(JWTConfig{
		SigningKey:         "test-secret-key-at-least-32-chars!",
		AccessTokenExpiry:  -1 * time.Hour, // already expired
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "smtp-proxy-test",
		Audience:           "smtp-proxy-api",
	})

	token, err := svc.GenerateAccessToken(uuid.New(), uuid.New(), "user@example.com", "member", "organization")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	_, err = svc.ValidateAccessToken(token)
	if err != ErrTokenExpired {
		t.Errorf("ValidateAccessToken() error = %v, want %v", err, ErrTokenExpired)
	}
}

func TestValidateAccessToken_InvalidSignature(t *testing.T) {
	svc := newTestJWTService()

	token, err := svc.GenerateAccessToken(uuid.New(), uuid.New(), "user@example.com", "member", "organization")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	// Validate with a different signing key
	otherSvc := NewJWTService(JWTConfig{
		SigningKey:         "completely-different-signing-key!!",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "smtp-proxy-test",
		Audience:           "smtp-proxy-api",
	})

	_, err = otherSvc.ValidateAccessToken(token)
	if err == nil {
		t.Error("ValidateAccessToken() expected error for invalid signature")
	}
}

func TestValidateAccessToken_Malformed(t *testing.T) {
	svc := newTestJWTService()

	_, err := svc.ValidateAccessToken("not-a-jwt-token")
	if err != ErrTokenMalformed {
		t.Errorf("ValidateAccessToken() error = %v, want %v", err, ErrTokenMalformed)
	}
}

func TestValidateRefreshToken_Valid(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	groupID := uuid.New()
	sessionID := uuid.New()

	token, err := svc.GenerateRefreshToken(userID, groupID, sessionID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	claims, err := svc.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() error = %v", err)
	}

	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
	if claims.GroupID != groupID.String() {
		t.Errorf("GroupID = %q, want %q", claims.GroupID, groupID.String())
	}
	if claims.SessionID != sessionID.String() {
		t.Errorf("SessionID = %q, want %q", claims.SessionID, sessionID.String())
	}
}

func TestValidateRefreshToken_Expired(t *testing.T) {
	svc := NewJWTService(JWTConfig{
		SigningKey:         "test-secret-key-at-least-32-chars!",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: -1 * time.Hour, // already expired
		Issuer:             "smtp-proxy-test",
		Audience:           "smtp-proxy-api",
	})

	token, err := svc.GenerateRefreshToken(uuid.New(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	_, err = svc.ValidateRefreshToken(token)
	if err != ErrTokenExpired {
		t.Errorf("ValidateRefreshToken() error = %v, want %v", err, ErrTokenExpired)
	}
}

func TestValidateAccessToken_WrongSigningMethod(t *testing.T) {
	// Create a token with a different signing method (none)
	claims := AccessTokenClaims{
		GroupID:   uuid.New().String(),
		GroupType: "organization",
		Email:     "user@example.com",
		Role:      "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to sign token with none method: %v", err)
	}

	svc := newTestJWTService()
	_, err = svc.ValidateAccessToken(tokenStr)
	if err == nil {
		t.Error("ValidateAccessToken() expected error for wrong signing method")
	}
}

func TestGenerateAccessToken_ClaimsExtraction(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	groupID := uuid.New()

	token, _ := svc.GenerateAccessToken(userID, groupID, "test@test.com", "owner", "organization")
	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	// Verify audience
	aud, _ := claims.GetAudience()
	if len(aud) != 1 || aud[0] != "smtp-proxy-api" {
		t.Errorf("Audience = %v, want [smtp-proxy-api]", aud)
	}

	// Verify expiry is in the future
	exp, _ := claims.GetExpirationTime()
	if exp == nil || exp.Time.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}

	// Verify issued at is not in the future
	iat, _ := claims.GetIssuedAt()
	if iat == nil || iat.Time.After(time.Now().Add(1*time.Second)) {
		t.Error("IssuedAt should not be in the future")
	}
}
