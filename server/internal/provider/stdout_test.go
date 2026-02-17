package provider

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestStdout_GetName(t *testing.T) {
	p := NewStdout(ProviderConfig{Type: "stdout"})
	if p.GetName() != "stdout" {
		t.Errorf("expected name stdout, got %s", p.GetName())
	}
}

func TestStdout_Send(t *testing.T) {
	var buf bytes.Buffer
	p := &Stdout{writer: &buf}

	msg := &Message{
		ID:      "test-123",
		From:    "sender@example.com",
		To:      []string{"a@example.com", "b@example.com"},
		Subject: "Test Subject",
		Headers: map[string]string{"X-Custom": "value"},
		Body:    []byte("Hello, World!"),
	}

	result, err := p.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ProviderMessageID != "stdout-test-123" {
		t.Errorf("expected provider message ID stdout-test-123, got %s", result.ProviderMessageID)
	}
	if result.Status != StatusSent {
		t.Errorf("expected status sent, got %s", result.Status)
	}

	output := buf.String()
	if !strings.Contains(output, "sender@example.com") {
		t.Error("expected output to contain sender address")
	}
	if !strings.Contains(output, "a@example.com, b@example.com") {
		t.Error("expected output to contain recipients")
	}
	if !strings.Contains(output, "Test Subject") {
		t.Error("expected output to contain subject")
	}
	if !strings.Contains(output, "X-Custom: value") {
		t.Error("expected output to contain custom header")
	}
	if !strings.Contains(output, "13 bytes") {
		t.Error("expected output to contain body size")
	}
}

func TestStdout_HealthCheck(t *testing.T) {
	p := NewStdout(ProviderConfig{Type: "stdout"})
	if err := p.HealthCheck(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
