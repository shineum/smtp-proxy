# smtp-proxy

Multi-tenant SMTP proxy server that accepts email via SMTP and delivers asynchronously through configurable ESP providers (SendGrid, SES, Mailgun, Microsoft Graph). Features pluggable message body storage, pluggable queue backend (Redis Streams or AWS SQS) with retry and dead-letter support, unified JWT/API-key authentication with group-based access control, and a REST API for management.

## Quick Start

```bash
# Start all services (zero prerequisites except Docker)
docker compose up -d --build

# (Dev only) First time: create dev group + SMTP account (dev/dev)
docker compose run --rm seed

# Send a test email
docker compose run --rm test-client

# View logs
docker compose logs smtp-server

# Stop
docker compose down
```

The API server auto-seeds a system admin on startup (`admin@localhost` / `admin`).
Run `docker compose run --rm seed` once to create a dev company group with an SMTP account (`dev` / `dev`) for local testing only.

## Architecture

### System Overview

```
                      ┌──────────────────────────────────────────────────┐
                      │                  smtp-proxy                      │
                      │                                                  │
  SMTP :2587/2465 ──▶ │  ┌─────────────┐     ┌─────────────────────┐    │
                      │  │ smtp-server  │────▶│  Message Storage    │    │
                      │  │  (go-smtp)   │     │  (local / S3)       │    │
                      │  └──────┬───────┘     └─────────────────────┘    │
                      │         │ enqueue ID                   ▲ fetch   │
                      │         ▼                              │         │
                      │  ┌─────────────┐     ┌────────────────┴────┐    │
                      │  │ Redis/SQS   │────▶│   queue-worker      │───▶│──▶ ESP
                      │  │   Queue     │     │ (10 concurrent)     │    │   (SendGrid,
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
       → enqueue ID-only reference to queue (Redis Streams or SQS)
       → retry enqueue up to 3x (500ms, 1s, 2s backoff)
       → return SMTP 250 OK
```

**Async Delivery** (queue-worker):

```
Dequeue message → fetch message metadata from PostgreSQL
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
                    → enqueue_failed (queue unreachable)
                    → storage_error (body not found)
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| ID-only queue messages | Keeps queue payload small; body stored externally |
| Pluggable MessageStore | Swap local filesystem for S3 without code changes |
| Per-group provider resolution | Each group configures their own ESP independently |
| Enqueue retry with backoff | Tolerates transient queue failures without losing mail |
| Row-Level Security | PostgreSQL RLS enforces group-level isolation at the database layer |
| Unified auth (JWT + API key) | Single middleware accepts both human (JWT) and SMTP (API key) users |
| Optional TLS with auto-generation | `tls.mode=none` for NLB termination; self-signed auto-generation for dev |

## Services

| Service | Port | Description |
|---------|------|-------------|
| `smtp-server` | 2587, 2465 | SMTP listener with STARTTLS and implicit TLS (NLB maps 587→2587) |
| `api-server` | 8080 | REST API for groups, users, providers, routing, auth |
| `queue-worker` | - | Async delivery worker (Redis Streams or SQS consumer) |
| `postgres` | - | PostgreSQL 18 with Row-Level Security |
| `redis` | - | Redis 7.4 (queue backend, optional if using SQS) |
| `migrate` | - | Database migrations (standalone, optional — api-server runs migrations on startup) |
| `seed` | - | **(Dev only)** Creates dev group + SMTP account (seed-init-dev-accounts profile, run manually) |
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
│   ├── queue/             # Queue backend (Redis Streams / SQS), DLQ, retry
│   ├── routing/           # Routing engine (primary + fallback providers)
│   ├── smtp/              # SMTP backend + session (go-smtp)
│   ├── storage/           # sqlc-generated PostgreSQL queries
│   ├── tlsutil/           # Self-signed TLS certificate generator
│   └── worker/            # Queue message handler (delivery orchestration)
├── migrations/            # 25 up/down SQL migration pairs
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
| `SMTP_PROXY_DATABASE_HOST` | `postgres` | PostgreSQL host |
| `SMTP_PROXY_DATABASE_PORT` | `5432` | PostgreSQL port |
| `SMTP_PROXY_DATABASE_USER` | `smtp_proxy` | PostgreSQL user |
| `SMTP_PROXY_DATABASE_PASSWORD` | `smtp_proxy_dev` | PostgreSQL password |
| `SMTP_PROXY_DATABASE_NAME` | `smtp_proxy` | PostgreSQL database name |
| `SMTP_PROXY_DATABASE_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `SMTP_PROXY_QUEUE_TYPE` | `redis` | Queue backend: `redis` or `sqs` |
| `SMTP_PROXY_QUEUE_REDIS_ADDR` | `redis:6379` | Redis address (when type=redis) |
| `SMTP_PROXY_QUEUE_SQS_QUEUE_URL` | *(empty)* | SQS queue URL (when type=sqs) |
| `SMTP_PROXY_QUEUE_SQS_DLQ_URL` | *(empty)* | SQS dead-letter queue URL (when type=sqs) |
| `SMTP_PROXY_QUEUE_SQS_REGION` | *(empty)* | AWS region for SQS (when type=sqs) |
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
  port: 2587
  max_connections: 1000
  max_message_size: 26214400  # 25MB

