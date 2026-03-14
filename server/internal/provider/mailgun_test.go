package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestMailgun_buildForm_PlainTextOnly(t *testing.T) {
	mg := &Mailgun{}
	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"a@example.com"},
		Subject: "Test",
		Body:    []byte("plain text body"),
	}

	form := mg.buildForm(msg)

	if form.Get("text") != "plain text body" {
		t.Errorf("expected text 'plain text body', got %q", form.Get("text"))
	}
	if form.Get("html") != "" {
		t.Error("expected no html field for plain text message")
	}
}

func TestMailgun_buildForm_HTMLAndText(t *testing.T) {
	mg := &Mailgun{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "text part",
		HTMLBody: "<h1>Hello</h1>",
	}

	form := mg.buildForm(msg)

	if form.Get("text") != "text part" {
		t.Errorf("expected text 'text part', got %q", form.Get("text"))
	}
	if form.Get("html") != "<h1>Hello</h1>" {
		t.Errorf("expected html '<h1>Hello</h1>', got %q", form.Get("html"))
	}
}

func TestMailgun_buildForm_TextBodyFallback(t *testing.T) {
	mg := &Mailgun{}
	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"a@example.com"},
		Subject: "Test",
		Body:    []byte("raw body fallback"),
	}

	form := mg.buildForm(msg)

	if form.Get("text") != "raw body fallback" {
		t.Errorf("expected text 'raw body fallback', got %q", form.Get("text"))
	}
}

func TestMailgun_buildForm_PrefersParsedText(t *testing.T) {
	mg := &Mailgun{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		Body:     []byte("raw body"),
		TextBody: "parsed text",
	}

	form := mg.buildForm(msg)

	if form.Get("text") != "parsed text" {
		t.Errorf("expected text 'parsed text', got %q", form.Get("text"))
	}
}

func TestMailgun_buildMultipartForm_WithAttachments(t *testing.T) {
	mg := &Mailgun{}
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
			{
				Filename:    "logo.png",
				ContentType: "image/png",
				Content:     []byte("PNG data"),
				ContentID:   "logo-cid",
				IsInline:    true,
			},
		},
	}

	body, contentType, err := mg.buildMultipartForm(msg)
	if err != nil {
		t.Fatalf("buildMultipartForm failed: %v", err)
	}

	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("expected multipart/form-data content type, got %s", contentType)
	}

	bodyStr := string(body)
	// Check form fields are present.
	if !strings.Contains(bodyStr, "sender@example.com") {
		t.Error("expected body to contain from address")
	}
	if !strings.Contains(bodyStr, "text body") {
		t.Error("expected body to contain text body")
	}
	if !strings.Contains(bodyStr, "<p>html</p>") {
		t.Error("expected body to contain html body")
	}
	// Check attachments are present.
	if !strings.Contains(bodyStr, "report.pdf") {
		t.Error("expected body to contain attachment filename report.pdf")
	}
	if !strings.Contains(bodyStr, "logo.png") {
		t.Error("expected body to contain inline filename logo.png")
	}
	if !strings.Contains(bodyStr, "PDF content") {
		t.Error("expected body to contain attachment content")
	}
	// Check inline uses correct field name.
	if !strings.Contains(bodyStr, `name="inline"`) {
		t.Error("expected inline attachment to use field name 'inline'")
	}
	if !strings.Contains(bodyStr, `name="attachment"`) {
		t.Error("expected regular attachment to use field name 'attachment'")
	}
}

func TestMailgun_Send_UsesMultipartForAttachments(t *testing.T) {
	var capturedContentType string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedContentType = req.Headers["Content-Type"]
			return &HTTPResponse{
				StatusCode: 200,
				Body:       []byte(`{"id":"<msg@mg>","message":"Queued"}`),
			}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type:   "mailgun",
		APIKey: "key-test",
		Domain: "mg.example.com",
	}, client)

	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "hello",
		Attachments: []Attachment{
			{
				Filename:    "file.txt",
				ContentType: "text/plain",
				Content:     []byte("data"),
			},
		},
	}

	_, err := mg.Send(nil, msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.HasPrefix(capturedContentType, "multipart/form-data") {
		t.Errorf("expected multipart/form-data, got %s", capturedContentType)
	}
}

