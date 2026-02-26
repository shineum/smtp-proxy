package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

const (
	sendgridDefaultEndpoint = "https://api.sendgrid.com"
	sendgridSendPath        = "/v3/mail/send"
	sendgridScopesPath      = "/v3/scopes"
)

// SendGrid implements the Provider interface for the SendGrid v3 API.
type SendGrid struct {
	apiKey   string
	endpoint string
	client   HTTPClient
}

// NewSendGrid creates a SendGrid provider from the given configuration.
func NewSendGrid(cfg ProviderConfig, client HTTPClient) *SendGrid {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = sendgridDefaultEndpoint
	}
	return &SendGrid{
		apiKey:   cfg.APIKey,
		endpoint: endpoint,
		client:   client,
	}
}

func (s *SendGrid) GetName() string { return "sendgrid" }

// Send delivers a message via the SendGrid v3 Mail Send API.
func (s *SendGrid) Send(ctx context.Context, msg *Message) (*DeliveryResult, error) {
	payload := s.buildPayload(msg)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("sendgrid: marshal request: %w", err)
	}

	resp, err := s.client.Do(&HTTPRequest{
		Method: "POST",
		URL:    s.endpoint + sendgridSendPath,
		Headers: map[string]string{
			"Authorization": "Bearer " + s.apiKey,
			"Content-Type":  "application/json",
		},
		Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("sendgrid: send request: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		messageID := ""
		if resp.Headers != nil {
			messageID = resp.Headers["X-Message-Id"]
		}
		return &DeliveryResult{
			ProviderMessageID: messageID,
			Status:            StatusSent,
			Timestamp:         time.Now(),
			Metadata: map[string]string{
				"status_code": fmt.Sprintf("%d", resp.StatusCode),
			},
		}, nil
	}

	return nil, ClassifyHTTPError("sendgrid", resp.StatusCode, string(resp.Body))
}

// HealthCheck verifies SendGrid API connectivity by calling the scopes endpoint.
func (s *SendGrid) HealthCheck(ctx context.Context) error {
	resp, err := s.client.Do(&HTTPRequest{
		Method: "GET",
		URL:    s.endpoint + sendgridScopesPath,
		Headers: map[string]string{
			"Authorization": "Bearer " + s.apiKey,
		},
	})
	if err != nil {
		return fmt.Errorf("sendgrid: health check request: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("sendgrid: health check returned status %d", resp.StatusCode)
	}
	return nil
}

// sendgridPayload matches the SendGrid v3 mail/send JSON schema.
type sendgridPayload struct {
	Personalizations []sendgridPersonalization `json:"personalizations"`
	From             sendgridEmail             `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []sendgridContent         `json:"content"`
	Headers          map[string]string         `json:"headers,omitempty"`
	Attachments      []sendgridAttachment      `json:"attachments,omitempty"`
}

type sendgridPersonalization struct {
	To []sendgridEmail `json:"to"`
}

type sendgridEmail struct {
	Email string `json:"email"`
}

type sendgridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type sendgridAttachment struct {
	Content     string `json:"content"`                // base64 encoded
	Type        string `json:"type"`                   // MIME type
	Filename    string `json:"filename"`
	Disposition string `json:"disposition"`             // "attachment" or "inline"
	ContentID   string `json:"content_id,omitempty"`
}

func (s *SendGrid) buildPayload(msg *Message) sendgridPayload {
	tos := make([]sendgridEmail, len(msg.To))
	for i, addr := range msg.To {
		tos[i] = sendgridEmail{Email: addr}
	}

	// Build content parts: prefer parsed bodies, fall back to raw Body.
	var content []sendgridContent
	if msg.TextBody != "" {
		content = append(content, sendgridContent{Type: "text/plain", Value: msg.TextBody})
	}
	if msg.HTMLBody != "" {
		content = append(content, sendgridContent{Type: "text/html", Value: msg.HTMLBody})
	}
	if len(content) == 0 {
		content = []sendgridContent{
			{Type: "text/plain", Value: string(msg.Body)},
		}
	}

	payload := sendgridPayload{
		Personalizations: []sendgridPersonalization{
			{To: tos},
		},
		From:    sendgridEmail{Email: msg.From},
		Subject: msg.Subject,
		Content: content,
		Headers: msg.Headers,
	}

	// Attach files if present.
	for _, att := range msg.Attachments {
		disposition := "attachment"
		if att.IsInline {
			disposition = "inline"
		}
		payload.Attachments = append(payload.Attachments, sendgridAttachment{
			Content:     base64.StdEncoding.EncodeToString(att.Content),
			Type:        att.ContentType,
			Filename:    att.Filename,
			Disposition: disposition,
			ContentID:   att.ContentID,
		})
	}

	return payload
}
