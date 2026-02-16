package smtp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/mail"
	"strings"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/internal/auth"
	"github.com/sungwon/smtp-proxy/internal/storage"
)

// Session handles a single SMTP connection and implements the go-smtp Session
// interface. It enforces authentication, domain validation, and message
// enqueue operations.
type Session struct {
	ctx            context.Context
	queries        storage.Querier
	log            zerolog.Logger
	backend        *Backend
	accountID      uuid.UUID
	authenticated  bool
	allowedDomains []string
	sender         string
	recipients     []string
}

// AuthPlain handles SMTP AUTH PLAIN. It validates the username/password
// against stored account credentials and populates the session with account
// details on success.
func (s *Session) AuthPlain(username, password string) error {
	s.log.Info().Str("username", username).Msg("auth attempt")

	account, err := s.queries.GetAccountByName(s.ctx, username)
	if err != nil {
		s.log.Warn().Str("username", username).Msg("auth failed: account not found")
		return &gosmtp.SMTPError{
			Code:         535,
			EnhancedCode: gosmtp.EnhancedCode{5, 7, 8},
			Message:      "Authentication failed",
		}
	}

	if err := auth.VerifyPassword(account.PasswordHash, password); err != nil {
		s.log.Warn().Str("username", username).Msg("auth failed: invalid password")
		return &gosmtp.SMTPError{
			Code:         535,
			EnhancedCode: gosmtp.EnhancedCode{5, 7, 8},
			Message:      "Authentication failed",
		}
	}

	s.accountID = account.ID
	s.authenticated = true

	// Parse allowed domains from JSONB column.
	var domains []string
	if len(account.AllowedDomains) > 0 {
		if err := json.Unmarshal(account.AllowedDomains, &domains); err != nil {
			s.log.Error().Err(err).Msg("failed to parse allowed domains")
			domains = nil
		}
	}
	s.allowedDomains = domains

	s.log.Info().
		Str("username", username).
		Str("account_id", account.ID.String()).
		Msg("auth successful")

	return nil
}

// Mail handles the MAIL FROM command. It validates that the session is
// authenticated and that the sender domain is in the account's allowed
// domains list.
func (s *Session) Mail(from string, opts *gosmtp.MailOptions) error {
	if !s.authenticated {
		return &gosmtp.SMTPError{
			Code:         530,
			EnhancedCode: gosmtp.EnhancedCode{5, 7, 0},
			Message:      "Authentication required",
		}
	}

	// Validate sender address format.
	addr, err := mail.ParseAddress(from)
	if err != nil {
		s.log.Warn().Str("from", from).Msg("invalid sender address format")
		return &gosmtp.SMTPError{
			Code:         550,
			EnhancedCode: gosmtp.EnhancedCode{5, 1, 7},
			Message:      "Invalid sender address",
		}
	}

	senderDomain := domainFromEmail(addr.Address)
	if !s.isDomainAllowed(senderDomain) {
		s.log.Warn().
			Str("from", from).
			Str("domain", senderDomain).
			Strs("allowed", s.allowedDomains).
			Msg("sender domain not allowed")
		return &gosmtp.SMTPError{
			Code:         550,
			EnhancedCode: gosmtp.EnhancedCode{5, 7, 1},
			Message:      "Sender domain not allowed",
		}
	}

	s.sender = addr.Address
	s.log.Info().Str("from", s.sender).Msg("MAIL FROM accepted")
	return nil
}

// Rcpt handles the RCPT TO command. It validates the recipient address format
// and appends it to the session's recipient list.
func (s *Session) Rcpt(to string, opts *gosmtp.RcptOptions) error {
	if !s.authenticated {
		return &gosmtp.SMTPError{
			Code:         530,
			EnhancedCode: gosmtp.EnhancedCode{5, 7, 0},
			Message:      "Authentication required",
		}
	}

	addr, err := mail.ParseAddress(to)
	if err != nil {
		// Try parsing as a bare address without angle brackets.
		if _, err2 := mail.ParseAddress("<" + to + ">"); err2 != nil {
			s.log.Warn().Str("to", to).Msg("invalid recipient address format")
			return &gosmtp.SMTPError{
				Code:         550,
				EnhancedCode: gosmtp.EnhancedCode{5, 1, 1},
				Message:      "Invalid recipient address",
			}
		}
		addr = &mail.Address{Address: to}
	}

	s.recipients = append(s.recipients, addr.Address)
	s.log.Info().Str("to", addr.Address).Msg("RCPT TO accepted")
	return nil
}

// Data handles the DATA command. It reads the message content, extracts
// headers and subject, and enqueues the message for processing.
// Per R-CORE-018, message body content is not logged.
func (s *Session) Data(r io.Reader) error {
	if !s.authenticated {
		return &gosmtp.SMTPError{
			Code:         530,
			EnhancedCode: gosmtp.EnhancedCode{5, 7, 0},
			Message:      "Authentication required",
		}
	}

	if len(s.recipients) == 0 {
		return &gosmtp.SMTPError{
			Code:         503,
			EnhancedCode: gosmtp.EnhancedCode{5, 5, 1},
			Message:      "No recipients specified",
		}
	}

	// Read the full message (headers + body).
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		s.log.Error().Err(err).Msg("failed to read message data")
		return &gosmtp.SMTPError{
			Code:         451,
			EnhancedCode: gosmtp.EnhancedCode{4, 3, 0},
			Message:      "Error reading message",
		}
	}

	body := buf.String()

	// Extract subject and headers from the message.
	subject := ""
	var headers map[string][]string
	msg, err := mail.ReadMessage(strings.NewReader(body))
	if err == nil {
		subject = msg.Header.Get("Subject")
		headers = map[string][]string(msg.Header)
	}

	// Marshal recipients and headers to JSON for storage.
	recipientsJSON, _ := json.Marshal(s.recipients)
	headersJSON, _ := json.Marshal(headers)

	// Enqueue message for later delivery.
	_, err = s.queries.EnqueueMessage(s.ctx, storage.EnqueueMessageParams{
		AccountID:  s.accountID,
		Sender:     s.sender,
		Recipients: recipientsJSON,
		Subject:    sql.NullString{String: subject, Valid: subject != ""},
		Headers:    headersJSON,
		Body:       body,
	})
	if err != nil {
		s.log.Error().Err(err).Msg("failed to enqueue message")
		return &gosmtp.SMTPError{
			Code:         451,
			EnhancedCode: gosmtp.EnhancedCode{4, 3, 0},
			Message:      "Error queuing message",
		}
	}

	s.log.Info().
		Str("from", s.sender).
		Int("recipient_count", len(s.recipients)).
		Msg("message enqueued")

	return nil
}

// Reset is called between messages in the same session. It clears the sender
// and recipients but preserves the authentication state.
func (s *Session) Reset() {
	s.sender = ""
	s.recipients = nil
}

// Logout is called when the client disconnects. It decrements the backend's
// active session counter and logs the session closure.
func (s *Session) Logout() error {
	s.backend.active.Add(-1)
	s.log.Info().Msg("session closed")
	return nil
}

// isDomainAllowed checks whether the given domain is in the account's allowed
// domains list. If no domains are configured, all domains are allowed.
func (s *Session) isDomainAllowed(domain string) bool {
	if len(s.allowedDomains) == 0 {
		return true
	}
	for _, d := range s.allowedDomains {
		if strings.EqualFold(d, domain) {
			return true
		}
	}
	return false
}

// domainFromEmail extracts the domain part from an email address.
func domainFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