queue:
  type: "redis"               # redis | sqs
  redis_addr: "localhost:6379" # for redis
  stream_name: "smtp-proxy"
  workers: 10
  block_timeout: "5s"
  # sqs_queue_url: ""         # for sqs
  # sqs_dlq_url: ""           # for sqs
  # sqs_region: "us-east-1"   # for sqs

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
  mode: "sync"                  # sync | async (requires redis or sqs)

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
| POST | `/api/v1/groups/{id}/service-accounts` | Owner/Admin | Create SMTP service account |
| PATCH | `/api/v1/groups/{id}/service-accounts/{uid}` | Owner/Admin | Update service account |
| POST | `/api/v1/groups/{id}/service-accounts/{uid}/api-keys` | Owner/Admin | Create API key for service account |
| GET | `/api/v1/groups/{id}/service-accounts/{uid}/api-keys` | Owner/Admin | List API keys |
| PATCH | `/api/v1/groups/{id}/service-accounts/{uid}/api-keys/{keyId}` | Owner/Admin | Update API key status (activate/deactivate) |
| DELETE | `/api/v1/groups/{id}/service-accounts/{uid}/api-keys/{keyId}` | Owner/Admin | Delete API key |
| GET | `/api/v1/groups/{id}/activity` | Member | List activity logs |

Group types: `system` (platform admin), `company` (tenant organization)

### Users (Unified Auth)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/users` | Authenticated | List users |
| POST | `/api/v1/users` | Authenticated | Create user |
| GET | `/api/v1/users/{id}` | Authenticated | Get user |
| PATCH | `/api/v1/users/{id}/status` | Authenticated | Update user status |
| DELETE | `/api/v1/users/{id}` | Authenticated | Soft delete user |
| POST | `/api/v1/users/{id}/restore` | Authenticated | Restore soft-deleted user |
| GET | `/api/v1/users/deleted` | Admin | List soft-deleted users |
| POST | `/api/v1/users/{id}/reset-api-key` | Authenticated | Reset API key |
| POST | `/api/v1/groups/{id}/service-accounts/{uid}/reset-api-key` | Owner/Admin | Reset service account API key |

Account types: `user` (JWT login), `smtp` (SMTP sending account)

Deleted users are soft-deleted with a 30-day retention period. A daily cleanup job in the queue-worker automatically purges users whose `deleted_at` exceeds 30 days.

### SMTP Authentication

SMTP service accounts authenticate via SASL PLAIN using `username@group_id` + `api_key` (as the password). The `group_id` is the UUID of the group (the `id` field in the group response). Each service account supports multiple API keys stored in the `api_keys` table with bcrypt-hashed credentials, a 12-character prefix for fast lookup, and an `is_active` flag per key. Usernames are unique per group (not globally) and are always stored in lowercase. The sender address (MAIL FROM) is independent of the login credentials, restricted only by `allowed_domains`.

