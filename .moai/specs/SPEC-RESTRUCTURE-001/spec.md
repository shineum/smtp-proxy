---
id: SPEC-RESTRUCTURE-001
title: Full Architecture Restructuring of smtp-proxy
version: 0.2.0
status: draft
created: 2026-02-25
updated: 2026-02-25
author: sungwon
priority: high
tags: architecture, restructuring, message-storage, fastapi, react, admin
related_specs:
  - SPEC-CORE-001
  - SPEC-GUARDRAIL-001
  - SPEC-INFRA-001
  - SPEC-MULTITENANT-001
  - SPEC-QUEUE-001
---

## HISTORY

| Version | Date       | Author  | Description                                      |
|---------|------------|---------|--------------------------------------------------|
| 0.1.0   | 2026-02-25 | sungwon | Initial draft - full architecture restructuring  |
| 0.2.0   | 2026-02-25 | sungwon | Add Module 7: Delivery Logging, Retry & Observability |

---

## 1. Environment

### 1.1 Current State

The smtp-proxy project is a multi-tenant SMTP relay proxy with ESP provider routing, written in Go 1.24. It currently operates as a monorepo under `server/` with 3 entry points:

- `cmd/smtp-server`: SMTP server accepting connections and enqueuing messages
- `cmd/queue-worker`: Background worker consuming queue and delivering via ESP providers
- `cmd/api-server`: Go-based REST API server (Chi router) for admin operations

**Current delivery flow:**
1. SMTP server authenticates, validates, persists message metadata + body to PostgreSQL
2. SMTP server enqueues full message payload to Redis Streams
3. Queue worker dequeues, resolves provider, delivers via ESP
4. Delivery has both sync (direct) and async (queue) modes

**Current storage:** PostgreSQL stores all message data including body in the `messages` table. Message body is passed inline through the queue.

### 1.2 Target State

Restructure into 4 clearly separated parts with a pluggable message body storage layer:

| Part | Technology | Purpose |
|------|-----------|---------|
| Part 1: SMTP Server | Go | Accept connections, auth, persist metadata, store body, enqueue ID |
| Part 2: Queue Worker | Go | Consume IDs, fetch body from storage, resolve ESP, deliver |
| Part 3: Admin Interface | FastAPI (Python) + React SPA | Account/tenant/provider management, delivery logs, analytics |
| Part 4: API Server | TBD | External API endpoints (placeholder) |

**Key architectural changes:**
- Message body stored in pluggable MessageStore (filesystem or S3), NOT in PostgreSQL
- PostgreSQL stores metadata only (message_id, account_id, sender, recipients, subject, headers, status, timestamps)
- Queue carries only message_id references, not full message payloads
- Sync delivery mode removed entirely
- Go API server replaced by FastAPI (Python) + React frontend
- SMTP auth stays in Go; admin auth moves to FastAPI with JWT

### 1.3 Infrastructure

- **Database:** PostgreSQL 15+ (shared across Go services and FastAPI)
- **Queue:** Redis 7.0+ with Streams (existing)
- **Message Storage:** Local filesystem (dev) or S3-compatible (production)
- **Container Runtime:** Docker with Docker Compose for local development
- **Go Version:** 1.24
- **Python Version:** 3.12+ for FastAPI
- **Node.js Version:** 20 LTS for React frontend

---

## 2. Assumptions

### 2.1 Technical Assumptions

- **A1:** The current PostgreSQL schema can be migrated to remove the body column from the messages table without data loss for in-flight messages (migration strategy required).
- **A2:** Local filesystem storage with configurable base path is sufficient for development and small-scale production; S3 is required for production at scale.
- **A3:** Redis Streams message payload size reduction (ID-only vs full body) will improve queue throughput and reduce Redis memory consumption.
- **A4:** FastAPI can connect to the same PostgreSQL database using the existing schema (via SQLAlchemy or raw asyncpg), with shared knowledge of the sqlc-generated schema.
- **A5:** JWT tokens issued by FastAPI admin backend and Go services can be independently validated if they share the same signing secret/key, OR authentication can be fully independent per service boundary.
- **A6:** The existing Go ESP provider implementations (SendGrid, SES, Mailgun, generic SMTP) are stable and require no changes beyond reading message body from storage instead of queue payload.
- **A7:** Docker Compose can orchestrate all 4 services plus PostgreSQL and Redis for local development.

