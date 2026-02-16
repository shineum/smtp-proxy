---
id: SPEC-GUARDRAIL-001
version: "1.1.0"
status: approved
created: "2026-02-15"
updated: "2026-02-16"
author: sungwon
priority: P1
---

# SPEC-GUARDRAIL-001: Email Guardrail Plugin System

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-02-15 | sungwon | Initial SPEC creation |
| 1.1.0 | 2026-02-16 | sungwon | Added per-account guardrail configuration support |

---

## Environment

### System Context

The Email Guardrail Plugin System is a middleware component within the smtp-proxy multi-tenant architecture. It operates between the SMTP message acceptance layer and the ESP provider delivery layer, providing content inspection, policy enforcement, and message filtering capabilities. Guardrails are configured at the account level (with tenant-wide defaults), allowing different SMTP accounts under the same tenant to have different guardrail configurations.

### Architecture Integration

The guardrail system integrates with the following smtp-proxy components:

- **Message Queue Worker**: Receives dequeued messages for guardrail processing
- **Database Layer**: Stores guardrail configuration per account (with tenant-wide defaults) via PostgreSQL
- **Admin Web UI**: Provides guardrail configuration interface using Next.js 14+
- **ESP Provider Layer**: Receives processed messages after guardrail approval
- **Monitoring System**: Logs all guardrail decisions for audit and analytics

### Technology Stack Alignment

Per `.moai/project/tech.md` constitution:

- **Backend**: Go 1.21+ with chi HTTP router
- **Database**: PostgreSQL 15+ with pgx driver
- **Frontend**: Next.js 14+ App Router with shadcn/ui components
- **Validation**: go-playground/validator/v10 for struct validation
- **Logging**: zerolog for structured JSON logging
- **Metrics**: Prometheus client_golang for metrics export

### External Dependencies

- **Guardrail Servers**: External HTTP/HTTPS endpoints for content inspection
- **HTTP Client**: Standard library net/http with timeout configuration
- **Circuit Breaker**: gobreaker library for failure protection
- **Retry Logic**: Exponential backoff with maximum retry limits

---

## Assumptions

### Operational Assumptions

1. **Guardrail Server Availability**: External guardrail servers may be temporarily unavailable (confidence: HIGH)
   - Evidence: Network partitions and service outages are common in distributed systems
   - Risk if wrong: System becomes dependent on external service uptime
   - Validation: Circuit breaker pattern required for graceful degradation

2. **Response Time Requirements**: Guardrail servers respond within 5 seconds for 95% of requests (confidence: MEDIUM)
   - Evidence: Industry standard for API response times
   - Risk if wrong: Queue processing latency exceeds SLA targets
   - Validation: Timeout configuration per guardrail server with fallback policy

3. **HTTPS Requirement**: All guardrail servers support HTTPS for secure communication (confidence: HIGH)
   - Evidence: Security best practice for transmitting email content
   - Risk if wrong: Email content exposed during transit
   - Validation: Reject HTTP-only configurations during setup

4. **Message Size Limits**: Guardrail servers accept messages up to 25MB (confidence: MEDIUM)
   - Evidence: Common ESP provider limit
   - Risk if wrong: Large attachments fail guardrail processing
   - Validation: Message size validation before guardrail submission

### Technical Assumptions

5. **JSON-Based API**: Guardrail servers accept and return JSON payloads (confidence: HIGH)
   - Evidence: REST API standard
   - Risk if wrong: Protocol mismatch requires custom adapters
   - Validation: Document API contract in SPEC

6. **Idempotency**: Guardrail servers produce consistent results for identical messages (confidence: MEDIUM)
   - Evidence: Content-based filtering should be deterministic
   - Risk if wrong: Retry attempts produce different outcomes
   - Validation: Log inconsistent results for investigation

7. **Plugin Interface Stability**: Guardrail plugin interface remains stable across versions (confidence: HIGH)
   - Evidence: Go interface compatibility guarantees
   - Risk if wrong: Breaking changes require all plugin rewrites
   - Validation: Semantic versioning for interface changes

### Business Assumptions

