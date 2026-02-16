---
id: SPEC-QUEUE-001
version: "1.0.0"
status: approved
created: "2026-02-15"
updated: "2026-02-15"
author: sungwon
priority: P0
---

# HISTORY

**2026-02-15 - v1.0.0 - Initial Specification**
- Created EARS-formatted requirements for asynchronous message processing
- Defined ESP provider abstraction with SendGrid, AWS SES, and Mailgun support
- Specified retry strategy with exponential backoff and DLQ routing
- Established delivery tracking and webhook integration patterns

---

# SPEC-QUEUE-001: Asynchronous Message Processing and ESP Provider Integration

## Overview

This specification defines the asynchronous message queue system for processing SMTP messages through Email Service Provider (ESP) integrations. The system decouples SMTP message reception from ESP delivery, providing resilience, retry logic, and delivery status tracking.

## Environment

- **Runtime**: Go 1.21+
- **Queue Backend**: Redis Streams with consumer groups
- **Database**: PostgreSQL 14+ for delivery tracking
- **ESP Providers**: SendGrid, AWS SES, Mailgun
- **Queue Library**: go-redis v9
- **ESP SDKs**: SendGrid Go SDK, AWS SDK Go v2 SES, Mailgun Go SDK

## Assumptions

1. Redis Streams is available and configured for persistent message storage
2. PostgreSQL database schema supports delivery_logs table with status history
3. ESP provider API credentials are available via environment variables
4. Worker pool runs as separate process from SMTP server
5. Webhook endpoints are accessible for ESP delivery notifications
6. Message size limit is enforced at SMTP layer (10MB maximum)
7. Tenant routing rules are configured in database or configuration
8. DLQ messages require manual intervention for reprocessing

## Requirements

### Ubiquitous Requirements (Always Active)

**REQ-QUEUE-U001**: Queue worker SHALL always process messages in FIFO order per tenant
- WHY: Order preservation ensures sequential processing for tenant-specific messages
- IMPACT: Out-of-order processing could violate tenant expectations

**REQ-QUEUE-U002**: System SHALL always track delivery status transitions: pending → queued → processing → sent|failed|bounced
- WHY: Status tracking provides delivery visibility and audit trail
- IMPACT: Missing status updates prevent delivery confirmation

**REQ-QUEUE-U003**: ESP provider abstraction SHALL always normalize response formats across all providers (SendGrid, AWS SES, Mailgun)
- WHY: Normalized responses enable consistent error handling and status mapping
- IMPACT: Provider-specific responses require per-provider logic duplication

**REQ-QUEUE-U004**: System SHALL always persist message metadata before acknowledging SMTP DATA command
- WHY: Persistence prevents message loss during SMTP session failures
- IMPACT: Message loss violates email delivery reliability guarantees

**REQ-QUEUE-U005**: System SHALL always log provider API responses with correlation IDs for debugging
- WHY: Correlation IDs enable end-to-end message tracing
- IMPACT: Missing correlation IDs prevent root cause analysis

### Event-Driven Requirements (Trigger-Response)

**REQ-QUEUE-E001**: WHEN message enters queue via SMTP DATA, THEN system SHALL assign unique message ID and set status to "queued"
- WHY: Unique IDs enable message tracking and idempotency
- IMPACT: Duplicate message IDs cause processing conflicts

**REQ-QUEUE-E002**: WHEN queue worker picks message, THEN system SHALL resolve ESP provider via routing rules engine (tenant-specific or default)
- WHY: Routing rules enable tenant-specific ESP selection and failover
- IMPACT: Incorrect routing sends messages to wrong ESP

**REQ-QUEUE-E003**: WHEN ESP returns 2xx success, THEN system SHALL mark message "sent" with provider response metadata (message ID, timestamp)
- WHY: Success confirmation enables delivery verification
- IMPACT: Missing metadata prevents delivery proof

**REQ-QUEUE-E004**: WHEN ESP returns 4xx client error, THEN system SHALL schedule retry with exponential backoff (base 30s, max 1h, 5 retries)
- WHY: Exponential backoff prevents overwhelming ESP APIs with retries
- IMPACT: Linear retries amplify temporary failures

**REQ-QUEUE-E005**: WHEN ESP returns 5xx server error, THEN system SHALL classify error (transient vs permanent) and route accordingly
- WHY: Error classification prevents retry of permanent failures
- IMPACT: Retrying permanent failures wastes resources

