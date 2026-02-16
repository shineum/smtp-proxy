# Acceptance Criteria: SPEC-QUEUE-001

Version: 1.1.0

## Overview

This document defines the acceptance criteria for the asynchronous message processing and ESP provider integration system. All scenarios use Given/When/Then format for clarity and testability.

## Test Scenarios

### Scenario 1: Message Enqueue from SMTP DATA Command

**Priority**: P0 (Critical Path)

**Given**:
- SMTP server receives DATA command with valid email message
- Message size is within 10MB limit
- Tenant ID is identified from SMTP session

**When**:
- SMTP server calls `queue.EnqueueMessage(ctx, msg)`
- EnqueueMessage() generates unique message ID (UUID)
- EnqueueMessage() executes XADD to Redis Stream

**Then**:
- Message is persisted in Redis Stream `queue:{tenant_id}`
- Message ID is assigned and returned to SMTP server
- Message status is set to "queued" in delivery_logs table
- SMTP server responds with "250 OK" to client
- Message metadata includes: tenant_id, from, to, subject, headers, body_path

**Success Metrics**:
- Message enqueue latency <100ms (P95)
- Zero message loss during Redis restart (AOF persistence verified)

**Test Implementation**:
```go
func TestMessageEnqueueFromSMTP(t *testing.T) {
    // Given: SMTP message with valid data
    msg := &queue.Message{
        TenantID: "tenant-001",
        From:     "sender@example.com",
        To:       []string{"recipient@example.com"},
        Subject:  "Test Email",
        BodyPath: "/tmp/body.txt",
    }

    // When: Enqueue message
    messageID, err := queueProducer.EnqueueMessage(ctx, msg)

    // Then: Verify results
    assert.NoError(t, err)
    assert.NotEmpty(t, messageID)

    // Verify Redis Stream entry
    entries := redisClient.XRange(ctx, "queue:tenant-001", "-", "+").Val()
    assert.Len(t, entries, 1)

    // Verify delivery log entry
    log := deliveryRepo.GetByMessageID(ctx, messageID)
    assert.Equal(t, "queued", log.Status)
}
```

---

### Scenario 2: Successful Delivery via SendGrid

**Priority**: P0 (Critical Path)

**Given**:
- Message is queued in Redis Stream
- SendGrid provider is configured with valid API key
- SendGrid API is healthy and responsive
- Routing rule for tenant specifies SendGrid as primary provider

**When**:
- Queue worker picks message from stream using XREADGROUP
- Worker resolves provider via routing engine → SendGrid
- Worker calls `sendgridProvider.Send(ctx, msg)`
- SendGrid API returns 202 Accepted with message ID

**Then**:
- Message status updated to "sent" in delivery_logs table
- Provider message ID stored (SendGrid message ID)
- Message acknowledged in Redis Stream with XACK
- Prometheus metric `messages_sent_total{provider="sendgrid"}` incremented
- Trace span completed with correlation ID

**Success Metrics**:
- End-to-end latency <5s (P95)
- SendGrid delivery success rate >99.5%

**Test Implementation**:
```go
func TestSuccessfulDeliveryViaSendGrid(t *testing.T) {
    // Given: Queued message and mocked SendGrid API
    mockSendGrid := &MockSendGridAPI{
        Response: &sendgrid.Response{
            StatusCode: 202,
            Body:       `{"message_id":"sg-msg-123"}`,
        },
    }
    provider := provider.NewSendGridProvider(mockSendGrid)

    msg := &queue.Message{
        ID:       "msg-001",
        TenantID: "tenant-001",
        From:     "sender@example.com",
        To:       []string{"recipient@example.com"},
    }

    // When: Send via provider
    result, err := provider.Send(ctx, msg)

    // Then: Verify success
    assert.NoError(t, err)
    assert.Equal(t, provider.StatusSent, result.Status)
    assert.Equal(t, "sg-msg-123", result.ProviderMessageID)

    // Verify delivery log update
    log := deliveryRepo.GetByMessageID(ctx, "msg-001")
    assert.Equal(t, "sent", log.Status)
    assert.Equal(t, "sendgrid", log.Provider)
    assert.Equal(t, "sg-msg-123", log.ProviderMessageID)
}
```

---

### Scenario 3: Successful Delivery via AWS SES

**Priority**: P0 (Critical Path)

