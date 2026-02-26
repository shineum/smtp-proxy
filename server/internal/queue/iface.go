package queue

import "context"

// Enqueuer publishes messages to the queue.
type Enqueuer interface {
	Enqueue(ctx context.Context, msg *Message) (string, error)
}

// Dequeuer consumes messages from the queue.
// Start begins consuming in background goroutines.
// Stop gracefully shuts down consumers.
type Dequeuer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// DeadLetterQueue manages failed messages.
type DeadLetterQueue interface {
	MoveToDLQ(ctx context.Context, msg *Message, reason string) error
	Reprocess(ctx context.Context, tenantID string, messageIDs []string) (int, error)
}

// MessageHandler processes a single queue message. Implementations define
// the actual delivery logic (e.g., sending via ESP provider).
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *Message) error
}
