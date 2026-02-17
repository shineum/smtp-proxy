package queue

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("WorkerCount", func(t *testing.T) {
		if cfg.WorkerCount != 10 {
			t.Errorf("DefaultConfig() WorkerCount = %d, want 10", cfg.WorkerCount)
		}
	})

	t.Run("BlockTimeout", func(t *testing.T) {
		want := 5 * time.Second
		if cfg.BlockTimeout != want {
			t.Errorf("DefaultConfig() BlockTimeout = %v, want %v", cfg.BlockTimeout, want)
		}
	})

	t.Run("ProcessTimeout", func(t *testing.T) {
		want := 30 * time.Second
		if cfg.ProcessTimeout != want {
			t.Errorf("DefaultConfig() ProcessTimeout = %v, want %v", cfg.ProcessTimeout, want)
		}
	})

	t.Run("ShutdownTimeout", func(t *testing.T) {
		want := 30 * time.Second
		if cfg.ShutdownTimeout != want {
			t.Errorf("DefaultConfig() ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, want)
		}
	})

	t.Run("MaxRetries", func(t *testing.T) {
		if cfg.MaxRetries != 5 {
			t.Errorf("DefaultConfig() MaxRetries = %d, want 5", cfg.MaxRetries)
		}
	})

	t.Run("RedisAddr", func(t *testing.T) {
		if cfg.RedisAddr != "localhost:6379" {
			t.Errorf("DefaultConfig() RedisAddr = %q, want %q", cfg.RedisAddr, "localhost:6379")
		}
	})

	t.Run("RedisDB", func(t *testing.T) {
		if cfg.RedisDB != 0 {
			t.Errorf("DefaultConfig() RedisDB = %d, want 0", cfg.RedisDB)
		}
	})

	t.Run("RedisPassword is empty", func(t *testing.T) {
		if cfg.RedisPassword != "" {
			t.Errorf("DefaultConfig() RedisPassword = %q, want empty string", cfg.RedisPassword)
		}
	})
}