8. **Per-Account Configuration**: Each account configures guardrails independently, with tenant-wide defaults (confidence: HIGH)
   - Evidence: Multi-tenant isolation requirement from product.md and need for account-level customization
   - Risk if wrong: Cross-tenant configuration leaks or inflexible per-account customization
   - Validation: Database row-level tenant isolation with account_id nullable column for tenant defaults

9. **Priority-Based Execution**: Guardrails execute in priority order (lowest number first) (confidence: HIGH)
   - Evidence: Common pattern for ordered middleware
   - Risk if wrong: Unpredictable guardrail execution sequence
   - Validation: Database query ORDER BY priority ASC

10. **Fallback Policy Configuration**: Tenants specify behavior when guardrail server is unreachable (confidence: MEDIUM)
    - Evidence: Different risk tolerances across tenants
    - Risk if wrong: Hard-coded fallback behavior unsuitable for all use cases
    - Validation: Fallback policy configuration field in database

---

## Requirements

### Ubiquitous Requirements (Always Active)

**EARS Pattern**: The [system] **shall** [response].

1. **Guardrail Chain Execution**: The system **shall always** execute the configured guardrail chain before ESP provider delivery for every message.
   - Rationale: Ensures no message bypasses content inspection
   - Traceability: [TAG-GUARDRAIL-EXECUTION]

2. **Decision Logging**: The system **shall always** log guardrail decisions including message ID, guardrail name, action taken, and timestamp.
   - Rationale: Audit trail for compliance and debugging
   - Traceability: [TAG-GUARDRAIL-LOGGING]

3. **Plugin Interface Compliance**: The guardrail plugin interface **shall always** support ALLOW, REJECT, and MODIFY response actions.
   - Rationale: Provides complete control over message processing
   - Traceability: [TAG-PLUGIN-INTERFACE]

4. **Tenant Isolation**: The system **shall always** enforce per-tenant guardrail configuration isolation preventing cross-tenant access, with support for per-account guardrail configuration within a tenant.
   - Rationale: Multi-tenant security requirement with account-level customization
   - Traceability: [TAG-TENANT-ISOLATION]

### Event-Driven Requirements (Trigger-Response)

**EARS Pattern**: **WHEN** [event], the [system] **shall** [response].

5. **Message Dequeue Processing**: **WHEN** a message is dequeued for processing, **THEN** the system **shall** load account-specific guardrails (if configured) or tenant-wide default guardrails and execute the chain in priority order (lowest number first).
   - Rationale: Predictable middleware execution sequence with account-level resolution
   - Traceability: [TAG-DEQUEUE-PROCESSING]

6. **ALLOW Action Handling**: **WHEN** a guardrail server returns ALLOW, **THEN** the system **shall** proceed to the next guardrail in the chain or ESP delivery if no remaining guardrails.
   - Rationale: Successful guardrail approval continues processing
   - Traceability: [TAG-ALLOW-ACTION]

7. **REJECT Action Handling**: **WHEN** a guardrail server returns REJECT, **THEN** the system **shall** mark the message as "rejected" with the provided reason, skip ESP delivery, and stop chain execution.
   - Rationale: Any rejection terminates processing immediately
   - Traceability: [TAG-REJECT-ACTION]

8. **MODIFY Action Handling**: **WHEN** a guardrail server returns MODIFY, **THEN** the system **shall** replace the message content with the modified version and continue chain execution with the updated content.
   - Rationale: Content transformation support for policy enforcement
   - Traceability: [TAG-MODIFY-ACTION]

9. **Guardrail Configuration Creation**: **WHEN** an admin creates a guardrail configuration via POST /api/v1/guardrails, **THEN** the system **shall** validate the server URL (HTTPS required), store the configuration with tenant association and optional account association, and return the created resource.
   - Rationale: Secure configuration management with account-level granularity
   - Traceability: [TAG-CONFIG-CREATE]

10. **Guardrail Test Endpoint**: **WHEN** an admin tests a guardrail via POST /api/v1/guardrails/{id}/test, **THEN** the system **shall** send a sample message to the guardrail server and return the result (ALLOW/REJECT/MODIFY) within the configured timeout.
    - Rationale: Validation before production deployment
    - Traceability: [TAG-TEST-ENDPOINT]

