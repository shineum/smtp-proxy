package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GroupContext returns an HTTP middleware that sets the PostgreSQL session variable
// app.current_group_id from the group ID in the request context.
// This enables Row Level Security (RLS) policies at the database level.
// Must be used after JWTAuth middleware.
func GroupContext(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			groupID := GroupIDFromContext(r.Context())
			if groupID == uuid.Nil {
				http.Error(w, `{"error":"group context required"}`, http.StatusUnauthorized)
				return
			}

			if err := setGroupID(r.Context(), pool, groupID); err != nil {
				http.Error(w, `{"error":"failed to set group context"}`, http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// setGroupID sets the app.current_group_id session variable on the database connection.
func setGroupID(ctx context.Context, pool *pgxpool.Pool, groupID uuid.UUID) error {
	_, err := pool.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_group_id = '%s'", groupID.String()))
	return err
}
