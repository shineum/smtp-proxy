---
id: SPEC-MULTITENANT-001
version: "1.0.0"
status: approved
created: "2026-02-15"
updated: "2026-02-15"
author: sungwon
priority: P0
---

# SPEC-MULTITENANT-001: Tenant Isolation, Authentication, and Access Control

## HISTORY

| Date       | Version | Author   | Changes                          |
|------------|---------|----------|----------------------------------|
| 2026-02-15 | 1.0.0   | sungwon  | Initial SPEC creation           |

---

## ENVIRONMENT

**Project Context:**
Multi-tenant SMTP proxy service requiring complete tenant isolation at the data layer, secure authentication mechanisms for both API and SMTP protocols, and role-based access control (RBAC) for administrative operations.

**Technology Stack:**
- Go 1.21+ with standard library net/smtp
- PostgreSQL 14+ with Row-Level Security (RLS)
- JWT tokens using golang-jwt/jwt v5 with RS256 asymmetric signing
- bcrypt password hashing (cost factor 12+)
- chi router with middleware chain architecture
- Redis for rate limiting with sliding window algorithm

**System Boundaries:**
- API Server: HTTP REST API for tenant/account management
- SMTP Server: RFC 5321 compliant SMTP service with AUTH extension
- Database: PostgreSQL with tenant_id foreign keys on all tenant-scoped tables
- Authentication: JWT-based API auth + SMTP AUTH PLAIN/LOGIN mechanisms
- Authorization: RBAC with three roles (owner, admin, member)
- Rate Limiting: Per-tenant enforcement using Redis

**Integration Points:**
- External SMTP Providers: Forwarding authenticated emails to configured providers
- API Clients: Web dashboard, CLI tools, third-party integrations
- Database: PostgreSQL RLS policies enforcing tenant isolation
- Cache Layer: Redis for rate limits and session state

---

## ASSUMPTIONS

1. **Security Model:**
   - Assumption: All SMTP communication uses TLS encryption (STARTTLS or implicit TLS)
   - Confidence: High
   - Evidence: Modern SMTP security standards require encryption
   - Risk if Wrong: Credentials transmitted in plaintext, catastrophic security breach
   - Validation: Enforce TLS in SMTP server configuration, reject unencrypted AUTH

2. **Database Performance:**
   - Assumption: PostgreSQL RLS policies will not cause significant performance degradation
   - Confidence: Medium
   - Evidence: RLS adds WHERE clause to queries, indexed tenant_id columns mitigate cost
   - Risk if Wrong: Slow query performance at scale, customer dissatisfaction
   - Validation: Benchmark queries with RLS enabled, monitor production metrics

3. **Token Lifecycle:**
   - Assumption: 15-minute access token expiry provides acceptable security/UX balance
   - Confidence: High
   - Evidence: Industry standard for short-lived tokens (AWS, Auth0 use 15-60 minutes)
   - Risk if Wrong: Excessive refresh requests or poor UX from frequent re-auth
   - Validation: Monitor refresh token usage patterns, gather user feedback

4. **SMTP AUTH Mechanism Support:**
   - Assumption: PLAIN and LOGIN mechanisms cover 95%+ of SMTP clients
   - Confidence: High
   - Evidence: Industry standard, supported by all major email clients
   - Risk if Wrong: Client compatibility issues, support escalations
   - Validation: Test with popular SMTP clients (Thunderbird, Outlook, Gmail)

5. **Rate Limiting Granularity:**
   - Assumption: Per-tenant rate limiting is sufficient (not per-account within tenant)
   - Confidence: Medium
   - Evidence: Simplifies implementation, aligns with tenant isolation model
   - Risk if Wrong: One account within tenant can exhaust quota for entire tenant
   - Validation: Monitor abuse patterns, consider per-account limits in future iteration

---

## REQUIREMENTS

### Ubiquitous Requirements (Always Active)

**UR-001: Tenant Data Isolation**
The system SHALL always isolate tenant data at PostgreSQL row level using tenant_id foreign key constraints and Row-Level Security policies.

**UR-002: Password Security**
The system SHALL always hash passwords using bcrypt with minimum cost factor of 12 before storage.

**UR-003: JWT Signature Validation**
The API SHALL always validate JWT token signature and expiration timestamp on protected endpoints before processing requests.

**UR-004: SMTP Rate Limiting**
The SMTP service SHALL always enforce rate limits per tenant account using sliding window algorithm with Redis.

**UR-005: Audit Logging**
The system SHALL always log authentication attempts, authorization failures, and administrative actions to audit_logs table with timestamp, user_id, tenant_id, action, and result fields.

---

### Event-Driven Requirements (Trigger-Response)

**ED-001: User Login**
WHEN user sends POST /api/v1/auth/login with valid credentials (email + password), THEN system SHALL return HTTP 200 with JWT access token (expiry 15 minutes) and refresh token (expiry 7 days) in response body.

**ED-002: Invalid Login Attempt**
WHEN user sends POST /api/v1/auth/login with invalid credentials, THEN system SHALL return HTTP 401 Unauthorized, increment failed_attempts counter, and apply exponential backoff after 5 failures.