**Given**:
- Message is queued in Redis Stream
- AWS SES provider is configured with IAM credentials
- SES API is healthy and responsive
- Routing rule for tenant specifies AWS SES as primary provider

**When**:
- Queue worker picks message from stream
- Worker resolves provider via routing engine → AWS SES
- Worker calls `sesProvider.Send(ctx, msg)`
- SES API returns SendEmailResponse with MessageId

**Then**:
- Message status updated to "sent" in delivery_logs table
- Provider message ID stored (SES MessageId)
- Message acknowledged in Redis Stream with XACK
- Prometheus metric `messages_sent_total{provider="ses"}` incremented
- Trace span completed with correlation ID

**Success Metrics**:
- End-to-end latency <5s (P95)
- SES delivery success rate >99.5%

**Test Implementation**:
```go
func TestSuccessfulDeliveryViaSES(t *testing.T) {
    // Given: Queued message and mocked SES API
    mockSES := &MockSESAPI{
        Response: &sesv2.SendEmailOutput{
            MessageId: aws.String("ses-msg-456"),
        },
    }
    provider := provider.NewSESProvider(mockSES)

    msg := &queue.Message{
        ID:       "msg-002",
        TenantID: "tenant-002",
        From:     "sender@example.com",
        To:       []string{"recipient@example.com"},
    }

    // When: Send via provider
    result, err := provider.Send(ctx, msg)

    // Then: Verify success
    assert.NoError(t, err)
    assert.Equal(t, provider.StatusSent, result.Status)
    assert.Equal(t, "ses-msg-456", result.ProviderMessageID)

    // Verify delivery log update
    log := deliveryRepo.GetByMessageID(ctx, "msg-002")
    assert.Equal(t, "sent", log.Status)
    assert.Equal(t, "ses", log.Provider)
}
```

---

### Scenario 4: Successful Delivery via Microsoft Graph

**Priority**: P0 (Critical Path)

**Given**:
- Message is queued in Redis Stream
- Microsoft Graph provider is configured with Azure AD credentials (tenant_id, client_id, client_secret)
- Microsoft Graph API is healthy and responsive
- OAuth 2.0 access token is cached and valid
- Routing rule for tenant specifies Microsoft Graph as primary provider

**When**:
- Queue worker picks message from stream using XREADGROUP
- Worker resolves provider via routing engine → Microsoft Graph
- Worker calls `msgraphProvider.Send(ctx, msg)`
- Microsoft Graph API POST /v1.0/users/{user-id}/sendMail returns 202 Accepted

**Then**:
- Message status updated to "sent" in delivery_logs table
- Provider message ID stored (Microsoft Graph tracking ID if available)
- Message acknowledged in Redis Stream with XACK
- Prometheus metric `messages_sent_total{provider="msgraph"}` incremented
- Trace span completed with correlation ID

**Success Metrics**:
- End-to-end latency <5s (P95)
- Microsoft Graph delivery success rate >99.5%

**Test Implementation**:
```go
func TestSuccessfulDeliveryViaMSGraph(t *testing.T) {
    // Given: Queued message and mocked Microsoft Graph API
    mockMSGraph := &MockMSGraphAPI{
        Response: &http.Response{
            StatusCode: 202,
            Body:       io.NopCloser(strings.NewReader(`{}`)),
        },
    }
    tokenManager := &MockTokenManager{
        Token: "valid-access-token",
    }
    provider := provider.NewMSGraphProvider(mockMSGraph, tokenManager)

    msg := &queue.Message{
        ID:       "msg-010",
        TenantID: "tenant-005",
        From:     "sender@example.com",
        To:       []string{"recipient@example.com"},
    }

    // When: Send via provider
    result, err := provider.Send(ctx, msg)

    // Then: Verify success
    assert.NoError(t, err)
    assert.Equal(t, provider.StatusSent, result.Status)

    // Verify delivery log update
    log := deliveryRepo.GetByMessageID(ctx, "msg-010")
    assert.Equal(t, "sent", log.Status)
    assert.Equal(t, "msgraph", log.Provider)
}
```

---

### Scenario 5: OAuth Token Refresh During Delivery

**Priority**: P0 (Critical Path)

**Given**:
- Message is queued in Redis Stream
- Microsoft Graph provider has expired OAuth token in cache
- Azure AD token endpoint is responsive
- Token refresh credentials are valid (tenant_id, client_id, client_secret)

