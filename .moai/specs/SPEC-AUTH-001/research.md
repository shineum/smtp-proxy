# Deep Codebase Research - Unified Account System Redesign

**Task:** Explore smtp-proxy codebase for SPEC-AUTH-001 unified account system redesign
**Date:** 2026-02-27
**Scope:** Complete architecture, data model, auth flows, API layer, and integration points

---

## Executive Summary

The smtp-proxy project currently implements two disconnected authentication systems:
1. **Legacy accounts system** (server/internal/storage/accounts.sql.go): SMTP auth only, API key auth, no roles, minimal metadata
2. **Multi-tenant users system** (server/internal/storage/users.sql.go): JWT-based web/API auth, role-based access, tenant isolation

The codebase is structured for eventual merging but currently bifurcated at the storage, auth, and API layers. Key integration challenges exist around scoping (accountID vs userID vs tenantID), SMTP authentication with group context, and the delivery pipeline's dependency on accountID.

---

## Part 1: Current Authentication Systems

### 1.1 Legacy Account System (Accounts Table)

**Location:** `server/internal/storage/models.go:155-165`, `server/internal/storage/queries/accounts.sql`

**Data Model:**
```go
type Account struct {
    ID             uuid.UUID          // SMTP auth username lookup key
    Name           string             // UNIQUE - SMTP AUTH username
    Email          string             // Informational, not auth key
    PasswordHash   string             // bcrypt hash (server/internal/auth/bcrypt.go)
    AllowedDomains []byte             // JSONB array of domain restrictions
    ApiKey         string             // UNIQUE - Bearer token for API key auth
    CreatedAt      pgtype.Timestamptz
    UpdatedAt      pgtype.Timestamptz
    TenantID       pgtype.UUID        // Added in migration 007, currently sparse
}
```

**Auth Methods:**
- SMTP AUTH: `username` + `password` via SASL PLAIN (server/internal/smtp/session.go:53-100)
- API Key: Bearer token in Authorization header (server/internal/auth/middleware.go:75-109)

**Key Queries (accounts.sql):**
- `GetAccountByName`: SMTP AUTH username lookup (line 12-13)
- `GetAccountByAPIKey`: API key bearer token lookup (line 9-10)
- `CreateAccount`: New account provisioning (line 1-4)
- `UpdateAccount`: Modify allowed_domains and email (line 15-19)
- `ListAccounts`: All accounts without pagination (line 24-25)

**Integration Points:**
- SMTP session uses `s.queries.GetAccountByName(s.ctx, username)` at session.go:61
- API router uses `accountLookup` closure: `cfg.Queries.GetAccountByAPIKey(ctx, apiKey)` at router.go:67-72
- All API key-protected routes use `auth.BearerAuth(accountLookup)` middleware at router.go:76
- Message enqueue stores accountID at worker/handler.go:64-80
- Delivery logs reference accountID: `storage.DeliveryLog.AccountID` at models.go:197

**Current Scoping:**
- Accounts are **NOT** scoped to tenants (sparse TenantID field, legacy design)
- Each account is an independent SMTP sender
- API key auth returns only accountID, no tenant context
- No group concept exists in this tier

---

### 1.2 Multi-Tenant User System (Users Table)

**Location:** `server/internal/storage/models.go:263-274`, `server/internal/storage/queries/users.sql`

**Data Model:**
```go
type User struct {
    ID             uuid.UUID          // Identity key
    TenantID       uuid.UUID          // REQUIRED - Foreign key
    Email          string             // UNIQUE - Web/API auth identity
    PasswordHash   string             // bcrypt hash
    Role           string             // 'owner' | 'admin' | 'member'
    Status         string             // 'active' | 'suspended' | 'deleted'
    FailedAttempts int32              // Rate limiting counter
    LastLogin      pgtype.Timestamptz
    CreatedAt      pgtype.Timestamptz
    UpdatedAt      pgtype.Timestamptz
}
```

**Auth Method:**
- JWT-based: Email + Password → Access Token + Refresh Token (api/auth_handler.go:35-120)
- JWT Claims: `sub` (userID), `tenant_id`, `email`, `role` (auth/jwt.go:22-27)

**Key Queries (users.sql):**
- `GetUserByEmail`: Login lookup (line 9-10)
- `GetUserByID`: Token validation (line 6-7)
- `ListUsersByTenantID`: Tenant member list (line 12-13)
- `CreateUser`: New user provisioning (line 1-4)
- `UpdateUserRole`: Role management (line 15-19)
- `UpdateUserStatus`: Suspension/deletion (line 21-25)
- `IncrementFailedAttempts`: Rate limit tracking (line 32-35)

**Integration Points:**
- Login uses `queries.GetUserByEmail(r.Context(), req.Email)` at api/auth_handler.go:62
- JWT middleware extracts claims at auth/middleware.go:114-161
- Context helpers: `TenantFromContext()`, `UserFromContext()`, `RoleFromContext()` at auth/middleware.go
- RLS policies enforce tenant isolation: `tenant_id = current_setting('app.current_tenant_id')` at migrations/007:89

**Current Scoping:**
- Every user is scoped to **exactly one tenant** in the User record
- Multi-group membership is **NOT** supported in current schema
- Role is tenant-scoped but not group-specific
- No many-to-many membership table exists

---

### 1.3 Tenant (Group) System

**Location:** `server/internal/storage/models.go:252-261`, `server/internal/storage/queries/tenants.sql`

**Data Model (Current):**
```go
type Tenant struct {
    ID           uuid.UUID          // Identifier
    Name         string             // UNIQUE company/organization name
    Status       string             // 'active' | 'suspended' | 'deleted'
    MonthlyLimit int32              // Email quota
    MonthlySent  int32              // Month counter
    AllowedIps   []netip.Prefix     // CIDR restrictions (unused)
    CreatedAt    pgtype.Timestamptz
    UpdatedAt    pgtype.Timestamptz
}
```

