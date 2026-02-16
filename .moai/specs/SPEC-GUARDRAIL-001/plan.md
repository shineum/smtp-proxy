# Implementation Plan: SPEC-GUARDRAIL-001

## Overview

This document outlines the implementation strategy for the Email Guardrail Plugin System, a middleware component that provides content inspection and policy enforcement before email delivery through ESP providers.

---

## 1. Implementation Priorities

### Priority High (Primary Goals)

**Milestone 1.1**: Core Guardrail Plugin Interface and Chain Executor
- Deliverables:
  - Guardrail interface definition (Guardrail, GuardrailResult, GuardrailAction types)
  - Chain executor with priority-based execution
  - Message processing pipeline integration
  - Basic error handling and logging
- Dependencies: SPEC-QUEUE-001 (message queue integration)
- Traceability: [TAG-PLUGIN-INTERFACE], [TAG-GUARDRAIL-EXECUTION]

**Milestone 1.2**: HTTPWebhookGuardrail Implementation
- Deliverables:
  - HTTP client with timeout configuration
  - HTTPS-only enforcement
  - Request/response JSON serialization
  - Custom header support
  - Error handling for network failures
- Dependencies: Milestone 1.1
- Traceability: [TAG-HTTPS-REQUIRED], [TAG-FALLBACK-POLICY]

**Milestone 1.3**: Circuit Breaker Integration
- Deliverables:
  - gobreaker library integration
  - Circuit state management (closed, open, half-open)
  - Failure threshold configuration (5 consecutive failures)
  - State change logging
  - Fallback policy execution during open state
- Dependencies: Milestone 1.2
- Traceability: [TAG-CIRCUIT-BREAKER], [TAG-NO-QUEUE-BLOCKING]

**Milestone 1.4**: Database Schema and Configuration Storage
- Deliverables:
  - Guardrails table with tenant and account association
  - Configuration JSONB storage
  - Priority and enabled status fields
  - account_id nullable column for tenant-wide defaults vs account-specific configs
  - Database migration scripts (up/down)
  - Tenant isolation via foreign key constraints
- Dependencies: SPEC-MULTITENANT-001 (tenants table), SPEC-ACCOUNT-001 (accounts table)
- Traceability: [TAG-TENANT-ISOLATION], [TAG-CONFIG-CREATE]

### Priority Medium (Secondary Goals)

**Milestone 2.1**: Admin API Endpoints
- Deliverables:
  - POST /api/v1/guardrails (create with optional account_id)
  - GET /api/v1/guardrails (list with account_id filter)
  - PUT /api/v1/guardrails/{id} (update)
  - DELETE /api/v1/guardrails/{id} (delete)
  - POST /api/v1/guardrails/{id}/test (test)
  - Request validation with go-playground/validator
  - Credential redaction in responses
  - Account-level filtering and resolution logic
- Dependencies: Milestone 1.4, SPEC-CORE-001 (API framework)
- Traceability: [TAG-CONFIG-CREATE], [TAG-TEST-ENDPOINT], [TAG-NO-CREDENTIAL-EXPOSURE]

**Milestone 2.2**: Frontend Guardrail Configuration UI
- Deliverables:
  - Guardrail list page with status indicators
  - Create/edit guardrail form
  - Test connection button
  - Priority reordering UI (drag-and-drop)
  - Enable/disable toggle
  - Delete confirmation dialog
- Dependencies: Milestone 2.1
- Traceability: [TAG-CONFIG-CREATE], [TAG-TEST-ENDPOINT]

**Milestone 2.3**: SpamFilterGuardrail Built-In Implementation
- Deliverables:
  - Heuristic spam detection
  - Spam score calculation
  - Blocklisted domain checking
  - Configurable threshold
  - Standalone guardrail (no external dependencies)
- Dependencies: Milestone 1.1
- Traceability: [TAG-PLUGIN-INTERFACE]

### Priority Low (Final Goals)

