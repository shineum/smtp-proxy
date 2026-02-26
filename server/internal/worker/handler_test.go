package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/msgstore"
	"github.com/sungwon/smtp-proxy/server/internal/provider"
	"github.com/sungwon/smtp-proxy/server/internal/queue"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// ---------------------------------------------------------------------------
// Mock: storage.Querier
// ---------------------------------------------------------------------------

type mockQuerier struct {
	statuses          []storage.MessageStatus
	createLogCalled   bool
	createLogProvider string
	createLogStatus   string
	createLogParams   storage.CreateDeliveryLogParams
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
	m.createLogParams = arg
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

// DeliveryLog analytics methods.
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

// Message methods.
func (m *mockQuerier) EnqueueMessage(_ context.Context, _ storage.EnqueueMessageParams) (storage.Message, error) {
	return storage.Message{}, nil
}
func (m *mockQuerier) EnqueueMessageMetadata(_ context.Context, _ storage.EnqueueMessageMetadataParams) (storage.Message, error) {
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

// ---------------------------------------------------------------------------
// Mock: msgstore.MessageStore
// ---------------------------------------------------------------------------

type mockMessageStore struct {
	data    map[string][]byte
	getFn   func(ctx context.Context, messageID string) ([]byte, error)
	getCalls int32 // atomic counter for Get calls
}

func (m *mockMessageStore) Put(_ context.Context, messageID string, data []byte) error {
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[messageID] = data
	return nil
}

func (m *mockMessageStore) Get(ctx context.Context, messageID string) ([]byte, error) {
	atomic.AddInt32(&m.getCalls, 1)
	if m.getFn != nil {
		return m.getFn(ctx, messageID)
	}
	if m.data != nil {
		if d, ok := m.data[messageID]; ok {
			return d, nil
		}
	}
	return nil, msgstore.ErrNotFound
}

func (m *mockMessageStore) Delete(_ context.Context, _ string) error {
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestDBMessage creates a storage.Message with populated metadata fields
// suitable for building a provider.Message.
func newTestDBMessage(accountID uuid.UUID) storage.Message {
	recipients, _ := json.Marshal([]string{"recipient@example.com"})
	headers, _ := json.Marshal(map[string][]string{"X-Test": {"value1"}})
	return storage.Message{
		AccountID:  accountID,
		Sender:     "sender@example.com",
		Recipients: recipients,
		Subject:    sql.NullString{String: "Test Subject", Valid: true},
		Headers:    headers,
	}
}

// newHandler creates a Handler for tests. If store is nil, storage fetch
// is not available (inline-body only).
func newHandler(t *testing.T, mq *mockQuerier, store msgstore.MessageStore) *Handler {
	t.Helper()
	log := zerolog.Nop()
	httpClient := provider.NewHTTPClient(0)
	resolver := provider.NewResolver(mq, httpClient, log)
	return NewHandler(resolver, mq, store, log)
}

// ---------------------------------------------------------------------------
// Tests: Inline body (legacy format / backward compatibility)
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_InlineBody_Success(t *testing.T) {
	accountID := uuid.New()
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return newTestDBMessage(accountID), nil
		},
	}
	h := newHandler(t, mq, nil)

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
	// Verify new delivery log fields.
	if !mq.createLogParams.AccountID.Valid {
		t.Error("expected AccountID to be set in delivery log")
	}
	if !mq.createLogParams.DurationMs.Valid {
		t.Error("expected DurationMs to be set in delivery log")
	}
	if mq.createLogParams.DurationMs.Int32 < 0 {
		t.Errorf("expected DurationMs >= 0, got %d", mq.createLogParams.DurationMs.Int32)
	}
	if mq.createLogParams.AttemptNumber != 1 {
		t.Errorf("expected AttemptNumber 1, got %d", mq.createLogParams.AttemptNumber)
	}
}

// ---------------------------------------------------------------------------
// Tests: Storage fetch (new format)
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_StorageFetch_Success(t *testing.T) {
	// Reduce backoff to make test faster.
	origBackoff := storageRetryBackoff
	storageRetryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { storageRetryBackoff = origBackoff }()

	accountID := uuid.New()
	msgID := uuid.New()
	bodyContent := []byte("Hello from storage")

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return newTestDBMessage(accountID), nil
		},
	}
	store := &mockMessageStore{
		data: map[string][]byte{msgID.String(): bodyContent},
	}
	h := newHandler(t, mq, store)

	// ID-only message (no inline body).
	msg := &queue.Message{
		ID:        msgID.String(),
		AccountID: accountID.String(),
		TenantID:  "tenant-1",
	}

	err := h.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(mq.statuses) < 2 {
		t.Fatalf("expected at least 2 status updates, got %d", len(mq.statuses))
	}
	if mq.statuses[0] != storage.MessageStatusProcessing {
		t.Errorf("expected first status processing, got %s", mq.statuses[0])
	}
	if mq.statuses[1] != storage.MessageStatusDelivered {
		t.Errorf("expected second status delivered, got %s", mq.statuses[1])
	}

	if !mq.createLogCalled {
		t.Error("expected delivery log to be created")
	}
	// Verify new delivery log fields.
	if !mq.createLogParams.AccountID.Valid {
		t.Error("expected AccountID to be set in delivery log")
	}
	if mq.createLogParams.AccountID.Bytes != accountID {
		t.Errorf("expected AccountID %v, got %v", accountID, mq.createLogParams.AccountID.Bytes)
	}
	if !mq.createLogParams.DurationMs.Valid {
		t.Error("expected DurationMs to be set in delivery log")
	}
	if mq.createLogParams.AttemptNumber != 1 {
		t.Errorf("expected AttemptNumber 1, got %d", mq.createLogParams.AttemptNumber)
	}
}

