package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

type contextKey string

const (
	accountIDKey  contextKey = "account_id"
	groupIDKey    contextKey = "group_id"
	groupTypeKey  contextKey = "group_type"
	userIDKey     contextKey = "user_id"
	userEmailKey  contextKey = "user_email"
	userRoleKey   contextKey = "user_role"
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

// GroupIDFromContext retrieves the group ID from the request context.
// Returns uuid.Nil if no group is set.
func GroupIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(groupIDKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// GroupTypeFromContext retrieves the group type from the request context.
// Returns an empty string if no group type is set.
func GroupTypeFromContext(ctx context.Context) string {
	if gt, ok := ctx.Value(groupTypeKey).(string); ok {
		return gt
	}
	return ""
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

// UnifiedAuth returns an HTTP middleware that accepts EITHER JWT tokens OR API keys.
// For JWT tokens (detected by containing dots), it validates the token and extracts claims.
// For API keys, it looks up the user by API key, resolves their group membership,
// and sets the same context values as JWT auth.
// @MX:ANCHOR: [AUTO] Unified authentication entry point for all /api/v1/ routes
// @MX:REASON: fan_in >= 3; all authenticated API routes depend on this middleware
func UnifiedAuth(jwtService *JWTService, queries storage.Querier) func(http.Handler) http.Handler {
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

			token := parts[1]
			if token == "" {
				http.Error(w, `{"error":"empty token"}`, http.StatusUnauthorized)
				return
			}

			// Try JWT first if token contains dots (JWT format: header.payload.signature)
			if strings.Contains(token, ".") {
				claims, err := jwtService.ValidateAccessToken(token)
				if err == nil {
					userID, err := uuid.Parse(claims.Subject)
					if err != nil {
						http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
						return
					}
					groupID, err := uuid.Parse(claims.GroupID)
					if err != nil {
						http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
						return
					}
					ctx := r.Context()
					ctx = context.WithValue(ctx, userIDKey, userID)
					ctx = context.WithValue(ctx, groupIDKey, groupID)
					ctx = context.WithValue(ctx, groupTypeKey, claims.GroupType)
					ctx = context.WithValue(ctx, userEmailKey, claims.Email)
					ctx = context.WithValue(ctx, userRoleKey, claims.Role)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// JWT validation failed; fall through to API key check
			}

			// Try API key lookup
			user, err := queries.GetUserByAPIKey(r.Context(), sql.NullString{String: token, Valid: true})
			if err != nil {
				http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
				return
			}

			if user.Status != "active" {
				http.Error(w, `{"error":"account is not active"}`, http.StatusUnauthorized)
				return
			}

			// Resolve group membership
			groups, err := queries.ListGroupsByUserID(r.Context(), user.ID)
			if err != nil || len(groups) == 0 {
				http.Error(w, `{"error":"no group membership found"}`, http.StatusUnauthorized)
				return
			}

			// Use first group (SMTP accounts typically belong to one group)
			group := groups[0]

			// Get role from group membership
			member, err := queries.GetGroupMemberByUserAndGroup(r.Context(), storage.GetGroupMemberByUserAndGroupParams{
				UserID:  user.ID,
				GroupID: group.ID,
			})
			if err != nil {
				http.Error(w, `{"error":"failed to resolve group role"}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDKey, user.ID)
			ctx = context.WithValue(ctx, groupIDKey, group.ID)
			ctx = context.WithValue(ctx, groupTypeKey, group.GroupType)
			ctx = context.WithValue(ctx, userEmailKey, user.Email)
			ctx = context.WithValue(ctx, userRoleKey, member.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JWTAuth returns an HTTP middleware that validates JWT Bearer tokens.
// It extracts the JWT from the Authorization header, validates it,
// and injects user/group claims into the request context.
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

			groupID, err := uuid.Parse(claims.GroupID)
			if err != nil {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDKey, userID)
			ctx = context.WithValue(ctx, groupIDKey, groupID)
			ctx = context.WithValue(ctx, groupTypeKey, claims.GroupType)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)
			ctx = context.WithValue(ctx, userRoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
