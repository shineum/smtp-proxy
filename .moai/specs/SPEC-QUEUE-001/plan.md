# Implementation Plan: SPEC-QUEUE-001

## Overview

This document outlines the implementation plan for asynchronous message processing and ESP provider integration. The plan is organized by functional areas with clear task decomposition, technology stack, and architecture decisions.

## Technology Stack

### Queue Infrastructure
- **Redis Streams**: Primary queue backend with consumer groups
- **go-redis v9**: Redis client library for Go with stream support
- **Redis Persistence**: AOF (Append-Only File) with fsync=everysec

### ESP Provider SDKs
- **SendGrid**: sendgrid/sendgrid-go v3.14+
- **AWS SES**: aws-sdk-go-v2/service/sesv2 v1.30+
- **Mailgun**: mailgun/mailgun-go/v4 v4.12+

### Database
- **PostgreSQL 14+**: Delivery tracking and audit logs
- **pgx/v5**: PostgreSQL driver with connection pooling
- **goose**: Database migration tool

### Observability
- **Prometheus**: Metrics collection (queue depth, processing rate, error rate)
- **OpenTelemetry**: Distributed tracing with correlation IDs
- **Zap**: Structured logging with log sanitization

## Architecture Decisions

### 1. Provider Interface Design

**Decision**: Generic provider interface with normalized response format

**Rationale**:
- Decouples queue worker from ESP-specific implementations
- Enables easy addition of new providers without worker changes
- Standardizes error handling and status mapping

**Interface**:
```go
type Provider interface {
    Send(ctx context.Context, msg *Message) (*DeliveryResult, error)
    GetName() string
    HealthCheck(ctx context.Context) error
}
```

**Implementation Strategy**:
- Each ESP implements provider interface with SDK-specific logic
- Factory pattern for provider instantiation based on routing rules
- Provider registry for dependency injection

### 2. Worker Pool Pattern

**Decision**: Fixed-size worker pool with concurrent message processing

**Configuration**:
- Default: 10 concurrent workers per instance
- Configurable via environment variable: `QUEUE_WORKER_COUNT`
- Each worker runs as independent goroutine

**Worker Lifecycle**:
1. Worker starts and joins Redis consumer group
2. Pulls message from stream with XREADGROUP (block=5s)
3. Processes message with provider.Send()
4. Updates delivery status in PostgreSQL
5. Acknowledges message with XACK
6. Repeats until shutdown signal

**Shutdown Protocol**:
- Context cancellation propagates to all workers
- Workers finish current message before exit (graceful timeout: 30s)
- Unacknowledged messages return to pending state

### 3. Retry Strategy

**Decision**: Exponential backoff with jitter and retry count limit

**Retry Schedule**:
- Attempt 1: Immediate
- Attempt 2: 30s + jitter
- Attempt 3: 1m + jitter
- Attempt 4: 2m + jitter
- Attempt 5: 5m + jitter
- Attempt 6: 15m + jitter
- After 6 attempts: Route to DLQ

**Jitter Calculation**:
```go
jitter = backoff_duration * (0.5 + rand.Float64() * 0.5)
actual_delay = backoff_duration + jitter
```

**Retry Logic**:
- 4xx errors: Retry with backoff (client errors may be transient)
- 5xx errors: Classify as transient (retry) or permanent (DLQ)
- Permanent failures: invalid recipient, authentication failure, blocked sender
- Retry metadata stored in Redis message payload

### 4. Dead Letter Queue (DLQ)

**Decision**: Separate Redis Stream per tenant for failed messages

**DLQ Structure**:
- Stream key pattern: `dlq:{tenant_id}`
- Message payload: original message + failure metadata
- Retention: 7 days (configurable)

**DLQ Message Metadata**:
```go
type DLQMessage struct {
    OriginalMessage *Message
    FailureReason   string
    RetryHistory    []RetryAttempt
    FinalError      error
    MovedToDLQAt    time.Time
}
```

