package provider

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestMSGraph_buildPayload_PlainTextOnly(t *testing.T) {
	mg := &MSGraph{}
	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"a@example.com"},
		Subject: "Test",
		Body:    []byte("plain text body"),
	}

	payload := mg.buildPayload(msg)

	if payload.Message.Body.ContentType != "Text" {
		t.Errorf("expected ContentType Text, got %s", payload.Message.Body.ContentType)
	}
	if payload.Message.Body.Content != "plain text body" {
		t.Errorf("expected body 'plain text body', got %q", payload.Message.Body.Content)
	}
	if len(payload.Message.Attachments) != 0 {
		t.Errorf("expected no attachments, got %d", len(payload.Message.Attachments))
	}
}

func TestMSGraph_buildPayload_HTMLBody(t *testing.T) {
	mg := &MSGraph{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		Body:     []byte("raw body"),
		HTMLBody: "<h1>Hello</h1>",
	}

	payload := mg.buildPayload(msg)

	if payload.Message.Body.ContentType != "HTML" {
		t.Errorf("expected ContentType HTML, got %s", payload.Message.Body.ContentType)
	}
	if payload.Message.Body.Content != "<h1>Hello</h1>" {
		t.Errorf("expected HTML content, got %q", payload.Message.Body.Content)
	}
}

func TestMSGraph_buildPayload_TextBodyPreferred(t *testing.T) {
	mg := &MSGraph{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "parsed text",
	}

	payload := mg.buildPayload(msg)

	if payload.Message.Body.ContentType != "Text" {
		t.Errorf("expected ContentType Text, got %s", payload.Message.Body.ContentType)
	}
	if payload.Message.Body.Content != "parsed text" {
		t.Errorf("expected text 'parsed text', got %q", payload.Message.Body.Content)
	}
}

func TestMSGraph_buildPayload_HTMLTakesPrecedence(t *testing.T) {
	mg := &MSGraph{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "text",
		HTMLBody: "<p>html</p>",
	}

	payload := mg.buildPayload(msg)

	// HTML should take precedence.
	if payload.Message.Body.ContentType != "HTML" {
		t.Errorf("expected ContentType HTML, got %s", payload.Message.Body.ContentType)
	}
	if payload.Message.Body.Content != "<p>html</p>" {
		t.Errorf("expected HTML content, got %q", payload.Message.Body.Content)
	}
}

func TestMSGraph_buildPayload_WithAttachments(t *testing.T) {
	mg := &MSGraph{}
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

	payload := mg.buildPayload(msg)

	if len(payload.Message.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(payload.Message.Attachments))
	}

	att0 := payload.Message.Attachments[0]
	if att0.OdataType != "#microsoft.graph.fileAttachment" {
		t.Errorf("expected odata.type #microsoft.graph.fileAttachment, got %s", att0.OdataType)
	}
	if att0.Name != "report.pdf" {
		t.Errorf("expected name report.pdf, got %s", att0.Name)
	}
	if att0.ContentType != "application/pdf" {
		t.Errorf("expected contentType application/pdf, got %s", att0.ContentType)
	}
	expectedBytes := base64.StdEncoding.EncodeToString([]byte("PDF content"))
	if att0.ContentBytes != expectedBytes {
		t.Errorf("expected contentBytes %q, got %q", expectedBytes, att0.ContentBytes)
	}
	if att0.IsInline {
		t.Error("expected isInline false for regular attachment")
	}

	att1 := payload.Message.Attachments[1]
	if !att1.IsInline {
		t.Error("expected isInline true for inline attachment")
	}
	if att1.ContentID != "logo-cid" {
		t.Errorf("expected contentId logo-cid, got %s", att1.ContentID)
	}
}

func TestMSGraph_buildPayload_JSONMarshal(t *testing.T) {
	mg := &MSGraph{}
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

	payload := mg.buildPayload(msg)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	// Verify it round-trips correctly.
	var decoded graphSendMailPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if len(decoded.Message.Attachments) != 1 {
		t.Errorf("expected 1 attachment after round-trip, got %d", len(decoded.Message.Attachments))
	}
	if decoded.Message.Body.ContentType != "HTML" {
		t.Errorf("expected HTML content type, got %s", decoded.Message.Body.ContentType)
	}
}
