package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

func TestSES_Send_Success(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{
			StatusCode: 200,
			Body:       []byte(`{"MessageId":"ses-msg-001"}`),
		},
	}

	ses := NewSES(ProviderConfig{
		Type:   "ses",
		Region: "us-east-1",
	}, mock)

	result, err := ses.Send(context.Background(), &Message{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test Subject",
		TextBody: "Hello, world!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProviderMessageID != "ses-msg-001" {
		t.Errorf("expected message ID 'ses-msg-001', got %q", result.ProviderMessageID)
	}
	if result.Status != StatusSent {
		t.Errorf("expected status sent, got %q", result.Status)
	}
	if result.Metadata["region"] != "us-east-1" {
		t.Errorf("expected region metadata 'us-east-1', got %q", result.Metadata["region"])
	}

	// Verify the request was sent to the correct URL.
	if mock.lastReq == nil {
		t.Fatal("expected request to be sent")
	}
	expectedURL := "https://email.us-east-1.amazonaws.com/v2/email/outbound-emails"
	if mock.lastReq.URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, mock.lastReq.URL)
	}

	// Verify JSON payload structure.
	var payload sesPayload
	if err := json.Unmarshal(mock.lastReq.Body, &payload); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if payload.FromEmailAddress != "sender@example.com" {
		t.Errorf("expected from 'sender@example.com', got %q", payload.FromEmailAddress)
	}
	if len(payload.Destination.ToAddresses) != 1 || payload.Destination.ToAddresses[0] != "recipient@example.com" {
		t.Errorf("unexpected To addresses: %v", payload.Destination.ToAddresses)
	}
}

func TestSES_Send_WithSigV4(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{
			StatusCode: 200,
			Body:       []byte(`{"MessageId":"signed-msg"}`),
		},
	}

	ses := NewSES(ProviderConfig{
		Type:      "ses",
		Region:    "us-west-2",
		APIKey:    "test-access-key",
		SecretKey: "test-secret-key",
	}, mock)

	_, err := ses.Send(context.Background(), &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Signed",
		TextBody: "body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request was signed.
	authHeader := mock.lastReq.Headers["Authorization"]
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		t.Errorf("expected AWS Sig V4 Authorization header, got %q", authHeader)
	}
	if !strings.Contains(authHeader, "us-west-2/ses/aws4_request") {
		t.Errorf("expected us-west-2/ses scope in Authorization, got %q", authHeader)
	}
}

func TestSES_Send_HTTPError(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{
			StatusCode: 400,
			Body:       []byte(`{"message":"Invalid email address"}`),
		},
	}

	ses := NewSES(ProviderConfig{
		Type:   "ses",
		Region: "us-east-1",
	}, mock)

	_, err := ses.Send(context.Background(), &Message{
		From:     "sender@example.com",
		To:       []string{"invalid"},
		Subject:  "Test",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestSES_Send_NetworkError(t *testing.T) {
	mock := &sesMockHTTPClient{
		err: fmt.Errorf("network timeout"),
	}

	ses := NewSES(ProviderConfig{
		Type:   "ses",
		Region: "us-east-1",
	}, mock)

	_, err := ses.Send(context.Background(), &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("expected network timeout error, got %v", err)
	}
}

func TestSES_HealthCheck_Success(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{StatusCode: 200, Body: []byte(`{}`)},
	}

	ses := NewSES(ProviderConfig{
		Type:   "ses",
		Region: "us-east-1",
	}, mock)

	err := ses.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedURL := "https://email.us-east-1.amazonaws.com/v2/email/account"
	if mock.lastReq.URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, mock.lastReq.URL)
	}
}

func TestSES_HealthCheck_Failure(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{StatusCode: 403, Body: []byte(`{"message":"Forbidden"}`)},
	}

	ses := NewSES(ProviderConfig{
		Type:   "ses",
		Region: "us-east-1",
	}, mock)

	err := ses.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected status 403 in error, got %v", err)
	}
}

func TestSES_GetName(t *testing.T) {
	ses := &SES{}
	if ses.GetName() != "ses" {
		t.Errorf("expected 'ses', got %q", ses.GetName())
	}
}

func TestSES_CustomEndpoint(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{StatusCode: 200, Body: []byte(`{"MessageId":"id"}`)},
	}

	ses := NewSES(ProviderConfig{
		Type:     "ses",
		Region:   "us-east-1",
		Endpoint: "https://custom-ses.example.com",
	}, mock)

	_, err := ses.Send(context.Background(), &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedURL := "https://custom-ses.example.com/v2/email/outbound-emails"
	if mock.lastReq.URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, mock.lastReq.URL)
	}
}
