package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	"github.com/sungwon/smtp-proxy/server/internal/routing"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name   string
	sendFn func(ctx context.Context, msg *provider.Message) (*provider.DeliveryResult, error)
}

func (m *mockProvider) Send(ctx context.Context, msg *provider.Message) (*provider.DeliveryResult, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return &provider.DeliveryResult{
		ProviderMessageID: "worker-msg-123",
		Status:            provider.StatusSent,
		Timestamp:         time.Now(),
	}, nil
}

func (m *mockProvider) GetName() string                     { return m.name }
func (m *mockProvider) HealthCheck(_ context.Context) error { return nil }

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

// mockQuerier implements storage.Querier for testing.
type mockQuerier struct {
	statuses          []storage.MessageStatus
	createLogCalled   bool
	createLogProvider string
	createLogStatus   string
}

func (m *mockQuerier) CreateAccount(_ context.Context, _ storage.CreateAccountParams) (storage.Account, error) {
	return storage.Account{}, nil
}
func (m *mockQuerier) CreateDeliveryLog(_ context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	m.createLogCalled = true
	m.createLogProvider = arg.Provider.String
	m.createLogStatus = arg.Status
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
func (m *mockQuerier) ListAccounts(_ context.Context) ([]storage.Account, error) { return nil, nil }
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
func (m *mockQuerier) UpdateMessageStatus(_ context.Context, arg storage.UpdateMessageStatusParams) error {
	m.statuses = append(m.statuses, arg.Status)
	return nil
}
func (m *mockQuerier) UpdateProvider(_ context.Context, _ storage.UpdateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}
func (m *mockQuerier) UpdateRoutingRule(_ context.Context, _ storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

func TestHandler_HandleMessage_Success(t *testing.T) {
	registry := provider.NewRegistry()
	mp := &mockProvider{name: "sendgrid"}
	registry.Register(mp)

	hc := &alwaysHealthy{healthy: map[string]bool{"sendgrid": true}}
	router := routing.NewEngine(hc)

	mq := &mockQuerier{}
	log := zerolog.Nop()

	h := NewHandler(registry, router, mq, log)

	msgID := uuid.New()
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
	if mq.createLogProvider != "sendgrid" {
		t.Errorf("expected provider sendgrid, got %s", mq.createLogProvider)
	}
}

func TestHandler_HandleMessage_SendFail(t *testing.T) {
	registry := provider.NewRegistry()
	mp := &mockProvider{
		name: "sendgrid",
		sendFn: func(_ context.Context, _ *provider.Message) (*provider.DeliveryResult, error) {
			return nil, errors.New("connection refused")
		},
	}
	registry.Register(mp)

	hc := &alwaysHealthy{healthy: map[string]bool{"sendgrid": true}}
	router := routing.NewEngine(hc)

	mq := &mockQuerier{}
	log := zerolog.Nop()

	h := NewHandler(registry, router, mq, log)

	msgID := uuid.New()
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
		t.Fatal("expected error when provider send fails")
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
	registry := provider.NewRegistry()
	hc := &alwaysHealthy{}
	router := routing.NewEngine(hc)
	mq := &mockQuerier{}
	log := zerolog.Nop()

	h := NewHandler(registry, router, mq, log)

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

func TestHandler_HandleMessage_NoHealthyProvider(t *testing.T) {
	registry := provider.NewRegistry()
	mp := &mockProvider{name: "sendgrid"}
	registry.Register(mp)

	hc := &alwaysHealthy{healthy: map[string]bool{"sendgrid": false, "ses": false, "mailgun": false, "msgraph": false}}
	router := routing.NewEngine(hc)

	mq := &mockQuerier{}
	log := zerolog.Nop()

	h := NewHandler(registry, router, mq, log)

	msgID := uuid.New()
	msg := &queue.Message{
		ID:       msgID.String(),
		TenantID: "tenant-1",
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
	}

	err := h.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when no healthy provider")
	}

	// Final status should be failed.
	if len(mq.statuses) == 0 {
		t.Fatal("expected at least one status update")
	}
	if mq.statuses[len(mq.statuses)-1] != storage.MessageStatusFailed {
		t.Errorf("expected final status failed, got %s", mq.statuses[len(mq.statuses)-1])
	}
}

