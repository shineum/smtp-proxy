# Acceptance Criteria: SPEC-GUARDRAIL-001

## Overview

This document defines detailed acceptance criteria for the Email Guardrail Plugin System using Given-When-Then format. Each scenario corresponds to requirements defined in spec.md and implementation milestones in plan.md.

---

## Acceptance Scenarios

### Scenario 1: Single Guardrail ALLOW Flow

**Related Requirements**: [TAG-ALLOW-ACTION], [TAG-GUARDRAIL-EXECUTION]

**Given**:
- A tenant has one HTTPWebhookGuardrail configured for a specific account with priority 100 and enabled status
- The guardrail server URL is `https://guardrail.example.com/check`
- The guardrail server is configured to return `{"action": "ALLOW"}` for all requests
- An email message from that account is queued for delivery with message ID `msg-12345`

**When**:
- The message is dequeued by the queue worker
- The guardrail chain executor runs

**Then**:
- The HTTPWebhookGuardrail.Check() method is called with the message
- An HTTPS POST request is sent to `https://guardrail.example.com/check` with JSON payload containing message fields (from, to, subject, body, headers)
- The guardrail server responds with `{"action": "ALLOW"}`
- The guardrail chain executor returns `GuardrailResult{Action: ActionAllow}`
- The message proceeds to ESP provider delivery
- A log entry is created with level INFO, message_id `msg-12345`, guardrail_name, and action "ALLOW"

**Validation**:
- Database message status updated to "delivered" (assuming ESP success)
- Prometheus counter `guardrail_checks_total{guardrail="...", action="ALLOW"}` incremented by 1
- Total processing latency (queue dequeue to ESP delivery) < 5 seconds p95

---

### Scenario 2: Single Guardrail REJECT Flow

**Related Requirements**: [TAG-REJECT-ACTION], [TAG-EARLY-TERMINATION]

**Given**:
- A tenant has one HTTPWebhookGuardrail configured for a specific account with priority 100 and enabled status
- The guardrail server is configured to return `{"action": "REJECT", "reason": "Spam detected by content policy"}`
- An email message from that account is queued with message ID `msg-67890`

**When**:
- The message is dequeued by the queue worker
- The guardrail chain executor runs

**Then**:
- The HTTPWebhookGuardrail.Check() method is called
- The guardrail server responds with `{"action": "REJECT", "reason": "Spam detected by content policy"}`
- The guardrail chain executor returns `GuardrailResult{Action: ActionReject, Reason: "Spam detected by content policy"}`
- The message is NOT sent to ESP provider
- The message status is updated to "rejected" in the database with the rejection reason
- A log entry is created with level WARN, message_id `msg-67890`, action "REJECT", and reason "Spam detected by content policy"

**Validation**:
- Database query `SELECT status, rejection_reason FROM messages WHERE id = 'msg-67890'` returns `status="rejected"` and `rejection_reason="Spam detected by content policy"`
- Prometheus counter `guardrail_checks_total{action="REJECT"}` incremented by 1
- No ESP API call is made (verified via ESP provider mock)

---

### Scenario 3: Guardrail MODIFY Flow (Content Replacement)

**Related Requirements**: [TAG-MODIFY-ACTION]

**Given**:
- A tenant has one HTTPWebhookGuardrail configured
- The guardrail server modifies message subject by prepending "[FILTERED]"
- The guardrail returns `{"action": "MODIFY", "modified": {"subject": "[FILTERED] Original Subject", ...}}`
- An email message has original subject "Original Subject"

**When**:
- The message is processed through the guardrail chain

**Then**:
- The guardrail server receives the original message
- The guardrail returns a MODIFY action with modified message content
- The guardrail chain executor updates the message variable with the modified content
- The modified message (subject "[FILTERED] Original Subject") is sent to the ESP provider
- A log entry records the MODIFY action with message ID

**Validation**:
- The ESP provider receives a message with subject "[FILTERED] Original Subject"
- The original message in the database remains unchanged (modification is in-flight only)
- Prometheus counter `guardrail_checks_total{action="MODIFY"}` incremented by 1