**When**:
- Worker calls `msgraphProvider.Send(ctx, msg)`
- Provider detects expired token (cache expiry or 401 response)
- Token manager calls Azure AD token endpoint with client credentials flow
- Azure AD returns new access token with 60-minute expiry
- Token manager caches new token
- Provider retries sendMail request with new token
- Microsoft Graph API returns 202 Accepted

**Then**:
- New access token cached with expiry timestamp
- Message delivered successfully via Microsoft Graph
- Message status updated to "sent" in delivery_logs table
- Prometheus metric `msgraph_token_refresh_total` incremented
- No user-visible error or retry count increment

**Success Metrics**:
- Token refresh latency <2s
- Token refresh success rate >99.9%

**Test Implementation**:
```go
func TestOAuthTokenRefreshDuringDelivery(t *testing.T) {
    // Given: Expired token and mock Azure AD
    mockAzureAD := &MockAzureADTokenEndpoint{
        Response: &oauth2.Token{
            AccessToken: "new-access-token",
            Expiry:      time.Now().Add(60 * time.Minute),
        },
    }
    tokenManager := provider.NewTokenManager(mockAzureAD, "tenant-id", "client-id", "client-secret")

    // Set expired token in cache
    tokenManager.SetToken(&oauth2.Token{
        AccessToken: "expired-token",
        Expiry:      time.Now().Add(-1 * time.Hour),
    })

    mockMSGraph := &MockMSGraphAPI{
        OnRequest: func(req *http.Request) (*http.Response, error) {
            // First request with expired token fails
            if req.Header.Get("Authorization") == "Bearer expired-token" {
                return &http.Response{StatusCode: 401}, nil
            }
            // Second request with new token succeeds
            return &http.Response{StatusCode: 202}, nil
        },
    }

    provider := provider.NewMSGraphProvider(mockMSGraph, tokenManager)
    msg := &queue.Message{ID: "msg-011"}

    // When: Send triggers token refresh
    result, err := provider.Send(ctx, msg)

    // Then: Verify success after refresh
    assert.NoError(t, err)
    assert.Equal(t, provider.StatusSent, result.Status)
    assert.Equal(t, "new-access-token", tokenManager.GetToken().AccessToken)
}
```

---

### Scenario 6: Retry on Transient ESP Failure (4xx)

**Priority**: P0 (Critical Path)

**Given**:
- Message is queued in Redis Stream
- SendGrid provider returns 429 Too Many Requests (rate limit)
- Retry count is 0 (first attempt)

**When**:
- Worker attempts send and receives 429 response
- Error classifier identifies error as transient
- Retry logic calculates backoff: 30s + jitter
- Worker schedules retry by updating message metadata
- Worker does NOT acknowledge message (leaves in pending)

**Then**:
- Message status updated to "processing" in delivery_logs table
- Retry count incremented to 1
- Retry metadata includes: attempt count, backoff duration, error message
- Message remains in Redis Stream pending state
- Next worker attempt occurs after backoff duration (30s + jitter)

**Success Metrics**:
- Retry success rate >80% for transient errors
- Exponential backoff properly applied

**Test Implementation**:
```go
func TestRetryOnTransientFailure(t *testing.T) {
    // Given: Message and rate-limited provider
    mockSendGrid := &MockSendGridAPI{
        Response: &sendgrid.Response{
            StatusCode: 429,
            Headers:    map[string][]string{"Retry-After": {"30"}},
        },
    }
    provider := provider.NewSendGridProvider(mockSendGrid)
    retryStrategy := retry.NewExponentialBackoff()

    msg := &queue.Message{
        ID:         "msg-003",
        RetryCount: 0,
    }

    // When: Send fails with 429
    result, err := provider.Send(ctx, msg)

    // Then: Verify retry scheduled
    assert.Error(t, err)
    assert.True(t, retryStrategy.IsRetryable(err))

    // Calculate backoff
    backoff := retryStrategy.Calculate(msg.RetryCount)
    assert.GreaterOrEqual(t, backoff, 30*time.Second)
    assert.LessOrEqual(t, backoff, 60*time.Second) // max with jitter

    // Verify message NOT acknowledged
    pending := redisClient.XPending(ctx, "queue:tenant-001", "workers").Val()
    assert.Greater(t, pending.Count, int64(0))

    // Verify retry metadata
    log := deliveryRepo.GetByMessageID(ctx, "msg-003")
    assert.Equal(t, 1, log.RetryCount)
    assert.Contains(t, log.LastError, "429")
}
```