**Reprocess API**:
- Endpoint: `POST /api/v1/dlq/reprocess`
- Request: `{"message_ids": ["msg-001", "msg-002"]}`
- Action: Re-enqueue to primary queue with reset retry counter

### 5. Delivery Tracking

**Decision**: PostgreSQL table with status history and indexing strategy

**Schema**:
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
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_message_id ON delivery_logs(message_id);
CREATE INDEX idx_tenant_status ON delivery_logs(tenant_id, status);
CREATE INDEX idx_created_at ON delivery_logs(created_at DESC);
CREATE INDEX idx_provider_message_id ON delivery_logs(provider_message_id) WHERE provider_message_id IS NOT NULL;
```

**Status Transitions**:
- `pending`: Message created by SMTP server, not yet queued
- `queued`: Message added to Redis Stream
- `processing`: Worker picked message, sending to ESP
- `sent`: ESP confirmed delivery (2xx response)
- `failed`: Permanent failure or retry exhaustion
- `bounced`: ESP webhook reported bounce
- `complained`: ESP webhook reported complaint

### 6. ESP Provider Failover

**Decision**: Health-based automatic failover with routing rules

**Health Check Strategy**:
- Each provider implements HealthCheck() method
- Background goroutine checks provider health every 30s
- Health status stored in-memory with expiration
- 3 consecutive failures mark provider as unhealthy

**Routing Rules Engine**:
```go
type RoutingRule struct {
    TenantID        string
    PrimaryProvider string
    FallbackOrder   []string
}
```

**Failover Logic**:
1. Resolve routing rule for tenant
2. Check primary provider health
3. If unhealthy, use first healthy provider in fallback order
4. If all unhealthy, retry with backoff (queue message remains pending)

### 7. Rate Limiting and Backpressure

**Decision**: Exponential backoff on ESP rate limit (429 response)

**Backpressure Mechanism**:
- Worker detects 429 response from provider
- Calculates backoff based on Retry-After header or default (60s)
- Worker sleeps for backoff duration
- Message remains in pending state (not acknowledged)
- Message will be reprocessed by same or different worker after backoff

**Queue Depth Monitoring**:
- Prometheus metric: `queue_depth{tenant_id}`
- Alert threshold: 10,000 messages
- Mitigation: Scale worker instances horizontally

## Task Decomposition

### Phase 1: Core Queue Infrastructure (10 tasks)

**Task 1.1**: Redis Streams setup and configuration
- File: `backend/pkg/queue/redis.go`
- Deliverable: Redis client initialization with connection pooling
- Estimate: 4 hours

**Task 1.2**: Message model definition
- File: `backend/internal/queue/message.go`
- Deliverable: Message struct with serialization (JSON)
- Estimate: 2 hours

**Task 1.3**: Queue producer implementation
- File: `backend/internal/queue/producer.go`
- Deliverable: EnqueueMessage() function using XADD
- Estimate: 4 hours

**Task 1.4**: Consumer group initialization
- File: `backend/internal/queue/consumer.go`
- Deliverable: CreateConsumerGroup() with XGROUP CREATE
- Estimate: 3 hours

**Task 1.5**: Worker pool implementation
- File: `backend/internal/queue/worker.go`
- Deliverable: Worker pool with configurable concurrency
- Estimate: 6 hours

**Task 1.6**: Message acknowledgment logic
- File: `backend/internal/queue/ack.go`
- Deliverable: AcknowledgeMessage() using XACK
- Estimate: 2 hours

**Task 1.7**: Pending message reprocessing
- File: `backend/internal/queue/reprocess.go`
- Deliverable: ClaimPendingMessages() using XPENDING/XCLAIM
- Estimate: 5 hours

**Task 1.8**: Queue depth monitoring
- File: `backend/internal/queue/metrics.go`
- Deliverable: Prometheus metrics for queue depth and processing rate
- Estimate: 3 hours

**Task 1.9**: Queue configuration management
- File: `backend/internal/queue/config.go`
- Deliverable: Configuration struct with validation
- Estimate: 2 hours

**Task 1.10**: Integration tests for queue operations
- File: `backend/internal/queue/queue_test.go`
- Deliverable: Tests for enqueue, dequeue, acknowledge flows
- Estimate: 6 hours

**Phase 1 Total**: 37 hours

### Phase 2: ESP Provider Abstraction (8 tasks)

**Task 2.1**: Provider interface definition
- File: `backend/internal/provider/provider.go`
- Deliverable: Provider interface with Send() and HealthCheck()
- Estimate: 2 hours

**Task 2.2**: SendGrid provider implementation
- File: `backend/internal/provider/sendgrid.go`
- Deliverable: SendGrid SDK integration with error mapping
- Estimate: 6 hours

**Task 2.3**: AWS SES provider implementation
- File: `backend/internal/provider/ses.go`
- Deliverable: AWS SDK v2 SES integration with error mapping
- Estimate: 6 hours

**Task 2.4**: Mailgun provider implementation
- File: `backend/internal/provider/mailgun.go`
- Deliverable: Mailgun SDK integration with error mapping
- Estimate: 6 hours

**Task 2.5**: Provider factory and registry
- File: `backend/internal/provider/factory.go`
- Deliverable: NewProvider() factory function with dependency injection
- Estimate: 3 hours

**Task 2.6**: Provider health check system
- File: `backend/internal/provider/health.go`
- Deliverable: Background health checker with status cache
- Estimate: 5 hours

**Task 2.7**: Provider configuration
- File: `backend/internal/provider/config.go`
- Deliverable: Configuration struct for API keys and endpoints
- Estimate: 2 hours

**Task 2.8**: Provider integration tests
- File: `backend/internal/provider/provider_test.go`
- Deliverable: Tests with mock ESP APIs
- Estimate: 8 hours

**Phase 2 Total**: 38 hours

### Phase 3: Retry and DLQ Logic (6 tasks)

**Task 3.1**: Retry strategy implementation
- File: `backend/internal/queue/retry.go`
- Deliverable: Exponential backoff with jitter calculation
- Estimate: 4 hours

**Task 3.2**: Error classification logic
- File: `backend/internal/provider/error_classifier.go`
- Deliverable: Classify 5xx errors as transient vs permanent
- Estimate: 4 hours

**Task 3.3**: DLQ producer
- File: `backend/internal/queue/dlq.go`
- Deliverable: MoveToDLQ() function with metadata storage
- Estimate: 4 hours

**Task 3.4**: DLQ reprocess API endpoint
- File: `backend/cmd/api/handlers/dlq.go`
- Deliverable: HTTP handler for POST /api/v1/dlq/reprocess
- Estimate: 5 hours

**Task 3.5**: Retry metadata storage
- File: `backend/internal/queue/retry_metadata.go`
- Deliverable: Store retry history in message payload
- Estimate: 3 hours

**Task 3.6**: Retry and DLQ tests
- File: `backend/internal/queue/retry_test.go`
- Deliverable: Unit tests for retry logic and DLQ routing
- Estimate: 6 hours

**Phase 3 Total**: 26 hours

### Phase 4: Delivery Tracking (5 tasks)

**Task 4.1**: Database schema migration
- File: `backend/migrations/001_create_delivery_logs.sql`
- Deliverable: Goose migration for delivery_logs table
- Estimate: 2 hours

**Task 4.2**: Delivery log repository
- File: `backend/internal/repository/delivery_log.go`
- Deliverable: CRUD operations for delivery_logs table
- Estimate: 5 hours

**Task 4.3**: Status update service
- File: `backend/internal/service/delivery_status.go`
- Deliverable: UpdateStatus() function with transaction support
- Estimate: 4 hours

**Task 4.4**: Webhook receiver for ESP callbacks
- File: `backend/cmd/api/handlers/webhook.go`
- Deliverable: HTTP handlers for SendGrid, SES, Mailgun webhooks
- Estimate: 8 hours

**Task 4.5**: Delivery tracking tests
- File: `backend/internal/repository/delivery_log_test.go`
- Deliverable: Repository and webhook tests
- Estimate: 6 hours

**Phase 4 Total**: 25 hours

### Phase 5: Routing Engine (4 tasks)

**Task 5.1**: Routing rule model
- File: `backend/internal/routing/rule.go`
- Deliverable: RoutingRule struct with validation
- Estimate: 2 hours

**Task 5.2**: Routing rule repository
- File: `backend/internal/repository/routing_rule.go`
- Deliverable: Database-backed routing rule storage
- Estimate: 4 hours

**Task 5.3**: Routing engine implementation
- File: `backend/internal/routing/engine.go`
- Deliverable: ResolveProvider() with failover logic
- Estimate: 5 hours

**Task 5.4**: Routing engine tests
- File: `backend/internal/routing/engine_test.go`
- Deliverable: Tests for routing and failover scenarios
- Estimate: 4 hours

**Phase 5 Total**: 15 hours

### Phase 6: Observability and Operations (5 tasks)

**Task 6.1**: Structured logging with sanitization
- File: `backend/pkg/logger/logger.go`
- Deliverable: Zap logger with API key redaction
- Estimate: 4 hours

**Task 6.2**: Prometheus metrics integration
- File: `backend/internal/queue/metrics.go`
- Deliverable: Metrics for queue depth, processing rate, error rate
- Estimate: 5 hours

**Task 6.3**: OpenTelemetry tracing
- File: `backend/pkg/telemetry/tracer.go`
- Deliverable: Trace spans for message processing with correlation IDs
- Estimate: 6 hours

**Task 6.4**: Health check endpoint
- File: `backend/cmd/api/handlers/health.go`
- Deliverable: HTTP handler for /health with provider status
- Estimate: 3 hours

**Task 6.5**: Operational documentation
- File: `backend/docs/operations.md`
- Deliverable: Runbook for scaling, monitoring, troubleshooting
- Estimate: 4 hours

**Phase 6 Total**: 22 hours

### Phase 7: Integration and E2E Testing (4 tasks)

**Task 7.1**: E2E test setup
- File: `backend/tests/e2e/queue_test.go`
- Deliverable: Docker Compose with Redis and PostgreSQL
- Estimate: 4 hours

**Task 7.2**: Normal flow E2E test
- File: `backend/tests/e2e/normal_flow_test.go`
- Deliverable: Test: SMTP → Queue → Provider → Delivery Log
- Estimate: 5 hours

**Task 7.3**: Retry and DLQ E2E test
- File: `backend/tests/e2e/retry_dlq_test.go`
- Deliverable: Test: Retry exhaustion → DLQ → Reprocess
- Estimate: 6 hours

**Task 7.4**: Webhook E2E test
- File: `backend/tests/e2e/webhook_test.go`
- Deliverable: Test: ESP webhook → Status update
- Estimate: 5 hours

**Phase 7 Total**: 20 hours

## Total Effort Estimate

- Phase 1: 37 hours
- Phase 2: 38 hours
- Phase 3: 26 hours
- Phase 4: 25 hours
- Phase 5: 15 hours
- Phase 6: 22 hours
- Phase 7: 20 hours

**Total**: 183 hours (~4.5 weeks for 1 developer at 40 hours/week)

## File Mapping

### Backend Queue System
- `backend/internal/queue/`: Core queue logic (producer, consumer, worker)
- `backend/internal/provider/`: ESP provider implementations
- `backend/internal/routing/`: Routing engine
- `backend/internal/repository/`: Database access layer
- `backend/internal/service/`: Business logic services
- `backend/cmd/queue-worker/`: Worker process entry point
- `backend/cmd/api/`: API server for DLQ reprocess and webhooks
- `backend/migrations/`: Database schema migrations
- `backend/pkg/`: Shared libraries (logger, telemetry, queue client)

### Configuration
- `backend/config/queue.yaml`: Queue worker configuration
- `backend/config/providers.yaml`: ESP provider credentials

### Tests
- `backend/internal/*/`: Unit tests (co-located with implementation)
- `backend/tests/e2e/`: End-to-end integration tests

## Performance Targets

### Throughput
- **Target**: 10,000 messages/minute sustained
- **Strategy**: Horizontal scaling of worker instances
- **Metric**: `queue_processing_rate{tenant_id}` (Prometheus)

### End-to-End Latency
- **Target**: <5 seconds (P95) from queue entry to ESP delivery
- **Breakdown**:
  - Queue enqueue: <100ms
  - Worker pickup: <1s (average)
  - ESP API call: <2s (average)
  - Status update: <100ms
  - Buffer: ~1.8s
- **Metric**: `queue_e2e_latency_seconds` histogram

### Queue Depth
- **Target**: <1,000 messages during normal operation
- **Alert Threshold**: 10,000 messages
- **Metric**: `queue_depth{tenant_id}`

### Worker Utilization
- **Target**: 60-80% under normal load
- **Metric**: `worker_utilization_percent`

### DLQ Rate
- **Target**: <1% of total messages
- **Metric**: `dlq_message_count / total_message_count`

## Risk Analysis and Mitigation

### Risk 1: Redis Streams Message Loss

**Probability**: Low
**Impact**: High (message loss violates reliability)

**Mitigation**:
- Enable Redis AOF persistence with fsync=everysec
- Use consumer groups with XACK for at-least-once delivery
- Monitor Redis disk usage and set maxmemory-policy=noeviction
- Implement pending message reprocessing with XCLAIM

### Risk 2: ESP API Rate Limiting

**Probability**: Medium
**Impact**: Medium (delivery delays)

**Mitigation**:
- Implement backpressure mechanism on 429 responses
- Configure per-provider rate limits in routing rules
- Monitor ESP API usage metrics
- Use batch sending where supported (Task 3.5 optional)

### Risk 3: PostgreSQL Write Bottleneck

**Probability**: Medium
**Impact**: Medium (delivery tracking delays)

**Mitigation**:
- Use connection pooling with pgx
- Batch status updates where possible
- Index delivery_logs table appropriately
- Consider async status updates with buffering

### Risk 4: Permanent Failure Misclassification

**Probability**: Low
**Impact**: Medium (wasted retry attempts)

**Mitigation**:
- Maintain comprehensive error classification rules
- Log all DLQ routings for manual review
- Implement manual override API for reprocessing
- Monitor DLQ rate for anomalies

### Risk 5: Worker Shutdown Message Loss

**Probability**: Low
**Impact**: Medium (message reprocessing needed)

**Mitigation**:
- Implement graceful shutdown with context cancellation
- Set shutdown timeout (30s) for in-flight messages
- Use XPENDING to detect orphaned messages
- Implement automatic pending message reclaim on worker startup

### Risk 6: ESP Provider Outage

**Probability**: Medium
**Impact**: High (delivery failures)

**Mitigation**:
- Implement automatic failover to secondary provider
- Monitor provider health with background checks
- Configure multi-provider fallback chains
- Alert on provider unavailability

## Next Steps

After SPEC approval:

1. **Phase 1-2**: Core queue and provider implementation (2 weeks)
2. **Phase 3-4**: Retry, DLQ, and delivery tracking (1.5 weeks)
3. **Phase 5-6**: Routing and observability (1 week)
4. **Phase 7**: Integration testing and deployment (1 week)

Total timeline: ~5.5 weeks with buffer for code review and testing.

## Traceability

- **SPEC-QUEUE-001**: All tasks implement requirements from spec.md
- **REQ-QUEUE-U001**: Task 1.5 (Worker pool FIFO processing)
- **REQ-QUEUE-E002**: Task 5.3 (Routing engine)
- **REQ-QUEUE-E004**: Task 3.1 (Retry strategy)
- **REQ-QUEUE-S002**: Task 2.6 (Provider health check)
- **REQ-QUEUE-N003**: Task 6.1 (Log sanitization)
