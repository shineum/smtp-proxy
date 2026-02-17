package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	mailgunDefaultEndpoint = "https://api.mailgun.net"
)

// Mailgun implements the Provider interface for the Mailgun API.
type Mailgun struct {
	apiKey   string
	domain   string
	endpoint string
	client   HTTPClient
}

// NewMailgun creates a Mailgun provider from the given configuration.
func NewMailgun(cfg ProviderConfig, client HTTPClient) *Mailgun {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = mailgunDefaultEndpoint
	}
	return &Mailgun{
		apiKey:   cfg.APIKey,
		domain:   cfg.Domain,
		endpoint: endpoint,
		client:   client,
	}
}

func (m *Mailgun) GetName() string { return "mailgun" }

// Send delivers a message via the Mailgun messages API.
func (m *Mailgun) Send(ctx context.Context, msg *Message) (*DeliveryResult, error) {
	form := m.buildForm(msg)
	body := form.Encode()

	resp, err := m.client.Do(&HTTPRequest{
		Method: "POST",
		URL:    fmt.Sprintf("%s/v3/%s/messages", m.endpoint, m.domain),
		Headers: map[string]string{
			"Authorization": "Basic " + basicAuth("api", m.apiKey),
			"Content-Type":  "application/x-www-form-urlencoded",
		},
		Body: []byte(body),
	})
	if err != nil {
		return nil, fmt.Errorf("mailgun: send request: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var mgResp mailgunResponse
		messageID := ""
		if err := json.Unmarshal(resp.Body, &mgResp); err == nil {
			messageID = mgResp.ID
		}
		return &DeliveryResult{
			ProviderMessageID: messageID,
			Status:            StatusSent,
			Timestamp:         time.Now(),
			Metadata: map[string]string{
				"message":     mgResp.Message,
				"status_code": fmt.Sprintf("%d", resp.StatusCode),
			},
		}, nil
	}

	return nil, ClassifyHTTPError("mailgun", resp.StatusCode, string(resp.Body))
}

// HealthCheck verifies Mailgun API connectivity by requesting domain info.
func (m *Mailgun) HealthCheck(ctx context.Context) error {
	resp, err := m.client.Do(&HTTPRequest{
		Method: "GET",
		URL:    fmt.Sprintf("%s/v3/domains/%s", m.endpoint, m.domain),
		Headers: map[string]string{
			"Authorization": "Basic " + basicAuth("api", m.apiKey),
		},
	})
	if err != nil {
		return fmt.Errorf("mailgun: health check request: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("mailgun: health check returned status %d", resp.StatusCode)
	}
	return nil
}

type mailgunResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func (m *Mailgun) buildForm(msg *Message) url.Values {
	form := url.Values{}
	form.Set("from", msg.From)
	form.Set("to", strings.Join(msg.To, ","))
	form.Set("subject", msg.Subject)
	form.Set("text", string(msg.Body))

	for key, value := range msg.Headers {
		form.Set("h:"+key, value)
	}
	return form
}

// basicAuth encodes credentials as base64 for HTTP Basic Authentication.
func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}