---

### Scenario 7: DLQ Routing on Permanent Failure

**Priority**: P0 (Critical Path)

**Given**:
- Message is queued in Redis Stream
- SendGrid provider returns 400 Bad Request (invalid recipient)
- Error classifier identifies error as permanent

**When**:
- Worker attempts send and receives 400 response
- Error classifier determines error is permanent (invalid recipient)
- Worker calls `queue.MoveToDLQ(ctx, msg, reason)`
- DLQ message includes original message + failure metadata

**Then**:
- Message moved to DLQ stream `dlq:{tenant_id}`
- Original message acknowledged and removed from primary queue
- Message status updated to "failed" in delivery_logs table
- DLQ message includes: original message, failure reason, retry history, final error
- Prometheus metric `dlq_messages_total{reason="invalid_recipient"}` incremented

**Success Metrics**:
- DLQ rate <1% of total messages
- Zero permanent failures retried

**Test Implementation**:
```go
func TestDLQRoutingOnPermanentFailure(t *testing.T) {
    // Given: Message and invalid recipient error
    mockSendGrid := &MockSendGridAPI{
        Response: &sendgrid.Response{
            StatusCode: 400,
            Body:       `{"errors":[{"message":"invalid recipient"}]}`,
        },
    }
    provider := provider.NewSendGridProvider(mockSendGrid)
    errorClassifier := provider.NewErrorClassifier()

    msg := &queue.Message{
        ID:       "msg-004",
        TenantID: "tenant-001",
        To:       []string{"invalid@invalid"},
    }

    // When: Send fails with permanent error
    result, err := provider.Send(ctx, msg)
    assert.Error(t, err)

    isPermanent := errorClassifier.IsPermanent(err)
    assert.True(t, isPermanent)

    // Move to DLQ
    err = queueProducer.MoveToDLQ(ctx, msg, "invalid recipient")
    assert.NoError(t, err)

    // Then: Verify DLQ entry
    dlqEntries := redisClient.XRange(ctx, "dlq:tenant-001", "-", "+").Val()
    assert.Len(t, dlqEntries, 1)

    var dlqMsg queue.DLQMessage
    json.Unmarshal([]byte(dlqEntries[0].Values["data"].(string)), &dlqMsg)
    assert.Equal(t, "invalid recipient", dlqMsg.FailureReason)

    // Verify delivery log
    log := deliveryRepo.GetByMessageID(ctx, "msg-004")
    assert.Equal(t, "failed", log.Status)
}
```

---

### Scenario 8: ESP Provider Failover

**Priority**: P1 (High)

**Given**:
- Message is queued in Redis Stream
- Routing rule specifies: Primary=SendGrid, Fallback=[SES, Mailgun]
- SendGrid provider is unhealthy (3 consecutive failures detected)
- AWS SES provider is healthy

**When**:
- Worker resolves provider via routing engine
- Routing engine detects SendGrid is unhealthy
- Routing engine selects first healthy fallback → AWS SES
- Worker calls `sesProvider.Send(ctx, msg)`
- SES API returns success

**Then**:
- Message sent via AWS SES instead of SendGrid
- Message status updated to "sent" with provider="ses"
- Provider message ID from SES stored
- Prometheus metric `provider_failover_total{from="sendgrid",to="ses"}` incremented
- Log entry indicates failover reason

**Success Metrics**:
- Failover latency <1s
- Failover success rate >95%

**Test Implementation**:
```go
func TestESPProviderFailover(t *testing.T) {
    // Given: Unhealthy SendGrid and healthy SES
    healthChecker := &MockHealthChecker{
        Status: map[string]bool{
            "sendgrid": false, // unhealthy
            "ses":      true,  // healthy
            "mailgun":  true,
        },
    }

    routingEngine := routing.NewEngine(healthChecker)
    rule := &routing.RoutingRule{
        TenantID:        "tenant-001",
        PrimaryProvider: "sendgrid",
        FallbackOrder:   []string{"ses", "mailgun"},
    }

    msg := &queue.Message{
        ID:       "msg-005",
        TenantID: "tenant-001",
    }

    // When: Resolve provider
    providerName := routingEngine.ResolveProvider(ctx, rule)

    // Then: Verify failover to SES
    assert.Equal(t, "ses", providerName)

    // Send via SES
    sesProvider := providerRegistry.Get("ses")
    result, err := sesProvider.Send(ctx, msg)
    assert.NoError(t, err)

    // Verify delivery log
    log := deliveryRepo.GetByMessageID(ctx, "msg-005")
    assert.Equal(t, "sent", log.Status)
    assert.Equal(t, "ses", log.Provider)
}
```

