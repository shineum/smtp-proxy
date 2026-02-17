package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sungwon/smtp-proxy/server/internal/config"
	"github.com/sungwon/smtp-proxy/server/internal/logger"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func main() {
	cfg, err := config.Load("config")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Logging.Level)
	log.Info().Msg("starting queue worker")

	ctx := context.Background()
	db, err := storage.NewDB(ctx, cfg.Database.URL, cfg.Database.PoolMin, cfg.Database.PoolMax, cfg.Database.ConnectTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	log.Warn().Msg("queue worker not yet implemented — waiting for SPEC-QUEUE-001")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("queue worker stopped")
}
