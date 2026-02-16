---
id: SPEC-CORE-001
version: "1.0.0"
status: approved
created: "2026-02-15"
updated: "2026-02-15"
author: sungwon
priority: P0
---

# SPEC-CORE-001: Core SMTP Relay and API Server Foundation

## HISTORY

| Version | Date       | Author   | Changes                           |
|---------|------------|----------|-----------------------------------|
| 1.0.0   | 2026-02-15 | sungwon  | Initial SPEC creation             |

---

## OVERVIEW

### Purpose

Establish the foundational SMTP relay server and RESTful API server for the multi-tenant SMTP proxy system. This SPEC defines the core infrastructure components that enable email message acceptance, account management, ESP provider configuration, and routing rule administration.

### Scope

**In Scope:**
- SMTP server accepting connections on port 587 with STARTTLS support
- RESTful API server for account, provider, and routing rule management
- PostgreSQL database schema with pgx connection pooling
- Structured JSON logging with correlation IDs using zerolog
- Configuration management with hot-reload capability
- Health check and metrics endpoints

**Out of Scope:**
- Message queue processing (covered by SPEC-QUEUE-001)
- Multi-tenant isolation logic (covered by SPEC-MULTITENANT-001)
- Email delivery to ESP providers (covered by SPEC-DELIVERY-001)
- Web dashboard UI (covered by SPEC-DASHBOARD-001)

### Business Context

This core foundation enables the SMTP proxy system to:
- Accept incoming SMTP connections from authenticated tenants
- Manage tenant accounts and ESP provider configurations via REST API
- Define routing rules for intelligent message distribution
- Provide operational visibility through logging and metrics

---

## ENVIRONMENT

### Technical Stack

**Programming Language:**
- Go 1.21+ (latest stable version recommended)

**Core Libraries:**
- SMTP Server: `github.com/emersion/go-smtp` v0.16+
- HTTP Router: `github.com/go-chi/chi` v5.0+
- Database Driver: `github.com/jackc/pgx` v5.5+
- Logging: `github.com/rs/zerolog` v1.32+
- Configuration: `github.com/spf13/viper` v1.18+
- Database Migrations: `github.com/golang-migrate/migrate` v4.17+

**Infrastructure:**
- Database: PostgreSQL 15+
- Cache (optional): Redis 7+ for session storage
- Container Runtime: Docker 24+ for local development
- Orchestration: Docker Compose for local stack

**Development Tools:**
- Go version manager: `asdf` or `gvm`
- Linter: `golangci-lint` v1.55+
- Testing: Go standard library `testing` package
- Code formatter: `gofmt` and `goimports`

### External Dependencies

**Required Services:**
- PostgreSQL database accessible at `DATABASE_URL`
- (Optional) Redis instance for session caching at `REDIS_URL`

**Configuration Files:**
- `config/config.yaml` - Application configuration
- `migrations/*.sql` - Database schema migration files
- `.env` - Environment variable overrides for local development

### Deployment Environment

**Development:**
- Docker Compose stack with PostgreSQL and Redis
- Hot-reload enabled for configuration changes
- Debug logging level enabled

**Staging/Production:**
- Kubernetes deployment with health probes
- PostgreSQL managed instance (e.g., AWS RDS, Google Cloud SQL)
- TLS certificates mounted as secrets
- Structured JSON logging to stdout

---

## ASSUMPTIONS

### Technical Assumptions

1. **SMTP Protocol Compliance:**
   - Clients support STARTTLS extension (RFC 3207)
   - Clients implement SMTP AUTH mechanisms (PLAIN, LOGIN)
   - Maximum message size configurable per tenant (default: 25MB)

2. **Database Performance:**
   - PostgreSQL connection pool maintains 20-100 connections
   - Database response time < 50ms for 95th percentile queries
   - Database supports concurrent connections from multiple server instances

