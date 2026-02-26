---
id: SPEC-ENHANCE-001
title: MIME Attachment Support + Pluggable Queue Abstraction
version: 0.1.0
status: draft
created: 2026-02-26
updated: 2026-02-26
author: sungwon
priority: high
tags: mime, attachment, queue, sqs, kafka, redis, abstraction
related_specs:
  - SPEC-RESTRUCTURE-001
---

## HISTORY

| Version | Date       | Author  | Description |
|---------|------------|---------|-------------|
| 0.1.0   | 2026-02-26 | sungwon | Initial draft |

---

## 1. Environment

### 1.1 Current State

The smtp-proxy currently has two limitations:

**Attachment handling:**
- SMTP DATA is read as raw bytes and stored as-is
- `mail.ReadMessage()` extracts only Subject and Headers
- No MIME parsing (`mime/multipart` not used)
- All providers send body as `text/plain` only
- Multipart emails (HTML, attachments, inline images) are corrupted

**Queue:**
- Queue layer is hardwired to Redis Streams (`XAdd`, `XReadGroup`, `XAck`)
- No abstraction interface — `Producer`, `Consumer`, `DLQ` are concrete structs with `*redis.Client`
- Retry uses goroutine + sleep (lost on process restart)
- Cannot swap to SQS/Kafka without rewriting all queue consumers

### 1.2 Target State

**Attachment handling:**
- Full MIME parsing of incoming SMTP messages
- Separate text, HTML, and attachment parts
- Each ESP provider sends with proper content types and attachments
- Backward compatible: plain text emails continue to work unchanged

**Queue:**
- Queue interface abstraction (`Enqueuer`, `Dequeuer`, `DeadLetterQueue`)
- Two implementations:
  - `redis` — for local dev/test (simple, existing infra)
  - `sqs` — for production (durable, managed, native DLQ + delayed retry)
- Kafka-ready interface (future implementation, not in this SPEC)
- Config-driven backend selection (`QUEUE_TYPE=redis|sqs`)

### 1.3 Infrastructure

- Go 1.24
- Docker / Docker Compose for local dev
- Redis 7.0+ (local queue backend)
- AWS SQS (production queue backend)
- PostgreSQL 15+ (unchanged)

---

## 2. Assumptions

- SMTP clients send standard RFC 5322 / RFC 2045-2049 MIME messages
- Attachment size is bounded by SMTP server's `max_message_size` config (no separate limit)
- Attachments are stored as part of the raw message in MessageStore (not extracted separately)
- SQS message size limit (256KB) applies to queue messages only — body is in MessageStore, queue carries ID only
- Local development uses Redis (already in docker-compose)

---

## 3. Requirements

### Module 1: MIME Parsing & Attachment Support

**REQ-MIME-001** (Ubiquitous)
The system shall parse incoming SMTP DATA as MIME messages using `mime/multipart` when Content-Type indicates multipart content.

**REQ-MIME-002** (Ubiquitous)
The system shall extract the following parts from MIME messages:
- `text/plain` body
- `text/html` body
- Attachments (filename, content-type, content bytes, Content-ID for inline)

**REQ-MIME-003** (Ubiquitous)
When the SMTP message is NOT multipart, the system shall treat the entire body as plain text (backward compatible with current behavior).

**REQ-MIME-004** (Ubiquitous)
The `provider.Message` struct shall carry `TextBody`, `HTMLBody`, and `Attachments` fields separately from raw `Body`.

**REQ-MIME-005** (Ubiquitous)
Each ESP provider shall send HTML body when available, plain text as fallback, and include all attachments using the provider's native attachment API.

**REQ-MIME-006** (Event-driven)
When an attachment exceeds the ESP provider's size limit, the system shall reject the message with an appropriate SMTP error code.

### Module 2: Pluggable Queue Abstraction

**REQ-QUEUE-001** (Ubiquitous)
The system shall define queue interfaces:

