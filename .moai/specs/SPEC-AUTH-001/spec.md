---
id: SPEC-AUTH-001
version: "1.0.0"
status: approved
created: "2026-02-28"
updated: "2026-02-28"
author: sungwon
priority: high
tags: [auth, groups, users, smtp, migration, rbac]
---

## HISTORY

| Version | Date       | Author  | Description                          |
|---------|------------|---------|--------------------------------------|
| 1.0.0   | 2026-02-28 | sungwon | Initial SPEC from research + planning |

---

# SPEC-AUTH-001: Unified Account System Redesign

## 1. Overview

### 1.1 Problem Statement

The smtp-proxy project currently implements two disconnected authentication systems:

1. **Legacy accounts system** (`server/internal/storage/accounts.sql.go`): SMTP auth + API key auth, no roles, no status field, no group context.
2. **Multi-tenant users system** (`server/internal/storage/users.sql.go`): JWT-based web/API auth, role-based access, tenant isolation via RLS, but not connected to SMTP.

These two systems share the same PostgreSQL database but operate independently with separate auth flows, context variables (accountID vs userID vs tenantID), API route trees, and authorization models. This bifurcation creates:

- Inconsistent scoping: messages reference accountID while quota tracking uses tenantID
- Duplicate identity management: admins must manage accounts and users separately
- No group-level SMTP account management: SMTP accounts have no group association
- No multi-group membership: users are locked to a single tenant

### 1.2 Goal

Merge accounts and users into a single `users` table, replace tenants with `groups` (supporting system vs company types), introduce a `group_members` many-to-many table for multi-group membership with per-group roles, and consolidate all API routes under a unified JWT + API key auth model. Replace `audit_logs` with a richer `activity_logs` table. Auto-seed a system admin on first startup.

### 1.3 Scope

- Database schema migration (single file: `010_unified_auth.up.sql`)
- sqlc query regeneration for all affected tables
- Auth layer refactoring (JWT claims, RBAC, RLS, middleware)
- SMTP session refactoring (accountID to userID + groupID)
- API handler and router consolidation
- System bootstrap seed logic
- docker-compose updates
- Test updates for all affected modules

### 1.4 Out of Scope

- Frontend/UI changes (web dashboard is a separate SPEC)
- OAuth/SSO integration
- Email quota enforcement policy changes beyond FK renaming
- New monitoring/stats endpoints (future SPEC)

---

## 2. Data Model

### 2.1 Groups Table (renamed from tenants)

```sql
groups
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid()
  name          VARCHAR(255) UNIQUE NOT NULL
  group_type    VARCHAR(20) NOT NULL DEFAULT 'company'  -- 'system' | 'company'
  status        VARCHAR(20) NOT NULL DEFAULT 'active'   -- 'active' | 'suspended' | 'deleted'
  monthly_limit INT NOT NULL DEFAULT 0                  -- 0 = unlimited
  monthly_sent  INT NOT NULL DEFAULT 0
  allowed_ips   CIDR[]
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
```

Constraints:
- Exactly one group with `group_type = 'system'` must exist at all times
- The system group cannot be deleted or suspended
- `name` is case-sensitive and unique across all groups

### 2.2 Users Table (unified)

```sql
users
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
  email           VARCHAR(255) UNIQUE NOT NULL
  username        VARCHAR(255) UNIQUE           -- SMTP AUTH username (NULL for human)
  password_hash   VARCHAR(255) NOT NULL
  account_type    VARCHAR(20) NOT NULL DEFAULT 'human'  -- 'human' | 'smtp'
  api_key         VARCHAR(255) UNIQUE           -- auto-generated for smtp, optional for human
  allowed_domains JSONB                         -- smtp: sender domain restrictions
  status          VARCHAR(20) NOT NULL DEFAULT 'active'  -- 'active' | 'suspended' | 'deleted'
  failed_attempts INT NOT NULL DEFAULT 0
  last_login      TIMESTAMPTZ
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
```