11. **Server Unreachable Fallback**: **WHEN** a guardrail server is unreachable (connection timeout, DNS failure, or network error), **THEN** the system **shall** apply the configured fallback policy (allow/reject/queue-for-retry).
    - Rationale: Graceful degradation during failures
    - Traceability: [TAG-FALLBACK-POLICY]

12. **Circuit Breaker Activation**: **WHEN** a guardrail server exceeds the failure threshold (5 consecutive failures within 60 seconds), **THEN** the system **shall** open the circuit breaker and apply the fallback policy for all requests until the circuit resets.
    - Rationale: Prevents cascading failures from unhealthy servers
    - Traceability: [TAG-CIRCUIT-BREAKER]

### State-Driven Requirements (Conditional Behavior)

**EARS Pattern**: **IF** [condition], **THEN** the [system] **shall** [response].

13. **No Guardrails Configured**: **IF** no guardrails are configured for an account or tenant, **THEN** the system **shall** skip guardrail processing entirely and proceed directly to ESP delivery.
    - Rationale: Zero overhead for accounts/tenants not using guardrails
    - Traceability: [TAG-NO-GUARDRAILS]

14. **Timeout Compliance**: **IF** a guardrail server responds within the configured timeout (default 5 seconds), **THEN** the system **shall** use the guardrail response.
    - Rationale: Timely responses processed normally
    - Traceability: [TAG-TIMEOUT-SUCCESS]

15. **Timeout Exceeded**: **IF** a guardrail server exceeds the configured timeout, **THEN** the system **shall** cancel the request and apply the configured fallback policy.
    - Rationale: Prevents queue processing delays
    - Traceability: [TAG-TIMEOUT-EXCEEDED]

16. **Multiple Guardrails Execution**: **IF** multiple guardrails are configured for an account (or tenant default), **THEN** the system **shall** execute them in priority order (lowest number first) until all complete or a REJECT action occurs.
    - Rationale: Ordered pipeline processing with account-level configuration
    - Traceability: [TAG-MULTIPLE-GUARDRAILS]

17. **Early Termination on Reject**: **IF** any guardrail in the chain returns REJECT, **THEN** the system **shall** immediately stop chain execution and skip remaining guardrails.
    - Rationale: Rejection is final, no further processing needed
    - Traceability: [TAG-EARLY-TERMINATION]

18. **Circuit Breaker Half-Open State**: **IF** the circuit breaker is in half-open state, **THEN** the system **shall** allow one test request through and close the circuit on success or reopen on failure.
    - Rationale: Automatic recovery from transient failures
    - Traceability: [TAG-CIRCUIT-HALFOPEN]

### Unwanted Requirements (Prohibited Actions)

**EARS Pattern**: The [system] **shall not** [prohibited action].

19. **Unencrypted Communication**: The system **shall not** send email content to guardrail servers over unencrypted HTTP connections (HTTPS required).
    - Rationale: Email content contains sensitive information
    - Traceability: [TAG-HTTPS-REQUIRED]

20. **Queue Blocking on Permanent Failure**: The system **shall not** block queue processing indefinitely when a guardrail server is permanently down (circuit breaker prevents this).
    - Rationale: System availability must not depend on external services
    - Traceability: [TAG-NO-QUEUE-BLOCKING]

21. **Cross-Tenant Configuration Access**: The system **shall not** allow guardrail plugins to access configuration or data belonging to other tenants.
    - Rationale: Multi-tenant security boundary enforcement
    - Traceability: [TAG-NO-CROSS-TENANT]

22. **Credential Exposure in API Responses**: The system **shall not** expose guardrail server credentials (API keys, authentication tokens) in API responses or logs.
    - Rationale: Credential security
    - Traceability: [TAG-NO-CREDENTIAL-EXPOSURE]

### Optional Requirements (Enhancement Features)

**EARS Pattern**: **WHERE** [feature exists], the [system] **shall** [response].

23. **Async Guardrail Processing**: **WHERE** possible, the system **shall** support asynchronous guardrail processing for non-blocking message pipelines.
    - Rationale: Improves throughput for high-volume tenants
    - Traceability: [TAG-ASYNC-PROCESSING]

24. **Guardrail Decision Caching**: **WHERE** possible, the system **shall** cache guardrail decisions for identical message content (content hash-based deduplication).
    - Rationale: Reduces redundant guardrail server calls
    - Traceability: [TAG-DECISION-CACHING]

