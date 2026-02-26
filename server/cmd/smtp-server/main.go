package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/redis/go-redis/v9"

	"github.com/sungwon/smtp-proxy/server/internal/config"
	"github.com/sungwon/smtp-proxy/server/internal/delivery"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
	"github.com/sungwon/smtp-proxy/server/internal/msgstore"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	smtpserver "github.com/sungwon/smtp-proxy/server/internal/smtp"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
	"github.com/sungwon/smtp-proxy/server/internal/tlsutil"
)

func main() {
	// Load configuration from the "config" directory.
	cfg, err := config.Load("config")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured JSON logger.
	log := logger.New(cfg.Logging.Level)
	log.Info().Msg("starting SMTP server")

	// Initialize database connection pool.
	ctx := context.Background()
	db, err := storage.NewDB(ctx, cfg.Database.URL, cfg.Database.PoolMin, cfg.Database.PoolMax, cfg.Database.ConnectTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	queries := storage.New(db.Pool)

	// Create async delivery service (Redis is required).
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Queue.RedisAddr,
		Password: cfg.Queue.RedisPassword,
		DB:       cfg.Queue.RedisDB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	producer := queue.NewProducer(redisClient)
	deliverySvc := delivery.NewAsyncService(producer, log)
	log.Info().Msg("delivery mode: async (Redis Streams)")

	// Initialize message body storage.
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
	log.Info().Str("type", cfg.Storage.Type).Msg("message store initialized")

	// Create SMTP backend with delivery service.
	backend := smtpserver.NewBackend(queries, deliverySvc, store, log, cfg.SMTP.MaxConnections)

	// Configure SMTP server.
	s := gosmtp.NewServer(backend)
	s.Addr = fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port)
	s.Domain = "smtp-proxy"
	s.ReadTimeout = cfg.SMTP.ReadTimeout
	s.WriteTimeout = cfg.SMTP.WriteTimeout
	s.MaxMessageBytes = cfg.SMTP.MaxMessageSize
	s.AllowInsecureAuth = false

	// Configure TLS: load from files if provided, otherwise auto-generate.
	var cert tls.Certificate
	if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
		cert, err = tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load TLS certificate")
		}
		log.Info().Msg("TLS: loaded certificate from files")
	} else {
		cert, err = tlsutil.GenerateSelfSigned()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to generate self-signed TLS certificate")
		}
		log.Info().Msg("TLS: using auto-generated self-signed certificate")
	}
	s.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	s.EnableSMTPUTF8 = true

	// Start listening on the configured address.
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", s.Addr).Msg("failed to listen")
	}

	// Serve connections in a goroutine.
	go func() {
		log.Info().Str("addr", s.Addr).Msg("SMTP server listening")
		if err := s.Serve(ln); err != nil {
			log.Error().Err(err).Msg("SMTP server error")
		}
	}()

	// Wait for interrupt signal for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down SMTP server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("SMTP server shutdown error")
	}

	log.Info().Msg("SMTP server stopped")
}
