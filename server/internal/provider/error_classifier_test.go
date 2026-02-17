package provider

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantNil    bool
		wantPerm   bool
	}{
		{
			name:       "200 returns nil",
			statusCode: 200,
			body:       "",
			wantNil:    true,
		},
		{
			name:       "202 returns nil",
			statusCode: 202,
			body:       "",
			wantNil:    true,
		},
		{
			name:       "299 returns nil",
			statusCode: 299,
			body:       "",
			wantNil:    true,
		},
		{
			name:       "400 with invalid email body is permanent",
			statusCode: 400,
			body:       "invalid email address provided",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "400 with temporary body is not permanent",
			statusCode: 400,
			body:       "temporary server issue",
			wantNil:    false,
			wantPerm:   false,
		},
		{
			name:       "400 with bad request body is permanent",
			statusCode: 400,
			body:       "Bad Request: missing field",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "401 is permanent",
			statusCode: 401,
			body:       "unauthorized",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "403 is permanent",
			statusCode: 403,
			body:       "forbidden",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "404 is permanent",
			statusCode: 404,
			body:       "not found",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "429 is transient (rate limited)",
			statusCode: 429,
			body:       "too many requests",
			wantNil:    false,
			wantPerm:   false,
		},
		{
			name:       "500 with permanent indicator is permanent",
			statusCode: 500,
			body:       "invalid api key in configuration",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "500 with generic body is transient",
			statusCode: 500,
			body:       "internal server error",
			wantNil:    false,
			wantPerm:   false,
		},
		{
			name:       "500 with authentication failed is permanent",
			statusCode: 500,
			body:       "authentication failed for account",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "500 with account suspended is permanent",
			statusCode: 500,
			body:       "account suspended",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "418 (other 4xx) is permanent",
			statusCode: 418,
			body:       "i'm a teapot",
			wantNil:    false,
			wantPerm:   true,
		},
		{
			name:       "405 (other 4xx) is permanent",
			statusCode: 405,
			body:       "method not allowed",
			wantNil:    false,
			wantPerm:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyHTTPError("test-provider", tt.statusCode, tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil ProviderError, got nil")
			}

			if result.Provider != "test-provider" {
				t.Errorf("expected provider %q, got %q", "test-provider", result.Provider)
			}
			if result.StatusCode != tt.statusCode {
				t.Errorf("expected status code %d, got %d", tt.statusCode, result.StatusCode)
			}
			if result.Permanent != tt.wantPerm {
				t.Errorf("expected Permanent=%v, got %v", tt.wantPerm, result.Permanent)
			}
		})
	}
}

func TestIsPermanent(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "permanent ProviderError returns true",
			err: &ProviderError{
				Provider:  "test",
				Permanent: true,
				Message:   "bad request",
			},
			want: true,
		},
		{
			name: "transient ProviderError returns false",
			err: &ProviderError{
				Provider:  "test",
				Permanent: false,
				Message:   "rate limited",
			},
			want: false,
		},
		{
			name: "non-ProviderError returns false",
			err:  errors.New("generic error"),
			want: false,
		},
		{
			name: "wrapped permanent ProviderError returns true",
			err: fmt.Errorf("wrapped: %w", &ProviderError{
				Provider:  "test",
				Permanent: true,
				Message:   "invalid",
			}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPermanent(tt.err)
			if got != tt.want {
				t.Errorf("IsPermanent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "transient ProviderError returns true",
			err: &ProviderError{
				Provider:  "test",
				Permanent: false,
				Message:   "rate limited",
			},
			want: true,
		},
		{
			name: "permanent ProviderError returns false",
			err: &ProviderError{
				Provider:  "test",
				Permanent: true,
				Message:   "bad request",
			},
			want: false,
		},
		{
			name: "non-ProviderError returns true (unknown treated as transient)",
			err:  errors.New("generic error"),
			want: true,
		},
		{
			name: "wrapped transient ProviderError returns true",
			err: fmt.Errorf("wrapped: %w", &ProviderError{
				Provider:  "test",
				Permanent: false,
				Message:   "timeout",
			}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTransient(tt.err)
			if got != tt.want {
				t.Errorf("IsTransient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProviderError_Error(t *testing.T) {
	pe := &ProviderError{
		Provider: "sendgrid",
		Message:  "bad request",
	}
	want := "sendgrid: bad request"
	if got := pe.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
