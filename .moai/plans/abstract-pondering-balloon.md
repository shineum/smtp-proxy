# Plan: Per-Account Provider Resolution with Stdout Default

## Context

The current delivery system has a structural disconnect: the DB schema (`esp_providers` table) stores per-account provider credentials, but the runtime code uses a global `provider.Registry` (which starts empty) and a hardcoded `routing.Engine`. As a result, no providers are ever loaded from DB, and all deliveries fail with "provider not found".

The goal is to make the delivery path account-aware: when an SMTP message arrives, look up the account's provider config from DB (with caching), and fall back to `stdout` when no provider is configured.

## Architecture Change

```
BEFORE:
  SMTP → delivery → routing.Engine (hardcoded) → Registry (empty) → FAIL

AFTER:
  SMTP → delivery → ProviderResolver(accountID) → DB/cache lookup → Provider instance
                                                 → (no provider?) → stdout default
```

## Changes

### 1. NEW: `server/internal/provider/resolver.go`

`ProviderResolver` replaces `Registry` + `routing.Engine` in the delivery path.

```go
type ProviderResolver struct {
    queries  storage.Querier
    log      zerolog.Logger
    client   HTTPClient       // shared HTTP client for ESP providers

    mu       sync.RWMutex
    cache    map[uuid.UUID]*cachedProvider  // keyed by account ID
    cacheTTL time.Duration                 // default 5m

    stdout   Provider          // shared singleton, returned when no provider configured
}

func NewResolver(queries storage.Querier, client HTTPClient, log zerolog.Logger) *ProviderResolver
func (r *ProviderResolver) Resolve(ctx context.Context, accountID uuid.UUID) (Provider, error)
```

**Resolve flow:**
1. Check `cache[accountID]` — if present and not expired, return cached provider
2. `queries.ListProvidersByAccountID(ctx, accountID)` — get account's providers
3. Find first `enabled=true` provider (ordered by `created_at DESC` from existing query)
4. If none found → return `r.stdout` (shared singleton)
5. Convert `storage.EspProvider` → `ProviderConfig` → `NewProvider(cfg, r.client)`
6. Cache with TTL, return

**DB model → ProviderConfig mapping** (helper function `espToConfig`):
- `ProviderType` → `cfg.Type`
- `ApiKey` → `cfg.APIKey`
- `SmtpConfig` JSONB → parse for extra fields: `region`, `domain`, `tenant_id`, `client_id`, `client_secret`, `user_id`, `endpoint`

### 2. NEW: `server/internal/provider/httpclient.go`

Simple adapter wrapping `net/http.Client` to implement the existing `provider.HTTPClient` interface.

```go
type DefaultHTTPClient struct {
    client *http.Client
}

func NewHTTPClient(timeout time.Duration) *DefaultHTTPClient
func (c *DefaultHTTPClient) Do(req *HTTPRequest) (*HTTPResponse, error)
```

### 3. MODIFY: `server/internal/delivery/sync.go`

Replace `registry *provider.Registry` and `router *routing.Engine` with `resolver *provider.ProviderResolver`.

```go
// BEFORE
type SyncService struct {
    registry *provider.Registry
    router   *routing.Engine
    queries  storage.Querier
    log      zerolog.Logger
}

// AFTER
type SyncService struct {
    resolver *provider.ProviderResolver
    queries  storage.Querier
    log      zerolog.Logger
}
```

`DeliverMessage` changes:
```go
// BEFORE
providerName, err := s.router.ResolveProvider(ctx, req.TenantID)
p, err := s.registry.Get(providerName)

// AFTER
p, err := s.resolver.Resolve(ctx, req.AccountID)
```

Also update `updateStatus` to use `p.GetName()` for the provider name in delivery logs.

### 4. MODIFY: `server/internal/delivery/sync_test.go`

Update tests to use `ProviderResolver` instead of `Registry` + `Engine`.

### 5. MODIFY: `server/internal/worker/handler.go`

Same pattern as SyncService — replace `registry` + `router` with `resolver`.

```go
// BEFORE
handler := worker.NewHandler(registry, router, queries, log)

// AFTER
handler := worker.NewHandler(resolver, queries, log)
```

### 6. MODIFY: `server/cmd/smtp-server/main.go`

```go
// REMOVE
registry := provider.NewRegistry()
healthChecker := provider.NewHealthChecker(registry)
healthChecker.Start()
defer healthChecker.Stop()
router := routing.NewEngine(healthChecker)
deliverySvc = delivery.NewSyncService(registry, router, queries, log)

// ADD
httpClient := provider.NewHTTPClient(30 * time.Second)
resolver := provider.NewResolver(queries, httpClient, log)
deliverySvc = delivery.NewSyncService(resolver, queries, log)
```

### 7. MODIFY: `server/cmd/queue-worker/main.go`

Same removal of `Registry` + `Engine`, replaced with `ProviderResolver`.

```go
httpClient := provider.NewHTTPClient(30 * time.Second)
resolver := provider.NewResolver(queries, httpClient, log)
handler := worker.NewHandler(resolver, queries, log)
```

## Files Summary

| File | Action | Description |
|------|--------|-------------|
| `server/internal/provider/resolver.go` | NEW | ProviderResolver with cache + stdout default |
| `server/internal/provider/httpclient.go` | NEW | HTTP client adapter for provider.HTTPClient |
| `server/internal/delivery/sync.go` | MODIFY | Use resolver instead of registry+engine |
| `server/internal/delivery/sync_test.go` | MODIFY | Update tests for new constructor |
| `server/internal/worker/handler.go` | MODIFY | Use resolver instead of registry+engine |
| `server/cmd/smtp-server/main.go` | MODIFY | Wire ProviderResolver |
| `server/cmd/queue-worker/main.go` | MODIFY | Wire ProviderResolver |

## Not Changed (kept as-is)

- `provider/factory.go` — `NewProvider()` factory is reused by resolver
- `provider/provider.go` — Provider interface unchanged
- `routing/` — Not removed, but no longer used in delivery path. Can be cleaned up later.
- `provider/health.go` — HealthChecker not used in new flow. Can be cleaned up later.
- DB schema — No migration needed. Existing `esp_providers` table is sufficient.
- `provider_type` enum — `stdout`/`file` not added (stdout is code-level default, not DB-stored)

## Verification

1. `docker compose up -d --build` — smtp-server starts, logs `delivery mode: sync`
2. `docker compose run --rm test-client` — email sent successfully
3. `docker compose logs smtp-server` — should show:
   - `"message persisted"` (DB save)
   - `"stdout provider: message"` (stdout default output since no provider in DB)
   - `"message delivered"` with `provider=stdout`
4. Unit tests: `go test ./internal/provider/ ./internal/delivery/ ./internal/worker/`
