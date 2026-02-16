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

	"github.com/sungwon/smtp-proxy/internal/config"
	"github.com/sungwon/smtp-proxy/internal/logger"
	smtpserver "github.com/sungwon/smtp-proxy/internal/smtp"
	"github.com/sungwon/smtp-proxy/internal/storage"
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

	// Create SMTP backend with connection limit.
	backend := smtpserver.NewBackend(queries, log, cfg.SMTP.MaxConnections)

	// Configure SMTP server.
	s := gosmtp.NewServer(backend)
	s.Addr = fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port)
	s.Domain = "smtp-proxy"
	s.ReadTimeout = cfg.SMTP.ReadTimeout
	s.WriteTimeout = cfg.SMTP.WriteTimeout
	s.MaxMessageBytes = cfg.SMTP.MaxMessageSize
	s.AllowInsecureAuth = false

	// Configure TLS if certificates are provided.
	if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load TLS certificate")
		}
		s.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		s.EnableSMTPUTF8 = true
	}

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