**Milestone 3.1**: Guardrail Metrics and Monitoring
- Deliverables:
  - Prometheus metrics exposition
  - guardrail_checks_total counter
  - guardrail_check_duration_seconds histogram
  - guardrail_errors_total counter
  - guardrail_circuit_breaker_state gauge
  - Grafana dashboard template
- Dependencies: Milestone 1.3
- Traceability: [TAG-METRICS-DASHBOARD], [TAG-GUARDRAIL-LOGGING]

**Milestone 3.2**: Comprehensive Logging
- Deliverables:
  - Structured JSON logging with zerolog
  - Guardrail decision logging (message ID, guardrail name, action, timestamp)
  - Error logging with context
  - Circuit state change logging
  - Correlation ID propagation
- Dependencies: Milestone 1.1
- Traceability: [TAG-GUARDRAIL-LOGGING]

**Milestone 3.3**: Per-Account and Per-Tenant Guardrail Isolation Testing
- Deliverables:
  - Integration tests for tenant isolation
  - Integration tests for account-level configuration resolution
  - Test scenarios for tenant-wide defaults vs account-specific overrides
  - Negative tests for cross-tenant access attempts
  - Configuration leak prevention validation
  - Database constraint verification
- Dependencies: Milestone 1.4
- Traceability: [TAG-TENANT-ISOLATION], [TAG-NO-CROSS-TENANT]

### Optional Goals (Enhancement Features)

**Milestone 4.1**: Async Guardrail Processing
- Deliverables:
  - Goroutine-based parallel processing
  - Context cancellation support
  - Timeout handling per guardrail
  - Async result aggregation
- Dependencies: Milestone 1.1
- Traceability: [TAG-ASYNC-PROCESSING]

**Milestone 4.2**: Guardrail Decision Caching
- Deliverables:
  - Content hash-based cache key generation
  - Redis-backed cache storage
  - Configurable TTL per guardrail
  - Cache invalidation on configuration changes
  - Cache key includes account_id for account-level isolation
- Dependencies: Milestone 1.3
- Traceability: [TAG-DECISION-CACHING]

**Milestone 4.3**: Real-Time Status Updates
- Deliverables:
  - WebSocket endpoint for guardrail status
  - Frontend WebSocket client integration
  - Real-time test result streaming
  - Connection management and reconnection logic
- Dependencies: Milestone 2.2
- Traceability: [TAG-REALTIME-STATUS]

---

## 2. Technical Approach

### 2.1. Plugin Architecture Design

**Interface Definition**:
- Define `Guardrail` interface in `backend/internal/guardrail/interface.go`
- Ensure interface stability across versions using semantic versioning
- Export types: `GuardrailAction`, `GuardrailResult`, `Message`, `Attachment`

**Plugin Registration**:
- Plugin factory pattern for type-based instantiation
- Register built-in guardrails: `http_webhook`, `spam_filter`
- Support for custom guardrail types via plugin loader

**Design Rationale**: Interface-based design enables extensibility without breaking existing implementations. Type-based factory pattern allows dynamic guardrail instantiation from database configuration.

### 2.2. Chain Executor Implementation

**Execution Flow**:
1. Load guardrails for the message's account from database:
   - First, load account-specific guardrails (WHERE account_id = X AND enabled = true)
   - Then, load tenant-wide default guardrails (WHERE account_id IS NULL AND tenant_id = T AND enabled = true)
   - Merge both sets, with account-specific guardrails taking precedence
2. Order merged guardrails by priority ASC
3. Iterate through guardrail list:
   - Call `guardrail.Check(ctx, msg)` with timeout context
   - Log decision (ALLOW, REJECT, MODIFY)
   - On REJECT: Return immediately, skip remaining guardrails
   - On MODIFY: Update `msg` variable for next iteration
   - On ALLOW: Continue to next guardrail
4. If all guardrails ALLOW or MODIFY, return final ALLOW result

**Error Handling**:
- On network error: Apply fallback policy (allow/reject/queue-for-retry)
- On timeout: Cancel context, apply fallback policy
- On circuit open: Skip guardrail call, apply fallback policy
- Log all errors with guardrail name and message ID

