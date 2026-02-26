package provider

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSendGrid_buildPayload_PlainTextOnly(t *testing.T) {
	sg := &SendGrid{}
	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"a@example.com"},
		Subject: "Test",
		Body:    []byte("plain text body"),
	}

	payload := sg.buildPayload(msg)

	if len(payload.Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(payload.Content))
	}
	if payload.Content[0].Type != "text/plain" {
		t.Errorf("expected text/plain, got %s", payload.Content[0].Type)
	}
	if payload.Content[0].Value != "plain text body" {
		t.Errorf("expected body 'plain text body', got %q", payload.Content[0].Value)
	}
	if len(payload.Attachments) != 0 {
		t.Errorf("expected no attachments, got %d", len(payload.Attachments))
	}
}

func TestSendGrid_buildPayload_HTMLAndText(t *testing.T) {
	sg := &SendGrid{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		Body:     []byte("raw body"),
		TextBody: "text part",
		HTMLBody: "<h1>Hello</h1>",
	}

	payload := sg.buildPayload(msg)

	if len(payload.Content) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(payload.Content))
	}

	// First should be text/plain, second text/html.
	if payload.Content[0].Type != "text/plain" {
		t.Errorf("expected first content text/plain, got %s", payload.Content[0].Type)
	}
	if payload.Content[0].Value != "text part" {
		t.Errorf("expected text 'text part', got %q", payload.Content[0].Value)
	}
	if payload.Content[1].Type != "text/html" {
		t.Errorf("expected second content text/html, got %s", payload.Content[1].Type)
	}
	if payload.Content[1].Value != "<h1>Hello</h1>" {
		t.Errorf("expected HTML '<h1>Hello</h1>', got %q", payload.Content[1].Value)
	}
}

func TestSendGrid_buildPayload_HTMLOnly(t *testing.T) {
	sg := &SendGrid{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		HTMLBody: "<p>HTML only</p>",
	}

	payload := sg.buildPayload(msg)

	if len(payload.Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(payload.Content))
	}
	if payload.Content[0].Type != "text/html" {
		t.Errorf("expected text/html, got %s", payload.Content[0].Type)
	}
}

func TestSendGrid_buildPayload_WithAttachments(t *testing.T) {
	sg := &SendGrid{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "body",
		Attachments: []Attachment{
			{
				Filename:    "report.pdf",
				ContentType: "application/pdf",
				Content:     []byte("PDF content"),
				IsInline:    false,
			},
			{
				Filename:    "logo.png",
				ContentType: "image/png",
				Content:     []byte("PNG data"),
				ContentID:   "logo-cid",
				IsInline:    true,
			},
		},
	}

	payload := sg.buildPayload(msg)

	if len(payload.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(payload.Attachments))
	}

	att0 := payload.Attachments[0]
	if att0.Filename != "report.pdf" {
		t.Errorf("expected filename report.pdf, got %s", att0.Filename)
	}
	if att0.Type != "application/pdf" {
		t.Errorf("expected type application/pdf, got %s", att0.Type)
	}
	if att0.Disposition != "attachment" {
		t.Errorf("expected disposition attachment, got %s", att0.Disposition)
	}
	expectedContent := base64.StdEncoding.EncodeToString([]byte("PDF content"))
	if att0.Content != expectedContent {
		t.Errorf("expected base64 content %q, got %q", expectedContent, att0.Content)
	}

	att1 := payload.Attachments[1]
	if att1.Disposition != "inline" {
		t.Errorf("expected disposition inline, got %s", att1.Disposition)
	}
	if att1.ContentID != "logo-cid" {
		t.Errorf("expected content_id logo-cid, got %s", att1.ContentID)
	}
}

func TestSendGrid_buildPayload_JSONMarshal(t *testing.T) {
	sg := &SendGrid{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		HTMLBody: "<b>Bold</b>",
		Attachments: []Attachment{
			{
				Filename:    "file.txt",
				ContentType: "text/plain",
				Content:     []byte("data"),
			},
		},
	}

	payload := sg.buildPayload(msg)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	// Verify it can be unmarshalled back.
	var decoded sendgridPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if len(decoded.Attachments) != 1 {
		t.Errorf("expected 1 attachment after round-trip, got %d", len(decoded.Attachments))
	}
}