```bash
# 1. Create service account (no API key generated at this step)
curl -X POST http://localhost:8080/api/v1/groups/<group-uuid>/service-accounts \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"username": "sender", "provider_id": "<provider-uuid>", "allowed_domains": ["example.com"]}'

# 2. Create an API key for the service account
curl -X POST http://localhost:8080/api/v1/groups/<group-uuid>/service-accounts/<sa-uuid>/api-keys \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"label": "production", "api_key_expires_in": "30d"}'
# Response includes plaintext api_key (shown only once)

# SMTP login format: username@group_id
# Username: sender@<group_uuid>
# Password: <api_key from response>
```

API keys can optionally have an expiration set via `api_key_expires_in` (e.g., `"7d"`, `"30d"`, `"365d"`). Expired or deactivated keys are rejected at SMTP AUTH. Each key can be individually activated/deactivated via the `is_active` flag without affecting other keys on the same service account.

### ESP Providers (Unified Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/providers` | Create provider |
| GET | `/api/v1/providers` | List providers (`?group_id=` to filter by group accessibility) |
| GET | `/api/v1/providers/{id}` | Get provider |
| PUT | `/api/v1/providers/{id}` | Update provider |
| DELETE | `/api/v1/providers/{id}` | Delete provider |
| GET | `/api/v1/providers/{id}/access` | List group access grants |
| POST | `/api/v1/providers/{id}/access` | Grant group access |
| DELETE | `/api/v1/providers/{id}/access/{gid}` | Revoke group access |
| POST | `/api/v1/providers/{id}/send` | Send a test email directly through the provider |

Supported provider types: `sendgrid`, `ses`, `mailgun`, `smtp`, `msgraph`

Provider visibility: `global` (all groups), `shared` (granted groups only), `private` (owner group only)

#### Provider Configuration

Each provider type requires different fields. The `api_key` and credentials are stored in the `smtp_config` JSONB column:

| Provider | Required `smtp_config` Fields |
|----------|-------------------------------|
| SendGrid | `api_key` |
| SES | `api_key` (Access Key ID), `secret_key` (Secret Access Key), `region`; optional: `default_sender` |
| Mailgun | `api_key`, `domain` |
| Microsoft Graph | `tenant_id`, `client_id`, `client_secret`, `user_id` |
| SMTP | `host`, `port`, `username`, `password`, `encryption` |
| Stdout | *(none)* |

**SES** uses AWS Signature V4 to sign every HTTP request to the SES v2 API. The `api_key` field holds the AWS Access Key ID and `secret_key` holds the AWS Secret Access Key. Requests are signed using the `aws-sdk-go-v2` signer (no full AWS SDK dependency for the HTTP call itself). Both Simple (text/HTML) and Raw (MIME with attachments) send modes are supported. When `default_sender` is set, it overrides the SMTP session's `MAIL FROM` address, ensuring emails are always sent from the SES verified identity.

#### Send Test Email

Send a test email directly through a provider, bypassing the queue. Useful for verifying provider configuration. Results are not recorded in delivery logs or statistics.

```bash
curl -X POST http://localhost:8080/api/v1/providers/<provider-uuid>/send \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "sender@example.com",
    "to": "recipient@example.com",
    "subject": "Test Email",
    "body": "Hello from SMTP Proxy"
  }'
```

Response (success):

```json
{
  "success": true,
  "provider_message_id": "abc123",
  "duration_ms": 342
}
```

Response (failure):

```json
{
  "success": false,
  "error": "sendgrid: 401 Unauthorized",
  "duration_ms": 150
}
```

The admin UI also provides a **Send Test Email** button on the provider detail page, which opens a dialog for composing and sending test emails with inline result display.

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
# Configure a SendGrid provider
curl -X POST http://localhost:8080/api/v1/providers \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-sendgrid",
    "provider_type": "sendgrid",
    "smtp_config": { "api_key": "SG.xxx" },
    "enabled": true
  }'

