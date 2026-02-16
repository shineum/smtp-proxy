# Implementation Plan: SPEC-CORE-001

## Overview

This plan outlines the implementation strategy for the Core SMTP Relay and API Server Foundation. The implementation follows a modular approach with three separate binaries sharing common internal packages.

---

## Technology Stack

### Core Dependencies

**Go Modules:**
- `github.com/emersion/go-smtp` v0.16+ - SMTP server implementation
- `github.com/go-chi/chi` v5.0+ - HTTP router with middleware support
- `github.com/jackc/pgx/v5` v5.5+ - PostgreSQL driver and connection pooling
- `github.com/rs/zerolog` v1.32+ - Structured JSON logging
- `github.com/spf13/viper` v1.18+ - Configuration management
- `github.com/golang-migrate/migrate/v4` v4.17+ - Database migrations
- `golang.org/x/crypto/bcrypt` - Password hashing
- `github.com/google/uuid` - UUID generation

**Development Tools:**
- `golangci-lint` v1.55+ - Comprehensive linting
- `go test` - Unit and integration testing
- `air` - Hot-reload for development
- `docker-compose` - Local development stack

### Infrastructure Components

**Required Services:**
- PostgreSQL 15+ with pgx connection pooling
- (Optional) Redis 7+ for session caching
- Docker 24+ for containerization
- Kubernetes 1.28+ for production deployment (optional)

**Monitoring Stack:**
- Prometheus for metrics collection
- Grafana for visualization
- Loki for log aggregation

---

## Architecture Decisions

### Decision 1: Three Separate Binaries

**Rationale:**
- SMTP server and API server have different scaling characteristics
- Independent deployment and rollback capabilities
- Clear separation of concerns (protocol handling vs API management)
- Future: Queue worker can scale independently based on message volume

**Trade-offs:**
- Increased deployment complexity (3 binaries vs 1)
- Shared code requires internal package structure
- Need for service discovery in distributed deployments

### Decision 2: PostgreSQL for Message Queue

**Rationale:**
- Simplifies infrastructure (no separate message broker initially)
- ACID guarantees for message persistence
- Built-in pgx connection pooling for performance
- PostgreSQL 15+ has excellent JSONB performance for metadata

**Trade-offs:**
- Database becomes single point of contention
- Less efficient than dedicated message queues (RabbitMQ, Kafka)
- Migration to dedicated queue system may be needed at scale

**Future Optimization Path:**
- Implement Redis-backed queue for high-throughput scenarios
- Maintain PostgreSQL as durable backup/audit log

### Decision 3: Shared Internal Packages

**Directory Structure:**
```
smtp-proxy/
├── cmd/
│   ├── smtp-server/    # SMTP server binary entry point
│   ├── api-server/     # API server binary entry point
│   └── queue-worker/   # Future: message processor
├── internal/
│   ├── models/         # Shared data models
│   ├── storage/        # Database access layer
│   ├── config/         # Configuration management
│   ├── logger/         # Structured logging setup
│   └── auth/           # Authentication helpers
├── migrations/         # Database schema migrations
├── config/             # Configuration files
└── docker-compose.yml  # Local development stack
```

**Package Boundaries:**
- `models/` - Pure data structures, no business logic
- `storage/` - Database queries using pgx, repository pattern
- `config/` - Viper-based configuration loading
- `logger/` - Zerolog logger initialization with correlation IDs
- `auth/` - Bcrypt password hashing, API key validation

---

## Task Decomposition

### Phase 1: Project Scaffolding and Database (Priority: P0)

**Task 1.1: Initialize Go Module and Directory Structure**
- Create `go.mod` with Go 1.21+
- Set up directory structure (cmd/, internal/, migrations/, config/)
- Create basic README.md with setup instructions
- Initialize Git repository with .gitignore

**Task 1.2: Database Schema Design**
- Write migration files in `migrations/` directory:
  - `001_create_accounts_table.up.sql`
  - `002_create_esp_providers_table.up.sql`
  - `003_create_routing_rules_table.up.sql`
  - `004_create_messages_table.up.sql`
  - `005_create_delivery_logs_table.up.sql`
  - `006_create_indexes.up.sql`