**Current Purpose:**
- Represents a single tenant/organization
- One tenant per user (1:N relationship)
- RLS context isolation point
- Quota tracking container

**Key Limitation:**
- No group_type field (cannot distinguish system vs company groups)
- No concept of user belonging to multiple groups
- No many-to-many group_members table
- No activity_logs table (only audit_logs with sparse action coverage)

---

## Part 2: JWT & Session Management

### 2.1 JWT Service (auth/jwt.go)

**Claims Structure:**
```go
// Access token
type AccessTokenClaims struct {
    TenantID string  // Current context tenant
    Email    string
    Role     string
    RegisteredClaims
}

// Refresh token
type RefreshTokenClaims struct {
    TenantID  string  // Tenant at session creation
    SessionID string  // Database session reference
    RegisteredClaims
}
```

**Generation (jwt.go:55-76, 79-99):**
- Access token expiry: configurable (default likely 15 min)
- Refresh token expiry: configurable (default likely 7 days)
- Algorithm: HS256 (HMAC-SHA256)
- Issuer: configurable from config

**Validation (jwt.go:103-142):**
- Strict method checking: only HMAC allowed
- Expiration validation: rejects expired tokens
- Claims validation: requires valid type assertion

**Implications for Redesign:**
- TenantID in JWT must become GroupID
- Single group per login (cannot embed multiple group memberships)
- Group switching requires new endpoint (per spec: POST /api/v1/auth/switch-group)

---

### 2.2 Session Management (storage/models.go:243-250, queries/sessions.sql)

**Current Session Model:**
```go
type Session struct {
    ID               uuid.UUID          // Session identifier
    UserID           uuid.UUID          // Foreign key
    TenantID         uuid.UUID          // Tenant at login time
    RefreshTokenHash string             // Refresh token hash
    ExpiresAt        pgtype.Timestamptz
    CreatedAt        pgtype.Timestamptz
}
```

**Assumptions:**
- Session = user + tenant pair
- One session per JWT refresh token
- Expiration drives refresh token invalidation
- No explicit logout tracking (only hard delete at queries/sessions.sql)

**Integration:**
- Created at api/auth_handler.go:96-127
- Validated during token refresh at api/auth_handler.go:175-210
- Deleted on logout at api/auth_handler.go:244-255

**For Redesign:**
- Session.TenantID → Session.GroupID
- Session expires via TTL at api/auth_handler.go:96 (session expires at JWT expiry)
- No session per SMTP account (SMTP auth is stateless per connection)

---

## Part 3: SMTP Authentication & Delivery Pipeline

### 3.1 SMTP Session Flow (server/internal/smtp/session.go)

**Session Struct (session.go:32-45):**
```go
type Session struct {
    ctx            context.Context
    queries        storage.Querier
    log            zerolog.Logger
    backend        *Backend
    accountID      uuid.UUID         // CRITICAL: Legacy scoping
    authenticated  bool
    allowedDomains []string
    sender         string
    recipients     []string
}
```

**Auth Flow (session.go:48-100):**
1. Client sends SASL PLAIN: `identity`, `username`, `password`
2. `s.queries.GetAccountByName(s.ctx, username)` - lookup by Account.Name
3. `auth.VerifyPassword(account.PasswordHash, password)` - bcrypt verify (auth/bcrypt.go)
4. Parse `account.AllowedDomains` JSONB → `s.allowedDomains` string array
5. Store `s.accountID = account.ID` in session (no group context)
6. Set `s.authenticated = true`

**Domain Validation (session.go:105-142):**
- MAIL FROM command validates sender domain against `s.allowedDomains`
- Enforces account's domain restriction list
- Returns SMTP 550 error if domain not allowed

**Message Enqueue (session.go:190-280+):**
- DATA command triggers message persistence
- Creates Message record with `accountID`, `tenantID` (sparse), sender, recipients
- Body stored inline or via MessageStore reference
- Metadata includes MIME headers if present
- Calls `s.backend.delivery.DeliverMessage(ctx, &delivery.Request{MessageID, AccountID, TenantID})`

**Critical Gap for Redesign:**
- accountID stored but group context is **missing**
- SMTP accounts cannot have explicit group association
- Message routing has no group context (only account context)
- Allow list enforcement is per-account, not per-group

---

### 3.2 Message Storage & Delivery Pipeline (internal/delivery/service.go, internal/worker/handler.go)

**Delivery Request (delivery/service.go:16-23):**
```go
type Request struct {
    MessageID uuid.UUID  // Message lookup key
    AccountID uuid.UUID  // Account that sent message (for quota/routing)
    TenantID  string     // String (unusual type choice)
}
```

**Worker Message Handler (worker/handler.go:36-80):**
1. Receives queue.Message with ID
2. Fetches Message by ID from DB: `h.queries.GetMessageByID(ctx, messageID)`
3. Message record contains: `accountID`, `tenantID`, sender, recipients, body (or storage ref)
4. Resolves ESP provider via `h.resolver.Resolve(ctx, accountID)` - **accountID-based routing**
5. Sends via provider, logs delivery result with timestamp/status/provider_message_id
6. Creates DeliveryLog record with Message.AccountID and Message.TenantID

**Provider Resolution (worker/handler.go:31-34):**
- Interface: `type providerResolver interface { Resolve(ctx context.Context, accountID uuid.UUID) }`
- Current: `ListProvidersByAccountID(accountID)` - per account, not per group
- Creates coupling: provider selection is account-specific

**Quota Tracking (worker/handler.go):**
- Calls `h.queries.IncrementMonthlySent(ctx, tenantID)` - increments tenant quota
- Quota is at tenant level, not account level (inconsistency with accountID routing)

**For Redesign:**
- Messages must store `user_id` (SMTP account) **and** `group_id`
- Provider resolution should be group-scoped: `ListProvidersByGroupID(groupID)`
- SMTP accounts belong to exactly one group at creation (enforced constraint)
- Quota is group-level (not tenant-level, not account-level)

