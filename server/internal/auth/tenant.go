package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantContext returns an HTTP middleware that sets the PostgreSQL session variable
// app.current_tenant_id from the tenant ID in the request context.
// This enables Row Level Security (RLS) policies at the database level.
// Must be used after JWTAuth middleware.
func TenantContext(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := TenantFromContext(r.Context())
			if tenantID == uuid.Nil {
				http.Error(w, `{"error":"tenant context required"}`, http.StatusUnauthorized)
				return
			}

			if err := setTenantID(r.Context(), pool, tenantID); err != nil {
				http.Error(w, `{"error":"failed to set tenant context"}`, http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// setTenantID sets the app.current_tenant_id session variable on the database connection.
func setTenantID(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	_, err := pool.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_tenant_id = '%s'", tenantID.String()))
	return err
}