**Design Rationale**: Priority-based ordered execution provides predictable behavior. Early termination on REJECT optimizes performance by avoiding unnecessary guardrail calls.

### 2.3. Admin API Implementation

**Endpoint Structure**:
- Base path: `/api/v1/guardrails`
- Authentication: JWT token required (tenant context from token claims)
- Validation: go-playground/validator for struct validation
- Error responses: Standardized JSON error format from SPEC-CORE-001

**Request Validation**:
- HTTPS URL requirement: Reject HTTP-only URLs with 400 Bad Request
- Priority range: 0-1000 (database constraint enforced)
- Fallback policy: Enum validation (allow, reject, queue-for-retry)
- Configuration JSON: Schema validation based on guardrail type

**Credential Redaction**:
- Filter sensitive fields from API responses: `config.headers.Authorization`, `config.api_key`
- Replace with placeholder: `"***REDACTED***"`
- Preserve structure for debugging without exposing secrets

**Design Rationale**: HTTPS enforcement at API layer prevents misconfigurations. Credential redaction ensures secrets never appear in logs or API responses while maintaining config structure visibility.

### 2.4. Circuit Breaker Strategy

**Circuit Configuration**:
- MaxRequests: 1 (half-open state allows 1 test request)
- Interval: 60 seconds (reset interval for failure counters)
- Timeout: 30 seconds (time before half-open transition)
- ReadyToTrip: 5 consecutive failures within interval

**State Transitions**:
- Closed → Open: After 5 consecutive failures
- Open → Half-Open: After 30 seconds timeout
- Half-Open → Closed: On successful test request
- Half-Open → Open: On failed test request

**Integration with Fallback**:
- Circuit open: Apply guardrail's fallback policy
- Circuit half-open: Allow one request through, reset to closed on success
- Circuit closed: Normal guardrail execution

**Design Rationale**: Circuit breaker prevents cascading failures from unhealthy guardrail servers. Automatic recovery via half-open state eliminates manual intervention for transient failures.

---

## 3. File Structure

### Backend Go Implementation

```
backend/
├── internal/
│   ├── guardrail/
│   │   ├── interface.go              # Guardrail interface, types, constants
│   │   ├── chain.go                  # GuardrailChain executor
│   │   ├── http_webhook.go           # HTTPWebhookGuardrail implementation
│   │   ├── spam_filter.go            # SpamFilterGuardrail implementation
│   │   ├── factory.go                # Type-based guardrail factory
│   │   ├── circuit_breaker.go        # Circuit breaker wrapper
│   │   └── metrics.go                # Prometheus metrics
│   ├── api/
│   │   └── v1/
│   │       └── guardrails/
│   │           ├── handlers.go       # HTTP handlers for CRUD endpoints
│   │           ├── requests.go       # Request validation structs
│   │           ├── responses.go      # Response DTOs with redaction
│   │           └── routes.go         # chi router configuration
│   ├── repository/
│   │   └── guardrail_repo.go         # Database access layer (pgx)
│   └── worker/
│       └── message_processor.go      # Integration with queue worker
└── migrations/
    ├── 003_create_guardrails.up.sql   # Guardrails table creation
    └── 003_create_guardrails.down.sql # Guardrails table rollback
```

### Frontend Next.js Implementation

```
frontend/
├── app/
│   └── (dashboard)/
│       └── guardrails/
│           ├── page.tsx               # List page with table and actions
│           ├── new/
│           │   └── page.tsx           # Create guardrail form
│           ├── [id]/
│           │   ├── page.tsx           # Edit guardrail form
│           │   └── test/
│           │       └── page.tsx       # Test guardrail page
│           └── components/
│               ├── GuardrailList.tsx  # Table component
│               ├── GuardrailForm.tsx  # Form component (create/edit)
│               ├── TestForm.tsx       # Test message form
│               └── PriorityReorder.tsx # Drag-and-drop reordering
├── lib/
│   └── api/
│       └── guardrails.ts              # API client functions
└── types/
    └── guardrail.ts                   # TypeScript type definitions
```

