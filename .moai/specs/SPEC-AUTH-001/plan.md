---
id: SPEC-AUTH-001
type: plan
version: "1.0.0"
tags: [auth, groups, users, smtp, migration, rbac]
---

# Implementation Plan: SPEC-AUTH-001 Unified Account System Redesign

## 1. Implementation Strategy

### 1.1 Development Methodology

Hybrid mode (per quality.yaml `development_mode: hybrid`):

- **TDD** for new code: group_handler, activity_handler, group.go (auth), seed logic
- **DDD** for modified code: auth_handler, session.go, worker/handler.go, provider resolver

### 1.2 Build and Test Commands

```bash
# Build
docker build --target builder -f server/Dockerfile server/

# Test
docker run --rm -w /app -v $(pwd)/server:/app golang:1.24-alpine sh -c "go test ./..."

# sqlc generate
docker run --rm -w /app -v $(pwd)/server:/app sqlc/sqlc:latest generate

# Full stack integration
docker compose up -d --build
```

---

## 2. Phased Execution Plan

### Phase 1: Migration SQL

**Priority:** Primary Goal (foundation for all subsequent phases)

**Files:**

| File | Action |
|------|--------|
| `server/migrations/010_unified_auth.up.sql`   | NEW |
| `server/migrations/010_unified_auth.down.sql`  | NEW |

**Steps:**

1. Write up migration with 16-step execution order (see spec.md Section 5.1)
2. Write down migration (lossy for multi-group memberships)
3. Handle email collision: synthetic `{username}@smtp.internal` for SMTP accounts
4. Recreate all RLS policies with `app.current_group_id`

**Verification Gate:**

```bash
# Apply migration, rollback, reapply
docker compose exec postgres psql -U smtp_proxy -d smtp_proxy -f /migrations/010_unified_auth.up.sql
docker compose exec postgres psql -U smtp_proxy -d smtp_proxy -f /migrations/010_unified_auth.down.sql
docker compose exec postgres psql -U smtp_proxy -d smtp_proxy -f /migrations/010_unified_auth.up.sql
# Verify row counts match expectations
```

---

### Phase 2: sqlc Queries + Regeneration

**Priority:** Primary Goal (required for compilation)

**Files:**

| File | Action |
|------|--------|
| `server/internal/storage/queries/groups.sql`         | NEW (replaces tenants.sql) |
| `server/internal/storage/queries/group_members.sql`  | NEW |
| `server/internal/storage/queries/activity_logs.sql`  | NEW (replaces audit_logs.sql) |
| `server/internal/storage/queries/users.sql`          | MODIFY (add account_type, username, api_key fields) |
| `server/internal/storage/queries/accounts.sql`       | DELETE |
| `server/internal/storage/queries/tenants.sql`        | DELETE |
| `server/internal/storage/queries/audit_logs.sql`     | DELETE |
| `server/internal/storage/queries/providers.sql`      | MODIFY (account_id -> group_id) |
| `server/internal/storage/queries/routing_rules.sql`  | MODIFY (account_id -> group_id) |
| `server/internal/storage/queries/messages.sql`       | MODIFY (account_id -> user_id + group_id) |
| `server/internal/storage/queries/delivery_logs.sql`  | MODIFY (account_id -> user_id, tenant_id -> group_id) |
| `server/internal/storage/queries/sessions.sql`       | MODIFY (tenant_id -> group_id) |
| `server/internal/storage/models.go`                  | REGENERATE |
| `server/internal/storage/querier.go`                 | REGENERATE |
| `server/internal/storage/*.sql.go`                   | REGENERATE |

**New Queries Required:**

