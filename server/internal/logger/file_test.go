package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileWriter_ReturnsNonNil(t *testing.T) {
	w := NewFileWriter(FileConfig{
		Path:      filepath.Join(t.TempDir(), "test.log"),
		MaxSizeMB: 10,
		MaxFiles:  3,
	})
	if w == nil {
		t.Fatal("expected non-nil writer")
	}
}

func TestNewFileWriter_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	w := NewFileWriter(FileConfig{
		Path:      logPath,
		MaxSizeMB: 10,
		MaxFiles:  3,
	})

	msg := []byte(`{"level":"info","message":"hello"}` + "\n")
	n, err := w.Write(msg)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("expected %d bytes written, got %d", len(msg), n)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if string(data) != string(msg) {
		t.Errorf("expected file content %q, got %q", msg, data)
	}
}

func TestNewFileWriter_CreatesFileAtPath(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "app.log")

	w := NewFileWriter(FileConfig{
		Path:      logPath,
		MaxSizeMB: 10,
		MaxFiles:  3,
	})

	_, err := w.Write([]byte("test\n"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("expected log file to be created at %s", logPath)
	}
}
