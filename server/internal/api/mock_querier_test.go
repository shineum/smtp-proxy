package api

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// errNotFound is a sentinel error for not-found simulation.
var errNotFound = errors.New("not found")

// mockQuerier implements storage.Querier for testing.
type mockQuerier struct {
	// Account methods
	createAccountFn     func(ctx context.Context, arg storage.CreateAccountParams) (storage.Account, error)
	getAccountByAPIKeyFn func(ctx context.Context, apiKey string) (storage.Account, error)
	getAccountByIDFn    func(ctx context.Context, id uuid.UUID) (storage.Account, error)
	getAccountByNameFn  func(ctx context.Context, name string) (storage.Account, error)
	listAccountsFn      func(ctx context.Context) ([]storage.Account, error)
	updateAccountFn     func(ctx context.Context, arg storage.UpdateAccountParams) (storage.Account, error)
	deleteAccountFn     func(ctx context.Context, id uuid.UUID) error

	// Provider methods
	createProviderFn        func(ctx context.Context, arg storage.CreateProviderParams) (storage.EspProvider, error)
	getProviderByIDFn       func(ctx context.Context, id uuid.UUID) (storage.EspProvider, error)
	listProvidersByAccountFn func(ctx context.Context, accountID uuid.UUID) ([]storage.EspProvider, error)
	updateProviderFn        func(ctx context.Context, arg storage.UpdateProviderParams) (storage.EspProvider, error)
	deleteProviderFn        func(ctx context.Context, id uuid.UUID) error

	// Routing Rule methods
	createRoutingRuleFn        func(ctx context.Context, arg storage.CreateRoutingRuleParams) (storage.RoutingRule, error)
	getRoutingRuleByIDFn       func(ctx context.Context, id uuid.UUID) (storage.RoutingRule, error)
	listRoutingRulesByAccountFn func(ctx context.Context, accountID uuid.UUID) ([]storage.RoutingRule, error)
	updateRoutingRuleFn        func(ctx context.Context, arg storage.UpdateRoutingRuleParams) (storage.RoutingRule, error)
	deleteRoutingRuleFn        func(ctx context.Context, id uuid.UUID) error

	// Delivery Log methods
	getDeliveryLogByProviderMessageIDFn func(ctx context.Context, providerMessageID sql.NullString) (storage.DeliveryLog, error)
	updateDeliveryLogStatusFn           func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error
}

// --- Account methods ---

func (m *mockQuerier) CreateAccount(ctx context.Context, arg storage.CreateAccountParams) (storage.Account, error) {
	if m.createAccountFn != nil {
		return m.createAccountFn(ctx, arg)
	}
	return storage.Account{}, nil
}

func (m *mockQuerier) GetAccountByAPIKey(ctx context.Context, apiKey string) (storage.Account, error) {
	if m.getAccountByAPIKeyFn != nil {
		return m.getAccountByAPIKeyFn(ctx, apiKey)
	}
	return storage.Account{}, nil
}

func (m *mockQuerier) GetAccountByID(ctx context.Context, id uuid.UUID) (storage.Account, error) {
	if m.getAccountByIDFn != nil {
		return m.getAccountByIDFn(ctx, id)
	}
	return storage.Account{}, nil
}

func (m *mockQuerier) GetAccountByName(ctx context.Context, name string) (storage.Account, error) {
	if m.getAccountByNameFn != nil {
		return m.getAccountByNameFn(ctx, name)
	}
	return storage.Account{}, nil
}