Constraints:
- `email` uniqueness: SMTP accounts use synthetic emails `{username}@smtp.internal`
- `username` is non-null only for `account_type = 'smtp'`
- `api_key` is auto-generated on creation for SMTP accounts
- Human users may optionally have an api_key
- Columns dropped from old schema: `tenant_id`, `role` (moved to group_members)

### 2.3 Group Members Table (new, many-to-many)

```sql
group_members
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid()
  group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
  role       VARCHAR(20) NOT NULL DEFAULT 'member'  -- 'owner' | 'admin' | 'member'
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  UNIQUE(group_id, user_id)
```

Constraints:
- SMTP accounts belong to exactly one group (enforced at application level)
- Each group must have at least one owner (enforced at application level)
- Role is per-group: same user can be admin in one group, member in another

### 2.4 Activity Logs Table (replaces audit_logs)

```sql
activity_logs
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid()
  group_id      UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE
  actor_id      UUID REFERENCES users(id) ON DELETE SET NULL
  action        VARCHAR(50) NOT NULL
  resource_type VARCHAR(50) NOT NULL
  resource_id   UUID
  changes       JSONB                -- before/after diff
  comment       TEXT                 -- optional human note
  ip_address    INET
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
```

Indexes:
- `(group_id, created_at DESC)` -- activity within a group
- `(resource_type, resource_id)` -- history of a specific resource
- `(actor_id, created_at DESC)` -- actions by a specific user

Actions tracked: `create`, `update`, `delete`, `suspend`, `activate`, `login`, `login_failed`, `password_change`

Resource types: `group`, `user`, `esp_provider`, `routing_rule`, `group_member`

### 2.5 FK Changes on Dependent Tables

| Table          | Column Before   | Column After              |
|----------------|-----------------|---------------------------|
| esp_providers  | account_id (FK) | group_id (FK -> groups)   |
| routing_rules  | account_id (FK) | group_id (FK -> groups)   |
| messages       | account_id (FK) | user_id (FK) + group_id (FK) |
| delivery_logs  | account_id (FK) | user_id (FK) + group_id (FK) |
| sessions       | tenant_id (FK)  | group_id (FK -> groups)   |

---

## 3. Requirements

### Module 1: System Bootstrap (REQ-AUTH-001 through REQ-AUTH-003)

**REQ-AUTH-001** (Event-Driven): When the API server starts and no user with system group membership exists, the system shall create a `system` group (group_type: system), an admin user (email from `SMTP_PROXY_ADMIN_EMAIL` env, default: `admin@localhost`), and add the admin as `owner` of the system group.

**REQ-AUTH-002** (State-Driven): While the environment variable `SMTP_PROXY_ADMIN_PASSWORD` is not set, the system shall generate a random password and log it to stdout on first creation. Where `SMTP_PROXY_ADMIN_PASSWORD` is set, the system shall use that value as the admin password.

**REQ-AUTH-003** (Ubiquitous): The system shall ensure exactly one group with `group_type = 'system'` exists at all times. The system group shall not be deletable or suspendable.

### Module 2: Group & Member Management (REQ-AUTH-004 through REQ-AUTH-009)

**REQ-AUTH-004** (Event-Driven): When a system admin sends POST /api/v1/groups with a valid name, the system shall create a new group with `group_type = 'company'` and `status = 'active'`.

**REQ-AUTH-005** (Event-Driven): When a system admin sends DELETE /api/v1/groups/{id}, the system shall soft-delete the group by setting `status = 'deleted'`. The system shall auto-suspend all SMTP accounts that belong exclusively to the deleted group.

**REQ-AUTH-006** (Unwanted): If a request attempts to delete or suspend the system group, then the system shall return HTTP 403 Forbidden.

**REQ-AUTH-007** (Event-Driven): When a group admin sends POST /api/v1/groups/{id}/members with a valid user_id and role, the system shall create a group_members record. Where the user is an SMTP account already belonging to another group, the system shall return HTTP 409 Conflict.

**REQ-AUTH-008** (Event-Driven): When a group admin sends DELETE /api/v1/groups/{id}/members/{uid}, the system shall remove the membership. If the target is the last owner of the group, the system shall return HTTP 409 Conflict with error "cannot remove last owner".

