# Acceptance Criteria: SPEC-CORE-001

## Overview

This document defines the acceptance criteria for SPEC-CORE-001 Core SMTP Relay and API Server Foundation using Given-When-Then format. All scenarios must pass before considering the implementation complete.

---

## Test Scenarios

### Category 1: SMTP Connection and Handshake

#### Scenario 1.1: Successful SMTP Connection and EHLO

**Given:**
- SMTP server is running on port 587
- TLS certificate is valid and configured
- No connection limit is reached

**When:**
- Client connects to port 587
- Client sends EHLO command with domain name

**Then:**
- Server responds with 220 greeting message
- Server advertises STARTTLS, AUTH, SIZE, PIPELINING extensions
- Server provides proper SMTP banner with system hostname
- Connection remains open for subsequent commands

**Validation:**
- EHLO response contains "250-STARTTLS"
- EHLO response contains "250-AUTH PLAIN LOGIN"
- EHLO response contains "250-SIZE 26214400" (25MB limit)
- Response code is 250 OK

---

#### Scenario 1.2: STARTTLS Upgrade

**Given:**
- Client has established connection and received EHLO response
- Server TLS certificate is valid

**When:**
- Client sends STARTTLS command

**Then:**
- Server responds with 220 Ready to start TLS
- Server initiates TLS handshake
- Connection upgrades to encrypted TLS session
- Minimum TLS version is 1.2

**Validation:**
- TLS handshake completes successfully
- Cipher suite is secure (ECDHE with AES-GCM)
- Client can send EHLO again after TLS upgrade
- AUTH command is now available

---

#### Scenario 1.3: Connection Limit Enforcement

**Given:**
- SMTP server max_connections is configured to 1000
- 1000 active SMTP connections are established

**When:**
- Client attempts to establish connection #1001

**Then:**
- Server rejects connection with 421 Service not available
- Existing connections remain unaffected
- Connection counter decrements when existing connections close
- New connections accepted after count drops below limit

**Validation:**
- Response code is 421
- Error message indicates connection limit reached
- Prometheus metric `smtp_active_sessions` equals 1000
- Metric decrements when connections close

---

### Category 2: SMTP Authentication

#### Scenario 2.1: Successful AUTH PLAIN

**Given:**
- Client has established TLS connection
- Account exists in database with username "tenant1" and password "secure123"
- Server has advertised AUTH PLAIN in EHLO response

**When:**
- Client sends AUTH PLAIN command with base64-encoded credentials

**Then:**
- Server validates credentials against accounts table
- Server responds with 235 Authentication successful
- Subsequent MAIL FROM commands are authorized
- Session is associated with account_id for audit logging

**Validation:**
- Response code is 235
- Database query executes: SELECT * FROM accounts WHERE name = 'tenant1'
- Password verification uses bcrypt.CompareHashAndPassword
- Session context contains account_id UUID
- Structured log entry includes account_id and correlation_id

---

#### Scenario 2.2: Failed Authentication - Invalid Credentials

**Given:**
- Client has established TLS connection
- Account "tenant1" exists but provided password is incorrect

**When:**
- Client sends AUTH PLAIN with username "tenant1" and password "wrongpass"

**Then:**
- Server rejects authentication with 535 Authentication failed
- Connection remains open for retry
- Failed attempt is logged with client IP and username
- Session is not authorized for MAIL FROM

**Validation:**
- Response code is 535
- Log entry contains: level=warn, event=auth_failed, username=tenant1, client_ip=<IP>
- Prometheus counter `smtp_auth_attempts_total{result="failure"}` increments
- No account_id in session context

---

#### Scenario 2.3: AUTH Rejected Over Plaintext

**Given:**
- Client has established connection without TLS upgrade
- Server has not received STARTTLS command

**When:**
- Client sends AUTH PLAIN command

**Then:**
- Server rejects with 530 Must issue STARTTLS first
- Connection remains open for STARTTLS command
- No password processing occurs

**Validation:**
- Response code is 530
- Error message indicates STARTTLS requirement
- No database query executed for authentication

---

### Category 3: MAIL FROM / RCPT TO / DATA Flow