```go
// Enqueuer publishes messages to the queue.
type Enqueuer interface {
    Enqueue(ctx context.Context, msg *Message) (string, error)
}

// Dequeuer consumes messages from the queue.
type Dequeuer interface {
    Dequeue(ctx context.Context, handler func(ctx context.Context, msg *Message) error) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

// DeadLetterQueue manages failed messages.
type DeadLetterQueue interface {
    MoveToDLQ(ctx context.Context, msg *Message, reason string) error
    Reprocess(ctx context.Context, tenantID string, messageIDs []string) (int, error)
}
```

**REQ-QUEUE-002** (Ubiquitous)
The system shall provide a Redis Streams implementation of the queue interfaces, preserving current behavior (consumer groups, XReadGroup, XAck).

**REQ-QUEUE-003** (Ubiquitous)
The system shall provide an SQS implementation of the queue interfaces:
- `Enqueue` → `sqs.SendMessage` with `MessageGroupId` (FIFO) or standard queue
- `Dequeue` → `sqs.ReceiveMessage` with long polling + visibility timeout
- `MoveToDLQ` → SQS native redrive policy (configured via infra, not code)
- Retry → visibility timeout extension (no goroutine sleep)

**REQ-QUEUE-004** (Ubiquitous)
Queue backend shall be selected by configuration:

```yaml
queue:
  type: redis          # redis | sqs
  # Redis-specific
  redis_addr: "localhost:6379"
  # SQS-specific
  sqs_queue_url: "https://sqs.us-east-1.amazonaws.com/..."
  sqs_dlq_url: "https://sqs.us-east-1.amazonaws.com/..."
  sqs_region: "us-east-1"
```

**REQ-QUEUE-005** (Ubiquitous)
The factory function shall create the appropriate backend:

```go
func NewQueue(cfg Config) (Enqueuer, Dequeuer, DeadLetterQueue, error)
```

**REQ-QUEUE-006** (Ubiquitous)
The `WorkerPool` shall use the `Dequeuer` interface instead of direct `*redis.Client` access. The `retryAfterBackoff` shall use the `Enqueuer` interface instead of creating inline `NewProducer`.

**REQ-QUEUE-007** (Ubiquitous)
The `delivery.AsyncService` shall depend on `queue.Enqueuer` interface, not concrete `*queue.Producer`.

**REQ-QUEUE-008** (Unwanted)
The system shall not require SQS/AWS credentials for local development. Redis backend shall be the default.

---

## 4. Technical Approach

### Module 1: MIME Parsing

**Phase 1-1: MIME Parser utility**

New package `server/internal/mimeparse/` with:

```go
type ParsedMessage struct {
    TextBody    string
    HTMLBody    string
    Attachments []Attachment
    Headers     map[string][]string
    Subject     string
}

type Attachment struct {
    Filename    string
    ContentType string
    Content     []byte
    ContentID   string   // for inline images (cid:)
    IsInline    bool
}

func Parse(raw []byte) (*ParsedMessage, error)
```

Parse logic:
1. `mail.ReadMessage()` to split headers from body
2. Check `Content-Type` header
3. If `multipart/*` → walk parts with `multipart.Reader`
4. If `text/plain` or `text/html` → single body part
5. Extract attachments from non-text parts

**Phase 1-2: Provider Message struct update**

```go
// provider/provider.go
type Message struct {
    ID          string
    TenantID    string
    From        string
    To          []string
    Subject     string
    Headers     map[string]string
    TextBody    string
    HTMLBody    string
    Attachments []Attachment
    RawBody     []byte    // original raw bytes (for providers that support raw send)
}

type Attachment struct {
    Filename    string
    ContentType string
    Content     []byte
    ContentID   string
    IsInline    bool
}
```

**Phase 1-3: Provider updates**

| Provider | Strategy |
|----------|----------|
| SendGrid | Add `text/html` content + `attachments` array in JSON payload |
| SES | Use `Raw` mode (`SendRawEmail`) when attachments present, `Simple` mode for text-only |
| Mailgun | Switch to `multipart/form-data` with `html`, `inline`, `attachment` fields |
| MS Graph | Set `ContentType: "HTML"`, add `attachments` array with base64 content |
| Stdout/File | Log attachment metadata, write raw bytes |

**Phase 1-4: Worker handler update**

`handler.go` `HandleMessage()`:
1. Fetch raw body from MessageStore (unchanged)
2. Call `mimeparse.Parse(rawBody)` to get ParsedMessage
3. Build `provider.Message` with TextBody, HTMLBody, Attachments
4. Pass to `provider.Send()`