### 2.2 Business Assumptions

- **B1:** Removing sync delivery mode is acceptable; all message delivery goes through the async queue path.
- **B2:** The admin interface is internal-facing (not customer-facing) and does not require public internet exposure.
- **B3:** The API Server (Part 4) scope will be defined in a future SPEC; this SPEC only reserves its placeholder in the architecture.
- **B4:** Existing SMTP clients require no changes; the SMTP protocol interface remains identical.

### 2.3 Constraints

- **C1:** Go services (SMTP server, queue worker) remain in the existing `server/` directory structure.
- **C2:** The admin interface (FastAPI + React) is added as new top-level directories (`admin/` for FastAPI, `frontend/` for React).
- **C3:** All services share the same PostgreSQL database; no separate databases per service.
- **C4:** Message body storage must be configurable via environment variable (STORAGE_TYPE=local or STORAGE_TYPE=s3).
- **C5:** The system must handle the case where a message body is not found in storage (storage read failure) gracefully with retry and DLQ routing.

---

## 3. Requirements

### Module 1: Message Storage Interface

**REQ-MSI-001 [Ubiquitous]**
The system shall provide a `MessageStore` interface with `Put(ctx, messageID, data) error` and `Get(ctx, messageID) ([]byte, error)` methods for storing and retrieving message bodies.

**REQ-MSI-002 [Ubiquitous]**
The system shall provide a `Delete(ctx, messageID) error` method on the `MessageStore` interface for message body cleanup after configurable retention.

**REQ-MSI-003 [Event-Driven]**
WHEN the configuration specifies `STORAGE_TYPE=local`, THEN the system shall instantiate a `LocalFileStore` implementation that stores message bodies as files under a configurable base directory path, using the message ID as the filename.

**REQ-MSI-004 [Event-Driven]**
WHEN the configuration specifies `STORAGE_TYPE=s3`, THEN the system shall instantiate an `S3Store` implementation that stores message bodies in an S3-compatible bucket, using the message ID as the object key with a configurable prefix.

**REQ-MSI-005 [State-Driven]**
IF `STORAGE_TYPE` is not set or is set to an unsupported value, THEN the system shall default to `local` storage and log a warning message.

**REQ-MSI-006 [Ubiquitous]**
The `LocalFileStore` shall create the base directory (including parent directories) on initialization if it does not exist.

**REQ-MSI-007 [Ubiquitous]**
The `S3Store` shall support configuration via environment variables: `S3_BUCKET`, `S3_PREFIX`, `S3_ENDPOINT` (for MinIO compatibility), `S3_REGION`, `AWS_ACCESS_KEY_ID`, and `AWS_SECRET_ACCESS_KEY`.

**REQ-MSI-008 [Unwanted]**
The system shall not store message body content in the PostgreSQL database. The `messages` table shall contain metadata only: message_id, account_id, tenant_id, sender, recipients, subject, headers (JSONB), status, created_at, updated_at.

### Module 2: SMTP Server Refactoring

**REQ-SMTP-001 [Ubiquitous]**
The SMTP server shall follow a simplified session flow: authenticate -> validate sender/recipients -> persist message metadata to PostgreSQL -> store message body in MessageStore -> enqueue message_id to Redis Streams -> return SMTP 250 OK response.

**REQ-SMTP-002 [Event-Driven]**
WHEN the SMTP DATA command is received with a valid message body, THEN the system shall:
1. Parse headers and extract subject
2. Insert metadata record into PostgreSQL (status: `queued`)
3. Store the full message body (headers + body) in MessageStore using the generated message_id
4. Enqueue only the message_id to Redis Streams
5. Return SMTP 250 response with the message_id

**REQ-SMTP-003 [Unwanted]**
The SMTP server shall not perform synchronous (inline) ESP delivery. The `SyncService` delivery implementation shall be removed entirely.