#### Scenario 3.1: Valid Email Sending Flow

**Given:**
- Client is authenticated as "tenant1"
- Account "tenant1" has allowed_domains: ["example.com", "example.org"]
- Database connection pool is healthy

**When:**
- Client sends: MAIL FROM:<sender@example.com>
- Client sends: RCPT TO:<recipient1@external.com>
- Client sends: RCPT TO:<recipient2@external.org>
- Client sends: DATA
- Client transmits message headers and body
- Client sends: . (dot on single line)

**Then:**
- Server accepts MAIL FROM with 250 OK
- Server accepts both RCPT TO with 250 OK
- Server accepts DATA with 354 Start mail input
- Server enqueues message to messages table with status=queued
- Server responds with 250 OK message queued
- Message includes sender, recipients array, headers, body, account_id, timestamp

**Validation:**
- Database INSERT executes: INSERT INTO messages (id, account_id, sender, recipients, headers, body, status, enqueued_at) VALUES (...)
- recipients field is JSONB array: ["recipient1@external.com", "recipient2@external.org"]
- Correlation ID is stored in message metadata
- Prometheus counter `smtp_message_enqueued_total` increments

---

#### Scenario 3.2: MAIL FROM Rejected - Sender Not in Allowed Domains

**Given:**
- Client is authenticated as "tenant1"
- Account "tenant1" has allowed_domains: ["example.com"]

**When:**
- Client sends: MAIL FROM:<sender@unauthorized.com>

**Then:**
- Server rejects with 550 Sender not allowed
- No message is enqueued
- Rejection is logged with account_id and attempted sender

**Validation:**
- Response code is 550
- Error message indicates sender domain not allowed
- Log entry contains: level=warn, event=sender_rejected, account_id=<UUID>, sender=sender@unauthorized.com
- No database INSERT occurs

---

#### Scenario 3.3: RCPT TO Rejected - Invalid Email Format

**Given:**
- Client is authenticated and MAIL FROM accepted

**When:**
- Client sends: RCPT TO:<invalid-email-format>

**Then:**
- Server rejects with 550 Invalid recipient format
- Session state allows retry with valid recipient
- No recipient is stored for message

**Validation:**
- Response code is 550
- Email format validation uses net/mail.ParseAddress
- Log entry contains: level=warn, event=recipient_rejected, recipient=invalid-email-format

---

#### Scenario 3.4: DATA Command - Message Size Limit

**Given:**
- Client is authenticated with valid MAIL FROM and RCPT TO
- Server max_message_size is configured to 26214400 bytes (25MB)

**When:**
- Client sends DATA command
- Client transmits message exceeding 25MB

**Then:**
- Server rejects with 552 Message exceeds size limit
- Connection remains open for new transaction
- No message is enqueued

**Validation:**
- Response code is 552
- Server stops reading after max_message_size bytes
- Log entry contains: level=warn, event=message_oversized, size_bytes=<actual_size>

---

### Category 4: API CRUD Operations for Accounts

#### Scenario 4.1: Create Account via API

**Given:**
- API server is running on port 8080
- Database is accessible and healthy

**When:**
- Client sends POST /api/v1/accounts with JSON body:
  ```json
  {
    "name": "tenant2",
    "email": "admin@tenant2.com",
    "password": "SecurePassword123!",
    "allowed_domains": ["tenant2.com"]
  }
  ```

**Then:**
- Server validates request format
- Server hashes password with bcrypt (cost factor 12)
- Server generates unique API key (32 bytes, hex encoded)
- Server inserts record into accounts table
- Server responds with 201 Created and account details (excluding password_hash)

**Validation:**
- Response code is 201
- Response body contains: id (UUID), name, email, api_key, allowed_domains, created_at
- Database record has password_hash with bcrypt prefix "$2a$12$"
- api_key is unique and 64 characters (hex encoded)
- Prometheus counter `api_requests_total{method="POST", path="/api/v1/accounts", status="201"}` increments

---

#### Scenario 4.2: Get Account Details via API

**Given:**
- Account with id "550e8400-e29b-41d4-a716-446655440000" exists
- Client has valid API key for authentication

