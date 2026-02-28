# smtp-proxy

Multi-tenant SMTP proxy server that accepts email via SMTP and delivers asynchronously through configurable ESP providers (SendGrid, SES, Mailgun, Microsoft Graph). Features pluggable message body storage, Redis Streams queue with retry and dead-letter support, unified JWT/API-key authentication with group-based access control, and a REST API for management.

## Quick Start

```bash
# Start all services (zero prerequisites except Docker)
docker compose up -d --build

# First time: create dev group + SMTP account
docker compose run --rm seed

# Send a test email
docker compose run --rm test-client

# View logs
docker compose logs smtp-server

# Stop
docker compose down
```

The API server auto-seeds a system admin on startup (`admin@localhost` / `admin`).
Run `docker compose run --rm seed` once to create a dev company group with an SMTP account (`dev`) for testing.

## Architecture

### System Overview

```
                      ┌──────────────────────────────────────────────────┐
                      │                  smtp-proxy                      │
                      │                                                  │
  SMTP :587/465 ────▶ │  ┌─────────────┐     ┌─────────────────────┐    │
                      │  │ smtp-server  │────▶│  Message Storage    │    │
                      │  │  (go-smtp)   │     │  (local / S3)       │    │
                      │  └──────┬───────┘     └─────────────────────┘    │
                      │         │ enqueue ID                   ▲ fetch   │
                      │         ▼                              │         │
                      │  ┌─────────────┐     ┌────────────────┴────┐    │
                      │  │    Redis     │────▶│   queue-worker      │───▶│──▶ ESP
                      │  │   Streams    │     │ (10 concurrent)     │    │   (SendGrid,
                      │  └─────────────┘     └─────────────────────┘    │    SES, ...)
                      │                                                  │
                      │  ┌─────────────┐     ┌─────────────────────┐    │
  REST :8080 ────────▶│  │ api-server  │────▶│    PostgreSQL 18    │    │
                      │  │   (chi)     │     │  (RLS, multi-tenant) │    │
                      │  └─────────────┘     └─────────────────────┘    │
                      └──────────────────────────────────────────────────┘
```

### Data Flow

**SMTP Ingestion** (smtp-server):

```
Client → SMTP AUTH (SASL PLAIN) → domain validation → read message
       → store body in MessageStore (local file / S3)
       → persist metadata in PostgreSQL (sender, recipients, headers, storage_ref)
       → enqueue ID-only reference to Redis Streams
       → retry enqueue up to 3x (500ms, 1s, 2s backoff)
       → return SMTP 250 OK
```

**Async Delivery** (queue-worker):

```
Redis XREADGROUP → fetch message metadata from PostgreSQL
                 → fetch body from MessageStore (3x retry: 1s, 2s, 4s)
                 → resolve ESP provider for account (5-min cache)
                 → deliver via provider
                 → record delivery log (duration, status, attempt)
                 → on failure: retry up to 5x (30s, 1m, 2m, 5m, 15m + jitter)
                 → on exhaustion: move to dead-letter queue
```

**Message Status Lifecycle:**

