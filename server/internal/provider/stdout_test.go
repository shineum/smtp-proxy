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

func TestStdout_Send_WithHTMLAndAttachments(t *testing.T) {
	var buf bytes.Buffer
	p := &Stdout{writer: &buf}

	msg := &Message{
		ID:       "test-html-123",
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "HTML Test",
		Body:     []byte("raw body"),
		TextBody: "text part",
		HTMLBody: "<h1>Hello</h1>",
		Attachments: []Attachment{
			{
				Filename:    "report.pdf",
				ContentType: "application/pdf",
				Content:     []byte("PDF data here"),
			},
			{
				Filename:    "logo.png",
				ContentType: "image/png",
				Content:     []byte("PNG"),
				ContentID:   "logo-cid",
				IsInline:    true,
			},
		},
	}

	result, err := p.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != StatusSent {
		t.Errorf("expected status sent, got %s", result.Status)
	}

	output := buf.String()
	if !strings.Contains(output, "Text:    (9 chars)") {
		t.Error("expected output to contain text body info")
	}
	if !strings.Contains(output, "HTML:    (14 chars)") {
		t.Error("expected output to contain html body info")
	}
	if !strings.Contains(output, "Attach:  2 file(s)") {
		t.Error("expected output to contain attachment count")
	}
	if !strings.Contains(output, "report.pdf") {
		t.Error("expected output to contain attachment filename")
	}
	if !strings.Contains(output, "logo.png") {
		t.Error("expected output to contain inline filename")
	}
}

func TestStdout_HealthCheck(t *testing.T) {
	p := NewStdout(ProviderConfig{Type: "stdout"})
	if err := p.HealthCheck(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