- Implement corresponding `.down.sql` files for rollback
- Test migrations with `migrate` CLI tool

**Task 1.3: Configuration Management**
- Implement `internal/config/config.go`:
  - Define Config struct matching spec.md requirements
  - Load from `config/config.yaml` using viper
  - Support environment variable overrides (DATABASE_URL, etc.)
  - Validate required fields on startup

**Task 1.4: Structured Logging Setup**
- Implement `internal/logger/logger.go`:
  - Initialize zerolog with JSON output
  - Add correlation ID middleware
  - Configure log levels (debug, info, warn, error)
  - Add context-aware logging helpers

### Phase 2: Database Access Layer (Priority: P0)

**Task 2.1: Define Data Models**
- Implement `internal/models/account.go`:
  - Account struct with UUID, Name, Email, PasswordHash, AllowedDomains, APIKey
  - Validation methods (ValidateEmail, ValidateDomains)
- Implement `internal/models/provider.go`:
  - ESPProvider struct with ProviderType enum
  - SMTP config for custom providers
- Implement `internal/models/routing_rule.go`:
  - RoutingRule struct with Priority, Conditions (JSONB)
- Implement `internal/models/message.go`:
  - Message struct with Status enum, Recipients (JSONB)

**Task 2.2: Repository Pattern Implementation**
- Implement `internal/storage/postgres.go`:
  - Initialize pgx connection pool
  - Implement health check methods (Ping, Stats)
- Implement `internal/storage/account_repository.go`:
  - Create(account) - Insert with bcrypt password hashing
  - GetByID(id), GetByAPIKey(apiKey)
  - Update(account), Delete(id)
- Implement `internal/storage/provider_repository.go`:
  - Create, GetByID, ListByAccountID, Update, Delete
- Implement `internal/storage/routing_rule_repository.go`:
  - Create, GetByID, ListByAccountID (ordered by priority), Update, Delete
- Implement `internal/storage/message_repository.go`:
  - Enqueue(message) - Insert with status=queued
  - GetNextBatch(limit) - Fetch messages for processing
  - UpdateStatus(id, status)

**Task 2.3: Unit Tests for Repositories**
- Use testcontainers-go for PostgreSQL integration tests
- Test CRUD operations for all repositories
- Test concurrent access scenarios
- Verify transaction isolation

### Phase 3: SMTP Server Implementation (Priority: P0)

**Task 3.1: SMTP Backend Implementation**
- Implement `internal/smtp/backend.go`:
  - Implement `go-smtp` Backend interface
  - NewSession method for connection handling
  - Session struct implementing smtp.Session interface
- Implement `internal/smtp/session.go`:
  - AuthPlain(username, password) - Validate against accounts table
  - Mail(from, opts) - Validate sender against allowed_domains
  - Rcpt(to) - Validate recipient format (RFC 5321)
  - Data(r) - Parse message, enqueue to database
  - Reset(), Logout() methods

**Task 3.2: SMTP Server Main Binary**
- Implement `cmd/smtp-server/main.go`:
  - Load configuration and initialize logger
  - Initialize database connection pool
  - Create SMTP server with TLS config
  - Listen on port 587
  - Implement graceful shutdown (SIGTERM handler)
- Implement connection limit enforcement
- Add correlation ID to each session

**Task 3.3: SMTP Server Tests**
- Unit tests for Backend and Session
- Integration test: Full SMTP handshake simulation
- Test authentication success/failure scenarios
- Test sender/recipient validation
- Test message enqueuing

### Phase 4: API Server Implementation (Priority: P0)

**Task 4.1: Authentication Middleware**
- Implement `internal/auth/middleware.go`:
  - ValidateAPIKey(apiKey) - Query accounts.api_key
  - HTTP middleware for Bearer token validation
  - Extract account_id into request context

**Task 4.2: API Handlers**
- Implement `internal/api/account_handler.go`:
  - POST /api/v1/accounts - CreateAccount (bcrypt password, generate API key)
  - GET /api/v1/accounts/:id - GetAccount
  - PUT /api/v1/accounts/:id - UpdateAccount
  - DELETE /api/v1/accounts/:id - DeleteAccount
