package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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

func TestMSGraph_buildPayload_WithCCAndBCC(t *testing.T) {
	mg := &MSGraph{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"to@example.com"},
		CC:       []string{"cc1@example.com", "cc2@example.com"},
		BCC:      []string{"bcc@example.com"},
		Subject:  "Test CC/BCC",
		TextBody: "body",
	}

	payload := mg.buildPayload(msg)

	if len(payload.Message.ToRecipients) != 1 {
		t.Fatalf("expected 1 toRecipient, got %d", len(payload.Message.ToRecipients))
	}
	if payload.Message.ToRecipients[0].EmailAddress.Address != "to@example.com" {
		t.Errorf("expected to@example.com, got %s", payload.Message.ToRecipients[0].EmailAddress.Address)
	}

	if len(payload.Message.CcRecipients) != 2 {
		t.Fatalf("expected 2 ccRecipients, got %d", len(payload.Message.CcRecipients))
	}
	if payload.Message.CcRecipients[0].EmailAddress.Address != "cc1@example.com" {
		t.Errorf("expected cc1@example.com, got %s", payload.Message.CcRecipients[0].EmailAddress.Address)
	}

	if len(payload.Message.BccRecipients) != 1 {
		t.Fatalf("expected 1 bccRecipient, got %d", len(payload.Message.BccRecipients))
	}
	if payload.Message.BccRecipients[0].EmailAddress.Address != "bcc@example.com" {
		t.Errorf("expected bcc@example.com, got %s", payload.Message.BccRecipients[0].EmailAddress.Address)
	}

	// Verify JSON marshaling includes cc/bcc fields.
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	var decoded graphSendMailPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(decoded.Message.CcRecipients) != 2 {
		t.Errorf("expected 2 ccRecipients after round-trip, got %d", len(decoded.Message.CcRecipients))
	}
	if len(decoded.Message.BccRecipients) != 1 {
		t.Errorf("expected 1 bccRecipient after round-trip, got %d", len(decoded.Message.BccRecipients))
	}
}

func TestMSGraph_buildPayload_NoCCBCC_OmittedFromJSON(t *testing.T) {
	mg := &MSGraph{}
	msg := &Message{
		From:     "sender@example.com",
		To:       []string{"to@example.com"},
		Subject:  "No CC/BCC",
		TextBody: "body",
	}

	payload := mg.buildPayload(msg)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify ccRecipients and bccRecipients are omitted when empty.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal raw: %v", err)
	}
	var msgRaw map[string]json.RawMessage
	if err := json.Unmarshal(raw["message"], &msgRaw); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}
	if _, exists := msgRaw["ccRecipients"]; exists {
		t.Error("expected ccRecipients to be omitted when empty")
	}
	if _, exists := msgRaw["bccRecipients"]; exists {
		t.Error("expected bccRecipients to be omitted when empty")
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

// graphMockClient routes requests: token endpoint returns a token, Graph API returns configured response.
func newGraphMockClient(sendStatus int, sendBody string) *mockHTTPClient2 {
	return &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			// Token request (Azure AD)
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"access_token":"mock-token","expires_in":3600}`),
				}, nil
			}
			// Graph API request
			return &HTTPResponse{
				StatusCode: sendStatus,
				Body:       []byte(sendBody),
			}, nil
		},
	}
}

func TestMSGraph_Send_Success(t *testing.T) {
	client := newGraphMockClient(202, ``)

	mg := NewMSGraph(ProviderConfig{
		Type:         "msgraph",
		TenantID:     "test-tenant",
		ClientID:     "test-client",
		ClientSecret: "test-value",
		UserID:       "user@example.com",
	}, client)

	result, err := mg.Send(context.Background(), &Message{
		ID:       "msg-123",
		From:     "sender@example.com",
		To:       []string{"r@example.com"},
		Subject:  "Test",
		TextBody: "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSent {
		t.Errorf("expected status sent, got %q", result.Status)
	}
	if result.ProviderMessageID != "msg-123" {
		t.Errorf("expected message ID 'msg-123', got %q", result.ProviderMessageID)
	}
	if result.Metadata["provider"] != "msgraph" {
		t.Errorf("expected provider metadata 'msgraph', got %q", result.Metadata["provider"])
	}
}

func TestMSGraph_Send_CorrectURL(t *testing.T) {
	var capturedURL string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"access_token":"tok","expires_in":3600}`),
				}, nil
			}
			capturedURL = req.URL
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "user@contoso.com",
	}, client)

	_, _ = mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})

	expected := "https://graph.microsoft.com/v1.0/users/user@contoso.com/sendMail"
	if capturedURL != expected {
		t.Errorf("expected URL %q, got %q", expected, capturedURL)
	}
}

