package queue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// mockSQSClient implements sqsAPI for testing.
type mockSQSClient struct {
	mu             sync.Mutex
	messages       []sqsReceivedMessage // messages to return from ReceiveMessage
	sent           []sqsSendInput       // track sent messages
	deleted        []sqsDeleteInput     // track deleted messages
	sendErr        error
	receiveErr     error
	deleteErr      error
	receiveCount   int  // how many times ReceiveMessage has been called
	receiveOnce    bool // if true, return messages only on first call then empty
	receiveCalled  chan struct{}
}

func newMockSQSClient() *mockSQSClient {
	return &mockSQSClient{
		receiveCalled: make(chan struct{}, 100),
	}
}

func (m *mockSQSClient) SendMessage(_ context.Context, input *sqsSendInput) (*sqsSendOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.sent = append(m.sent, *input)
	return &sqsSendOutput{MessageID: "mock-msg-id"}, nil
}

func (m *mockSQSClient) ReceiveMessage(_ context.Context, _ *sqsReceiveInput) (*sqsReceiveOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receiveCount++

	select {
	case m.receiveCalled <- struct{}{}:
	default:
	}

	if m.receiveErr != nil {
		return nil, m.receiveErr
	}
	if m.receiveOnce && m.receiveCount > 1 {
		return &sqsReceiveOutput{}, nil
	}
	msgs := make([]sqsReceivedMessage, len(m.messages))
	copy(msgs, m.messages)
	if m.receiveOnce {
		m.messages = nil
	}
	return &sqsReceiveOutput{Messages: msgs}, nil
}

func (m *mockSQSClient) DeleteMessage(_ context.Context, input *sqsDeleteInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = append(m.deleted, *input)
	return nil
}

func (m *mockSQSClient) ChangeMessageVisibility(_ context.Context, _ *sqsChangeVisibilityInput) error {
	return nil
}

func (m *mockSQSClient) getSent() []sqsSendInput {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]sqsSendInput, len(m.sent))
	copy(out, m.sent)
	return out
}

func (m *mockSQSClient) getDeleted() []sqsDeleteInput {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]sqsDeleteInput, len(m.deleted))
	copy(out, m.deleted)
	return out
}

// mockHandler implements MessageHandler for testing.
type mockHandler struct {
	err error
}

func (h *mockHandler) HandleMessage(_ context.Context, _ *Message) error {
	return h.err
}

// testLogger returns a zerolog.Logger that discards all output.
func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

// --- Enqueuer Tests ---

func TestSQSEnqueuer_Enqueue(t *testing.T) {
	t.Parallel()

	mock := newMockSQSClient()
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.us-east-1.amazonaws.com/123/test-queue", testLogger())

	msg := &Message{
		ID:       "msg-001",
		TenantID: "tenant-1",
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		Body:     []byte("Hello"),
	}

	msgID, err := enqueuer.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgID != "mock-msg-id" {
		t.Errorf("expected message ID %q, got %q", "mock-msg-id", msgID)
	}

	sent := mock.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].QueueURL != "https://sqs.us-east-1.amazonaws.com/123/test-queue" {
		t.Errorf("unexpected queue URL: %s", sent[0].QueueURL)
	}

	// Verify the message body is valid JSON containing our message.
	var decoded Message
	if err := json.Unmarshal([]byte(sent[0].MessageBody), &decoded); err != nil {
		t.Fatalf("failed to unmarshal sent body: %v", err)
	}
	if decoded.ID != "msg-001" {
		t.Errorf("expected message ID %q in body, got %q", "msg-001", decoded.ID)
	}
	if decoded.TenantID != "tenant-1" {
		t.Errorf("expected tenant ID %q in body, got %q", "tenant-1", decoded.TenantID)
	}
}

func TestSQSEnqueuer_Enqueue_Error(t *testing.T) {
	t.Parallel()

	mock := newMockSQSClient()
	mock.sendErr = errors.New("sqs unavailable")
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())

	msg := &Message{ID: "msg-002", TenantID: "tenant-1"}
	_, err := enqueuer.Enqueue(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, mock.sendErr) {
		// The error is wrapped, so check the message.
		if got := err.Error(); got != "sqs send message: sqs unavailable" {
			t.Errorf("unexpected error message: %s", got)
		}
	}
}