**Edge Case**: If multiple MODIFY actions occur in a chain, each subsequent guardrail receives the cumulatively modified message.

---

### Scenario 4: Chained Guardrails Execution Order

**Related Requirements**: [TAG-MULTIPLE-GUARDRAILS], [TAG-EARLY-TERMINATION], [TAG-DEQUEUE-PROCESSING]

**Given**:
- An account has 3 guardrails configured:
  - Guardrail A: Priority 100, Type HTTPWebhook, Returns ALLOW, account-specific
  - Guardrail B: Priority 200, Type SpamFilter, Returns ALLOW, account-specific
  - Guardrail C: Priority 300, Type HTTPWebhook, Returns ALLOW, tenant-wide default
- All guardrails are enabled

**When**:
- A message from that account is processed through the chain

**Then**:
- Guardrails execute in order: A (priority 100) → B (priority 200) → C (priority 300)
- Account-specific guardrails (A, B) and tenant default (C) are merged
- Each guardrail receives the message and returns ALLOW
- The message proceeds to ESP delivery after all guardrails pass
- Log entries show execution sequence with timestamps in ascending priority order

**Validation**:
- Database queries load both account-specific and tenant-wide guardrails
- Merged list sorted by priority ASC
- Log timestamps confirm sequential execution
- Prometheus histogram `guardrail_check_duration_seconds` records latency for each guardrail

**Negative Test (Early Termination)**:
- If Guardrail B returns REJECT, Guardrail C is NOT executed
- Log shows only 2 guardrail entries (A ALLOW, B REJECT)
- Prometheus counter shows only 2 checks (not 3)

---

### Scenario 5: Guardrail Server Timeout with Fallback Policy

**Related Requirements**: [TAG-TIMEOUT-EXCEEDED], [TAG-FALLBACK-POLICY]

**Given**:
- A guardrail is configured with timeout 5 seconds and fallback policy "allow"
- The guardrail server has a 10-second delay before responding (exceeds timeout)

**When**:
- A message is sent through the guardrail

**Then**:
- The HTTP request to the guardrail server is initiated
- After 5 seconds, the request context is cancelled (timeout)
- The HTTPWebhookGuardrail.Check() method returns an error: `context deadline exceeded`
- The guardrail chain executor applies the fallback policy "allow"
- The message proceeds to ESP delivery
- A log entry is created with level ERROR, error message "context deadline exceeded", and fallback policy applied

**Validation**:
- Prometheus counter `guardrail_errors_total{error_type="timeout"}` incremented by 1
- Message delivered despite guardrail timeout
- Total processing time reflects timeout duration (5 seconds + ESP delivery time)

**Negative Test (Fallback Reject)**:
- If fallback policy is "reject", the message is marked rejected on timeout
- Database status = "rejected", rejection_reason = "Guardrail timeout: fallback reject"

**Edge Case (Fallback Queue-for-Retry)**:
- If fallback policy is "queue-for-retry", the message is requeued with retry count incremented
- Maximum retry attempts apply (e.g., 3 retries before DLQ)

---

### Scenario 6: Circuit Breaker Activation on Repeated Failures

**Related Requirements**: [TAG-CIRCUIT-BREAKER], [TAG-NO-QUEUE-BLOCKING], [TAG-CIRCUIT-HALFOPEN]

**Given**:
- A guardrail is configured pointing to a URL that returns 500 Internal Server Error
- Circuit breaker settings: MaxRequests=1, Interval=60s, Timeout=30s, ReadyToTrip after 5 consecutive failures

**When**:
- 5 consecutive messages are sent through the guardrail (all fail with 500 error)
- The circuit breaker transitions to OPEN state
- After 30 seconds, the circuit transitions to HALF-OPEN state
- A 6th message is sent (test request)

**Then**:
- **Messages 1-5**: Each request fails, circuit remains CLOSED, failure counter increments
- **After 5th failure**: Circuit breaker opens (`State: Open`)
- **Messages during OPEN state**: Guardrail is bypassed, fallback policy applied immediately (no HTTP request)
- **After 30 seconds**: Circuit transitions to HALF-OPEN, allows 1 test request
- **Message 6**: If guardrail server is still unhealthy (500 error), circuit reopens; if healthy (200 OK), circuit closes