**REQ-SMTP-004 [Event-Driven]**
WHEN the MessageStore `Put` operation fails during SMTP DATA processing, THEN the system shall:
1. Delete the already-inserted metadata record from PostgreSQL (or mark as `failed`)
2. Return SMTP 451 temporary error response to the client
3. Log the storage failure with message_id and error details

**REQ-SMTP-005 [Event-Driven]**
WHEN the Redis enqueue operation fails after successful metadata persist and body storage, THEN the system shall:
1. Retry the enqueue operation up to 3 times with exponential backoff (500ms, 1s, 2s)
2. If all retries fail, update the message status to `enqueue_failed` in PostgreSQL
3. Return SMTP 451 temporary error response to the client
4. Log the enqueue failure with message_id, retry count, and error details

**REQ-SMTP-006 [Ubiquitous]**
The SMTP server shall inject the `MessageStore` dependency via constructor, configured at startup based on the `STORAGE_TYPE` environment variable.

### Module 3: Queue Worker Refactoring

**REQ-QW-001 [Event-Driven]**
WHEN the queue worker dequeues a message_id from Redis Streams, THEN it shall:
1. Fetch message metadata from PostgreSQL using the message_id
2. Fetch the full message body from MessageStore using the message_id
3. Resolve the ESP provider for the account
4. Deliver the message via the resolved provider
5. Update delivery status in PostgreSQL

**REQ-QW-002 [Event-Driven]**
WHEN the MessageStore `Get` operation fails for a dequeued message_id, THEN the system shall:
1. Retry the storage read up to 3 times with exponential backoff (1s, 2s, 4s)
2. If all retries fail, mark the message as `storage_error` in PostgreSQL
3. Route the message to the dead letter queue
4. Log the storage read failure with full context

**REQ-QW-003 [Ubiquitous]**
The queue worker shall preserve all existing retry logic, exponential backoff, and dead letter queue behavior for ESP delivery failures. Only the message body source changes (from queue payload to MessageStore).

**REQ-QW-004 [Ubiquitous]**
The queue worker shall inject the `MessageStore` dependency via constructor, using the same configuration mechanism as the SMTP server.

**REQ-QW-005 [Event-Driven]**
WHEN the queue worker receives a message_id that does not exist in PostgreSQL, THEN it shall log a warning and acknowledge the queue message without delivery (orphaned message_id cleanup).

### Module 4: Admin Interface (FastAPI + React)

#### 4A: FastAPI Backend

**REQ-ADMIN-001 [Ubiquitous]**
The admin backend shall be a FastAPI application (Python 3.12+) providing a REST API for account, tenant, user, provider, routing rule, and delivery log management.

**REQ-ADMIN-002 [Ubiquitous]**
The admin backend shall connect to the same PostgreSQL database used by Go services, using asyncpg or SQLAlchemy async for database access.

**REQ-ADMIN-003 [Ubiquitous]**
The admin backend shall implement JWT-based authentication with access and refresh tokens. Tokens shall use HS256 or RS256 signing with a configurable secret/key.

**REQ-ADMIN-004 [Ubiquitous]**
The admin backend shall enforce role-based access control (RBAC) with the following roles: `owner`, `admin`, `user`, `viewer`.

| Role    | Accounts | Tenants | Users | Providers | Routing Rules | Delivery Logs |
|---------|----------|---------|-------|-----------|---------------|---------------|
| owner   | CRUD     | CRUD    | CRUD  | CRUD      | CRUD          | Read          |
| admin   | Read     | CRUD    | CRUD  | CRUD      | CRUD          | Read          |
| user    | Read     | Read    | Read  | Read      | Read          | Read          |
| viewer  | --       | Read    | --    | --        | --            | Read          |

**REQ-ADMIN-005 [Event-Driven]**
WHEN an API request lacks a valid JWT token or the token has expired, THEN the system shall return HTTP 401 Unauthorized.

**REQ-ADMIN-006 [Event-Driven]**
WHEN a user attempts an operation not permitted by their role, THEN the system shall return HTTP 403 Forbidden.

**REQ-ADMIN-007 [Ubiquitous]**
The admin backend shall provide the following REST API endpoints:

