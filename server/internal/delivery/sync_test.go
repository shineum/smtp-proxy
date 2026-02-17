package delivery

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/routing"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name    string
	sendFn  func(ctx context.Context, msg *provider.Message) (*provider.DeliveryResult, error)
	healthFn func(ctx context.Context) error
}

func (m *mockProvider) Send(ctx context.Context, msg *provider.Message) (*provider.DeliveryResult, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return &provider.DeliveryResult{ProviderMessageID: "mock-id-123", Status: provider.StatusSent}, nil
}

func (m *mockProvider) GetName() string { return m.name }

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

// alwaysHealthy implements routing.HealthChecker.
type alwaysHealthy struct {
	healthy map[string]bool
}

func (h *alwaysHealthy) IsHealthy(name string) bool {
	if v, ok := h.healthy[name]; ok {
		return v
	}
	return true
}

// mockQuerier implements the subset of storage.Querier needed by SyncService.
type mockQuerier struct {
	updateStatusFn     func(ctx context.Context, arg storage.UpdateMessageStatusParams) error
	createDeliveryFn   func(ctx context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error)
	capturedStatus     storage.MessageStatus
	capturedLogParams  storage.CreateDeliveryLogParams
}

func (m *mockQuerier) CreateAccount(_ context.Context, _ storage.CreateAccountParams) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) CreateDeliveryLog(ctx context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	m.capturedLogParams = arg
	if m.createDeliveryFn != nil {
		return m.createDeliveryFn(ctx, arg)
	}
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) CreateProvider(_ context.Context, _ storage.CreateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}
func (m *mockQuerier) CreateRoutingRule(_ context.Context, _ storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}
func (m *mockQuerier) DeleteAccount(_ context.Context, _ uuid.UUID) error  { return nil }
func (m *mockQuerier) DeleteProvider(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) DeleteRoutingRule(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *mockQuerier) EnqueueMessage(_ context.Context, _ storage.EnqueueMessageParams) (storage.Message, error) {
	return storage.Message{}, nil
}
func (m *mockQuerier) GetAccountByAPIKey(_ context.Context, _ string) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) GetAccountByID(_ context.Context, _ uuid.UUID) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) GetAccountByName(_ context.Context, _ string) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) GetDeliveryLogByMessageID(_ context.Context, _ uuid.UUID) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) GetDeliveryLogByProviderMessageID(_ context.Context, _ sql.NullString) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
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
func (m *mockQuerier) IncrementRetryCount(_ context.Context, _ storage.IncrementRetryCountParams) error {
	return nil
}
func (m *mockQuerier) ListAccounts(_ context.Context) ([]storage.Account, error) {
	return nil, nil
}
func (m *mockQuerier) ListDeliveryLogsByMessageID(_ context.Context, _ uuid.UUID) ([]storage.DeliveryLog, error) {
	return nil, nil
}
func (m *mockQuerier) ListDeliveryLogsByTenantAndStatus(_ context.Context, _ storage.ListDeliveryLogsByTenantAndStatusParams) ([]storage.DeliveryLog, error) {
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
func (m *mockQuerier) UpdateDeliveryLogStatus(_ context.Context, _ storage.UpdateDeliveryLogStatusParams) error {
	return nil
}
func (m *mockQuerier) UpdateMessageStatus(ctx context.Context, arg storage.UpdateMessageStatusParams) error {
	m.capturedStatus = arg.Status
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, arg)
	}
	return nil
}
func (m *mockQuerier) UpdateProvider(_ context.Context, _ storage.UpdateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}
func (m *mockQuerier) UpdateRoutingRule(_ context.Context, _ storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

func TestSyncService_DeliverMessage_Success(t *testing.T) {
	registry := provider.NewRegistry()
	mp := &mockProvider{name: "sendgrid"}
	registry.Register(mp)

	hc := &alwaysHealthy{healthy: map[string]bool{"sendgrid": true}}
	router := routing.NewEngine(hc)

	mq := &mockQuerier{}
	log := zerolog.Nop()

	svc := NewSyncService(registry, router, mq, log)

	req := &Request{
		MessageID:  uuid.New(),
		AccountID:  uuid.New(),
		TenantID:   "tenant-1",
		Sender:     "sender@example.com",
		Recipients: []string{"recipient@example.com"},
		Subject:    "Test",
		Body:       []byte("Hello"),
	}

	err := svc.DeliverMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mq.capturedStatus != storage.MessageStatusDelivered {
		t.Errorf("expected status delivered, got %s", mq.capturedStatus)
	}

	if mq.capturedLogParams.Provider.String != "sendgrid" {
		t.Errorf("expected provider sendgrid, got %s", mq.capturedLogParams.Provider.String)
	}

	if mq.capturedLogParams.ProviderMessageID.String != "mock-id-123" {
		t.Errorf("expected provider message ID mock-id-123, got %s", mq.capturedLogParams.ProviderMessageID.String)
	}
}

func TestSyncService_DeliverMessage_SendError(t *testing.T) {
	registry := provider.NewRegistry()
	mp := &mockProvider{
		name: "sendgrid",
		sendFn: func(_ context.Context, _ *provider.Message) (*provider.DeliveryResult, error) {
			return nil, errors.New("send failed")
		},
	}
	registry.Register(mp)

	hc := &alwaysHealthy{healthy: map[string]bool{"sendgrid": true}}
	router := routing.NewEngine(hc)

	mq := &mockQuerier{}
	log := zerolog.Nop()

	svc := NewSyncService(registry, router, mq, log)

	req := &Request{
		MessageID:  uuid.New(),
		AccountID:  uuid.New(),
		TenantID:   "tenant-1",
		Sender:     "sender@example.com",
		Recipients: []string{"recipient@example.com"},
		Subject:    "Test",
		Body:       []byte("Hello"),
	}

	err := svc.DeliverMessage(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from send failure")
	}

	if mq.capturedStatus != storage.MessageStatusFailed {
		t.Errorf("expected status failed, got %s", mq.capturedStatus)
	}

	if !mq.capturedLogParams.LastError.Valid {
		t.Error("expected last_error to be set")
	}
}

func TestSyncService_DeliverMessage_NoHealthyProvider(t *testing.T) {
	registry := provider.NewRegistry()
	mp := &mockProvider{name: "sendgrid"}
	registry.Register(mp)

	// All providers unhealthy.
	hc := &alwaysHealthy{healthy: map[string]bool{"sendgrid": false, "ses": false, "mailgun": false, "msgraph": false}}
	router := routing.NewEngine(hc)

	mq := &mockQuerier{}
	log := zerolog.Nop()

	svc := NewSyncService(registry, router, mq, log)

	req := &Request{
		MessageID:  uuid.New(),
		AccountID:  uuid.New(),
		TenantID:   "tenant-1",
		Sender:     "sender@example.com",
		Recipients: []string{"recipient@example.com"},
		Subject:    "Test",
		Body:       []byte("Hello"),
	}

	err := svc.DeliverMessage(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when no healthy provider")
	}

	if mq.capturedStatus != storage.MessageStatusFailed {
		t.Errorf("expected status failed, got %s", mq.capturedStatus)
	}
}

func TestSyncService_DeliverMessage_Fallback(t *testing.T) {
	registry := provider.NewRegistry()
	mp1 := &mockProvider{name: "sendgrid"}
	mp2 := &mockProvider{name: "ses"}
	registry.Register(mp1)
	registry.Register(mp2)

	// Primary (sendgrid) is unhealthy, fallback to ses.
	hc := &alwaysHealthy{healthy: map[string]bool{"sendgrid": false, "ses": true}}
	router := routing.NewEngine(hc)

	mq := &mockQuerier{}
	log := zerolog.Nop()

	svc := NewSyncService(registry, router, mq, log)

	req := &Request{
		MessageID:  uuid.New(),
		AccountID:  uuid.New(),
		TenantID:   "tenant-1",
		Sender:     "sender@example.com",
		Recipients: []string{"recipient@example.com"},
		Subject:    "Test",
		Body:       []byte("Hello"),
	}

	err := svc.DeliverMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mq.capturedLogParams.Provider.String != "ses" {
		t.Errorf("expected provider ses (fallback), got %s", mq.capturedLogParams.Provider.String)
	}
}