// ---------------------------------------------------------------------------
// Tests: Storage read failure with retry exhaustion -> storage_error
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_StorageRetryExhaustion(t *testing.T) {
	// Reduce backoff to make test faster.
	origBackoff := storageRetryBackoff
	storageRetryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { storageRetryBackoff = origBackoff }()

	accountID := uuid.New()
	msgID := uuid.New()
	storageErr := errors.New("disk I/O error")

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return newTestDBMessage(accountID), nil
		},
	}
	store := &mockMessageStore{
		getFn: func(_ context.Context, _ string) ([]byte, error) {
			return nil, storageErr
		},
	}
	h := newHandler(t, mq, store)

	msg := &queue.Message{
		ID:        msgID.String(),
		AccountID: accountID.String(),
		TenantID:  "tenant-1",
	}

	err := h.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when storage retries are exhausted")
	}

	// Verify Get was called exactly 3 times (the number of retry attempts).
	calls := atomic.LoadInt32(&store.getCalls)
	if calls != 3 {
		t.Errorf("expected 3 storage Get calls, got %d", calls)
	}

	// Status should be: processing, storage_error, failed.
	foundStorageError := false
	for _, s := range mq.statuses {
		if s == storage.MessageStatusStorageError {
			foundStorageError = true
			break
		}
	}
	if !foundStorageError {
		t.Errorf("expected storage_error status, got statuses: %v", mq.statuses)
	}

	if mq.createLogStatus != string(storage.MessageStatusFailed) {
		t.Errorf("expected delivery log status failed, got %s", mq.createLogStatus)
	}
}

// ---------------------------------------------------------------------------
// Tests: Orphaned message_id -> return nil (ack), no delivery
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_OrphanedMessageID(t *testing.T) {
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return storage.Message{}, pgx.ErrNoRows
		},
	}
	h := newHandler(t, mq, nil)

	msg := &queue.Message{
		ID:       msgID.String(),
		TenantID: "tenant-1",
	}

	err := h.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected nil error for orphaned message, got %v", err)
	}

	// Should not have recorded a failure (no delivery log).
	if mq.createLogCalled {
		t.Error("expected no delivery log for orphaned message")
	}

	// Status updates: processing only (set before GetMessageByID).
	if len(mq.statuses) != 1 {
		t.Errorf("expected 1 status update (processing), got %d: %v", len(mq.statuses), mq.statuses)
	}
}

// ---------------------------------------------------------------------------
// Tests: Provider resolution failure
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_SendFail(t *testing.T) {
	accountID := uuid.New()
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return newTestDBMessage(accountID), nil
		},
		listProvidersFn: func(_ context.Context, _ uuid.UUID) ([]storage.EspProvider, error) {
			return nil, errors.New("database error")
		},
	}
	h := newHandler(t, mq, nil)

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
	// Verify AccountID is set in failure delivery log (we have dbMsg at this point).
	if !mq.createLogParams.AccountID.Valid {
		t.Error("expected AccountID to be set in failure delivery log")
	}
}