```
queued → processing → delivered
                    → failed (ESP error)
                    → enqueue_failed (Redis unreachable)
                    → storage_error (body not found)
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| ID-only queue messages | Keeps Redis payload small; body stored externally |
| Pluggable MessageStore | Swap local filesystem for S3 without code changes |
| Per-group provider resolution | Each group configures their own ESP independently |
| Enqueue retry with backoff | Tolerates transient Redis failures without losing mail |
| Row-Level Security | PostgreSQL RLS enforces group-level isolation at the database layer |
| Unified auth (JWT + API key) | Single middleware accepts both human (JWT) and SMTP (API key) users |
| Optional TLS with auto-generation | `tls.mode=none` for NLB termination; self-signed auto-generation for dev |

## Services

| Service | Port | Description |
|---------|------|-------------|
| `smtp-server` | 587, 465 | SMTP listener with STARTTLS and implicit TLS |
| `api-server` | 8080 | REST API for groups, users, providers, routing, auth |
| `queue-worker` | - | Async delivery worker (Redis Streams consumer) |
| `postgres` | - | PostgreSQL 18 with Row-Level Security |
| `redis` | - | Redis 7.4 (queue + rate limiting) |
| `migrate` | - | Database migrations (runs once on startup) |
| `seed` | - | Creates dev group + SMTP account (seed-init-dev-accounts profile, run manually) |
| `test-client` | - | CLI tool for sending test emails |

## Project Structure

```
server/
├── cmd/
│   ├── smtp-server/       # SMTP ingestion service
│   ├── api-server/        # REST API service
│   ├── queue-worker/      # Async delivery worker
│   └── test-client/       # CLI email sender
├── internal/
│   ├── api/               # HTTP handlers, middleware, router (chi)
│   ├── auth/              # JWT, API key, unified auth, RBAC, rate limiting, audit
│   ├── bootstrap/         # System admin auto-seed on startup
│   ├── config/            # Viper config loading with env override
│   ├── delivery/          # Delivery service interface + async implementation
│   ├── logger/            # zerolog wrapper (stdout / file / cloudwatch)
│   ├── metrics/           # Prometheus metrics (SMTP, API, DB, queue)
│   ├── msgstore/          # Message body storage (local filesystem, S3)
│   ├── provider/          # ESP provider interface + implementations
│   ├── queue/             # Redis Streams producer, consumer, DLQ, retry
│   ├── routing/           # Routing engine (primary + fallback providers)
│   ├── smtp/              # SMTP backend + session (go-smtp)
│   ├── storage/           # sqlc-generated PostgreSQL queries
│   ├── tlsutil/           # Self-signed TLS certificate generator
│   └── worker/            # Queue message handler (delivery orchestration)
├── migrations/            # 10 up/down SQL migration pairs
└── config/config.yaml     # Default application config
```

## Configuration

All settings can be overridden via environment variables prefixed with `SMTP_PROXY_`.

Copy `.env.example` to `.env` for local customization:

```bash
cp .env.example .env
```

### Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SMTP_PROXY_DATABASE_URL` | `postgres://...@postgres:5432/smtp_proxy` | PostgreSQL connection string |
| `SMTP_PROXY_QUEUE_REDIS_ADDR` | `redis:6379` | Redis address |
| `SMTP_PROXY_AUTH_SIGNING_KEY` | `change-me-in-production...` | JWT HMAC signing key |
| `SMTP_PROXY_STORAGE_TYPE` | `local` | Message body storage: `local` or `s3` |
| `SMTP_PROXY_STORAGE_PATH` | `/data/messages` | Local storage directory |
| `SMTP_PROXY_STORAGE_S3_BUCKET` | *(empty)* | S3 bucket name (when type=s3) |
| `SMTP_PROXY_STORAGE_S3_ENDPOINT` | *(empty)* | S3 endpoint (MinIO-compatible) |
| `SMTP_PROXY_TLS_CERT_FILE` | *(auto-generate)* | Path to TLS certificate |
| `SMTP_PROXY_TLS_KEY_FILE` | *(auto-generate)* | Path to TLS private key |
| `SMTP_PROXY_TLS_MODE` | `starttls` | TLS mode: `starttls` or `none` (for NLB/proxy) |
| `SMTP_PROXY_ADMIN_EMAIL` | `admin@localhost` | System admin email (auto-seeded on startup) |
| `SMTP_PROXY_ADMIN_PASSWORD` | `admin` | System admin password (auto-seeded on startup) |
| `SMTP_PROXY_LOGGING_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SMTP_PROXY_LOGGING_OUTPUT` | `stdout` | `stdout`, `file`, `cloudwatch` |

### Application Config

Full configuration in `server/config/config.yaml`:

```yaml
smtp:
  host: 0.0.0.0
  port: 587
  max_connections: 1000
  max_message_size: 26214400  # 25MB

queue:
  redis_addr: "localhost:6379"
  stream_name: "smtp-proxy"
  workers: 10
  block_timeout: "5s"

storage:
  type: "local"               # local | s3
  path: "/data/messages"
  s3_bucket: ""
  s3_endpoint: ""             # MinIO-compatible endpoint
  s3_region: "us-east-1"

tls:
  mode: "starttls"              # starttls | none (for NLB/proxy)
  cert_file: ""
  key_file: ""

delivery:
  mode: "sync"                  # sync | async (requires Redis)

auth:
  signing_key: "..."
  access_token_expiry: 15m
  refresh_token_expiry: 168h  # 7 days

rate_limit:
  default_monthly_limit: 10000
  login_attempts_limit: 5
  login_lockout_duration: 15m
```

## API Endpoints

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness check |
| GET | `/readyz` | Readiness check (includes DB) |

### Authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/login` | None | Login (returns access + refresh tokens) |
| POST | `/api/v1/auth/refresh` | None | Refresh access token |
| POST | `/api/v1/auth/logout` | None | Invalidate refresh token |
| POST | `/api/v1/auth/switch-group` | JWT | Switch active group context |

### Groups (Unified Auth)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/groups` | System admin | Create group |
| GET | `/api/v1/groups` | System admin | List all groups |
| GET | `/api/v1/groups/{id}` | Member | Get group details |
| DELETE | `/api/v1/groups/{id}` | System admin | Delete group |
| GET | `/api/v1/groups/{id}/members` | Member | List group members |
| POST | `/api/v1/groups/{id}/members` | Member | Add member to group |
| PATCH | `/api/v1/groups/{id}/members/{uid}` | Member | Update member role |
| DELETE | `/api/v1/groups/{id}/members/{uid}` | Member | Remove member |
| GET | `/api/v1/groups/{id}/activity` | Member | List activity logs |

Group types: `system` (platform admin), `company` (tenant organization)

### Users (Unified Auth)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/users` | Authenticated | List users |
| POST | `/api/v1/users` | Authenticated | Create user |
| GET | `/api/v1/users/{id}` | Authenticated | Get user |
| PATCH | `/api/v1/users/{id}/status` | Authenticated | Update user status |
| DELETE | `/api/v1/users/{id}` | Authenticated | Delete user |

Account types: `human` (JWT login), `smtp` (API key auth for SMTP)

### ESP Providers (Unified Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/providers` | Create provider |
| GET | `/api/v1/providers` | List providers |
| GET | `/api/v1/providers/{id}` | Get provider |
| PUT | `/api/v1/providers/{id}` | Update provider |
| DELETE | `/api/v1/providers/{id}` | Delete provider |

Supported provider types: `sendgrid`, `ses`, `mailgun`, `smtp`, `msgraph`

### Routing Rules (Unified Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/routing-rules` | Create routing rule |
| GET | `/api/v1/routing-rules` | List routing rules |
| GET | `/api/v1/routing-rules/{id}` | Get routing rule |
| PUT | `/api/v1/routing-rules/{id}` | Update routing rule |
| DELETE | `/api/v1/routing-rules/{id}` | Delete routing rule |

### Webhooks (No Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/webhooks/sendgrid` | SendGrid delivery events |
| POST | `/api/v1/webhooks/ses` | AWS SES delivery events |
| POST | `/api/v1/webhooks/mailgun` | Mailgun delivery events |

### Dead-Letter Queue (API Key Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/dlq/reprocess` | Reprocess failed messages from DLQ |

## Provider Resolution

When a message is dequeued for delivery, the worker resolves the ESP provider:

1. Check in-memory cache (5-minute TTL per group)
2. Query the group's providers from PostgreSQL (ordered by creation date)
3. Select the first enabled provider
4. If no provider configured, fall back to `stdout` (prints to server logs)

```bash
# Configure a SendGrid provider for a group
curl -X POST http://localhost:8080/api/v1/providers \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-sendgrid",
    "provider_type": "sendgrid",
    "api_key": "SG.xxx",
    "enabled": true
  }'
```

The group is automatically resolved from the authenticated user's context.

## Message Storage

Message bodies are stored externally (not in the database) for scalability.

| Backend | Config | Description |
|---------|--------|-------------|
| `local` | `type: local`, `path: /data/messages` | Local filesystem with atomic writes |
| `s3` | `type: s3`, `s3_bucket: ...` | AWS S3 or MinIO-compatible storage |

The SMTP server stores the body via `MessageStore.Put()` and persists only metadata + a `storage_ref` in PostgreSQL. The worker fetches the body via `MessageStore.Get()` at delivery time.

If the MessageStore write fails during SMTP ingestion, the system falls back to inline body storage in PostgreSQL for reliability.

## Retry and Error Handling

| Stage | Retries | Backoff Schedule | On Exhaustion |
|-------|---------|------------------|---------------|
| SMTP enqueue (session to Redis) | 3 | 500ms, 1s, 2s | Status: `enqueue_failed`, SMTP 451 |
| Worker storage read | 3 | 1s, 2s, 4s | Status: `storage_error`, delivery log |
| Worker ESP delivery | 5 | 30s, 1m, 2m, 5m, 15m (+jitter) | Move to DLQ |

Failed messages in the dead-letter queue can be reprocessed via `POST /api/v1/dlq/reprocess`.

## Database

PostgreSQL 18 with 10 migrations applied automatically on startup.

