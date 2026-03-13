# E2E Test Cases

Automation script: [`e2e-test.sh`](../e2e-test.sh)

---

## Setup & Infrastructure

### TC-001: Clean Build & Service Startup

| Field | Value |
|-------|-------|
| **Precondition** | Docker and Docker Compose installed |
| **Steps** | 1. `docker compose down -v --remove-orphans` <br> 2. `docker compose up -d --build` <br> 3. Wait for API server healthcheck (`/healthz`) |
| **Expected** | All services start. Migration completes without error. API healthcheck returns 200 within 60s. |

### TC-002: Admin Login

| Field | Value |
|-------|-------|
| **Precondition** | TC-001 passed. System admin auto-seeded (`admin@localhost` / `admin`). |
| **Steps** | POST `/api/v1/auth/login` with `{"email":"admin@localhost","password":"admin"}` |
| **Expected** | 200 OK with `access_token`, `refresh_token`, `token_type: "Bearer"`, `expires_in: 900` |

---

## Data Setup

### TC-003: Create Provider (stdout, private)

| Field | Value |
|-------|-------|
| **Precondition** | TC-002 passed. |
| **Steps** | POST `/api/v1/providers` with `{"name":"test-stdout-private","provider_type":"stdout","enabled":true,"visibility":"private"}` |
| **Expected** | 201 Created. `visibility: "private"`, `group_id` = system group. |

### TC-004: Create Provider (stdout, global)

| Field | Value |
|-------|-------|
| **Precondition** | TC-002 passed. |
| **Steps** | POST `/api/v1/providers` with `{"name":"test-stdout-global","provider_type":"stdout","enabled":true,"visibility":"global"}` |
| **Expected** | 201 Created. `visibility: "global"`. Accessible to all groups. |

### TC-005: Create Provider (stdout, shared)

| Field | Value |
|-------|-------|
| **Precondition** | TC-002 passed. |
| **Steps** | POST `/api/v1/providers` with `{"name":"test-stdout-shared","provider_type":"stdout","enabled":true,"visibility":"shared"}` |
| **Expected** | 201 Created. `visibility: "shared"`. Not accessible to other groups until access is granted. |

### TC-006: Create Groups

| Field | Value |
|-------|-------|
| **Precondition** | TC-002 passed. |
| **Steps** | POST `/api/v1/groups` for Group A and Group B |
| **Expected** | 201 Created for both. Each has unique `id` (UUID), `group_type: "company"`. |

### TC-007: Switch Group Context

| Field | Value |
|-------|-------|
| **Precondition** | TC-006 passed. |
| **Steps** | POST `/api/v1/auth/switch-group` with `{"group_id":"<group_a_id>"}` |
| **Expected** | 200 OK. New `access_token` with group context. JWT claims include `group_id` and `role: "owner"`. |

### TC-008: Create Provider (group-scoped, private)

| Field | Value |
|-------|-------|
| **Precondition** | TC-007 passed. Group A context. |
| **Steps** | POST `/api/v1/providers` with `{"name":"test-group-provider","provider_type":"stdout","enabled":true,"visibility":"private"}` |
| **Expected** | 201 Created. `group_id` matches Group A. |

### TC-009: Create Human User

| Field | Value |
|-------|-------|
| **Precondition** | TC-007 passed. |
| **Steps** | POST `/api/v1/users` with `{"email":"testuser@example.com","password":"testpass123","account_type":"user","group_id":"<group_a_id>"}` |
| **Expected** | 201 Created. `account_type: "user"`, `status: "active"`. No `api_key`. |

### TC-010: Create SMTP Service Account

| Field | Value |
|-------|-------|
| **Precondition** | TC-007, TC-008 passed. |
| **Steps** | POST `/api/v1/groups/<group_a_id>/service-accounts` with `{"username":"e2e-smtp","provider_id":"<provider_3_id>"}` |
| **Expected** | 201 Created. `username: "e2e-smtp"` (lowercase), `account_type: "smtp"`. No API key generated (keys are created separately via TC-025). |

### TC-010b: Create Initial API Key for Service Account

| Field | Value |
|-------|-------|
| **Precondition** | TC-010 passed. |
| **Steps** | POST `/api/v1/groups/<group_a_id>/service-accounts/<sa_id>/api-keys` with `{"label":"default","api_key_expires_in":"30d"}` |
| **Expected** | 201 Created. `api_key` returned (plaintext, shown once), `key_prefix` (12 chars), `label: "default"`, `expires_at` set 30 days from now. |

---

## SMTP Send (Positive Cases)

### TC-011: SMTP Send - Simple Case

| Field | Value |
|-------|-------|
| **Precondition** | TC-010 passed. |
| **Steps** | SMTP AUTH PLAIN with `e2e-smtp@<group_id>` / `<api_key>`. Send: From, To, Subject, plain text body. |
| **Expected** | 250 OK. Auth log: `auth successful`. Message persisted and delivered via stdout provider. |