25. **Guardrail Metrics Dashboard**: **WHERE** possible, the system **shall** provide guardrail processing metrics including latency, pass/reject rates, and failure counts.
    - Rationale: Operational visibility for admins
    - Traceability: [TAG-METRICS-DASHBOARD]

26. **Real-Time Status Updates**: **WHERE** possible, the system **shall** support WebSocket connections for real-time guardrail processing status updates in the admin UI.
    - Rationale: Enhanced user experience during testing
    - Traceability: [TAG-REALTIME-STATUS]

---

## Specifications

### Plugin Interface Design

```go
// GuardrailAction represents the decision made by a guardrail
type GuardrailAction string

const (
    ActionAllow  GuardrailAction = "ALLOW"
    ActionReject GuardrailAction = "REJECT"
    ActionModify GuardrailAction = "MODIFY"
)

// GuardrailResult contains the guardrail decision and optional modifications
type GuardrailResult struct {
    Action   GuardrailAction `json:"action"`
    Reason   string          `json:"reason"`
    Modified *Message        `json:"modified,omitempty"` // Set when Action == MODIFY
}

// Guardrail interface for all guardrail implementations
type Guardrail interface {
    // Name returns the unique identifier for this guardrail
    Name() string

    // Check processes a message and returns the guardrail decision
    Check(ctx context.Context, msg *Message) (*GuardrailResult, error)
}

// Message represents an email message in the guardrail system
type Message struct {
    ID          string            `json:"id"`
    From        string            `json:"from"`
    To          []string          `json:"to"`
    Subject     string            `json:"subject"`
    Body        string            `json:"body"`
    Headers     map[string]string `json:"headers"`
    Attachments []Attachment      `json:"attachments,omitempty"`
}

// Attachment represents an email attachment
type Attachment struct {
    Filename    string `json:"filename"`
    ContentType string `json:"content_type"`
    Size        int64  `json:"size"`
    Data        []byte `json:"data"` // Base64-encoded in JSON
}
```

### Built-In Guardrail Implementations

**HTTPWebhookGuardrail**: Generic HTTP/HTTPS webhook integration

```go
type HTTPWebhookGuardrail struct {
    name        string
    url         string
    timeout     time.Duration
    headers     map[string]string
    breaker     *gobreaker.CircuitBreaker
    fallback    FallbackPolicy
    httpClient  *http.Client
}

// FallbackPolicy defines behavior when guardrail is unavailable
type FallbackPolicy string

const (
    FallbackAllow        FallbackPolicy = "allow"
    FallbackReject       FallbackPolicy = "reject"
    FallbackQueueRetry   FallbackPolicy = "queue-for-retry"
)
```

**SpamFilterGuardrail**: Built-in spam detection using heuristics

```go
type SpamFilterGuardrail struct {
    name               string
    spamScoreThreshold float64
    blocklistedDomains []string
}
```

### Chain Executor Pattern

```go
// GuardrailChain executes guardrails in priority order
type GuardrailChain struct {
    guardrails []GuardrailConfig
    logger     zerolog.Logger
    metrics    *prometheus.CounterVec
}

// Execute runs the guardrail chain and returns the final result
func (gc *GuardrailChain) Execute(ctx context.Context, msg *Message) (*GuardrailResult, error) {
    for _, config := range gc.guardrails {
        if !config.Enabled {
            continue
        }

        result, err := config.Guardrail.Check(ctx, msg)
        if err != nil {
            gc.logger.Error().Err(err).
                Str("guardrail", config.Name).
                Str("message_id", msg.ID).
                Msg("Guardrail execution error")

            // Apply fallback policy on error
            return gc.applyFallback(config.FallbackPolicy, err)
        }

        gc.logDecision(msg.ID, config.Name, result)

        switch result.Action {
        case ActionReject:
            return result, nil // Early termination
        case ActionModify:
            msg = result.Modified // Update message for next guardrail
        case ActionAllow:
            continue // Proceed to next guardrail
        }
    }

    return &GuardrailResult{Action: ActionAllow}, nil
}
```

### Database Schema

