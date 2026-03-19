package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sungwon/smtp-proxy/server/internal/config"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
	"github.com/sungwon/smtp-proxy/server/internal/msgstore"
	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
	"github.com/sungwon/smtp-proxy/server/internal/worker"
)

func main() {
	cfg, err := config.Load("config")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Logging.Level)
	log.Info().Msg("starting queue worker")

	// Initialize database connection pool.
	ctx := context.Background()
	db, err := storage.NewDB(ctx, cfg.Database.DSN(), cfg.Database.PoolMin, cfg.Database.PoolMax, cfg.Database.ConnectTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	queries := storage.New(db.Pool)

	// Initialize provider resolver with HTTP client.
	// Allow stdout fallback only when SMTP_PROXY_PROVIDER_STDOUT_FALLBACK=true (dev/test).
	httpClient := provider.NewHTTPClient(30 * time.Second)
	stdoutFallback := os.Getenv("SMTP_PROXY_PROVIDER_STDOUT_FALLBACK") == "true"
	resolver := provider.NewResolver(queries, httpClient, log, stdoutFallback)

	// Initialize message body store (REQ-QW-004).
	store, err := msgstore.New(msgstore.Config{
		Type:       cfg.Storage.Type,
		Path:       cfg.Storage.Path,
		S3Bucket:   cfg.Storage.S3Bucket,
		S3Prefix:   cfg.Storage.S3Prefix,
		S3Endpoint: cfg.Storage.S3Endpoint,
		S3Region:   cfg.Storage.S3Region,
	}, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize message store")
	}

	// Create message handler with delivery logic.
	handler := worker.NewHandler(resolver, queries, store, log)

	// Build worker pool configuration.
	workerCount := cfg.Queue.Workers
	if workerCount <= 0 {
		workerCount = 10
	}
	blockTimeout := cfg.Queue.BlockTimeout
	if blockTimeout <= 0 {
		blockTimeout = 5 * time.Second
	}

	queueCfg := queue.Config{
		Type:            cfg.Queue.Type,
		RedisAddr:       cfg.Queue.RedisAddr,
		RedisPassword:   cfg.Queue.RedisPassword,
		RedisDB:         cfg.Queue.RedisDB,
		WorkerCount:     workerCount,
		BlockTimeout:    blockTimeout,
		ProcessTimeout:  30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		MaxRetries:      5,
		SQSQueueURL:     cfg.Queue.SQSQueueURL,
		SQSDLQueueURL:   cfg.Queue.SQSDLQueueURL,
		SQSRegion:       cfg.Queue.SQSRegion,
	}

	// Create queue components using configured backend (redis or sqs).
	_, dequeuer, _, err := queue.NewQueue(queueCfg, handler, log, cfg.Queue.StreamName, cfg.Queue.GroupName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create queue")
	}

	if err := dequeuer.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start dequeuer")
	}
	log.Info().
		Int("workers", workerCount).
		Str("type", cfg.Queue.Type).
		Msg("queue worker pool started")

	// Start daily cleanup job for soft-deleted users (>30 days).
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	defer cleanupCancel()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Run once at startup.
		if err := queries.PurgeDeletedUsers(cleanupCtx); err != nil {
			log.Error().Err(err).Msg("failed to purge deleted users")
		} else {
			log.Info().Msg("purged expired soft-deleted users")
		}

		for {
			select {
			case <-ticker.C:
				if err := queries.PurgeDeletedUsers(cleanupCtx); err != nil {
					log.Error().Err(err).Msg("failed to purge deleted users")
				} else {
					log.Info().Msg("purged expired soft-deleted users")
				}
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	// Wait for interrupt signal for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down queue worker")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := dequeuer.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("dequeuer shutdown error")
	}

	log.Info().Msg("queue worker stopped")
}
