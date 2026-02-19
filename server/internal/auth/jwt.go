package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTConfig holds JWT signing and expiry configuration.
type JWTConfig struct {
	SigningKey         string        `mapstructure:"signing_key"`
	AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	Issuer             string        `mapstructure:"issuer"`
	Audience           string        `mapstructure:"audience"`
}

// AccessTokenClaims represents claims in an access token.
type AccessTokenClaims struct {
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// RefreshTokenClaims represents claims in a refresh token.
type RefreshTokenClaims struct {
	TenantID  string `json:"tenant_id"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token generation and validation.
type JWTService struct {
	config JWTConfig
}

// NewJWTService creates a new JWTService with the given configuration.
func NewJWTService(config JWTConfig) *JWTService {
	return &JWTService{config: config}
}

// Predefined errors for JWT operations.
var (
	ErrTokenExpired   = errors.New("token has expired")
	ErrTokenInvalid   = errors.New("token is invalid")
	ErrTokenMalformed = errors.New("token is malformed")
	ErrSigningMethod  = errors.New("unexpected signing method")
)

// GenerateAccessToken creates a signed JWT access token for the given user.
func (s *JWTService) GenerateAccessToken(userID, tenantID uuid.UUID, email, role string) (string, error) {
	now := time.Now()
	claims := AccessTokenClaims{
		TenantID: tenantID.String(),
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    s.config.Issuer,
			Audience:  jwt.ClaimStrings{s.config.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.AccessTokenExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.config.SigningKey))
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// GenerateRefreshToken creates a signed JWT refresh token for the given session.
func (s *JWTService) GenerateRefreshToken(userID, tenantID, sessionID uuid.UUID) (string, error) {
	now := time.Now()
	claims := RefreshTokenClaims{
		TenantID:  tenantID.String(),
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    s.config.Issuer,
			Audience:  jwt.ClaimStrings{s.config.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.RefreshTokenExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.config.SigningKey))
	if err != nil {
		return "", fmt.Errorf("sign refresh token: %w", err)
	}
	return signed, nil
}

// ValidateAccessToken parses and validates a JWT access token string.
// Returns the claims if valid, or an error if the token is expired, invalid, or malformed.
func (s *JWTService) ValidateAccessToken(tokenString string) (*AccessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrSigningMethod
		}
		return []byte(s.config.SigningKey), nil
	})
	if err != nil {
		return nil, classifyJWTError(err)
	}

	claims, ok := token.Claims.(*AccessTokenClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// ValidateRefreshToken parses and validates a JWT refresh token string.
// Returns the claims if valid, or an error if the token is expired, invalid, or malformed.
func (s *JWTService) ValidateRefreshToken(tokenString string) (*RefreshTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrSigningMethod
		}
		return []byte(s.config.SigningKey), nil
	})
	if err != nil {
		return nil, classifyJWTError(err)
	}

	claims, ok := token.Claims.(*RefreshTokenClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// classifyJWTError maps jwt library errors to domain-specific errors.
func classifyJWTError(err error) error {
	if errors.Is(err, jwt.ErrTokenExpired) {
		return ErrTokenExpired
	}
	if errors.Is(err, jwt.ErrTokenMalformed) {
		return ErrTokenMalformed
	}
	if errors.Is(err, jwt.ErrSignatureInvalid) {
		return ErrTokenInvalid
	}
	if errors.Is(err, ErrSigningMethod) {
		return ErrSigningMethod
	}
	return fmt.Errorf("validate token: %w", err)
}