| Resource       | Endpoints                                               |
|----------------|---------------------------------------------------------|
| Auth           | POST /auth/login, POST /auth/refresh, POST /auth/logout |
| Accounts       | GET /accounts, GET /accounts/{id}, POST /accounts, PUT /accounts/{id}, DELETE /accounts/{id} |
| Tenants        | GET /tenants, GET /tenants/{id}, POST /tenants, PUT /tenants/{id}, DELETE /tenants/{id} |
| Users          | GET /users, GET /users/{id}, POST /users, PUT /users/{id}, DELETE /users/{id} |
| Providers      | GET /providers, GET /providers/{id}, POST /providers, PUT /providers/{id}, DELETE /providers/{id} |
| Routing Rules  | GET /routing-rules, GET /routing-rules/{id}, POST /routing-rules, PUT /routing-rules/{id}, DELETE /routing-rules/{id} |
| Delivery Logs  | GET /delivery-logs, GET /delivery-logs/{id}, GET /delivery-logs/stats |
| Health         | GET /health                                              |

**REQ-ADMIN-008 [Ubiquitous]**
All list endpoints shall support pagination (offset/limit), sorting, and filtering via query parameters.

**REQ-ADMIN-009 [Unwanted]**
The admin backend shall not directly send emails or interact with ESP providers. It manages configuration only.

#### 4B: React Frontend

**REQ-ADMIN-010 [Ubiquitous]**
The admin frontend shall be a React SPA providing a dashboard and CRUD screens for all admin backend resources.

**REQ-ADMIN-011 [Ubiquitous]**
The admin frontend shall include the following pages:

| Page              | Purpose                                           |
|-------------------|---------------------------------------------------|
| Login             | JWT authentication with email/password             |
| Dashboard         | Overview metrics: message volume, delivery rate, error rate, queue depth |
| Accounts          | Account list with create/edit/delete               |
| Tenants           | Tenant list with create/edit/delete                |
| Users             | User management with role assignment               |
| Providers         | ESP provider configuration with create/edit/delete |
| Routing Rules     | Routing rule management with priority ordering     |
| Delivery Logs     | Searchable, filterable delivery log viewer         |
| Analytics         | Charts: delivery trends, provider performance, error breakdown |

**REQ-ADMIN-012 [Event-Driven]**
WHEN a user's JWT token expires during an active session, THEN the frontend shall attempt to refresh the token automatically. IF the refresh fails, THEN redirect to the login page.

**REQ-ADMIN-013 [Ubiquitous]**
The admin frontend shall display role-appropriate navigation and disable/hide actions not permitted by the user's role.

### Module 5: Service Boundaries and Deployment

**REQ-DEPLOY-001 [Ubiquitous]**
The Docker Compose configuration shall define the following services for local development:

| Service        | Image/Build         | Ports         | Dependencies           |
|----------------|--------------------:|---------------|------------------------|
| postgres       | postgres:15-alpine  | 5432          | --                     |
| redis          | redis:7-alpine      | 6379          | --                     |
| smtp-server    | server/Dockerfile   | 2525, 4650    | postgres, redis        |
| queue-worker   | server/Dockerfile   | --            | postgres, redis        |
| admin-api      | admin/Dockerfile    | 8000          | postgres               |
| admin-frontend | frontend/Dockerfile | 3000          | admin-api              |

**REQ-DEPLOY-002 [Ubiquitous]**
Each service shall be independently configurable via environment variables. Shared configuration (database URL, Redis URL) shall be defined in a common `.env` file.

**REQ-DEPLOY-003 [Ubiquitous]**
The Go services (SMTP server, queue worker) shall remain in the `server/` directory. The FastAPI admin shall reside in `admin/`. The React frontend shall reside in `frontend/`.

**REQ-DEPLOY-004 [Ubiquitous]**
The new `MessageStore` package shall be located at `server/internal/msgstore/` to distinguish it from the existing `server/internal/storage/` package (which handles PostgreSQL via sqlc).

**REQ-DEPLOY-005 [State-Driven]**
IF the deployment is local development, THEN the MessageStore shall default to `LocalFileStore` with a Docker volume-mapped directory. IF the deployment is production, THEN the MessageStore shall use `S3Store`.

