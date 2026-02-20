package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/sungwon/smtp-proxy/server/internal/config"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
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
	db, err := storage.NewDB(ctx, cfg.Database.URL, cfg.Database.PoolMin, cfg.Database.PoolMax, cfg.Database.ConnectTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	queries := storage.New(db.Pool)

	// Initialize provider resolver with HTTP client and stdout fallback.
	httpClient := provider.NewHTTPClient(30 * time.Second)
	resolver := provider.NewResolver(queries, httpClient, log)

	// Connect to Redis.
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Queue.RedisAddr,
		Password: cfg.Queue.RedisPassword,
		DB:       cfg.Queue.RedisDB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	// Create consumer group for the configured stream.
	consumer := queue.NewConsumer(redisClient)
	if err := consumer.CreateConsumerGroup(ctx, cfg.Queue.StreamName, cfg.Queue.GroupName); err != nil {
		log.Fatal().Err(err).Msg("failed to create consumer group")
	}

	// Create message handler with delivery logic.
	handler := worker.NewHandler(resolver, queries, log)

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
		WorkerCount:     workerCount,
		BlockTimeout:    blockTimeout,
		ProcessTimeout:  30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		MaxRetries:      5,
	}

	// Create and start worker pool.
	dlq := queue.NewDLQ(redisClient, queue.NewProducer(redisClient))
	retryStrategy := queue.NewRetryStrategy(queueCfg.MaxRetries)

	pool := queue.NewWorkerPool(
		redisClient,
		consumer,
		dlq,
		handler,
		retryStrategy,
		queueCfg,
		log,
		cfg.Queue.StreamName,
		cfg.Queue.GroupName,
	)

	pool.Start(ctx)
	log.Info().
		Int("workers", workerCount).
		Str("stream", cfg.Queue.StreamName).
		Str("group", cfg.Queue.GroupName).
		Msg("queue worker pool started")

	// Wait for interrupt signal for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down queue worker")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool.Stop(shutdownCtx)

	log.Info().Msg("queue worker stopped")
}
