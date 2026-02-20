# Plan: Make Redis Queue Optional (Sync/Async Delivery Mode)

## Context

Currently, the SMTP server stores messages to PostgreSQL with status `queued` but never delivers them. The ESP providers (SendGrid, SES, Mailgun, MSGraph), routing engine, and queue system are all implemented but not wired into the message flow. The user wants the system to work **without Redis** in a synchronous delivery mode, while keeping async (Redis-based) delivery as an optional feature.

## Approach: DeliveryService Abstraction

Create a `delivery.Service` interface with two implementations:
- **Sync mode**: Delivers directly via ESP providers after DB insert (no Redis required)
- **Async mode**: Enqueues to Redis Streams, delivered by queue-worker (existing queue package)

Config-driven mode selection via `delivery.mode: "sync" | "async"` in config.

## Files to Create

### 1. `server/internal/delivery/service.go` - Interface

```go
type Service interface {
    DeliverMessage(ctx context.Context, msg *DeliveryRequest) error
}

type DeliveryRequest struct {
    MessageID  int64
    AccountID  int64
    Sender     string
    Recipients []string
    Subject    string
    Headers    json.RawMessage
    Body       []byte
}
```

### 2. `server/internal/delivery/sync.go` - Sync Implementation

- Uses `routing.Engine` to resolve provider for tenant
- Uses `provider.Registry` to get provider instance
- Calls `provider.Send()` directly
- Updates message status in DB (delivered/failed) via `storage.Querier`
- Creates delivery_log entry

### 3. `server/internal/delivery/async.go` - Async Implementation

- Uses existing `queue.Producer` to XADD message to Redis Streams
- Message delivered later by queue-worker process
- Minimal implementation: marshal DeliveryRequest, call producer.Enqueue()

### 4. `server/internal/worker/handler.go` - Queue Worker Handler

- Implements `queue.MessageHandler` interface
- Deserializes queue message into DeliveryRequest
- Uses sync delivery logic (routing + provider) to deliver
- Updates DB status and creates delivery_log

## Files to Modify

### 5. `server/internal/config/config.go`

Add config sections:

```go
type DeliveryConfig struct {
    Mode string `mapstructure:"mode"` // "sync" or "async", default "sync"
}

type QueueConfig struct {
    RedisURL    string `mapstructure:"redis_url"`
    StreamName  string `mapstructure:"stream_name"`
    GroupName   string `mapstructure:"group_name"`
    ConsumerID  string `mapstructure:"consumer_id"`
    Workers     int    `mapstructure:"workers"`
}
```

Add to Config struct: `Delivery DeliveryConfig` and `Queue QueueConfig`

### 6. `server/internal/smtp/backend.go`

- Add `delivery delivery.Service` field to Backend struct
- Update `NewBackend` signature: add `delivery.Service` parameter

### 7. `server/internal/smtp/session.go`

- After `EnqueueMessage()` DB insert (line ~200), call `s.backend.delivery.DeliverMessage()`
- In sync mode: delivery happens inline, update message status based on result
- In async mode: enqueue returns quickly, worker delivers later
- Handle delivery errors gracefully (log + update status, don't fail SMTP transaction)

### 8. `server/cmd/smtp-server/main.go`

- Initialize provider registry with configured providers
- Initialize health checker
- Initialize routing engine
- Create delivery.Service based on config mode (sync or async)
- Pass delivery service to NewBackend()

### 9. `server/cmd/queue-worker/main.go`

- Replace stub with full implementation
- Connect to Redis, create consumer group
- Initialize provider registry, routing engine (same as smtp-server)
- Create worker handler, start worker pool
- Graceful shutdown on SIGINT/SIGTERM

## Existing Code to Reuse

- `server/internal/provider/provider.go` - Provider interface, Registry, HealthChecker
- `server/internal/provider/factory.go` - NewProvider() factory
- `server/internal/provider/error_classifier.go` - IsPermanent/IsTransient
- `server/internal/routing/engine.go` - ResolveProvider() with health-based failover
- `server/internal/queue/producer.go` - Redis Streams producer
- `server/internal/queue/worker.go` - WorkerPool with MessageHandler interface
- `server/internal/queue/consumer.go` - Redis Streams consumer
- `server/internal/storage/queries.go` - EnqueueMessage, UpdateMessageStatus

## Implementation Order

1. Config changes (config.go) - add Delivery and Queue sections
2. Create delivery package (service.go, sync.go, async.go)
3. Create worker handler (worker/handler.go)
4. Modify SMTP backend/session to use delivery service
5. Wire smtp-server main.go
6. Implement queue-worker main.go
7. Write tests for delivery package

## Default Behavior

- `delivery.mode` defaults to `"sync"` - no Redis required
- PostgreSQL is the only required dependency for basic operation
- Redis only needed when `delivery.mode: "async"` is explicitly configured

## Verification

1. **Docker build**: `docker compose run --rm go-build` to compile all 3 binaries
2. **Unit tests**: `go test -race ./server/internal/delivery/...`
3. **Sync mode test**: Start smtp-server with sync config, send test email via SMTP, verify provider.Send() called and message status updated
4. **Async mode test**: Start smtp-server + queue-worker with async config + Redis, send email, verify message queued then delivered
