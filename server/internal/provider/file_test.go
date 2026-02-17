package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFile_GetName(t *testing.T) {
	p := NewFile(ProviderConfig{Type: "file"})
	if p.GetName() != "file" {
		t.Errorf("expected name file, got %s", p.GetName())
	}
}

func TestFile_Send(t *testing.T) {
	dir := t.TempDir()
	p := NewFile(ProviderConfig{Type: "file", Endpoint: dir})

	msg := &Message{
		ID:      "msg-456",
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "File Test",
		Headers: map[string]string{"X-Test": "yes"},
		Body:    []byte("Hello from file provider"),
	}

	result, err := p.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ProviderMessageID != "file-msg-456" {
		t.Errorf("expected provider message ID file-msg-456, got %s", result.ProviderMessageID)
	}
	if result.Status != StatusSent {
		t.Errorf("expected status sent, got %s", result.Status)
	}

	path := result.Metadata["path"]
	if path == "" {
		t.Fatal("expected path in metadata")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "From: sender@example.com") {
		t.Error("expected file to contain From header")
	}
	if !strings.Contains(content, "To: recipient@example.com") {
		t.Error("expected file to contain To header")
	}
	if !strings.Contains(content, "Subject: File Test") {
		t.Error("expected file to contain Subject header")
	}
	if !strings.Contains(content, "Hello from file provider") {
		t.Error("expected file to contain body")
	}
}

func TestFile_Send_DefaultDir(t *testing.T) {
	// Use a temp dir as working directory to avoid polluting the repo.
	dir := t.TempDir()
	defaultDir := filepath.Join(dir, "mail_output")

	p := &File{outputDir: defaultDir}

	msg := &Message{
		ID:   "default-dir-test",
		From: "test@example.com",
		To:   []string{"to@example.com"},
		Body: []byte("test"),
	}

	result, err := p.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.HasPrefix(result.Metadata["path"], defaultDir) {
		t.Errorf("expected path under %s, got %s", defaultDir, result.Metadata["path"])
	}

	// Cleanup.
	os.RemoveAll(defaultDir)
}

func TestFile_HealthCheck(t *testing.T) {
	dir := t.TempDir()
	p := NewFile(ProviderConfig{Type: "file", Endpoint: dir})

	if err := p.HealthCheck(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestFile_Send_SlashInID(t *testing.T) {
	dir := t.TempDir()
	p := NewFile(ProviderConfig{Type: "file", Endpoint: dir})

	msg := &Message{
		ID:   "msg/with/slashes",
		From: "test@example.com",
		To:   []string{"to@example.com"},
		Body: []byte("test"),
	}

	result, err := p.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Slashes should be replaced with underscores in filename.
	if strings.Contains(filepath.Base(result.Metadata["path"]), "/") {
		t.Error("expected slashes to be sanitized in filename")
	}
}
