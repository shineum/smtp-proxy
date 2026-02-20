package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// mockQuerier implements storage.Querier for testing.
type mockQuerier struct {
	statuses          []storage.MessageStatus
	createLogCalled   bool
	createLogProvider string
	createLogStatus   string
	listProvidersFn   func(ctx context.Context, accountID uuid.UUID) ([]storage.EspProvider, error)
	getMessageFn      func(ctx context.Context, id uuid.UUID) (storage.Message, error)
}

// Account methods.
func (m *mockQuerier) CreateAccount(_ context.Context, _ storage.CreateAccountParams) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) DeleteAccount(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetAccountByAPIKey(_ context.Context, _ string) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) GetAccountByID(_ context.Context, _ uuid.UUID) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) GetAccountByName(_ context.Context, _ string) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) ListAccounts(_ context.Context) ([]storage.Account, error) { return nil, nil }
func (m *mockQuerier) UpdateAccount(_ context.Context, _ storage.UpdateAccountParams) (storage.Account, error) {
	return storage.Account{}, nil
}

// AuditLog methods.
func (m *mockQuerier) CreateAuditLog(_ context.Context, _ storage.CreateAuditLogParams) (storage.AuditLog, error) {
	return storage.AuditLog{}, nil
}
func (m *mockQuerier) ListAuditLogsByTenantID(_ context.Context, _ storage.ListAuditLogsByTenantIDParams) ([]storage.AuditLog, error) {
	return nil, nil
}

// DeliveryLog methods.
func (m *mockQuerier) CreateDeliveryLog(_ context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	m.createLogCalled = true
	m.createLogProvider = arg.Provider.String
	m.createLogStatus = arg.Status
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) GetDeliveryLogByMessageID(_ context.Context, _ uuid.UUID) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) GetDeliveryLogByProviderMessageID(_ context.Context, _ sql.NullString) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) ListDeliveryLogsByMessageID(_ context.Context, _ uuid.UUID) ([]storage.DeliveryLog, error) {
	return nil, nil
}
func (m *mockQuerier) ListDeliveryLogsByTenantAndStatus(_ context.Context, _ storage.ListDeliveryLogsByTenantAndStatusParams) ([]storage.DeliveryLog, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateDeliveryLogStatus(_ context.Context, _ storage.UpdateDeliveryLogStatusParams) error {
	return nil
}

// Message methods.
func (m *mockQuerier) EnqueueMessage(_ context.Context, _ storage.EnqueueMessageParams) (storage.Message, error) {
	return storage.Message{}, nil
}
func (m *mockQuerier) GetMessageByID(ctx context.Context, id uuid.UUID) (storage.Message, error) {
	if m.getMessageFn != nil {
		return m.getMessageFn(ctx, id)
	}
	return storage.Message{AccountID: uuid.New()}, nil
}
func (m *mockQuerier) GetQueuedMessages(_ context.Context, _ int32) ([]storage.Message, error) {
	return nil, nil
}
func (m *mockQuerier) IncrementRetryCount(_ context.Context, _ storage.IncrementRetryCountParams) error {
	return nil
}
func (m *mockQuerier) ListMessagesByAccountID(_ context.Context, _ storage.ListMessagesByAccountIDParams) ([]storage.Message, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateMessageStatus(_ context.Context, arg storage.UpdateMessageStatusParams) error {
	m.statuses = append(m.statuses, arg.Status)
	return nil
}

// Provider methods.
func (m *mockQuerier) CreateProvider(_ context.Context, _ storage.CreateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}
func (m *mockQuerier) DeleteProvider(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetProviderByID(_ context.Context, _ uuid.UUID) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}
func (m *mockQuerier) ListProvidersByAccountID(ctx context.Context, accountID uuid.UUID) ([]storage.EspProvider, error) {
	if m.listProvidersFn != nil {
		return m.listProvidersFn(ctx, accountID)
	}
	return nil, nil
}
func (m *mockQuerier) UpdateProvider(_ context.Context, _ storage.UpdateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}

// RoutingRule methods.
func (m *mockQuerier) CreateRoutingRule(_ context.Context, _ storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}
func (m *mockQuerier) DeleteRoutingRule(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetRoutingRuleByID(_ context.Context, _ uuid.UUID) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}
func (m *mockQuerier) ListRoutingRulesByAccountID(_ context.Context, _ uuid.UUID) ([]storage.RoutingRule, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateRoutingRule(_ context.Context, _ storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

// Session methods.
func (m *mockQuerier) CreateSession(_ context.Context, _ storage.CreateSessionParams) (storage.Session, error) {
	return storage.Session{}, nil
}
func (m *mockQuerier) DeleteSession(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) DeleteExpiredSessions(_ context.Context) error      { return nil }
func (m *mockQuerier) DeleteSessionsByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *mockQuerier) GetSessionByID(_ context.Context, _ uuid.UUID) (storage.Session, error) {
	return storage.Session{}, nil
}
func (m *mockQuerier) ListSessionsByUserID(_ context.Context, _ uuid.UUID) ([]storage.Session, error) {
	return nil, nil
}

// Tenant methods.
func (m *mockQuerier) CreateTenant(_ context.Context, _ storage.CreateTenantParams) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}
func (m *mockQuerier) DeleteTenant(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetTenantByID(_ context.Context, _ uuid.UUID) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}
func (m *mockQuerier) GetTenantByName(_ context.Context, _ string) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}
func (m *mockQuerier) ListTenants(_ context.Context) ([]storage.Tenant, error) { return nil, nil }
func (m *mockQuerier) UpdateTenant(_ context.Context, _ storage.UpdateTenantParams) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}

