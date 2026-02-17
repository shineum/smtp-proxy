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

// NewRouter creates a chi.Mux with all routes, middleware, and handlers configured.
// The dlq parameter is optional; when nil, DLQ reprocess endpoints are not registered.
func NewRouter(queries storage.Querier, db *storage.DB, log zerolog.Logger, dlq *queue.DLQ) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(CorrelationIDMiddleware)
	r.Use(LoggingMiddleware(log))
	r.Use(RecoverMiddleware(log))

	// Health endpoints (no auth required)
	r.Get("/healthz", HealthzHandler())
	r.Get("/readyz", ReadyzHandler(db))

	// Account creation endpoint (no auth required)
	r.Post("/api/v1/accounts", CreateAccountHandler(queries))

	// Webhook endpoints (no auth required - called by ESP providers)
	r.Post("/api/v1/webhooks/sendgrid", SendGridWebhookHandler(queries))
	r.Post("/api/v1/webhooks/ses", SESWebhookHandler(queries))
	r.Post("/api/v1/webhooks/mailgun", MailgunWebhookHandler(queries))

	// API routes (auth required)
	accountLookup := func(ctx context.Context, apiKey string) (uuid.UUID, error) {
		account, err := queries.GetAccountByAPIKey(ctx, apiKey)
		if err != nil {
			return uuid.Nil, err
		}
		return account.ID, nil
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.BearerAuth(accountLookup))

		// Accounts (CRUD except create which is above without auth)
		r.Get("/accounts/{id}", GetAccountHandler(queries))
		r.Put("/accounts/{id}", UpdateAccountHandler(queries))
		r.Delete("/accounts/{id}", DeleteAccountHandler(queries))

		// Providers
		r.Post("/providers", CreateProviderHandler(queries))
		r.Get("/providers", ListProvidersHandler(queries))
		r.Get("/providers/{id}", GetProviderHandler(queries))
		r.Put("/providers/{id}", UpdateProviderHandler(queries))
		r.Delete("/providers/{id}", DeleteProviderHandler(queries))

		// Routing Rules
		r.Post("/routing-rules", CreateRoutingRuleHandler(queries))
		r.Get("/routing-rules", ListRoutingRulesHandler(queries))
		r.Get("/routing-rules/{id}", GetRoutingRuleHandler(queries))
		r.Put("/routing-rules/{id}", UpdateRoutingRuleHandler(queries))
		r.Delete("/routing-rules/{id}", DeleteRoutingRuleHandler(queries))

		// DLQ Reprocess
		if dlq != nil {
			r.Post("/dlq/reprocess", DLQReprocessHandler(dlq))
		}
	})

	return r
}
