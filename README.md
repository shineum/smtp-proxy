# smtp-proxy

SMTP proxy server that accepts email via SMTP and delivers through configurable ESP providers (SendGrid, SES, Mailgun, Microsoft Graph). Supports multi-tenant accounts, sync/async delivery modes, and a REST API for management.

## Quick Start

```bash
# Start all services (zero prerequisites except Docker)
docker compose up -d --build

# Send a test email
docker compose run --rm test-client

# View logs
docker compose logs smtp-server

# Stop
docker compose down
```

The seed service automatically creates a dev account (`dev@example.com` / `dev`).

## Architecture

```
                    ┌─────────────┐
  SMTP :587/465 ──▶ │ smtp-server  │──▶ ProviderResolver ──▶ ESP (SendGrid, SES, ...)
                    └──────┬──────┘          │
                           │            (no provider?)
                           │                ▼
                    ┌──────┴──────┐     stdout default
                    │  PostgreSQL │
                    └──────┬──────┘
                           │
  REST :8080 ──────▶ │  api-server  │
                    └─────────────┘
```

**Delivery Modes:**

| Mode | Description | Redis Required |
|------|-------------|----------------|
| `sync` (default) | Delivers inline during SMTP session | No |
| `async` | Queues to Redis Streams, processed by worker | Yes |

## Services

| Service | Port | Description |
|---------|------|-------------|
| `smtp-server` | 587, 465 | SMTP listener with STARTTLS and implicit TLS |
| `api-server` | 8080 | REST API for accounts, providers, routing |
| `queue-worker` | - | Async delivery worker (when `DELIVERY_MODE=async`) |
| `postgres` | - | PostgreSQL 17 |
| `redis` | - | Redis 7.4 (queue backend) |
| `migrate` | - | Database migrations (runs once) |
| `seed` | - | Creates dev account (runs once) |
| `test-client` | - | CLI tool for sending test emails |

## Configuration

All settings can be overridden via environment variables prefixed with `SMTP_PROXY_`.

Copy `.env.example` to `.env` for local customization:

```bash
cp .env.example .env
```

### Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SMTP_PROXY_DATABASE_URL` | `postgres://smtp_proxy:smtp_proxy_dev@postgres:5432/smtp_proxy?sslmode=disable` | PostgreSQL connection string |
| `SMTP_PROXY_DELIVERY_MODE` | `sync` | `sync` or `async` |
| `SMTP_PROXY_QUEUE_REDIS_ADDR` | `redis:6379` | Redis address (async mode) |
| `SMTP_PROXY_AUTH_SIGNING_KEY` | `dev-signing-key-change-in-production` | JWT signing key |
| `SMTP_PROXY_TLS_CERT_FILE` | *(auto-generate)* | Path to TLS certificate |
| `SMTP_PROXY_TLS_KEY_FILE` | *(auto-generate)* | Path to TLS private key |
| `SMTP_PROXY_LOGGING_LEVEL` | `debug` | `debug`, `info`, `warn`, `error` |

### Application Config

Full configuration is in `server/config/config.yaml`:

```yaml
smtp:
  host: 0.0.0.0
  port: 587
  max_connections: 1000
  max_message_size: 26214400  # 25MB

delivery:
  mode: sync                  # sync | async

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

### Accounts (API Key Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts` | Create account |
| GET | `/api/v1/accounts/{id}` | Get account |
| PUT | `/api/v1/accounts/{id}` | Update account |
| DELETE | `/api/v1/accounts/{id}` | Delete account |

### ESP Providers (API Key Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/providers` | Create provider |
| GET | `/api/v1/providers` | List providers |
| GET | `/api/v1/providers/{id}` | Get provider |
| PUT | `/api/v1/providers/{id}` | Update provider |
| DELETE | `/api/v1/providers/{id}` | Delete provider |

Supported provider types: `sendgrid`, `ses`, `mailgun`, `smtp`, `msgraph`

### Authentication (JWT)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | Login (returns access + refresh tokens) |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| POST | `/api/v1/auth/logout` | Invalidate refresh token |

### Webhooks (No Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/webhooks/sendgrid` | SendGrid delivery events |
| POST | `/api/v1/webhooks/ses` | AWS SES delivery events |
| POST | `/api/v1/webhooks/mailgun` | Mailgun delivery events |

## Provider Resolution

When the SMTP server receives a message, it resolves the ESP provider for the account:

1. Look up the account's providers from the database (cached for 5 minutes)
2. Select the first enabled provider (ordered by creation date)
3. If no provider is configured, fall back to `stdout` (prints to server logs)

To configure a provider for an account, use the REST API:

```bash
curl -X POST http://localhost:8080/api/v1/providers \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "<account-uuid>",
    "name": "my-sendgrid",
    "provider_type": "sendgrid",
    "api_key": "SG.xxx",
    "enabled": true
  }'
```

## Test Client

```bash
# Default: sends one test email via STARTTLS on port 587
docker compose run --rm test-client

# Custom options
docker compose run --rm test-client \
  --from sender@example.com \
  --to recipient@example.com \
  --subject "Hello" \
  --body "Test message" \
  --count 10 \
  --rate 5
```

| Flag | Default | Description |
|------|---------|-------------|
| `--host` | `smtp-server` | SMTP server hostname |
| `--port` | `587` | SMTP port |
| `--tls` | `starttls` | `starttls`, `implicit`, or `none` |
| `--insecure` | `true` | Skip TLS certificate verification |
| `--user` | `dev@example.com` | SMTP AUTH username |
| `--password` | `dev` | SMTP AUTH password |
| `--from` | `dev@example.com` | Sender address |
| `--to` | `test@example.com` | Recipient address |
| `--count` | `1` | Number of emails to send |
| `--rate` | `1` | Emails per second |

## Database

PostgreSQL 17 with 7 migrations applied automatically on startup.

**Tables:** `accounts`, `esp_providers`, `routing_rules`, `messages`, `delivery_logs`, `tenants`, `users`, `sessions`, `audit_logs`

Data is persisted in a Docker volume (`postgres-data`). To reset:

```bash
docker compose down -v   # removes volumes
docker compose up -d --build
```

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
