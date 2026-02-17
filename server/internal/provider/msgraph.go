package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	graphDefaultEndpoint = "https://graph.microsoft.com"
	graphSendMailPathFmt = "/v1.0/users/%s/sendMail"
)

// MSGraph implements the Provider interface for Microsoft Graph Mail API.
// It uses OAuth2 client credentials flow for authentication and auto-retries
// on 401 with token refresh.
type MSGraph struct {
	userID       string
	endpoint     string
	client       HTTPClient
	tokenManager *TokenManager
}

// NewMSGraph creates a Microsoft Graph provider from the given configuration.
func NewMSGraph(cfg ProviderConfig, client HTTPClient) *MSGraph {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = graphDefaultEndpoint
	}
	tm := NewTokenManager(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, client)
	return &MSGraph{
		userID:       cfg.UserID,
		endpoint:     endpoint,
		client:       client,
		tokenManager: tm,
	}
}

func (m *MSGraph) GetName() string { return "msgraph" }

// Send delivers a message via the Microsoft Graph sendMail API.
// On 401 responses, it invalidates the token and retries once.
func (m *MSGraph) Send(ctx context.Context, msg *Message) (*DeliveryResult, error) {
	result, err := m.sendWithToken(msg)
	if err == nil {
		return result, nil
	}

	// Auto-retry on 401: refresh token and try again.
	var pe *ProviderError
	if isProviderError(err, &pe) && pe.StatusCode == 401 {
		m.tokenManager.InvalidateToken()
		return m.sendWithToken(msg)
	}

	return nil, err
}

func (m *MSGraph) sendWithToken(msg *Message) (*DeliveryResult, error) {
	token, err := m.tokenManager.GetToken()
	if err != nil {
		return nil, fmt.Errorf("msgraph: acquire token: %w", err)
	}

	payload := m.buildPayload(msg)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("msgraph: marshal request: %w", err)
	}

	sendURL := m.endpoint + fmt.Sprintf(graphSendMailPathFmt, m.userID)
	resp, err := m.client.Do(&HTTPRequest{
		Method: "POST",
		URL:    sendURL,
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
			"Content-Type":  "application/json",
		},
		Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("msgraph: send request: %w", err)
	}

	// Graph API returns 202 Accepted for sendMail.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &DeliveryResult{
			ProviderMessageID: msg.ID,
			Status:            StatusSent,
			Timestamp:         time.Now(),
			Metadata: map[string]string{
				"status_code": fmt.Sprintf("%d", resp.StatusCode),
				"provider":    "msgraph",
			},
		}, nil
	}

	return nil, ClassifyHTTPError("msgraph", resp.StatusCode, string(resp.Body))
}

// HealthCheck verifies MS Graph API connectivity.
func (m *MSGraph) HealthCheck(ctx context.Context) error {
	token, err := m.tokenManager.GetToken()
	if err != nil {
		return fmt.Errorf("msgraph: health check token: %w", err)
	}

	resp, err := m.client.Do(&HTTPRequest{
		Method: "GET",
		URL:    m.endpoint + fmt.Sprintf("/v1.0/users/%s", m.userID),
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
		},
	})
	if err != nil {
		return fmt.Errorf("msgraph: health check request: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("msgraph: health check returned status %d", resp.StatusCode)
	}
	return nil
}

// isProviderError extracts a ProviderError from an error if present.
func isProviderError(err error, target **ProviderError) bool {
	return errors.As(err, target)
}

// graphSendMailPayload matches the Microsoft Graph sendMail JSON schema.
type graphSendMailPayload struct {
	Message graphMessage `json:"message"`
}

type graphMessage struct {
	Subject      string              `json:"subject"`
	Body         graphBody           `json:"body"`
	ToRecipients []graphRecipient    `json:"toRecipients"`
	From         *graphRecipient     `json:"from,omitempty"`
}

type graphBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type graphRecipient struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
}

type graphEmailAddress struct {
	Address string `json:"address"`
}

func (m *MSGraph) buildPayload(msg *Message) graphSendMailPayload {
	recipients := make([]graphRecipient, len(msg.To))
	for i, addr := range msg.To {
		recipients[i] = graphRecipient{
			EmailAddress: graphEmailAddress{Address: addr},
		}
	}

	return graphSendMailPayload{
		Message: graphMessage{
			Subject: msg.Subject,
			Body: graphBody{
				ContentType: "Text",
				Content:     string(msg.Body),
			},
			ToRecipients: recipients,
			From: &graphRecipient{
				EmailAddress: graphEmailAddress{Address: msg.From},
			},
		},
	}
}
