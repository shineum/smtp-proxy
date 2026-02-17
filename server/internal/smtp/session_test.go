package smtp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// errNotFound is a sentinel error for simulating "not found" database results.
var errNotFound = errors.New("no rows")

// mockQuerier implements storage.Querier with controllable responses.
type mockQuerier struct {
	// GetAccountByName behavior
	getAccountByNameFn func(ctx context.Context, name string) (storage.Account, error)

	// EnqueueMessage behavior
	enqueueMessageFn func(ctx context.Context, arg storage.EnqueueMessageParams) (storage.Message, error)
}

func (m *mockQuerier) CreateAccount(_ context.Context, _ storage.CreateAccountParams) (storage.Account, error) {
	return storage.Account{}, nil
}

func (m *mockQuerier) CreateDeliveryLog(_ context.Context, _ storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) CreateProvider(_ context.Context, _ storage.CreateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) CreateRoutingRule(_ context.Context, _ storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) DeleteAccount(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteProvider(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteRoutingRule(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) EnqueueMessage(ctx context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
	if m.enqueueMessageFn != nil {
		return m.enqueueMessageFn(ctx, arg)
	}
	return storage.Message{
		ID:        uuid.New(),
		AccountID: arg.AccountID,
		Sender:    arg.Sender,
		Status:    storage.MessageStatusQueued,
	}, nil
}

func (m *mockQuerier) GetAccountByAPIKey(_ context.Context, _ string) (storage.Account, error) {
	return storage.Account{}, nil
}

func (m *mockQuerier) GetAccountByID(_ context.Context, _ uuid.UUID) (storage.Account, error) {
	return storage.Account{}, nil
}

func (m *mockQuerier) GetAccountByName(ctx context.Context, name string) (storage.Account, error) {
	if m.getAccountByNameFn != nil {
		return m.getAccountByNameFn(ctx, name)
	}
	return storage.Account{}, errNotFound
}

func (m *mockQuerier) GetMessageByID(_ context.Context, _ uuid.UUID) (storage.Message, error) {
	return storage.Message{}, nil
}

func (m *mockQuerier) GetProviderByID(_ context.Context, _ uuid.UUID) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) GetQueuedMessages(_ context.Context, _ int32) ([]storage.Message, error) {
	return nil, nil
}

func (m *mockQuerier) GetRoutingRuleByID(_ context.Context, _ uuid.UUID) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) ListAccounts(_ context.Context) ([]storage.Account, error) {
	return nil, nil
}

