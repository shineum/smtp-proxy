package delivery

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/queue"
)

// mockEnqueuer implements queue.Enqueuer for testing.
type mockEnqueuer struct {
	enqueueFn func(ctx context.Context, msg *queue.Message) (string, error)
}

func (m *mockEnqueuer) Enqueue(ctx context.Context, msg *queue.Message) (string, error) {
	if m.enqueueFn != nil {
		return m.enqueueFn(ctx, msg)
	}
	return "mock-entry-id", nil
}

// Since AsyncService depends on a queue.Enqueuer interface,
// we test the request conversion logic and verify the interface is satisfied.

func TestAsyncService_ImplementsInterface(t *testing.T) {
	// Verify that AsyncService satisfies the Service interface at compile time.
	var _ Service = (*AsyncService)(nil)
}

func TestRequest_Fields(t *testing.T) {
	msgID := uuid.New()
	accID := uuid.New()

	req := &Request{
		MessageID: msgID,
		AccountID: accID,
		TenantID:  "tenant-1",
	}

	if req.MessageID != msgID {
		t.Errorf("expected MessageID=%s, got %s", msgID, req.MessageID)
	}
	if req.AccountID != accID {
		t.Errorf("expected AccountID=%s, got %s", accID, req.AccountID)
	}
	if req.TenantID != "tenant-1" {
		t.Errorf("expected TenantID=tenant-1, got %s", req.TenantID)
	}
}

func TestNewAsyncService(t *testing.T) {
	log := zerolog.Nop()

	// NewAsyncService accepts any queue.Enqueuer.
	mock := &mockEnqueuer{}
	svc := NewAsyncService(mock, log)
	if svc == nil {
		t.Fatal("expected non-nil AsyncService")
	}
}

func TestAsyncService_DeliverMessage(t *testing.T) {
	log := zerolog.Nop()

	var capturedMsg *queue.Message
	mock := &mockEnqueuer{
		enqueueFn: func(ctx context.Context, msg *queue.Message) (string, error) {
			capturedMsg = msg
			return "entry-123", nil
		},
	}

	svc := NewAsyncService(mock, log)

	req := &Request{
		MessageID: uuid.New(),
		AccountID: uuid.New(),
		TenantID:  "test-tenant",
	}

	err := svc.DeliverMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("DeliverMessage() error: %v", err)
	}

	if capturedMsg == nil {
		t.Fatal("expected Enqueue to be called")
	}
	if capturedMsg.ID != req.MessageID.String() {
		t.Errorf("message ID = %q, want %q", capturedMsg.ID, req.MessageID.String())
	}
	if capturedMsg.AccountID != req.AccountID.String() {
		t.Errorf("account ID = %q, want %q", capturedMsg.AccountID, req.AccountID.String())
	}
	if capturedMsg.TenantID != req.TenantID {
		t.Errorf("tenant ID = %q, want %q", capturedMsg.TenantID, req.TenantID)
	}
}