- `GetGroupByName(name)` -- bootstrap check
- `GetGroupByID(id)` -- detail view
- `ListGroups()` -- admin listing
- `CreateGroup(name, group_type)` -- group creation
- `UpdateGroupStatus(id, status)` -- suspend/delete
- `GetGroupMemberByUserAndGroup(user_id, group_id)` -- membership check
- `ListGroupMembersByGroupID(group_id)` -- members list
- `ListGroupsByUserID(user_id)` -- user's groups
- `CreateGroupMember(group_id, user_id, role)` -- add member
- `UpdateGroupMemberRole(id, role)` -- change role
- `DeleteGroupMember(id)` -- remove member
- `CountGroupOwners(group_id)` -- last owner check
- `GetUserByUsername(username)` -- SMTP auth
- `GetUserByAPIKey(api_key)` -- API key auth (moved from accounts)
- `CreateActivityLog(...)` -- activity logging
- `ListActivityLogs(group_id, filters)` -- activity list
- `GetActivityLogByID(id)` -- activity detail
- `ListProvidersByGroupID(group_id)` -- replaces ListProvidersByAccountID

**Verification Gate:**

```bash
# sqlc generate must succeed
docker run --rm -w /app -v $(pwd)/server:/app sqlc/sqlc:latest generate
echo $?  # must be 0
```

---

### Phase 3: Auth Layer

**Priority:** Primary Goal (core auth changes)

**Files:**

| File | Action |
|------|--------|
| `server/internal/auth/jwt.go`        | MODIFY (TenantID -> GroupID in claims) |
| `server/internal/auth/middleware.go`  | MODIFY (tenantIDKey -> groupIDKey, context helpers) |
| `server/internal/auth/rbac.go`       | MODIFY (add system admin checks, group-type-aware) |
| `server/internal/auth/tenant.go`     | RENAME to `group.go` (TenantContext -> GroupContext) |
| `server/internal/auth/audit.go`      | MODIFY (adapt to activity_logs interface) |

**Key Changes:**

- JWT claims: `TenantID` field renamed to `GroupID` (same UUID type)
- Context helpers: `TenantFromContext()` -> `GroupIDFromContext()`
- `RequireRole()` preserved but extended with `RequireSystemAdmin()` middleware
- GroupContext middleware: sets `app.current_group_id` via `SET LOCAL`
- AuditLogger adapted to write to activity_logs instead of audit_logs

**Verification Gate:**

```bash
docker run --rm -w /app -v $(pwd)/server:/app golang:1.24-alpine sh -c "go build ./internal/auth/..."
```

---

### Phase 4: SMTP Session Refactor

**Priority:** Primary Goal (SMTP must keep working)

**Files:**

| File | Action |
|------|--------|
| `server/internal/smtp/session.go` | MODIFY (accountID -> userID + groupID) |

**Key Changes:**

- Session struct: `accountID uuid.UUID` replaced with `userID uuid.UUID` + `groupID uuid.UUID`
- Auth method: `GetAccountByName` replaced with `GetUserByUsername` (WHERE account_type='smtp' AND status='active')
- Group resolution: after user lookup, query `group_members` for user's group
- Group status check: verify group is active before allowing auth
- Message enqueue: pass userID + groupID instead of accountID
- Domain validation: preserved (allowedDomains still per-user)

**Verification Gate:**

```bash
docker run --rm -w /app -v $(pwd)/server:/app golang:1.24-alpine sh -c "go build ./internal/smtp/..."
```

---

### Phase 5: API Handlers

**Priority:** Secondary Goal (new features and handler refactoring)

**Files:**

| File | Action |
|------|--------|
| `server/internal/api/group_handler.go`        | NEW (replaces tenant_handler.go) |
| `server/internal/api/activity_handler.go`      | NEW |
| `server/internal/api/auth_handler.go`          | MODIFY (multi-group login, group switching) |
| `server/internal/api/user_handler.go`          | MODIFY (support account_type, unified CRUD) |
| `server/internal/api/provider_handler.go`      | MODIFY (account_id -> group_id) |
| `server/internal/api/routing_rule_handler.go`  | MODIFY (account_id -> group_id) |
| `server/internal/api/account_handler.go`       | DELETE |
| `server/internal/api/tenant_handler.go`        | DELETE |

