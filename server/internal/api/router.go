package api

import (
	"github.com/go-chi/chi/v5"
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
	DLQ         queue.DeadLetterQueue
	JWTService  *auth.JWTService
	AuditLogger *auth.AuditLogger
	RateLimiter *auth.RateLimiter
}

// NewRouterWithConfig creates a chi.Mux with all routes using the full RouterConfig.
// All authenticated routes use UnifiedAuth which accepts both JWT tokens and API keys.
// @MX:ANCHOR: [AUTO] Central API route configuration
// @MX:REASON: fan_in >= 3; entry point for all HTTP request handling
func NewRouterWithConfig(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(CorrelationIDMiddleware)
	r.Use(LoggingMiddleware(cfg.Log))
	r.Use(RecoverMiddleware(cfg.Log))

	// Health endpoints (no auth required)
	r.Get("/healthz", HealthzHandler())
	r.Get("/readyz", ReadyzHandler(cfg.DB))

	// Webhook endpoints (no auth required - called by ESP providers)
	r.Post("/api/v1/webhooks/sendgrid", SendGridWebhookHandler(cfg.Queries))
	r.Post("/api/v1/webhooks/ses", SESWebhookHandler(cfg.Queries))
	r.Post("/api/v1/webhooks/mailgun", MailgunWebhookHandler(cfg.Queries))

	// Auth endpoints (no auth required for login/refresh/logout)
	r.Post("/api/v1/auth/login", LoginHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger, cfg.RateLimiter))
	r.Post("/api/v1/auth/refresh", RefreshHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger))
	r.Post("/api/v1/auth/logout", LogoutHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger))

	// Switch group requires JWT auth only (human users only)
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTAuth(cfg.JWTService))
		r.Post("/api/v1/auth/switch-group", SwitchGroupHandler(cfg.Queries, cfg.JWTService, cfg.AuditLogger))
	})

	// Unified authenticated routes: accepts both JWT tokens and API keys
	r.Group(func(r chi.Router) {
		r.Use(auth.UnifiedAuth(cfg.JWTService, cfg.Queries))

		// Group management (system admin only for create/list)
		r.Route("/api/v1/groups", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireSystemAdmin())
				r.Post("/", CreateGroupHandler(cfg.Queries, cfg.AuditLogger))
				r.Get("/", ListGroupsHandler(cfg.Queries))
			})

			// Group detail routes
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", GetGroupHandler(cfg.Queries))

				// System admin only: delete group
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireSystemAdmin())
					r.Delete("/", DeleteGroupHandler(cfg.Queries, cfg.AuditLogger))
				})

				// Members
				r.Get("/members", ListGroupMembersHandler(cfg.Queries))
				r.Post("/members", AddGroupMemberHandler(cfg.Queries, cfg.AuditLogger))
				r.Patch("/members/{uid}", UpdateGroupMemberRoleHandler(cfg.Queries, cfg.AuditLogger))
				r.Delete("/members/{uid}", RemoveGroupMemberHandler(cfg.Queries, cfg.AuditLogger))

				// Activity logs
				r.Get("/activity", ListActivityLogsHandler(cfg.Queries))
			})
		})

		// User management
		r.Route("/api/v1/users", func(r chi.Router) {
			r.Get("/", ListUsersHandler(cfg.Queries))
			r.Post("/", CreateUserHandler(cfg.Queries, cfg.AuditLogger))
			r.Get("/{id}", GetUserHandler(cfg.Queries))
			r.Patch("/{id}/status", UpdateUserStatusHandler(cfg.Queries, cfg.AuditLogger))
			r.Delete("/{id}", DeleteUserHandler(cfg.Queries, cfg.AuditLogger))
		})

		// Providers
		r.Route("/api/v1/providers", func(r chi.Router) {
			r.Post("/", CreateProviderHandler(cfg.Queries))
			r.Get("/", ListProvidersHandler(cfg.Queries))
			r.Get("/{id}", GetProviderHandler(cfg.Queries))
			r.Put("/{id}", UpdateProviderHandler(cfg.Queries))
			r.Delete("/{id}", DeleteProviderHandler(cfg.Queries))
		})

		// Routing Rules
		r.Route("/api/v1/routing-rules", func(r chi.Router) {
			r.Post("/", CreateRoutingRuleHandler(cfg.Queries))
			r.Get("/", ListRoutingRulesHandler(cfg.Queries))
			r.Get("/{id}", GetRoutingRuleHandler(cfg.Queries))
			r.Put("/{id}", UpdateRoutingRuleHandler(cfg.Queries))
			r.Delete("/{id}", DeleteRoutingRuleHandler(cfg.Queries))
		})

		// DLQ Reprocess
		if cfg.DLQ != nil {
			r.Post("/api/v1/dlq/reprocess", DLQReprocessHandler(cfg.DLQ))
		}
	})

	return r
}