```sql
CREATE TABLE guardrails (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id      UUID REFERENCES accounts(id) ON DELETE CASCADE, -- Nullable: NULL = tenant-wide default, NOT NULL = account-specific
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50) NOT NULL, -- 'http_webhook', 'spam_filter', 'custom'
    config_json     JSONB NOT NULL,
    priority        INTEGER NOT NULL DEFAULT 100,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    fallback_policy VARCHAR(50) NOT NULL DEFAULT 'allow', -- 'allow', 'reject', 'queue-for-retry'
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_tenant_account_guardrail_name UNIQUE (tenant_id, account_id, name),
    CONSTRAINT valid_priority CHECK (priority >= 0 AND priority <= 1000)
);

CREATE INDEX idx_guardrails_tenant_enabled ON guardrails(tenant_id, enabled);
CREATE INDEX idx_guardrails_account_enabled ON guardrails(account_id, enabled) WHERE account_id IS NOT NULL;
CREATE INDEX idx_guardrails_priority ON guardrails(tenant_id, account_id, priority) WHERE enabled = true;

-- Resolution order when processing a message from account X:
-- 1. Load account-specific guardrails (WHERE account_id = X)
-- 2. Load tenant-wide guardrails (WHERE account_id IS NULL AND tenant_id = T)
-- 3. Merge: account-specific guardrails take precedence, tenant defaults fill gaps

-- Example config_json for HTTPWebhookGuardrail:
-- {
--   "url": "https://guardrail.example.com/check",
--   "timeout_seconds": 5,
--   "headers": {
--     "Authorization": "Bearer secret_token",
--     "X-Custom-Header": "value"
--   }
-- }
```

### Admin API Endpoints

**POST /api/v1/guardrails**: Create new guardrail configuration

Request:
```json
{
  "name": "content-policy-check",
  "type": "http_webhook",
  "account_id": "account-uuid-or-null",
  "config": {
    "url": "https://guardrail.example.com/check",
    "timeout_seconds": 5,
    "headers": {
      "Authorization": "Bearer secret_token"
    }
  },
  "priority": 100,
  "enabled": true,
  "fallback_policy": "reject"
}
```

Response (201 Created):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "tenant-uuid",
  "account_id": "account-uuid",
  "name": "content-policy-check",
  "type": "http_webhook",
  "config": {
    "url": "https://guardrail.example.com/check",
    "timeout_seconds": 5
  },
  "priority": 100,
  "enabled": true,
  "fallback_policy": "reject",
  "created_at": "2026-02-15T10:00:00Z",
  "updated_at": "2026-02-15T10:00:00Z"
}
```

Note: If `account_id` is null, the guardrail applies as a tenant-wide default. If set, it applies only to that specific account.

**GET /api/v1/guardrails**: List all guardrails for current tenant

**PUT /api/v1/guardrails/{id}**: Update guardrail configuration

**DELETE /api/v1/guardrails/{id}**: Delete guardrail configuration

**POST /api/v1/guardrails/{id}/test**: Test guardrail with sample message

Request:
```json
{
  "from": "test@example.com",
  "to": ["recipient@example.com"],
  "subject": "Test Message",
  "body": "This is a test message for guardrail validation."
}
```

Response (200 OK):
```json
{
  "action": "ALLOW",
  "reason": "Content passed all policy checks",
  "latency_ms": 234
}
```

### Frontend Components

**Guardrail Configuration Page** (`app/(dashboard)/guardrails/page.tsx`):
- List all configured guardrails with status indicators
- Drag-and-drop priority reordering
- Enable/disable toggle per guardrail
- Test button for sample message validation
- Delete confirmation dialog

**Guardrail Form Component** (`app/(dashboard)/guardrails/new/page.tsx`):
- Form fields: Name, Type (dropdown), URL, Headers (key-value pairs), Priority, Fallback Policy
- Validation: HTTPS URL required, priority 0-1000
- Test connection button before saving

### Circuit Breaker Configuration

Using `gobreaker` library:

```go
settings := gobreaker.Settings{
    Name:        "guardrail-" + guardrailID,
    MaxRequests: 1,    // Allow 1 request in half-open state
    Interval:    60 * time.Second,
    Timeout:     30 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        // Open circuit after 5 consecutive failures
        return counts.ConsecutiveFailures >= 5
    },
    OnStateChange: func(name string, from, to gobreaker.State) {
        logger.Info().
            Str("guardrail", name).
            Str("from", from.String()).
            Str("to", to.String()).
            Msg("Circuit breaker state changed")
    },
}

