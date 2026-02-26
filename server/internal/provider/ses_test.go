package provider

import (
	"encoding/json"
	"testing"
)

func TestSES_buildPayload_PlainTextOnly(t *testing.T) {
	s := &SES{}
	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"a@example.com"},
		Subject: "Test",
		Body:    []byte("plain text body"),
	}

	payload := s.buildPayload(msg)

	if payload.Content.Simple == nil {
		t.Fatal("expected Simple content, got nil")
	}
	if payload.Content.Raw != nil {
		t.Error("expected no Raw content for plain text message")
	}
	if payload.Content.Simple.Body.Text == nil {
		t.Fatal("expected Text body part")
	}
	if payload.Content.Simple.Body.Text.Data != "plain text body" {
		t.Errorf("expected body 'plain text body', got %q", payload.Content.Simple.Body.Text.Data)
	}
	if payload.Content.Simple.Body.Html != nil {
		t.Error("expected no Html body part for plain text message")
	}
}

func TestSES_buildPayload_HTMLAndText(t *testing.T) {
	s := &SES{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "text part",
		HTMLBody: "<h1>Hello</h1>",
	}

	payload := s.buildPayload(msg)

	if payload.Content.Simple == nil {
		t.Fatal("expected Simple content")
	}
	if payload.Content.Simple.Body.Text == nil {
		t.Fatal("expected Text body part")
	}
	if payload.Content.Simple.Body.Text.Data != "text part" {
		t.Errorf("expected text 'text part', got %q", payload.Content.Simple.Body.Text.Data)
	}
	if payload.Content.Simple.Body.Html == nil {
		t.Fatal("expected Html body part")
	}
	if payload.Content.Simple.Body.Html.Data != "<h1>Hello</h1>" {
		t.Errorf("expected HTML '<h1>Hello</h1>', got %q", payload.Content.Simple.Body.Html.Data)
	}
}

func TestSES_buildPayload_WithAttachments_UsesRaw(t *testing.T) {
	s := &SES{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "text body",
		HTMLBody: "<p>html</p>",
		Attachments: []Attachment{
			{
				Filename:    "report.pdf",
				ContentType: "application/pdf",
				Content:     []byte("PDF content"),
			},
		},
	}

	payload := s.buildPayload(msg)

	if payload.Content.Raw == nil {
		t.Fatal("expected Raw content when attachments are present")
	}
	if payload.Content.Simple != nil {
		t.Error("expected no Simple content when using Raw mode")
	}
	if payload.Content.Raw.Data == "" {
		t.Error("expected non-empty Raw.Data")
	}
}

func TestSES_buildPayload_NoAttachments_NoRaw(t *testing.T) {
	s := &SES{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		HTMLBody: "<p>html only</p>",
	}

	payload := s.buildPayload(msg)

	if payload.Content.Raw != nil {
		t.Error("expected no Raw content when no attachments")
	}
	if payload.Content.Simple == nil {
		t.Fatal("expected Simple content")
	}
	if payload.Content.Simple.Body.Html == nil {
		t.Fatal("expected Html body part")
	}
}

func TestSES_buildPayload_BackwardCompat(t *testing.T) {
	s := &SES{}
	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"a@example.com"},
		Subject: "Test",
		Body:    []byte("  raw body with spaces  "),
	}

	payload := s.buildPayload(msg)

	if payload.Content.Simple == nil {
		t.Fatal("expected Simple content")
	}
	if payload.Content.Simple.Body.Text == nil {
		t.Fatal("expected Text body part")
	}
	// Backward compat: uses trimmed Body when no TextBody/HTMLBody.
	if payload.Content.Simple.Body.Text.Data != "raw body with spaces" {
		t.Errorf("expected trimmed body, got %q", payload.Content.Simple.Body.Text.Data)
	}
}

func TestSES_buildPayload_JSONMarshal(t *testing.T) {
	s := &SES{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com", "b@example.com"},
		Subject:  "Test",
		TextBody: "hello",
		HTMLBody: "<p>hello</p>",
	}

	payload := s.buildPayload(msg)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	// Verify Simple is present and Raw is omitted.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	content := raw["Content"].(map[string]interface{})
	if _, ok := content["Simple"]; !ok {
		t.Error("expected Simple key in JSON")
	}
	if _, ok := content["Raw"]; ok {
		t.Error("expected Raw to be omitted in JSON")
	}
}

func TestBuildRawMIME(t *testing.T) {
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "text body",
		HTMLBody: "<p>html body</p>",
		Attachments: []Attachment{
			{
				Filename:    "file.txt",
				ContentType: "text/plain",
				Content:     []byte("file content"),
			},
			{
				Filename:    "img.png",
				ContentType: "image/png",
				Content:     []byte("png data"),
				ContentID:   "img-cid",
				IsInline:    true,
			},
		},
	}

	raw, err := buildRawMIME(msg)
	if err != nil {
		t.Fatalf("buildRawMIME failed: %v", err)
	}

	rawStr := string(raw)
	// Check headers are present.
	if len(rawStr) == 0 {
		t.Fatal("expected non-empty raw MIME message")
	}

	// Verify the raw message contains expected parts.
	expectations := []string{
		"From: sender@example.com",
		"To: a@example.com",
		"Subject: Test",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed",
		"text/plain",
		"text/html",
		"text body",
		"<p>html body</p>",
	}
	for _, exp := range expectations {
		found := false
		for i := 0; i < len(rawStr)-len(exp)+1; i++ {
			if rawStr[i:i+len(exp)] == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected raw MIME to contain %q", exp)
		}
	}
}