**Per-Handler Development Order:**

1. `group_handler.go` (TDD -- new file)
   - CreateGroup, ListGroups, GetGroup, DeleteGroup
   - ListMembers, AddMember, UpdateMemberRole, RemoveMember
2. `activity_handler.go` (TDD -- new file)
   - ListActivityLogs, GetActivityLog
3. `auth_handler.go` (DDD -- existing file)
   - Add optional group_id to login
   - Add switch-group endpoint
   - Update token refresh to use group_id
4. `user_handler.go` (DDD -- existing file)
   - Add account_type support (human vs smtp)
   - Add username field for SMTP accounts
   - Auto-generate API key for SMTP accounts
5. `provider_handler.go` (DDD -- existing file)
   - Replace account_id with group_id in all queries
6. `routing_rule_handler.go` (DDD -- existing file)
   - Replace account_id with group_id in all queries

**Verification Gate:**

```bash
# Per-handler verification during development
docker run --rm -w /app -v $(pwd)/server:/app golang:1.24-alpine sh -c "go build ./internal/api/..."
```

---

### Phase 6: Router Consolidation

**Priority:** Secondary Goal (unify route trees)

**Files:**

| File | Action |
|------|--------|
| `server/internal/api/router.go` | MODIFY (merge dual route trees) |

**Key Changes:**