func (m *mockQuerier) ListAccounts(ctx context.Context) ([]storage.Account, error) {
	if m.listAccountsFn != nil {
		return m.listAccountsFn(ctx)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateAccount(ctx context.Context, arg storage.UpdateAccountParams) (storage.Account, error) {
	if m.updateAccountFn != nil {
		return m.updateAccountFn(ctx, arg)
	}
	return storage.Account{}, nil
}

func (m *mockQuerier) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	if m.deleteAccountFn != nil {
		return m.deleteAccountFn(ctx, id)
	}
	return nil
}

// --- Provider methods ---

func (m *mockQuerier) CreateProvider(ctx context.Context, arg storage.CreateProviderParams) (storage.EspProvider, error) {
	if m.createProviderFn != nil {
		return m.createProviderFn(ctx, arg)
	}
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) GetProviderByID(ctx context.Context, id uuid.UUID) (storage.EspProvider, error) {
	if m.getProviderByIDFn != nil {
		return m.getProviderByIDFn(ctx, id)
	}
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) ListProvidersByAccountID(ctx context.Context, accountID uuid.UUID) ([]storage.EspProvider, error) {
	if m.listProvidersByAccountFn != nil {
		return m.listProvidersByAccountFn(ctx, accountID)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateProvider(ctx context.Context, arg storage.UpdateProviderParams) (storage.EspProvider, error) {
	if m.updateProviderFn != nil {
		return m.updateProviderFn(ctx, arg)
	}
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	if m.deleteProviderFn != nil {
		return m.deleteProviderFn(ctx, id)
	}
	return nil
}

// --- Routing Rule methods ---

func (m *mockQuerier) CreateRoutingRule(ctx context.Context, arg storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
	if m.createRoutingRuleFn != nil {
		return m.createRoutingRuleFn(ctx, arg)
	}
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) GetRoutingRuleByID(ctx context.Context, id uuid.UUID) (storage.RoutingRule, error) {
	if m.getRoutingRuleByIDFn != nil {
		return m.getRoutingRuleByIDFn(ctx, id)
	}
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) ListRoutingRulesByAccountID(ctx context.Context, accountID uuid.UUID) ([]storage.RoutingRule, error) {
	if m.listRoutingRulesByAccountFn != nil {
		return m.listRoutingRulesByAccountFn(ctx, accountID)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateRoutingRule(ctx context.Context, arg storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	if m.updateRoutingRuleFn != nil {
		return m.updateRoutingRuleFn(ctx, arg)
	}
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) DeleteRoutingRule(ctx context.Context, id uuid.UUID) error {
	if m.deleteRoutingRuleFn != nil {
		return m.deleteRoutingRuleFn(ctx, id)
	}
	return nil
}

// --- Message methods (implement interface, return zero values) ---

func (m *mockQuerier) EnqueueMessage(ctx context.Context, arg storage.EnqueueMessageParams) (storage.Message, error) {
	return storage.Message{}, nil
}

func (m *mockQuerier) GetMessageByID(ctx context.Context, id uuid.UUID) (storage.Message, error) {
	return storage.Message{}, nil
}

func (m *mockQuerier) GetQueuedMessages(ctx context.Context, limit int32) ([]storage.Message, error) {
	return nil, nil
}

func (m *mockQuerier) ListMessagesByAccountID(ctx context.Context, arg storage.ListMessagesByAccountIDParams) ([]storage.Message, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateMessageStatus(ctx context.Context, arg storage.UpdateMessageStatusParams) error {
	return nil
}

// --- Delivery Log methods (implement interface, return zero values) ---

func (m *mockQuerier) CreateDeliveryLog(ctx context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) GetDeliveryLogByMessageID(ctx context.Context, messageID uuid.UUID) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) GetDeliveryLogByProviderMessageID(ctx context.Context, providerMessageID sql.NullString) (storage.DeliveryLog, error) {
	if m.getDeliveryLogByProviderMessageIDFn != nil {
		return m.getDeliveryLogByProviderMessageIDFn(ctx, providerMessageID)
	}
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) IncrementRetryCount(ctx context.Context, arg storage.IncrementRetryCountParams) error {
	return nil
}

func (m *mockQuerier) ListDeliveryLogsByMessageID(ctx context.Context, messageID uuid.UUID) ([]storage.DeliveryLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListDeliveryLogsByTenantAndStatus(ctx context.Context, arg storage.ListDeliveryLogsByTenantAndStatusParams) ([]storage.DeliveryLog, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateDeliveryLogStatus(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
	if m.updateDeliveryLogStatusFn != nil {
		return m.updateDeliveryLogStatusFn(ctx, arg)
	}
	return nil
}

// --- Aggregate query methods (implement interface, return zero values) ---

func (m *mockQuerier) AverageDeliveryDuration(_ context.Context, _ storage.AverageDeliveryDurationParams) ([]storage.AverageDeliveryDurationRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByAccount(_ context.Context, _ storage.CountDeliveryLogsByAccountParams) ([]storage.CountDeliveryLogsByAccountRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByProvider(_ context.Context, _ storage.CountDeliveryLogsByProviderParams) ([]storage.CountDeliveryLogsByProviderRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByStatus(_ context.Context, _ storage.CountDeliveryLogsByStatusParams) ([]storage.CountDeliveryLogsByStatusRow, error) {
	return nil, nil
}

// --- Message metadata method (implement interface, return zero values) ---

func (m *mockQuerier) EnqueueMessageMetadata(_ context.Context, _ storage.EnqueueMessageMetadataParams) (storage.Message, error) {
	return storage.Message{}, nil
}

// --- Audit Log methods (implement interface, return zero values) ---

func (m *mockQuerier) CreateAuditLog(_ context.Context, _ storage.CreateAuditLogParams) (storage.AuditLog, error) {
	return storage.AuditLog{}, nil
}

func (m *mockQuerier) ListAuditLogsByTenantID(_ context.Context, _ storage.ListAuditLogsByTenantIDParams) ([]storage.AuditLog, error) {
	return nil, nil
}

// --- Session methods (implement interface, return zero values) ---

func (m *mockQuerier) CreateSession(_ context.Context, _ storage.CreateSessionParams) (storage.Session, error) {
	return storage.Session{}, nil
}

func (m *mockQuerier) DeleteSession(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteExpiredSessions(_ context.Context) error {
	return nil
}

func (m *mockQuerier) DeleteSessionsByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) GetSessionByID(_ context.Context, _ uuid.UUID) (storage.Session, error) {
	return storage.Session{}, nil
}

func (m *mockQuerier) ListSessionsByUserID(_ context.Context, _ uuid.UUID) ([]storage.Session, error) {
	return nil, nil
}

// --- Tenant methods (implement interface, return zero values) ---

func (m *mockQuerier) CreateTenant(_ context.Context, _ storage.CreateTenantParams) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}

func (m *mockQuerier) DeleteTenant(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) GetTenantByID(_ context.Context, _ uuid.UUID) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}

func (m *mockQuerier) GetTenantByName(_ context.Context, _ string) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}

func (m *mockQuerier) ListTenants(_ context.Context) ([]storage.Tenant, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateTenant(_ context.Context, _ storage.UpdateTenantParams) (storage.Tenant, error) {
	return storage.Tenant{}, nil
}

// --- User methods (implement interface, return zero values) ---

func (m *mockQuerier) CreateUser(_ context.Context, _ storage.CreateUserParams) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) DeleteUser(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) GetUserByEmail(_ context.Context, _ string) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByID(_ context.Context, _ uuid.UUID) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) IncrementFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) IncrementMonthlySent(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) ListUsersByTenantID(_ context.Context, _ uuid.UUID) ([]storage.User, error) {
	return nil, nil
}

func (m *mockQuerier) ResetFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) ResetMonthlySent(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) UpdateUserLastLogin(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) UpdateUserRole(_ context.Context, _ storage.UpdateUserRoleParams) (storage.User, error) {
	return storage.User{}, nil
}

func (m *mockQuerier) UpdateUserStatus(_ context.Context, _ storage.UpdateUserStatusParams) (storage.User, error) {
	return storage.User{}, nil
}

// --- Test helpers ---

// testAccount returns a sample Account for testing.
func testAccount() storage.Account {
	return storage.Account{
		ID:             uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:           "test-account",
		Email:          "test@example.com",
		PasswordHash:   "$2a$12$fakehash",
		AllowedDomains: []byte(`["example.com"]`),
		ApiKey:         "testapikey123",
		CreatedAt:      pgtype.Timestamptz{Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Valid: true},
	}
}

// testProvider returns a sample EspProvider for testing.
func testProvider() storage.EspProvider {
	return storage.EspProvider{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		AccountID:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:         "test-provider",
		ProviderType: storage.ProviderTypeSendgrid,
		ApiKey:       sql.NullString{String: "sg-key-123", Valid: true},
		SmtpConfig:   []byte(`{}`),
		Enabled:      true,
		CreatedAt:    pgtype.Timestamptz{Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Valid: true},
	}
}

// testRoutingRule returns a sample RoutingRule for testing.
func testRoutingRule() storage.RoutingRule {
	return storage.RoutingRule{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		AccountID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Priority:   10,
		Conditions: []byte(`{"from":"*@example.com"}`),
		ProviderID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Enabled:    true,
		CreatedAt:  pgtype.Timestamptz{Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Valid: true},
	}
}

// Compile-time verification that mockQuerier implements storage.Querier.
var _ storage.Querier = (*mockQuerier)(nil)