**REQ-QUEUE-E006**: WHEN retry count exceeds 5 attempts, THEN system SHALL move message to Dead Letter Queue with failure reason
- WHY: DLQ prevents infinite retry loops and enables manual intervention
- IMPACT: Infinite retries block queue processing

**REQ-QUEUE-E007**: WHEN ESP webhook received (bounce, complaint, delivery), THEN system SHALL update delivery status and trigger notification
- WHY: Webhook updates provide real-time delivery status
- IMPACT: Missing webhook handling loses delivery status updates

**REQ-QUEUE-E008**: WHEN DLQ reprocess requested via API, THEN system SHALL re-enqueue messages with reset retry counter
- WHY: Retry counter reset enables fresh retry attempts
- IMPACT: Non-reset counters prevent reprocessing

**REQ-QUEUE-E009**: WHEN primary ESP provider health check fails, THEN system SHALL failover to secondary provider per routing rules
- WHY: Automatic failover ensures delivery continuity
- IMPACT: No failover causes delivery outages

**REQ-QUEUE-E010**: WHEN message processing completes (sent or failed), THEN system SHALL acknowledge message in Redis consumer group
- WHY: Acknowledgment removes message from pending state
- IMPACT: Unacknowledged messages reappear in queue

### State-Driven Requirements (Conditional)

**REQ-QUEUE-S001**: IF retry count exceeds 5, THEN system SHALL move message to Dead Letter Queue
- WHY: Prevents infinite retry loops
- IMPACT: Unbounded retries block queue throughput

**REQ-QUEUE-S002**: IF primary ESP provider is unhealthy (3 consecutive failures), THEN system SHALL failover to secondary provider per routing rules
- WHY: Health-based failover maintains delivery SLA
- IMPACT: Sticky primary selection ignores provider failures

**REQ-QUEUE-S003**: IF queue depth exceeds threshold (default 10,000 messages), THEN system SHALL emit alert metric via monitoring system
- WHY: Queue depth alerts indicate processing bottlenecks
- IMPACT: Undetected queue growth causes delivery delays

**REQ-QUEUE-S004**: IF worker pool utilization exceeds 80%, THEN system SHALL log warning for scaling consideration
- WHY: Utilization warnings enable proactive capacity planning
- IMPACT: Saturated workers cause message backlog

**REQ-QUEUE-S005**: IF ESP provider rate limit reached (429 response), THEN system SHALL apply backpressure to queue consumption with exponential backoff
- WHY: Backpressure prevents rate limit violations
- IMPACT: Ignoring rate limits causes ESP account suspension

**REQ-QUEUE-S006**: IF message size exceeds ESP provider limit, THEN system SHALL reject with permanent failure status
- WHY: Oversized messages cannot be delivered
- IMPACT: Retry of oversized messages wastes resources

**REQ-QUEUE-S007**: IF tenant routing rule missing, THEN system SHALL use default ESP provider with fallback order (SendGrid → AWS SES → Mailgun)
- WHY: Fallback ensures delivery even without explicit routing
- IMPACT: Missing fallback causes delivery failures

### Unwanted Requirements (Prohibitions)

**REQ-QUEUE-N001**: System SHALL NOT lose messages during queue backend restarts (persistence required)
- WHY: Message loss violates reliability guarantees
- IMPACT: Redis Streams with AOF/RDB persistence prevents data loss

**REQ-QUEUE-N002**: System SHALL NOT retry messages with permanent 5xx failures (invalid recipient, authentication failure, blocked sender)
- WHY: Permanent failures cannot succeed on retry
- IMPACT: Retrying permanent failures wastes resources and delays DLQ routing

**REQ-QUEUE-N003**: System SHALL NOT expose ESP provider API keys in any log output, response, or metric
- WHY: API key exposure creates security vulnerability
- IMPACT: Key leakage enables unauthorized ESP usage

**REQ-QUEUE-N004**: System SHALL NOT process messages out of order within same tenant queue
- WHY: Order violation breaks sequential processing expectations
- IMPACT: Out-of-order delivery confuses users

**REQ-QUEUE-N005**: System SHALL NOT acknowledge message before delivery confirmation or DLQ routing
- WHY: Early acknowledgment risks message loss
- IMPACT: Premature ACK causes delivery failures to be silent

### Optional Requirements (Enhancements)