- Remove legacy `accountLookup` closure and `BearerAuth(accountLookup)` middleware
- Merge API key auth into unified middleware (users.api_key lookup)
- Single `/api/v1/` route tree with JWT + API key auth
- New middleware: `RequireSystemAdmin()` for group management routes
- New middleware: `GroupContext` replaces `TenantContext`
- New routes: /groups, /groups/{id}/members, /activity, /auth/switch-group
- Remove: /accounts endpoints, /tenants endpoints
- Preserve: /webhooks/* (no auth, backward compatible)
- Preserve: /healthz, /readyz (no auth)

**Verification Gate:**

```bash
docker run --rm -w /app -v $(pwd)/server:/app golang:1.24-alpine sh -c "go build ./..."
```

---

### Phase 7: Delivery Pipeline & Worker

**Priority:** Secondary Goal (message flow adaptation)

**Files:**

| File | Action |
|------|--------|
| `server/internal/delivery/service.go`  | MODIFY (AccountID -> UserID + GroupID in Request) |
| `server/internal/worker/handler.go`    | MODIFY (provider resolution by group_id) |

**Key Changes:**

- `delivery.Request`: replace `AccountID` with `UserID` + `GroupID`
- Worker handler: `providerResolver.Resolve(ctx, groupID)` replaces `Resolve(ctx, accountID)`
- Quota tracking: `IncrementMonthlySent(ctx, groupID)` (already group-level via renamed tenant)
- DeliveryLog: store userID + groupID

**Verification Gate:**

```bash
docker run --rm -w /app -v $(pwd)/server:/app golang:1.24-alpine sh -c "go build ./internal/delivery/... ./internal/worker/..."
```

---

### Phase 8: Seed Bootstrap + Docker Compose

**Priority:** Secondary Goal

**Files:**

| File | Action |
|------|--------|
| `server/cmd/api-server/main.go` | MODIFY (add seed bootstrap) |
| `docker-compose.yml`            | MODIFY (update seed service, env vars) |

**Seed Logic in main.go:**

```
1. After DB connection, before server start
2. Check: queries.GetGroupByName(ctx, "system")
3. If not found:
   a. Create system group (group_type: 'system')
   b. Read SMTP_PROXY_ADMIN_EMAIL (default: admin@localhost)
   c. Read SMTP_PROXY_ADMIN_PASSWORD (or generate random)
   d. Create admin user (account_type: 'human')
   e. Create group_member (group_id: system, user_id: admin, role: 'owner')
   f. Log credentials to stdout
```

**docker-compose.yml Changes:**

- Remove or replace seed service that creates legacy SMTP accounts
- Add env vars: `SMTP_PROXY_ADMIN_EMAIL`, `SMTP_PROXY_ADMIN_PASSWORD`
- Bootstrap happens in app code, not in separate seed service

**Verification Gate:**

```bash
docker compose up -d --build
# Verify: admin seeded, login works
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@localhost","password":"<seeded_password>"}'
```

---

### Phase 9: Tests

**Priority:** Final Goal

**Files:**

| File | Action |
|------|--------|
| `server/internal/api/mock_querier_test.go`       | MODIFY (add new query methods) |
| `server/internal/api/group_handler_test.go`       | NEW (TDD) |
| `server/internal/api/activity_handler_test.go`    | NEW (TDD) |
| `server/internal/api/auth_handler_test.go`        | MODIFY (multi-group login, group switch) |
| `server/internal/api/user_handler_test.go`        | MODIFY (account_type support) |
| `server/internal/api/provider_handler_test.go`    | MODIFY (group_id scoping) |
| `server/internal/api/routing_rule_handler_test.go`| MODIFY (group_id scoping) |
| `server/internal/api/account_handler_test.go`     | DELETE |
| `server/internal/api/tenant_handler_test.go`      | DELETE |
| `server/internal/auth/jwt_test.go`                | MODIFY (GroupID claims) |
| `server/internal/auth/middleware_test.go`          | MODIFY (group context) |
| `server/internal/auth/rbac_test.go`               | MODIFY (system admin checks) |
| `server/internal/auth/tenant_test.go`             | RENAME to group_test.go |
| `server/internal/smtp/session_test.go`            | MODIFY (userID + groupID) |
| `server/internal/worker/handler_test.go`          | MODIFY (group-based resolution) |

**Verification Gate:**

```bash
docker run --rm -w /app -v $(pwd)/server:/app golang:1.24-alpine sh -c "go test ./..."
# All tests must pass
```

---

## 3. File Impact Summary

### New Files (8)

| File | Purpose |
|------|---------|
| `server/migrations/010_unified_auth.up.sql`             | Schema migration |
| `server/migrations/010_unified_auth.down.sql`           | Rollback migration |
| `server/internal/storage/queries/groups.sql`             | Group queries |
| `server/internal/storage/queries/group_members.sql`      | Membership queries |
| `server/internal/storage/queries/activity_logs.sql`      | Activity log queries |
| `server/internal/api/group_handler.go`                   | Group CRUD + members |
| `server/internal/api/activity_handler.go`                | Activity log endpoints |
| `server/internal/api/group_handler_test.go`              | Group handler tests |
| `server/internal/api/activity_handler_test.go`           | Activity handler tests |

### Modified Files (21)

| File | Change Summary |
|------|----------------|
| `server/internal/storage/queries/users.sql`              | Add account_type, username, api_key columns |
| `server/internal/storage/queries/providers.sql`          | account_id -> group_id |
| `server/internal/storage/queries/routing_rules.sql`      | account_id -> group_id |
| `server/internal/storage/queries/messages.sql`           | account_id -> user_id + group_id |
| `server/internal/storage/queries/delivery_logs.sql`      | account_id -> user_id, tenant_id -> group_id |
| `server/internal/storage/queries/sessions.sql`           | tenant_id -> group_id |
| `server/internal/storage/models.go`                      | REGENERATE via sqlc |
| `server/internal/storage/*.sql.go`                       | REGENERATE via sqlc |
| `server/internal/auth/jwt.go`                            | TenantID -> GroupID |
| `server/internal/auth/middleware.go`                     | Context helpers rename |
| `server/internal/auth/rbac.go`                           | System admin checks |
| `server/internal/auth/tenant.go` -> `group.go`          | RENAME + update |
| `server/internal/auth/audit.go`                          | Adapt to activity_logs |
| `server/internal/smtp/session.go`                        | accountID -> userID + groupID |
| `server/internal/api/auth_handler.go`                    | Multi-group login, switch |
| `server/internal/api/user_handler.go`                    | account_type support |
| `server/internal/api/provider_handler.go`                | account_id -> group_id |
| `server/internal/api/routing_rule_handler.go`            | account_id -> group_id |
| `server/internal/api/router.go`                          | Route consolidation |
| `server/internal/delivery/service.go`                    | Request struct update |
| `server/internal/worker/handler.go`                      | Group-based resolution |
| `server/cmd/api-server/main.go`                          | Bootstrap seed |
| `docker-compose.yml`                                     | Seed service update |

### Deleted Files (10)

| File | Reason |
|------|--------|
| `server/internal/storage/queries/accounts.sql`     | Merged into users |
| `server/internal/storage/queries/tenants.sql`      | Replaced by groups.sql |
| `server/internal/storage/queries/audit_logs.sql`   | Replaced by activity_logs.sql |
| `server/internal/storage/accounts.sql.go`          | Regenerated away |
| `server/internal/storage/tenants.sql.go`           | Regenerated away |
| `server/internal/storage/audit_logs.sql.go`        | Regenerated away |
| `server/internal/api/account_handler.go`           | Merged into user_handler |
| `server/internal/api/tenant_handler.go`            | Replaced by group_handler |
| `server/internal/api/account_handler_test.go`      | Removed with handler |
| `server/internal/api/tenant_handler_test.go`       | Removed with handler |

---

## 4. Parallelization Opportunities

| Parallel Group | Tasks | Dependency |
|----------------|-------|------------|
| Group A        | Phase 1 (Migration SQL) | None (must be first) |
| Group B        | Phase 2 (sqlc queries)  | Phase 1 complete |
| Group C        | Phase 3 (Auth) + Phase 4 (SMTP) | Phase 2 complete (can run in parallel) |
| Group D        | Phase 5 (Handlers) + Phase 7 (Worker) | Phase 3 complete |
| Group E        | Phase 6 (Router) | Phase 5 complete |
| Group F        | Phase 8 (Seed + Docker) | Phase 6 complete |
| Group G        | Phase 9 (Tests) | All phases complete |

Phases 3 and 4 can be developed in parallel since they touch independent packages (`auth/` vs `smtp/`). Phase 5 handlers can be developed in parallel per-handler.

---

## 5. Risk Mitigation Plan

| Risk | Mitigation Action | Phase |
|------|-------------------|-------|
| R-1: Migration integrity | Test up/down/up cycle before proceeding | Phase 1 |
| R-2: SMTP compatibility  | Verify same username + password_hash after migration | Phase 1, 4 |
| R-3: sqlc breakage       | Run sqlc generate immediately after query changes | Phase 2 |
| R-4: RLS leakage         | Write integration tests for cross-group queries | Phase 9 |
| R-5: Docker-only build   | All verification gates use Docker commands | All |
| R-6: Test fixtures       | Update mock_querier_test.go as first step of Phase 9 | Phase 9 |
| R-7: Token invalidation  | Document that existing sessions expire post-migration | Phase 8 |

---

## 6. Definition of Done

- [ ] Migration 010 applies cleanly (up/down/up cycle passes)
- [ ] `sqlc generate` produces no errors
- [ ] `go build ./...` compiles without errors
- [ ] `go test ./...` all tests pass
- [ ] SMTP auth works with migrated credentials
- [ ] Web login works with multi-group support
- [ ] Group switching returns new JWT with correct role
- [ ] API key auth resolves user + group context
- [ ] System admin seeded on first startup
- [ ] Activity logs recorded for CRUD and auth events
- [ ] RLS policies enforce group isolation
- [ ] Permission matrix enforced per spec
- [ ] No cross-group data leakage in integration tests
- [ ] Docker compose boots cleanly with admin seeded