**When:**
- Client sends GET /api/v1/accounts/550e8400-e29b-41d4-a716-446655440000
- Client includes header: Authorization: Bearer <valid_api_key>

**Then:**
- Server validates API key against accounts table
- Server queries account by id
- Server responds with 200 OK and account details
- password_hash is excluded from response

**Validation:**
- Response code is 200
- Response body contains: id, name, email, allowed_domains, created_at, updated_at
- Response does NOT contain: password_hash, api_key
- Database query: SELECT id, name, email, allowed_domains, created_at, updated_at FROM accounts WHERE id = $1

---

#### Scenario 4.3: Update Account via API

**Given:**
- Account with id "550e8400-e29b-41d4-a716-446655440000" exists
- Client has valid API key

**When:**
- Client sends PUT /api/v1/accounts/550e8400-e29b-41d4-a716-446655440000
- Request body:
  ```json
  {
    "allowed_domains": ["tenant2.com", "tenant2.org"]
  }
  ```

**Then:**
- Server validates API key
- Server updates allowed_domains field in database
- Server updates updated_at timestamp
- Server responds with 200 OK and updated account

**Validation:**
- Response code is 200
- Database UPDATE executes: UPDATE accounts SET allowed_domains = $1, updated_at = NOW() WHERE id = $2
- Response body contains updated allowed_domains array

---

#### Scenario 4.4: Delete Account via API

**Given:**
- Account with id "550e8400-e29b-41d4-a716-446655440000" exists
- Account has associated providers and routing rules

**When:**
- Client sends DELETE /api/v1/accounts/550e8400-e29b-41d4-a716-446655440000

**Then:**
- Server validates API key
- Server deletes account from database
- Cascade deletes associated providers and routing rules (via foreign key constraints)
- Server responds with 204 No Content

**Validation:**
- Response code is 204
- Database DELETE executes: DELETE FROM accounts WHERE id = $1
- Foreign key constraints trigger cascade delete for esp_providers and routing_rules
- Subsequent GET request returns 404 Not Found

---

### Category 5: API CRUD Operations for ESP Providers

#### Scenario 5.1: Create ESP Provider

**Given:**
- Account "tenant1" exists with id "550e8400-e29b-41d4-a716-446655440000"
- Client authenticated as tenant1

**When:**
- Client sends POST /api/v1/providers with JSON body:
  ```json
  {
    "name": "SendGrid Production",
    "provider_type": "sendgrid",
    "api_key": "SG.xxxxxxxxxxxxxxxxxxxxx",
    "enabled": true
  }
  ```

**Then:**
- Server validates request format
- Server associates provider with account_id from authentication context
- Server encrypts api_key before storage
- Server inserts record into esp_providers table
- Server responds with 201 Created and provider details

**Validation:**
- Response code is 201
- Response body contains: id, name, provider_type, enabled, created_at
- Response does NOT contain: api_key (security)
- Database record has encrypted api_key
- account_id foreign key references accounts(id)

---

#### Scenario 5.2: List Providers for Authenticated Tenant

**Given:**
- Account "tenant1" has 3 providers configured
- Account "tenant2" has 2 providers configured
- Client authenticated as tenant1

**When:**
- Client sends GET /api/v1/providers

**Then:**
- Server extracts account_id from authentication context
- Server queries providers filtered by account_id
- Server responds with 200 OK and array of providers
- Only tenant1's providers are returned (tenant isolation)

**Validation:**
- Response code is 200
- Response body is JSON array with 3 elements
- All elements have matching account_id
- Database query: SELECT * FROM esp_providers WHERE account_id = $1 ORDER BY created_at DESC

---

#### Scenario 5.3: Update Provider Status

**Given:**
- Provider with id "660e8400-e29b-41d4-a716-446655440000" exists
- Provider is currently enabled

**When:**
- Client sends PUT /api/v1/providers/660e8400-e29b-41d4-a716-446655440000
- Request body:
  ```json
  {
    "enabled": false
  }
  ```

**Then:**
- Server updates enabled field to false
- Server updates updated_at timestamp
- Server responds with 200 OK and updated provider

