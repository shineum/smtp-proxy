package smtp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/delivery"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// mockDeliveryService implements delivery.Service for testing.
type mockDeliveryService struct {
	deliverFn func(ctx context.Context, req *delivery.Request) error
}

func (m *mockDeliveryService) DeliverMessage(ctx context.Context, req *delivery.Request) error {
	if m.deliverFn != nil {
		return m.deliverFn(ctx, req)
	}
	return nil
}

// errNotFound is a sentinel error for simulating "not found" database results.
var errNotFound = errors.New("no rows")

// mockQuerier implements storage.Querier with controllable responses.
type mockQuerier struct {
	// Auth-related behavior
	getUserByUsernameFn  func(ctx context.Context, username sql.NullString) (storage.User, error)
	listGroupsByUserIDFn func(ctx context.Context, userID uuid.UUID) ([]storage.Group, error)
	getGroupByIDFn       func(ctx context.Context, id uuid.UUID) (storage.Group, error)

	// EnqueueMessage behavior
	enqueueMessageFn func(ctx context.Context, arg storage.EnqueueMessageParams) (storage.Message, error)

	// EnqueueMessageMetadata behavior
	enqueueMessageMetadataFn func(ctx context.Context, arg storage.EnqueueMessageMetadataParams) (storage.Message, error)

	// UpdateMessageStatus behavior
	updateMessageStatusFn func(ctx context.Context, arg storage.UpdateMessageStatusParams) error
}

// --- Stub implementations for the full Querier interface ---

