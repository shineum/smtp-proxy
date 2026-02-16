package smtp

import (
	"testing"
)

func TestValidateEmailAddress_Valid(t *testing.T) {
	validAddresses := []string{
		"user@example.com",
		"user+tag@example.com",
		"first.last@example.com",
		"user@sub.domain.example.com",
		"\"quoted user\"@example.com",
	}

	for _, addr := range validAddresses {
		t.Run(addr, func(t *testing.T) {
			if err := ValidateEmailAddress(addr); err != nil {
				t.Errorf("expected %q to be valid, got error: %v", addr, err)
			}
		})
	}
}

func TestValidateEmailAddress_Invalid(t *testing.T) {
	invalidAddresses := []string{
		"",
		"plaintext",
		"@no-local.com",
		"missing-at-sign",
		"@",
	}

	for _, addr := range invalidAddresses {
		t.Run(addr, func(t *testing.T) {
			if err := ValidateEmailAddress(addr); err == nil {
				t.Errorf("expected %q to be invalid, got no error", addr)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string
	}{
		{
			name:     "standard email",
			email:    "user@example.com",
			expected: "example.com",
		},
		{
			name:     "subdomain",
			email:    "user@mail.example.com",
			expected: "mail.example.com",
		},
		{
			name:     "no at sign",
			email:    "noemail",
			expected: "",
		},
		{
			name:     "empty string",
			email:    "",
			expected: "",
		},
		{
			name:     "only at sign",
			email:    "@",
			expected: "",
		},
		{
			name:     "at sign at end",
			email:    "user@",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDomain(tt.email)
			if got != tt.expected {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.email, got, tt.expected)
			}
		})
	}
}

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected bool
	}{
		{
			name:     "valid domain",
			domain:   "example.com",
			expected: true,
		},
		{
			name:     "valid subdomain",
			domain:   "sub.example.com",
			expected: true,
		},
		{
			name:     "empty string",
			domain:   "",
			expected: false,
		},
		{
			name:     "starts with dot",
			domain:   ".example.com",
			expected: false,
		},
		{
			name:     "ends with dot",
			domain:   "example.com.",
			expected: false,
		},
		{
			name:     "no dot",
			domain:   "localhost",
			expected: false,
		},
		{
			name:     "single character with dot",
			domain:   "a.b",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidDomain(tt.domain)
			if got != tt.expected {
				t.Errorf("IsValidDomain(%q) = %v, want %v", tt.domain, got, tt.expected)
			}
		})
	}
}