func TestMSGraph_Send_BearerToken(t *testing.T) {
	var capturedAuth string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"access_token":"my-access-token","expires_in":3600}`),
				}, nil
			}
			capturedAuth = req.Headers["Authorization"]
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "u@example.com",
	}, client)

	_, _ = mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})

	if capturedAuth != "Bearer my-access-token" {
		t.Errorf("expected 'Bearer my-access-token', got %q", capturedAuth)
	}
}

func TestMSGraph_Send_HTTPError(t *testing.T) {
	client := newGraphMockClient(400, `{"error":{"message":"bad request"}}`)

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "u@example.com",
	}, client)

	_, err := mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestMSGraph_Send_401RetryWithTokenRefresh(t *testing.T) {
	callCount := 0
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"access_token":"fresh-token","expires_in":3600}`),
				}, nil
			}
			callCount++
			if callCount == 1 {
				// First attempt: 401 (expired token)
				return &HTTPResponse{StatusCode: 401, Body: []byte(`{"error":"InvalidAuthenticationToken"}`)}, nil
			}
			// Retry after token refresh: success
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "u@example.com",
	}, client)

	result, err := mg.Send(context.Background(), &Message{
		ID: "retry-msg", From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if result.Status != StatusSent {
		t.Errorf("expected sent, got %q", result.Status)
	}
	if callCount != 2 {
		t.Errorf("expected 2 send attempts (1 fail + 1 retry), got %d", callCount)
	}
}

func TestMSGraph_Send_NetworkError(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"access_token":"tok","expires_in":3600}`),
				}, nil
			}
			return nil, fmt.Errorf("connection reset")
		},
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "u@example.com",
	}, client)

	_, err := mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestMSGraph_Send_TokenAcquireError(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 400,
					Body:       []byte(`{"error":"invalid_client"}`),
				}, nil
			}
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "bad-client",
		ClientSecret: "s", UserID: "u@example.com",
	}, client)

	_, err := mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})
	if err == nil {
		t.Fatal("expected error when token acquisition fails")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("expected token-related error, got %v", err)
	}
}

func TestMSGraph_HealthCheck_Success(t *testing.T) {
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"access_token":"tok","expires_in":3600}`),
				}, nil
			}
			if !strings.Contains(req.URL, "/v1.0/users/u@example.com") {
				t.Errorf("expected user endpoint, got %s", req.URL)
			}
			return &HTTPResponse{StatusCode: 200, Body: []byte(`{}`)}, nil
		},
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "u@example.com",
	}, client)

	if err := mg.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMSGraph_HealthCheck_Failure(t *testing.T) {
	client := newGraphMockClient(403, `{"error":"Access Denied"}`)
	// Override: make healthcheck return 403 but token endpoint return OK
	client.doFn = func(req *HTTPRequest) (*HTTPResponse, error) {
		if strings.Contains(req.URL, "login.microsoftonline.com") {
			return &HTTPResponse{
				StatusCode: 200,
				Body:       []byte(`{"access_token":"tok","expires_in":3600}`),
			}, nil
		}
		return &HTTPResponse{StatusCode: 403, Body: []byte(`{}`)}, nil
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "u@example.com",
	}, client)

	err := mg.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got %v", err)
	}
}

func TestMSGraph_GetName(t *testing.T) {
	mg := &MSGraph{}
	if mg.GetName() != "msgraph" {
		t.Errorf("expected 'msgraph', got %q", mg.GetName())
	}
}

func TestMSGraph_CustomEndpoint(t *testing.T) {
	var capturedURL string
	client := &mockHTTPClient2{
		doFn: func(req *HTTPRequest) (*HTTPResponse, error) {
			if strings.Contains(req.URL, "login.microsoftonline.com") {
				return &HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"access_token":"tok","expires_in":3600}`),
				}, nil
			}
			capturedURL = req.URL
			return &HTTPResponse{StatusCode: 202, Body: []byte(``)}, nil
		},
	}

	mg := NewMSGraph(ProviderConfig{
		Type: "msgraph", TenantID: "t", ClientID: "c",
		ClientSecret: "s", UserID: "u@example.com",
		Endpoint: "https://graph.microsoft.us",
	}, client)

	_, _ = mg.Send(context.Background(), &Message{
		From: "s@example.com", To: []string{"r@example.com"},
		Subject: "Test", TextBody: "body",
	})

	expected := "https://graph.microsoft.us/v1.0/users/u@example.com/sendMail"
	if capturedURL != expected {
		t.Errorf("expected URL %q, got %q", expected, capturedURL)
	}
}
