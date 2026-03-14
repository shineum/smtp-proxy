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

	// Auth/me endpoints (JWT auth only)
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTAuth(cfg.JWTService))
		r.Get("/api/v1/auth/me", MeHandler(cfg.Queries))
		r.Patch("/api/v1/users/me/password", ChangePasswordHandler(cfg.Queries))
	})

	// Unified authenticated routes: accepts both JWT tokens and API keys
	r.Group(func(r chi.Router) {
		r.Use(auth.UnifiedAuth(cfg.JWTService, cfg.Queries))

		// Group management (open to all authenticated users)
		r.Route("/api/v1/groups", func(r chi.Router) {
			r.Post("/", CreateGroupHandler(cfg.Queries, cfg.AuditLogger))
			r.Get("/", ListGroupsHandler(cfg.Queries))

			// Group detail routes
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", GetGroupHandler(cfg.Queries))
				r.Put("/", UpdateGroupHandler(cfg.Queries, cfg.AuditLogger))
				r.Delete("/", DeleteGroupHandler(cfg.Queries, cfg.AuditLogger))

				// Members
				r.Get("/members", ListGroupMembersHandler(cfg.Queries))
				r.Post("/members", AddGroupMemberHandler(cfg.Queries, cfg.AuditLogger))
				r.Patch("/members/{uid}", UpdateGroupMemberRoleHandler(cfg.Queries, cfg.AuditLogger))
				r.Delete("/members/{uid}", RemoveGroupMemberHandler(cfg.Queries, cfg.AuditLogger))

				// Service accounts (group-scoped)
				r.Post("/service-accounts", CreateServiceAccountHandler(cfg.Queries, cfg.AuditLogger))
				r.Patch("/service-accounts/{uid}", UpdateServiceAccountHandler(cfg.Queries, cfg.AuditLogger))
				r.Post("/service-accounts/{uid}/reset-api-key", ResetServiceAccountAPIKeyHandler(cfg.Queries, cfg.AuditLogger))

				// API keys management for service accounts
				r.Post("/service-accounts/{uid}/api-keys", CreateAPIKeyHandler(cfg.Queries, cfg.AuditLogger))
				r.Get("/service-accounts/{uid}/api-keys", ListAPIKeysHandler(cfg.Queries))
				r.Patch("/service-accounts/{uid}/api-keys/{keyId}", UpdateAPIKeyStatusHandler(cfg.Queries, cfg.AuditLogger))
				r.Delete("/service-accounts/{uid}/api-keys/{keyId}", DeleteAPIKeyHandler(cfg.Queries, cfg.AuditLogger))

				// Activity logs
				r.Get("/activity", ListActivityLogsHandler(cfg.Queries))
			})
		})

		// User management
		r.Route("/api/v1/users", func(r chi.Router) {
			r.Get("/", ListUsersHandler(cfg.Queries))
			r.Post("/", CreateUserHandler(cfg.Queries, cfg.AuditLogger))
			r.Get("/deleted", ListDeletedUsersHandler(cfg.Queries))
			r.Get("/{id}", GetUserHandler(cfg.Queries))
			r.Patch("/{id}/status", UpdateUserStatusHandler(cfg.Queries, cfg.AuditLogger))
			r.Delete("/{id}", DeleteUserHandler(cfg.Queries, cfg.AuditLogger))
			r.Post("/{id}/restore", RestoreUserHandler(cfg.Queries, cfg.AuditLogger))
			r.Post("/{id}/reset-password", ResetPasswordHandler(cfg.Queries, cfg.AuditLogger))
			r.Patch("/{id}/password-disabled", UpdatePasswordDisabledHandler(cfg.Queries, cfg.AuditLogger))
			r.Get("/{id}/memberships", ListUserMembershipsHandler(cfg.Queries))
			r.Post("/{id}/reset-api-key", ResetAPIKeyHandler(cfg.Queries, cfg.AuditLogger))
		})

		// Providers
		r.Route("/api/v1/providers", func(r chi.Router) {
			r.Post("/", CreateProviderHandler(cfg.Queries))
			r.Get("/", ListProvidersHandler(cfg.Queries))
			r.Get("/{id}", GetProviderHandler(cfg.Queries))
			r.Put("/{id}", UpdateProviderHandler(cfg.Queries))
			r.Delete("/{id}", DeleteProviderHandler(cfg.Queries))
			r.Get("/{id}/health", ProviderHealthHandler(cfg.Queries))
			r.Get("/{id}/usage", ProviderUsageHandler(cfg.Queries))
			r.Get("/{id}/access", ListProviderAccessHandler(cfg.Queries))
			r.Post("/{id}/access", GrantProviderAccessHandler(cfg.Queries))
			r.Delete("/{id}/access/{groupId}", RevokeProviderAccessHandler(cfg.Queries))
			r.Post("/{id}/send", TestProviderHandler(cfg.Queries))
		})

		// Routing Rules
		r.Route("/api/v1/routing-rules", func(r chi.Router) {
			r.Post("/", CreateRoutingRuleHandler(cfg.Queries))
			r.Get("/", ListRoutingRulesHandler(cfg.Queries))
			r.Get("/{id}", GetRoutingRuleHandler(cfg.Queries))
			r.Put("/{id}", UpdateRoutingRuleHandler(cfg.Queries))
			r.Delete("/{id}", DeleteRoutingRuleHandler(cfg.Queries))
		})

		// Stats (dashboard, timeseries, usage)
		r.Route("/api/v1/stats", func(r chi.Router) {
			r.Get("/dashboard", DashboardHandler(cfg.Queries))
			r.Get("/timeseries", TimeSeriesHandler(cfg.Queries))
			r.Get("/by-user", UsageByUserHandler(cfg.Queries))
			r.Get("/by-group", UsageByGroupHandler(cfg.Queries))
			r.Get("/by-provider", UsageByProviderHandler(cfg.Queries))
		})

		// Messages
		r.Route("/api/v1/messages", func(r chi.Router) {
			r.Get("/", ListMessagesHandler(cfg.Queries))
			r.Get("/{id}", GetMessageHandler(cfg.Queries))
		})

		// DLQ Reprocess
		if cfg.DLQ != nil {
			r.Post("/api/v1/dlq/reprocess", DLQReprocessHandler(cfg.DLQ))
		}
	})

	return r
}