**ED-003: SMTP AUTH PLAIN**
WHEN SMTP AUTH command received with PLAIN mechanism and base64-encoded credentials, THEN system SHALL decode credentials, validate against accounts table with bcrypt.CompareHashAndPassword, and respond with 235 Authentication successful or 535 Authentication failed.

**ED-004: SMTP AUTH LOGIN**
WHEN SMTP AUTH command received with LOGIN mechanism, THEN system SHALL prompt for username with 334 response, await base64-encoded username, prompt for password, await base64-encoded password, validate against accounts table, and respond with 235 or 535.

**ED-005: Rate Limit Exceeded**
WHEN rate limit exceeded for tenant account, THEN system SHALL reject SMTP connection with 421 Service not available response including Retry-After header with seconds until quota reset.

**ED-006: Tenant Creation**
WHEN API receives POST /api/v1/tenants with name and owner_email fields, THEN system SHALL create tenant record with isolated namespace, create owner user with generated password, send welcome email with credentials, and return HTTP 201 with tenant_id.

**ED-007: Role Update**
WHEN admin updates user role via PATCH /api/v1/users/{id}/role, THEN system SHALL validate requester has owner or admin role, update users.role column, invalidate all active sessions for target user by deleting from sessions table, and return HTTP 200.

**ED-008: Token Refresh**
WHEN API receives POST /api/v1/auth/refresh with valid refresh token, THEN system SHALL issue new access token with 15-minute expiry, optionally rotate refresh token if older than 24 hours, and return HTTP 200 with tokens.

**ED-009: Expired Access Token**
WHEN API receives request with expired access token but valid refresh token in headers, THEN system SHALL return HTTP 401 Unauthorized with error code TOKEN_EXPIRED, prompting client to refresh.

**ED-010: Tenant Quota Exhaustion**
WHEN tenant email quota exhausted (monthly_sent >= monthly_limit), THEN system SHALL reject SMTP MAIL FROM command with 452 Requested action not taken: mailbox quota exceeded response.

---

### State-Driven Requirements (Conditional)

**SD-001: Owner Privileges**
IF user role is "owner", THEN system SHALL allow tenant deletion via DELETE /api/v1/tenants/{id}, billing operations via /api/v1/billing endpoints, and all admin/member capabilities.

**SD-002: Admin Privileges**
IF user role is "admin", THEN system SHALL allow account CRUD operations via /api/v1/accounts endpoints, provider configuration via /api/v1/providers endpoints, dashboard read access, but deny tenant deletion and billing operations.

**SD-003: Member Privileges**
IF user role is "member", THEN system SHALL restrict to read-only dashboard access via GET /api/v1/dashboard, deny all write operations, and deny account/provider configuration.

**SD-004: Token Refresh Flow**
IF JWT access token is expired (exp claim < current timestamp) but refresh token valid (not expired, exists in sessions table), THEN system SHALL issue new access token via POST /api/v1/auth/refresh without requiring re-authentication.

**SD-005: Account Status Check**
IF SMTP account status is "suspended" (accounts.status = 'suspended'), THEN system SHALL reject SMTP AUTH with 535 Authentication failed and log suspension reason.

---

### Unwanted Requirements (Prohibitions)

**UW-001: Cross-Tenant Data Access**
The system SHALL NOT allow cross-tenant data access under any circumstance, even for administrative users with owner role. Each user belongs to exactly one tenant and can only access data within that tenant's namespace.

**UW-002: Plaintext Password Storage**
The system SHALL NOT store passwords in plaintext format. All passwords must be hashed using bcrypt with cost factor >= 12 before database insertion.

**UW-003: Unencrypted SMTP AUTH**
The SMTP service SHALL NOT accept AUTH commands over unencrypted connections. Clients must establish TLS via STARTTLS before authentication.

**UW-004: Sensitive Data in Logs**
The system SHALL NOT include sensitive data (passwords, API keys, JWT tokens, email content) in application logs or error responses. Log only non-sensitive identifiers (user_id, tenant_id, action type).

**UW-005: Unconfirmed Tenant Deletion**
The system SHALL NOT allow tenant deletion without explicit confirmation workflow requiring owner role authentication, email verification, and 24-hour grace period for recovery.

**UW-006: Concurrent Session Limit Bypass**
The system SHALL NOT allow users to exceed concurrent session limits (default 5 sessions per user). New login attempts beyond limit must invalidate oldest session.

---

### Optional Requirements (Enhancements)

**OP-001: OAuth 2.0 Support**
WHERE possible, system SHALL support OAuth 2.0 (Google, GitHub) for admin panel login via /api/v1/auth/oauth/{provider} endpoints.

**OP-002: API Key Authentication**
WHERE possible, system SHALL support API key authentication for programmatic access via X-API-Key header with key management at /api/v1/api-keys endpoints.