- Implement `internal/api/provider_handler.go`:
  - POST /api/v1/providers - CreateProvider
  - GET /api/v1/providers - ListProviders (filtered by account_id)
  - GET /api/v1/providers/:id - GetProvider
  - PUT /api/v1/providers/:id - UpdateProvider
  - DELETE /api/v1/providers/:id - DeleteProvider
- Implement `internal/api/routing_rule_handler.go`:
  - POST /api/v1/routing-rules - CreateRule
  - GET /api/v1/routing-rules - ListRules (ordered by priority)
  - GET /api/v1/routing-rules/:id - GetRule
  - PUT /api/v1/routing-rules/:id - UpdateRule
  - DELETE /api/v1/routing-rules/:id - DeleteRule
- Implement `internal/api/health_handler.go`:
  - GET /healthz - Return 200 OK
  - GET /readyz - Check database connection
  - GET /metrics - Prometheus metrics (optional)

**Task 4.3: API Server Main Binary**
- Implement `cmd/api-server/main.go`:
  - Load configuration and initialize logger
  - Initialize database connection pool
  - Set up chi router with middleware (CORS, logging, auth)
  - Register API routes
  - Listen on port 8080
  - Implement graceful shutdown

**Task 4.4: API Server Tests**
- Unit tests for handlers with mocked repositories
- Integration tests with testcontainers PostgreSQL
- Test authentication success/failure
- Test CRUD operations for all entities
- Test error handling (400, 401, 404, 500)

### Phase 5: Docker and Deployment (Priority: P1)

**Task 5.1: Dockerfile for Binaries**
- Multi-stage Dockerfile:
  - Build stage with Go 1.21+ Alpine
  - Runtime stage with minimal Alpine base
  - Separate targets for smtp-server and api-server
  - Copy TLS certificates from configurable paths

**Task 5.2: Docker Compose for Local Development**
- `docker-compose.yml`:
  - PostgreSQL service with initialization scripts
  - Redis service (optional, for future caching)
  - smtp-server service with port 587 exposed
  - api-server service with port 8080 exposed
  - Healthchecks for all services
- `docker-compose.override.yml` for development-specific settings

**Task 5.3: Kubernetes Manifests (Optional)**
- Deployment manifests for smtp-server and api-server
- Service definitions with LoadBalancer type
- ConfigMap for configuration
- Secret for TLS certificates and database credentials
- PodDisruptionBudget for high availability
- HorizontalPodAutoscaler based on CPU/memory

### Phase 6: Observability and Metrics (Priority: P2)

**Task 6.1: Prometheus Metrics**
- Implement `internal/metrics/prometheus.go`:
  - SMTP connection counter (accepted, rejected)
  - Active SMTP sessions gauge
  - API request duration histogram
  - Database query duration histogram
  - Message queue depth gauge
- Integrate metrics into SMTP and API servers
- Add /metrics endpoint to API server

**Task 6.2: Structured Logging Enhancements**
- Add correlation ID propagation across SMTP and API layers
- Log sampling for high-throughput scenarios (1% sample rate)
- Log enrichment with account_id, session_id, message_id
- Error tracking with stack traces (zerolog.ErrorStackMarshaler)

**Task 6.3: Health Check Improvements**
- /healthz - Simple liveness check (always 200)
- /readyz - Readiness check:
  - Database connection pool health
  - TLS certificate validity check
  - Available memory above threshold

---

## File Mapping

### SMTP Server

**Primary Files:**
- `cmd/smtp-server/main.go` - Entry point, server setup
- `internal/smtp/backend.go` - SMTP backend implementation
- `internal/smtp/session.go` - SMTP session handling
- `internal/smtp/validator.go` - Email address validation

**Tests:**
- `internal/smtp/backend_test.go`
- `internal/smtp/session_test.go`
- `internal/smtp/integration_test.go`

### API Server

**Primary Files:**
- `cmd/api-server/main.go` - Entry point, HTTP server setup
- `internal/api/router.go` - Chi router configuration
- `internal/api/account_handler.go` - Account CRUD endpoints
- `internal/api/provider_handler.go` - Provider CRUD endpoints
- `internal/api/routing_rule_handler.go` - Routing rule CRUD endpoints
- `internal/api/health_handler.go` - Health and metrics endpoints
- `internal/auth/middleware.go` - API key authentication