**REQ-AUTH-009** (Event-Driven): When a group admin sends PATCH /api/v1/groups/{id}/members/{uid} with a new role, the system shall update the member's role. If the change would leave the group with no owners, the system shall return HTTP 409 Conflict.

### Module 3: User Management (REQ-AUTH-010 through REQ-AUTH-014)

**REQ-AUTH-010** (Event-Driven): When an authorized user sends POST /api/v1/users with `account_type = 'human'`, the system shall create a user with the provided email and password, and add them to the caller's active group via group_members.

**REQ-AUTH-011** (Event-Driven): When an authorized user sends POST /api/v1/users with `account_type = 'smtp'`, the system shall create a user with a synthetic email (`{username}@smtp.internal`), the provided username and password, auto-generate an API key, and add them to the caller's active group via group_members.

**REQ-AUTH-012** (Ubiquitous): The system shall enforce that SMTP accounts (`account_type = 'smtp'`) belong to exactly one group. Any attempt to add an SMTP account to a second group shall be rejected with HTTP 409 Conflict.

**REQ-AUTH-013** (Event-Driven): When an authorized user sends PATCH /api/v1/users/{id}, the system shall update the user's mutable fields (email, allowed_domains, status). The system shall not allow changing `account_type`.

**REQ-AUTH-014** (Event-Driven): When an authorized user sends DELETE /api/v1/users/{id}, the system shall soft-delete the user by setting `status = 'deleted'` and removing all group memberships.

### Module 4: Authentication Flows (REQ-AUTH-015 through REQ-AUTH-022)

**REQ-AUTH-015** (Event-Driven): When a client sends SASL PLAIN credentials via SMTP, the system shall look up the user by username where `account_type = 'smtp'` and `status = 'active'`, verify the bcrypt password hash, resolve the user's group_id from group_members, and establish an SMTP session with userID + groupID context.

**REQ-AUTH-016** (Event-Driven): When a client sends POST /api/v1/auth/login with email and password, the system shall authenticate the user where `account_type = 'human'`, verify the password, and return JWT access + refresh tokens. The JWT claims shall include `sub` (userID), `group_id`, `email`, and `role` (from group_members for the active group).

**REQ-AUTH-017** (State-Driven): While a user belongs to multiple groups, when the login request includes an optional `group_id` parameter, the system shall use that group as the active context. Where no `group_id` is provided, the system shall default to the first group membership (ordered by created_at).

**REQ-AUTH-018** (Event-Driven): When an authenticated user sends POST /api/v1/auth/switch-group with a valid group_id, the system shall verify membership, create a new session, and return new JWT tokens with the switched group context. Previously issued tokens remain valid until their natural expiry.

**REQ-AUTH-019** (Event-Driven): When a client sends an API request with `Authorization: Bearer <api_key>`, the system shall look up the user by api_key, resolve the user's group_id from group_members, and set the full context (userID + groupID + role).

**REQ-AUTH-020** (State-Driven): While a group's status is `suspended`, when an SMTP client attempts authentication, the system shall return SMTP 535 Authentication Failed. When a human user attempts web login, the system shall return HTTP 403 with error "group suspended".

**REQ-AUTH-021** (Event-Driven): When a client sends POST /api/v1/auth/refresh with a valid refresh token, the system shall validate the session, generate new access + refresh tokens, and return them. The group_id in the new tokens shall match the session's group_id.

**REQ-AUTH-022** (Event-Driven): When a client sends POST /api/v1/auth/logout, the system shall delete the session record from the database.

### Module 5: Authorization & Infrastructure (REQ-AUTH-023 through REQ-AUTH-030)

**REQ-AUTH-023** (Ubiquitous): The system shall enforce the following permission matrix:

| Action                  | sys.owner | sys.admin | co.owner | co.admin | co.member |
|-------------------------|-----------|-----------|----------|----------|-----------|
| Create/delete groups    | YES       | YES       | -        | -        | -         |
| Manage company admins   | YES       | YES       | YES      | -        | -         |
| Manage company users    | YES       | YES       | YES      | YES      | -         |
| Manage SMTP accounts    | YES       | YES       | YES      | YES      | -         |
| Manage ESP providers    | YES       | YES       | YES      | YES      | -         |
| View usage/monitoring   | YES       | YES       | YES      | YES      | YES       |