### TC-012: SMTP Send - Complex Case

| Field | Value |
|-------|-------|
| **Precondition** | TC-010 passed. Attachment files exist. |
| **Steps** | SMTP AUTH PLAIN. Send: From, To, CC, BCC, Subject, plain text body, HTML body, 2 attachments. |
| **Expected** | 3 RCPT TO accepted. `recipient_count: 3`. Delivered via stdout provider. BCC not in headers. |

---

## SMTP Auth Negative Cases

### TC-013: Human User Cannot SMTP Auth

| Field | Value |
|-------|-------|
| **Precondition** | TC-009 passed. Human user exists. |
| **Steps** | SMTP AUTH PLAIN with `testuser@example.com@<group_id>` / `testpass123` |
| **Expected** | 535 5.7.8 "Authentication failed". Human (`account_type: "user"`) accounts are not eligible for SMTP. |

### TC-014: Suspended Account Cannot SMTP Auth

| Field | Value |
|-------|-------|
| **Precondition** | TC-010 passed. |
| **Steps** | 1. PATCH `/api/v1/users/<sa_id>/status` with `{"status":"suspended"}` <br> 2. SMTP AUTH PLAIN with service account credentials <br> 3. PATCH status back to `"active"` |
| **Expected** | Step 1: `status: "suspended"`. Step 2: 535 "Authentication failed". Step 3: auth works again. |

### TC-015: Expired API Key Cannot SMTP Auth

| Field | Value |
|-------|-------|
| **Precondition** | TC-010 passed. |
| **Steps** | 1. Set `expires_at` to past via DB: `UPDATE api_keys SET expires_at = NOW() - INTERVAL '1 day' WHERE user_id = '<sa_id>'` <br> 2. SMTP AUTH PLAIN with service account credentials <br> 3. Restore expiration to future |
| **Expected** | Step 2: 535 "API key expired". Step 3: auth works again after restoring. |

---

## Provider Access Control

### TC-016: Private Provider Not Accessible to Other Groups