---

## Part 4: API Layer & Route Structure

### 4.1 Router Architecture (api/router.go:27-100+)

**Current Route Organization:**
```
Global Middleware
├── CorrelationID, Logging, Recovery
├─ /healthz, /readyz (no auth)
├─ POST /api/v1/accounts (no auth, legacy account creation)
├─ POST /api/v1/webhooks/* (no auth, ESP provider callbacks)
├─ POST /api/v1/auth/login (no auth, multi-tenant)
├─ POST /api/v1/auth/refresh (no auth, multi-tenant)
├─ POST /api/v1/auth/logout (no auth, multi-tenant)
└─ /api/v1/* (Bearer auth via accountLookup)
    ├── GET /accounts/{id} (get by account API key)
    ├── PUT /accounts/{id}
    ├── GET /providers
    ├── GET /routing-rules
    ├── POST /webhooks/dlq/* (dead-letter queue reprocess)
    └─ /api/v1/* (JWT auth, separate tree below)
        ├── GET /tenants (list all)
        ├── POST /users
        ├── GET /users/{id}
        └── PATCH /users/{id}/role
```

**Dual Authorization Trees:**
1. **Legacy account API key auth** (router.go:66-90+): Uses accountLookup function, stores accountID in context
2. **Multi-tenant JWT auth** (router.go:92+): Uses JWTAuth middleware, stores userID + tenantID + role in context

**Problem:**
- Routes appear in both trees (e.g., users can be created both ways)
- Account and user contexts are separate
- No unified group/account membership model
- TenantContext middleware (auth/tenant.go:12-33) sets RLS via `app.current_tenant_id` session variable

---

### 4.2 Account Handler (api/account_handler.go:1-60)

**CreateAccountHandler (not shown in limit but referenced):**
- No auth required - legacy behavior for backward compatibility
- Takes: name (SMTP username), email, password, allowed_domains
- Generates API key via `auth.GenerateAPIKey()` (auth/apikey.go:13-19)
- Stores in accounts table with sparse tenant_id (null)
- Returns account response with API key (only on creation)

**Key Observation:**
- Account creation is not scoped to tenant (legacy standalone accounts)
- API key is user-facing secret (returned on creation only, no retrieval after)
- Allowed domains are account-level restrictions, not group-level

---

### 4.3 User Handler (api/user_handler.go:1-50+)

