package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultOutputDir = "./mail_output"

// File implements the Provider interface by writing messages to individual files.
// Each message is saved as a .eml-style text file in the configured output directory.
// Intended for development and debugging; messages are never actually delivered.
type File struct {
	outputDir string
}

// NewFile creates a File provider that writes messages to the given directory.
// If ProviderConfig.Endpoint is set, it is used as the output directory;
// otherwise defaults to "./mail_output".
func NewFile(cfg ProviderConfig) *File {
	dir := cfg.Endpoint
	if dir == "" {
		dir = defaultOutputDir
	}
	return &File{outputDir: dir}
}

func (f *File) GetName() string { return "file" }

// Send writes the message to a file named <timestamp>_<message-id>.eml
// in the output directory and returns a successful result.
func (f *File) Send(_ context.Context, msg *Message) (*DeliveryResult, error) {
	if err := os.MkdirAll(f.outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("file: create output dir: %w", err)
	}

	ts := time.Now().Format("20060102_150405")
	safeID := strings.ReplaceAll(msg.ID, "/", "_")
	filename := fmt.Sprintf("%s_%s.eml", ts, safeID)
	path := filepath.Join(f.outputDir, filename)

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\n", msg.From)
	fmt.Fprintf(&b, "To: %s\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\n", msg.Subject)
	for k, v := range msg.Headers {
		fmt.Fprintf(&b, "%s: %s\n", k, v)
	}
	fmt.Fprintf(&b, "X-Provider-Message-ID: file-%s\n", msg.ID)
	b.WriteString("\n")
	b.Write(msg.Body)

	if err := os.WriteFile(path, []byte(b.String()), 0o640); err != nil {
		return nil, fmt.Errorf("file: write %s: %w", path, err)
	}

	return &DeliveryResult{
		ProviderMessageID: "file-" + msg.ID,
		Status:            StatusSent,
		Timestamp:         time.Now(),
		Metadata:          map[string]string{"path": path},
	}, nil
}

// HealthCheck verifies the output directory is writable.
func (f *File) HealthCheck(_ context.Context) error {
	if err := os.MkdirAll(f.outputDir, 0o750); err != nil {
		return fmt.Errorf("file: output dir not writable: %w", err)
	}
	return nil
}