func (m *mockQuerier) AverageDeliveryDuration(_ context.Context, _ storage.AverageDeliveryDurationParams) ([]storage.AverageDeliveryDurationRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByGroup(_ context.Context, _ storage.CountDeliveryLogsByGroupParams) ([]storage.CountDeliveryLogsByGroupRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByProvider(_ context.Context, _ storage.CountDeliveryLogsByProviderParams) ([]storage.CountDeliveryLogsByProviderRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByStatus(_ context.Context, _ storage.CountDeliveryLogsByStatusParams) ([]storage.CountDeliveryLogsByStatusRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountGroupOwners(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *mockQuerier) CreateActivityLog(_ context.Context, _ storage.CreateActivityLogParams) (storage.ActivityLog, error) {
	return storage.ActivityLog{}, nil
}

func (m *mockQuerier) CreateDeliveryLog(_ context.Context, _ storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) CreateGroup(_ context.Context, _ storage.CreateGroupParams) (storage.Group, error) {
	return storage.Group{}, nil
}

func (m *mockQuerier) CreateGroupMember(_ context.Context, _ storage.CreateGroupMemberParams) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
}

func (m *mockQuerier) CreateProvider(_ context.Context, _ storage.CreateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) CreateRoutingRule(_ context.Context, _ storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) CreateSession(_ context.Context, _ storage.CreateSessionParams) (storage.Session, error) {
	return storage.Session{}, nil
}

func (m *mockQuerier) CreateUser(_ context.Context, _ storage.CreateUserParams) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) DeleteExpiredSessions(_ context.Context) error {
	return nil
}

func (m *mockQuerier) DeleteGroup(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteGroupMember(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteGroupMembersByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteProvider(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteRoutingRule(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteSession(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteSessionsByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteUser(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) EnqueueMessage(ctx context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
	if m.enqueueMessageFn != nil {
		return m.enqueueMessageFn(ctx, arg)
	}
	return storage.Message{
		ID:     uuid.New(),
		UserID: arg.UserID,
		Status: storage.MessageStatusQueued,
	}, nil
}

func (m *mockQuerier) EnqueueMessageMetadata(ctx context.Context, arg storage.EnqueueMessageMetadataParams) (storage.Message, error) {
	if m.enqueueMessageMetadataFn != nil {
		return m.enqueueMessageMetadataFn(ctx, arg)
	}
	return storage.Message{
		ID:     uuid.New(),
		UserID: arg.UserID,
		Status: storage.MessageStatusQueued,
	}, nil
}

func (m *mockQuerier) GetActivityLogByID(_ context.Context, _ uuid.UUID) (storage.ActivityLog, error) {
	return storage.ActivityLog{}, nil
}

func (m *mockQuerier) GetDeliveryLogByMessageID(_ context.Context, _ uuid.UUID) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) GetDeliveryLogByProviderMessageID(_ context.Context, _ sql.NullString) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) GetGroupByID(ctx context.Context, id uuid.UUID) (storage.Group, error) {
	if m.getGroupByIDFn != nil {
		return m.getGroupByIDFn(ctx, id)
	}
	return storage.Group{}, errNotFound
}

func (m *mockQuerier) GetGroupByName(_ context.Context, _ string) (storage.Group, error) {
	return storage.Group{}, nil
}

func (m *mockQuerier) GetGroupMemberByID(_ context.Context, _ uuid.UUID) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
}

func (m *mockQuerier) GetGroupMemberByUserAndGroup(_ context.Context, _ storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
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

func (m *mockQuerier) GetSessionByID(_ context.Context, _ uuid.UUID) (storage.Session, error) {
	return storage.Session{}, nil
}

func (m *mockQuerier) GetUserByAPIKey(_ context.Context, _ sql.NullString) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByEmail(_ context.Context, _ string) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByID(_ context.Context, _ uuid.UUID) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByUsername(ctx context.Context, username sql.NullString) (storage.User, error) {
	if m.getUserByUsernameFn != nil {
		return m.getUserByUsernameFn(ctx, username)
	}
	return storage.User{}, errNotFound
}

func (m *mockQuerier) IncrementFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) IncrementMonthlySent(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) IncrementRetryCount(_ context.Context, _ storage.IncrementRetryCountParams) error {
	return nil
}

func (m *mockQuerier) ListActivityLogsByActorID(_ context.Context, _ storage.ListActivityLogsByActorIDParams) ([]storage.ActivityLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListActivityLogsByGroupID(_ context.Context, _ storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListActivityLogsByResource(_ context.Context, _ storage.ListActivityLogsByResourceParams) ([]storage.ActivityLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListDeliveryLogsByGroupAndStatus(_ context.Context, _ storage.ListDeliveryLogsByGroupAndStatusParams) ([]storage.DeliveryLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListDeliveryLogsByMessageID(_ context.Context, _ uuid.UUID) ([]storage.DeliveryLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListGroupMembersByGroupID(_ context.Context, _ uuid.UUID) ([]storage.GroupMember, error) {
	return nil, nil
}

func (m *mockQuerier) ListGroups(_ context.Context) ([]storage.Group, error) {
	return nil, nil
}

func (m *mockQuerier) ListGroupsByUserID(ctx context.Context, userID uuid.UUID) ([]storage.Group, error) {
	if m.listGroupsByUserIDFn != nil {
		return m.listGroupsByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockQuerier) ListMessagesByGroupID(_ context.Context, _ storage.ListMessagesByGroupIDParams) ([]storage.Message, error) {
	return nil, nil
}

func (m *mockQuerier) ListProvidersByGroupID(_ context.Context, _ uuid.UUID) ([]storage.EspProvider, error) {
	return nil, nil
}

func (m *mockQuerier) ListRoutingRulesByGroupID(_ context.Context, _ uuid.UUID) ([]storage.RoutingRule, error) {
	return nil, nil
}

func (m *mockQuerier) ListSessionsByUserID(_ context.Context, _ uuid.UUID) ([]storage.Session, error) {
	return nil, nil
}

func (m *mockQuerier) ListUsers(_ context.Context) ([]storage.User, error) {
	return nil, nil
}

func (m *mockQuerier) ResetFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) ResetMonthlySent(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) UpdateDeliveryLogStatus(_ context.Context, _ storage.UpdateDeliveryLogStatusParams) error {
	return nil
}

func (m *mockQuerier) UpdateGroup(_ context.Context, _ storage.UpdateGroupParams) (storage.Group, error) {
	return storage.Group{}, nil
}

func (m *mockQuerier) UpdateGroupMemberRole(_ context.Context, _ storage.UpdateGroupMemberRoleParams) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
}

func (m *mockQuerier) UpdateGroupStatus(_ context.Context, _ storage.UpdateGroupStatusParams) (storage.Group, error) {
	return storage.Group{}, nil
}

func (m *mockQuerier) UpdateMessageStatus(ctx context.Context, arg storage.UpdateMessageStatusParams) error {
	if m.updateMessageStatusFn != nil {
		return m.updateMessageStatusFn(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) UpdateProvider(_ context.Context, _ storage.UpdateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) UpdateRoutingRule(_ context.Context, _ storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) UpdateUser(_ context.Context, _ storage.UpdateUserParams) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) UpdateUserLastLogin(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) UpdateUserPassword(_ context.Context, _ storage.UpdateUserPasswordParams) error {
	return nil
}

func (m *mockQuerier) UpdateUserStatus(_ context.Context, _ storage.UpdateUserStatusParams) (storage.User, error) {
	return storage.User{}, nil
}

// newTestSession creates a Session with a mock backend for testing.
func newTestSession(mock *mockQuerier) *Session {
	log := zerolog.Nop()
	b := NewBackend(mock, &mockDeliveryService{}, nil, log, 100)
	b.active.Add(1) // Simulate that the session was counted on creation.
	return &Session{
		ctx:     context.Background(),
		queries: mock,
		log:     log,
		backend: b,
	}
}

// newAuthenticatedSession creates a session that has already been authenticated.
func newAuthenticatedSession(mock *mockQuerier, userID, groupID uuid.UUID, allowedDomains []string) *Session {
	s := newTestSession(mock)
	s.userID = userID
	s.groupID = groupID
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

// newMockWithAuth creates a mockQuerier pre-configured for a successful auth flow.
func newMockWithAuth(userID, groupID uuid.UUID, passwordHash string, domainsJSON []byte) *mockQuerier {
	return &mockQuerier{
		getUserByUsernameFn: func(_ context.Context, username sql.NullString) (storage.User, error) {
			if username.String == "testuser" {
				return storage.User{
					ID:             userID,
					Username:       sql.NullString{String: "testuser", Valid: true},
					PasswordHash:   passwordHash,
					AccountType:    "smtp",
					Status:         "active",
					AllowedDomains: domainsJSON,
				}, nil
			}
			return storage.User{}, errNotFound
		},
		listGroupsByUserIDFn: func(_ context.Context, _ uuid.UUID) ([]storage.Group, error) {
			return []storage.Group{{ID: groupID, Name: "test-group", Status: "active"}}, nil
		},
		getGroupByIDFn: func(_ context.Context, id uuid.UUID) (storage.Group, error) {
			if id == groupID {
				return storage.Group{ID: groupID, Name: "test-group", Status: "active"}, nil
			}
			return storage.Group{}, errNotFound
		},
	}
}

// --- Auth Tests ---

// authenticateSession exercises the full SASL PLAIN flow via AuthMechanisms + Auth.
func authenticateSession(t *testing.T, s *Session, username, password string) error {
	t.Helper()

	mechs := s.AuthMechanisms()
	if len(mechs) != 1 || mechs[0] != sasl.Plain {
		t.Fatalf("expected [PLAIN], got %v", mechs)
	}

	server, err := s.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth(PLAIN) returned error: %v", err)
	}

	// SASL PLAIN initial response: \x00username\x00password
	response := []byte("\x00" + username + "\x00" + password)
	_, done, err := server.Next(response)
	if err != nil {
		return err
	}
	if !done {
		t.Fatal("expected SASL exchange to be done after one step")
	}
	return nil
}

func TestSession_Auth_Success(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	passwordHash := hashTestPassword(t, "correct-password")
	domainsJSON, _ := json.Marshal([]string{"example.com"})

	mock := newMockWithAuth(userID, groupID, passwordHash, domainsJSON)
	s := newTestSession(mock)

	err := authenticateSession(t, s, "testuser", "correct-password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !s.authenticated {
		t.Error("expected session to be authenticated")
	}
	if s.userID != userID {
		t.Errorf("expected userID=%s, got %s", userID, s.userID)
	}
	if s.groupID != groupID {
		t.Errorf("expected groupID=%s, got %s", groupID, s.groupID)
	}
	if len(s.allowedDomains) != 1 || s.allowedDomains[0] != "example.com" {
		t.Errorf("expected allowedDomains=[example.com], got %v", s.allowedDomains)
	}
}

func TestSession_Auth_InvalidPassword(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	passwordHash := hashTestPassword(t, "correct-password")

	mock := newMockWithAuth(userID, groupID, passwordHash, nil)
	s := newTestSession(mock)

	err := authenticateSession(t, s, "testuser", "wrong-password")
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

func TestSession_Auth_UnknownUser(t *testing.T) {
	mock := &mockQuerier{
		getUserByUsernameFn: func(_ context.Context, _ sql.NullString) (storage.User, error) {
			return storage.User{}, errNotFound
		},
	}

	s := newTestSession(mock)
	err := authenticateSession(t, s, "unknown", "any-password")
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

func TestSession_Auth_UnsupportedMechanism(t *testing.T) {
	s := newTestSession(&mockQuerier{})

	_, err := s.Auth("LOGIN")
	if err == nil {
		t.Fatal("expected error for unsupported mechanism")
	}
}

func TestSession_Auth_InactiveUser(t *testing.T) {
	userID := uuid.New()
	passwordHash := hashTestPassword(t, "correct-password")

	mock := &mockQuerier{
		getUserByUsernameFn: func(_ context.Context, _ sql.NullString) (storage.User, error) {
			return storage.User{
				ID:           userID,
				Username:     sql.NullString{String: "testuser", Valid: true},
				PasswordHash: passwordHash,
				AccountType:  "smtp",
				Status:       "suspended",
			}, nil
		},
	}

	s := newTestSession(mock)
	err := authenticateSession(t, s, "testuser", "correct-password")
	if err == nil {
		t.Fatal("expected error for inactive user")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 535 {
		t.Errorf("expected code 535, got %d", smtpErr.Code)
	}
}

func TestSession_Auth_NonSmtpAccountType(t *testing.T) {
	userID := uuid.New()
	passwordHash := hashTestPassword(t, "correct-password")

	mock := &mockQuerier{
		getUserByUsernameFn: func(_ context.Context, _ sql.NullString) (storage.User, error) {
			return storage.User{
				ID:           userID,
				Username:     sql.NullString{String: "testuser", Valid: true},
				PasswordHash: passwordHash,
				AccountType:  "human",
				Status:       "active",
			}, nil
		},
	}

	s := newTestSession(mock)
	err := authenticateSession(t, s, "testuser", "correct-password")
	if err == nil {
		t.Fatal("expected error for non-smtp account type")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 535 {
		t.Errorf("expected code 535, got %d", smtpErr.Code)
	}
}

func TestSession_Auth_NoGroupMembership(t *testing.T) {
	userID := uuid.New()
	passwordHash := hashTestPassword(t, "correct-password")

	mock := &mockQuerier{
		getUserByUsernameFn: func(_ context.Context, _ sql.NullString) (storage.User, error) {
			return storage.User{
				ID:           userID,
				Username:     sql.NullString{String: "testuser", Valid: true},
				PasswordHash: passwordHash,
				AccountType:  "smtp",
				Status:       "active",
			}, nil
		},
		listGroupsByUserIDFn: func(_ context.Context, _ uuid.UUID) ([]storage.Group, error) {
			return nil, nil // No groups
		},
	}

	s := newTestSession(mock)
	err := authenticateSession(t, s, "testuser", "correct-password")
	if err == nil {
		t.Fatal("expected error for no group membership")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 535 {
		t.Errorf("expected code 535, got %d", smtpErr.Code)
	}
}

func TestSession_Auth_SuspendedGroup(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	passwordHash := hashTestPassword(t, "correct-password")

	mock := &mockQuerier{
		getUserByUsernameFn: func(_ context.Context, _ sql.NullString) (storage.User, error) {
			return storage.User{
				ID:           userID,
				Username:     sql.NullString{String: "testuser", Valid: true},
				PasswordHash: passwordHash,
				AccountType:  "smtp",
				Status:       "active",
			}, nil
		},
		listGroupsByUserIDFn: func(_ context.Context, _ uuid.UUID) ([]storage.Group, error) {
			return []storage.Group{{ID: groupID, Name: "test-group", Status: "suspended"}}, nil
		},
		getGroupByIDFn: func(_ context.Context, _ uuid.UUID) (storage.Group, error) {
			return storage.Group{ID: groupID, Name: "test-group", Status: "suspended"}, nil
		},
	}

	s := newTestSession(mock)
	err := authenticateSession(t, s, "testuser", "correct-password")
	if err == nil {
		t.Fatal("expected error for suspended group")
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
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, []string{"example.com"})

	err := s.Mail("sender@example.com", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.sender != "sender@example.com" {
		t.Errorf("expected sender=sender@example.com, got %s", s.sender)
	}
}

func TestSession_Mail_ValidSender_NoDomainRestrictions(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, nil)

	err := s.Mail("sender@anydomain.com", nil)
	if err != nil {
		t.Fatalf("expected no error when no domain restrictions, got %v", err)
	}
}

func TestSession_Mail_UnauthorizedDomain(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, []string{"allowed.com"})

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
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, nil)

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
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, nil)

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
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, nil)

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
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, nil)

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
	userID := uuid.New()
	groupID := uuid.New()
	var capturedParams storage.EnqueueMessageParams

	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
			capturedParams = arg
			return storage.Message{
				ID:     uuid.New(),
				UserID: arg.UserID,
				Status: storage.MessageStatusQueued,
			}, nil
		},
	}

	s := newAuthenticatedSession(mock, userID, groupID, nil)
	s.sender = "sender@example.com"
	s.recipients = []string{"recipient@example.com"}

	messageContent := "Subject: Test\r\n\r\nHello, World!"
	err := s.Data(strings.NewReader(messageContent))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedUserPgID := pgtype.UUID{Bytes: userID, Valid: true}
	expectedGroupPgID := pgtype.UUID{Bytes: groupID, Valid: true}
	if capturedParams.UserID != expectedUserPgID {
		t.Errorf("expected UserID=%v, got %v", expectedUserPgID, capturedParams.UserID)
	}
	if capturedParams.GroupID != expectedGroupPgID {
		t.Errorf("expected GroupID=%v, got %v", expectedGroupPgID, capturedParams.GroupID)
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

	if capturedParams.Body.String != messageContent || !capturedParams.Body.Valid {
		t.Errorf("expected body to match message content")
	}
}

func TestSession_Data_NoRecipients(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, nil)
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
	userID := uuid.New()
	groupID := uuid.New()
	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, _ storage.EnqueueMessageParams) (storage.Message, error) {
			return storage.Message{}, errors.New("database error")
		},
	}

	s := newAuthenticatedSession(mock, userID, groupID, nil)
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
	userID := uuid.New()
	groupID := uuid.New()
	var capturedParams storage.EnqueueMessageParams

	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
			capturedParams = arg
			return storage.Message{ID: uuid.New(), Status: storage.MessageStatusQueued}, nil
		},
	}

	s := newAuthenticatedSession(mock, userID, groupID, nil)
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
	userID := uuid.New()
	groupID := uuid.New()
	s := newAuthenticatedSession(&mockQuerier{}, userID, groupID, []string{"example.com"})
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
	if s.userID != userID {
		t.Error("expected userID to be preserved after reset")
	}
	if s.groupID != groupID {
		t.Error("expected groupID to be preserved after reset")
	}
}

// --- Logout Test ---

func TestSession_Logout_DecrementsCounter(t *testing.T) {
	mock := &mockQuerier{}
	log := zerolog.Nop()
	b := NewBackend(mock, &mockDeliveryService{}, nil, log, 100)
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

// --- MessageStore Tests ---

// mockMessageStore implements msgstore.MessageStore for testing.
type mockMessageStore struct {
	putFn    func(ctx context.Context, messageID string, data []byte) error
	getFn    func(ctx context.Context, messageID string) ([]byte, error)
	deleteFn func(ctx context.Context, messageID string) error
}

func (m *mockMessageStore) Put(ctx context.Context, messageID string, data []byte) error {
	if m.putFn != nil {
		return m.putFn(ctx, messageID, data)
	}
	return nil
}

func (m *mockMessageStore) Get(ctx context.Context, messageID string) ([]byte, error) {
	if m.getFn != nil {
		return m.getFn(ctx, messageID)
	}
	return nil, nil
}

func (m *mockMessageStore) Delete(ctx context.Context, messageID string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, messageID)
	}
	return nil
}

func TestSession_Data_WithMessageStore(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	var putCalled bool
	var capturedPutData []byte
	var capturedMetadataParams storage.EnqueueMessageMetadataParams

	mockStore := &mockMessageStore{
		putFn: func(_ context.Context, _ string, data []byte) error {
			putCalled = true
			capturedPutData = data
			return nil
		},
	}

	mock := &mockQuerier{
		enqueueMessageMetadataFn: func(_ context.Context, arg storage.EnqueueMessageMetadataParams) (storage.Message, error) {
			capturedMetadataParams = arg
			return storage.Message{
				ID:     uuid.New(),
				UserID: arg.UserID,
				Status: storage.MessageStatusQueued,
			}, nil
		},
	}

	log := zerolog.Nop()
	b := NewBackend(mock, &mockDeliveryService{}, mockStore, log, 100)
	b.active.Add(1)
	s := &Session{
		ctx:           context.Background(),
		queries:       mock,
		log:           log,
		backend:       b,
		userID:        userID,
		groupID:       groupID,
		authenticated: true,
		sender:        "sender@example.com",
		recipients:    []string{"recipient@example.com"},
	}

	messageContent := "Subject: Test\r\n\r\nHello, World!"
	err := s.Data(strings.NewReader(messageContent))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !putCalled {
		t.Error("expected MessageStore.Put to be called")
	}
	if string(capturedPutData) != messageContent {
		t.Errorf("expected Put data to match message content")
	}

	// Verify EnqueueMessageMetadata was called (not EnqueueMessage).
	expectedUserPgID := pgtype.UUID{Bytes: userID, Valid: true}
	expectedGroupPgID := pgtype.UUID{Bytes: groupID, Valid: true}
	if capturedMetadataParams.UserID != expectedUserPgID {
		t.Errorf("expected UserID=%v, got %v", expectedUserPgID, capturedMetadataParams.UserID)
	}
	if capturedMetadataParams.GroupID != expectedGroupPgID {
		t.Errorf("expected GroupID=%v, got %v", expectedGroupPgID, capturedMetadataParams.GroupID)
	}
	if capturedMetadataParams.Sender != "sender@example.com" {
		t.Errorf("expected sender=sender@example.com, got %s", capturedMetadataParams.Sender)
	}
	if !capturedMetadataParams.StorageRef.Valid {
		t.Error("expected StorageRef to be valid")
	}
	if capturedMetadataParams.StorageRef.String == "" {
		t.Error("expected StorageRef to be non-empty")
	}
}

func TestSession_Data_MessageStoreWriteFails_FallsBack(t *testing.T) {
	userID := uuid.New()
	groupID := uuid.New()
	var enqueueMessageCalled bool
	var capturedParams storage.EnqueueMessageParams

	mockStore := &mockMessageStore{
		putFn: func(_ context.Context, _ string, _ []byte) error {
			return errors.New("disk full")
		},
	}

	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
			enqueueMessageCalled = true
			capturedParams = arg
			return storage.Message{
				ID:     uuid.New(),
				UserID: arg.UserID,
				Status: storage.MessageStatusQueued,
			}, nil
		},
	}

	log := zerolog.Nop()
	b := NewBackend(mock, &mockDeliveryService{}, mockStore, log, 100)
	b.active.Add(1)
	s := &Session{
		ctx:           context.Background(),
		queries:       mock,
		log:           log,
		backend:       b,
		userID:        userID,
		groupID:       groupID,
		authenticated: true,
		sender:        "sender@example.com",
		recipients:    []string{"recipient@example.com"},
	}

	messageContent := "Subject: Fallback\r\n\r\nBody text"
	err := s.Data(strings.NewReader(messageContent))
	if err != nil {
		t.Fatalf("expected no error (fallback should succeed), got %v", err)
	}

	if !enqueueMessageCalled {
		t.Error("expected EnqueueMessage (inline fallback) to be called")
	}
	if capturedParams.Body.String != messageContent || !capturedParams.Body.Valid {
		t.Error("expected inline body to match message content")
	}
}