---

### Scenario 9: Delivery Status Webhook Processing

**Priority**: P1 (High)

**Given**:
- Message was sent via SendGrid with provider message ID "sg-msg-123"
- SendGrid webhook configured to POST to `/api/v1/webhooks/sendgrid`
- Webhook payload contains bounce event

**When**:
- SendGrid sends webhook POST request with JSON payload:
  ```json
  {
    "event": "bounce",
    "sg_message_id": "sg-msg-123",
    "email": "recipient@example.com",
    "reason": "mailbox full",
    "timestamp": 1676400000
  }
  ```
- Webhook handler verifies SendGrid signature (HMAC)
- Webhook handler parses event and extracts message ID
- Webhook handler calls `deliveryService.UpdateStatus(ctx, messageID, "bounced")`

**Then**:
- Delivery log status updated to "bounced"
- Bounce metadata stored in JSONB field
- Notification triggered (if configured)
- Webhook responds with 200 OK to SendGrid
- Prometheus metric `webhook_events_total{provider="sendgrid",event="bounce"}` incremented

**Success Metrics**:
- Webhook processing latency <500ms
- Webhook verification success rate 100%

**Test Implementation**:
```go
func TestDeliveryStatusWebhookProcessing(t *testing.T) {
    // Given: Message in database
    existingLog := &repository.DeliveryLog{
        MessageID:         "msg-006",
        Status:            "sent",
        Provider:          "sendgrid",
        ProviderMessageID: "sg-msg-123",
    }
    deliveryRepo.Create(ctx, existingLog)

    // When: Webhook received
    webhookPayload := `{
        "event": "bounce",
        "sg_message_id": "sg-msg-123",
        "email": "recipient@example.com",
        "reason": "mailbox full",
        "timestamp": 1676400000
    }`

    req := httptest.NewRequest("POST", "/api/v1/webhooks/sendgrid",
        strings.NewReader(webhookPayload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-SendGrid-Signature", generateSignature(webhookPayload))

    resp := httptest.NewRecorder()
    webhookHandler.HandleSendGrid(resp, req)

    // Then: Verify status update
    assert.Equal(t, 200, resp.Code)

    updatedLog := deliveryRepo.GetByProviderMessageID(ctx, "sendgrid", "sg-msg-123")
    assert.Equal(t, "bounced", updatedLog.Status)
    assert.Contains(t, updatedLog.Metadata["bounce_reason"], "mailbox full")
}
```

---

### Scenario 10: DLQ Manual Reprocessing

**Priority**: P2 (Medium)

**Given**:
- Message "msg-007" exists in DLQ stream `dlq:tenant-001`
- DLQ message includes original message and failure metadata
- Issue causing failure has been resolved (e.g., recipient email fixed)

**When**:
- Admin calls DLQ reprocess API: `POST /api/v1/dlq/reprocess`
  ```json
  {
    "message_ids": ["msg-007"],
    "reset_retry_count": true
  }
  ```
- API handler validates message IDs exist in DLQ
- API handler calls `queue.ReprocessFromDLQ(ctx, messageIDs)`
- ReprocessFromDLQ removes message from DLQ stream
- ReprocessFromDLQ resets retry count to 0
- ReprocessFromDLQ enqueues message to primary queue

**Then**:
- Message removed from `dlq:tenant-001` stream
- Message re-added to `queue:tenant-001` stream with retry_count=0
- Delivery log status updated to "queued"
- API responds with 200 OK and reprocess count
- Worker picks message and attempts delivery

**Success Metrics**:
- Reprocess success rate >90%
- Reprocess API latency <1s

