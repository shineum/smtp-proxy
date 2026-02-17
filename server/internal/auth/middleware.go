package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const accountIDKey contextKey = "account_id"

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
