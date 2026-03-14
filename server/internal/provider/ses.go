package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
	"time"
)

const (
	sesDefaultEndpointFmt = "https://email.%s.amazonaws.com"
	sesSendPath           = "/v2/email/outbound-emails"
)

// SES implements the Provider interface for AWS SES v2 API.
// Requests are signed with AWS Signature V4 via SigV4HTTPClient.
type SES struct {
	region   string
	endpoint string
	client   HTTPClient
}

// NewSES creates an AWS SES provider from the given configuration.
// When both APIKey (Access Key ID) and SecretKey (Secret Access Key) are
// provided, requests are automatically signed with AWS Signature V4.
// When credentials are absent (e.g. in tests), the client is used as-is.
func NewSES(cfg ProviderConfig, client HTTPClient) *SES {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf(sesDefaultEndpointFmt, cfg.Region)
	}

	// Wrap with Sig V4 signing when credentials are available.
	httpClient := client
	if cfg.APIKey != "" && cfg.SecretKey != "" {
		httpClient = NewSigV4HTTPClient(client, cfg.APIKey, cfg.SecretKey, cfg.Region)
	}

	return &SES{
		region:   cfg.Region,
		endpoint: endpoint,
		client:   httpClient,
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
	FromEmailAddress string         `json:"FromEmailAddress"`
	Destination      sesDestination `json:"Destination"`
	Content          sesContent     `json:"Content"`
}

type sesDestination struct {
	ToAddresses  []string `json:"ToAddresses"`
	CcAddresses  []string `json:"CcAddresses,omitempty"`
	BccAddresses []string `json:"BccAddresses,omitempty"`
}

type sesContent struct {
	Simple *sesSimpleContent `json:"Simple,omitempty"`
	Raw    *sesRawContent    `json:"Raw,omitempty"`
}

type sesSimpleContent struct {
	Subject sesBodyPart `json:"Subject"`
	Body    sesBody     `json:"Body"`
}

type sesBody struct {
	Text *sesBodyPart `json:"Text,omitempty"`
	Html *sesBodyPart `json:"Html,omitempty"`
}

type sesBodyPart struct {
	Data    string `json:"Data"`
	Charset string `json:"Charset"`
}

type sesRawContent struct {
	Data string `json:"Data"` // base64 encoded raw MIME message
}

type sesResponse struct {
	MessageID string `json:"MessageId"`
}

func (s *SES) buildPayload(msg *Message) sesPayload {
	dest := sesDestination{
		ToAddresses: msg.To,
	}
	if len(msg.CC) > 0 {
		dest.CcAddresses = msg.CC
	}
	if len(msg.BCC) > 0 {
		dest.BccAddresses = msg.BCC
	}
	payload := sesPayload{
		FromEmailAddress: msg.From,
		Destination:      dest,
	}

	// Use Raw mode when attachments are present.
	if len(msg.Attachments) > 0 {
		rawData, err := buildRawMIME(msg)
		if err == nil {
			payload.Content = sesContent{
				Raw: &sesRawContent{
					Data: base64.StdEncoding.EncodeToString(rawData),
				},
			}
			return payload
		}
		// Fall through to Simple mode if raw build fails.
	}

	// Simple mode: text and optional HTML.
	body := sesBody{}

	textData := msg.TextBody
	if textData == "" && msg.HTMLBody == "" {
		textData = strings.TrimSpace(string(msg.Body))
	}
	if textData != "" {
		body.Text = &sesBodyPart{Data: textData, Charset: "UTF-8"}
	}
	if msg.HTMLBody != "" {
		body.Html = &sesBodyPart{Data: msg.HTMLBody, Charset: "UTF-8"}
	}

	payload.Content = sesContent{
		Simple: &sesSimpleContent{
			Subject: sesBodyPart{
				Data:    msg.Subject,
				Charset: "UTF-8",
			},
			Body: body,
		},
	}
	return payload
}

// buildRawMIME constructs a raw RFC 5322 multipart/mixed MIME message.
func buildRawMIME(msg *Message) ([]byte, error) {
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)

	// Write top-level headers.
	fmt.Fprintf(&buf, "From: %s\r\n", msg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(msg.To, ", "))
	if len(msg.CC) > 0 {
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(msg.CC, ", "))
	}
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n", writer.Boundary())
	fmt.Fprintf(&buf, "\r\n")

	// Text body part.
	textBody := msg.TextBody
	if textBody == "" {
		textBody = string(msg.Body)
	}
	if textBody != "" {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", "text/plain; charset=UTF-8")
		header.Set("Content-Transfer-Encoding", "quoted-printable")
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		part.Write([]byte(textBody))
	}

	// HTML body part.
	if msg.HTMLBody != "" {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", "text/html; charset=UTF-8")
		header.Set("Content-Transfer-Encoding", "quoted-printable")
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		part.Write([]byte(msg.HTMLBody))
	}

	// Attachment parts.
	for _, att := range msg.Attachments {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", att.ContentType)
		header.Set("Content-Transfer-Encoding", "base64")
		if att.IsInline {
			header.Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", att.Filename))
			if att.ContentID != "" {
				header.Set("Content-Id", "<"+att.ContentID+">")
			}
		} else {
			header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", att.Filename))
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(att.Content)
		part.Write([]byte(encoded))
	}

	writer.Close()
	return buf.Bytes(), nil
}
