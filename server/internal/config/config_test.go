package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidConfigFile(t *testing.T) {
	cfg, err := Load("../../config")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// SMTP defaults from config.yaml
	if cfg.SMTP.Host != "0.0.0.0" {
		t.Errorf("expected SMTP host 0.0.0.0, got %s", cfg.SMTP.Host)
	}
	if cfg.SMTP.Port != 587 {
		t.Errorf("expected SMTP port 587, got %d", cfg.SMTP.Port)
	}
	if cfg.SMTP.MaxConnections != 1000 {
		t.Errorf("expected max connections 1000, got %d", cfg.SMTP.MaxConnections)
	}
	if cfg.SMTP.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.SMTP.ReadTimeout)
	}
	if cfg.SMTP.WriteTimeout != 30*time.Second {
		t.Errorf("expected write timeout 30s, got %v", cfg.SMTP.WriteTimeout)
	}
	if cfg.SMTP.MaxMessageSize != 26214400 {
		t.Errorf("expected max message size 26214400, got %d", cfg.SMTP.MaxMessageSize)
	}

	// API defaults
	if cfg.API.Host != "0.0.0.0" {
		t.Errorf("expected API host 0.0.0.0, got %s", cfg.API.Host)
	}
	if cfg.API.Port != 8080 {
		t.Errorf("expected API port 8080, got %d", cfg.API.Port)
	}
	if cfg.API.ReadTimeout != 10*time.Second {
		t.Errorf("expected API read timeout 10s, got %v", cfg.API.ReadTimeout)
	}
	if cfg.API.WriteTimeout != 10*time.Second {
		t.Errorf("expected API write timeout 10s, got %v", cfg.API.WriteTimeout)
	}

	// Database defaults
	if cfg.Database.URL != "postgres://smtp_proxy:smtp_proxy_dev@localhost:5432/smtp_proxy?sslmode=disable" {
		t.Errorf("unexpected database URL: %s", cfg.Database.URL)
	}
	if cfg.Database.PoolMin != 5 {
		t.Errorf("expected pool min 5, got %d", cfg.Database.PoolMin)
	}
	if cfg.Database.PoolMax != 20 {
		t.Errorf("expected pool max 20, got %d", cfg.Database.PoolMax)
	}
	if cfg.Database.ConnectTimeout != 5*time.Second {
		t.Errorf("expected connect timeout 5s, got %v", cfg.Database.ConnectTimeout)
	}

	// Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected log format json, got %s", cfg.Logging.Format)
	}

	// TLS defaults
	if cfg.TLS.CertFile != "" {
		t.Errorf("expected empty cert file, got %s", cfg.TLS.CertFile)
	}
	if cfg.TLS.KeyFile != "" {
		t.Errorf("expected empty key file, got %s", cfg.TLS.KeyFile)
	}
}

func TestLoad_EnvironmentVariableOverride(t *testing.T) {
	overrideURL := "postgres://override:override@remotehost:5432/override_db?sslmode=require"
	t.Setenv("SMTP_PROXY_DATABASE_URL", overrideURL)

	cfg, err := Load("../../config")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Database.URL != overrideURL {
		t.Errorf("expected database URL override %s, got %s", overrideURL, cfg.Database.URL)
	}

	// Other values should still be from config file
	if cfg.SMTP.Port != 587 {
		t.Errorf("expected SMTP port 587, got %d", cfg.SMTP.Port)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	partialConfig := `
smtp:
  port: 2525
logging:
  level: debug
`
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(partialConfig), 0o644)
	if err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Explicitly set values
	if cfg.SMTP.Port != 2525 {
		t.Errorf("expected SMTP port 2525, got %d", cfg.SMTP.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}

	// Default values for unset fields
	if cfg.SMTP.Host != "" {
		t.Errorf("expected empty SMTP host for partial config, got %s", cfg.SMTP.Host)
	}
	if cfg.API.Port != 0 {
		t.Errorf("expected API port 0 for partial config, got %d", cfg.API.Port)
	}
}

func TestLoad_MissingConfigFile(t *testing.T) {
	_, err := Load("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}
}

func TestLoad_StorageDefaults(t *testing.T) {
	cfg, err := Load("../../config")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Storage.Type != "local" {
		t.Errorf("expected storage type local, got %s", cfg.Storage.Type)
	}
	if cfg.Storage.Path != "/data/messages" {
		t.Errorf("expected storage path /data/messages, got %s", cfg.Storage.Path)
	}
	if cfg.Storage.S3Region != "us-east-1" {
		t.Errorf("expected storage s3_region us-east-1, got %s", cfg.Storage.S3Region)
	}
	if cfg.Storage.S3Bucket != "" {
		t.Errorf("expected empty storage s3_bucket, got %s", cfg.Storage.S3Bucket)
	}
	if cfg.Storage.S3Prefix != "" {
		t.Errorf("expected empty storage s3_prefix, got %s", cfg.Storage.S3Prefix)
	}
	if cfg.Storage.S3Endpoint != "" {
		t.Errorf("expected empty storage s3_endpoint, got %s", cfg.Storage.S3Endpoint)
	}
}

func TestLoad_StorageEnvironmentVariableOverride(t *testing.T) {
	t.Setenv("SMTP_PROXY_STORAGE_TYPE", "s3")
	t.Setenv("SMTP_PROXY_STORAGE_PATH", "/custom/path")

	cfg, err := Load("../../config")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Storage.Type != "s3" {
		t.Errorf("expected storage type s3 from env override, got %s", cfg.Storage.Type)
	}
	if cfg.Storage.Path != "/custom/path" {
		t.Errorf("expected storage path /custom/path from env override, got %s", cfg.Storage.Path)
	}
}

func TestLoad_EnvironmentVariableOverrideSMTPPort(t *testing.T) {
	t.Setenv("SMTP_PROXY_SMTP_PORT", "2525")

	cfg, err := Load("../../config")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.SMTP.Port != 2525 {
		t.Errorf("expected SMTP port 2525 from env override, got %d", cfg.SMTP.Port)
	}
}
