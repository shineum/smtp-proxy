---
id: SPEC-AUTH-001
type: acceptance
version: "1.0.0"
tags: [auth, groups, users, smtp, migration, rbac]
---

# Acceptance Criteria: SPEC-AUTH-001 Unified Account System Redesign

## 1. User Story Acceptance Criteria

### US-1: System Bootstrap

**AC-1.1: Auto-seed on first startup**

```gherkin
Given the database has no user with system group membership
When the API server starts
Then a group with name "system" and group_type "system" is created
And an admin user with email from SMTP_PROXY_ADMIN_EMAIL (default: admin@localhost) is created
And the admin user is added to the system group with role "owner"
And the admin credentials are logged to stdout
```

**AC-1.2: Skip seed on subsequent startups**

```gherkin
Given the database already has a user with system group membership
When the API server starts
Then no new group or user is created
And no credentials are logged to stdout
```

**AC-1.3: Password from environment**

```gherkin
Given SMTP_PROXY_ADMIN_PASSWORD is set to "SecurePass123"
When the system bootstrap runs for the first time
Then the admin user is created with password "SecurePass123"
And the password hash is stored via bcrypt
```

---

### US-2: Group Management by System Admins

**AC-2.1: Create company group**

```gherkin
Given a system admin is authenticated with a valid JWT
When POST /api/v1/groups is called with {"name": "Company A"}
Then a group with group_type "company" and status "active" is created
And HTTP 201 is returned with the group details
And an activity_log entry with action "create" and resource_type "group" is recorded
```

**AC-2.2: Delete company group**

```gherkin
Given a system admin is authenticated
And a group "Company A" exists with status "active"
When DELETE /api/v1/groups/{id} is called
Then the group status is set to "deleted"
And all SMTP accounts belonging exclusively to that group are suspended
And HTTP 200 is returned
And an activity_log entry with action "delete" is recorded
```

**AC-2.3: Prevent system group deletion**

```gherkin
Given a system admin is authenticated
When DELETE /api/v1/groups/{system_group_id} is called
Then HTTP 403 Forbidden is returned with error "cannot delete system group"
And the system group remains unchanged
```

---

### US-3: Group Member Management

**AC-3.1: Add member to group**

```gherkin
Given a group admin is authenticated for group "Company A"
And user "user@example.com" exists
When POST /api/v1/groups/{id}/members is called with {"user_id": "<user_id>", "role": "member"}
Then a group_members record is created
And HTTP 201 is returned
And an activity_log entry with action "create" and resource_type "group_member" is recorded
```

**AC-3.2: Remove member from group**

```gherkin
Given a group admin is authenticated for group "Company A"
And user "user@example.com" is a member of group "Company A"
When DELETE /api/v1/groups/{id}/members/{uid} is called
Then the group_members record is deleted
And HTTP 200 is returned
And an activity_log entry with action "delete" and resource_type "group_member" is recorded
```

**AC-3.3: Prevent last owner removal**

```gherkin
Given group "Company A" has exactly one owner "owner@example.com"
And the owner is authenticated
When DELETE /api/v1/groups/{id}/members/{owner_uid} is called
Then HTTP 409 Conflict is returned with error "cannot remove last owner"
And the membership remains unchanged
```

**AC-3.4: Update member role**

```gherkin
Given a group owner is authenticated for group "Company A"
And user "user@example.com" is a member with role "member"
When PATCH /api/v1/groups/{id}/members/{uid} is called with {"role": "admin"}
Then the member's role is updated to "admin"
And HTTP 200 is returned
And an activity_log entry with action "update" and resource_type "group_member" is recorded
```

---

### US-4: Unified User Creation

**AC-4.1: Create human user**

```gherkin
Given an admin is authenticated for group "Company A"
When POST /api/v1/users is called with {"email": "new@example.com", "password": "pass123", "account_type": "human"}
Then a user with account_type "human" is created
And the user is added to group "Company A" with role "member" via group_members
And HTTP 201 is returned with user details (excluding password_hash)
```

**AC-4.2: Create SMTP account**

```gherkin
Given an admin is authenticated for group "Company A"
When POST /api/v1/users is called with {"username": "smtp-notifications", "password": "smtppass", "account_type": "smtp"}
Then a user is created with email "smtp-notifications@smtp.internal"
And the user has account_type "smtp" and an auto-generated api_key
And the user is added to group "Company A" via group_members
And HTTP 201 is returned with user details including the api_key
```

**AC-4.3: Reject SMTP account creation with duplicate username**

```gherkin
Given an SMTP account with username "smtp-notifications" already exists
When POST /api/v1/users is called with {"username": "smtp-notifications", "account_type": "smtp"}
Then HTTP 409 Conflict is returned with error "username already exists"
```