breaker := gobreaker.NewCircuitBreaker(settings)
```

### Performance Metrics

Prometheus metrics exposed:

- `guardrail_checks_total{guardrail, action, tenant}`: Counter for guardrail decisions
- `guardrail_check_duration_seconds{guardrail, tenant}`: Histogram for latency
- `guardrail_errors_total{guardrail, error_type, tenant}`: Counter for errors
- `guardrail_circuit_breaker_state{guardrail, state}`: Gauge for circuit state (0=closed, 1=open, 2=half-open)

---

## Traceability Tags

- [TAG-GUARDRAIL-EXECUTION]: Requirement 1 → Plan Section 3.1, Acceptance Scenario 1
- [TAG-GUARDRAIL-LOGGING]: Requirement 2 → Plan Section 3.2, Acceptance Scenario 9
- [TAG-PLUGIN-INTERFACE]: Requirement 3 → Plan Section 2.1, Acceptance Scenario 2
- [TAG-TENANT-ISOLATION]: Requirement 4 → Plan Section 3.3, Acceptance Scenario 9
- [TAG-DEQUEUE-PROCESSING]: Requirement 5 → Plan Section 2.2, Acceptance Scenario 4
- [TAG-ALLOW-ACTION]: Requirement 6 → Plan Section 2.2, Acceptance Scenario 1
- [TAG-REJECT-ACTION]: Requirement 7 → Plan Section 2.2, Acceptance Scenario 2
- [TAG-MODIFY-ACTION]: Requirement 8 → Plan Section 2.2, Acceptance Scenario 3
- [TAG-CONFIG-CREATE]: Requirement 9 → Plan Section 2.3, Acceptance Scenario 7
- [TAG-TEST-ENDPOINT]: Requirement 10 → Plan Section 2.3, Acceptance Scenario 8
- [TAG-FALLBACK-POLICY]: Requirement 11 → Plan Section 2.4, Acceptance Scenario 5
- [TAG-CIRCUIT-BREAKER]: Requirement 12 → Plan Section 2.4, Acceptance Scenario 6
- [TAG-NO-GUARDRAILS]: Requirement 13 → Plan Section 2.2, Acceptance Scenario 10
- [TAG-TIMEOUT-SUCCESS]: Requirement 14 → Plan Section 2.4, Acceptance Scenario 5
- [TAG-TIMEOUT-EXCEEDED]: Requirement 15 → Plan Section 2.4, Acceptance Scenario 5
- [TAG-MULTIPLE-GUARDRAILS]: Requirement 16 → Plan Section 2.2, Acceptance Scenario 4
- [TAG-EARLY-TERMINATION]: Requirement 17 → Plan Section 2.2, Acceptance Scenario 4
- [TAG-CIRCUIT-HALFOPEN]: Requirement 18 → Plan Section 2.4, Acceptance Scenario 6
- [TAG-HTTPS-REQUIRED]: Requirement 19 → Plan Section 3.4, Acceptance Scenario 7
- [TAG-NO-QUEUE-BLOCKING]: Requirement 20 → Plan Section 2.4, Acceptance Scenario 6
- [TAG-NO-CROSS-TENANT]: Requirement 21 → Plan Section 3.3, Acceptance Scenario 9
- [TAG-NO-CREDENTIAL-EXPOSURE]: Requirement 22 → Plan Section 3.4, Acceptance Scenario 7
- [TAG-ASYNC-PROCESSING]: Requirement 23 → Plan Section 4.1
- [TAG-DECISION-CACHING]: Requirement 24 → Plan Section 4.2
- [TAG-METRICS-DASHBOARD]: Requirement 25 → Plan Section 3.2
- [TAG-REALTIME-STATUS]: Requirement 26 → Plan Section 4.3

---

**SPEC Approval**: Approved for implementation
**Next Phase**: `/moai:2-run SPEC-GUARDRAIL-001` for DDD implementation