**REQ-DEPLOY-006 [Ubiquitous]**
The PostgreSQL database schema shall be shared across Go services and FastAPI. Schema migrations shall be managed by golang-migrate in the `server/` directory and applied before any service starts.

### Module 6: API Server Placeholder

**REQ-API-001 [Optional]**
Where the external API server is needed, the system shall provide a placeholder entry point at `server/cmd/api-server/main.go` that starts an HTTP server returning health check responses only.

**REQ-API-002 [Optional]**
Where the external API is required, its scope, authentication model, and endpoints shall be defined in a future SPEC (SPEC-API-001).

### Module 7: Delivery Logging, Retry & Observability

#### 7A: Queue Enqueue Retry

**REQ-LOG-001 [Event-Driven]**
WHEN the SMTP server's Redis enqueue operation fails, THEN the system shall retry the enqueue up to 3 times with exponential backoff (500ms, 1s, 2s) before marking the message as `enqueue_failed`.

**REQ-LOG-002 [Ubiquitous]**
Each enqueue retry attempt shall be logged with: message_id, attempt number, elapsed time, and error reason.

#### 7B: Delivery Result Logging

**REQ-LOG-003 [Event-Driven]**
WHEN the queue worker successfully delivers a message via ESP provider, THEN the system shall:
1. Record a delivery log entry in PostgreSQL `delivery_logs` table with status `delivered`, provider name, response details, and duration
2. Write a structured log entry (JSON) to the application log with level INFO

**REQ-LOG-004 [Event-Driven]**
WHEN the queue worker fails to deliver a message via ESP provider, THEN the system shall:
1. Record a delivery log entry in PostgreSQL `delivery_logs` table with status `failed`, provider name, error message, retry count, and duration
2. Write a structured log entry (JSON) to the application log with level ERROR
3. Include full error context: message_id, account_id, provider, attempt number, error type, error message

**REQ-LOG-005 [Event-Driven]**
WHEN a message is routed to the dead letter queue after all retries are exhausted, THEN the system shall:
1. Record a final delivery log entry in PostgreSQL with status `dlq`
2. Write a structured log entry with level WARN including total retry count and all error reasons

#### 7C: Statistics in Database

**REQ-LOG-006 [Ubiquitous]**
The PostgreSQL `delivery_logs` table shall store all delivery attempt records with the following fields: id, message_id, account_id, tenant_id, provider_name, status (delivered/failed/dlq/retrying), error_message, attempt_number, duration_ms, created_at.

**REQ-LOG-007 [Ubiquitous]**
The system shall persist aggregate delivery statistics queryable from the database, including: total sent, total delivered, total failed, total DLQ, delivery rate per provider, delivery rate per account, and average delivery duration.

**REQ-LOG-008 [Optional]**
WHERE the admin interface is available, delivery statistics shall be queryable via the FastAPI GET /delivery-logs/stats endpoint with time range and grouping parameters (by provider, by account, by hour/day).

#### 7D: Application Log Output (File / CloudWatch)

**REQ-LOG-009 [Ubiquitous]**
The application log output shall be configurable via environment variable `LOG_OUTPUT` with the following options:

| Value | Behavior |
|-------|----------|
| `stdout` | Write structured JSON logs to stdout (default, for Docker/k8s) |
| `file` | Write structured JSON logs to a configurable file path (`LOG_FILE_PATH`) |
| `cloudwatch` | Send structured logs to AWS CloudWatch Logs (`LOG_CW_GROUP`, `LOG_CW_STREAM`) |

**REQ-LOG-010 [State-Driven]**
IF `LOG_OUTPUT=file`, THEN the system shall write logs to the path specified by `LOG_FILE_PATH` with automatic log rotation (max size configurable via `LOG_MAX_SIZE_MB`, default 100MB, max files configurable via `LOG_MAX_FILES`, default 10).

**REQ-LOG-011 [State-Driven]**
IF `LOG_OUTPUT=cloudwatch`, THEN the system shall send logs to the AWS CloudWatch Logs group specified by `LOG_CW_GROUP` and stream specified by `LOG_CW_STREAM`, using AWS credentials from the environment (standard AWS SDK credential chain).