**Validation**:
- Prometheus gauge `guardrail_circuit_breaker_state{state="open"}` = 1 during open state
- Log entries show state transitions: CLOSED → OPEN → HALF-OPEN → CLOSED (if recovery successful) or OPEN (if recovery failed)
- Queue processing continues during circuit OPEN state (no blocking)
- HTTP requests stop during OPEN state (verified via network traffic monitoring)

**Edge Case**: If 3 failures occur, then 1 success, then 2 more failures, circuit does NOT open (requires 5 consecutive failures).

---

### Scenario 7: Admin CRUD Operations for Guardrail Configuration

**Related Requirements**: [TAG-CONFIG-CREATE], [TAG-HTTPS-REQUIRED], [TAG-NO-CREDENTIAL-EXPOSURE]

**Given**:
- An admin user is authenticated with tenant ID `tenant-abc`
- The admin has a valid JWT token

**When (Create)**:
- The admin sends POST /api/v1/guardrails with JSON body:
  ```json
  {
    "name": "content-policy",
    "type": "http_webhook",
    "account_id": "account-123",
    "config": {
      "url": "https://guardrail.example.com/check",
      "timeout_seconds": 5,
      "headers": {
        "Authorization": "Bearer secret_abc123"
      }
    },
    "priority": 100,
    "enabled": true,
    "fallback_policy": "reject"
  }
  ```

**Then (Create)**:
- The server validates the request:
  - URL starts with `https://` (HTTPS required)
  - Priority is between 0-1000
  - Fallback policy is one of: allow, reject, queue-for-retry
  - If account_id is provided, verify it belongs to the authenticated tenant
- The guardrail is inserted into the database with tenant_id `tenant-abc` and account_id `account-123`
- Response 201 Created with JSON body including guardrail ID
- **Credential redaction**: Response `config.headers.Authorization` field is `"***REDACTED***"`

**Validation**:
- Database query `SELECT * FROM guardrails WHERE tenant_id = 'tenant-abc' AND account_id = 'account-123' AND name = 'content-policy'` returns 1 row
- Response does not contain `"Bearer secret_abc123"` (credential redacted)

**When (List)**:
- Admin sends GET /api/v1/guardrails

**Then (List)**:
- Response 200 OK with array of guardrails for `tenant-abc` only
- All guardrails have redacted credentials in `config` field

**When (Update)**:
- Admin sends PUT /api/v1/guardrails/{id} with updated priority 200

**Then (Update)**:
- Guardrail priority updated to 200 in database
- Response 200 OK with updated guardrail

**When (Delete)**:
- Admin sends DELETE /api/v1/guardrails/{id}

**Then (Delete)**:
- Guardrail deleted from database
- Response 204 No Content

**Negative Test (HTTP URL Rejection)**:
- Admin attempts to create guardrail with `"url": "http://insecure.example.com"`
- Response 400 Bad Request with error message: `"HTTPS required for guardrail server URLs"`

**Negative Test (Cross-Tenant Access)**:
- Admin with tenant `tenant-abc` attempts to update guardrail belonging to `tenant-xyz`
- Response 404 Not Found (tenant check in WHERE clause prevents access)

---

### Scenario 8: Guardrail Test Endpoint

**Related Requirements**: [TAG-TEST-ENDPOINT]

**Given**:
- A guardrail is configured with ID `guardrail-123`
- The guardrail server URL is `https://guardrail.example.com/check`
- The guardrail server returns `{"action": "ALLOW"}` for test messages

**When**:
- Admin sends POST /api/v1/guardrails/guardrail-123/test with sample message:
  ```json
  {
    "from": "test@example.com",
    "to": ["recipient@example.com"],
    "subject": "Test Message",
    "body": "This is a test."
  }
  ```