**Validation:**
- Response code is 200
- Database UPDATE executes: UPDATE esp_providers SET enabled = false, updated_at = NOW() WHERE id = $1
- Routing rules using this provider are not affected (validation happens during delivery)

---

#### Scenario 5.4: Delete Provider

**Given:**
- Provider with id "660e8400-e29b-41d4-a716-446655440000" exists
- Routing rules reference this provider

**When:**
- Client sends DELETE /api/v1/providers/660e8400-e29b-41d4-a716-446655440000

**Then:**
- Server checks for dependent routing rules
- Server either:
  - (Option A) Rejects deletion with 409 Conflict if rules exist
  - (Option B) Cascade deletes routing rules with warning

**Validation:**
- Response code is 409 or 204 (based on business logic decision)
- If 409: Response body explains dependency conflict
- If 204: Database CASCADE DELETE removes routing rules

---

### Category 6: API Routing Rules Management

#### Scenario 6.1: Create Routing Rule with Priority

**Given:**
- Account "tenant1" exists
- Provider "SendGrid Production" exists for tenant1
- Client authenticated as tenant1

**When:**
- Client sends POST /api/v1/routing-rules with JSON body:
  ```json
  {
    "priority": 10,
    "conditions": {
      "recipient_domain": "gmail.com"
    },
    "provider_id": "660e8400-e29b-41d4-a716-446655440000",
    "enabled": true
  }
  ```

**Then:**
- Server validates provider_id belongs to same account
- Server inserts rule into routing_rules table
- Server responds with 201 Created and rule details

**Validation:**
- Response code is 201
- Response body contains: id, priority, conditions, provider_id, enabled, created_at
- Database foreign key constraint validates provider_id
- account_id matches authenticated tenant

---

#### Scenario 6.2: List Routing Rules Ordered by Priority

**Given:**
- Account "tenant1" has 5 routing rules with priorities: 10, 20, 5, 30, 15

**When:**
- Client sends GET /api/v1/routing-rules

**Then:**
- Server queries rules for authenticated account
- Server orders by priority ascending (lower = higher priority)
- Server responds with 200 OK and ordered array

**Validation:**
- Response code is 200
- Response array order: [5, 10, 15, 20, 30]
- Database query: SELECT * FROM routing_rules WHERE account_id = $1 ORDER BY priority ASC

---

#### Scenario 6.3: Update Routing Rule Conditions

**Given:**
- Routing rule with id "770e8400-e29b-41d4-a716-446655440000" exists

**When:**
- Client sends PUT /api/v1/routing-rules/770e8400-e29b-41d4-a716-446655440000
- Request body:
  ```json
  {
    "conditions": {
      "recipient_domain": "gmail.com",
      "sender_domain": "example.com"
    }
  }
  ```

**Then:**
- Server updates conditions JSONB field
- Server updates updated_at timestamp
- Server responds with 200 OK and updated rule

**Validation:**
- Response code is 200
- Database UPDATE executes: UPDATE routing_rules SET conditions = $1, updated_at = NOW() WHERE id = $2
- conditions field is valid JSONB

---

### Category 7: Edge Cases and Error Handling

#### Scenario 7.1: Malformed SMTP Command

**Given:**
- Client has established SMTP connection

**When:**
- Client sends invalid command: "INVALID_CMD test@example.com"

**Then:**
- Server responds with 500 Syntax error, command unrecognized
- Connection remains open for retry
- Invalid command is logged

**Validation:**
- Response code is 500
- Log entry contains: level=warn, event=invalid_command, command=INVALID_CMD

---

#### Scenario 7.2: API Request with Missing Required Field

**Given:**
- Client attempts to create account

**When:**
- Client sends POST /api/v1/accounts with incomplete JSON:
  ```json
  {
    "name": "tenant3"
    // missing email and password
  }
  ```

**Then:**
- Server validates request format
- Server responds with 400 Bad Request
- Response body includes validation errors

**Validation:**
- Response code is 400
- Response body:
  ```json
  {
    "error": "validation_failed",
    "details": [
      "email is required",
      "password is required"
    ]
  }
  ```