**REQ-LOG-012 [State-Driven]**
IF `LOG_OUTPUT` is not set, THEN the system shall default to `stdout` output.

**REQ-LOG-013 [Unwanted]**
The system shall not store full application logs (debug, info, warn, error) in PostgreSQL. Only delivery statistics and delivery attempt records shall be persisted to the database.

---

## 4. Specifications

### 4.1 MessageStore Interface (Go)

```go
package msgstore

import "context"

// MessageStore provides pluggable storage for email message bodies.
// Implementations: LocalFileStore (filesystem), S3Store (S3-compatible).
type MessageStore interface {
    // Put stores the message body data keyed by messageID.
    Put(ctx context.Context, messageID string, data []byte) error

    // Get retrieves the message body data by messageID.
    // Returns ErrNotFound if the message does not exist.
    Get(ctx context.Context, messageID string) ([]byte, error)

    // Delete removes the message body data by messageID.
    // Returns nil if the message does not exist (idempotent).
    Delete(ctx context.Context, messageID string) error
}

// ErrNotFound is returned when a message body is not found in storage.
var ErrNotFound = errors.New("message not found in storage")
```

### 4.2 Directory Structure (Target)

```
smtp-proxy/
├── server/                          # Go services (existing, refactored)
│   ├── cmd/
│   │   ├── smtp-server/main.go      # Part 1: SMTP Server
│   │   ├── queue-worker/main.go     # Part 2: Queue Worker
│   │   ├── api-server/main.go       # Part 4: API Server (placeholder)
│   │   └── test-client/main.go      # Test utility
│   ├── internal/
│   │   ├── msgstore/                # NEW: Message body storage
│   │   │   ├── store.go             # MessageStore interface
│   │   │   ├── local.go             # LocalFileStore implementation
│   │   │   ├── s3.go                # S3Store implementation
│   │   │   ├── local_test.go
│   │   │   └── s3_test.go
│   │   ├── smtp/                    # Refactored: uses MessageStore
│   │   ├── worker/                  # Refactored: fetches from MessageStore
│   │   ├── delivery/                # Refactored: remove SyncService
│   │   ├── config/                  # Extended: STORAGE_TYPE config
│   │   ├── provider/                # Unchanged
│   │   ├── queue/                   # Unchanged (payload now ID-only)
│   │   ├── routing/                 # Unchanged
│   │   ├── storage/                 # PostgreSQL via sqlc (metadata only)
│   │   ├── auth/                    # SMTP auth stays (admin auth removed)
│   │   ├── logger/                  # Unchanged
│   │   ├── metrics/                 # Unchanged
│   │   └── tlsutil/                 # Unchanged
│   ├── go.mod
│   └── Dockerfile
├── admin/                           # NEW: FastAPI admin backend
│   ├── app/
│   │   ├── __init__.py
│   │   ├── main.py                  # FastAPI app entry point
│   │   ├── config.py                # Settings via pydantic-settings
│   │   ├── database.py              # AsyncPG / SQLAlchemy async setup
│   │   ├── auth/
│   │   │   ├── __init__.py
│   │   │   ├── jwt.py               # JWT token generation/validation
│   │   │   ├── dependencies.py      # FastAPI dependencies for auth
│   │   │   └── rbac.py              # Role-based access control
│   │   ├── models/                  # SQLAlchemy models (mirror sqlc schema)
│   │   ├── schemas/                 # Pydantic request/response schemas
│   │   ├── routers/                 # API route handlers
│   │   │   ├── auth.py
│   │   │   ├── accounts.py
│   │   │   ├── tenants.py
│   │   │   ├── users.py
│   │   │   ├── providers.py
│   │   │   ├── routing_rules.py
│   │   │   └── delivery_logs.py
│   │   └── services/                # Business logic layer
│   ├── tests/
│   ├── pyproject.toml
│   ├── Dockerfile
│   └── alembic/                     # DB migrations (read-only, managed by Go)
├── frontend/                        # NEW: React admin SPA
│   ├── src/
│   │   ├── App.tsx
│   │   ├── main.tsx
│   │   ├── pages/
│   │   ├── components/
│   │   ├── hooks/
│   │   ├── lib/
│   │   ├── types/
│   │   └── styles/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── Dockerfile
├── docker-compose.yml               # All services for local dev
├── .env.example                     # Shared environment template
└── README.md
```