**Then**:
- The server creates a temporary Message struct with the provided data
- The guardrail's Check() method is called with the test message
- The guardrail server is contacted via HTTPS POST
- Response 200 OK with result:
  ```json
  {
    "action": "ALLOW",
    "reason": "",
    "latency_ms": 234
  }
  ```

**Validation**:
- No message is actually queued or delivered (test is isolated)
- Latency field reflects actual guardrail server response time
- Admin can verify guardrail configuration before enabling

**Negative Test (Timeout)**:
- Guardrail server delays response beyond timeout
- Response 200 OK with:
  ```json
  {
    "error": "Guardrail timeout after 5s",
    "fallback_policy": "reject"
  }
  ```

**Edge Case (MODIFY Action)**:
- Guardrail returns MODIFY with modified subject
- Response includes modified message content for admin review

---

### Scenario 9: Per-Account and Per-Tenant Guardrail Isolation

**Related Requirements**: [TAG-TENANT-ISOLATION], [TAG-NO-CROSS-TENANT]

**Given**:
- Tenant A (`tenant-aaa`) has 2 account-specific guardrails for account-1 and 1 tenant-wide default guardrail
- Tenant B (`tenant-bbb`) has 3 guardrails configured
- Admin user A is authenticated with tenant ID `tenant-aaa`

**When**:
- Admin A sends GET /api/v1/guardrails?account_id=account-1 (list guardrails for specific account)

**Then**:
- Response contains account-specific guardrails for account-1 plus tenant-wide defaults
- Tenant B's guardrails are NOT included in the response
- Database query includes `WHERE (account_id = 'account-1' OR (account_id IS NULL AND tenant_id = 'tenant-aaa'))` clause

**Validation**:
- Database foreign key constraints enforce referential integrity:
  - `guardrails.tenant_id → tenants.id`
  - `guardrails.account_id → accounts.id`
- Attempting to insert guardrail with non-existent tenant ID or account ID fails with foreign key violation

**Negative Test (Cross-Tenant Update Attempt)**:
- Admin A retrieves guardrail ID belonging to Tenant B via SQL injection attempt or direct ID guessing
- Admin A sends PUT /api/v1/guardrails/{tenant-b-guardrail-id}
- Response 404 Not Found (UPDATE query includes `WHERE tenant_id = 'tenant-aaa'` which matches zero rows)
- Tenant B's guardrail remains unchanged

**Negative Test (Cross-Account Access)**:
- Admin A from tenant-aaa attempts to create a guardrail with account_id belonging to tenant-bbb
- Response 403 Forbidden (account ownership validation fails)
- Tenant B's accounts remain unaffected

**Negative Test (Guardrail Plugin Access)**:
- A guardrail plugin receives a Message object during Check()
- The plugin attempts to access database to query other tenants' data
- **Mitigation**: Guardrail plugins receive only the message, not database connection or tenant context
- Plugins are stateless and cannot access external data sources

---

### Scenario 10: No Guardrails Configured (Passthrough)

**Related Requirements**: [TAG-NO-GUARDRAILS]

**Given**:
- An account has zero account-specific guardrails configured
- The tenant has zero tenant-wide default guardrails configured
- An email message from that account is queued for delivery

**When**:
- The queue worker dequeues the message
- The guardrail chain executor is invoked

**Then**:
- Database queries return 0 rows:
  - `SELECT * FROM guardrails WHERE account_id = '...' AND enabled = true`
  - `SELECT * FROM guardrails WHERE account_id IS NULL AND tenant_id = '...' AND enabled = true`
- The guardrail chain executor immediately returns `GuardrailResult{Action: ActionAllow}` without executing any checks
- The message proceeds directly to ESP delivery
- Total processing latency is minimized (no guardrail overhead)

**Validation**:
- Prometheus histogram `guardrail_check_duration_seconds` does NOT record any samples for this message
- Log entry shows: "No guardrails configured for account or tenant, skipping guardrail processing"
- Performance test confirms < 10ms overhead for guardrail system when no guardrails configured

---

### Scenario 11: Account-Specific Guardrail Overrides Tenant Default

