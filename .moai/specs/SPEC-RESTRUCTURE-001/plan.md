---
id: SPEC-RESTRUCTURE-001
title: Full Architecture Restructuring - Implementation Plan
version: 0.2.0
status: draft
created: 2026-02-25
updated: 2026-02-25
author: sungwon
---

## Implementation Plan

This plan covers the full architecture restructuring of smtp-proxy into 4 separated parts with a pluggable message storage layer. Phases are ordered by dependency; later phases build on earlier ones.

---

## Phase 1: Message Storage Interface

**Priority:** Primary Goal (all other phases depend on this)

**Scope:** Create the `msgstore` package with `MessageStore` interface, `LocalFileStore`, and `S3Store` implementations.

**Requirements Covered:** REQ-MSI-001 through REQ-MSI-008

### Tasks

1. **Create `server/internal/msgstore/store.go`**
   - Define `MessageStore` interface with `Put`, `Get`, `Delete` methods
   - Define `ErrNotFound` sentinel error
   - Define `Config` struct for storage configuration
   - Define `New(cfg Config) (MessageStore, error)` factory function that reads `STORAGE_TYPE`

2. **Create `server/internal/msgstore/local.go`**
   - Implement `LocalFileStore` struct
   - `Put`: Write file to `{basePath}/{messageID}` with atomic write (write to temp, rename)
   - `Get`: Read file from `{basePath}/{messageID}`, return `ErrNotFound` if missing
   - `Delete`: Remove file, return nil if already absent
   - Auto-create base directory on initialization (including parents)

3. **Create `server/internal/msgstore/s3.go`**
   - Implement `S3Store` struct using AWS SDK Go v2
   - `Put`: PutObject to `{bucket}/{prefix}{messageID}`
   - `Get`: GetObject, return `ErrNotFound` on NoSuchKey
   - `Delete`: DeleteObject (idempotent)
   - Support `S3_ENDPOINT` for MinIO compatibility in dev/test

4. **Create comprehensive tests**
   - `local_test.go`: Test Put/Get/Delete, missing file, directory creation, concurrent access
   - `s3_test.go`: Test with mocked S3 client, test with real MinIO in integration tests

5. **Extend `server/internal/config/`**
   - Add `STORAGE_TYPE`, `STORAGE_PATH`, `S3_BUCKET`, `S3_PREFIX`, `S3_ENDPOINT`, `S3_REGION` configuration fields

### Technical Approach

- Use `os.MkdirAll` for directory creation in LocalFileStore
- Use `os.WriteFile` with temp file + `os.Rename` for atomic writes (prevents partial reads)
- Use `aws-sdk-go-v2/service/s3` for S3 operations
- Use `context.Context` for all operations to support timeouts and cancellation
- Keep the interface minimal; avoid premature abstraction

### Architecture Decision

The `msgstore` package is deliberately separate from `storage` (which is the sqlc-generated PostgreSQL layer). This separation prevents confusion between "metadata storage" (PostgreSQL) and "body storage" (filesystem/S3).

---

## Phase 2: SMTP Server Refactoring

**Priority:** Primary Goal

**Scope:** Refactor SMTP session flow to use MessageStore and remove sync delivery.

**Requirements Covered:** REQ-SMTP-001 through REQ-SMTP-006

### Tasks

1. **Remove sync delivery mode**
   - Delete `server/internal/delivery/sync.go` and `sync_test.go`
   - Remove `SyncService` from the `delivery` package
   - Update `delivery.Service` interface if needed (or simplify to async-only)
   - Remove sync-mode configuration and flags from `cmd/smtp-server/main.go`

2. **Inject MessageStore into SMTP session**
   - Add `msgstore.MessageStore` field to `smtp.Backend` struct
   - Pass MessageStore to `Session` during creation
   - Update `cmd/smtp-server/main.go` to initialize MessageStore from config

3. **Refactor `session.go` DATA handler**
   - After parsing message and inserting metadata to PostgreSQL:
     a. Call `messageStore.Put(ctx, messageID.String(), rawBody)`
     b. On Put failure: delete/mark metadata record, return SMTP 451
   - Change Redis enqueue payload to contain only `{id, account_id, tenant_id}`
   - On enqueue failure: update status to `enqueue_failed`, return SMTP 451

4. **Update delivery.Request struct**
   - Remove `Body []byte` field (body is now in storage, not passed inline)
   - Remove `Subject` and `Headers` fields if worker reads from DB