### 4.3 Database Schema Changes

**Migration: Remove body from messages table**

```sql
-- Up migration
ALTER TABLE messages DROP COLUMN IF EXISTS body;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS storage_ref TEXT;

-- Down migration (lossy - body data cannot be recovered from storage)
ALTER TABLE messages ADD COLUMN IF EXISTS body BYTEA;
ALTER TABLE messages DROP COLUMN IF EXISTS storage_ref;
```

The `storage_ref` column stores the storage key/path for the message body, typically equal to the message_id UUID string. This provides an indirection layer if the storage key format changes.

### 4.4 Queue Message Format Change

**Current format (full payload):**
```json
{
  "id": "message-uuid",
  "tenant_id": "tenant-1",
  "from": "sender@example.com",
  "to": ["recipient@example.com"],
  "subject": "Hello",
  "headers": {"X-Custom": "value"},
  "body": "<base64 encoded full message>"
}
```

**New format (ID reference only):**
```json
{
  "id": "message-uuid",
  "account_id": "account-uuid",
  "tenant_id": "tenant-1"
}
```

The worker fetches all other data from PostgreSQL (metadata) and MessageStore (body).

### 4.5 Configuration Environment Variables

| Variable        | Service         | Description                                  | Default    |
|-----------------|-----------------|----------------------------------------------|------------|
| STORAGE_TYPE    | smtp, worker    | Message storage backend: `local` or `s3`     | local      |
| STORAGE_PATH    | smtp, worker    | Base directory for local file storage         | /data/messages |
| S3_BUCKET       | smtp, worker    | S3 bucket name                               | --         |
| S3_PREFIX       | smtp, worker    | S3 object key prefix                         | messages/  |
| S3_ENDPOINT     | smtp, worker    | S3 endpoint URL (for MinIO)                  | --         |
| S3_REGION       | smtp, worker    | AWS region                                   | us-east-1  |
| DATABASE_URL    | all             | PostgreSQL connection string                 | --         |
| REDIS_URL       | smtp, worker    | Redis connection string                      | --         |
| ADMIN_SECRET_KEY| admin-api       | JWT signing secret                           | --         |
| ADMIN_DB_URL    | admin-api       | PostgreSQL URL (same DB, may use different pool) | --     |
| LOG_OUTPUT      | smtp, worker    | Log output target: `stdout`, `file`, `cloudwatch` | stdout |
| LOG_FILE_PATH   | smtp, worker    | File path for log output (when LOG_OUTPUT=file)  | /var/log/smtp-proxy.log |
| LOG_MAX_SIZE_MB | smtp, worker    | Max log file size before rotation                | 100    |
| LOG_MAX_FILES   | smtp, worker    | Max number of rotated log files to retain        | 10     |
| LOG_CW_GROUP    | smtp, worker    | CloudWatch Logs group name                       | --     |
| LOG_CW_STREAM   | smtp, worker    | CloudWatch Logs stream name                      | --     |

### 4.6 Traceability Matrix

| Requirement     | Module                | Implementation File(s)                        |
|-----------------|-----------------------|-----------------------------------------------|
| REQ-MSI-001..008| Message Storage       | `server/internal/msgstore/`                   |
| REQ-SMTP-001..006| SMTP Server          | `server/internal/smtp/session.go`, `server/cmd/smtp-server/main.go` |
| REQ-QW-001..005 | Queue Worker          | `server/internal/worker/handler.go`, `server/cmd/queue-worker/main.go` |
| REQ-ADMIN-001..013| Admin Interface     | `admin/`, `frontend/`                         |
| REQ-DEPLOY-001..006| Deployment          | `docker-compose.yml`, Dockerfiles             |
| REQ-API-001..002| API Placeholder       | `server/cmd/api-server/main.go`               |
| REQ-LOG-001..013| Logging & Observability| `server/internal/logger/`, `server/internal/worker/handler.go`, `server/internal/smtp/session.go` |