---

## 4. Architecture Integration Points

### 4.1. Message Queue Integration

**Integration Point**: Queue worker message processing pipeline

**Flow**:
1. Worker dequeues message from Redis Streams / NATS JetStream
2. Load guardrails for the message's account from database (cached with 5-minute TTL, cache key includes account_id)
   - Resolution: account-specific guardrails override tenant defaults
3. Execute guardrail chain via `GuardrailChain.Execute(ctx, msg)`
4. Handle result:
   - ALLOW: Proceed to ESP provider delivery
   - REJECT: Mark message as rejected, update status in database, skip delivery
   - MODIFY: Use modified message for ESP delivery
5. Log guardrail decision for audit

**Code Location**: `backend/internal/worker/message_processor.go`

**Dependency**: SPEC-QUEUE-001 must implement message dequeue and ESP delivery

### 4.2. Database Integration

**Schema Dependencies**:
- `tenants` table (from SPEC-MULTITENANT-001) for foreign key relationship
- `accounts` table (from SPEC-ACCOUNT-001) for foreign key relationship
- `guardrails` table (this SPEC) for configuration storage
- Foreign keys:
  - `guardrails.tenant_id → tenants.id ON DELETE CASCADE`
  - `guardrails.account_id → accounts.id ON DELETE CASCADE`

**Query Patterns**:
- List guardrails for account:
  - Account-specific: `SELECT * FROM guardrails WHERE account_id = $1 AND enabled = true ORDER BY priority ASC`
  - Tenant defaults: `SELECT * FROM guardrails WHERE account_id IS NULL AND tenant_id = $1 AND enabled = true ORDER BY priority ASC`
  - Combined resolution: Load both sets, merge with account-specific taking precedence
- Create guardrail: `INSERT INTO guardrails (tenant_id, account_id, name, type, config_json, priority, enabled, fallback_policy) VALUES (...)`
- Update guardrail: `UPDATE guardrails SET ... WHERE id = $1 AND tenant_id = $2` (tenant check prevents cross-tenant updates)
- Delete guardrail: `DELETE FROM guardrails WHERE id = $1 AND tenant_id = $2`

**Code Location**: `backend/internal/repository/guardrail_repo.go`

### 4.3. Admin API Integration

**Integration Point**: chi HTTP router with tenant authentication middleware

**Router Configuration**:
```go
r.Route("/api/v1/guardrails", func(r chi.Router) {
    r.Use(middleware.RequireAuth)      // JWT authentication
    r.Use(middleware.ExtractTenant)    // Extract tenant ID from token
    r.Post("/", handlers.CreateGuardrail)
    r.Get("/", handlers.ListGuardrails)
    r.Get("/{id}", handlers.GetGuardrail)
    r.Put("/{id}", handlers.UpdateGuardrail)
    r.Delete("/{id}", handlers.DeleteGuardrail)
    r.Post("/{id}/test", handlers.TestGuardrail)
})
```

**Code Location**: `backend/internal/api/v1/guardrails/routes.go`

**Dependency**: SPEC-CORE-001 must implement authentication middleware and tenant context

---

## 5. Risks and Mitigation Strategies

### Risk 1: Guardrail Server Latency Exceeds SLA

**Description**: External guardrail servers respond slowly, causing queue processing delays and breaching p95 latency target of 5 seconds.

**Probability**: Medium
**Impact**: High

**Mitigation**:
1. **Strict Timeout Configuration**: Default 5-second timeout per guardrail, configurable down to 1 second
2. **Circuit Breaker**: Automatically bypass slow servers after 5 consecutive failures
3. **Async Processing** (Optional): Run guardrails in parallel where dependencies allow
4. **Monitoring**: Alert on guardrail latency p95 exceeding 3 seconds

**Validation**: Load testing with simulated slow servers (2-10 second response times)

### Risk 2: Circuit Breaker Causes False Positives

**Description**: Circuit breaker opens prematurely due to transient network issues, bypassing guardrails when servers are actually healthy.

