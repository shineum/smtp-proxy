package smtp

import (
	"net/mail"
	"strings"
)

// ValidateEmailAddress validates an email address per RFC 5321.
// It uses net/mail.ParseAddress which supports RFC 5322 address format.
func ValidateEmailAddress(email string) error {
	_, err := mail.ParseAddress(email)
	return err
}

// ExtractDomain extracts the domain part from an email address.
// Returns an empty string if the address does not contain an @ symbol.
func ExtractDomain(email string) string {
	return domainFromEmail(email)
}

// IsValidDomain performs basic domain format validation. It checks that the
// domain is non-empty, does not start or end with a dot, and contains at
// least one dot separator.
func IsValidDomain(domain string) bool {
	if domain == "" {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}