**Tests:**
- `internal/api/account_handler_test.go`
- `internal/api/provider_handler_test.go`
- `internal/api/routing_rule_handler_test.go`
- `internal/api/integration_test.go`

### Shared Packages

**Models:**
- `internal/models/account.go`
- `internal/models/provider.go`
- `internal/models/routing_rule.go`
- `internal/models/message.go`

**Storage Layer:**
- `internal/storage/postgres.go` - Connection pool setup
- `internal/storage/account_repository.go`
- `internal/storage/provider_repository.go`
- `internal/storage/routing_rule_repository.go`
- `internal/storage/message_repository.go`
- `internal/storage/repository_test.go` - Integration tests

**Infrastructure:**
- `internal/config/config.go` - Configuration loading
- `internal/logger/logger.go` - Structured logging setup
- `internal/auth/bcrypt.go` - Password hashing utilities
- `internal/auth/apikey.go` - API key generation

**Migrations:**
- `migrations/001_create_accounts_table.up.sql`
- `migrations/001_create_accounts_table.down.sql`
- `migrations/002_create_esp_providers_table.up.sql`
- `migrations/002_create_esp_providers_table.down.sql`
- `migrations/003_create_routing_rules_table.up.sql`
- `migrations/003_create_routing_rules_table.down.sql`
- `migrations/004_create_messages_table.up.sql`
- `migrations/004_create_messages_table.down.sql`
- `migrations/005_create_delivery_logs_table.up.sql`
- `migrations/005_create_delivery_logs_table.down.sql`
- `migrations/006_create_indexes.up.sql`

---

## Dependencies Management

### go.mod Structure

```
module github.com/sungwon/smtp-proxy

go 1.21

require (
    github.com/emersion/go-smtp v0.16.0
    github.com/go-chi/chi/v5 v5.0.11
    github.com/go-chi/cors v1.2.1
    github.com/google/uuid v1.5.0
    github.com/jackc/pgx/v5 v5.5.1
    github.com/rs/zerolog v1.32.0
    github.com/spf13/viper v1.18.2
    github.com/golang-migrate/migrate/v4 v4.17.0
    golang.org/x/crypto v0.17.0
)

require (
    github.com/stretchr/testify v1.8.4 // test
    github.com/testcontainers/testcontainers-go v0.27.0 // test
)
```

### Dependency Justification

- **go-smtp**: Battle-tested SMTP server library with TLS support
- **chi v5**: Lightweight HTTP router, idiomatic Go middleware
- **pgx v5**: High-performance PostgreSQL driver with connection pooling
- **zerolog**: Zero-allocation JSON logger, production-grade
- **viper**: Configuration management with multiple sources (files, env vars)
- **migrate**: Database migration tool with PostgreSQL support
- **testcontainers-go**: Integration testing with real PostgreSQL instances

---

## Risk Analysis and Mitigation

### Risk 1: Database Connection Pool Exhaustion

**Likelihood:** Medium
**Impact:** High (service degradation, connection timeout errors)

**Mitigation Strategies:**
1. Configure pool_max based on PostgreSQL max_connections setting
2. Implement connection timeout (5 seconds) to detect pool exhaustion early
3. Monitor pool usage with Prometheus metrics (active, idle, waiting connections)
4. Set up alerts when pool usage exceeds 80%
5. Implement graceful degradation (return 503 when pool exhausted)

**Implementation:**
- `internal/storage/postgres.go`: Configure pgxpool.Config with Min/Max connections
- Add `pool_stats` Prometheus gauge metric
- Health check endpoint verifies pool availability

### Risk 2: SMTP Connection Floods (DoS Attack)

**Likelihood:** High
**Impact:** High (service disruption, resource exhaustion)

**Mitigation Strategies:**
1. Implement max_connections limit (default: 1000) in SMTP server
2. Add per-IP rate limiting (e.g., 10 connections per minute per IP)
3. Implement connection timeout (30 seconds for idle connections)
4. Deploy fail2ban or cloud-native DDoS protection (e.g., AWS Shield)
5. Monitor connection metrics and alert on anomalies