**Probability**: Low
**Impact**: Medium

**Mitigation**:
1. **Failure Threshold Tuning**: Require 5 consecutive failures (not intermittent) within 60-second window
2. **Half-Open Recovery**: Automatically test recovery after 30 seconds
3. **Manual Override**: Admin UI toggle to force circuit closed for troubleshooting
4. **Alerting**: Notify admins on circuit open events for investigation

**Validation**: Network partition simulation tests with automatic recovery verification

### Risk 3: Guardrail Configuration Errors Block All Email

**Description**: Misconfigured guardrail (invalid URL, wrong credentials) causes all messages to be rejected via fallback policy.

**Probability**: Medium
**Impact**: High

**Mitigation**:
1. **Test Endpoint**: Require successful test before enabling guardrail
2. **Gradual Rollout**: Admin UI warning when enabling guardrail for first time
3. **Fallback Policy Default**: Default to "allow" for new guardrails (fail-open)
4. **Dashboard Monitoring**: Real-time rejection rate alerts

**Validation**: Integration tests with intentionally misconfigured guardrails

### Risk 4: HTTPS Certificate Validation Failures

**Description**: Guardrail servers with self-signed or expired certificates cause connection failures.

**Probability**: Medium
**Impact**: Medium

**Mitigation**:
1. **Standard Library TLS**: Use Go's `net/http` default TLS configuration (strict validation)
2. **Certificate Error Logging**: Log certificate validation errors with details
3. **Documentation**: Admin documentation for certificate requirements
4. **Test Endpoint Validation**: Surface certificate errors during guardrail testing

**Validation**: Unit tests with self-signed and expired certificate scenarios

### Risk 5: Database Performance Degradation

**Description**: High-volume tenants with many guardrails cause slow database queries, blocking queue workers.

**Probability**: Low
**Impact**: Medium

**Mitigation**:
1. **Database Indexes**: Create composite index on `(tenant_id, enabled, priority)`
2. **In-Memory Caching**: Cache tenant guardrail configurations with 5-minute TTL
3. **Query Optimization**: Use prepared statements with pgx for efficient execution
4. **Monitoring**: Track database query latency in Prometheus

**Validation**: Load testing with 1000 tenants, 10 guardrails each, 10K messages/minute

---

## 6. Testing Strategy

### 6.1. Unit Tests (Target: 85%+ Coverage)

**Backend Tests**:
- `guardrail/interface_test.go`: GuardrailResult, GuardrailAction enum tests
- `guardrail/chain_test.go`: Chain execution logic, priority ordering, early termination
- `guardrail/http_webhook_test.go`: HTTP client, timeout handling, response parsing
- `guardrail/spam_filter_test.go`: Spam score calculation, blocklisted domain checks
- `guardrail/circuit_breaker_test.go`: State transitions, failure counting, recovery
- `repository/guardrail_repo_test.go`: CRUD operations, tenant isolation queries

**Frontend Tests**:
- `guardrails/GuardrailList.test.tsx`: List rendering, enable/disable toggle
- `guardrails/GuardrailForm.test.tsx`: Form validation, HTTPS enforcement
- `guardrails/TestForm.test.tsx`: Test message submission, result display

### 6.2. Integration Tests

**Scenario 1**: Single HTTPWebhookGuardrail ALLOW flow
- Setup: Create HTTP test server returning `{"action": "ALLOW"}`
- Test: Send message through guardrail chain, verify ESP delivery
- Validation: Message delivered, guardrail decision logged

**Scenario 2**: Single HTTPWebhookGuardrail REJECT flow
- Setup: Create HTTP test server returning `{"action": "REJECT", "reason": "Spam detected"}`
- Test: Send message through guardrail chain
- Validation: Message marked rejected, no ESP delivery, reason logged

**Scenario 3**: HTTPWebhookGuardrail MODIFY flow
- Setup: Create HTTP test server returning `{"action": "MODIFY", "modified": {...}}`
- Test: Send message through guardrail chain
- Validation: Modified message delivered to ESP, original message replaced

