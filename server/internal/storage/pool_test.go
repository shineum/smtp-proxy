//go:build integration

package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestNewDB_ConnectAndPing(t *testing.T) {
	db, _ := setupTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		t.Fatalf("expected ping to succeed, got %v", err)
	}
}

func TestNewDB_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := storage.NewDB(ctx, "postgres://invalid:invalid@localhost:1/invalid?sslmode=disable", 1, 5, 2*time.Second)
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}

func TestNewDB_SeparatePoolCloseDoesNotAffectShared(t *testing.T) {
	// Create a separate DB connection to the same container to test Close behavior.
	ctx := context.Background()
	db, err := storage.NewDB(ctx, sharedDSN, 1, 2, 10*time.Second)
	if err != nil {
		t.Fatalf("failed to create separate DB: %v", err)
	}

	// Close the separate pool
	db.Close()

	// Ping on closed pool should fail
	err = db.Ping(ctx)
	if err == nil {
		t.Error("expected ping to fail after close")
	}

	// Shared DB should still work
	if err := sharedDB.Ping(ctx); err != nil {
		t.Fatalf("shared DB ping should still work, got %v", err)
	}
}