func TestSQSEnqueuer_EnqueueWithDelay(t *testing.T) {
	t.Parallel()

	mock := newMockSQSClient()
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())

	msg := &Message{ID: "msg-003", TenantID: "tenant-1"}

	_, err := enqueuer.EnqueueWithDelay(context.Background(), msg, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := mock.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].DelaySeconds != 60 {
		t.Errorf("expected delay 60, got %d", sent[0].DelaySeconds)
	}
}

func TestSQSEnqueuer_EnqueueWithDelay_Capped(t *testing.T) {
	t.Parallel()

	mock := newMockSQSClient()
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())

	msg := &Message{ID: "msg-004", TenantID: "tenant-1"}

	// Delay exceeding 900 should be capped to 900.
	_, err := enqueuer.EnqueueWithDelay(context.Background(), msg, 1200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := mock.getSent()
	if sent[0].DelaySeconds != 900 {
		t.Errorf("expected delay capped to 900, got %d", sent[0].DelaySeconds)
	}
}

// --- Dequeuer Tests ---

func TestSQSDequeuer_StartStop(t *testing.T) {
	t.Parallel()

	mock := newMockSQSClient()
	// Return empty messages so workers just loop on long-poll.
	handler := &mockHandler{}
	retry := NewRetryStrategy(3)
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())
	cfg := Config{
		WorkerCount:     2,
		SQSWaitTime:     1,
		SQSVisTimeout:   30,
		ProcessTimeout:  5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	dequeuer := NewSQSDequeuer(mock, "https://sqs.example.com/queue", handler, nil, retry, enqueuer, cfg, testLogger())

	ctx := context.Background()
	if err := dequeuer.Start(ctx); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	// Wait for at least one receive call to confirm workers are running.
	select {
	case <-mock.receiveCalled:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for workers to start polling")
	}

	if err := dequeuer.Stop(ctx); err != nil {
		t.Fatalf("unexpected stop error: %v", err)
	}
}