**Test Implementation**:
```go
func TestDLQManualReprocessing(t *testing.T) {
    // Given: Message in DLQ
    dlqMsg := &queue.DLQMessage{
        OriginalMessage: &queue.Message{
            ID:         "msg-007",
            TenantID:   "tenant-001",
            From:       "sender@example.com",
            To:         []string{"fixed@example.com"},
        },
        FailureReason: "invalid recipient",
        RetryHistory:  []queue.RetryAttempt{{Attempt: 1, Error: "400"}},
    }

    dlqData, _ := json.Marshal(dlqMsg)
    redisClient.XAdd(ctx, &redis.XAddArgs{
        Stream: "dlq:tenant-001",
        Values: map[string]interface{}{"data": dlqData},
    })

    // When: Reprocess API called
    reqBody := `{"message_ids":["msg-007"],"reset_retry_count":true}`
    req := httptest.NewRequest("POST", "/api/v1/dlq/reprocess",
        strings.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")

    resp := httptest.NewRecorder()
    dlqHandler.Reprocess(resp, req)

    // Then: Verify reprocessing
    assert.Equal(t, 200, resp.Code)

    // Verify removed from DLQ
    dlqEntries := redisClient.XRange(ctx, "dlq:tenant-001", "-", "+").Val()
    assert.Len(t, dlqEntries, 0)

    // Verify re-added to primary queue
    queueEntries := redisClient.XRange(ctx, "queue:tenant-001", "-", "+").Val()
    assert.Len(t, queueEntries, 1)

    // Verify retry count reset
    var requeuedMsg queue.Message
    json.Unmarshal([]byte(queueEntries[0].Values["data"].(string)), &requeuedMsg)
    assert.Equal(t, 0, requeuedMsg.RetryCount)
}
```

---

## Edge Cases

### Edge Case 1: Queue Backend Failure During Processing

**Scenario**:
- Worker picks message from Redis Stream
- Redis connection drops during ESP API call
- Worker successfully sends message via ESP
- Worker fails to acknowledge message (XACK fails)

**Expected Behavior**:
- Message remains in Redis pending state
- Background job claims pending messages using XCLAIM
- Delivery log shows "sent" status (idempotency prevents duplicate send)
- Message eventually acknowledged by reclaim process

**Test**:
```go
func TestQueueBackendFailureDuringProcessing(t *testing.T) {
    // Simulate Redis failure after ESP success
    // Verify pending message reclaim logic
}
```

### Edge Case 2: ESP Timeout

**Scenario**:
- Worker calls `provider.Send(ctx, msg)` with 30s timeout
- ESP API does not respond within timeout
- Context deadline exceeded

**Expected Behavior**:
- Worker treats timeout as transient error
- Retry scheduled with exponential backoff
- Message NOT acknowledged (remains pending)
- Timeout logged with correlation ID

**Test**:
```go
func TestESPTimeout(t *testing.T) {
    // Mock ESP with 35s delay
    // Verify context timeout handling
}
```

### Edge Case 3: Oversized Message

**Scenario**:
- Message size exceeds ESP provider limit (e.g., 10MB for SendGrid)
- Size check occurs during enqueue phase

**Expected Behavior**:
- SMTP server rejects message with 552 error code
- Message never enters queue
- No delivery log entry created

**Test**:
```go
func TestOversizedMessage(t *testing.T) {
    // Create 15MB message
    // Verify SMTP rejection
}
```

### Edge Case 4: Concurrent Worker Processing

**Scenario**:
- Two workers claim same message simultaneously (race condition)
- Redis consumer group prevents duplicate claims

**Expected Behavior**:
- Only one worker receives message via XREADGROUP
- Other worker receives empty result
- No duplicate ESP API calls

**Test**:
```go
func TestConcurrentWorkerProcessing(t *testing.T) {
    // Launch 2 workers simultaneously
    // Verify single delivery
}
```

### Edge Case 5: Azure AD Token Expiry Mid-Batch

**Scenario**:
- Worker processes batch of 50 messages
- OAuth token valid at batch start (expires in 5 minutes)
- Token expires after processing 30 messages
- Remaining 20 messages encounter 401 Unauthorized

**Expected Behavior**:
- Token manager detects expiry on first 401 response
- Token manager refreshes token via client credentials flow
- Failed request automatically retried with new token
- Remaining messages use refreshed token
- No messages moved to DLQ due to token expiry

**Test**:
```go
func TestAzureADTokenExpiryMidBatch(t *testing.T) {
    // Simulate token expiry after 30 messages
    // Verify automatic refresh and retry
    // Verify all 50 messages delivered successfully
}
```