5. **Update tests**
   - Update `session_test.go` to mock MessageStore
   - Update integration tests for new flow
   - Verify error handling paths (storage failure, enqueue failure)

### Technical Approach

- The SMTP session already does: auth -> validate -> persist to DB -> enqueue
- We add one step between persist and enqueue: store body in MessageStore
- The key principle is "fail fast, clean up": if any step fails, undo previous steps
- Use database transaction for metadata insert to enable rollback on storage failure

### Risk: In-Flight Messages During Migration

Messages that were enqueued with full body (old format) before migration must still be processable. The queue worker should handle both old (full payload) and new (ID-only) formats during a transition period.

---

## Phase 3: Queue Worker Refactoring

**Priority:** Primary Goal

**Scope:** Refactor queue worker to fetch message body from MessageStore instead of queue payload.

**Requirements Covered:** REQ-QW-001 through REQ-QW-005

### Tasks

1. **Inject MessageStore into worker Handler**
   - Add `msgstore.MessageStore` field to `worker.Handler`
   - Update `NewHandler` constructor to accept MessageStore
   - Update `cmd/queue-worker/main.go` to initialize MessageStore

2. **Refactor `handler.go` HandleMessage**
   - After fetching metadata from PostgreSQL, call `messageStore.Get(ctx, messageID)`
   - Build `provider.Message` from DB metadata + storage body
   - Handle `ErrNotFound`: retry 3x with exponential backoff, then DLQ
   - Handle orphaned message_id: log warning, acknowledge without delivery

3. **Add backward compatibility layer**
   - Check if queue message contains body field (old format)
   - If body present: use inline body (old behavior)
   - If body absent: fetch from MessageStore (new behavior)
   - Remove backward compatibility after migration window

4. **Update queue.Message struct**
   - Simplify payload fields for new format
   - Keep old fields as optional for backward compatibility

5. **Update tests**
   - Update `handler_test.go` to mock MessageStore
   - Test storage read failure with retry
   - Test orphaned message_id handling
   - Test backward compatibility with old format

### Technical Approach

- Retry with exponential backoff: 1s, 2s, 4s for storage read failures
- Use existing DLQ mechanism for permanently failed storage reads
- The provider resolution and ESP delivery logic remain completely unchanged
- Only the "where does the body come from" changes

---

## Phase 3.5: Delivery Logging, Retry & Observability

**Priority:** Primary Goal (integrated with Phases 2-3)

**Scope:** Enqueue retry logic, delivery result logging, DB statistics, configurable log output (file/CloudWatch).

**Requirements Covered:** REQ-LOG-001 through REQ-LOG-013

### Tasks

1. **Enqueue retry logic in SMTP server**
   - Add retry wrapper in `smtp/session.go` DATA handler for Redis XADD
   - Exponential backoff: 500ms, 1s, 2s (3 attempts max)
   - Log each retry attempt with message_id, attempt number, error
   - Only mark `enqueue_failed` after all retries exhausted

2. **Delivery result logging in queue worker**
   - After each provider.Send() call, record delivery log entry in PostgreSQL `delivery_logs`
   - Success: status `delivered`, provider name, duration_ms
   - Failure: status `failed`, provider name, error message, attempt number, duration_ms
   - DLQ: status `dlq`, total retry count, all error reasons
   - Structured JSON log output at appropriate log levels (INFO/ERROR/WARN)

3. **Extend delivery_logs table**
   - Ensure fields: id, message_id, account_id, tenant_id, provider_name, status, error_message, attempt_number, duration_ms, created_at
   - Add indexes for stats queries: (account_id, created_at), (tenant_id, created_at), (provider_name, status)
   - Add migration file for any schema changes

4. **Configurable log output**
   - Extend `logger/` package to support multiple output targets
   - `stdout` (default): JSON to stdout via zerolog (existing behavior)
   - `file`: JSON to configurable file path with log rotation (using lumberjack or similar)
   - `cloudwatch`: JSON to AWS CloudWatch Logs (using AWS SDK Go v2)
   - Configuration via `LOG_OUTPUT`, `LOG_FILE_PATH`, `LOG_MAX_SIZE_MB`, `LOG_MAX_FILES`, `LOG_CW_GROUP`, `LOG_CW_STREAM`
   - Factory function: `NewLogger(cfg LogConfig) zerolog.Logger`