3. **Network and Infrastructure:**
   - SMTP port 587 (submission) is accessible and not firewalled
   - TLS certificates are valid and auto-renewed (e.g., Let's Encrypt)
   - Load balancer supports TCP passthrough for SMTP connections

4. **Scalability:**
   - Initial deployment targets 1,000 concurrent SMTP connections
   - API server handles 100 requests/second sustained load
   - Horizontal scaling supported through stateless design

### Business Assumptions

1. **Authentication Model:**
   - Each tenant has unique credentials (username/password)
   - Credentials stored securely with bcrypt hashing (cost factor 12)
   - API access requires Bearer token authentication (JWT or API keys)

2. **Operational Requirements:**
   - System availability target: 99.9% uptime
   - Graceful shutdown drains connections within 30 seconds
   - Configuration changes apply without full restart where possible

3. **Compliance and Security:**
   - No email body content logged (GDPR/privacy compliance)
   - Audit trail maintained for account and routing rule changes
   - TLS 1.2+ required for all SMTP connections

---

## REQUIREMENTS

### Ubiquitous Requirements (Always Active)

**R-CORE-001: Structured Logging**
- The system SHALL always log incoming connections with timestamp, client IP, and authentication status
- WHY: Operational visibility and security audit trail
- IMPACT: Missing logs prevent troubleshooting and security incident investigation

**R-CORE-002: API Request Validation**
- The API server SHALL always validate request format and return proper HTTP status codes (400 for malformed requests, 401 for unauthorized, 500 for server errors)
- WHY: Clear error communication and API contract enforcement
- IMPACT: Ambiguous errors cause client integration failures

**R-CORE-003: Database Connection Pool**
- The system SHALL always maintain PostgreSQL connection pool via pgx with configurable min/max connections
- WHY: Database performance and resource efficiency
- IMPACT: Missing connection pooling causes database connection exhaustion

**R-CORE-004: Correlation ID Tracking**
- The system SHALL always use structured JSON logging via zerolog with correlation IDs for request tracing
- WHY: Distributed tracing across SMTP and API requests
- IMPACT: Debugging failures in multi-component workflows becomes impossible without correlation

### Event-Driven Requirements (Trigger-Response)

**R-CORE-005: SMTP Connection Initiation**
- WHEN client connects on port 587, THEN system SHALL initiate STARTTLS handshake
- WHY: Security requirement to encrypt SMTP traffic
- IMPACT: Plaintext SMTP exposes credentials and message metadata

**R-CORE-006: EHLO Command Handling**
- WHEN EHLO command received, THEN system SHALL return supported extensions including AUTH, STARTTLS, SIZE, and PIPELINING
- WHY: SMTP protocol compliance and capability advertisement
- IMPACT: Clients cannot negotiate features without proper EHLO response

**R-CORE-007: Sender Validation**
- WHEN MAIL FROM received, THEN system SHALL validate sender against account's allowed domains
- WHY: Prevent unauthorized sender spoofing
- IMPACT: Missing validation allows tenant accounts to send from arbitrary domains

**R-CORE-008: Recipient Format Validation**
- WHEN RCPT TO received, THEN system SHALL validate recipient format per RFC 5321
- WHY: Email address format compliance
- IMPACT: Malformed recipients cause downstream ESP delivery failures

**R-CORE-009: Message Enqueuing**
- WHEN DATA command completes, THEN system SHALL enqueue message with metadata (sender, recipients, tenant_id, timestamp)
- WHY: Decouple message acceptance from delivery processing
- IMPACT: Synchronous delivery blocks SMTP server and reduces throughput

**R-CORE-010: Account Creation**
- WHEN API receives POST /api/v1/accounts, THEN system SHALL create account with validated fields (name, email, allowed_domains)
- WHY: Enable tenant onboarding via API
- IMPACT: Manual account creation does not scale

**R-CORE-011: Provider List Retrieval**
- WHEN API receives GET /api/v1/providers, THEN system SHALL return ESP provider list for authenticated tenant
- WHY: Tenant configuration visibility
- IMPACT: Tenants cannot configure routing without provider visibility

**R-CORE-012: Routing Rule Creation**
- WHEN API receives POST /api/v1/routing-rules, THEN system SHALL create routing rule with priority ordering
- WHY: Enable intelligent message distribution
- IMPACT: Static routing prevents load balancing and failover

### State-Driven Requirements (Conditional)

**R-CORE-013: AUTH Command Authorization**
- IF SMTP connection has valid TLS, THEN system SHALL allow AUTH command
- WHY: Prevent credential transmission over plaintext
- IMPACT: Plaintext authentication exposes credentials to network sniffing

**R-CORE-014: Connection Limit Enforcement**
- IF connection count exceeds configured max (default 1000), THEN system SHALL reject with 421 response
- WHY: Protect server resources from connection exhaustion
- IMPACT: Unlimited connections cause memory exhaustion and service disruption

**R-CORE-015: Database Connection Failure Handling**
- IF database connection fails, THEN API SHALL return 503 Service Unavailable with Retry-After header
- WHY: Graceful degradation during database outages
- IMPACT: Returning 500 errors without retry guidance causes client retry storms

**R-CORE-016: Configuration Hot-Reload**
- IF configuration file changes, THEN system SHALL hot-reload without restart (where supported: connection limits, logging levels)
- WHY: Operational flexibility without downtime
- IMPACT: Configuration changes require full restarts and connection draining

### Unwanted Requirements (Prohibitions)

**R-CORE-017: No Plaintext AUTH**
- The system SHALL NOT accept SMTP AUTH over unencrypted connections
- WHY: Credential security
- IMPACT: Plaintext credentials vulnerable to interception

**R-CORE-018: No Email Body Logging**
- The system SHALL NOT log email body content or attachments
- WHY: GDPR compliance and privacy protection
- IMPACT: Logging email bodies creates legal liability

**R-CORE-019: No Internal Error Exposure**
- The system SHALL NOT expose internal error details in API responses (use error codes instead)
- WHY: Security best practice to prevent information disclosure
- IMPACT: Detailed error messages reveal system internals to attackers

**R-CORE-020: No SQL Injection**
- The system SHALL NOT allow SQL injection through any input path (use parameterized queries)
- WHY: Security vulnerability prevention
- IMPACT: SQL injection enables data exfiltration and privilege escalation

### Optional Requirements (Enhancements)

**R-CORE-021: Prometheus Metrics**
- WHERE possible, system SHALL provide /metrics endpoint for Prometheus scraping
- WHY: Observability integration with modern monitoring stacks
- IMPACT: Missing metrics reduce operational visibility

**R-CORE-022: Graceful Shutdown**
- WHERE possible, system SHALL support graceful shutdown with connection draining (30-second timeout)
- WHY: Zero-downtime deployments
- IMPACT: Abrupt shutdowns cause in-flight message loss

**R-CORE-023: Kubernetes Probes**
- WHERE possible, system SHALL provide /healthz and /readyz endpoints for Kubernetes probes
- WHY: Container orchestration integration
- IMPACT: Missing probes cause traffic routing to unhealthy instances

---

## SPECIFICATIONS

### Architecture Design

**Component Structure:**

The system consists of three separate binaries with shared internal packages:

1. **smtp-server** - SMTP protocol handler
   - Listens on port 587
   - Handles STARTTLS, AUTH, MAIL FROM, RCPT TO, DATA commands
   - Enqueues messages to database

2. **api-server** - RESTful API handler
   - Listens on port 8080
   - Provides CRUD endpoints for accounts, providers, routing rules
   - JWT-based authentication

3. **queue-worker** - Message processing daemon (future SPEC)
   - Dequeues messages from database
   - Applies routing rules
   - Delivers to ESP providers

**Shared Packages:**
- `internal/models` - Data models (Account, Provider, RoutingRule, Message)
- `internal/storage` - Database access layer with pgx
- `internal/config` - Configuration management with viper
- `internal/logger` - Structured logging with zerolog

### Database Schema

**Core Tables:**

1. **accounts** - Tenant account information
   - id (UUID, primary key)
   - name (VARCHAR, unique)
   - email (VARCHAR)
   - password_hash (VARCHAR, bcrypt)
   - allowed_domains (JSONB array)
   - api_key (VARCHAR, unique, indexed)
   - created_at, updated_at (TIMESTAMP)

2. **esp_providers** - Email service provider configurations
   - id (UUID, primary key)
   - account_id (UUID, foreign key to accounts)
   - name (VARCHAR)
   - provider_type (ENUM: sendgrid, mailgun, ses, smtp)
   - api_key (VARCHAR, encrypted)
   - smtp_config (JSONB, for custom SMTP)
   - enabled (BOOLEAN, default true)
   - created_at, updated_at (TIMESTAMP)

3. **routing_rules** - Message routing logic
   - id (UUID, primary key)
   - account_id (UUID, foreign key to accounts)
   - priority (INTEGER, lower = higher priority)
   - conditions (JSONB, matching criteria)
   - provider_id (UUID, foreign key to esp_providers)
   - enabled (BOOLEAN, default true)
   - created_at, updated_at (TIMESTAMP)

4. **messages** - Queued messages for processing
   - id (UUID, primary key)
   - account_id (UUID, foreign key to accounts)
   - sender (VARCHAR)
   - recipients (JSONB array)
   - subject (VARCHAR, extracted from headers)
   - headers (JSONB)
   - body (TEXT, encrypted)
   - status (ENUM: queued, processing, delivered, failed)
   - provider_id (UUID, foreign key to esp_providers, nullable)
   - enqueued_at, processed_at (TIMESTAMP)

5. **delivery_logs** - Audit trail for message delivery
   - id (UUID, primary key)
   - message_id (UUID, foreign key to messages)
   - provider_id (UUID, foreign key to esp_providers)
   - status (VARCHAR)
   - response_code (INTEGER)
   - response_body (TEXT)
   - delivered_at (TIMESTAMP)

**Indexes:**
- accounts(api_key) - API authentication lookup
- messages(account_id, status) - Queue processing queries
- routing_rules(account_id, priority) - Rule evaluation ordering
- delivery_logs(message_id) - Message status tracking

### API Endpoints

**Authentication:**
- All API endpoints require `Authorization: Bearer <api_key>` header
- API key validated against accounts.api_key column

**Account Management:**
- POST /api/v1/accounts - Create tenant account
- GET /api/v1/accounts/:id - Retrieve account details
- PUT /api/v1/accounts/:id - Update account settings
- DELETE /api/v1/accounts/:id - Deactivate account

**Provider Management:**
- POST /api/v1/providers - Add ESP provider configuration
- GET /api/v1/providers - List providers for authenticated tenant
- GET /api/v1/providers/:id - Retrieve provider details
- PUT /api/v1/providers/:id - Update provider settings
- DELETE /api/v1/providers/:id - Remove provider

**Routing Rules:**
- POST /api/v1/routing-rules - Create routing rule
- GET /api/v1/routing-rules - List rules for authenticated tenant
- GET /api/v1/routing-rules/:id - Retrieve rule details
- PUT /api/v1/routing-rules/:id - Update rule conditions/priority
- DELETE /api/v1/routing-rules/:id - Delete rule

**Health and Metrics:**
- GET /healthz - Health check (returns 200 OK)
- GET /readyz - Readiness check (validates database connection)
- GET /metrics - Prometheus metrics (optional)

### Configuration Format

**config/config.yaml:**
```yaml
smtp:
  host: 0.0.0.0
  port: 587
  max_connections: 1000
  read_timeout: 30s
  write_timeout: 30s
  max_message_size: 26214400  # 25MB

api:
  host: 0.0.0.0
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

database:
  url: ${DATABASE_URL}
  pool_min: 20
  pool_max: 100
  connect_timeout: 5s

logging:
  level: info  # debug, info, warn, error
  format: json

tls:
  cert_file: /etc/smtp-proxy/tls/cert.pem
  key_file: /etc/smtp-proxy/tls/key.pem
```

### Security Controls

1. **Password Storage:**
   - Use bcrypt with cost factor 12 for password hashing
   - API keys generated with crypto/rand (32 bytes, hex encoded)

2. **TLS Configuration:**
   - Minimum TLS 1.2
   - Cipher suites: TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256

3. **Input Validation:**
   - Email address format validation using net/mail package
   - JSON schema validation for API requests
   - SQL injection prevention via parameterized queries

4. **Rate Limiting:**
   - Per-tenant connection limits (configurable)
   - API rate limiting: 100 requests/minute per API key

---

## TRACEABILITY

**Related SPECs:**
- SPEC-QUEUE-001: Message Queue Processing (depends on SPEC-CORE-001)
- SPEC-MULTITENANT-001: Multi-Tenant Isolation (extends SPEC-CORE-001)
- SPEC-DELIVERY-001: ESP Provider Integration (consumes messages from SPEC-CORE-001)

**External References:**
- RFC 5321: Simple Mail Transfer Protocol
- RFC 3207: SMTP Service Extension for Secure SMTP over TLS
- RFC 4954: SMTP Service Extension for Authentication
- OWASP Top 10: Security best practices

---

## ACCEPTANCE CRITERIA

See `acceptance.md` for detailed test scenarios.

---

## RISKS AND MITIGATION

**Risk 1: Database Connection Pool Exhaustion**
- Likelihood: Medium
- Impact: High (service disruption)
- Mitigation: Monitor connection pool usage, implement connection timeout, set max_connections limit

**Risk 2: SMTP Connection Floods**
- Likelihood: High (during attacks)
- Impact: High (resource exhaustion)
- Mitigation: Implement rate limiting per IP, connection limits, fail2ban integration

**Risk 3: TLS Certificate Expiry**
- Likelihood: Low (with automation)
- Impact: High (SMTP service disruption)
- Mitigation: Automated certificate renewal (certbot), monitoring for expiry dates

**Risk 4: Message Queue Growth**
- Likelihood: Medium
- Impact: Medium (database storage growth)
- Mitigation: Implement message retention policy, monitor queue depth, alert on thresholds

---

**SPEC Status:** Approved
**Next Steps:** Proceed to implementation via `/moai:2-run SPEC-CORE-001`