**Implementation:**
- `internal/smtp/backend.go`: Track active connections with atomic counter
- Reject connections with 421 response when limit exceeded
- Add connection_count Prometheus gauge metric

### Risk 3: Message Queue Growth

**Likelihood:** Medium
**Impact:** Medium (database storage growth, query performance degradation)

**Mitigation Strategies:**
1. Implement message retention policy (e.g., delete delivered messages after 7 days)
2. Archive old messages to object storage (S3, GCS) before deletion
3. Monitor queue depth with Prometheus metrics
4. Set up alerts when queue depth exceeds thresholds (e.g., 10,000 messages)
5. Implement queue worker scaling to process backlog

**Implementation:**
- `internal/storage/message_repository.go`: Add CleanupOldMessages(retentionDays) method
- Add scheduled job (cron) to run cleanup daily
- Add `queue_depth` Prometheus gauge metric by status

### Risk 4: TLS Certificate Expiry

**Likelihood:** Low (with automation)
**Impact:** High (SMTP service disruption, security warnings)

**Mitigation Strategies:**
1. Automate certificate renewal with certbot or cloud provider (ACM, Let's Encrypt)
2. Implement certificate validity check in /readyz endpoint
3. Monitor certificate expiry with Prometheus metrics (days until expiry)
4. Set up alerts 30 days before expiry
5. Use cert-manager in Kubernetes for automated renewal

**Implementation:**
- `internal/api/health_handler.go`: Check TLS cert validity in readiness probe
- Add `tls_cert_expiry_days` Prometheus gauge metric
- Document certificate renewal process in README.md

### Risk 5: API Key Leakage

**Likelihood:** Medium
**Impact:** High (unauthorized access, tenant data exposure)

**Mitigation Strategies:**
1. Generate API keys with cryptographically secure random (crypto/rand)
2. Store API keys hashed in database (bcrypt or SHA-256)
3. Implement API key rotation mechanism
4. Log API key usage with correlation IDs for audit trail
5. Implement rate limiting per API key (100 requests/minute)

**Implementation:**
- `internal/auth/apikey.go`: Generate 32-byte random keys, hex encode
- `internal/storage/account_repository.go`: Store hashed API keys
- Add API key rotation endpoint: POST /api/v1/accounts/:id/rotate-key

---

## Testing Strategy

### Unit Testing

**Coverage Target:** 85% minimum per package

**Key Test Areas:**
- Repository CRUD operations with mocked database
- SMTP session validation logic (sender, recipient, authentication)
- API handler request/response validation
- Configuration loading and validation
- Bcrypt password hashing and verification

**Testing Tools:**
- `testing` package (Go standard library)
- `testify/assert` for assertions
- `testify/mock` for mocking interfaces

### Integration Testing

**Scope:** End-to-end flows with real PostgreSQL

**Key Test Scenarios:**
1. SMTP full handshake: EHLO, AUTH, MAIL FROM, RCPT TO, DATA
2. API account creation → SMTP authentication with new account
3. API provider creation → routing rule creation → message enqueuing
4. Database connection pool behavior under load
5. Graceful shutdown with active connections

**Testing Tools:**
- `testcontainers-go` for PostgreSQL container
- `net/smtp` package for SMTP client simulation
- `httptest` package for API testing

### Load Testing

**Performance Targets:**
- SMTP: 1,000 concurrent connections sustained
- API: 100 requests/second with p95 latency < 100ms
- Database: Query latency p95 < 50ms

**Testing Tools:**
- `k6` or `wrk` for HTTP load testing
- Custom Go script for SMTP connection simulation
- `pgbench` for database performance baseline

**Load Test Scenarios:**
1. Ramp up to 1,000 SMTP connections over 60 seconds
2. Sustained 100 API requests/second for 5 minutes
3. Mixed load: 500 SMTP + 50 API req/s simultaneously

---

## Deployment Strategy

### Local Development

**Setup Steps:**
1. Clone repository and run `go mod download`
2. Start Docker Compose: `docker-compose up -d postgres redis`
3. Run migrations: `migrate -path migrations -database $DATABASE_URL up`
4. Start SMTP server: `go run cmd/smtp-server/main.go`
5. Start API server: `go run cmd/api-server/main.go`

**Hot-Reload:**
- Use `air` for automatic rebuild on code changes
- Configuration hot-reload for connection limits and log levels

### Staging Environment

**Infrastructure:**
- Kubernetes cluster with 2 replicas per service
- PostgreSQL managed instance (AWS RDS, Google Cloud SQL)
- Load balancer with TLS termination
- Prometheus and Grafana for monitoring

**Deployment Process:**
1. Build Docker images with version tags
2. Push images to container registry
3. Apply Kubernetes manifests with `kubectl apply`
4. Verify health checks before routing traffic
5. Monitor metrics for anomalies

### Production Environment

**High Availability:**
- Minimum 3 replicas per service across availability zones
- PostgreSQL with read replicas and automated backups
- PodDisruptionBudget to maintain quorum during updates
- Circuit breaker for database connection failures

**Blue-Green Deployment:**
1. Deploy new version (green) alongside current (blue)
2. Run smoke tests against green environment
3. Gradually shift traffic from blue to green (10%, 50%, 100%)
4. Monitor error rates and rollback if anomalies detected
5. Decommission blue environment after validation

---

## Monitoring and Alerts

### Key Metrics

**SMTP Server:**
- `smtp_connections_total{status="accepted|rejected"}` - Counter
- `smtp_active_sessions` - Gauge
- `smtp_message_enqueued_total` - Counter
- `smtp_auth_attempts_total{result="success|failure"}` - Counter

**API Server:**
- `api_requests_total{method, path, status}` - Counter
- `api_request_duration_seconds{method, path}` - Histogram
- `api_auth_failures_total` - Counter

**Database:**
- `db_connections_active` - Gauge
- `db_connections_idle` - Gauge
- `db_query_duration_seconds{query}` - Histogram
- `db_errors_total{query}` - Counter

**Queue:**
- `queue_depth{status="queued|processing|failed"}` - Gauge
- `queue_processing_duration_seconds` - Histogram

### Alert Conditions

**Critical Alerts:**
- Database connection pool exhausted (95% usage for 5 minutes)
- SMTP server connection limit reached (95% of max_connections)
- API error rate > 5% for 5 minutes
- Queue depth > 10,000 messages for 10 minutes
- TLS certificate expiring in < 7 days

**Warning Alerts:**
- Database connection pool > 80% usage for 10 minutes
- API p95 latency > 200ms for 10 minutes
- Queue depth > 5,000 messages for 10 minutes
- Failed authentication attempts > 100/minute

---

## Success Criteria

### Functional Requirements

- [ ] SMTP server accepts connections on port 587 with STARTTLS
- [ ] SMTP AUTH validates against accounts database
- [ ] SMTP server enqueues messages to database with metadata
- [ ] API server provides CRUD endpoints for accounts, providers, routing rules
- [ ] API authentication validates Bearer tokens against accounts.api_key
- [ ] Database migrations create all required tables and indexes
- [ ] Health check endpoints return correct status

### Performance Requirements

- [ ] SMTP server handles 1,000 concurrent connections
- [ ] API server handles 100 requests/second with p95 < 100ms
- [ ] Database queries execute with p95 < 50ms
- [ ] Message enqueuing latency p95 < 200ms

### Quality Requirements

- [ ] Unit test coverage > 85% per package
- [ ] Integration tests cover all critical paths
- [ ] Zero linter warnings with golangci-lint
- [ ] Graceful shutdown drains connections within 30 seconds
- [ ] No secrets logged or exposed in error messages

### Operational Requirements

- [ ] Docker Compose stack runs locally with single command
- [ ] Prometheus metrics exposed on /metrics endpoint
- [ ] Health check endpoints compatible with Kubernetes probes
- [ ] Configuration hot-reload works for connection limits and log levels
- [ ] README documentation covers setup, development, and deployment

---

**Plan Status:** Approved
**Next Steps:** Begin implementation with Phase 1 (Project Scaffolding and Database)
**Estimated Effort:** 40-60 hours for Phases 1-4, additional 20 hours for Phases 5-6