func (m *mockQuerier) ListDeliveryLogsByMessageID(_ context.Context, _ uuid.UUID) ([]storage.DeliveryLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListMessagesByAccountID(_ context.Context, _ storage.ListMessagesByAccountIDParams) ([]storage.Message, error) {
	return nil, nil
}

func (m *mockQuerier) ListProvidersByAccountID(_ context.Context, _ uuid.UUID) ([]storage.EspProvider, error) {
	return nil, nil
}

func (m *mockQuerier) ListRoutingRulesByAccountID(_ context.Context, _ uuid.UUID) ([]storage.RoutingRule, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateAccount(_ context.Context, _ storage.UpdateAccountParams) (storage.Account, error) {
	return storage.Account{}, nil
}

func (m *mockQuerier) UpdateMessageStatus(_ context.Context, _ storage.UpdateMessageStatusParams) error {
	return nil
}

func (m *mockQuerier) UpdateProvider(_ context.Context, _ storage.UpdateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) UpdateRoutingRule(_ context.Context, _ storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

// newTestSession creates a Session with a mock backend for testing.
func newTestSession(mock *mockQuerier) *Session {
	log := zerolog.Nop()
	b := NewBackend(mock, log, 100)
	b.active.Add(1) // Simulate that the session was counted on creation.
	return &Session{
		ctx:     context.Background(),
		queries: mock,
		log:     log,
		backend: b,
	}
}

// newAuthenticatedSession creates a session that has already been authenticated.
func newAuthenticatedSession(mock *mockQuerier, accountID uuid.UUID, allowedDomains []string) *Session {
	s := newTestSession(mock)
	s.accountID = accountID
	s.authenticated = true
	s.allowedDomains = allowedDomains
	return s
}

// hashTestPassword creates a bcrypt hash for testing.
func hashTestPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return hash
}

// --- AuthPlain Tests ---

func TestSession_AuthPlain_Success(t *testing.T) {
	accountID := uuid.New()
	passwordHash := hashTestPassword(t, "correct-password")
	domainsJSON, _ := json.Marshal([]string{"example.com"})

	mock := &mockQuerier{
		getAccountByNameFn: func(_ context.Context, name string) (storage.Account, error) {
			if name == "testuser" {
				return storage.Account{
					ID:             accountID,
					Name:           "testuser",
					PasswordHash:   passwordHash,
					AllowedDomains: domainsJSON,
				}, nil
			}
			return storage.Account{}, errNotFound
		},
	}

	s := newTestSession(mock)
	err := s.AuthPlain("testuser", "correct-password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !s.authenticated {
		t.Error("expected session to be authenticated")
	}
	if s.accountID != accountID {
		t.Errorf("expected accountID=%s, got %s", accountID, s.accountID)
	}
	if len(s.allowedDomains) != 1 || s.allowedDomains[0] != "example.com" {
		t.Errorf("expected allowedDomains=[example.com], got %v", s.allowedDomains)
	}
}

func TestSession_AuthPlain_InvalidPassword(t *testing.T) {
	passwordHash := hashTestPassword(t, "correct-password")

	mock := &mockQuerier{
		getAccountByNameFn: func(_ context.Context, _ string) (storage.Account, error) {
			return storage.Account{
				ID:           uuid.New(),
				Name:         "testuser",
				PasswordHash: passwordHash,
			}, nil
		},
	}

	s := newTestSession(mock)
	err := s.AuthPlain("testuser", "wrong-password")
	if err == nil {
		t.Fatal("expected error for invalid password")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 535 {
		t.Errorf("expected code 535, got %d", smtpErr.Code)
	}
	if s.authenticated {
		t.Error("session should not be authenticated")
	}
}

func TestSession_AuthPlain_UnknownUser(t *testing.T) {
	mock := &mockQuerier{
		getAccountByNameFn: func(_ context.Context, _ string) (storage.Account, error) {
			return storage.Account{}, errNotFound
		},
	}

	s := newTestSession(mock)
	err := s.AuthPlain("unknown", "any-password")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 535 {
		t.Errorf("expected code 535, got %d", smtpErr.Code)
	}
}

// --- Mail Tests ---

func TestSession_Mail_ValidSender(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, []string{"example.com"})

	err := s.Mail("sender@example.com", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.sender != "sender@example.com" {
		t.Errorf("expected sender=sender@example.com, got %s", s.sender)
	}
}

func TestSession_Mail_ValidSender_NoDomainRestrictions(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, nil)

	err := s.Mail("sender@anydomain.com", nil)
	if err != nil {
		t.Fatalf("expected no error when no domain restrictions, got %v", err)
	}
}

func TestSession_Mail_UnauthorizedDomain(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, []string{"allowed.com"})

	err := s.Mail("sender@forbidden.com", nil)
	if err == nil {
		t.Fatal("expected error for unauthorized domain")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 550 {
		t.Errorf("expected code 550, got %d", smtpErr.Code)
	}
}

func TestSession_Mail_Unauthenticated(t *testing.T) {
	s := newTestSession(&mockQuerier{})

	err := s.Mail("sender@example.com", nil)
	if err == nil {
		t.Fatal("expected error for unauthenticated session")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 530 {
		t.Errorf("expected code 530, got %d", smtpErr.Code)
	}
}

func TestSession_Mail_InvalidAddress(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, nil)

	err := s.Mail("not-an-email", nil)
	if err == nil {
		t.Fatal("expected error for invalid address")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 550 {
		t.Errorf("expected code 550, got %d", smtpErr.Code)
	}
}

// --- Rcpt Tests ---

func TestSession_Rcpt_ValidRecipient(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, nil)

	err := s.Rcpt("recipient@example.com", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(s.recipients) != 1 {
		t.Fatalf("expected 1 recipient, got %d", len(s.recipients))
	}
	if s.recipients[0] != "recipient@example.com" {
		t.Errorf("expected recipient=recipient@example.com, got %s", s.recipients[0])
	}
}

func TestSession_Rcpt_MultipleRecipients(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, nil)

	if err := s.Rcpt("first@example.com", nil); err != nil {
		t.Fatalf("first Rcpt failed: %v", err)
	}
	if err := s.Rcpt("second@example.com", nil); err != nil {
		t.Fatalf("second Rcpt failed: %v", err)
	}

	if len(s.recipients) != 2 {
		t.Fatalf("expected 2 recipients, got %d", len(s.recipients))
	}
}

func TestSession_Rcpt_InvalidFormat(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, nil)

	err := s.Rcpt("not-a-valid-address", nil)
	if err == nil {
		t.Fatal("expected error for invalid address format")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 550 {
		t.Errorf("expected code 550, got %d", smtpErr.Code)
	}
}

func TestSession_Rcpt_Unauthenticated(t *testing.T) {
	s := newTestSession(&mockQuerier{})

	err := s.Rcpt("recipient@example.com", nil)
	if err == nil {
		t.Fatal("expected error for unauthenticated session")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 530 {
		t.Errorf("expected code 530, got %d", smtpErr.Code)
	}
}

// --- Data Tests ---

func TestSession_Data_EnqueuesMessage(t *testing.T) {
	accountID := uuid.New()
	var capturedParams storage.EnqueueMessageParams

	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
			capturedParams = arg
			return storage.Message{
				ID:        uuid.New(),
				AccountID: arg.AccountID,
				Status:    storage.MessageStatusQueued,
			}, nil
		},
	}

	s := newAuthenticatedSession(mock, accountID, nil)
	s.sender = "sender@example.com"
	s.recipients = []string{"recipient@example.com"}

	messageContent := "Subject: Test\r\n\r\nHello, World!"
	err := s.Data(strings.NewReader(messageContent))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if capturedParams.AccountID != accountID {
		t.Errorf("expected accountID=%s, got %s", accountID, capturedParams.AccountID)
	}
	if capturedParams.Sender != "sender@example.com" {
		t.Errorf("expected sender=sender@example.com, got %s", capturedParams.Sender)
	}

	var recipients []string
	if err := json.Unmarshal(capturedParams.Recipients, &recipients); err != nil {
		t.Fatalf("failed to unmarshal recipients: %v", err)
	}
	if len(recipients) != 1 || recipients[0] != "recipient@example.com" {
		t.Errorf("unexpected recipients: %v", recipients)
	}

	if capturedParams.Subject.String != "Test" || !capturedParams.Subject.Valid {
		t.Errorf("expected subject=Test, got %v", capturedParams.Subject)
	}

	if capturedParams.Body != messageContent {
		t.Errorf("expected body to match message content")
	}
}

func TestSession_Data_NoRecipients(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, nil)
	s.sender = "sender@example.com"
	// No recipients set.

	err := s.Data(strings.NewReader("Subject: Test\r\n\r\nBody"))
	if err == nil {
		t.Fatal("expected error when no recipients")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 503 {
		t.Errorf("expected code 503, got %d", smtpErr.Code)
	}
}

func TestSession_Data_Unauthenticated(t *testing.T) {
	s := newTestSession(&mockQuerier{})

	err := s.Data(strings.NewReader("Subject: Test\r\n\r\nBody"))
	if err == nil {
		t.Fatal("expected error for unauthenticated session")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 530 {
		t.Errorf("expected code 530, got %d", smtpErr.Code)
	}
}

func TestSession_Data_EnqueueError(t *testing.T) {
	accountID := uuid.New()
	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, _ storage.EnqueueMessageParams) (storage.Message, error) {
			return storage.Message{}, errors.New("database error")
		},
	}

	s := newAuthenticatedSession(mock, accountID, nil)
	s.sender = "sender@example.com"
	s.recipients = []string{"recipient@example.com"}

	err := s.Data(strings.NewReader("Subject: Test\r\n\r\nBody"))
	if err == nil {
		t.Fatal("expected error when enqueue fails")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 451 {
		t.Errorf("expected code 451, got %d", smtpErr.Code)
	}
}

func TestSession_Data_NoSubjectHeader(t *testing.T) {
	accountID := uuid.New()
	var capturedParams storage.EnqueueMessageParams

	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
			capturedParams = arg
			return storage.Message{ID: uuid.New(), Status: storage.MessageStatusQueued}, nil
		},
	}

	s := newAuthenticatedSession(mock, accountID, nil)
	s.sender = "sender@example.com"
	s.recipients = []string{"recipient@example.com"}

	// Message with no Subject header.
	err := s.Data(strings.NewReader("From: sender@example.com\r\n\r\nPlain body"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if capturedParams.Subject.Valid {
		t.Errorf("expected subject to be invalid (empty), got %v", capturedParams.Subject)
	}
}

// --- Reset Test ---

func TestSession_Reset(t *testing.T) {
	accountID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, accountID, []string{"example.com"})
	s.sender = "sender@example.com"
	s.recipients = []string{"a@example.com", "b@example.com"}

	s.Reset()

	if s.sender != "" {
		t.Errorf("expected sender to be empty after reset, got %s", s.sender)
	}
	if s.recipients != nil {
		t.Errorf("expected recipients to be nil after reset, got %v", s.recipients)
	}
	// Authentication state should be preserved across reset.
	if !s.authenticated {
		t.Error("expected authentication to be preserved after reset")
	}
	if s.accountID != accountID {
		t.Error("expected accountID to be preserved after reset")
	}
}

// --- Logout Test ---

func TestSession_Logout_DecrementsCounter(t *testing.T) {
	mock := &mockQuerier{}
	log := zerolog.Nop()
	b := NewBackend(mock, log, 100)
	b.active.Add(3) // Simulate 3 active sessions.

	s := &Session{
		ctx:     context.Background(),
		queries: mock,
		log:     log,
		backend: b,
	}

	before := b.ActiveSessions()
	if before != 3 {
		t.Fatalf("expected 3 active sessions, got %d", before)
	}

	err := s.Logout()
	if err != nil {
		t.Fatalf("expected no error from logout, got %v", err)
	}

	after := b.ActiveSessions()
	if after != 2 {
		t.Errorf("expected 2 active sessions after logout, got %d", after)
	}
}

// Ensure unused imports do not cause issues.
var (
	_ sql.NullString
	_ pgtype.Timestamptz
)