func TestSQSDequeuer_ProcessMessage(t *testing.T) {
	t.Parallel()

	msg := &Message{
		ID:       "msg-010",
		TenantID: "tenant-1",
		From:     "a@b.com",
		To:       []string{"c@d.com"},
		Subject:  "Test",
	}
	body, _ := json.Marshal(msg)

	mock := newMockSQSClient()
	mock.messages = []sqsReceivedMessage{
		{MessageID: "sqs-1", ReceiptHandle: "receipt-1", Body: string(body)},
	}
	mock.receiveOnce = true

	handler := &mockHandler{} // success
	retry := NewRetryStrategy(3)
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())
	cfg := Config{
		WorkerCount:     1,
		SQSWaitTime:     1,
		SQSVisTimeout:   30,
		ProcessTimeout:  5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	dequeuer := NewSQSDequeuer(mock, "https://sqs.example.com/queue", handler, nil, retry, enqueuer, cfg, testLogger())

	ctx := context.Background()
	if err := dequeuer.Start(ctx); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	// Wait for the message to be processed and deleted.
	deadline := time.After(5 * time.Second)
	for {
		deleted := mock.getDeleted()
		if len(deleted) > 0 {
			if deleted[0].ReceiptHandle != "receipt-1" {
				t.Errorf("expected receipt handle %q, got %q", "receipt-1", deleted[0].ReceiptHandle)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for message deletion")
		case <-time.After(50 * time.Millisecond):
		}
	}

	if err := dequeuer.Stop(ctx); err != nil {
		t.Fatalf("unexpected stop error: %v", err)
	}

	// No messages should have been re-enqueued (handler succeeded).
	sent := mock.getSent()
	if len(sent) != 0 {
		t.Errorf("expected 0 sent messages (no retry), got %d", len(sent))
	}
}

func TestSQSDequeuer_ProcessMessage_Retry(t *testing.T) {
	t.Parallel()

	msg := &Message{
		ID:         "msg-020",
		TenantID:   "tenant-1",
		RetryCount: 0,
	}
	body, _ := json.Marshal(msg)

	mock := newMockSQSClient()
	mock.messages = []sqsReceivedMessage{
		{MessageID: "sqs-2", ReceiptHandle: "receipt-2", Body: string(body)},
	}
	mock.receiveOnce = true

	handler := &mockHandler{err: errors.New("temporary failure")}
	retry := NewRetryStrategy(3) // max 3, so retry_count=1 should retry
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())
	dlq := NewSQSDLQ(mock, "https://sqs.example.com/dlq", "https://sqs.example.com/queue", enqueuer, testLogger())
	cfg := Config{
		WorkerCount:     1,
		SQSWaitTime:     1,
		SQSVisTimeout:   30,
		ProcessTimeout:  5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	dequeuer := NewSQSDequeuer(mock, "https://sqs.example.com/queue", handler, dlq, retry, enqueuer, cfg, testLogger())

	ctx := context.Background()
	if err := dequeuer.Start(ctx); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	// Wait for re-enqueue (sent) and deletion.
	deadline := time.After(5 * time.Second)
	for {
		sent := mock.getSent()
		deleted := mock.getDeleted()
		// Expect: 1 sent (re-enqueue with delay) and 1 deleted (original).
		if len(sent) >= 1 && len(deleted) >= 1 {
			// Verify the re-enqueued message has delay > 0.
			if sent[0].DelaySeconds <= 0 {
				t.Errorf("expected positive delay for retry, got %d", sent[0].DelaySeconds)
			}
			// Verify the re-enqueued message has incremented retry count.
			var retriedMsg Message
			if err := json.Unmarshal([]byte(sent[0].MessageBody), &retriedMsg); err != nil {
				t.Fatalf("failed to unmarshal retried message: %v", err)
			}
			if retriedMsg.RetryCount != 1 {
				t.Errorf("expected retry count 1, got %d", retriedMsg.RetryCount)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out: sent=%d deleted=%d", len(sent), len(deleted))
		case <-time.After(50 * time.Millisecond):
		}
	}

	if err := dequeuer.Stop(ctx); err != nil {
		t.Fatalf("unexpected stop error: %v", err)
	}
}

func TestSQSDequeuer_ProcessMessage_DLQ(t *testing.T) {
	t.Parallel()

	msg := &Message{
		ID:         "msg-030",
		TenantID:   "tenant-1",
		RetryCount: 4, // already at 4, max is 5 -> after increment becomes 5 -> no more retries
	}
	body, _ := json.Marshal(msg)

	mock := newMockSQSClient()
	mock.messages = []sqsReceivedMessage{
		{MessageID: "sqs-3", ReceiptHandle: "receipt-3", Body: string(body)},
	}
	mock.receiveOnce = true

	handler := &mockHandler{err: errors.New("permanent failure")}
	retry := NewRetryStrategy(5) // max 5, msg already at 4 -> 5 after increment -> exhausted
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())
	dlq := NewSQSDLQ(mock, "https://sqs.example.com/dlq", "https://sqs.example.com/queue", enqueuer, testLogger())
	cfg := Config{
		WorkerCount:     1,
		SQSWaitTime:     1,
		SQSVisTimeout:   30,
		ProcessTimeout:  5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	dequeuer := NewSQSDequeuer(mock, "https://sqs.example.com/queue", handler, dlq, retry, enqueuer, cfg, testLogger())

	ctx := context.Background()
	if err := dequeuer.Start(ctx); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	// Wait for DLQ send and original deletion.
	deadline := time.After(5 * time.Second)
	for {
		sent := mock.getSent()
		deleted := mock.getDeleted()
		// Expect: 1 sent (to DLQ URL) and 1 deleted (original).
		if len(sent) >= 1 && len(deleted) >= 1 {
			// Verify the sent message went to the DLQ URL.
			if sent[0].QueueURL != "https://sqs.example.com/dlq" {
				t.Errorf("expected DLQ URL, got %s", sent[0].QueueURL)
			}
			// Verify it is a DLQMessage envelope.
			var dlqMsg DLQMessage
			if err := json.Unmarshal([]byte(sent[0].MessageBody), &dlqMsg); err != nil {
				t.Fatalf("failed to unmarshal DLQ message: %v", err)
			}
			if dlqMsg.OriginalMessage.ID != "msg-030" {
				t.Errorf("expected original message ID %q, got %q", "msg-030", dlqMsg.OriginalMessage.ID)
			}
			if dlqMsg.FailureReason != "permanent failure" {
				t.Errorf("expected failure reason %q, got %q", "permanent failure", dlqMsg.FailureReason)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out: sent=%d deleted=%d", len(sent), len(deleted))
		case <-time.After(50 * time.Millisecond):
		}
	}

	if err := dequeuer.Stop(ctx); err != nil {
		t.Fatalf("unexpected stop error: %v", err)
	}
}

// --- DLQ Tests ---

func TestSQSDLQ_MoveToDLQ(t *testing.T) {
	t.Parallel()

	mock := newMockSQSClient()
	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())
	dlq := NewSQSDLQ(mock, "https://sqs.example.com/dlq", "https://sqs.example.com/queue", enqueuer, testLogger())

	msg := &Message{
		ID:         "msg-040",
		TenantID:   "tenant-1",
		RetryCount: 5,
	}

	err := dlq.MoveToDLQ(context.Background(), msg, "max retries exceeded")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := mock.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].QueueURL != "https://sqs.example.com/dlq" {
		t.Errorf("expected DLQ URL, got %s", sent[0].QueueURL)
	}

	var dlqMsg DLQMessage
	if err := json.Unmarshal([]byte(sent[0].MessageBody), &dlqMsg); err != nil {
		t.Fatalf("failed to unmarshal DLQ message: %v", err)
	}
	if dlqMsg.OriginalMessage.ID != "msg-040" {
		t.Errorf("expected original message ID %q, got %q", "msg-040", dlqMsg.OriginalMessage.ID)
	}
	if dlqMsg.FailureReason != "max retries exceeded" {
		t.Errorf("expected failure reason %q, got %q", "max retries exceeded", dlqMsg.FailureReason)
	}
	if dlqMsg.FinalError != "max retries exceeded" {
		t.Errorf("expected final error %q, got %q", "max retries exceeded", dlqMsg.FinalError)
	}
}

func TestSQSDLQ_Reprocess(t *testing.T) {
	t.Parallel()

	originalMsg := &Message{
		ID:         "msg-050",
		TenantID:   "tenant-1",
		RetryCount: 5,
		From:       "a@b.com",
		To:         []string{"c@d.com"},
	}
	dlqMsg := DLQMessage{
		OriginalMessage: originalMsg,
		FailureReason:   "some error",
		FinalError:      "some error",
		MovedAt:         time.Now(),
	}
	dlqBody, _ := json.Marshal(dlqMsg)

	mock := newMockSQSClient()
	mock.messages = []sqsReceivedMessage{
		{MessageID: "dlq-sqs-1", ReceiptHandle: "dlq-receipt-1", Body: string(dlqBody)},
	}

	enqueuer := NewSQSEnqueuer(mock, "https://sqs.example.com/queue", testLogger())
	dlq := NewSQSDLQ(mock, "https://sqs.example.com/dlq", "https://sqs.example.com/queue", enqueuer, testLogger())

	count, err := dlq.Reprocess(context.Background(), "tenant-1", []string{"dlq-sqs-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 reprocessed, got %d", count)
	}

	// Verify: 1 message sent to primary queue (re-enqueue) with reset retry count.
	sent := mock.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].QueueURL != "https://sqs.example.com/queue" {
		t.Errorf("expected primary queue URL, got %s", sent[0].QueueURL)
	}

	var requeued Message
	if err := json.Unmarshal([]byte(sent[0].MessageBody), &requeued); err != nil {
		t.Fatalf("failed to unmarshal requeued message: %v", err)
	}
	if requeued.RetryCount != 0 {
		t.Errorf("expected retry count reset to 0, got %d", requeued.RetryCount)
	}
	if requeued.ID != "msg-050" {
		t.Errorf("expected message ID %q, got %q", "msg-050", requeued.ID)
	}

	// Verify: 1 message deleted from DLQ.
	deleted := mock.getDeleted()
	if len(deleted) != 1 {
		t.Fatalf("expected 1 deleted message, got %d", len(deleted))
	}
	if deleted[0].QueueURL != "https://sqs.example.com/dlq" {
		t.Errorf("expected DLQ URL for deletion, got %s", deleted[0].QueueURL)
	}
	if deleted[0].ReceiptHandle != "dlq-receipt-1" {
		t.Errorf("expected receipt handle %q, got %q", "dlq-receipt-1", deleted[0].ReceiptHandle)
	}
}