5. **Delivery statistics query support**
   - Add sqlc queries for aggregate statistics: total by status, by provider, by account, by time range
   - These queries power the FastAPI /delivery-logs/stats endpoint (Phase 4)

6. **Tests**
   - Test enqueue retry: mock Redis to fail N times then succeed
   - Test delivery logging: verify DB records after success/failure/DLQ
   - Test log output configuration: file creation, rotation behavior
   - Test CloudWatch integration with mocked AWS client

### Technical Approach

- Use `github.com/natefinch/lumberjack/v2` for log file rotation
- Use `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs` for CloudWatch
- zerolog supports io.MultiWriter for writing to multiple outputs simultaneously
- Delivery statistics use existing `delivery_logs` table with added indexes
- No separate statistics table needed; aggregate queries on `delivery_logs` are sufficient

### Files Created/Modified

| Action | File | Purpose |
|--------|------|---------|
| MODIFY | `server/internal/logger/logger.go` | Add file and CloudWatch output targets |
| CREATE | `server/internal/logger/file.go` | File output with log rotation |
| CREATE | `server/internal/logger/cloudwatch.go` | CloudWatch Logs output |
| CREATE | `server/internal/logger/file_test.go` | Tests |
| CREATE | `server/internal/logger/cloudwatch_test.go` | Tests |
| MODIFY | `server/internal/smtp/session.go` | Add enqueue retry logic |
| MODIFY | `server/internal/worker/handler.go` | Add delivery result logging |
| MODIFY | `server/internal/config/config.go` | Add LOG_* config fields |
| CREATE | `server/internal/storage/migrations/NNN_add_delivery_log_indexes.up.sql` | DB indexes |

---

## Phase 4: FastAPI Admin Backend

**Priority:** Secondary Goal

**Scope:** Create FastAPI application replacing the Go API server for admin operations.

**Requirements Covered:** REQ-ADMIN-001 through REQ-ADMIN-009

### Tasks

1. **Project setup (`admin/`)**
   - Initialize Python project with `pyproject.toml` (using `uv` or `poetry`)
   - Dependencies: fastapi, uvicorn, asyncpg, sqlalchemy[asyncio], pydantic, pydantic-settings, python-jose (JWT), passlib (password hashing), alembic
   - Create Dockerfile for containerized deployment
   - Create `app/main.py` with FastAPI app, CORS middleware, exception handlers

2. **Database layer**
   - Create `app/database.py` with asyncpg connection pool or SQLAlchemy async engine
   - Create `app/models/` mirroring the existing PostgreSQL schema
   - Do NOT run migrations from FastAPI; migrations are managed by `golang-migrate` in `server/`
   - Read-only awareness: FastAPI writes to accounts, tenants, users, providers, routing_rules tables; reads delivery_logs

3. **Authentication and RBAC**
   - `app/auth/jwt.py`: JWT creation (access + refresh tokens), validation, secret configuration
   - `app/auth/dependencies.py`: FastAPI `Depends()` for extracting current user from JWT
   - `app/auth/rbac.py`: Role checking decorator/dependency (owner, admin, user, viewer)
   - Password hashing with bcrypt via passlib

4. **API routers**
   - `app/routers/auth.py`: Login, refresh, logout endpoints
   - `app/routers/accounts.py`: CRUD for accounts
   - `app/routers/tenants.py`: CRUD for tenants
   - `app/routers/users.py`: CRUD for users with role management
   - `app/routers/providers.py`: CRUD for ESP provider configurations
   - `app/routers/routing_rules.py`: CRUD for routing rules
   - `app/routers/delivery_logs.py`: Read-only log viewing with stats aggregation
   - All list endpoints: pagination, sorting, filtering support

5. **Pydantic schemas**
   - Request/response schemas for all resources
   - Validation rules matching existing DB constraints
   - Consistent error response format

6. **Tests**
   - pytest + httpx for API testing
   - pytest-asyncio for async test support
   - Fixture factories for test data
   - Test RBAC enforcement for each endpoint

### Technical Approach

- FastAPI async with asyncpg provides high-performance database access
- Pydantic v2 for request validation and response serialization
- SQLAlchemy models reflect existing schema (no schema changes from FastAPI)
- JWT secret shared between FastAPI and Go services (if cross-service auth needed) OR independent auth (separate user tables for admin users)

### Architecture Decision: Independent Admin Auth