// User methods.
func (m *mockQuerier) CreateUser(_ context.Context, _ storage.CreateUserParams) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) DeleteUser(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetUserByEmail(_ context.Context, _ string) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) GetUserByID(_ context.Context, _ uuid.UUID) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) IncrementFailedAttempts(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) IncrementMonthlySent(_ context.Context, _ uuid.UUID) error    { return nil }
func (m *mockQuerier) ListUsersByTenantID(_ context.Context, _ uuid.UUID) ([]storage.User, error) {
	return nil, nil
}
func (m *mockQuerier) ResetFailedAttempts(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) ResetMonthlySent(_ context.Context, _ uuid.UUID) error    { return nil }
func (m *mockQuerier) UpdateUserLastLogin(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) UpdateUserRole(_ context.Context, _ storage.UpdateUserRoleParams) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) UpdateUserStatus(_ context.Context, _ storage.UpdateUserStatusParams) (storage.User, error) {
	return storage.User{}, nil
}

func TestHandler_HandleMessage_Success(t *testing.T) {
	accountID := uuid.New()
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return storage.Message{AccountID: accountID}, nil
		},
		// No providers -> stdout fallback.
	}
	log := zerolog.Nop()

	httpClient := provider.NewHTTPClient(0)
	resolver := provider.NewResolver(mq, httpClient, log)
	h := NewHandler(resolver, mq, log)

	msg := &queue.Message{
		ID:       msgID.String(),
		TenantID: "tenant-1",
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		Body:     []byte("Hello"),
	}

	err := h.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have set processing then delivered.
	if len(mq.statuses) < 2 {
		t.Fatalf("expected at least 2 status updates, got %d", len(mq.statuses))
	}
	if mq.statuses[0] != storage.MessageStatusProcessing {
		t.Errorf("expected first status to be processing, got %s", mq.statuses[0])
	}
	if mq.statuses[1] != storage.MessageStatusDelivered {
		t.Errorf("expected second status to be delivered, got %s", mq.statuses[1])
	}

	if !mq.createLogCalled {
		t.Error("expected delivery log to be created")
	}
	if mq.createLogProvider != "stdout" {
		t.Errorf("expected provider stdout, got %s", mq.createLogProvider)
	}
}

func TestHandler_HandleMessage_SendFail(t *testing.T) {
	accountID := uuid.New()
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return storage.Message{AccountID: accountID}, nil
		},
		listProvidersFn: func(_ context.Context, _ uuid.UUID) ([]storage.EspProvider, error) {
			return nil, errors.New("database error")
		},
	}
	log := zerolog.Nop()

	httpClient := provider.NewHTTPClient(0)
	resolver := provider.NewResolver(mq, httpClient, log)
	h := NewHandler(resolver, mq, log)

	msg := &queue.Message{
		ID:       msgID.String(),
		TenantID: "tenant-1",
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		Body:     []byte("Hello"),
	}

	err := h.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when resolver fails")
	}

	// Should have set processing then failed.
	if len(mq.statuses) < 2 {
		t.Fatalf("expected at least 2 status updates, got %d", len(mq.statuses))
	}
	if mq.statuses[len(mq.statuses)-1] != storage.MessageStatusFailed {
		t.Errorf("expected final status to be failed, got %s", mq.statuses[len(mq.statuses)-1])
	}

	if mq.createLogStatus != string(storage.MessageStatusFailed) {
		t.Errorf("expected log status failed, got %s", mq.createLogStatus)
	}
}

func TestHandler_HandleMessage_InvalidMessageID(t *testing.T) {
	mq := &mockQuerier{}
	log := zerolog.Nop()

	httpClient := provider.NewHTTPClient(0)
	resolver := provider.NewResolver(mq, httpClient, log)
	h := NewHandler(resolver, mq, log)

	msg := &queue.Message{
		ID:       "not-a-uuid",
		TenantID: "tenant-1",
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
	}

	err := h.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for invalid message ID")
	}
}

func TestHandler_HandleMessage_GetMessageError(t *testing.T) {
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return storage.Message{}, errors.New("message not found")
		},
	}
	log := zerolog.Nop()

	httpClient := provider.NewHTTPClient(0)
	resolver := provider.NewResolver(mq, httpClient, log)
	h := NewHandler(resolver, mq, log)

	msg := &queue.Message{
		ID:       msgID.String(),
		TenantID: "tenant-1",
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
	}

	err := h.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when message not found in database")
	}

	// Final status should be failed.
	if len(mq.statuses) == 0 {
		t.Fatal("expected at least one status update")
	}
	if mq.statuses[len(mq.statuses)-1] != storage.MessageStatusFailed {
		t.Errorf("expected final status failed, got %s", mq.statuses[len(mq.statuses)-1])
	}
}
