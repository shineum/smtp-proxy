package delivery

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// mockQuerier implements storage.Querier for testing.
type mockQuerier struct {
	updateStatusFn    func(ctx context.Context, arg storage.UpdateMessageStatusParams) error
	createDeliveryFn  func(ctx context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error)
	listProvidersFn   func(ctx context.Context, accountID uuid.UUID) ([]storage.EspProvider, error)
	capturedStatus    storage.MessageStatus
	capturedLogParams storage.CreateDeliveryLogParams
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
func (m *mockQuerier) CreateDeliveryLog(ctx context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	m.capturedLogParams = arg
	if m.createDeliveryFn != nil {
		return m.createDeliveryFn(ctx, arg)
	}
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
func (m *mockQuerier) CountDeliveryLogsByStatus(_ context.Context, _ storage.CountDeliveryLogsByStatusParams) ([]storage.CountDeliveryLogsByStatusRow, error) {
	return nil, nil
}
func (m *mockQuerier) CountDeliveryLogsByProvider(_ context.Context, _ storage.CountDeliveryLogsByProviderParams) ([]storage.CountDeliveryLogsByProviderRow, error) {
	return nil, nil
}
func (m *mockQuerier) CountDeliveryLogsByAccount(_ context.Context, _ storage.CountDeliveryLogsByAccountParams) ([]storage.CountDeliveryLogsByAccountRow, error) {
	return nil, nil
}
func (m *mockQuerier) AverageDeliveryDuration(_ context.Context, _ storage.AverageDeliveryDurationParams) ([]storage.AverageDeliveryDurationRow, error) {
	return nil, nil
}

// Message methods.
func (m *mockQuerier) EnqueueMessage(_ context.Context, _ storage.EnqueueMessageParams) (storage.Message, error) {
	return storage.Message{}, nil
}
func (m *mockQuerier) EnqueueMessageMetadata(_ context.Context, _ storage.EnqueueMessageMetadataParams) (storage.Message, error) {
	return storage.Message{}, nil
}
func (m *mockQuerier) GetMessageByID(_ context.Context, _ uuid.UUID) (storage.Message, error) {
	return storage.Message{}, nil
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
func (m *mockQuerier) UpdateMessageStatus(ctx context.Context, arg storage.UpdateMessageStatusParams) error {
	m.capturedStatus = arg.Status
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, arg)
	}
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