---

## Performance Criteria

### Throughput

**Target**: 10,000 messages/minute sustained

**Test Method**:
- Load test with 10,000 messages over 60 seconds
- Measure queue depth variation
- Verify worker pool does not saturate

**Acceptance**:
- All 10,000 messages processed within 60 seconds
- Queue depth remains <1,000 throughout test
- Worker utilization 60-80%

### End-to-End Latency

**Target**: <5 seconds (P95)

**Test Method**:
- Measure timestamp from queue enqueue to delivery log "sent" update
- Collect latency histogram across 10,000 messages
- Calculate P50, P95, P99 percentiles

**Acceptance**:
- P95 latency <5 seconds
- P50 latency <2 seconds
- P99 latency <10 seconds

### Message Loss Prevention

**Target**: Zero message loss during queue backend restart

**Test Method**:
- Enqueue 1,000 messages
- Restart Redis while workers are processing
- Verify all messages eventually processed

**Acceptance**:
- All 1,000 messages transition to "sent" or "failed" status
- No messages disappear from queue or delivery log
- Pending messages reclaimed after Redis restart

---

## Quality Gate Criteria

### Code Coverage

**Target**: 85%+ overall coverage

**Breakdown**:
- Unit tests: 90%+ for business logic
- Integration tests: 70%+ for queue and provider layers
- E2E tests: 60%+ for full workflow scenarios

**Tools**: go test -cover, go tool cover -html

### Linter Compliance

**Target**: Zero linter warnings

**Tools**:
- golangci-lint with standard configuration
- gofmt for code formatting
- go vet for suspicious constructs

### Security Audit

**Target**: No API key exposure in logs or metrics

**Verification**:
- Manual log inspection for API key patterns
- Automated log sanitization tests
- Secrets scanning with trufflehog or gitleaks

### Performance Benchmarks

**Target**: Meet all performance criteria

**Benchmarks**:
- Throughput: 10K msg/min
- Latency P95: <5s
- Queue depth: <1K
- DLQ rate: <1%

---

## Traceability Matrix

| Requirement ID | Test Scenario | Edge Case | Performance Criteria |
|---------------|---------------|-----------|---------------------|
| REQ-QUEUE-U001 | Scenario 1 | Edge Case 4 | Throughput |
| REQ-QUEUE-U002 | Scenario 7 | - | - |
| REQ-QUEUE-U003 | Scenario 2, 3 | - | - |
| REQ-QUEUE-U004 | Scenario 1 | Edge Case 1 | Message Loss |
| REQ-QUEUE-E001 | Scenario 1 | - | Throughput |
| REQ-QUEUE-E002 | Scenario 6 | - | - |
| REQ-QUEUE-E003 | Scenario 2, 3 | - | Latency |
| REQ-QUEUE-E004 | Scenario 4 | Edge Case 2 | - |
| REQ-QUEUE-E005 | Scenario 5 | - | - |
| REQ-QUEUE-E006 | Scenario 5 | - | DLQ Rate |
| REQ-QUEUE-E007 | Scenario 7 | - | - |
| REQ-QUEUE-E008 | Scenario 8 | - | - |
| REQ-QUEUE-S001 | Scenario 5 | - | DLQ Rate |
| REQ-QUEUE-S002 | Scenario 6 | - | - |
| REQ-QUEUE-N001 | - | Edge Case 1 | Message Loss |
| REQ-QUEUE-N002 | Scenario 5 | - | - |
| REQ-QUEUE-N003 | - | - | Security Audit |

---

## Definition of Done

1. All 10 test scenarios pass with automated tests (including MS Graph scenarios)
2. All 5 edge cases verified with integration tests (including Azure AD token expiry)
3. Performance criteria met: 10K msg/min, <5s P95 latency, zero message loss
4. Code coverage ≥85% with unit + integration tests
5. Zero linter warnings (golangci-lint)
6. Security audit confirms no API key or client_secret exposure
7. Operational documentation complete (runbook, monitoring setup)
8. E2E tests run successfully in Docker Compose environment
9. Manual verification of DLQ reprocess workflow
10. Peer code review completed and approved
11. Microsoft Graph OAuth token refresh verified in integration tests
12. Azure AD client credentials flow tested with mock token endpoint