**OP-003: IP Allowlisting**
WHERE possible, system SHALL support IP allowlisting per tenant via tenants.allowed_ips CIDR array column, rejecting requests from unauthorized IPs with 403 Forbidden.

**OP-004: Audit Log Export**
WHERE possible, system SHALL provide audit log export functionality at GET /api/v1/audit-logs/export with JSON/CSV format options and date range filters.

**OP-005: Multi-Factor Authentication**
WHERE possible, system SHALL support TOTP-based MFA via /api/v1/auth/mfa endpoints with QR code enrollment and backup codes.

---

## SPECIFICATIONS

### Database Schema

**Core Tables:**

```sql
-- Tenants table
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    monthly_limit INTEGER DEFAULT 10000,
    monthly_sent INTEGER DEFAULT 0,
    allowed_ips CIDR[],
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    status VARCHAR(50) DEFAULT 'active',
    failed_attempts INTEGER DEFAULT 0,
    last_login TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- SMTP Accounts table
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    username VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(tenant_id, username)
);

-- Sessions table
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Audit Logs table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100),
    resource_id UUID,
    result VARCHAR(50) NOT NULL CHECK (result IN ('success', 'failure')),
    metadata JSONB,
    ip_address INET,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Rate Limits tracking (Redis alternative for persistence)
CREATE TABLE rate_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id UUID REFERENCES accounts(id) ON DELETE CASCADE,
    window_start TIMESTAMP NOT NULL,
    count INTEGER DEFAULT 0,
    UNIQUE(tenant_id, account_id, window_start)
);
```

**Row-Level Security Policies:**

```sql
-- Enable RLS on tenant-scoped tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

-- Policy: Users can only access data within their tenant
CREATE POLICY tenant_isolation_users ON users
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE POLICY tenant_isolation_accounts ON accounts
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE POLICY tenant_isolation_sessions ON sessions
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE POLICY tenant_isolation_audit_logs ON audit_logs
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
```

### JWT Token Structure

**Access Token Claims:**
```json
{
  "sub": "user_id",
  "tenant_id": "tenant_uuid",
  "email": "user@example.com",
  "role": "admin",
  "exp": 1708012345,
  "iat": 1708011445,
  "iss": "smtp-proxy-api",
  "aud": "smtp-proxy-clients"
}
```

**Refresh Token Claims:**
```json
{
  "sub": "user_id",
  "tenant_id": "tenant_uuid",
  "session_id": "session_uuid",
  "exp": 1708617245,
  "iat": 1708011445,
  "iss": "smtp-proxy-api"
}
```

### Middleware Chain

**Request Flow:**
```
HTTP Request
  ↓
[1] CORS Middleware (chi/cors)
  ↓
[2] Request Logging (chi/logger)
  ↓
[3] JWT Authentication (custom)
  ↓
[4] Tenant Context Setting (custom)
  ↓
[5] RBAC Authorization (custom)
  ↓
[6] Rate Limiting (custom)
  ↓
Handler
```

### Security Standards

**OWASP Top 10 Compliance Checklist:**

1. Broken Access Control: Mitigated by RLS policies + RBAC middleware
2. Cryptographic Failures: Mitigated by bcrypt (cost 12+) + TLS enforcement
3. Injection: Mitigated by parameterized queries (sqlx) + input validation
4. Insecure Design: Mitigated by SPEC-first design + security review
5. Security Misconfiguration: Mitigated by secure defaults + configuration validation
6. Vulnerable Components: Mitigated by dependency scanning (govulncheck)
7. Identification/Auth Failures: Mitigated by JWT + bcrypt + rate limiting
8. Software/Data Integrity: Mitigated by signed JWTs + audit logs
9. Logging/Monitoring Failures: Mitigated by comprehensive audit logging
10. SSRF: Mitigated by SMTP provider URL validation + allowlist

---

## TRACEABILITY

**Related SPECs:**
- SPEC-SMTP-001: SMTP Server Implementation (depends on this SPEC for authentication)
- SPEC-API-001: REST API Design (implements endpoints defined here)
- SPEC-PROVIDER-001: External Provider Integration (uses authenticated accounts)

**External References:**
- RFC 5321: Simple Mail Transfer Protocol
- RFC 4954: SMTP Service Extension for Authentication
- RFC 7519: JSON Web Token (JWT)
- OWASP Top 10 2021: https://owasp.org/Top10/

**Git Branch:**
- feature/multitenant-auth

**Test Coverage Target:**
- 85%+ overall coverage
- 95%+ coverage for authentication/authorization code paths

---

## NOTES

**Security Review Required:**
This SPEC introduces authentication and authorization mechanisms that are security-critical. Mandatory security review by security-expert agent before implementation.

**Performance Considerations:**
PostgreSQL RLS adds WHERE clause to every query. Ensure tenant_id columns are indexed on all affected tables. Benchmark query performance during implementation.

**Future Enhancements:**
- Single Sign-On (SSO) integration with SAML/OIDC
- Fine-grained permissions beyond owner/admin/member roles
- API rate limiting (separate from SMTP rate limiting)
- Session analytics and anomaly detection
