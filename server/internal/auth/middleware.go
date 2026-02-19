package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	accountIDKey contextKey = "account_id"
	tenantIDKey  contextKey = "tenant_id"
	userIDKey    contextKey = "user_id"
	userEmailKey contextKey = "user_email"
	userRoleKey  contextKey = "user_role"
)

// AccountFromContext retrieves the authenticated account ID from the request context.
// Returns uuid.Nil if no account is set.
func AccountFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(accountIDKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// withAccountID stores the account ID in the request context.
func withAccountID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, accountIDKey, id)
}

// TenantFromContext retrieves the tenant ID from the request context.
// Returns uuid.Nil if no tenant is set.
func TenantFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(tenantIDKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// UserFromContext retrieves the user ID from the request context.
// Returns uuid.Nil if no user is set.
func UserFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(userIDKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// UserEmailFromContext retrieves the user email from the request context.
// Returns an empty string if no email is set.
func UserEmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value(userEmailKey).(string); ok {
		return email
	}
	return ""
}

// RoleFromContext retrieves the user role from the request context.
// Returns an empty string if no role is set.
func RoleFromContext(ctx context.Context) string {
	if role, ok := ctx.Value(userRoleKey).(string); ok {
		return role
	}
	return ""
}

// AccountLookupFunc is a function that looks up an account by API key.
// It returns the account ID if found, or an error if not.
type AccountLookupFunc func(ctx context.Context, apiKey string) (uuid.UUID, error)

// BearerAuth returns an HTTP middleware that validates Bearer token authentication.
// It extracts the API key from the Authorization header and looks up the account.
// On success, the account ID is stored in the request context.
func BearerAuth(lookup AccountLookupFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"authorization header required"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"error":"invalid authorization format, expected Bearer <token>"}`, http.StatusUnauthorized)
				return
			}

			apiKey := parts[1]
			if apiKey == "" {
				http.Error(w, `{"error":"empty API key"}`, http.StatusUnauthorized)
				return
			}

			accountID, err := lookup(r.Context(), apiKey)
			if err != nil {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}

			ctx := withAccountID(r.Context(), accountID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JWTAuth returns an HTTP middleware that validates JWT Bearer tokens.
// It extracts the JWT from the Authorization header, validates it,
// and injects user/tenant claims into the request context.
func JWTAuth(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"authorization header required"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"error":"invalid authorization format, expected Bearer <token>"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := parts[1]
			if tokenStr == "" {
				http.Error(w, `{"error":"empty token"}`, http.StatusUnauthorized)
				return
			}

			claims, err := jwtService.ValidateAccessToken(tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			tenantID, err := uuid.Parse(claims.TenantID)
			if err != nil {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDKey, userID)
			ctx = context.WithValue(ctx, tenantIDKey, tenantID)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)
			ctx = context.WithValue(ctx, userRoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
