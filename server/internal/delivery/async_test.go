package delivery

import (
	"context"
	"fmt"
	"testing"

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
	var msgID int64 = 1001
	var userID int32 = 10
	var groupID int32 = 20

	req := &Request{
		MessageID: msgID,
		UserID:    userID,
		GroupID:   groupID,
	}

	if req.MessageID != msgID {
		t.Errorf("expected MessageID=%d, got %d", msgID, req.MessageID)
	}
	if req.UserID != userID {
		t.Errorf("expected UserID=%d, got %d", userID, req.UserID)
	}
	if req.GroupID != groupID {
		t.Errorf("expected GroupID=%d, got %d", groupID, req.GroupID)
	}
}

func TestNewAsyncService(t *testing.T) {
	log := zerolog.Nop()

	// NewAsyncService accepts any queue.Enqueuer.
	mock := &mockEnqueuer{}
	svc := NewAsyncService(mock, "smtp-proxy", log)
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

	svc := NewAsyncService(mock, "smtp-proxy", log)

	req := &Request{
		MessageID: 1001,
		UserID:    10,
		GroupID:   20,
	}

	err := svc.DeliverMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("DeliverMessage() error: %v", err)
	}

	if capturedMsg == nil {
		t.Fatal("expected Enqueue to be called")
	}
	expectedID := fmt.Sprintf("%d", req.MessageID)
	if capturedMsg.ID != expectedID {
		t.Errorf("message ID = %q, want %q", capturedMsg.ID, expectedID)
	}
	expectedAccountID := fmt.Sprintf("%d", req.GroupID)
	if capturedMsg.AccountID != expectedAccountID {
		t.Errorf("account ID (group) = %q, want %q", capturedMsg.AccountID, expectedAccountID)
	}
	if capturedMsg.TenantID != "smtp-proxy" {
		t.Errorf("tenant ID (stream) = %q, want %q", capturedMsg.TenantID, "smtp-proxy")
	}
}