**REQ-QUEUE-O001**: WHERE possible, system SHALL support priority queue for urgent/transactional messages
- WHY: Priority processing improves user experience for critical emails
- IMPACT: FIFO queue delays urgent messages

**REQ-QUEUE-O002**: WHERE possible, system SHALL provide batch sending for bulk operations (up to 100 recipients per batch)
- WHY: Batch sending improves throughput for newsletters
- IMPACT: Single-message processing limits bulk email performance

**REQ-QUEUE-O003**: WHERE possible, system SHALL support NATS JetStream as alternative queue backend
- WHY: NATS provides distributed queue with built-in persistence
- IMPACT: Redis Streams lacks multi-region replication

**REQ-QUEUE-O004**: WHERE possible, system SHALL provide queue depth and processing rate metrics per tenant
- WHY: Per-tenant metrics enable capacity planning
- IMPACT: Aggregate metrics hide tenant-specific bottlenecks

**REQ-QUEUE-O005**: WHERE possible, system SHALL support message scheduling for delayed delivery
- WHY: Scheduled delivery enables campaign timing control
- IMPACT: Immediate delivery only limits campaign flexibility

## Specifications

### Provider Interface Design

```go
type Provider interface {
    Send(ctx context.Context, msg *Message) (*DeliveryResult, error)
    GetName() string
    HealthCheck(ctx context.Context) error
}

type DeliveryResult struct {
    ProviderMessageID string
    Status            DeliveryStatus
    Timestamp         time.Time
    Metadata          map[string]string
}

type DeliveryStatus string

const (
    StatusSent    DeliveryStatus = "sent"
    StatusFailed  DeliveryStatus = "failed"
    StatusBounced DeliveryStatus = "bounced"
)
```

### Worker Pool Pattern

- Configurable concurrency (default: 10 workers)
- Each worker consumes from Redis consumer group
- Context-based cancellation for graceful shutdown
- Worker goroutines use `context.WithTimeout` for ESP API calls (30s timeout)

### Retry Strategy

- Exponential backoff with jitter: 30s, 1m, 2m, 5m, 15m (5 total retries)
- Jitter calculation: `backoff_duration * (0.5 + rand.Float64() * 0.5)`
- Retry schedule stored in message metadata
- Permanent failures skip retry and route to DLQ immediately

### Dead Letter Queue

- Separate Redis Stream: `dlq:{tenant_id}`
- Manual reprocess API endpoint: `POST /api/v1/dlq/reprocess`
- DLQ messages include: original message, failure reason, retry history, final error

### Delivery Tracking Schema

```sql
CREATE TABLE delivery_logs (
    id BIGSERIAL PRIMARY KEY,
    message_id VARCHAR(255) UNIQUE NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    provider VARCHAR(50),
    provider_message_id VARCHAR(255),
    retry_count INT DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    INDEX idx_message_id (message_id),
    INDEX idx_tenant_status (tenant_id, status),
    INDEX idx_created_at (created_at)
);
```

### Performance Targets

- Throughput: 10,000 messages/minute sustained
- End-to-End Latency (P95): <5 seconds from queue entry to ESP delivery
- Worker Pool Utilization: 60-80% under normal load
- Queue Depth: <1,000 messages during normal operation
- DLQ Rate: <1% of total messages

## Traceability

- **REQ-QUEUE-U001** → Worker implementation in `backend/internal/queue/worker.go`
- **REQ-QUEUE-E002** → Routing engine in `backend/internal/queue/router.go`
- **REQ-QUEUE-E004** → Retry logic in `backend/internal/queue/retry.go`
- **REQ-QUEUE-S002** → Health check in `backend/internal/provider/health.go`
- **REQ-QUEUE-N003** → Log sanitization in `backend/pkg/logger/sanitizer.go`

## Success Criteria

1. All EARS requirements implemented and verified
2. Provider abstraction supports SendGrid, AWS SES, and Mailgun with normalized responses
3. Retry strategy with exponential backoff tested under failure scenarios
4. DLQ routing functional with manual reprocess API
5. Delivery tracking schema populated with status transitions
6. Performance targets achieved: 10K msg/min, <5s P95 latency
7. Zero message loss during queue backend restart (Redis persistence verified)
8. Integration tests cover normal, retry, DLQ, and webhook scenarios
9. 85%+ code coverage with unit and integration tests
10. Security audit confirms no API key exposure in logs/metrics
