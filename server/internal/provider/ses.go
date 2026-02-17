package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	sesDefaultEndpointFmt = "https://email.%s.amazonaws.com"
	sesSendPath           = "/v2/email/outbound-emails"
)

// SES implements the Provider interface for AWS SES v2 API.
// It uses a configurable HTTP client for testability rather than the AWS SDK.
type SES struct {
	region   string
	endpoint string
	client   HTTPClient
}

// NewSES creates an AWS SES provider from the given configuration.
// The APIKey field in config is used as a placeholder; real AWS auth
// (Signature V4) should be handled by the HTTPClient wrapper in production.
func NewSES(cfg ProviderConfig, client HTTPClient) *SES {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf(sesDefaultEndpointFmt, cfg.Region)
	}
	return &SES{
		region:   cfg.Region,
		endpoint: endpoint,
		client:   client,
	}
}

func (s *SES) GetName() string { return "ses" }

// Send delivers a message via the AWS SES v2 SendEmail API.
func (s *SES) Send(ctx context.Context, msg *Message) (*DeliveryResult, error) {
	payload := s.buildPayload(msg)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ses: marshal request: %w", err)
	}

	resp, err := s.client.Do(&HTTPRequest{
		Method: "POST",
		URL:    s.endpoint + sesSendPath,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("ses: send request: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var sesResp sesResponse
		messageID := ""
		if err := json.Unmarshal(resp.Body, &sesResp); err == nil {
			messageID = sesResp.MessageID
		}
		return &DeliveryResult{
			ProviderMessageID: messageID,
			Status:            StatusSent,
			Timestamp:         time.Now(),
			Metadata: map[string]string{
				"region":      s.region,
				"status_code": fmt.Sprintf("%d", resp.StatusCode),
			},
		}, nil
	}

	return nil, ClassifyHTTPError("ses", resp.StatusCode, string(resp.Body))
}

// HealthCheck verifies AWS SES connectivity by calling GetAccount.
func (s *SES) HealthCheck(ctx context.Context) error {
	resp, err := s.client.Do(&HTTPRequest{
		Method: "GET",
		URL:    s.endpoint + "/v2/email/account",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})
	if err != nil {
		return fmt.Errorf("ses: health check request: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("ses: health check returned status %d", resp.StatusCode)
	}
	return nil
}

type sesPayload struct {
	FromEmailAddress string      `json:"FromEmailAddress"`
	Destination      sesDestination `json:"Destination"`
	Content          sesContent  `json:"Content"`
}

type sesDestination struct {
	ToAddresses []string `json:"ToAddresses"`
}

type sesContent struct {
	Simple sesSimpleContent `json:"Simple"`
}

type sesSimpleContent struct {
	Subject sesBodyPart `json:"Subject"`
	Body    sesBody     `json:"Body"`
}

type sesBody struct {
	Text sesBodyPart `json:"Text"`
}

type sesBodyPart struct {
	Data    string `json:"Data"`
	Charset string `json:"Charset"`
}

type sesResponse struct {
	MessageID string `json:"MessageId"`
}

func (s *SES) buildPayload(msg *Message) sesPayload {
	return sesPayload{
		FromEmailAddress: msg.From,
		Destination: sesDestination{
			ToAddresses: msg.To,
		},
		Content: sesContent{
			Simple: sesSimpleContent{
				Subject: sesBodyPart{
					Data:    msg.Subject,
					Charset: "UTF-8",
				},
				Body: sesBody{
					Text: sesBodyPart{
						Data:    strings.TrimSpace(string(msg.Body)),
						Charset: "UTF-8",
					},
				},
			},
		},
	}
}