**Related Requirements**: [TAG-TENANT-ISOLATION], [TAG-MULTIPLE-GUARDRAILS]

**Given**:
- Tenant has a tenant-wide default guardrail "default-spam-filter" with priority 100 (account_id IS NULL)
- Account A has an account-specific guardrail "account-spam-filter" with priority 100 and same name
- Both guardrails are enabled
- An email message from Account A is queued

**When**:
- The message is processed through the guardrail chain

**Then**:
- System loads both tenant-wide and account-specific guardrails
- Account-specific guardrail "account-spam-filter" takes precedence over tenant default
- Only the account-specific guardrail executes (tenant default is overridden)
- Message proceeds according to account-specific guardrail decision

**Validation**:
- Resolution logic merges account-specific and tenant-wide guardrails
- Account-specific guardrails with same name/priority override tenant defaults
- Log shows only account-specific guardrail execution
- Prometheus metrics track account_id in labels

---

### Scenario 12: Tenant-Wide Default Applies When No Account-Specific Guardrail

**Related Requirements**: [TAG-TENANT-ISOLATION], [TAG-NO-GUARDRAILS]

**Given**:
- Tenant has a tenant-wide default guardrail "default-content-policy" (account_id IS NULL)
- Account B has zero account-specific guardrails
- An email message from Account B is queued

**When**:
- The message is processed through the guardrail chain

**Then**:
- System loads tenant-wide default guardrails only (no account-specific guardrails found)
- Tenant default "default-content-policy" executes for Account B
- Message proceeds according to tenant default guardrail decision

**Validation**:
- Database query for account-specific guardrails returns 0 rows
- Database query for tenant-wide defaults returns 1 row
- Tenant default guardrail executes successfully
- Log shows tenant default guardrail execution with account context

---

### Scenario 13: Account Deleted But Guardrail Config Remains (Edge Case)

**Related Requirements**: [TAG-TENANT-ISOLATION]

**Given**:
- Account X had account-specific guardrails configured
- Account X is deleted from the system
- ON DELETE CASCADE removes all guardrails with account_id = X

**When**:
- Database constraint triggers cascade delete

**Then**:
- All guardrails with account_id = X are automatically deleted
- No orphaned guardrail configurations remain
- Foreign key constraint ensures referential integrity

**Validation**:
- Database query `SELECT * FROM guardrails WHERE account_id = 'deleted-account-id'` returns 0 rows
- Foreign key constraint `guardrails.account_id → accounts.id ON DELETE CASCADE` enforced
- No manual cleanup required

---

## Edge Cases

### Edge Case 1: Large Message Attachments

**Scenario**:
- Email message has 20MB attachment (approaching 25MB limit)
- Guardrail server accepts messages up to 25MB

**Expected Behavior**:
- Message is successfully serialized to JSON with base64-encoded attachment
- HTTP POST request completes within timeout
- Guardrail server processes message without error

**Failure Mode**:
- If attachment exceeds 25MB, message is rejected during serialization
- Error logged: "Message size exceeds guardrail limit"
- Fallback policy NOT applied (this is a validation error, not server failure)

### Edge Case 2: Slow Guardrail Server (Within Timeout)

**Scenario**:
- Guardrail server responds in 4.8 seconds (just under 5-second timeout)
- Multiple messages queued simultaneously

**Expected Behavior**:
- All requests complete successfully
- Queue processing slows due to guardrail latency
- Prometheus histogram records high p95 latency
- Alert triggers if p95 > 3 seconds sustained

**Mitigation**:
- Admin receives alert to investigate slow guardrail server
- Consider reducing timeout or disabling guardrail

### Edge Case 3: Malformed Guardrail Server Response

**Scenario**:
- Guardrail server returns invalid JSON: `{"action": "INVALID_ACTION"}`
- Action value is not one of: ALLOW, REJECT, MODIFY

**Expected Behavior**:
- JSON unmarshal succeeds but validation fails
- Error logged: "Invalid guardrail action: INVALID_ACTION"
- Fallback policy applied (treat as server error)

**Validation**:
- Prometheus counter `guardrail_errors_total{error_type="invalid_response"}` incremented