### Module 2: Queue Abstraction

**Phase 2-1: Interface definitions**

New file `server/internal/queue/iface.go`:

```go
type Enqueuer interface {
    Enqueue(ctx context.Context, msg *Message) (string, error)
}

type Dequeuer interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type DeadLetterQueue interface {
    MoveToDLQ(ctx context.Context, msg *Message, reason string) error
    Reprocess(ctx context.Context, tenantID string, messageIDs []string) (int, error)
}
```

**Phase 2-2: Redis implementation (refactor existing)**

```
server/internal/queue/
  iface.go              ← NEW: interfaces
  message.go            ← KEEP: Message struct, shared
  config.go             ← UPDATE: add Type field
  retry.go              ← KEEP: RetryStrategy (transport-agnostic)
  redis_enqueuer.go     ← RENAME from producer.go
  redis_dequeuer.go     ← REFACTOR from worker.go + consumer.go
  redis_dlq.go          ← RENAME from dlq.go
  factory.go            ← NEW: NewQueue(cfg) factory
```

Key changes:
- `Producer` → `RedisEnqueuer` implementing `Enqueuer`
- `WorkerPool` → `RedisDequeuer` implementing `Dequeuer`, uses Consumer internally
- `DLQ` → `RedisDLQ` implementing `DeadLetterQueue`
- Remove direct `*redis.Client` from WorkerPool, route through Consumer

**Phase 2-3: SQS implementation**

```
server/internal/queue/
  sqs_enqueuer.go       ← NEW
  sqs_dequeuer.go       ← NEW
  sqs_dlq.go            ← NEW
```

SQS specifics:
- `SQSEnqueuer.Enqueue()` → `sqs.SendMessage(QueueUrl, MessageBody: json)`
- `SQSDequeuer.Start()` → long-polling loop with `sqs.ReceiveMessage(WaitTimeSeconds: 20)`
- `SQSDequeuer` → on failure, message automatically reappears after visibility timeout (no goroutine sleep)
- `SQSDLQ` → SQS redrive policy handles DLQ automatically; `Reprocess()` reads from DLQ URL and re-sends to primary queue
- AWS SDK: `github.com/aws/aws-sdk-go-v2`

**Phase 2-4: Wiring update**

`cmd/smtp-server/main.go` and `cmd/queue-worker/main.go`:
```go
enqueuer, dequeuer, dlq, err := queue.NewQueue(cfg.Queue)
```

`delivery.AsyncService` → takes `queue.Enqueuer` instead of `*queue.Producer`
`api/dlq_handler.go` → takes `queue.DeadLetterQueue` instead of `*queue.DLQ`

---

## 5. Implementation Order

```
Phase 1-1: mimeparse package + tests
Phase 1-2: provider.Message struct update
Phase 1-3: Provider implementations (SendGrid, SES, Mailgun, MSGraph)
Phase 1-4: Worker handler MIME integration
  ↕ (independent, can parallel)
Phase 2-1: Queue interfaces
Phase 2-2: Redis implementation (refactor existing)
Phase 2-3: SQS implementation
Phase 2-4: Wiring + config update
```

Module 1 and Module 2 are independent and can be implemented in parallel.

---

## 6. Acceptance Criteria

### Module 1: MIME / Attachments

- [ ] AC-MIME-01: Plain text email (no MIME) is delivered as text/plain (backward compatible)
- [ ] AC-MIME-02: HTML-only email is delivered with HTML body via each provider
- [ ] AC-MIME-03: multipart/alternative (text + HTML) delivers both parts
- [ ] AC-MIME-04: multipart/mixed with 1 attachment delivers body + attachment via each provider
- [ ] AC-MIME-05: multipart/mixed with multiple attachments delivers all attachments
- [ ] AC-MIME-06: Inline image (Content-ID) is delivered as inline attachment
- [ ] AC-MIME-07: Nested multipart (mixed > alternative > text + html) is parsed correctly
- [ ] AC-MIME-08: Non-MIME message with only headers is handled gracefully
- [ ] AC-MIME-09: mimeparse package has 90%+ test coverage with RFC 2045 edge cases