func TestMailgun_Send_UsesURLEncodedWithoutAttachments(t *testing.T) {
	var capturedContentType string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedContentType = req.Headers["Content-Type"]
			return &HTTPResponse{
				StatusCode: 200,
				Body:       []byte(`{"id":"<msg@mg>","message":"Queued"}`),
			}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type:   "mailgun",
		APIKey: "key-test",
		Domain: "mg.example.com",
	}, client)

	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"a@example.com"},
		Subject:  "Test",
		TextBody: "hello",
	}

	_, err := mg.Send(nil, msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if capturedContentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected application/x-www-form-urlencoded, got %s", capturedContentType)
	}
}

// mockHTTPClient2 is a flexible mock for HTTP tests.
type mockHTTPClient2 struct {
	doFn func(req *HTTPRequest) (*HTTPResponse, error)
}

func (m *mockHTTPClient2) Do(req *HTTPRequest) (*HTTPResponse, error) {
	return m.doFn(req)
}

func TestMailgun_Send_Success(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{
				StatusCode: 200,
				Body:       []byte(`{"id":"<msg-id@mg>","message":"Queued. Thank you."}`),
			}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "key-test", Domain: "mg.example.com",
	}, client)

	result, err := mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProviderMessageID != "<msg-id@mg>" {
		t.Errorf("expected message ID '<msg-id@mg>', got %q", result.ProviderMessageID)
	}
	if result.Status != StatusSent {
		t.Errorf("expected status sent, got %q", result.Status)
	}
	if result.Metadata["message"] != "Queued. Thank you." {
		t.Errorf("expected metadata message, got %q", result.Metadata["message"])
	}
}

func TestMailgun_Send_BasicAuth(t *testing.T) {
	var capturedAuth string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedAuth = req.Headers["Authorization"]
			return &HTTPResponse{StatusCode: 200, Body: []byte(`{"id":"<id>","message":"Queued"}`)}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "key-abc123", Domain: "mg.example.com",
	}, client)

	_, _ = mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})

	expectedAuth := "Basic " + basicAuth("api", "key-abc123")
	if capturedAuth != expectedAuth {
		t.Errorf("expected %q, got %q", expectedAuth, capturedAuth)
	}
}

func TestMailgun_Send_CorrectURL(t *testing.T) {
	var capturedURL string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedURL = req.URL
			return &HTTPResponse{StatusCode: 200, Body: []byte(`{"id":"<id>","message":"Queued"}`)}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "key", Domain: "mg.example.com",
	}, client)

	_, _ = mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})

	expected := "https://api.mailgun.net/v3/mg.example.com/messages"
	if capturedURL != expected {
		t.Errorf("expected URL %q, got %q", expected, capturedURL)
	}
}

func TestMailgun_Send_HTTPError(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{
				StatusCode: 401,
				Body:       []byte(`{"message":"Forbidden"}`),
			}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "bad-key", Domain: "mg.example.com",
	}, client)

	_, err := mg.Send(context.Background(), &Message{
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
}

func TestMailgun_Send_NetworkError(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return nil, fmt.Errorf("dns resolution failed")
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "key", Domain: "mg.example.com",
	}, client)

	_, err := mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	if !strings.Contains(err.Error(), "dns resolution failed") {
		t.Errorf("expected dns error, got %v", err)
	}
}

func TestMailgun_HealthCheck_Success(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if !strings.Contains(req.URL, "/v3/domains/mg.example.com") {
				t.Errorf("expected domains URL, got %s", req.URL)
			}
			return &HTTPResponse{StatusCode: 200, Body: []byte(`{}`)}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "key", Domain: "mg.example.com",
	}, client)

	if err := mg.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMailgun_HealthCheck_Failure(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{StatusCode: 404, Body: []byte(`{}`)}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "key", Domain: "bad-domain.com",
	}, client)

	err := mg.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got %v", err)
	}
}

func TestMailgun_CustomEndpoint(t *testing.T) {
	var capturedURL string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			capturedURL = req.URL
			return &HTTPResponse{StatusCode: 200, Body: []byte(`{"id":"<id>","message":"Queued"}`)}, nil
		},
	}

	mg := NewMailgun(ProviderConfig{
		Type: "mailgun", APIKey: "key", Domain: "mg.example.com",
		Endpoint: "https://api.eu.mailgun.net",
	}, client)

	_, _ = mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})

	expected := "https://api.eu.mailgun.net/v3/mg.example.com/messages"
	if capturedURL != expected {
		t.Errorf("expected URL %q, got %q", expected, capturedURL)
	}
}