---

#### Scenario 7.3: API Request with Invalid UUID Format

**Given:**
- Client attempts to retrieve account

**When:**
- Client sends GET /api/v1/accounts/invalid-uuid-format

**Then:**
- Server validates UUID format before database query
- Server responds with 400 Bad Request
- No database query is executed

**Validation:**
- Response code is 400
- Response body contains error message indicating invalid UUID
- No SELECT query executed (prevents unnecessary database load)

---

#### Scenario 7.4: Database Connection Failure During API Request

**Given:**
- API server is running
- PostgreSQL database becomes unavailable

**When:**
- Client sends GET /api/v1/accounts/550e8400-e29b-41d4-a716-446655440000

**Then:**
- Server attempts database connection
- Connection attempt fails within timeout (5 seconds)
- Server responds with 503 Service Unavailable
- Response includes Retry-After header (e.g., 30 seconds)

**Validation:**
- Response code is 503
- Response headers contain: Retry-After: 30
- Log entry contains: level=error, event=db_connection_failed, error=<error_message>
- /readyz health check returns unhealthy status

---

#### Scenario 7.5: SMTP Connection Timeout

**Given:**
- Client establishes SMTP connection
- Server read_timeout is configured to 30 seconds

**When:**
- Client connects but sends no commands for 35 seconds

**Then:**
- Server closes connection after 30 seconds
- Connection timeout is logged

**Validation:**
- Connection closed after timeout
- Log entry contains: level=info, event=connection_timeout, idle_seconds=30
- Prometheus metric `smtp_active_sessions` decrements

---

### Category 8: Performance Criteria

#### Scenario 8.1: 1,000 Concurrent SMTP Connections

**Given:**
- SMTP server is deployed with adequate resources
- Load testing tool (k6, custom Go script) is configured

**When:**
- 1,000 concurrent clients establish SMTP connections
- Each client sends EHLO and waits

**Then:**
- All connections are accepted (no 421 rejection)
- Server memory usage remains stable
- EHLO responses are delivered within 100ms (p95)

**Validation:**
- Prometheus metric `smtp_active_sessions` reaches 1000
- No OOM (out of memory) errors in logs
- p95 response time < 100ms

---

#### Scenario 8.2: API Request Throughput

**Given:**
- API server is deployed
- Load testing tool sends GET /api/v1/providers requests

**When:**
- Sustained load of 100 requests/second for 5 minutes

**Then:**
- All requests return successful responses (200 OK)
- p95 latency < 100ms
- Error rate < 0.1%

**Validation:**
- Prometheus histogram `api_request_duration_seconds{path="/api/v1/providers"}` p95 < 0.1
- Prometheus counter `api_requests_total{status="200"}` shows 30,000 requests
- Error rate calculated from status codes 500-599

---

#### Scenario 8.3: Database Query Performance

**Given:**
- Database contains 10,000 accounts, 50,000 providers, 100,000 routing rules
- API server queries accounts table

**When:**
- Client sends GET /api/v1/accounts/:id request

**Then:**
- Database query executes within 50ms (p95)
- Index on accounts(id) is used
- No table scan occurs

**Validation:**
- Prometheus histogram `db_query_duration_seconds{query="get_account_by_id"}` p95 < 0.05
- PostgreSQL EXPLAIN ANALYZE shows index usage
- Query plan does not include "Seq Scan"

---

#### Scenario 8.4: Message Enqueuing Latency

**Given:**
- SMTP client sends message via DATA command
- Message size is 1MB

**When:**
- Client completes DATA transmission with . (dot)

**Then:**
- Message is enqueued to database within 200ms (p95)
- Server responds with 250 OK message queued
- Database INSERT completes successfully

**Validation:**
- Prometheus histogram `smtp_message_enqueue_duration_seconds` p95 < 0.2
- No database deadlocks or transaction conflicts in logs

---

### Category 9: Graceful Shutdown

#### Scenario 9.1: Graceful Shutdown with Active SMTP Connections

**Given:**
- SMTP server has 50 active connections
- Some connections are in the middle of DATA transmission