# Configure an Amazon SES provider
curl -X POST http://localhost:8080/api/v1/providers \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-ses",
    "provider_type": "ses",
    "smtp_config": {
      "api_key": "<AWS Access Key ID>",
      "secret_key": "<AWS Secret Access Key>",
      "region": "us-east-1"
    },
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
| SMTP enqueue (session to queue) | 3 | 500ms, 1s, 2s | Status: `enqueue_failed`, SMTP 451 |
| Worker storage read | 3 | 1s, 2s, 4s | Status: `storage_error`, delivery log |
| Worker ESP delivery | 5 | 30s, 1m, 2m, 5m, 15m (+jitter) | Move to DLQ |

Failed messages in the dead-letter queue can be reprocessed via `POST /api/v1/dlq/reprocess`.

## Database

PostgreSQL 18 with 25 migrations applied automatically on api-server startup (with advisory lock for safe concurrent deployment).

**Tables:** `groups`, `group_members`, `users`, `api_keys`, `esp_providers`, `provider_group_access`, `routing_rules`, `messages`, `delivery_logs`, `sessions`, `activity_logs`

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
# Default: sends one test email via STARTTLS on port 2587
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
| `--port` | `2587` | SMTP port |
| `--tls` | `starttls` | `starttls`, `implicit`, or `none` |
| `--insecure` | `false` | Skip TLS certificate verification |
| `--user` | *(empty)* | SMTP AUTH username |
| `--password` | *(empty)* | SMTP AUTH password |
| `--from` | *(required)* | Sender email address |
| `--to` | *(required)* | Recipient address (repeatable) |
| `--cc` | *(empty)* | CC recipient (repeatable) |
| `--bcc` | *(empty)* | BCC recipient (repeatable) |
| `--subject` | `Test Email` | Email subject |
| `--body` | `This is a test...` | Plain text body |
| `--html` | *(empty)* | HTML body (sends multipart/alternative) |
| `--attach` | *(empty)* | File attachment path (repeatable) |
| `--count` | `1` | Number of emails to send |
| `--rate` | `1` | Emails per second |

## Testing

### Unit Tests

```bash
# Run all Go tests (Go is only available inside Docker)
docker run --rm -w /app \
  -v $(pwd)/server:/app \
  golang:1.26-alpine \
  sh -c "go test ./... -count=1"
```

### E2E Integration Test

A full end-to-end test script covers the entire flow from clean build to email delivery:

```bash
# Run the full E2E suite (requires Docker)
bash e2e-test.sh
```

The script performs:

1. **Clean build** - `docker compose down -v` + `docker compose up -d --build`
2. **API data setup** - Login, create providers (private/global/group-scoped), create group, create human user and SMTP service account
3. **SMTP send (simple)** - Plain text email with from, to, subject, body
4. **SMTP send (complex)** - Email with from, to, CC, BCC, HTML body, and file attachments
5. **Delivery verification** - Check SMTP server and queue-worker logs for successful delivery

Test case documentation: [`docs/e2e-test-cases.md`](docs/e2e-test-cases.md) (32 test cases).

### Manual SMTP Test

```bash
# Using the built-in test-client (after services are running)
docker compose run --rm test-client \
  --host=smtp-server --port=2587 --tls=starttls --insecure \
  --user="<username>@<group_id>" --password="<api_key>" \
  --from="sender@example.com" \
  --to="recipient@example.com" \
  --cc="cc@example.com" \
  --subject="Test" \
  --body="Hello"
```

## Development

```bash
# Build only (verify compilation)
docker build --target builder -f server/Dockerfile .

# Rebuild a single service
docker compose up -d --build smtp-server
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.26 |
| SMTP Server | go-smtp + go-sasl |
| HTTP Router | chi v5 |
| Database | PostgreSQL 18 (pgx v5, sqlc) |
| Queue | Redis 7.4 Streams / AWS SQS |
| Auth | JWT (HS256) + API keys for SMTP (bcrypt-hashed, prefix-based lookup) |
| Metrics | Prometheus client_golang |
| Logging | zerolog |
| Config | Viper |
| Object Storage | AWS SDK v2 (S3-compatible) |
| Container | Docker multi-stage builds |