---

### US-5: SMTP Authentication Flow

**AC-5.1: Successful SMTP auth**

```gherkin
Given an SMTP account "smtp-notifications" exists with status "active" in group "Company A"
And group "Company A" has status "active"
When an SMTP client connects and sends AUTH PLAIN with username "smtp-notifications" and valid password
Then the SMTP session is authenticated
And the session context contains userID and groupID
And the SMTP server responds with 235 Authentication successful
```

**AC-5.2: SMTP auth with suspended group**

```gherkin
Given an SMTP account "smtp-notifications" exists in group "Company A"
And group "Company A" has status "suspended"
When an SMTP client sends AUTH PLAIN with valid credentials
Then the SMTP server responds with 535 Authentication failed
And the session is not authenticated
```

**AC-5.3: SMTP auth preserves domain validation**

```gherkin
Given an SMTP account "smtp-notifications" has allowed_domains ["example.com"]
And the account is authenticated
When MAIL FROM: <user@other.com> is sent
Then the SMTP server responds with 550 Sender domain not allowed
```

---

### US-6: Web/API Authentication

**AC-6.1: Login with single group**

```gherkin
Given user "admin@example.com" exists with account_type "human" and status "active"
And the user belongs to exactly one group "Company A" with role "admin"
When POST /api/v1/auth/login is called with {"email": "admin@example.com", "password": "validpass"}
Then HTTP 200 is returned with access_token, refresh_token, token_type "Bearer", and expires_in
And the JWT access_token claims contain group_id matching "Company A" and role "admin"
```

**AC-6.2: Login with multiple groups and explicit group_id**

```gherkin
Given user "multi@example.com" belongs to "Company A" (role: admin) and "Company B" (role: member)
When POST /api/v1/auth/login is called with {"email": "multi@example.com", "password": "validpass", "group_id": "<company_b_id>"}
Then the JWT access_token claims contain group_id matching "Company B" and role "member"
```

**AC-6.3: Login with multiple groups and no group_id**

```gherkin
Given user "multi@example.com" belongs to "Company A" (created first) and "Company B"
When POST /api/v1/auth/login is called with {"email": "multi@example.com", "password": "validpass"}
Then the JWT access_token claims contain group_id matching "Company A" (first membership by created_at)
```

**AC-6.4: Reject SMTP account web login**

```gherkin
Given an SMTP account with email "smtp-notifications@smtp.internal" exists
When POST /api/v1/auth/login is called with {"email": "smtp-notifications@smtp.internal", "password": "pass"}
Then HTTP 401 Unauthorized is returned
```

---

### US-7: Group Switching

**AC-7.1: Switch to another group**

```gherkin
Given user "multi@example.com" is authenticated with group "Company A"
And the user also belongs to group "Company B" with role "member"
When POST /api/v1/auth/switch-group is called with {"group_id": "<company_b_id>"}
Then HTTP 200 is returned with new access_token and refresh_token
And the new JWT claims contain group_id matching "Company B" and role "member"
```

**AC-7.2: Reject switch to non-member group**

```gherkin
Given user "admin@example.com" is authenticated
And the user does not belong to group "Company C"
When POST /api/v1/auth/switch-group is called with {"group_id": "<company_c_id>"}
Then HTTP 403 Forbidden is returned with error "not a member of this group"
```

**AC-7.3: Old tokens remain valid after switch**

```gherkin
Given user "multi@example.com" switches from "Company A" to "Company B"
When a request is made using the old access_token (Company A context)
Then the request succeeds with Company A context until the token expires naturally
```

---

### US-8: API Key Authentication

**AC-8.1: Authenticate with API key**

```gherkin
Given an SMTP account "smtp-notifications" exists with a valid api_key
And the account belongs to group "Company A" with role "member"
When a request is made with header "Authorization: Bearer <api_key>"
Then the request is authenticated
And the context contains userID, groupID (Company A), and role "member"
And RLS is set with app.current_group_id = Company A's ID
```

**AC-8.2: Reject invalid API key**

```gherkin
Given no user exists with api_key "invalid-key-12345"
When a request is made with header "Authorization: Bearer invalid-key-12345"
Then HTTP 401 Unauthorized is returned
```

---

### US-9: Activity Logging

**AC-9.1: CRUD operations logged**

```gherkin
Given an admin creates a new user in group "Company A"
When the user creation succeeds
Then an activity_logs record is created with:
  | field         | value               |
  | group_id      | Company A's ID      |
  | actor_id      | admin's user ID     |
  | action        | create              |
  | resource_type | user                |
  | resource_id   | new user's ID       |
  | changes       | {"username": "...", "account_type": "..."} |
  | ip_address    | request IP          |
```

