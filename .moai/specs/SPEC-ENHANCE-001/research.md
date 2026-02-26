# Research: SPEC-ENHANCE-001

## Area 1: Attachment Handling — Current State

### Data Flow (Body)

```
SMTP DATA (raw RFC 5322) → bytes.Buffer → io.Copy
  → mail.ReadMessage() extracts Subject + Headers ONLY
  → raw bytes stored as-is to MessageStore (or inline PostgreSQL)
  → Worker fetches raw bytes → passes to provider as provider.Message.Body
  → Each provider treats Body as text/plain string
```

### Critical Finding: No MIME Parsing

- `net/mail.ReadMessage()` is used only for Subject/Headers extraction
- No `mime/multipart` usage anywhere in the codebase
- `msg.Body` = entire RFC 5322 raw message (headers + body), NOT just body part
- All 4 providers hardcode `text/plain` content type

### Provider Payload Format (Current)

| Provider | Body Treatment |
|----------|---------------|
| SendGrid | `Content: [{Type: "text/plain", Value: string(msg.Body)}]` |
| SES | `Body.Text.Data: strings.TrimSpace(string(msg.Body))` |
| Mailgun | `form.Set("text", string(msg.Body))` |
| MS Graph | `Body: {ContentType: "Text", Content: string(msg.Body)}` |

### Files to Modify

- `smtp/session.go` — MIME parsing in Data()
- `provider/provider.go` — Message struct (add TextBody, HTMLBody, Attachments)
- `provider/sendgrid.go` — buildPayload() attachments
- `provider/ses.go` — switch to Raw mode for attachments
- `provider/mailgun.go` — multipart/form-data with attachment fields
- `provider/msgraph.go` — attachments array
- `worker/handler.go` — MIME parse before building provider.Message

---

## Area 2: Queue Abstraction — Current State

### Existing Interfaces

Only `MessageHandler` exists. Producer, Consumer, DLQ are all concrete structs with `*redis.Client`.

### Redis Dependency Map

| File | Redis Call | Purpose |
|------|-----------|---------|
| producer.go:29 | XAdd | Enqueue to primary stream |
| consumer.go:25 | XGroupCreateMkStream | Create consumer group |
| consumer.go:36 | XReadGroup | Read from group |
| consumer.go:72 | XAck | Acknowledge message |
| worker.go:110 | XReadGroup | Direct read (bypasses Consumer!) |
| dlq.go:46 | XAdd | Move to DLQ stream |
| dlq.go:70 | XRange | Read from DLQ |
| dlq.go:95 | XDel | Remove from DLQ |

### Design Issues

1. `WorkerPool` calls `XReadGroup` directly instead of through `Consumer`
2. `retryAfterBackoff` creates inline `NewProducer(wp.client)` — tightly coupled
3. `delivery.AsyncService` holds concrete `*queue.Producer`
4. `api/dlq_handler.go` holds concrete `*queue.DLQ`
5. Retry is goroutine + sleep — lost on process restart

### Existing Abstraction Patterns to Reuse

- `msgstore.MessageStore` (Put/Get/Delete) — same factory pattern for queue
- `delivery.Service` interface — clean boundary, SMTP doesn't know about Redis
- `provider.Provider` + `provider.HTTPClient` — injectable abstraction for testing
