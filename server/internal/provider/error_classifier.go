package provider

import (
	"errors"
	"strings"
)

// ProviderError wraps an ESP API error with classification metadata.
type ProviderError struct {
	// Provider is the name of the ESP that returned the error.
	Provider string
	// StatusCode is the HTTP status code from the ESP API.
	StatusCode int
	// Message is the error description from the ESP API.
	Message string
	// Permanent indicates the error will not succeed on retry.
	Permanent bool
}

func (e *ProviderError) Error() string {
	return e.Provider + ": " + e.Message
}

// IsPermanent returns true if the error is a permanent failure that should
// not be retried and should be routed to the DLQ.
func IsPermanent(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Permanent
	}
	return false
}

// IsTransient returns true if the error is a temporary failure that may
// succeed on retry.
func IsTransient(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return !pe.Permanent
	}
	// Unknown errors are treated as transient to avoid data loss.
	return true
}

// ClassifyHTTPError creates a ProviderError from an HTTP status code and
// response body, classifying it as permanent or transient.
func ClassifyHTTPError(providerName string, statusCode int, body string) *ProviderError {
	pe := &ProviderError{
		Provider:   providerName,
		StatusCode: statusCode,
		Message:    body,
	}

	switch {
	case statusCode >= 200 && statusCode < 300:
		// Not an error.
		return nil

	case statusCode == 400:
		pe.Permanent = containsPermanentIndicator(body)

	case statusCode == 401:
		// 401 is permanent for most providers. MS Graph handles token
		// refresh at the provider level before returning errors here.
		pe.Permanent = true

	case statusCode == 403:
		pe.Permanent = true

	case statusCode == 404:
		pe.Permanent = true

	case statusCode == 429:
		// Rate limited - always transient.
		pe.Permanent = false

	case statusCode >= 500:
		pe.Permanent = containsPermanentServerIndicator(body)

	default:
		// Other 4xx codes are treated as permanent.
		pe.Permanent = statusCode >= 400 && statusCode < 500
	}

	return pe
}

// containsPermanentIndicator checks if a 400 response body indicates a
// permanent failure (e.g., invalid recipient, bad request that won't change).
func containsPermanentIndicator(body string) bool {
	lower := strings.ToLower(body)
	permanentPatterns := []string{
		"invalid recipient",
		"invalid email",
		"does not exist",
		"mailbox not found",
		"recipient rejected",
		"bad request",
		"validation error",
		"invalid address",
	}
	for _, pattern := range permanentPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// containsPermanentServerIndicator checks if a 5xx response body indicates
// a permanent server-side failure (e.g., invalid auth configuration).
func containsPermanentServerIndicator(body string) bool {
	lower := strings.ToLower(body)
	permanentPatterns := []string{
		"invalid api key",
		"authentication failed",
		"account suspended",
		"account disabled",
		"unauthorized",
	}
	for _, pattern := range permanentPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