### Edge Case 4: Concurrent Requests to Same Guardrail

**Scenario**:
- 100 messages dequeued simultaneously, all routed through same guardrail
- Guardrail server has rate limiting: 10 requests/second

**Expected Behavior**:
- HTTP client respects connection pooling (default 100 concurrent connections)
- Guardrail server returns 429 Too Many Requests for excess requests
- 429 errors treated as server failures, fallback policy applied
- Circuit breaker may open if 5+ consecutive 429 responses

**Mitigation**:
- Configure guardrail timeout to account for rate limiting
- Use queue-for-retry fallback policy for 429 errors
- Implement client-side rate limiting (future enhancement)

### Edge Case 5: Guardrail Configuration Changes During Processing

**Scenario**:
- Admin disables a guardrail while messages are in-flight
- Queue worker has cached guardrail configuration

**Expected Behavior**:
- Guardrail configuration cache TTL is 5 minutes
- Cache key includes account_id for account-level isolation
- Messages processed within cache window use old configuration (guardrail still enabled)
- After cache expiration, new messages use updated configuration (guardrail disabled)

**Validation**:
- Cache invalidation occurs on configuration update (future enhancement)
- Log entries show configuration version used for each message
- Cache key format: `guardrails:{tenant_id}:{account_id}`

---

### Edge Case 6: Account with Mixed Account-Specific and Tenant-Wide Guardrails

**Scenario**:
- Tenant has 2 tenant-wide default guardrails (priority 100, 300)
- Account has 1 account-specific guardrail (priority 200)
- Message from account should execute all 3 guardrails in priority order

**Expected Behavior**:
- System loads both account-specific and tenant-wide guardrails
- Merged list contains all 3 guardrails sorted by priority (100 → 200 → 300)
- All guardrails execute in correct order

**Validation**:
- Resolution logic correctly merges both sets
- Execution order follows priority regardless of source (account vs tenant)
- Log shows all 3 guardrails executed

---

## Performance Acceptance Criteria

### Latency Targets

- **Guardrail Processing Overhead**: < 500ms added latency at p95
  - Baseline (no guardrails): 200ms queue dequeue to ESP delivery
  - With 1 guardrail: < 700ms total (200ms + 500ms overhead)
  - With 3 guardrails (sequential): < 1500ms total (200ms + 3 * 400ms)

- **Guardrail Server Response Time**: < 3 seconds at p95
  - Alert threshold: p95 > 3 seconds for 5 minutes
  - Timeout threshold: 5 seconds (hard limit)

- **Database Configuration Load**: < 10ms at p95
  - Query: `SELECT * FROM guardrails WHERE tenant_id = ? AND enabled = true ORDER BY priority`
  - Indexed query with composite index on (tenant_id, enabled, priority)

### Throughput Targets

- **Message Processing Rate**: 10,000 messages/minute with guardrails enabled
  - Per-guardrail capacity: 10,000 requests/minute (assumes 1-second average response time)
  - System scales horizontally with multiple queue workers

- **Concurrent Guardrail Requests**: 100 concurrent requests per guardrail
  - HTTP client connection pooling: 100 max idle connections
  - No goroutine leaks under load (verified via pprof)

### Resource Utilization

- **Memory Usage**: < 100MB additional memory per guardrail chain executor
  - Message size average: 50KB
  - 100 concurrent messages: 5MB message data + 95MB overhead

- **CPU Usage**: < 10% CPU per guardrail chain (single core)
  - JSON serialization/deserialization
  - HTTP client overhead
  - Circuit breaker state management

---

## Quality Gate Criteria

### TRUST 5 Framework Compliance

**Tested**:
- [x] Unit test coverage ≥ 85% for all guardrail components
- [x] Integration tests cover all 10 acceptance scenarios
- [x] Performance tests validate latency and throughput targets
- [x] Edge case tests for large messages, concurrent requests, malformed responses

**Readable**:
- [x] golangci-lint passes with zero warnings
- [x] Code comments explain circuit breaker state transitions
- [x] API documentation generated from OpenAPI annotations
- [x] Admin UI includes guardrail configuration tooltips