// --- Enqueue Retry Tests ---

func TestSession_Data_EnqueueRetrySucceeds(t *testing.T) {
	origBackoff := enqueueRetryBackoff
	enqueueRetryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { enqueueRetryBackoff = origBackoff }()

	userID := uuid.New()
	groupID := uuid.New()
	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
			return storage.Message{ID: uuid.New(), UserID: arg.UserID, Status: storage.MessageStatusQueued}, nil
		},
	}

	callCount := 0
	deliverySvc := &mockDeliveryService{
		deliverFn: func(_ context.Context, _ *delivery.Request) error {
			callCount++
			if callCount == 1 {
				return errors.New("redis connection refused")
			}
			return nil
		},
	}

	log := zerolog.Nop()
	b := NewBackend(mock, deliverySvc, nil, log, 100)
	b.active.Add(1)
	s := &Session{
		ctx:           context.Background(),
		queries:       mock,
		log:           log,
		backend:       b,
		userID:        userID,
		groupID:       groupID,
		authenticated: true,
		sender:        "sender@example.com",
		recipients:    []string{"recipient@example.com"},
	}

	err := s.Data(strings.NewReader("Subject: Test\r\n\r\nHello"))
	if err != nil {
		t.Fatalf("expected no error after retry success, got %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 delivery attempts, got %d", callCount)
	}
}