**Scenario 4**: Chained guardrails execution order
- Setup: Create 3 guardrails with priorities 100, 200, 300
- Test: Send message, verify execution order via logs
- Validation: Guardrails execute in ascending priority order

**Scenario 5**: Timeout with fallback policy
- Setup: Create guardrail with 1-second timeout, test server with 5-second delay
- Test: Send message through guardrail
- Validation: Timeout after 1 second, fallback policy applied (allow/reject based on config)

**Scenario 6**: Circuit breaker activation and recovery
- Setup: Create guardrail pointing to unavailable server
- Test: Send 5 messages (trigger circuit open), wait 30 seconds, send 1 message (half-open), send message (closed)
- Validation: Circuit opens after 5 failures, half-open after timeout, closes on success

**Scenario 7**: Admin CRUD operations
- Test: Create guardrail via POST, list via GET, update via PUT, delete via DELETE
- Validation: All operations succeed, tenant isolation enforced, credentials redacted in responses

**Scenario 8**: Test endpoint
- Test: POST /api/v1/guardrails/{id}/test with sample message
- Validation: Returns ALLOW/REJECT/MODIFY result, latency reported

**Scenario 9**: Cross-tenant isolation
- Setup: Create 2 tenants with separate guardrails
- Test: Attempt to access tenant B's guardrail from tenant A's token
- Validation: 403 Forbidden, no data leak

**Scenario 10**: No guardrails configured (passthrough)
- Setup: Tenant with zero guardrails
- Test: Send message
- Validation: Message proceeds directly to ESP delivery, zero latency overhead

### 6.3. Performance Tests

**Load Test 1**: Guardrail latency under normal load
- Test: 1000 messages/minute through single guardrail
- Target: p95 latency < 500ms added by guardrail processing
- Tool: k6 with custom Go script

**Load Test 2**: Concurrent guardrail requests
- Test: 100 concurrent messages through guardrail chain
- Target: No goroutine leaks, stable memory usage
- Validation: Go pprof analysis

**Load Test 3**: Database query performance
- Test: 10,000 tenants with 10 guardrails each
- Target: Guardrail configuration load < 10ms p95
- Validation: Database query EXPLAIN ANALYZE

### 6.4. Security Tests

**Test 1**: HTTP URL rejection
- Test: Attempt to create guardrail with `http://` URL
- Expected: 400 Bad Request with error message

**Test 2**: Credential exposure check
- Test: Create guardrail with Authorization header, call GET /api/v1/guardrails/{id}
- Expected: Authorization value redacted in response

**Test 3**: Cross-tenant guardrail access
- Test: Use tenant A token to access tenant B guardrail
- Expected: 404 Not Found or 403 Forbidden

---

## 7. Dependencies

### External Dependencies

| Dependency | Version | Purpose | License |
|------------|---------|---------|---------|
| gobreaker | v1.0.0+ | Circuit breaker pattern | MIT |
| go-playground/validator | v10.0.0+ | Struct validation | MIT |
| rs/zerolog | v1.31.0+ | Structured logging | MIT |
| prometheus/client_golang | v1.17.0+ | Metrics exposition | Apache 2.0 |

### Internal Dependencies

| Component | SPEC | Dependency Type | Description |
|-----------|------|-----------------|-------------|
| SMTP Queue Worker | SPEC-QUEUE-001 | Required | Message dequeue and ESP delivery integration point |
| Multi-Tenant System | SPEC-MULTITENANT-001 | Required | Tenants table for foreign key relationship |
| Account Management | SPEC-ACCOUNT-001 | Required | Accounts table for foreign key relationship and account-level configuration |
| API Framework | SPEC-CORE-001 | Required | Authentication middleware and router configuration |

---

## 8. Deployment Considerations

### 8.1. Database Migration

**Migration Sequence**:
1. Run migration `003_create_guardrails.up.sql` on staging environment
2. Verify indexes created: `idx_guardrails_tenant_enabled`, `idx_guardrails_priority`
3. Test foreign key constraint by attempting to delete tenant with guardrails
4. Deploy to production during maintenance window

