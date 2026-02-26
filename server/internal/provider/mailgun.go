package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
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
// When attachments are present, it uses multipart/form-data encoding;
// otherwise it uses the simpler application/x-www-form-urlencoded format.
func (m *Mailgun) Send(ctx context.Context, msg *Message) (*DeliveryResult, error) {
	var reqBody []byte
	var contentType string

	if len(msg.Attachments) > 0 {
		body, ct, err := m.buildMultipartForm(msg)
		if err != nil {
			return nil, fmt.Errorf("mailgun: build multipart form: %w", err)
		}
		reqBody = body
		contentType = ct
	} else {
		form := m.buildForm(msg)
		reqBody = []byte(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}

	resp, err := m.client.Do(&HTTPRequest{
		Method: "POST",
		URL:    fmt.Sprintf("%s/v3/%s/messages", m.endpoint, m.domain),
		Headers: map[string]string{
			"Authorization": "Basic " + basicAuth("api", m.apiKey),
			"Content-Type":  contentType,
		},
		Body: reqBody,
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

	// Prefer parsed text body; fall back to raw Body.
	text := msg.TextBody
	if text == "" {
		text = string(msg.Body)
	}
	form.Set("text", text)

	if msg.HTMLBody != "" {
		form.Set("html", msg.HTMLBody)
	}

	for key, value := range msg.Headers {
		form.Set("h:"+key, value)
	}
	return form
}

// buildMultipartForm creates a multipart/form-data request body that includes
// form fields and file attachments.
func (m *Mailgun) buildMultipartForm(msg *Message) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add form fields.
	writer.WriteField("from", msg.From)
	writer.WriteField("to", strings.Join(msg.To, ","))
	writer.WriteField("subject", msg.Subject)

	text := msg.TextBody
	if text == "" {
		text = string(msg.Body)
	}
	writer.WriteField("text", text)

	if msg.HTMLBody != "" {
		writer.WriteField("html", msg.HTMLBody)
	}

	for key, value := range msg.Headers {
		writer.WriteField("h:"+key, value)
	}

	// Add attachments.
	for _, att := range msg.Attachments {
		fieldName := "attachment"
		if att.IsInline {
			fieldName = "inline"
		}

		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition",
			fmt.Sprintf("form-data; name=%q; filename=%q", fieldName, att.Filename))
		header.Set("Content-Type", att.ContentType)

		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(att.Content); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), writer.FormDataContentType(), nil
}

// basicAuth encodes credentials as base64 for HTTP Basic Authentication.
func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}
