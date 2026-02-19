package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/api"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/config"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.Logging.Level)
	log.Info().Msg("starting API server")

	// Connect to database
	ctx := context.Background()
	db, err := storage.NewDB(
		ctx,
		cfg.Database.URL,
		cfg.Database.PoolMin,
		cfg.Database.PoolMax,
		cfg.Database.ConnectTimeout,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	log.Info().Msg("database connection established")

	// Create sqlc queries instance
	queries := storage.New(db.Pool)

	// Initialize JWT service
	jwtService := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         cfg.Auth.SigningKey,
		AccessTokenExpiry:  cfg.Auth.AccessTokenExpiry,
		RefreshTokenExpiry: cfg.Auth.RefreshTokenExpiry,
		Issuer:             cfg.Auth.Issuer,
		Audience:           cfg.Auth.Audience,
	})

	if cfg.Auth.SigningKey == "" || cfg.Auth.SigningKey == "change-me-in-production-use-a-strong-secret" {
		log.Warn().Msg("JWT signing key is not set or using default value; set SMTP_PROXY_AUTH_SIGNING_KEY in production")
	}

	// Initialize audit store (bridges auth.AuditStore to storage.Querier)
	auditStore := auth.NewFuncAuditStore(func(ctx context.Context, entry auth.AuditEntry) error {
		_, err := queries.CreateAuditLog(ctx, storage.CreateAuditLogParams{
			TenantID:     entry.TenantID,
			UserID:       pgtype.UUID{Bytes: entry.UserID, Valid: entry.UserID != uuid.Nil},
			Action:       entry.Action,
			ResourceType: entry.ResourceType,
			ResourceID:   pgtype.Text{String: entry.ResourceID, Valid: entry.ResourceID != ""},
			Result:       entry.Result,
			Metadata:     auth.MetadataToJSON(entry.Metadata),
			IPAddress:    auth.IPToInet(entry.IPAddress),
		})
		return err
	})
	auditLogger := auth.NewAuditLogger(auditStore, log)

	// Initialize rate limiter (nil Redis client for sync mode, will skip rate limiting)
	var rateLimiter *auth.RateLimiter
	if cfg.Delivery.Mode == "async" {
		rateLimiter = auth.NewRateLimiter(nil, auth.RateLimitConfig{
			DefaultMonthlyLimit:  cfg.RateLimit.DefaultMonthlyLimit,
			LoginAttemptsLimit:   cfg.RateLimit.LoginAttemptsLimit,
			LoginLockoutDuration: cfg.RateLimit.LoginLockoutDuration,
		})
		log.Info().Msg("rate limiter initialized (Redis will be configured with queue)")
	}

	// Build router with full config
	router := api.NewRouterWithConfig(api.RouterConfig{
		Queries:     queries,
		DB:          db,
		Log:         log,
		DLQ:         nil,
		JWTService:  jwtService,
		AuditLogger: auditLogger,
		RateLimiter: rateLimiter,
	})

	// Configure HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.API.ReadTimeout,
		WriteTimeout: cfg.API.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Info().Str("addr", addr).Msg("API server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down server")

	// Graceful shutdown with 30-second timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("server stopped")
}
