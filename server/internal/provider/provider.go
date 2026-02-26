package provider

import (
	"context"
	"time"
)

// Provider defines the interface for sending email through an ESP.
type Provider interface {
	// Send delivers a message through the ESP and returns a delivery result.
	Send(ctx context.Context, msg *Message) (*DeliveryResult, error)
	// GetName returns the provider's identifier (e.g., "sendgrid", "ses").
	GetName() string
	// HealthCheck verifies the provider is reachable and functional.
	HealthCheck(ctx context.Context) error
}

// HTTPClient abstracts HTTP operations for testability.
type HTTPClient interface {
	Do(req *HTTPRequest) (*HTTPResponse, error)
}

// HTTPRequest represents an outgoing HTTP request.
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

// HTTPResponse represents an HTTP response from a provider API.
type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Message represents an email message to be delivered.
type Message struct {
	ID          string
	TenantID    string
	From        string
	To          []string
	Subject     string
	Headers     map[string]string
	Body        []byte       // raw body (kept for backward compat, used by stdout/file)
	TextBody    string       // parsed plain text body
	HTMLBody    string       // parsed HTML body
	Attachments []Attachment // parsed attachments
}

// Attachment represents a single MIME attachment or inline part.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
	ContentID   string // for inline images (cid:xxx)
	IsInline    bool
}

// DeliveryResult contains the outcome of a delivery attempt.
type DeliveryResult struct {
	ProviderMessageID string
	Status            DeliveryStatus
	Timestamp         time.Time
	Metadata          map[string]string
}

// DeliveryStatus represents the outcome of an ESP delivery.
type DeliveryStatus string

const (
	StatusSent    DeliveryStatus = "sent"
	StatusFailed  DeliveryStatus = "failed"
	StatusBounced DeliveryStatus = "bounced"
)