func TestSession_Data_EnqueueRetryExhausted(t *testing.T) {
	origBackoff := enqueueRetryBackoff
	enqueueRetryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { enqueueRetryBackoff = origBackoff }()

	userID := uuid.New()
	groupID := uuid.New()
	var capturedStatus storage.MessageStatus
	mock := &mockQuerier{
		enqueueMessageFn: func(_ context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
			return storage.Message{ID: uuid.New(), UserID: arg.UserID, Status: storage.MessageStatusQueued}, nil
		},
		updateMessageStatusFn: func(_ context.Context, arg storage.UpdateMessageStatusParams) error {
			capturedStatus = arg.Status
			return nil
		},
	}

	deliverySvc := &mockDeliveryService{
		deliverFn: func(_ context.Context, _ *delivery.Request) error {
			return errors.New("redis connection refused")
		},
	}

	log := zerolog.Nop()
	b := NewBackend(mock, deliverySvc, nil, log, 100)
	b.active.Add(1)
	s := &Session{
		ctx:           context.Background(),
		queries:       mock,
		log:           log,
		backend:       b,
		userID:        userID,
		groupID:       groupID,
		authenticated: true,
		sender:        "sender@example.com",
		recipients:    []string{"recipient@example.com"},
	}

	err := s.Data(strings.NewReader("Subject: Test\r\n\r\nHello"))
	if err == nil {
		t.Fatal("expected SMTP error after retry exhaustion")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected SMTPError, got %T", err)
	}
	if smtpErr.Code != 451 {
		t.Errorf("expected code 451, got %d", smtpErr.Code)
	}

	if capturedStatus != storage.MessageStatusEnqueueFailed {
		t.Errorf("expected status enqueue_failed, got %s", capturedStatus)
	}
}
