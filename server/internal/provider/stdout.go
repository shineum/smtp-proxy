package provider

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Stdout implements the Provider interface by writing messages to standard output.
// Intended for development and debugging; messages are never actually delivered.
type Stdout struct {
	writer io.Writer
}

// NewStdout creates a Stdout provider that prints messages to os.Stdout.
func NewStdout(_ ProviderConfig) *Stdout {
	return &Stdout{writer: os.Stdout}
}

func (s *Stdout) GetName() string { return "stdout" }

// Send prints the message details to stdout and returns a successful result.
func (s *Stdout) Send(_ context.Context, msg *Message) (*DeliveryResult, error) {
	var b strings.Builder
	b.WriteString("--- stdout provider: message ---\n")
	fmt.Fprintf(&b, "ID:      %s\n", msg.ID)
	fmt.Fprintf(&b, "From:    %s\n", msg.From)
	fmt.Fprintf(&b, "To:      %s\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\n", msg.Subject)
	for k, v := range msg.Headers {
		fmt.Fprintf(&b, "Header:  %s: %s\n", k, v)
	}
	fmt.Fprintf(&b, "Body:    (%d bytes)\n", len(msg.Body))
	b.WriteString("--- end ---\n")

	if _, err := io.WriteString(s.writer, b.String()); err != nil {
		return nil, fmt.Errorf("stdout: write: %w", err)
	}

	return &DeliveryResult{
		ProviderMessageID: "stdout-" + msg.ID,
		Status:            StatusSent,
		Timestamp:         time.Now(),
	}, nil
}

// HealthCheck always returns nil since stdout is always available.
func (s *Stdout) HealthCheck(_ context.Context) error {
	return nil
}
