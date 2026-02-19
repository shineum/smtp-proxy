package api

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// RouterConfig holds dependencies for the router.
type RouterConfig struct {
	Queries     storage.Querier
	DB          *storage.DB
	Log         zerolog.Logger
	DLQ         *queue.DLQ
	JWTService  *auth.JWTService
	AuditLogger *auth.AuditLogger
	RateLimiter *auth.RateLimiter
}

// NewRouter creates a chi.Mux with all routes, middleware, and handlers configured.
// The dlq parameter is optional; when nil, DLQ reprocess endpoints are not registered.
func NewRouter(queries storage.Querier, db *storage.DB, log zerolog.Logger, dlq *queue.DLQ) *chi.Mux {
	return NewRouterWithConfig(RouterConfig{
		Queries: queries,
		DB:      db,
		Log:     log,
		DLQ:     dlq,
	})
}

// NewRouterWithConfig creates a chi.Mux with all routes using the full RouterConfig.
// Supports both legacy API key auth and new JWT-based multi-tenant auth.
func NewRouterWithConfig(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(CorrelationIDMiddleware)
	r.Use(LoggingMiddleware(cfg.Log))
	r.Use(RecoverMiddleware(cfg.Log))

	// Health endpoints (no auth required)
	r.Get("/healthz", HealthzHandler())
	r.Get("/readyz", ReadyzHandler(cfg.DB))

	// Account creation endpoint (no auth required, legacy)
	r.Post("/api/v1/accounts", CreateAccountHandler(cfg.Queries))

	// Webhook endpoints (no auth required - called by ESP providers)
	r.Post("/api/v1/webhooks/sendgrid", SendGridWebhookHandler(cfg.Queries))
	r.Post("/api/v1/webhooks/ses", SESWebhookHandler(cfg.Queries))
	r.Post("/api/v1/webhooks/mailgun", MailgunWebhookHandler(cfg.Queries))

	// Multi-tenant auth endpoints (no auth required)
	if cfg.JWTService != nil {
		r.Post("/api/v1/tenants", CreateTenantHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger))
		r.Post("/api/v1/auth/login", LoginHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger, cfg.RateLimiter))
		r.Post("/api/v1/auth/refresh", RefreshHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger))
		r.Post("/api/v1/auth/logout", LogoutHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger))
	}

	// Legacy API key auth routes
	accountLookup := func(ctx context.Context, apiKey string) (uuid.UUID, error) {
		account, err := cfg.Queries.GetAccountByAPIKey(ctx, apiKey)
		if err != nil {
			return uuid.Nil, err
		}
		return account.ID, nil
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.BearerAuth(accountLookup))

		// Accounts (CRUD except create which is above without auth)
		r.Get("/accounts/{id}", GetAccountHandler(cfg.Queries))
		r.Put("/accounts/{id}", UpdateAccountHandler(cfg.Queries))
		r.Delete("/accounts/{id}", DeleteAccountHandler(cfg.Queries))

		// Providers
		r.Post("/providers", CreateProviderHandler(cfg.Queries))
		r.Get("/providers", ListProvidersHandler(cfg.Queries))
		r.Get("/providers/{id}", GetProviderHandler(cfg.Queries))
		r.Put("/providers/{id}", UpdateProviderHandler(cfg.Queries))
		r.Delete("/providers/{id}", DeleteProviderHandler(cfg.Queries))

		// Routing Rules
		r.Post("/routing-rules", CreateRoutingRuleHandler(cfg.Queries))
		r.Get("/routing-rules", ListRoutingRulesHandler(cfg.Queries))
		r.Get("/routing-rules/{id}", GetRoutingRuleHandler(cfg.Queries))
		r.Put("/routing-rules/{id}", UpdateRoutingRuleHandler(cfg.Queries))
		r.Delete("/routing-rules/{id}", DeleteRoutingRuleHandler(cfg.Queries))

		// DLQ Reprocess
		if cfg.DLQ != nil {
			r.Post("/dlq/reprocess", DLQReprocessHandler(cfg.DLQ))
		}
	})

	// JWT-protected multi-tenant routes
	if cfg.JWTService != nil {
		r.Route("/api/v1/mt", func(r chi.Router) {
			r.Use(auth.JWTAuth(cfg.JWTService))

			// Tenant management (owner/admin only)
			r.Route("/tenants", func(r chi.Router) {
				r.Use(auth.RequireRole("owner", "admin"))
				r.Get("/{id}", GetTenantHandler(cfg.Queries))

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole("owner"))
					r.Delete("/{id}", DeleteTenantHandler(cfg.Queries, cfg.AuditLogger))
				})
			})

			// User management
			r.Route("/users", func(r chi.Router) {
				r.Get("/", ListUsersHandler(cfg.Queries))

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole("owner", "admin"))
					r.Post("/", CreateUserHandler(cfg.Queries, cfg.AuditLogger))
					r.Patch("/{id}/role", UpdateUserRoleHandler(cfg.Queries, cfg.AuditLogger))
				})
			})
		})
	}

	return r
}
