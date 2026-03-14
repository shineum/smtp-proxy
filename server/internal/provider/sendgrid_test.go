package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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

func TestSendGrid_Send_Success(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{
				StatusCode: 202,
				Headers:    map[string]string{"X-Message-Id": "sg-msg-001"},
				Body:       []byte(``),
			}, nil
		},
	}

	sg := NewSendGrid(ProviderConfig{
		Type:   "sendgrid",
		APIKey: "test-key",
	}, client)

	result, err := sg.Send(context.Background(), &Message{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProviderMessageID != "sg-msg-001" {
		t.Errorf("expected message ID 'sg-msg-001', got %q", result.ProviderMessageID)
	}
	if result.Status != StatusSent {
		t.Errorf("expected status sent, got %q", result.Status)
	}
}

func TestSendGrid_Send_AuthHeader(t *testing.T) {
	var capturedAuth string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedAuth = req.Headers["Authorization"]
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	sg := NewSendGrid(ProviderConfig{
		Type:   "sendgrid",
		APIKey: "SG.test-token",
	}, client)

	_, err := sg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedAuth != "Bearer SG.test-token" {
		t.Errorf("expected 'Bearer SG.test-token', got %q", capturedAuth)
	}
}

func TestSendGrid_Send_HTTPError(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{
				StatusCode: 401,
				Body:       []byte(`{"errors":[{"message":"invalid api key"}]}`),
			}, nil
		},
	}

	sg := NewSendGrid(ProviderConfig{Type: "sendgrid", APIKey: "bad-key"}, client)

	_, err := sg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	var pe *ProviderError
	if !isProviderError(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", pe.StatusCode)
	}
}

func TestSendGrid_Send_NetworkError(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	sg := NewSendGrid(ProviderConfig{Type: "sendgrid", APIKey: "key"}, client)

	_, err := sg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestSendGrid_Send_WithCCAndBCC(t *testing.T) {
	var capturedBody []byte
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedBody = req.Body
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	sg := NewSendGrid(ProviderConfig{Type: "sendgrid", APIKey: "key"}, client)

	_, err := sg.Send(context.Background(), &Message{
		From:     "s@example.com",
		To:       []string{"to@example.com"},
		CC:       []string{"cc@example.com"},
		BCC:      []string{"bcc@example.com"},
		Subject:  "Test",
		TextBody: "body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload sendgridPayload
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(payload.Personalizations[0].Cc) != 1 || payload.Personalizations[0].Cc[0].Email != "cc@example.com" {
		t.Errorf("expected CC cc@example.com, got %+v", payload.Personalizations[0].Cc)
	}
	if len(payload.Personalizations[0].Bcc) != 1 || payload.Personalizations[0].Bcc[0].Email != "bcc@example.com" {
		t.Errorf("expected BCC bcc@example.com, got %+v", payload.Personalizations[0].Bcc)
	}
}

func TestSendGrid_HealthCheck_Success(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if !strings.HasSuffix(req.URL, "/v3/scopes") {
				t.Errorf("expected scopes URL, got %s", req.URL)
			}
			return &HTTPResponse{StatusCode: 200, Body: []byte(`{"scopes":["mail.send"]}`)}, nil
		},
	}

	sg := NewSendGrid(ProviderConfig{Type: "sendgrid", APIKey: "key"}, client)

	if err := sg.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendGrid_HealthCheck_Failure(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{StatusCode: 403, Body: []byte(`{}`)}, nil
		},
	}

	sg := NewSendGrid(ProviderConfig{Type: "sendgrid", APIKey: "bad-key"}, client)

	err := sg.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got %v", err)
	}
}

func TestSendGrid_CustomEndpoint(t *testing.T) {
	var capturedURL string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedURL = req.URL
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	sg := NewSendGrid(ProviderConfig{
		Type:     "sendgrid",
		APIKey:   "key",
		Endpoint: "https://custom.sendgrid.example.com",
	}, client)

	_, _ = sg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})

	if capturedURL != "https://custom.sendgrid.example.com/v3/mail/send" {
		t.Errorf("expected custom endpoint URL, got %s", capturedURL)
	}
}