**AC-9.2: Query activity logs with filters**

```gherkin
Given multiple activity log entries exist for group "Company A"
When GET /api/v1/activity?resource_type=user&resource_id={user_id} is called
Then only activity logs matching the filter criteria are returned
And results are ordered by created_at DESC
```

**AC-9.3: Auth events logged**

```gherkin
Given a user attempts to log in
When the login succeeds
Then an activity_log entry with action "login" is created
When the login fails
Then an activity_log entry with action "login_failed" is created
```

---

### US-10: Permission Enforcement

**AC-10.1: System admin can manage groups**

```gherkin
Given a user with role "admin" in the system group is authenticated
When POST /api/v1/groups is called with {"name": "New Company"}
Then HTTP 201 is returned and the group is created
```

**AC-10.2: Company admin cannot manage groups**

```gherkin
Given a user with role "admin" in a company group (not system) is authenticated
When POST /api/v1/groups is called
Then HTTP 403 Forbidden is returned
```

**AC-10.3: Company member read-only access**

```gherkin
Given a user with role "member" in group "Company A" is authenticated
When GET /api/v1/providers is called
Then HTTP 200 is returned with providers scoped to Company A
When POST /api/v1/providers is called
Then HTTP 403 Forbidden is returned
```

---

## 2. Edge Case Test Scenarios

### EC-1: Multi-group role divergence

```gherkin
Given user "multi@example.com" is admin in "Company A" and member in "Company B"
When the user logs in with group_id = Company B
Then the JWT role claim is "member" (not "admin")
And the user cannot perform admin actions in Company B
```

### EC-2: SMTP account single-group enforcement

```gherkin
Given SMTP account "smtp-alerts" belongs to "Company A"
When POST /api/v1/groups/{company_b_id}/members is called with {"user_id": "<smtp_alerts_id>", "role": "member"}
Then HTTP 409 Conflict is returned with error "SMTP accounts can only belong to one group"
```

### EC-3: System group deletion prevention

```gherkin
Given the system group exists
When DELETE /api/v1/groups/{system_group_id} is called by any user
Then HTTP 403 Forbidden is returned
And the system group status remains "active"
```

### EC-4: Admin password handling

```gherkin
Given SMTP_PROXY_ADMIN_PASSWORD is not set
When the system bootstraps for the first time
Then a random password is generated
And the password is logged to stdout exactly once
And the password hash is stored via bcrypt
```

### EC-5: Last owner protection

```gherkin
Given group "Company A" has one owner and two admins
When PATCH /api/v1/groups/{id}/members/{owner_uid} is called with {"role": "member"}
Then HTTP 409 Conflict is returned with error "cannot remove last owner"
And the owner's role remains "owner"
```

### EC-6: Concurrent group switching

```gherkin
Given user "multi@example.com" has a valid JWT for "Company A"
When the user switches to "Company B" and receives a new JWT
Then the old JWT for "Company A" continues to work until its expiry
And the new JWT contains "Company B" context
```

### EC-7: Migration data integrity

```gherkin
Given the database has existing accounts, users, tenants, and related records
When migration 010_unified_auth.up.sql is applied
Then all accounts are migrated to users with account_type "smtp"
And all users retain their original password_hash values
And all tenant-user relationships are migrated to group_members
And row counts for migrated data match the originals
When migration 010_unified_auth.down.sql is applied
Then the schema reverts to pre-migration state
When migration 010_unified_auth.up.sql is reapplied
Then the migration succeeds without errors
```

### EC-8: API key collision prevention

```gherkin
Given the system generates an API key that collides with an existing key
When the UNIQUE constraint violation occurs
Then the system retries with a new key (up to 3 retries)
And the user is created successfully with a unique api_key
```

### EC-9: Suspended group behavior

```gherkin
Given group "Company A" has status "suspended"
When an SMTP client authenticates with credentials from "Company A"
Then SMTP 535 Authentication Failed is returned
When a human user from "Company A" attempts web login
Then HTTP 403 Forbidden is returned with error "group suspended"
```

### EC-10: Orphaned SMTP accounts on group deletion

```gherkin
Given group "Company A" is deleted (status set to "deleted")
And SMTP accounts "smtp-a" and "smtp-b" belong exclusively to "Company A"
Then both SMTP accounts have their status set to "suspended"
And an activity_log entry is recorded for each suspension
```

### EC-11: RLS policy migration

```gherkin
Given the system uses RLS with app.current_tenant_id before migration
When migration 010 is applied
Then all RLS policies reference app.current_group_id
And queries with GroupContext middleware correctly isolate data by group
And no cross-group data leakage occurs
```