**Tables:** `groups`, `group_members`, `users`, `esp_providers`, `routing_rules`, `messages`, `delivery_logs`, `sessions`, `activity_logs`

**Multi-tenant isolation:** Row-Level Security (RLS) policies enforce group-level boundaries using the `app.current_group_id` PostgreSQL session variable, set automatically by API middleware.

Data is persisted in a Docker volume (`postgres-data`). To reset:

```bash
docker compose down -v   # removes volumes
docker compose up -d --build
```

## Observability

### Logging

Structured JSON logging via zerolog with per-session correlation IDs.

| Output | Description |
|--------|-------------|
| `stdout` | Default, writes to standard output |
| `file` | Rotating log files via lumberjack |
| `cloudwatch` | CloudWatch Logs integration (placeholder) |

### Metrics

Prometheus metrics exposed by the API server:

| Namespace | Examples |
|-----------|---------|
| SMTP | `smtp_connections_total`, `smtp_active_sessions`, `smtp_message_enqueued_total` |
| API | `api_requests_total`, `api_request_duration_seconds` |
| Database | `db_connections_active`, `db_query_duration_seconds` |
| Queue | `queue_depth` |

## TLS Modes

The SMTP server supports two TLS modes, configured via `tls.mode` (or `SMTP_PROXY_TLS_MODE`):

| Mode | Behavior |
|------|----------|
| `starttls` (default) | STARTTLS enabled. Loads certs from files, or auto-generates self-signed certs if none provided. |
| `none` | TLS disabled entirely. Use when TLS is terminated by an upstream NLB or reverse proxy. |

### Running behind an NLB (TLS disabled)

```bash
# Option 1: Environment variable
SMTP_PROXY_TLS_MODE=none docker compose up -d --build

# Option 2: Uncomment in docker-compose.yml (smtp-server service)
#   SMTP_PROXY_TLS_MODE: "none"

# Test against the non-TLS server
docker compose run --rm test-client --tls=none
```

When `mode=none`, the server skips all certificate loading and allows plaintext authentication. A warning is logged on startup to confirm TLS is disabled.

## Test Client

```bash
# Default: sends one test email via STARTTLS on port 587
docker compose run --rm test-client

# Plain text (no TLS) - for NLB-terminated deployments
docker compose run --rm test-client --tls=none

# Custom options
docker compose run --rm test-client \
  --from sender@example.com \
  --to recipient@example.com \
  --subject "Hello" \
  --body "Test message" \
  --count 10 \
  --rate 5

# HTML email with attachment
docker compose run --rm test-client \
  --from sender@example.com \
  --to recipient@example.com \
  --html "<h1>Hello</h1>" \
  --attach /test-data/sample.txt
```

| Flag | Default | Description |
|------|---------|-------------|
| `--host` | `localhost` | SMTP server hostname |
| `--port` | `587` | SMTP port |
| `--tls` | `starttls` | `starttls`, `implicit`, or `none` |
| `--insecure` | `false` | Skip TLS certificate verification |
| `--user` | *(empty)* | SMTP AUTH username |
| `--password` | *(empty)* | SMTP AUTH password |
| `--from` | *(required)* | Sender email address |
| `--to` | *(required)* | Recipient address (repeatable) |
| `--subject` | `Test Email` | Email subject |
| `--body` | `This is a test...` | Plain text body |
| `--html` | *(empty)* | HTML body (sends multipart/alternative) |
| `--attach` | *(empty)* | File attachment path (repeatable) |
| `--count` | `1` | Number of emails to send |
| `--rate` | `1` | Emails per second |

## Development

```bash
# Run tests (Go is only available inside Docker)
docker run --rm -w /app \
  -v $(pwd)/server:/app \
  golang:1.24-alpine \
  sh -c "go test ./... -count=1"

# Build only (verify compilation)
docker build --target builder -f server/Dockerfile server/

# Rebuild a single service
docker compose up -d --build smtp-server
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.24 |
| SMTP Server | go-smtp + go-sasl |
| HTTP Router | chi v5 |
| Database | PostgreSQL 18 (pgx v5, sqlc) |
| Queue | Redis 7.4 Streams |
| Auth | JWT (HS256) + bcrypt + API keys (unified auth) |
| Metrics | Prometheus client_golang |
| Logging | zerolog |
| Config | Viper |
| Object Storage | AWS SDK v2 (S3-compatible) |
| Container | Docker multi-stage builds |