**Rollback Plan**:
- Run `003_create_guardrails.down.sql` to drop table
- No data loss if migration failed before tenant guardrail creation

### 8.2. Feature Flags

**Recommended Flags**:
- `guardrail_enabled`: Global toggle to enable/disable guardrail processing
- `guardrail_async_processing`: Enable async guardrail execution (optional feature)
- `guardrail_decision_caching`: Enable Redis-based decision caching (optional feature)

**Implementation**: Environment variables or configuration service (viper)

### 8.3. Monitoring Setup

**Required Dashboards**:
- Guardrail latency by tenant and guardrail name
- Circuit breaker state visualization (closed/open/half-open)
- Rejection rate by guardrail
- Fallback policy execution counts

**Alerting Rules**:
- Alert: Guardrail p95 latency > 3 seconds for 5 minutes
- Alert: Circuit breaker open for > 5 minutes
- Alert: Rejection rate > 10% for 10 minutes
- Alert: Guardrail error rate > 1% for 5 minutes

---

## 9. Success Criteria

### Functional Completeness

- [ ] All EARS requirements from spec.md implemented
- [ ] 3 built-in guardrail types: HTTPWebhookGuardrail, SpamFilterGuardrail, Custom
- [ ] Admin API endpoints functional with proper authentication
- [ ] Frontend UI enables guardrail CRUD operations
- [ ] Circuit breaker prevents cascading failures
- [ ] Test endpoint validates guardrail configurations

### Quality Gates (TRUST 5 Framework)

**Tested**:
- [ ] Unit test coverage ≥ 85%
- [ ] Integration tests cover all 10 scenarios
- [ ] Load tests validate p95 latency < 500ms

**Readable**:
- [ ] golangci-lint passes with zero warnings
- [ ] Code comments explain complex logic (circuit breaker, chain executor)
- [ ] API documentation generated from OpenAPI annotations

**Unified**:
- [ ] Consistent error handling patterns across all guardrail types
- [ ] Standardized logging format with zerolog
- [ ] Naming conventions follow Go best practices

**Secured**:
- [ ] HTTPS-only enforcement for guardrail server URLs
- [ ] Credential redaction in API responses and logs
- [ ] Tenant isolation validated with integration tests
- [ ] gosec security scanner passes

**Trackable**:
- [ ] All guardrail decisions logged with message ID and timestamp
- [ ] Prometheus metrics exported for observability
- [ ] Correlation IDs propagated through guardrail chain

### Performance Validation

- [ ] Guardrail processing adds < 500ms to p95 message latency
- [ ] System handles 10,000 messages/minute with guardrails enabled
- [ ] Database queries for guardrail configuration < 10ms p95
- [ ] Circuit breaker prevents queue blocking during server failures

### Documentation

- [ ] Admin documentation for guardrail configuration
- [ ] API documentation with request/response examples
- [ ] Architecture diagram showing guardrail integration points
- [ ] Troubleshooting guide for common guardrail issues

---

## 10. Next Steps

**Upon SPEC Approval**:
1. Execute `/moai:2-run SPEC-GUARDRAIL-001` to begin DDD implementation
2. Start with Milestone 1.1 (Plugin Interface) as foundation
3. Implement milestones sequentially to maintain dependency integrity
4. Run integration tests after each milestone completion
5. Deploy to staging environment for validation before production release

**Post-Implementation**:
1. Execute `/moai:3-sync SPEC-GUARDRAIL-001` for documentation generation
2. Update project README with guardrail feature description
3. Create admin user guide with configuration examples
4. Set up Grafana dashboards for guardrail metrics monitoring

---

**Plan Version**: 1.1.0
**Dependencies**: SPEC-QUEUE-001, SPEC-MULTITENANT-001, SPEC-ACCOUNT-001, SPEC-CORE-001
**Estimated Complexity**: High (multiple external integrations, circuit breaker complexity, account-level resolution)
**Recommended Team Size**: 2 developers (1 backend, 1 frontend)