**Unified**:
- [x] Consistent error response format across all API endpoints
- [x] Standardized JSON logging with zerolog
- [x] Prometheus metric naming follows convention: `guardrail_<metric>_<unit>`

**Secured**:
- [x] HTTPS enforcement for all guardrail server URLs (HTTP blocked)
- [x] Credential redaction in API responses and logs
- [x] Tenant isolation validated with cross-tenant access tests
- [x] gosec security scanner passes with zero high-severity issues

**Trackable**:
- [x] All guardrail decisions logged with message ID, timestamp, action
- [x] Prometheus metrics exported for all guardrail operations
- [x] Circuit breaker state changes logged
- [x] Admin audit log for guardrail configuration changes

---

## Definition of Done

A feature is considered complete when:

1. **Functional Requirements Met**:
   - All 26 EARS requirements from spec.md are implemented
   - All 10 acceptance scenarios pass integration tests
   - Edge cases handled gracefully

2. **Quality Gates Passed**:
   - TRUST 5 framework compliance verified
   - Unit test coverage ≥ 85%
   - golangci-lint and gosec pass
   - Performance targets met (p95 latency < 500ms)

3. **Documentation Complete**:
   - API documentation published
   - Admin user guide available
   - Architecture diagram updated with guardrail integration
   - Troubleshooting guide created

4. **Deployment Ready**:
   - Database migrations tested (up/down)
   - Feature flags configured
   - Monitoring dashboards created
   - Alerting rules configured

5. **User Acceptance**:
   - Product owner approves guardrail configuration UI
   - Test endpoint validated by QA team
   - Performance benchmarks reviewed by DevOps

---

## Acceptance Test Execution Checklist

### Pre-Deployment Testing (Staging Environment)

- [ ] Scenario 1: Single guardrail ALLOW flow ✅
- [ ] Scenario 2: Single guardrail REJECT flow ✅
- [ ] Scenario 3: Guardrail MODIFY flow ✅
- [ ] Scenario 4: Chained guardrails execution order ✅
- [ ] Scenario 5: Timeout with fallback policy ✅
- [ ] Scenario 6: Circuit breaker activation and recovery ✅
- [ ] Scenario 7: Admin CRUD operations ✅
- [ ] Scenario 8: Test endpoint validation ✅
- [ ] Scenario 9: Per-account and per-tenant isolation ✅
- [ ] Scenario 11: Account-specific guardrail overrides tenant default ✅
- [ ] Scenario 12: Tenant-wide default applies when no account-specific guardrail ✅
- [ ] Scenario 13: Account deleted but guardrail config cascade delete ✅
- [ ] Scenario 10: No guardrails passthrough ✅

### Edge Case Testing

- [ ] Large message attachments (20MB) ✅
- [ ] Slow guardrail server (4.8s response) ✅
- [ ] Malformed server response ✅
- [ ] Concurrent requests to same guardrail ✅
- [ ] Configuration changes during processing ✅
- [ ] Account with mixed account-specific and tenant-wide guardrails ✅

### Performance Testing

- [ ] Guardrail latency p95 < 500ms ✅
- [ ] Database configuration load p95 < 10ms ✅
- [ ] System handles 10K messages/minute ✅
- [ ] Memory usage < 100MB per executor ✅

### Security Testing

- [ ] HTTP URL rejection ✅
- [ ] Credential redaction in responses ✅
- [ ] Cross-tenant access prevention ✅
- [ ] gosec scan passes ✅

### Post-Deployment Validation (Production)

- [ ] Monitor guardrail latency for 24 hours
- [ ] Verify circuit breaker does not open under normal load
- [ ] Check Prometheus metrics dashboard
- [ ] Review log entries for errors
- [ ] Validate tenant isolation with production data

---

**Acceptance Criteria Version**: 1.1.0
**Related SPEC**: SPEC-GUARDRAIL-001
**Approved By**: Product Owner (pending)
**Test Execution Date**: TBD (post-implementation)