**When:**
- Server receives SIGTERM signal (e.g., Kubernetes pod termination)

**Then:**
- Server stops accepting new connections (returns 421)
- Server waits up to 30 seconds for active connections to complete
- Completed messages are enqueued successfully
- Incomplete transactions are aborted gracefully
- Server exits after timeout or when all connections close

**Validation:**
- Log entry contains: level=info, event=graceful_shutdown_started
- New connection attempts receive 421 response
- After 30 seconds, server exits with code 0
- All completed messages are in database
- No partial message records exist

---

#### Scenario 9.2: Graceful Shutdown with Active API Requests

**Given:**
- API server is processing 20 concurrent requests
- Some requests are long-running database queries

**When:**
- Server receives SIGTERM signal

**Then:**
- Server stops accepting new requests (returns 503)
- Server waits for active requests to complete (up to 30 seconds)
- Database connections are closed cleanly
- Server exits after all requests complete or timeout

**Validation:**
- Log entry contains: level=info, event=graceful_shutdown_started
- New requests receive 503 Service Unavailable
- Database connections are released to pool
- No hung connections in PostgreSQL

---

## Quality Gate Criteria

### Code Coverage

**Requirement:** 85%+ test coverage per package

**Measured by:**
- `go test -cover ./...`
- Report generated with `go test -coverprofile=coverage.out`
- Critical packages (storage, smtp, api) must exceed 90%

**Coverage Breakdown:**
- `internal/storage`: 90%+
- `internal/smtp`: 88%+
- `internal/api`: 87%+
- `internal/models`: 85%+
- `internal/config`: 80%+

---

### Linter Warnings

**Requirement:** Zero warnings from golangci-lint

**Linters Enabled:**
- `errcheck` - Check error handling
- `gosimple` - Suggest code simplifications
- `govet` - Report suspicious constructs
- `ineffassign` - Detect ineffectual assignments
- `staticcheck` - Go static analysis
- `unused` - Find unused code
- `misspell` - Catch common spelling mistakes

**Validation:**
- `golangci-lint run ./...` exits with code 0
- No warnings in CI/CD pipeline

---

### Security Scan

**Requirement:** Zero high/critical vulnerabilities

**Tools:**
- `gosec` - Go security checker
- `nancy` - Dependency vulnerability scanner (Sonatype)
- `trivy` - Container image scanner

**Validation:**
- `gosec ./...` reports 0 high-severity issues
- `nancy sleuth -p Gopkg.lock` reports 0 vulnerabilities
- `trivy image smtp-proxy:latest` reports 0 high/critical CVEs

---

### Documentation

**Requirement:** Complete documentation for setup and operation

**Required Documents:**
- README.md with setup instructions
- API documentation (OpenAPI/Swagger spec)
- Database schema documentation
- Configuration reference
- Deployment guide (Docker, Kubernetes)

**Validation:**
- All documents exist and are up-to-date
- Code examples are tested and working
- Links to external resources are valid

---

## Acceptance Sign-Off

**Definition of Done:**

- [ ] All 45+ test scenarios pass successfully
- [ ] Code coverage exceeds 85% per package
- [ ] Zero linter warnings (golangci-lint)
- [ ] Zero high/critical security vulnerabilities
- [ ] Performance criteria met (1K connections, 100 req/s API)
- [ ] Graceful shutdown works within 30-second timeout
- [ ] Documentation complete and accurate
- [ ] Docker Compose stack runs locally
- [ ] Kubernetes manifests deploy successfully (if applicable)
- [ ] Prometheus metrics exposed and validated
- [ ] Health check endpoints return correct status

**Approval Required From:**
- Technical Lead: Code review and architecture validation
- QA: Test scenario execution and validation
- DevOps: Deployment and operational readiness
- Security: Security scan results and compliance

**Next Phase:**
Upon acceptance, proceed to:
- SPEC-QUEUE-001: Message Queue Processing implementation
- SPEC-MULTITENANT-001: Multi-Tenant Isolation implementation

---

**Document Version:** 1.0.0
**Last Updated:** 2026-02-15
**Status:** Ready for Implementation