**REQ-AUTH-024** (Ubiquitous): The system shall use PostgreSQL Row-Level Security with `app.current_group_id` session variable to enforce group isolation for all group-scoped queries.

**REQ-AUTH-025** (Event-Driven): When the migration 010_unified_auth runs, the system shall:
1. Rename `tenants` to `groups` and add `group_type` column
2. Add new columns to `users` (username, account_type, api_key, allowed_domains)
3. Create `group_members` table
4. Migrate existing users into group_members from their tenant_id
5. Migrate accounts into users as `account_type = 'smtp'` with synthetic emails
6. Update all FK references (account_id to group_id, tenant_id to group_id)
7. Drop accounts table and users.tenant_id column
8. Drop audit_logs, create activity_logs
9. Recreate RLS policies with group_id references

**REQ-AUTH-026** (Unwanted): The system shall not allow API key collisions. If a UNIQUE constraint violation occurs during API key generation, the system shall retry with a new key (maximum 3 retries).

**REQ-AUTH-027** (Event-Driven): When CRUD operations or auth events occur, the system shall create an activity_logs record with the actor_id, action, resource_type, resource_id, changes (JSONB diff), and ip_address.

**REQ-AUTH-028** (Event-Driven): When an authorized user sends GET /api/v1/activity with optional filters (resource_type, resource_id, actor_id, group_id), the system shall return paginated activity log entries.

**REQ-AUTH-029** (Ubiquitous): The system shall consolidate the dual API route trees (legacy account API key auth + JWT auth) into a single `/api/v1/` tree with unified JWT + API key authentication middleware.

**REQ-AUTH-030** (Ubiquitous): The system shall maintain backward compatibility for existing SMTP credentials, API keys, and webhook endpoints during and after migration.

---

## 4. Auth Flow Specifications

### 4.1 SMTP Authentication Flow

```
Client -> EHLO -> AUTH PLAIN (username, password)
  -> GetUserByUsername(username) WHERE account_type='smtp' AND status='active'
  -> bcrypt verify password
  -> GetGroupMemberByUserID(userID) -> resolve groupID
  -> Check group.status = 'active' (reject if suspended)
  -> Set session: userID + groupID + allowedDomains
  -> Provider lookup: ListProvidersByGroupID(groupID)
  -> Message enqueue: store userID + groupID
```

### 4.2 Web/API JWT Authentication Flow

```
POST /api/v1/auth/login { email, password, group_id? }
  -> Check rate limit by email
  -> GetUserByEmail(email) WHERE account_type='human'
  -> Verify user.status = 'active'
  -> bcrypt verify password
  -> Resolve group context:
     IF group_id provided: verify membership, use that group
     ELSE: use first group membership (ORDER BY created_at ASC LIMIT 1)
  -> Check group.status = 'active' (reject if suspended)
  -> Get role from group_members for (userID, groupID)
  -> Create session: userID + groupID
  -> Generate JWT: { sub: userID, group_id, email, role }
  -> Log activity: login event
  -> Return { access_token, refresh_token, token_type, expires_in }
```

### 4.3 API Key Authentication Flow

```
Authorization: Bearer <api_key>
  -> GetUserByAPIKey(api_key) WHERE status='active'
  -> GetGroupMemberByUserID(userID) -> resolve groupID + role
  -> For SMTP accounts (single group): direct resolution
  -> For human accounts (multi-group): use first group membership
  -> Set context: userID + groupID + role
  -> Set RLS: app.current_group_id = groupID
```

### 4.4 Group Switching Flow

```
POST /api/v1/auth/switch-group { group_id }
  -> Verify JWT (existing auth)
  -> Verify membership: group_members WHERE user_id AND group_id
  -> Verify group.status = 'active'
  -> Get role from group_members for new group
  -> Create new session: userID + new groupID
  -> Generate new JWT: { sub: userID, group_id: new, email, role: newRole }
  -> Log activity: group switch event
  -> Return new tokens
  -> Old tokens remain valid until natural expiry
```