| Field | Value |
|-------|-------|
| **Precondition** | TC-006, TC-008 passed. Provider 3 is private to Group A. |
| **Steps** | In Group B context, POST service account with `provider_id` = Provider 3 (Group A's private) |
| **Expected** | 400 "provider not found or not accessible to this group" |

### TC-017: Global Provider Accessible to All Groups

| Field | Value |
|-------|-------|
| **Precondition** | TC-004, TC-006 passed. Provider 2 is global. |
| **Steps** | In Group B context, POST service account with `provider_id` = Provider 2 (global) |
| **Expected** | 201 Created. Global providers are accessible to all groups. |

### TC-018: Shared Provider Not Accessible Before Grant

| Field | Value |
|-------|-------|
| **Precondition** | TC-005, TC-006 passed. Provider 4 is shared, no access granted. |
| **Steps** | In Group B context, POST service account with `provider_id` = Provider 4 (shared, no grant) |
| **Expected** | 400 "provider not found or not accessible to this group" |

### TC-019: Shared Provider Accessible After Access Grant

| Field | Value |
|-------|-------|
| **Precondition** | TC-018 passed. |
| **Steps** | 1. POST `/api/v1/providers/<p4_id>/access` with `{"group_id":"<group_b_id>"}` (as system admin) <br> 2. In Group B context, POST service account with `provider_id` = Provider 4 |
| **Expected** | Step 1: 204 No Content. Step 2: 201 Created. |

### TC-020: Shared Provider Blocked After Access Revoke

| Field | Value |
|-------|-------|
| **Precondition** | TC-019 passed. |
| **Steps** | 1. DELETE `/api/v1/providers/<p4_id>/access/<group_b_id>` <br> 2. In Group B context, POST service account with `provider_id` = Provider 4 |
| **Expected** | Step 1: 204 No Content. Step 2: 400 "provider not found or not accessible to this group" |

---

## API Key Reset

### TC-021: API Key Reset Generates New Key

| Field | Value |
|-------|-------|
| **Precondition** | TC-010 passed. Old API key known. |
| **Steps** | POST `/api/v1/groups/<group_a_id>/service-accounts/<sa_id>/reset-api-key` with `{"api_key_expires_in":"30d"}` |
| **Expected** | 200 OK. New `api_key` returned (different from old). `api_key_expires_at` set. |

### TC-022: Old API Key Rejected After Reset

| Field | Value |
|-------|-------|
| **Precondition** | TC-021 passed. |
| **Steps** | SMTP AUTH PLAIN with old API key |
| **Expected** | 535 "Authentication failed". Old key is immediately invalidated. |

### TC-023: New API Key Works After Reset

| Field | Value |
|-------|-------|
| **Precondition** | TC-021 passed. |
| **Steps** | SMTP AUTH PLAIN with new API key |
| **Expected** | Auth successful. Email sent and delivered. |

---

## Activity Log

### TC-024: Activity Logs Recorded

| Field | Value |
|-------|-------|
| **Precondition** | Previous test cases executed (user creation, status changes, API key resets). |
| **Steps** | GET `/api/v1/groups/<group_a_id>/activity?limit=20` |
| **Expected** | Array with entries. Actions include: `admin.create_user`, `admin.update_user_status`, `admin.reset_api_key`, etc. |

---

## Multi-API Key Support

### TC-025: Create Additional API Key for Service Account

| Field | Value |
|-------|-------|
| **Precondition** | TC-010b passed (service account exists with one key). |
| **Steps** | POST `/api/v1/groups/<group_a_id>/service-accounts/<sa_id>/api-keys` with `{"label":"ci-pipeline","api_key_expires_in":"90d"}` |
| **Expected** | 201 Created. Response includes plaintext `api_key`, `key_prefix` (first 12 chars), `label: "ci-pipeline"`, `expires_at` set 90 days from now. |

### TC-026: List API Keys for Service Account

| Field | Value |
|-------|-------|
| **Precondition** | TC-025 passed (2 keys exist for the service account). |
| **Steps** | GET `/api/v1/groups/<group_a_id>/service-accounts/<sa_id>/api-keys` |
| **Expected** | 200 OK. Array with 2 entries. Each has `id`, `key_prefix`, `label`, `is_active`, `expires_at`, `last_used_at`, `created_at`. No `key_hash` or plaintext exposed. |

### TC-027: SMTP Auth with Second API Key

| Field | Value |
|-------|-------|
| **Precondition** | TC-025 passed (second API key created). |
| **Steps** | SMTP AUTH PLAIN with `e2e-smtp@<group_a_id>` / `<new_api_key_from_tc025>` |
| **Expected** | 250 OK. Auth successful with the second API key. Message sent and delivered. |

### TC-027b: Deactivated API Key Rejected

| Field | Value |
|-------|-------|
| **Precondition** | TC-025 passed (second API key created and active). |
| **Steps** | 1. PATCH `/api/v1/groups/<group_a_id>/service-accounts/<sa_id>/api-keys/<key_id>` with `{"is_active":false}` <br> 2. SMTP AUTH PLAIN with `e2e-smtp@<group_a_id>` / `<api_key_from_tc025>` <br> 3. PATCH with `{"is_active":true}` to re-activate |
| **Expected** | Step 1: 200 OK, `is_active: false`. Step 2: 535 "Authentication failed" (inactive key rejected). Step 3: auth works again. |

### TC-028: Delete Specific API Key

| Field | Value |
|-------|-------|
| **Precondition** | TC-025 passed (2 keys exist). |
| **Steps** | DELETE `/api/v1/groups/<group_a_id>/service-accounts/<sa_id>/api-keys/<key_id_of_first_key>` |
| **Expected** | 204 No Content. First (default) API key is removed. Service account still has second key. |

### TC-029: Deleted API Key Rejected

| Field | Value |
|-------|-------|
| **Precondition** | TC-028 passed (first key deleted). |
| **Steps** | SMTP AUTH PLAIN with `e2e-smtp@<group_a_id>` / `<api_key_from_tc010>` (the deleted first key) |
| **Expected** | 535 "Authentication failed". Deleted API key is rejected. |

### TC-030: Remaining API Key Still Works

| Field | Value |
|-------|-------|
| **Precondition** | TC-028 passed (first key deleted, second key remains). |
| **Steps** | SMTP AUTH PLAIN with `e2e-smtp@<group_a_id>` / `<api_key_from_tc025>` (the second key) |
| **Expected** | 250 OK. Auth successful with remaining API key. Message sent and delivered. |

---

## Test Data Summary

| Resource | Name | Type | Scope |
|----------|------|------|-------|
| Provider 1 | test-stdout-private | stdout | System group, private |
| Provider 2 | test-stdout-global | stdout | Global (all groups) |
| Provider 3 | test-group-provider | stdout | Group A, private |
| Provider 4 | test-stdout-shared | stdout | System group, shared |
| Group A | e2e-test-group-a | company | - |
| Group B | e2e-test-group-b | company | - |
| Human User | testuser@example.com | user | Group A |
| Service Account | e2e-smtp | smtp | Group A, Provider 3 |
| API Key 1 | default | smtp-key | Group A, e2e-smtp, 30d expiry |
| API Key 2 | ci-pipeline | smtp-key | Group A, e2e-smtp, 90d expiry |

---

## Running

```bash
# Full E2E suite (clean build + assertions across 32 test cases)
bash e2e-test.sh

# Manual SMTP test
docker compose run --rm test-client \
  --host=smtp-server --port=587 --tls=starttls --insecure \
  --user="<username>@<group_id>" --password="<api_key>" \
  --from="sender@example.com" --to="recipient@example.com" \
  --subject="Test" --body="Hello"
```