The admin interface uses its own `users` table (already exists in schema) with its own JWT tokens. Go SMTP auth uses account credentials (different authentication path). No token sharing needed between services.

---

## Phase 5: React Admin Frontend

**Priority:** Secondary Goal

**Scope:** Create React SPA for admin dashboard consuming the FastAPI backend.

**Requirements Covered:** REQ-ADMIN-010 through REQ-ADMIN-013

### Tasks

1. **Project setup (`frontend/`)**
   - Initialize with Vite + React + TypeScript template
   - Dependencies: react, react-router-dom, @tanstack/react-query, axios, tailwindcss, shadcn/ui, recharts, react-hook-form, zod
   - Create Dockerfile with multi-stage build (build + nginx)

2. **Core infrastructure**
   - `lib/api-client.ts`: Axios instance with JWT interceptor, refresh token logic
   - `lib/auth.ts`: Login/logout/token management utilities
   - `hooks/use-auth.ts`: Authentication hook with context provider
   - `types/api.ts`: TypeScript types matching FastAPI schemas

3. **Pages and routing**
   - Login page with form validation
   - Dashboard with metrics overview cards and charts
   - Accounts CRUD page with table and form dialogs
   - Tenants CRUD page
   - Users management page with role assignment
   - Providers configuration page
   - Routing Rules page with priority drag-and-drop
   - Delivery Logs page with search, filter, pagination
   - Analytics page with delivery trend charts

4. **Component library**
   - Initialize shadcn/ui components (Button, Table, Dialog, Form, Input, Select, Badge)
   - Dashboard metrics cards
   - Data table with sorting, filtering, pagination
   - Chart wrappers using Recharts

5. **RBAC in frontend**
   - Parse user role from JWT claims
   - Conditional rendering based on role permissions
   - Disable/hide actions not permitted by role

6. **Tests**
   - Vitest for unit tests
   - React Testing Library for component tests
   - Playwright for E2E tests

### Technical Approach

- Vite for fast development builds (faster than CRA/webpack)
- TanStack Query for server state management (caching, background refetch)
- shadcn/ui for accessible, customizable components
- React Router v6 with nested layouts for dashboard

---

## Phase 6: Integration Testing and Docker Compose

**Priority:** Secondary Goal

**Scope:** Create unified Docker Compose configuration and integration tests.

**Requirements Covered:** REQ-DEPLOY-001 through REQ-DEPLOY-006

### Tasks

1. **Create/update `docker-compose.yml`**
   - Define all services: postgres, redis, smtp-server, queue-worker, admin-api, admin-frontend
   - Configure shared network
   - Create `.env.example` with all required variables
   - Add volume mounts for local message storage
   - Add health checks for all services

2. **Database migration strategy**
   - Migrations run via golang-migrate (existing)
   - Add migration step to docker-compose (init container or startup command)
   - Ensure migrations complete before other services start (depends_on with health check)

3. **Integration test suite**
   - End-to-end SMTP flow: connect -> auth -> send -> verify queued -> verify stored -> verify delivered
   - Admin API integration: create account -> configure provider -> verify SMTP can use it
   - Cross-service verification: admin creates config, SMTP server uses it, worker delivers

4. **Local development documentation**
   - Update README with new setup instructions
   - Document environment variables
   - Provide development workflow guide

### Technical Approach

- Docker Compose v2 with health checks and depends_on conditions
- Use `docker compose up -d --build` for full stack startup
- MinIO container for S3-compatible storage testing locally
- Testcontainers for integration tests (Go and Python)

---

## Phase 7: API Server Placeholder

**Priority:** Final Goal (Lowest Priority)

**Scope:** Simplify the existing Go API server to a health-check-only placeholder.

**Requirements Covered:** REQ-API-001, REQ-API-002

### Tasks

1. **Strip `cmd/api-server/main.go`**
   - Remove all existing Chi router routes
   - Keep only health check endpoint (`GET /health`)
   - Remove dependency on `internal/api/` package

2. **Remove or archive `internal/api/`**
   - Remove `internal/api/` handlers and middleware
   - Or move to `internal/api_deprecated/` if reference needed

3. **Remove Go admin auth code**
   - Remove admin-specific auth from `internal/auth/` (keep SMTP auth)
   - Clean up any admin-only middleware

4. **Update documentation**
   - Note that API server is placeholder pending SPEC-API-001
   - Document that admin operations are now via FastAPI

---

