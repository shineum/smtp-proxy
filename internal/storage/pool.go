package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool.Pool for database operations.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB creates a new database connection pool and verifies connectivity.
func NewDB(ctx context.Context, databaseURL string, minConns, maxConns int32, connectTimeout time.Duration) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	config.MinConns = minConns
	config.MaxConns = maxConns
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes all connections in the pool.
func (db *DB) Close() {
	db.Pool.Close()
}

// Ping verifies database connectivity.
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