---

## 5. Migration Strategy

### 5.1 Single Migration File

File: `server/migrations/010_unified_auth.up.sql`

Execution order (15 steps):

1. Rename `tenants` table to `groups`
2. Add `group_type` column to groups (default: 'company'), set existing rows to 'company'
3. Add new columns to `users`: username, account_type (default: 'human'), api_key, allowed_domains
4. Remove `role` and `tenant_id` columns from users (after migration step 6)
5. Create `group_members` table with UNIQUE(group_id, user_id)
6. Migrate existing users: INSERT INTO group_members (group_id, user_id, role) SELECT tenant_id, id, role FROM users
7. Migrate accounts to users: INSERT INTO users (email, username, password_hash, account_type, api_key, allowed_domains) SELECT '{name}@smtp.internal', name, password_hash, 'smtp', api_key, allowed_domains FROM accounts
8. Create group_members entries for migrated SMTP accounts (assign to their tenant or default group)
9. Update `esp_providers`: rename account_id to group_id, update FK
10. Update `routing_rules`: rename account_id to group_id, update FK
11. Update `messages`: add group_id column, rename account_id to user_id
12. Update `delivery_logs`: add group_id column, rename account_id to user_id
13. Update `sessions`: rename tenant_id to group_id
14. Drop `audit_logs` table, create `activity_logs` with new schema
15. Drop `accounts` table, drop `users.tenant_id` and `users.role`
16. Recreate all RLS policies with `app.current_group_id`

### 5.2 Down Migration

File: `server/migrations/010_unified_auth.down.sql`

Trade-off: Down migration is lossy for multi-group memberships. Users with multiple group memberships will be assigned to their first group only. SMTP accounts will be restored to the accounts table.

### 5.3 Email Collision Solution

SMTP accounts receive synthetic emails in the format `{username}@smtp.internal` to satisfy the users.email UNIQUE constraint without conflicting with real email addresses.

---

## 6. Non-Functional Requirements

### 6.1 Performance

- SMTP authentication latency must not increase by more than 10ms compared to current implementation
- JWT token generation and validation must remain under 5ms
- API key lookup must remain a single indexed query
- Activity log writes must be non-blocking to the main request flow

### 6.2 Security

- All passwords stored as bcrypt hashes (existing pattern preserved)
- API keys generated with cryptographically secure random bytes (existing `auth.GenerateAPIKey()`)
- RLS policies enforce group isolation at the database level
- JWT signing uses HS256 with configurable secret (existing pattern)
- SMTP accounts cannot log in via web/API (account_type enforcement)
- Rate limiting preserved for login attempts

### 6.3 Backward Compatibility

- Existing SMTP credentials (username + password) must continue to work after migration
- Existing API keys must continue to work after migration
- Webhook endpoints must remain functional
- No data loss during migration (accounts data preserved in users table)

### 6.4 Observability

- All CRUD operations and auth events logged to activity_logs
- Structured logging via zerolog preserved
- Correlation ID tracking preserved across request lifecycle

---

## 7. Risks and Mitigations

| ID   | Risk                              | Severity | Mitigation                                                   |
|------|-----------------------------------|----------|--------------------------------------------------------------|
| R-1  | Migration data integrity          | HIGH     | Test migration with up/down/up cycle; verify row counts      |
| R-2  | SMTP auth backward compatibility  | HIGH     | SMTP accounts migrated with same username + password_hash    |
| R-3  | sqlc regeneration breakage        | MEDIUM   | Run `sqlc generate` after query updates; verify go build     |
| R-4  | RLS policy consistency            | MEDIUM   | Test cross-group data leakage with integration tests         |
| R-5  | Docker-only build constraint      | LOW      | All build/test via Docker commands; no local Go needed       |
| R-6  | Test fixture updates              | MEDIUM   | Update mock_querier_test.go and all test fixtures atomically |
| R-7  | Token invalidation on migration   | LOW      | Existing JWTs become invalid; users re-login post-migration  |