**Data Structures:**
```go
type createUserRequest struct {
    Email    string
    Password string
    Role     string  // 'owner' | 'admin' | 'member'
}

type userResponse struct {
    ID        uuid.UUID
    TenantID  uuid.UUID
    Email     string
    Role      string
    Status    string
    LastLogin *time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**Integration:**
- POST /api/v1/users requires JWT auth + tenant context
- User is created with tenant_id from JWT claims
- Role is set at creation time
- Status defaults to 'active'

**For Redesign:**
- Need to add `account_type` field (human vs smtp)
- Need to support SMTP account creation with username field
- Role becomes group-level (stored in group_members table, not users table)
- User.Role will be deprecated

---

### 4.4 Auth Handler (api/auth_handler.go:35-127+)

**Login Flow:**
1. Client: POST /api/v1/auth/login { email, password }
2. Handler: Check rate limit via `rateLimiter.CheckLoginRateLimit(ctx, email)` (auth/ratelimit.go)
3. Handler: Lookup user by email: `queries.GetUserByEmail(ctx, email)`
4. Handler: Check user.Status == 'active'
5. Handler: Verify password via `auth.VerifyPassword(user.PasswordHash, password)` (auth/bcrypt.go:59-65)
6. Handler: Create session record: `queries.CreateSession(ctx, ...)`
7. Handler: Generate tokens via `jwtService.GenerateAccessToken(userID, tenantID, email, role)` + refresh token
8. Handler: Return { access_token, refresh_token, token_type: "Bearer", expires_in }
9. Handler: Audit log: `auditLogger.LogAuthAttempt(ctx, r, tenantID, userID, action)`

**Rate Limiting (auth/ratelimit.go):**
- Login attempt tracking per email
- Configurable lockout duration (default likely 15 min after N failures)
- Per-account failure counter on User record: `failed_attempts` (user_handler_test.go)

**For Redesign:**
- Login request should accept optional `group_id` parameter
- If user belongs to multiple groups, must select group
- JWT claims must include group_id (currently tenant_id)
- Group context flows through to role evaluation

---

## Part 5: RBAC & Authorization

### 5.1 RBAC Middleware (auth/rbac.go:1-32)

**Current Implementation:**
```go
func RequireRole(roles ...string) func(http.Handler) http.Handler {
    // Allowed is a set of role strings
    allowed := make(map[string]struct{}, len(roles))

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            role := RoleFromContext(r.Context())
            if _, ok := allowed[role]; !ok {
                http.Error(w, `{"error":"insufficient permissions"}`, 403)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

**Semantics:**
- Simple string matching from JWT claims
- No group context in authorization logic
- No system admin vs company admin distinction
- No per-resource scoping

**For Redesign:**
- Role is group-scoped (stored in group_members.role, not users.role)
- Need system-level vs group-level permission checks
- Need group_id + role-based authorization
- Example: system.admin can create groups; company.admin can manage members in their group

---

### 5.2 Tenant Context RLS (auth/tenant.go:12-39)

**Mechanism:**
```go
func TenantContext(pool *pgxpool.Pool) func(http.Handler) http.Handler {
    // Middleware that sets PostgreSQL session variable
    setTenantID(r.Context(), pool, tenantID)
    // Subsequent queries see: current_setting('app.current_tenant_id') = tenantID
}
```

**RLS Policies (migrations/007:83-117):**
- Users: `WHERE tenant_id = current_setting('app.current_tenant_id')`
- Sessions: Same isolation
- Audit logs: Same isolation
- Accounts: `OR tenant_id IS NULL` (legacy accounts have null tenant_id, always visible)
- Messages, Delivery Logs: Same pattern

**For Redesign:**
- RLS must shift to `app.current_group_id` (or keep app.current_tenant_id, rename to current_group_id)
- Accounts table should be removed (merged into users)
- Need to handle users belonging to multiple groups

---

## Part 6: Activity & Audit Logging

### 6.1 Current Audit System (auth/audit.go, storage/models.go:167-178)

**Audit Log Structure:**
```go
type AuditLog struct {
    ID           uuid.UUID
    TenantID     uuid.UUID
    UserID       pgtype.UUID        // Nullable (who performed action)
    Action       string             // 'auth.login' | 'admin.create_user' | etc.
    ResourceType string             // 'session' | 'user' | 'tenant'
    ResourceID   sql.NullString     // Target resource ID
    Result       string             // 'success' | 'failure'
    Metadata     []byte             // JSONB arbitrary data
    IpAddress    *netip.Addr        // INET type
    CreatedAt    pgtype.Timestamptz
}
```

**Actions Tracked (auth/audit.go:15-24):**
- AuditActionLogin = "auth.login"
- AuditActionLoginFailed = "auth.login_failed"
- AuditActionLogout = "auth.logout"
- AuditActionTokenRefresh = "auth.token_refresh"
- AuditActionCreateUser = "admin.create_user"
- AuditActionUpdateRole = "admin.update_role"
- AuditActionCreateTenant = "admin.create_tenant"
- AuditActionDeleteTenant = "admin.delete_tenant"

**Persistence (auth/audit.go:114-143):**
- AuditLogger wraps AuditStore (interface)
- Logs to zerolog AND database
- Stores metadata as JSONB (e.g., {"reason": "rate limited"})
- IP extraction via X-Forwarded-For, X-Real-IP, or RemoteAddr

**For Redesign:**
- audit_logs table is **replaced** by activity_logs (richer schema from spec)
- activity_logs.changes column tracks before/after diffs (JSONB)
- activity_logs.comment allows human notes (optional TEXT)
- More action types: 'suspend', 'activate', 'delete', 'login_failed', 'password_change'

---

## Part 7: Data & Storage Layer

### 7.1 Migration Strategy (server/migrations/)

**Existing Migrations (up to 009):**
- 001: Create accounts table (accounts, api_key index)
- 002: Create ESP providers (esp_providers, linked to accounts)
- 003: Create routing rules (linked to accounts)
- 004: Create messages table (linked to accounts)
- 005: Create delivery_logs (linked to accounts)
- 006: Enhance delivery_logs (add more fields)
- 007: Create tenants + users + sessions + audit_logs (NEW multi-tenant layer)
- 008: Restructure messages (add tenant_id column)
- 009: Enhance delivery_logs (add more fields, tenant_id linking)

**Key Observation:**
- Migrations add tenant_id columns to existing tables **additively** (columns added, old logic untouched)
- No removal of account_id columns
- accounts table still exists but is side-by-side with users
- RLS policies use OR logic to include both old and new models

**For Redesign:**
- Single migration 010_unified_auth.up.sql will:
  1. Rename tenants table → groups, add group_type column
  2. Add columns to users: username, account_type, api_key, allowed_domains
  3. Create group_members table (many-to-many)
  4. Migrate accounts → users (as account_type='smtp')
  5. Migrate accounts-to-tenant → group_members assignments
  6. Drop accounts table after migration
  7. Drop users.tenant_id (users no longer have implicit tenant_id)
  8. Update FKs: esp_providers.account_id → group_id, routing_rules.account_id → group_id
  9. Drop audit_logs, create activity_logs

---

### 7.2 sqlc Code Generation (storage/models.go, storage/querier.go)

**Tool:** sqlc v1.28.0 (code generated from .sql files in storage/queries/)

**Query Files:**
- accounts.sql: Account CRUD
- users.sql: User CRUD
- tenants.sql: Tenant CRUD
- sessions.sql: Session CRUD
- audit_logs.sql: Audit log writes
- messages.sql: Message enqueue/retrieval
- delivery_logs.sql: Delivery status tracking
- providers.sql: ESP provider CRUD
- routing_rules.sql: Routing rule CRUD

**Regeneration Required For Redesign:**
1. Update accounts.sql → remove (merged into users)
2. Update users.sql → add account_type, username, api_key, allowed_domains fields
3. Create groups.sql → replace tenants.sql (add group_type column, queries)
4. Create group_members.sql → new many-to-many queries
5. Create activity_logs.sql → replace audit_logs.sql (new schema with changes, comment fields)
6. Update esp_providers.sql → change account_id FK → group_id FK
7. Update routing_rules.sql → same FK change
8. Update messages.sql → change account_id → user_id (SMTP account), add group_id
9. Update delivery_logs.sql → change account_id → user_id, change tenant_id → group_id
10. Run `sqlc generate` to regenerate storage/models.go, storage/*.sql.go files

---

## Part 8: Bootstrap & Initialization (cmd/api-server/main.go)

**Current Startup (main.go:22-100+):**
1. Load config from environment/config files
2. Initialize logger (zerolog)
3. Connect to PostgreSQL (create connection pool)
4. Create sqlc Queries instance
5. Initialize JWTService with signing key, expiry configs
6. Create AuditStore (bridges auth.AuditStore to sqlc)
7. Create AuditLogger (wraps AuditStore + zerolog)
8. Create RateLimiter (for login rate limiting)
9. Build router via NewRouterWithConfig(RouterConfig{...})
10. Start HTTP server (not shown in limit)

**Missing: Seed Logic**
- No bootstrap of system admin account
- No check for initial setup
- Spec requires: auto-create "system" group + admin user on first startup

**For Redesign:**
- Add seed check in main.go before starting server:
  ```go
  // Check if system group exists
  systemGroup, err := queries.GetGroupByName(ctx, "system")
  if err == sql.ErrNoRows {
      // Create system group + admin user
      // Log credentials to stdout
  }
  ```
- System admin email from SMTP_PROXY_ADMIN_EMAIL env (default admin@localhost)
- Password from SMTP_PROXY_ADMIN_PASSWORD env (required in production)

---

## Part 9: Integration Points & Coupling Analysis

### 9.1 Critical Integration Points

**1. SMTP Session → Message Enqueue → Worker Delivery**
- Entry: SMTP client authenticates via Account.Name (username)
- Session stores: accountID, allowedDomains (from Account)
- Message creation: stores accountID, tenantID (sparse)
- Delivery: worker resolves provider via accountID
- **Problem:** accountID is connection-level, group context is missing
- **Solution:** SMTP accounts must have explicit group assignment; worker uses userID + groupID for provider resolution

**2. User Authentication → JWT Claims → API Authorization**
- Entry: /api/v1/auth/login { email, password }
- Session created: stores userID, tenantID
- JWT claims: sub=userID, tenant_id, email, role
- Middleware: extracts claims, sets context variables
- RBAC: RequireRole checks JWT role
- **Problem:** role is not group-specific; JWT has only one tenant_id
- **Solution:** JWT must include group_id; role stored in group_members, not users; RBAC checks (group_id, role) tuple

**3. API Key Authentication → Account Context**
- Entry: Bearer token in Authorization header
- Middleware: looks up Account by api_key
- Context: stores accountID only (no tenantID, no user context)
- **Problem:** different context from JWT auth (accountID vs userID vs tenantID)
- **Solution:** Merge into users.api_key field; api_key auth should resolve user + groups, set full context

**4. Message Quota Tracking**
- Write: SMTP session enqueues message (increments counter)
- Tracking: Tenant.MonthlySent (incremented by worker)
- Reset: Monthly batch job (not shown, likely external)
- **Problem:** quota is tenant-level but scoped by accountID routing
- **Solution:** quota is group-level; SMTP accounts belong to one group; increment group.monthly_sent

**5. RLS Enforcement**
- Trigger: TenantContext middleware sets `app.current_tenant_id`
- Scope: all user/session/audit queries filter by this variable
- Legacy: accounts table has `OR tenant_id IS NULL` to include null accounts
- **Problem:** two-tier model (old tenant_id, new group_id)
- **Solution:** rename current_tenant_id → current_group_id; drop legacy OR clause once accounts merged

---

### 9.2 Dependency Graph

```
main.go (bootstrap)
├── config.Load()
├── storage.NewDB(ctx, dbURL, ...)
├── queries := storage.New(db.Pool)  // sqlc instance
├── auth.JWTService (uses signing key from config)
├── auth.AuditLogger (uses queries.CreateAuditLog)
├── auth.RateLimiter (uses in-memory store or custom backend)
└── api.NewRouterWithConfig(RouterConfig)
    ├── auth.BearerAuth(accountLookup)
    │   └── queries.GetAccountByAPIKey(ctx, apiKey)
    ├── auth.JWTAuth(jwtService)
    │   └── jwtService.ValidateAccessToken(tokenStr)
    ├── auth.TenantContext(db.Pool)
    │   └── setTenantID(ctx, pool, tenantID) - RLS enforcement
    ├── handlers
    │   ├── CreateAccountHandler(queries)
    │   │   └── queries.CreateAccount(ctx, ...)
    │   ├── LoginHandler(queries, jwtService, auditLogger, rateLimiter)
    │   │   ├── queries.GetUserByEmail(...)
    │   │   ├── jwtService.GenerateAccessToken(...)
    │   │   ├── queries.CreateSession(...)
    │   │   └── auditLogger.LogAuthAttempt(...)
    │   └── [other handlers]
    └── SMTP backend (separate from HTTP)
        ├── storage.NewDB() - shared DB connection
        ├── smtp.NewBackend(queries, deliveryService, store, log, maxConns)
        │   └── Per connection: smtp.Session with queries
        │       ├── queries.GetAccountByName(username)  // SMTP auth
        │       └── deliveryService.DeliverMessage(...) // enqueue
        └── Worker (queue processor)
            ├── queries.GetMessageByID(messageID)
            ├── providerResolver.Resolve(accountID)
            │   └── queries.ListProvidersByAccountID(accountID)
            └── queries.CreateDeliveryLog(...) / UpdateDeliveryLogStatus(...)
```

---

## Part 10: Test Coverage & Test Patterns

### 10.1 Existing Test Files

**Auth Layer Tests:**
- auth/bcrypt_test.go: Password hashing/verification
- auth/apikey_test.go: API key generation
- auth/jwt_test.go: Token generation/validation, claim extraction
- auth/middleware_test.go: Bearer auth, JWT auth, context extraction
- auth/rbac_test.go: Role-based access control
- auth/tenant_test.go: RLS context setting
- auth/ratelimit_test.go: Rate limiter functionality

**API Handler Tests:**
- api/auth_handler_test.go: Login flow, token refresh, logout, hash consistency
- api/account_handler_test.go: Account CRUD operations
- api/user_handler_test.go: User CRUD, role updates
- api/tenant_handler_test.go: Tenant creation, users in tenant
- api/provider_handler_test.go: ESP provider CRUD
- api/routing_rule_handler_test.go: Routing rule CRUD
- api/webhook_handler_test.go: Webhook processing
- api/health_handler_test.go: Health checks
- api/dlq_handler_test.go: Dead-letter queue reprocessing

**Storage Tests:**
- storage/queries_test.go: Query execution (mocked Querier)
- storage/pool_test.go: Connection pool behavior
- storage/testhelper_test.go: Test helpers

**SMTP Tests:**
- smtp/backend_test.go: Connection limits, session creation
- smtp/session_test.go: SMTP protocol, auth, message handling, domain validation
- smtp/validator_test.go: Email address validation

### 10.2 Test Pattern: Mock Querier (api/mock_querier_test.go)

**Purpose:** Test handlers without database

**Key Methods (from querier.go interface):**
- GetUserByEmail, GetUserByID, CreateUser, UpdateUserRole, etc.
- GetAccountByName, GetAccountByAPIKey, CreateAccount, etc.
- CreateSession, GetSessionByID, DeleteSession, etc.
- Mock returns provide fixtures for testing

**For Redesign:**
- Mock must support new group_members queries
- Mock must support group-scoped provider lookups
- Mock must support SMTP account lookups via username

---

## Part 11: Code Quality & Patterns

### 11.1 Error Handling

**Pattern: Explicit Error Wrapping**
```go
if err != nil {
    return fmt.Errorf("operation: %w", err)
}
```
- Used consistently in auth layer (jwt.go, bcrypt.go)
- Worker handler uses structured wrapping (handler.go:67)
- All public functions return errors explicitly

**SMTP Error Responses:**
- Custom gosmtp.SMTPError with code + message
- Example: 535 Authentication failed, 550 Sender domain not allowed

### 11.2 Logging

**Tool:** zerolog (structured logging)

**Patterns:**
- Correlation ID injected into context per request (logger.NewCorrelationID at backend_test.go:56)
- Auth events logged at info level (jwt validation, login)
- Rate limit + auth failures logged at warn level
- Database/provider errors logged at error level
- Metadata in logs: action, result, resource_type, user_id, ip_address

### 11.3 Concurrency

**SMTP Backend (backend.go:23-78):**
- Uses atomic.Int64 for active connection tracking
- NewSession increments counter, checks against maxConns limit
- No mutex (single atomic variable, simple increment/decrement)

**DB Connection Pool:**
- pgxpool.Pool handles concurrency internally
- Config: PoolMin, PoolMax from config

**Queue Worker:**
- Concurrent message processing via queue.StreamConsumer (not shown)
- Message handler is synchronous per message

### 11.4 Configuration

**Pattern: Struct-based with mapstructure**
```go
type JWTConfig struct {
    SigningKey         string        `mapstructure:"signing_key"`
    AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
    RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
    Issuer             string        `mapstructure:"issuer"`
    Audience           string        `mapstructure:"audience"`
}
```
- Environment-based overrides
- Example: SMTP_PROXY_AUTH_SIGNING_KEY env var
- Default checks in main.go (warn if signing key is default)

---

## Part 12: Reference Implementations in Codebase

### 12.1 Tenant Isolation via RLS

**Location:** migrations/007_create_tenants.up.sql:73-117

**Pattern Used:**
```sql
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY user_tenant_isolation ON users
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);
```

**Application in Redesign:**
- Same pattern for groups table (rename tenants → groups)
- group_members table: `WHERE group_id = current_setting('app.current_group_id')`
- users table: JOIN via group_members for multi-group membership check

### 12.2 API Key Generation & Validation

**Location:** auth/apikey.go:13-19, storage/queries/accounts.sql:9-10

**Pattern:**
- Generate: 32 random bytes → hex encode → 64-char string
- Validate: UNIQUE index on api_key column
- Lookup: Direct query `SELECT * WHERE api_key = $1`
- Return: Only on creation (not retrievable after)

**For Redesign:**
- Move api_key to users table
- Lookup changes: `GetUserByAPIKey(apiKey)` instead of `GetAccountByAPIKey`
- Same generation/validation logic applies

### 12.3 Bcrypt Password Hashing (auth/bcrypt.go)

**Pattern:**
```go
// Hash: bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
// Verify: bcrypt.CompareHashAndPassword(hash, password)
```

**Used Consistently:**
- Account creation (api/account_handler.go)
- User creation (api/user_handler.go)
- SMTP session auth (smtp/session.go:71)
- Login (api/auth_handler.go:84)

**For Redesign:**
- No changes to hashing algorithm
- SMTP accounts use same bcrypt mechanism
- Human users continue using bcrypt

### 12.4 Many-to-Many Member Association

**Current Implementation:** Not present (users are 1:N with tenants via tenant_id column)

**Reference Pattern in Spec:**
- group_members table: (group_id, user_id, role, created_at)
- UNIQUE(group_id, user_id) prevents duplicates
- Role per group (owner/admin/member)
- User can appear in multiple group_members rows

**Similar Pattern to Study:**
- routing_rules ↔ providers (via provider_id in routing_rules)
- delivery_logs ↔ messages (via message_id in delivery_logs)

---

## Part 13: Risk Areas & Technical Debt

### 13.1 TenantID vs GroupID Confusion

**Current State:**
- tenants table holds what will become groups
- users.tenant_id will be dropped (use group_members instead)
- accounts.tenant_id is sparse (null for legacy accounts)
- messages.tenant_id is sparse (filled during migration only)
- RLS uses app.current_tenant_id session variable

**Risk:** Incomplete migration leaves orphaned data
- Solution: Backfill messages.tenant_id during migration from accounts.id → tenant_id lookup

### 13.2 accountID in Delivery Pipeline

**Current State:**
- SMTP stores accountID in session
- Message record stores accountID
- Worker resolves provider via accountID
- Quota incremented by tenantID (not accountID) - **mismatch**

**Risk:** Quota tracking can be inconsistent
- Solution: Group assignment happens at SMTP account creation; quota is group-level

### 13.3 API Key vs JWT Authentication Mismatch

**Current State:**
- BearerAuth extracts accountID only
- JWTAuth extracts userID + tenantID + role
- Different context setup → handlers must branch on auth type

**Risk:** Code duplication, inconsistent authorization
- Solution: Unified context after merge (all auth paths set same context keys)

### 13.4 RLS Complexity in Transition

**Current State:**
- RLS policies use `OR tenant_id IS NULL` for legacy accounts
- Creates two-tier visibility logic
- Queries don't filter by tenant_id if NULL

**Risk:** Data leakage if RLS policy is not correctly written
- Solution: Drop OR clauses after full migration; validate with test suite

### 13.5 Session Expiration Logic

**Current State:**
- Session expires at JWT refresh token expiry time
- Expired sessions are lazily deleted (DeleteExpiredSessions query)
- No explicit logout that invalidates immediate

**Risk:** Sessions linger in DB, refresh tokens can be validated even after logout
- Solution: Explicit session deletion on logout (already implemented at api/auth_handler.go:244-255)

---

## Part 14: Data Flow Examples

### 14.1 SMTP Email Send Flow (Current)

```
1. Client connects to SMTP server
   smtp.Backend.NewSession() → creates smtp.Session

2. Client: EHLO
   Session sets correlation_id, log

3. Client: AUTH PLAIN username password
   session.Auth() → sasl.NewPlainServer(callback)
   queries.GetAccountByName(username) → Account record
   bcrypt.CompareHashAndPassword(account.PasswordHash, password)
   session.accountID = account.ID
   session.allowedDomains = parse(account.AllowedDomains JSONB)

4. Client: MAIL FROM <sender@domain.com>
   session.Mail() validates domain in allowedDomains
   session.sender = sender@domain.com

5. Client: RCPT TO <recipient@example.com>
   session.Rcpt() validates format
   session.recipients = [recipient@example.com, ...]

6. Client: DATA <body>
   session.Data() → enqueueMessage()
   Message created:
     id: uuid.New()
     accountID: session.accountID
     tenantID: sparse/null (from Account.TenantID)
     sender, recipients, body
     status: 'queued'

7. SMTP delivery.DeliverMessage(MessageID, AccountID, TenantID)
   Enqueues to Redis Stream for async processing

8. Worker process (separate service)
   Consumes from Redis Stream
   queries.GetMessageByID(messageID) → Message record
   resolver.Resolve(accountID) → ListProvidersByAccountID(accountID)
   provider.Send(message) → API call to ESP (SendGrid, Mailgun, etc.)
   CreateDeliveryLog(status, response_code, provider_message_id)
   UpdateMessageStatus(delivered)
   IncrementMonthlySent(tenantID) - quota tracking
```

### 14.2 Web Login Flow (Current)

```
1. Client: POST /api/v1/auth/login
   { email, password }

2. LoginHandler:
   rateLimiter.CheckLoginRateLimit(email)
   queries.GetUserByEmail(email) → User record
   Check user.Status == 'active'
   bcrypt.CompareHashAndPassword(user.PasswordHash, password)

3. Create session
   sessionID = uuid.New()
   queries.CreateSession(UserID, TenantID, hash(refreshToken))

4. Generate tokens
   accessToken = jwtService.GenerateAccessToken(userID, tenantID, email, role)
   refreshToken = jwtService.GenerateRefreshToken(userID, tenantID, sessionID)

5. Return to client
   { access_token, refresh_token, token_type, expires_in }

6. Client: subsequent requests include "Authorization: Bearer <accessToken>"

7. JWTAuth middleware
   Parses and validates JWT
   Extracts claims: sub (userID), tenant_id, email, role
   Sets context: userIDKey, tenantIDKey, userEmailKey, userRoleKey

8. TenantContext middleware
   Reads tenantID from context
   setTenantID(ctx, pool, tenantID) → sets PostgreSQL session variable
   Subsequent queries enforce RLS filtering
```

---

## Part 15: Synthesis & Key Findings

### 15.1 Data Model Readiness

**High Readiness Areas:**
- JWT claims structure can support group_id without major changes
- Session model can accommodate group_id instead of tenant_id
- RBAC pattern (RequireRole) is extensible for group-specific checks
- RLS mechanism is proven and can be adapted to group-based isolation
- Audit logging infrastructure (AuditLogger) can support activity_logs schema

**Low Readiness Areas:**
- accounts table must be entirely merged into users (no partial migration strategy)
- group_members table is completely new (no existing many-to-many pattern in codebase)
- SMTP session context is accountID-only (must be refactored to userID + groupID)
- Message routing is accountID-based (must become groupID-based)
- No existing seed/bootstrap logic (must be added to main.go)

### 15.2 Migration Complexity

**Straightforward:**
- Rename tenants → groups, add group_type
- Add columns to users table (username, account_type, api_key, allowed_domains)
- Replace audit_logs with activity_logs
- Update FK references in esp_providers, routing_rules, messages, delivery_logs
- Update RLS policies (tenant_id → group_id)
- Regenerate sqlc code

**Complex:**
- Migrate accounts → users (account_type='smtp'), preserving api_key uniqueness
- Migrate account-to-tenant association → group_members (with role='member' as default)
- Update SMTP session to lookup users by username instead of accounts by name
- Update message creation to store user_id (SMTP account) + group_id
- Update provider resolution to be group-scoped
- Update API handlers to create both human and SMTP users from unified users table

### 15.3 API & Handler Changes

**Routes to Consolidate:**
- POST /api/v1/accounts → POST /api/v1/users (with account_type field)
- POST /api/v1/auth/login → enhance with group_id selection logic
- New: POST /api/v1/auth/switch-group
- New: POST /api/v1/groups (system admin only)
- New: POST /api/v1/groups/{id}/members
- New: GET /api/v1/activity (activity log queries)
- Modify: PATCH /api/v1/users/{id}/role → operates on group_members, not users.role

**Handler Consolidation:**
- account_handler.go → merge into user_handler.go
- tenant_handler.go → rename to group_handler.go
- Add auth_group_handler.go for group management endpoints

### 15.4 Test Coverage Strategy

**High Priority:**
- SMTP account creation and auth with group scoping
- Multi-group user login and group switching
- Group-based provider resolution and routing
- Activity log creation for all operations
- Seed bootstrap (system group + admin user)

**Medium Priority:**
- Role changes within group_members table
- RBAC checks for system admin vs group admin
- RLS enforcement with group context
- API key auth with group resolution

**Lower Priority:**
- Legacy API key behavior (deprecated after migration)
- Rate limiting (existing tests pass, no major changes)
- Email validation (no changes)

---

## Part 16: Codebase Architecture Summary

```
server/
├── cmd/
│   └── api-server/main.go                  ← Bootstrap, config, DB init, seed logic
├── internal/
│   ├── auth/
│   │   ├── jwt.go                          ← JWTService, claim generation
│   │   ├── middleware.go                   ← BearerAuth, JWTAuth, context helpers
│   │   ├── rbac.go                         ← RequireRole authorization
│   │   ├── tenant.go                       ← RLS context setting (rename → group.go)
│   │   ├── bcrypt.go                       ← Password hashing
│   │   ├── apikey.go                       ← API key generation
│   │   ├── audit.go                        ← AuditLogger, audit entry structures
│   │   ├── ratelimit.go                    ← Login attempt tracking
│   │   └── *_test.go                       ← Auth layer tests
│   ├── api/
│   │   ├── router.go                       ← Route registration, middleware chains
│   │   ├── account_handler.go              ← Account CRUD (to be merged)
│   │   ├── user_handler.go                 ← User CRUD (to be enhanced)
│   │   ├── auth_handler.go                 ← Login, refresh, logout (to be enhanced)
│   │   ├── tenant_handler.go               ← Tenant CRUD (rename → group_handler.go)
│   │   ├── provider_handler.go             ← ESP provider CRUD (to be scoped to groups)
│   │   ├── routing_rule_handler.go         ← Routing rule CRUD (to be scoped to groups)
│   │   ├── webhook_handler.go              ← ESP webhook processors
│   │   ├── health_handler.go               ← Health/readiness
│   │   ├── dlq_handler.go                  ← Dead-letter queue reprocessing
│   │   ├── middleware.go                   ← HTTP middleware (logging, recovery, CORS)
│   │   ├── response.go                     ← Response helpers
│   │   └── *_test.go                       ← API handler tests
│   ├── storage/
│   │   ├── models.go                       ← GENERATED sqlc types
│   │   ├── querier.go                      ← GENERATED sqlc interface
│   │   ├── *.sql.go                        ← GENERATED sqlc methods
│   │   ├── queries/
│   │   │   ├── accounts.sql                ← To be removed
│   │   │   ├── users.sql                   ← To be enhanced
│   │   │   ├── tenants.sql                 ← To be renamed/enhanced (groups.sql)
│   │   │   ├── sessions.sql                ← Minor updates (tenant_id → group_id)
│   │   │   ├── audit_logs.sql              ← To be replaced (activity_logs.sql)
│   │   │   ├── esp_providers.sql           ← FK changes (account_id → group_id)
│   │   │   ├── routing_rules.sql           ← FK changes
│   │   │   ├── messages.sql                ← Schema changes (account_id → user_id)
│   │   │   ├── delivery_logs.sql           ← Schema changes
│   │   │   └── [other query files]
│   │   ├── pool.go                         ← Connection pool wrapper
│   │   ├── db.go                           ← DB initialization
│   │   ├── testhelper_test.go              ← Test fixtures
│   │   └── *_test.go                       ← Storage tests
│   ├── smtp/
│   │   ├── backend.go                      ← SMTP server backend, connection limits
│   │   ├── session.go                      ← SMTP session, auth flow (to be refactored)
│   │   ├── validator.go                    ← Email validation
│   │   └── *_test.go                       ← SMTP tests
│   ├── delivery/
│   │   ├── service.go                      ← Message enqueue interface
│   │   └── [async/http implementations]
│   ├── worker/
│   │   ├── handler.go                      ← Queue message processor (to be refactored)
│   │   └── [worker implementations]
│   ├── provider/
│   │   └── [ESP provider implementations]
│   ├── config/
│   │   └── [Configuration loading]
│   ├── logger/
│   │   └── [Zerolog wrapper]
│   ├── queue/
│   │   └── [Redis/message queue]
│   ├── msgstore/
│   │   └── [Message body storage]
│   └── [other packages]
├── migrations/
│   ├── 001_create_accounts.up/down.sql     ← Legacy
│   ├── 002_create_esp_providers.up/down.sql
│   ├── 003_create_routing_rules.up/down.sql
│   ├── 004_create_messages.up/down.sql
│   ├── 005_create_delivery_logs.up/down.sql
│   ├── 006_update_delivery_logs.up/down.sql
│   ├── 007_create_tenants.up/down.sql      ← Multi-tenant intro
│   ├── 008_restructure_messages.up/down.sql
│   ├── 009_enhance_delivery_logs.up/down.sql
│   └── 010_unified_auth.up/down.sql        ← NEW - full unification
└── sqlc.yaml                               ← sqlc config (points to queries/)
```

---

## Conclusions & Implications

1. **Schema unification is feasible** - the accounts and users tables can be merged with accounts becoming a special case of users (account_type='smtp')

2. **Auth layer is highly reusable** - JWT, RBAC, and RLS mechanisms are solid and require mainly parameter name changes (tenantID → groupID)

3. **SMTP session refactoring is required** - must shift from accountID-only context to userID + groupID context; username lookups change from Account table to User table

4. **Delivery pipeline is highly coupled to accountID** - provider resolution, quota tracking, and routing must all shift from accountID to groupID; this is non-trivial but mechanical

5. **Test coverage exists and is solid** - most major flows have tests; rewriting tests for new schema is straightforward since mocks are in place

6. **Bootstrap logic is missing** - seed service or main.go must be enhanced to create system group + admin user on first run

7. **Activity logging infrastructure is ready** - audit_logs table replacement is well-structured, just needs richer schema (changes, comment fields)

8. **API consolidation is achievable** - two separate trees (account API key auth, user JWT auth) can be unified once users table includes account_type and api_key fields

