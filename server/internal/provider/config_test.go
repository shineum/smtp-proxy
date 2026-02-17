package provider

import (
	"testing"
	"time"
)

func TestProviderConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ProviderConfig
		wantErr string
	}{
		{
			name:    "empty type returns error",
			config:  ProviderConfig{},
			wantErr: "provider type is required",
		},
		{
			name: "sendgrid without auth token returns error",
			config: ProviderConfig{
				Type: "sendgrid",
			},
			wantErr: "sendgrid: api_key is required",
		},
		{
			name: "sendgrid with auth token succeeds",
			config: ProviderConfig{
				Type:   "sendgrid",
				APIKey: "test-token-sg",
			},
			wantErr: "",
		},
		{
			name: "ses without region returns error",
			config: ProviderConfig{
				Type:   "ses",
				APIKey: "test-token-ses",
			},
			wantErr: "ses: region is required",
		},
		{
			name: "ses without auth token returns error",
			config: ProviderConfig{
				Type:   "ses",
				Region: "us-east-1",
			},
			wantErr: "ses: api_key (access key ID) is required",
		},
		{
			name: "ses with region and auth token succeeds",
			config: ProviderConfig{
				Type:   "ses",
				Region: "us-east-1",
				APIKey: "test-token-ses",
			},
			wantErr: "",
		},
		{
			name: "mailgun without auth token returns error",
			config: ProviderConfig{
				Type:   "mailgun",
				Domain: "mg.example.com",
			},
			wantErr: "mailgun: api_key is required",
		},
		{
			name: "mailgun without domain returns error",
			config: ProviderConfig{
				Type:   "mailgun",
				APIKey: "test-token-mg",
			},
			wantErr: "mailgun: domain is required",
		},
		{
			name: "mailgun with auth token and domain succeeds",
			config: ProviderConfig{
				Type:   "mailgun",
				APIKey: "test-token-mg",
				Domain: "mg.example.com",
			},
			wantErr: "",
		},
		{
			name: "msgraph without tenant returns error",
			config: ProviderConfig{
				Type:         "msgraph",
				ClientID:     "test-client",
				ClientSecret: "test-value",
				UserID:       "user@example.com",
			},
			wantErr: "msgraph: tenant_id is required",
		},
		{
			name: "msgraph without client id returns error",
			config: ProviderConfig{
				Type:         "msgraph",
				TenantID:     "test-tenant",
				ClientSecret: "test-value",
				UserID:       "user@example.com",
			},
			wantErr: "msgraph: client_id is required",
		},
		{
			name: "msgraph without client credential returns error",
			config: ProviderConfig{
				Type:     "msgraph",
				TenantID: "test-tenant",
				ClientID: "test-client",
				UserID:   "user@example.com",
			},
			wantErr: "msgraph: client_secret is required",
		},
		{
			name: "msgraph without user returns error",
			config: ProviderConfig{
				Type:         "msgraph",
				TenantID:     "test-tenant",
				ClientID:     "test-client",
				ClientSecret: "test-value",
			},
			wantErr: "msgraph: user_id is required",
		},
		{
			name: "msgraph with all fields succeeds",
			config: ProviderConfig{
				Type:         "msgraph",
				TenantID:     "test-tenant",
				ClientID:     "test-client",
				ClientSecret: "test-value",
				UserID:       "user@example.com",
			},
			wantErr: "",
		},
		{
			name: "unknown type returns error",
			config: ProviderConfig{
				Type: "postmark",
			},
			wantErr: "unknown provider type: postmark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %q, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestProviderConfig_Validate_TimeoutDefaultsTo30s(t *testing.T) {
	cfg := ProviderConfig{
		Type:   "sendgrid",
		APIKey: "test-token-sg",
	}

	if cfg.Timeout != 0 {
		t.Fatalf("precondition: expected zero timeout before validation, got %v", cfg.Timeout)
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := 30 * time.Second
	if cfg.Timeout != want {
		t.Errorf("expected timeout to default to %v, got %v", want, cfg.Timeout)
	}
}

func TestProviderConfig_Validate_TimeoutPreservedWhenSet(t *testing.T) {
	cfg := ProviderConfig{
		Type:    "sendgrid",
		APIKey:  "test-token-sg",
		Timeout: 60 * time.Second,
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := 60 * time.Second
	if cfg.Timeout != want {
		t.Errorf("expected timeout to remain %v, got %v", want, cfg.Timeout)
	}
}