// ---------------------------------------------------------------------------
// Tests: Invalid message ID
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_InvalidMessageID(t *testing.T) {
	mq := &mockQuerier{}
	h := newHandler(t, mq, nil)

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

// ---------------------------------------------------------------------------
// Tests: GetMessageByID error (non-ErrNoRows)
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_GetMessageError(t *testing.T) {
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return storage.Message{}, errors.New("connection refused")
		},
	}
	h := newHandler(t, mq, nil)

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
	// AccountID should NOT be set since we failed before getting dbMsg.
	if mq.createLogParams.AccountID.Valid {
		t.Error("expected AccountID to be unset when message lookup fails")
	}
}

// ---------------------------------------------------------------------------
// Tests: Context cancellation during storage retry
// ---------------------------------------------------------------------------

func TestHandler_HandleMessage_StorageRetryContextCancelled(t *testing.T) {
	origBackoff := storageRetryBackoff
	storageRetryBackoff = []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond}
	defer func() { storageRetryBackoff = origBackoff }()

	accountID := uuid.New()
	msgID := uuid.New()

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return newTestDBMessage(accountID), nil
		},
	}
	store := &mockMessageStore{
		getFn: func(_ context.Context, _ string) ([]byte, error) {
			return nil, errors.New("temporary error")
		},
	}
	h := newHandler(t, mq, store)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay to interrupt the retry loop.
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	msg := &queue.Message{
		ID:        msgID.String(),
		AccountID: accountID.String(),
		TenantID:  "tenant-1",
	}

	err := h.HandleMessage(ctx, msg)
	if err == nil {
		t.Fatal("expected error when context is cancelled during retry")
	}
}

// ---------------------------------------------------------------------------
// Tests: Helper functions
// ---------------------------------------------------------------------------