### Module 2: Queue Abstraction

- [ ] AC-QUEUE-01: `QUEUE_TYPE=redis` uses Redis Streams (existing behavior preserved)
- [ ] AC-QUEUE-02: `QUEUE_TYPE=sqs` uses AWS SQS for enqueue/dequeue
- [ ] AC-QUEUE-03: Redis backend: consumer groups, retry, DLQ work as before
- [ ] AC-QUEUE-04: SQS backend: visibility timeout handles retry without goroutine sleep
- [ ] AC-QUEUE-05: SQS backend: DLQ reprocess reads from DLQ queue and re-sends to primary
- [ ] AC-QUEUE-06: Default config uses Redis (no AWS credentials needed for local dev)
- [ ] AC-QUEUE-07: `delivery.AsyncService` depends on `Enqueuer` interface only
- [ ] AC-QUEUE-08: `api/dlq_handler.go` depends on `DeadLetterQueue` interface only
- [ ] AC-QUEUE-09: All existing tests pass with Redis backend
- [ ] AC-QUEUE-10: SQS implementation has 85%+ test coverage with mocked AWS SDK

---

## 7. Out of Scope

- Kafka implementation (interface-ready, but not implemented in this SPEC)
- Per-attachment size limits (uses SMTP server's existing max_message_size)
- Attachment virus scanning
- S/MIME encryption or PGP
- SQS FIFO queue ordering guarantees
- Infrastructure provisioning (Terraform/CloudFormation for SQS)

---

## 8. Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| SES Raw mode complexity | Medium | Use Raw mode only when attachments present; Simple mode for text-only |
| SQS 256KB message limit | Low | Queue carries ID only (not body), well within limit |
| MIME parsing edge cases | Medium | Use Go stdlib `mime/multipart` + extensive test fixtures from RFC 2045 |
| Breaking provider tests | Medium | Phase 1-2 is additive (new fields), not destructive |

---

## 9. File Change Summary

### New Files

| File | Purpose |
|------|---------|
| `server/internal/mimeparse/parse.go` | MIME parsing logic |
| `server/internal/mimeparse/parse_test.go` | MIME parser tests with RFC fixtures |
| `server/internal/queue/iface.go` | Queue interface definitions |
| `server/internal/queue/redis_enqueuer.go` | Redis Enqueuer (from producer.go) |
| `server/internal/queue/redis_dequeuer.go` | Redis Dequeuer (from worker.go + consumer.go) |
| `server/internal/queue/redis_dlq.go` | Redis DLQ (from dlq.go) |
| `server/internal/queue/sqs_enqueuer.go` | SQS Enqueuer |
| `server/internal/queue/sqs_dequeuer.go` | SQS Dequeuer |
| `server/internal/queue/sqs_dlq.go` | SQS DLQ |
| `server/internal/queue/factory.go` | NewQueue() factory |

### Modified Files

| File | Change |
|------|--------|
| `server/internal/provider/provider.go` | Message struct: add TextBody, HTMLBody, Attachments |
| `server/internal/provider/sendgrid.go` | buildPayload() with HTML + attachments |
| `server/internal/provider/ses.go` | Raw mode for attachments |
| `server/internal/provider/mailgun.go` | multipart/form-data with attachments |
| `server/internal/provider/msgraph.go` | HTML body + attachments array |
| `server/internal/worker/handler.go` | MIME parse before building provider.Message |
| `server/internal/delivery/async.go` | Use Enqueuer interface |
| `server/internal/api/dlq_handler.go` | Use DeadLetterQueue interface |
| `server/internal/config/config.go` | Queue type config field |
| `server/cmd/smtp-server/main.go` | Factory-based queue init |
| `server/cmd/queue-worker/main.go` | Factory-based queue init |

### Deleted Files

| File | Replaced By |
|------|------------|
| `server/internal/queue/producer.go` | `redis_enqueuer.go` |
| `server/internal/queue/consumer.go` | `redis_dequeuer.go` (merged) |
| `server/internal/queue/worker.go` | `redis_dequeuer.go` (merged) |
| `server/internal/queue/dlq.go` | `redis_dlq.go` |
