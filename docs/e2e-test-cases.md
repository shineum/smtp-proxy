# E2E Test Cases

Automation script: [`e2e-test.sh`](../e2e-test.sh)

---

## TC-001: Clean Build & Service Startup

| Field | Value |
|-------|-------|
| **Precondition** | Docker and Docker Compose installed |
| **Steps** | 1. `docker compose down -v --remove-orphans` <br> 2. `docker compose up -d --build` <br> 3. Wait for API server healthcheck (`/healthz`) |
| **Expected** | All services start: postgres, redis, migrate, api-server, smtp-server, queue-worker. Migration completes without error. API healthcheck returns 200 within 60s. |

---

## TC-002: Admin Login

| Field | Value |
|-------|-------|
| **Precondition** | TC-001 passed. System admin auto-seeded (`admin@localhost` / `admin`). |
| **Steps** | POST `/api/v1/auth/login` with `{"email":"admin@localhost","password":"admin"}` |
| **Expected** | 200 OK with `access_token`, `refresh_token`, `token_type: "Bearer"`, `expires_in: 900` |

---

## TC-003: Create Provider (stdout, private)

| Field | Value |
|-------|-------|
| **Precondition** | TC-002 passed. Admin JWT available. |
| **Steps** | POST `/api/v1/providers` with `{"name":"test-stdout-private","provider_type":"stdout","enabled":true,"visibility":"private"}` |
| **Expected** | 201 Created. Response includes `id`, `provider_type: "stdout"`, `visibility: "private"`, `group_id` matching system group. |

---

## TC-004: Create Provider (stdout, global)

| Field | Value |
|-------|-------|
| **Precondition** | TC-002 passed. Admin JWT available. |
| **Steps** | POST `/api/v1/providers` with `{"name":"test-stdout-global","provider_type":"stdout","enabled":true,"visibility":"global"}` |
| **Expected** | 201 Created. `visibility: "global"`. Accessible to all groups. |

---

## TC-005: Create Group

| Field | Value |
|-------|-------|
| **Precondition** | TC-002 passed. Admin JWT available. |
| **Steps** | POST `/api/v1/groups` with `{"name":"e2e-test-group","monthly_limit":1000,"display_name":"E2E Test Group"}` |
| **Expected** | 201 Created. Response includes `id`, `group_key` (UUID), `group_type: "company"`, `status: "active"`. |

---

## TC-006: Switch Group Context

| Field | Value |
|-------|-------|
| **Precondition** | TC-005 passed. Group ID available. |
| **Steps** | POST `/api/v1/auth/switch-group` with `{"group_id":"<group_id>"}` |
| **Expected** | 200 OK. New `access_token` with group context. JWT claims include `group_id` and `role: "owner"`. |

---

## TC-007: Create Provider (group-scoped, private)

| Field | Value |
|-------|-------|
| **Precondition** | TC-006 passed. Group context JWT available. |
| **Steps** | POST `/api/v1/providers` with `{"name":"test-group-provider","provider_type":"stdout","enabled":true,"visibility":"private"}` |
| **Expected** | 201 Created. `group_id` matches test group. Only accessible within the test group. |

---

## TC-008: Create Human User

| Field | Value |
|-------|-------|
| **Precondition** | TC-006 passed. Group context JWT available. |
| **Steps** | POST `/api/v1/users` with `{"email":"testuser@example.com","password":"testpass123","account_type":"user","group_id":"<group_id>"}` |
| **Expected** | 201 Created. `account_type: "user"`, `status: "active"`. No `api_key` returned. |

---

## TC-009: Create SMTP Service Account

| Field | Value |
|-------|-------|
| **Precondition** | TC-006, TC-007 passed. Group context JWT and provider ID available. |
| **Steps** | POST `/api/v1/groups/<group_id>/service-accounts` with `{"username":"e2e-smtp","provider_id":"<provider_id>"}` |
| **Expected** | 201 Created. `account_type: "smtp"`, `username: "e2e-smtp"` (lowercase), `api_key` returned (visible only at creation), `home_group_id` matches group. |

---

## TC-010: SMTP Send - Simple Case

| Field | Value |
|-------|-------|
| **Precondition** | TC-009 passed. Service account username and api_key available. |
| **Steps** | Connect to SMTP server (port 587, STARTTLS). <br> AUTH PLAIN with `e2e-smtp@<group_key>` / `<api_key>`. <br> Send email: From `sender@example.com`, To `recipient@example.com`, Subject `E2E Simple Test`, plain text body. |
| **Expected** | SMTP server accepts and returns 250 OK. Auth logs show `auth successful`. Message persisted and enqueued. Queue worker delivers via stdout provider. |
| **Verify** | SMTP server log: `message persisted`, `message enqueued`. Queue worker log: `message delivered by worker`. |

---

## TC-011: SMTP Send - Complex Case

| Field | Value |
|-------|-------|
| **Precondition** | TC-009 passed. Service account username and api_key available. Test attachment files exist. |
| **Steps** | Connect to SMTP server (port 587, STARTTLS). <br> AUTH PLAIN with `e2e-smtp@<group_key>` / `<api_key>`. <br> Send email with: From `sender@example.com`, To `recipient@example.com`, CC `cc-user@example.com`, BCC `bcc-user@example.com`, Subject `E2E Complex Test`, plain text body, HTML body, 2 attachments (`sample.txt`, `sample.html`). |
| **Expected** | SMTP server accepts 3 RCPT TO commands (to, cc, bcc). Message persisted with `recipient_count: 3`. Queue worker delivers via stdout provider. BCC not visible in message headers. |
| **Verify** | SMTP server log: 3x `RCPT TO accepted`, `message persisted` with `recipient_count: 3`. Queue worker log: `message delivered by worker`. |

---

## Test Data Summary

| Resource | Name | Type | Scope |
|----------|------|------|-------|
| Provider 1 | test-stdout-private | stdout | System group, private |
| Provider 2 | test-stdout-global | stdout | Global (all groups) |
| Provider 3 | test-group-provider | stdout | Test group, private |
| Group | e2e-test-group | company | - |
| Human User | testuser@example.com | user | Test group |
| Service Account | e2e-smtp | smtp | Test group, Provider 3 |

---

## Running

```bash
# Full E2E suite (clean build + all test cases)
bash e2e-test.sh

# Manual: just the SMTP tests (assumes services are running)
docker compose run --rm test-client \
  --host=smtp-server --port=587 --tls=starttls --insecure \
  --user="e2e-smtp@<group_key>" --password="<api_key>" \
  --from="sender@example.com" --to="recipient@example.com" \
  --subject="Manual Test" --body="Hello"
```