func TestParseRecipients(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []string
	}{
		{"valid", mustJSON([]string{"a@b.com", "c@d.com"}), []string{"a@b.com", "c@d.com"}},
		{"empty", nil, nil},
		{"invalid json", []byte("not json"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRecipients(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d recipients, got %d", len(tt.expected), len(result))
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected map[string]string
	}{
		{
			"valid",
			mustJSON(map[string][]string{"X-Test": {"v1", "v2"}, "X-Other": {"v3"}}),
			map[string]string{"X-Test": "v1", "X-Other": "v3"},
		},
		{"empty", nil, nil},
		{"invalid json", []byte("not json"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseHeaders(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}

func TestNullStringValue(t *testing.T) {
	if v := nullStringValue(sql.NullString{String: "hello", Valid: true}); v != "hello" {
		t.Errorf("expected 'hello', got %q", v)
	}
	if v := nullStringValue(sql.NullString{}); v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
}

func mustJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

// ---------------------------------------------------------------------------
// Tests: MIME parsing integration (multipart message with HTML + attachments)
// ---------------------------------------------------------------------------

// mockCaptureProvider captures the provider.Message it receives via Send.
type mockCaptureProvider struct {
	captured *provider.Message
}

func (m *mockCaptureProvider) GetName() string { return "capture" }
func (m *mockCaptureProvider) Send(_ context.Context, msg *provider.Message) (*provider.DeliveryResult, error) {
	m.captured = msg
	return &provider.DeliveryResult{
		ProviderMessageID: "capture-" + msg.ID,
		Status:            provider.StatusSent,
	}, nil
}
func (m *mockCaptureProvider) HealthCheck(_ context.Context) error { return nil }

// mockCaptureResolver returns a fixed provider for any account.
type mockCaptureResolver struct {
	provider provider.Provider
}

func (r *mockCaptureResolver) Resolve(_ context.Context, _ uuid.UUID) (provider.Provider, error) {
	return r.provider, nil
}

func TestHandler_HandleMessage_MIMEParsing(t *testing.T) {
	// Reduce backoff for fast tests.
	origBackoff := storageRetryBackoff
	storageRetryBackoff = []time.Duration{time.Millisecond}
	defer func() { storageRetryBackoff = origBackoff }()

	accountID := uuid.New()
	msgID := uuid.New()

	// Build a real multipart MIME message with HTML body and an attachment.
	boundary := "----TestBoundary123"
	mimeMsg := "MIME-Version: 1.0\r\n" +
		"Subject: MIME Parsed Subject\r\n" +
		"Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n" +
		"\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Hello plain text\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<h1>Hello HTML</h1>\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Type: application/pdf; name=\"report.pdf\"\r\n" +
		"Content-Disposition: attachment; filename=\"report.pdf\"\r\n" +
		"\r\n" +
		"PDF-CONTENT-HERE\r\n" +
		"--" + boundary + "--\r\n"

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return newTestDBMessage(accountID), nil
		},
	}
	store := &mockMessageStore{
		data: map[string][]byte{msgID.String(): []byte(mimeMsg)},
	}

	capture := &mockCaptureProvider{}
	log := zerolog.Nop()
	resolver := &mockCaptureResolver{provider: capture}
	h := &Handler{
		resolver: resolver,
		queries:  mq,
		store:    store,
		log:      log,
	}

	msg := &queue.Message{
		ID:        msgID.String(),
		AccountID: accountID.String(),
		TenantID:  "tenant-1",
	}

	err := h.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify delivery succeeded.
	if len(mq.statuses) < 2 {
		t.Fatalf("expected at least 2 status updates, got %d", len(mq.statuses))
	}
	if mq.statuses[1] != storage.MessageStatusDelivered {
		t.Errorf("expected delivered status, got %s", mq.statuses[1])
	}

	// Verify the provider received MIME-parsed fields.
	if capture.captured == nil {
		t.Fatal("expected provider to receive a message")
	}
	pm := capture.captured

	// MIME-parsed subject should override the DB subject.
	if pm.Subject != "MIME Parsed Subject" {
		t.Errorf("expected subject 'MIME Parsed Subject', got %q", pm.Subject)
	}
	if pm.TextBody != "Hello plain text" {
		t.Errorf("expected TextBody 'Hello plain text', got %q", pm.TextBody)
	}
	if pm.HTMLBody != "<h1>Hello HTML</h1>" {
		t.Errorf("expected HTMLBody '<h1>Hello HTML</h1>', got %q", pm.HTMLBody)
	}
	if len(pm.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(pm.Attachments))
	}
	if pm.Attachments[0].Filename != "report.pdf" {
		t.Errorf("expected attachment filename 'report.pdf', got %q", pm.Attachments[0].Filename)
	}
	if pm.Attachments[0].ContentType != "application/pdf" {
		t.Errorf("expected attachment content type 'application/pdf', got %q", pm.Attachments[0].ContentType)
	}
}

func TestHandler_HandleMessage_MIMEParseFallback(t *testing.T) {
	// When the body is not valid MIME, the handler should fall back
	// to using the raw body as TextBody.
	origBackoff := storageRetryBackoff
	storageRetryBackoff = []time.Duration{time.Millisecond}
	defer func() { storageRetryBackoff = origBackoff }()

	accountID := uuid.New()
	msgID := uuid.New()
	plainBody := []byte("Just a plain string, not MIME at all")

	mq := &mockQuerier{
		getMessageFn: func(_ context.Context, _ uuid.UUID) (storage.Message, error) {
			return newTestDBMessage(accountID), nil
		},
	}
	store := &mockMessageStore{
		data: map[string][]byte{msgID.String(): plainBody},
	}

	capture := &mockCaptureProvider{}
	log := zerolog.Nop()
	resolver := &mockCaptureResolver{provider: capture}
	h := &Handler{
		resolver: resolver,
		queries:  mq,
		store:    store,
		log:      log,
	}

	msg := &queue.Message{
		ID:        msgID.String(),
		AccountID: accountID.String(),
		TenantID:  "tenant-1",
	}

	err := h.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if capture.captured == nil {
		t.Fatal("expected provider to receive a message")
	}
	pm := capture.captured

	// Fallback: raw body should be used as TextBody.
	if pm.TextBody != string(plainBody) {
		t.Errorf("expected TextBody to be raw body, got %q", pm.TextBody)
	}
	if pm.HTMLBody != "" {
		t.Errorf("expected no HTMLBody, got %q", pm.HTMLBody)
	}
	if len(pm.Attachments) != 0 {
		t.Errorf("expected no attachments, got %d", len(pm.Attachments))
	}
}
