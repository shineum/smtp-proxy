// Package migrate runs SQL migrations from a filesystem directory.
// It is compatible with golang-migrate's schema_migrations table format.
package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// advisoryLockID is a fixed hash used for PostgreSQL advisory lock.
// Prevents concurrent migration execution across multiple app instances.
const advisoryLockID int64 = 0x736d7470_70726f78 // "smtpprox"

// Up runs all pending up migrations from the given directory.
// It acquires a PostgreSQL advisory lock to prevent concurrent execution.
func Up(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, log zerolog.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	// Acquire advisory lock (blocks until available).
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("migrate: acquire advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockID)
	}()

	// Ensure schema_migrations table exists (compatible with golang-migrate).
	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint NOT NULL PRIMARY KEY,
			dirty boolean NOT NULL DEFAULT false
		)
	`); err != nil {
		return fmt.Errorf("migrate: create schema_migrations table: %w", err)
	}

	// Get current version.
	var currentVersion int64
	err = conn.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations WHERE NOT dirty").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("migrate: get current version: %w", err)
	}

	// Read migration files.
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("migrate: read directory %s: %w", migrationsDir, err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	applied := 0
	for _, f := range upFiles {
		version, err := parseVersion(f)
		if err != nil {
			return fmt.Errorf("migrate: parse version from %s: %w", f, err)
		}

		if version <= currentVersion {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", f, err)
		}

		// Mark as dirty before executing.
		if _, err := conn.Exec(ctx,
			"INSERT INTO schema_migrations (version, dirty) VALUES ($1, true) ON CONFLICT (version) DO UPDATE SET dirty = true",
			version,
		); err != nil {
			return fmt.Errorf("migrate: mark dirty %s: %w", f, err)
		}

		if _, err := conn.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("migrate: execute %s: %w", f, err)
		}

		// Mark as clean.
		if _, err := conn.Exec(ctx,
			"UPDATE schema_migrations SET dirty = false WHERE version = $1",
			version,
		); err != nil {
			return fmt.Errorf("migrate: mark clean %s: %w", f, err)
		}

		applied++
		log.Info().Int64("version", version).Str("file", f).Msg("migration applied")
	}

	if applied == 0 {
		log.Info().Int64("current_version", currentVersion).Msg("database is up to date")
	} else {
		log.Info().Int("applied", applied).Msg("migrations complete")
	}

	return nil
}

// parseVersion extracts the numeric prefix from a migration filename like "001_create_foo.up.sql".
func parseVersion(filename string) (int64, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid migration filename: %s", filename)
	}
	return strconv.ParseInt(parts[0], 10, 64)
}
