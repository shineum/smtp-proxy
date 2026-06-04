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

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	gosmtp "github.com/emersion/go-smtp"

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
	db, err := storage.NewDB(ctx, cfg.Database.DSN(), cfg.Database.PoolMin, cfg.Database.PoolMax, cfg.Database.ConnectTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	queries := storage.New(db.Pool)

	// Create async delivery service using configured queue backend.
	queueCfg := queue.Config{
		Type:          cfg.Queue.Type,
		RedisAddr:     cfg.Queue.RedisAddr,
		RedisPassword: cfg.Queue.RedisPassword,
		RedisDB:       cfg.Queue.RedisDB,
		SQSQueueURL:   cfg.Queue.SQSQueueURL,
		SQSDLQueueURL: cfg.Queue.SQSDLQueueURL,
		SQSRegion:     cfg.Queue.SQSRegion,
	}
	enqueuer, err := queue.NewEnqueuer(queueCfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create queue enqueuer")
	}
	deliverySvc := delivery.NewAsyncService(enqueuer, cfg.Queue.StreamName, log)
	log.Info().Str("type", cfg.Queue.Type).Msg("delivery mode: async")

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
	// Configure TLS based on mode.
	// The tls.mode == "none" path is intentionally unchanged (NLB/proxy TLS termination).
	if cfg.TLS.Mode == "none" {
		s.AllowInsecureAuth = true
		log.Warn().Msg("TLS disabled (mode=none); ensure TLS is terminated upstream (NLB/proxy)")
	} else {
		s.AllowInsecureAuth = false

		// Precedence:
		//   1. CertFile+KeyFile set → use file cert (static, no hot-reload).
		//   2. SecretID set          → use Secrets Manager provider (hot-reload).
		//   3. Neither               → generate and serve a self-signed cert.
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
			var cert tls.Certificate
			cert, err = tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to load TLS certificate from files")
			}
			log.Info().Msg("TLS: loaded certificate from files")
			s.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
		} else {
			// Generate the self-signed certificate once; it is used as the fallback
			// when Secrets Manager is not configured or a reload/handshake match fails.
			selfSigned, ssErr := tlsutil.GenerateSelfSigned()
			if ssErr != nil {
				log.Fatal().Err(ssErr).Msg("failed to generate self-signed TLS certificate")
			}

			// Build a cancellable context for the provider reload goroutine.
			// It will be cancelled on server shutdown.
			providerCtx, providerCancel := context.WithCancel(ctx)

			var smFetcher tlsutil.SecretsManagerFetcher
			if cfg.TLS.SecretID != "" {
				awsCfg, awsErr := awsconfig.LoadDefaultConfig(ctx)
				if awsErr != nil {
					log.Fatal().Err(awsErr).Msg("failed to load AWS config for Secrets Manager")
				}
				smFetcher = secretsmanager.NewFromConfig(awsCfg)
				log.Info().Str("secret_id", cfg.TLS.SecretID).Msg("TLS: Secrets Manager provider enabled")
			} else {
				log.Info().Msg("TLS: using auto-generated self-signed certificate (no SecretID configured)")
			}

			reloadInterval := time.Duration(cfg.TLS.ReloadInterval) * time.Hour
			provider := tlsutil.NewProvider(
				smFetcher,
				cfg.TLS.SecretID,
				cfg.TLS.DefaultCert,
				reloadInterval,
				selfSigned,
				log,
			)
			provider.Start(providerCtx)

			s.TLSConfig = &tls.Config{
				GetCertificate: provider.GetCertificate,
				MinVersion:     tls.VersionTLS12,
			}

			// Ensure the provider goroutine stops on shutdown.
			defer providerCancel()
		}
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