### EC-12: Dual auth route consolidation

```gherkin
Given the system has legacy /api/v1/ routes with BearerAuth(accountLookup) middleware
When the router is consolidated
Then all routes use unified JWT + API key auth middleware
And /api/v1/accounts endpoints are removed
And /api/v1/tenants endpoints are removed
And /api/v1/webhooks/* remains accessible without auth
```

### EC-13: Session table migration

```gherkin
Given sessions exist with tenant_id references
When migration 010 renames tenant_id to group_id in sessions
Then existing sessions become invalid (users must re-login)
And new sessions store group_id correctly
And session-based operations (refresh, logout) work with group_id
```

---

## 3. Integration Test Scenarios

### IT-1: End-to-end SMTP flow post-migration

```gherkin
Given the system is bootstrapped with admin in system group
And admin creates group "TestCo" and SMTP account "smtp-test" in "TestCo"
When an SMTP client connects with username "smtp-test" and valid password
And sends MAIL FROM: <sender@allowed-domain.com>
And sends RCPT TO: <recipient@example.com>
And sends DATA with valid email body
Then the message is enqueued with user_id (smtp-test) and group_id (TestCo)
And the delivery worker resolves provider via group_id (TestCo)
And a delivery_log record is created with user_id and group_id
```

### IT-2: Full auth lifecycle

```gherkin
Given admin@localhost is seeded in the system group
When admin logs in and creates group "Company A"
And creates user "user@example.com" in "Company A"
And user@example.com logs in with group_id = Company A
And user@example.com views providers (GET /api/v1/providers)
Then providers scoped to Company A are returned
When admin@localhost adds user@example.com to a second group "Company B"
And user@example.com switches group to "Company B"
Then new JWT contains Company B context
And providers scoped to Company B are returned
```

### IT-3: Docker compose full stack

```gherkin
Given docker compose up -d --build is run
When the API server starts
Then the system group and admin user are seeded
And login with admin credentials returns a valid JWT
And creating a group, user, and SMTP account all succeed
And sending email via SMTP with the new account succeeds
And activity logs reflect all operations performed
```

---

## 4. Performance Criteria

| Metric | Target | Measurement Method |
|--------|--------|--------------------|
| SMTP auth latency | < current + 10ms | Benchmark test comparing before/after |
| JWT generation     | < 5ms            | Unit test with timer |
| API key lookup     | Single indexed query | EXPLAIN ANALYZE on GetUserByAPIKey |
| Activity log write | Non-blocking (async or within 50ms) | Handler response time measurement |
| Migration execution | < 60s for datasets up to 10K accounts | Timed migration run |
| Group switching    | < 100ms total     | End-to-end handler timing |

---

## 5. Quality Gate Criteria (TRUST 5)

### Tested

- [ ] All new code has corresponding tests (TDD for new handlers)
- [ ] All modified code has updated tests (DDD for existing handlers)
- [ ] Test coverage >= 85% for affected packages
- [ ] Migration up/down/up cycle passes
- [ ] Integration tests verify end-to-end flows
- [ ] No cross-group data leakage in RLS tests

### Readable

- [ ] All new functions have Go doc comments
- [ ] Code comments in English (per language.yaml)
- [ ] Clear error messages for all HTTP error responses
- [ ] Consistent naming: group_id, user_id (no mixed tenant_id/account_id)

### Unified

- [ ] ruff/golangci-lint passes with zero errors
- [ ] Consistent JSON response format across all endpoints
- [ ] Consistent error response format: `{"error": "message"}`
- [ ] sqlc generated code is formatted

### Secured

- [ ] Passwords stored as bcrypt hashes only
- [ ] API keys generated with crypto/rand
- [ ] RLS policies enforce group isolation
- [ ] SMTP accounts cannot log in via web API
- [ ] Rate limiting preserved for login attempts
- [ ] Permission matrix enforced for all endpoints

### Trackable

- [ ] Conventional commit messages for all changes
- [ ] Activity logs record all CRUD and auth events
- [ ] Migration file includes clear step comments
- [ ] SPEC-AUTH-001 tag referenced in commits

---

## 6. Definition of Done

All of the following must be true:

1. Migration 010 applies cleanly with up/down/up cycle
2. `sqlc generate` succeeds with no errors
3. `go build ./...` compiles without errors
4. `go test ./...` passes with all tests green
5. All acceptance criteria in this document pass
6. All 13 edge case scenarios handled
7. Integration tests verify end-to-end SMTP and web auth flows
8. Performance targets met
9. TRUST 5 quality gates satisfied
10. Docker compose boots with admin seeded and functional
