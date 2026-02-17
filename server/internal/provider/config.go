package provider

import (
	"errors"
	"time"
)

// ProviderConfig holds configuration for an ESP provider.
type ProviderConfig struct {
	// Type identifies the provider: "sendgrid", "ses", "mailgun", "msgraph", "stdout", "file".
	Type string

	// APIKey is the authentication credential for the provider.
	APIKey string

	// Endpoint overrides the default API URL (useful for testing).
	Endpoint string

	// Timeout is the maximum duration for API calls.
	Timeout time.Duration

	// Region is used for AWS SES to determine the API endpoint.
	Region string

	// Domain is the Mailgun sending domain.
	Domain string

	// MSGraph-specific fields.
	TenantID     string // Azure AD tenant ID
	ClientID     string // Azure AD application client ID
	ClientSecret string // Azure AD application client secret
	UserID       string // Microsoft 365 user ID or UPN for sendMail
}

const defaultTimeout = 30 * time.Second

// Validate checks that required fields are set based on provider type.
func (c *ProviderConfig) Validate() error {
	if c.Type == "" {
		return errors.New("provider type is required")
	}

	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}

	switch c.Type {
	case "sendgrid":
		if c.APIKey == "" {
			return errors.New("sendgrid: api_key is required")
		}
	case "ses":
		if c.Region == "" {
			return errors.New("ses: region is required")
		}
		if c.APIKey == "" {
			return errors.New("ses: api_key (access key ID) is required")
		}
	case "mailgun":
		if c.APIKey == "" {
			return errors.New("mailgun: api_key is required")
		}
		if c.Domain == "" {
			return errors.New("mailgun: domain is required")
		}
	case "msgraph":
		if c.TenantID == "" {
			return errors.New("msgraph: tenant_id is required")
		}
		if c.ClientID == "" {
			return errors.New("msgraph: client_id is required")
		}
		if c.ClientSecret == "" {
			return errors.New("msgraph: client_secret is required")
		}
		if c.UserID == "" {
			return errors.New("msgraph: user_id is required")
		}
	case "stdout":
		// No configuration required.
	case "file":
		// Endpoint is used as output directory; optional (defaults to ./mail_output).
	default:
		return errors.New("unknown provider type: " + c.Type)
	}

	return nil
}
