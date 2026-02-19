package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// loginRequest is the JSON body for POST /api/v1/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// tokenResponse is the JSON response containing access and refresh tokens.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// refreshRequest is the JSON body for POST /api/v1/auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LoginHandler handles POST /api/v1/auth/login.
// Authenticates a user by email and password, creates a session, and returns JWT tokens.
func LoginHandler(queries storage.Querier, jwtService *auth.JWTService, auditLogger *auth.AuditLogger, rateLimiter *auth.RateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Email == "" || req.Password == "" {
			respondError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		// Check login rate limit
		if rateLimiter != nil {
			if err := rateLimiter.CheckLoginRateLimit(r.Context(), req.Email); err != nil {
				if auditLogger != nil {
					auditLogger.LogAuthFailure(r.Context(), r, auth.AuditActionLoginFailed, "rate limited")
				}
				respondError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
				return
			}
		}

		// Look up user by email
		user, err := queries.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			if rateLimiter != nil {
				_ = rateLimiter.RecordFailedLogin(r.Context(), req.Email)
			}
			if auditLogger != nil {
				auditLogger.LogAuthFailure(r.Context(), r, auth.AuditActionLoginFailed, "user not found")
			}
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		// Check user status
		if user.Status != "active" {
			if auditLogger != nil {
				auditLogger.LogAuthFailure(r.Context(), r, auth.AuditActionLoginFailed, "user inactive")
			}
			respondError(w, http.StatusUnauthorized, "account is not active")
			return
		}

		// Verify password
		if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
			_ = queries.IncrementFailedAttempts(r.Context(), user.ID)
			if rateLimiter != nil {
				_ = rateLimiter.RecordFailedLogin(r.Context(), req.Email)
			}
			if auditLogger != nil {
				auditLogger.LogAuthFailure(r.Context(), r, auth.AuditActionLoginFailed, "invalid password")
			}
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		// Create session
		sessionID := uuid.New()
		accessToken, err := jwtService.GenerateAccessToken(user.ID, user.TenantID, user.Email, user.Role)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		refreshToken, err := jwtService.GenerateRefreshToken(user.ID, user.TenantID, sessionID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Hash refresh token for storage
		refreshHash := hashToken(refreshToken)

		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		_, err = queries.CreateSession(r.Context(), storage.CreateSessionParams{
			UserID:           user.ID,
			TenantID:         user.TenantID,
			RefreshTokenHash: refreshHash,
			ExpiresAt:        pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Update last login
		_ = queries.UpdateUserLastLogin(r.Context(), user.ID)

		// Clear failed login attempts
		if rateLimiter != nil {
			_ = rateLimiter.ClearFailedLogins(r.Context(), req.Email)
		}

		// Audit log
		if auditLogger != nil {
			auditLogger.LogAuthAttempt(r.Context(), r, user.TenantID, user.ID, auth.AuditActionLogin)
		}

		respondJSON(w, http.StatusOK, tokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			TokenType:    "Bearer",
			ExpiresIn:    900, // 15 minutes in seconds
		})
	}
}

// RefreshHandler handles POST /api/v1/auth/refresh.
// Validates the refresh token and returns a new access token.
func RefreshHandler(queries storage.Querier, jwtService *auth.JWTService, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.RefreshToken == "" {
			respondError(w, http.StatusBadRequest, "refresh_token is required")
			return
		}

		// Validate the refresh token JWT
		claims, err := jwtService.ValidateRefreshToken(req.RefreshToken)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}

		sessionID, err := uuid.Parse(claims.SessionID)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}

		// Look up session
		session, err := queries.GetSessionByID(r.Context(), sessionID)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "session not found or expired")
			return
		}

		// Verify refresh token hash matches
		refreshHash := hashToken(req.RefreshToken)
		if session.RefreshTokenHash != refreshHash {
			respondError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}

		// Check session expiry
		if session.ExpiresAt.Valid && session.ExpiresAt.Time.Before(time.Now()) {
			_ = queries.DeleteSession(r.Context(), sessionID)
			respondError(w, http.StatusUnauthorized, "session expired")
			return
		}

		// Get user for fresh claims
		user, err := queries.GetUserByID(r.Context(), session.UserID)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "user not found")
			return
		}

		if user.Status != "active" {
			respondError(w, http.StatusUnauthorized, "account is not active")
			return
		}

		// Generate new access token
		accessToken, err := jwtService.GenerateAccessToken(user.ID, user.TenantID, user.Email, user.Role)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if auditLogger != nil {
			auditLogger.LogAuthAttempt(r.Context(), r, user.TenantID, user.ID, auth.AuditActionTokenRefresh)
		}

		respondJSON(w, http.StatusOK, tokenResponse{
			AccessToken:  accessToken,
			RefreshToken: req.RefreshToken, // Return the same refresh token
			TokenType:    "Bearer",
			ExpiresIn:    900,
		})
	}
}

// LogoutHandler handles POST /api/v1/auth/logout.
// Invalidates the user's session.
func LogoutHandler(queries storage.Querier, jwtService *auth.JWTService, auditLogger *auth.AuditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.RefreshToken == "" {
			respondError(w, http.StatusBadRequest, "refresh_token is required")
			return
		}

		// Validate the refresh token to get the session ID
		claims, err := jwtService.ValidateRefreshToken(req.RefreshToken)
		if err != nil {
			// Even if the token is invalid/expired, we return success
			w.WriteHeader(http.StatusNoContent)
			return
		}

		sessionID, err := uuid.Parse(claims.SessionID)
		if err != nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Delete the session
		_ = queries.DeleteSession(r.Context(), sessionID)

		if auditLogger != nil {
			userID := auth.UserFromContext(r.Context())
			tenantID := auth.TenantFromContext(r.Context())
			if userID != uuid.Nil {
				auditLogger.LogAuthAttempt(r.Context(), r, tenantID, userID, auth.AuditActionLogout)
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// hashToken creates a SHA-256 hash of a token string for secure storage.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
