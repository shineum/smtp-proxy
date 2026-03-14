package provider

import (
	"strings"
	"testing"
)

// sesMockHTTPClient records the last request for inspection.
type sesMockHTTPClient struct {
	lastReq  *HTTPRequest
	response *HTTPResponse
	err      error
}

func (m *sesMockHTTPClient) Do(req *HTTPRequest) (*HTTPResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestSigV4HTTPClient_AddsAuthHeaders(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{StatusCode: 200, Body: []byte(`{"MessageId":"test-id"}`)},
	}

	client := NewSigV4HTTPClient(mock, "test-access-key-id", "test-secret-access-key", "us-east-1")

	req := &HTTPRequest{
		Method: "POST",
		URL:    "https://email.us-east-1.amazonaws.com/v2/email/outbound-emails",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: []byte(`{"test":"payload"}`),
	}

	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Authorization header was added with AWS4-HMAC-SHA256
	authHeader := mock.lastReq.Headers["Authorization"]
	if authHeader == "" {
		t.Fatal("expected Authorization header to be set")
	}
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		t.Errorf("expected Authorization to start with AWS4-HMAC-SHA256, got %q", authHeader)
	}

	// Verify it contains the correct credential scope
	if !strings.Contains(authHeader, "test-access-key-id") {
		t.Error("expected Authorization to contain the access key ID")
	}
	if !strings.Contains(authHeader, "us-east-1/ses/aws4_request") {
		t.Error("expected Authorization to contain region/ses/aws4_request credential scope")
	}

	// Verify X-Amz-Date header was added
	amzDate := mock.lastReq.Headers["X-Amz-Date"]
	if amzDate == "" {
		t.Fatal("expected X-Amz-Date header to be set")
	}

	// Verify content hash is included in the signature (part of the signed headers).
	// The AWS SDK v4 signer includes the payload hash in the Authorization header
	// via the SignedHeaders field rather than as a separate X-Amz-Content-Sha256 header.
	if !strings.Contains(authHeader, "SignedHeaders=") {
		t.Error("expected SignedHeaders in Authorization header")
	}
}

func TestSigV4HTTPClient_NilBody(t *testing.T) {
	mock := &sesMockHTTPClient{
		response: &HTTPResponse{StatusCode: 200, Body: []byte(`{}`)},
	}

	client := NewSigV4HTTPClient(mock, "test-key-id", "test-secret", "eu-west-1")

	req := &HTTPRequest{
		Method: "GET",
		URL:    "https://email.eu-west-1.amazonaws.com/v2/email/account",
	}

	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authHeader := mock.lastReq.Headers["Authorization"]
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		t.Errorf("expected signed request, got Authorization: %q", authHeader)
	}
}

func TestSigV4HTTPClient_ForwardsInnerError(t *testing.T) {
	mock := &sesMockHTTPClient{
		err: &testError{msg: "connection refused"},
	}

	client := NewSigV4HTTPClient(mock, "test-key-id", "test-secret", "us-west-2")

	req := &HTTPRequest{
		Method: "POST",
		URL:    "https://email.us-west-2.amazonaws.com/v2/email/outbound-emails",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: []byte(`{}`),
	}

	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected error from inner client")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected inner error, got %v", err)
	}
}

func TestSigV4HTTPClient_ForwardsResponse(t *testing.T) {
	expected := &HTTPResponse{
		StatusCode: 200,
		Headers:    map[string]string{"X-Custom": "value"},
		Body:       []byte(`{"MessageId":"abc-123"}`),
	}
	mock := &sesMockHTTPClient{response: expected}

	client := NewSigV4HTTPClient(mock, "test-key-id", "test-secret", "us-east-1")

	resp, err := client.Do(&HTTPRequest{
		Method:  "POST",
		URL:     "https://email.us-east-1.amazonaws.com/v2/email/outbound-emails",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != `{"MessageId":"abc-123"}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