## Risk Analysis

### Risk 1: In-Flight Message Migration

**Risk:** Messages enqueued with old format (full body in Redis) may be lost or unprocessable during migration.

**Mitigation:**
- Queue worker supports both old and new message formats during transition
- Deploy worker with backward compatibility BEFORE deploying new SMTP server
- Drain old-format messages before removing backward compatibility code
- Migration order: Phase 1 (storage) -> Phase 3 (worker with compat) -> Phase 2 (SMTP server) -> Remove compat

### Risk 2: Shared Database Contention

**Risk:** FastAPI and Go services accessing the same PostgreSQL database may cause connection pool exhaustion or lock contention.

**Mitigation:**
- Each service uses its own connection pool with configurable size
- FastAPI uses read-heavy queries (delivery logs) with read replica support
- Monitor connection counts and query performance during integration testing
- PostgreSQL connection limit set to accommodate all services (default 100 connections)

### Risk 3: Storage Reliability

**Risk:** Message body storage failure (filesystem full, S3 outage) could cause message loss.

**Mitigation:**
- SMTP server returns 451 (temp error) on storage failure; client will retry
- Worker retries storage reads 3x before DLQ routing
- Monitor storage capacity and availability
- LocalFileStore: disk space alerts; S3Store: multi-AZ bucket replication

### Risk 4: Technology Stack Expansion

**Risk:** Adding Python (FastAPI) and TypeScript (React) to a Go project increases operational and hiring complexity.

**Mitigation:**
- Clear separation of concerns: Go for SMTP/queue (performance-critical), Python for admin (CRUD-heavy), React for UI
- Each technology has its own Dockerfile and CI pipeline
- Team can specialize by component
- Docker Compose abstracts deployment complexity

### Risk 5: Data Consistency Between DB and Storage

**Risk:** Message metadata exists in PostgreSQL but body is missing from storage (or vice versa) due to partial failures.

**Mitigation:**
- SMTP server uses write-then-verify: persist metadata -> store body -> enqueue (rollback on failure)
- Worker handles missing body gracefully (retry + DLQ)
- Periodic consistency check job (compare metadata records with storage contents)
- Status field tracks pipeline progress: queued, processing, delivered, failed, storage_error

---

## Migration Strategy

### Phase Ordering for Zero-Downtime Migration

```
Step 1: Deploy Phase 1 (msgstore package) - No runtime changes
Step 2: Deploy Phase 3 (worker with backward compat) - Worker handles both formats
Step 3: Deploy Phase 2 (SMTP server with storage) - New messages use storage
Step 4: Wait for old-format queue to drain completely
Step 5: Run DB migration to drop body column
Step 6: Remove backward compat from worker
Step 7: Deploy Phases 4-7 independently (no dependency on Go services)
```

### Rollback Plan

- **Phase 1 rollback:** No runtime impact; just remove unused package
- **Phase 2 rollback:** Revert SMTP server to previous version; worker handles both formats
- **Phase 3 rollback:** Revert worker; new-format messages will fail (need manual intervention)
- **Phase 4-7 rollback:** Independent services; roll back individually

---

## Dependencies Between Phases

```
Phase 1 (Message Storage) ──┬──> Phase 2 (SMTP Refactor) ──┐
                            └──> Phase 3 (Worker Refactor) ─┤
                                                            └──> Phase 3.5 (Logging & Retry)
Phase 4 (FastAPI) ──────────────> Phase 6 (Integration)
Phase 5 (React) ────────────────> Phase 6 (Integration)
                                        │
Phase 7 (API Placeholder) ─────> Phase 6 (Integration)
```

- Phases 1, 4, 5 can start in parallel
- Phases 2 and 3 require Phase 1 completion
- Phase 3.5 integrates into Phases 2 and 3 (can be done during or immediately after)
- Phase 6 requires Phases 2, 3, 3.5, 4, 5 completion
- Phase 7 can proceed independently

---

## Expert Consultation Recommendations

| Domain | Agent | Reason |
|--------|-------|--------|
| Backend (Go) | expert-backend | MessageStore interface design, SMTP session refactoring, worker handler changes |
| Frontend (React) | expert-frontend | React SPA architecture, component library setup, state management |
| Backend (Python) | expert-backend | FastAPI application structure, async database access, JWT implementation |
| DevOps | expert-devops | Docker Compose orchestration, multi-language CI/CD, deployment strategy |
