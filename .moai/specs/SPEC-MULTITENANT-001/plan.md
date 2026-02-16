# Implementation Plan: SPEC-MULTITENANT-001

## Task Decomposition

### Phase 1: Database Schema and Security (Priority: P0)

**Task 1.1: Create Core Database Schema**
- Create migration files for tenants, users, accounts, sessions, audit_logs, rate_limits tables
- Add NOT NULL constraints, foreign keys, and CHECK constraints
- Create indexes on tenant_id, email, username, session tokens
- Estimated Complexity: Medium (2-3 files)

**Task 1.2: Implement Row-Level Security**
- Enable RLS on tenant-scoped tables
- Create tenant_isolation policies for users, accounts, sessions, audit_logs
- Test RLS enforcement with multi-tenant queries
- Estimated Complexity: Medium (1 file + tests)

**Task 1.3: Database Seeding and Test Data**
- Create seed script for development environment
- Generate test tenants with owner/admin/member users
- Generate test SMTP accounts for integration testing
- Estimated Complexity: Low (1 file)

---

### Phase 2: Authentication Core (Priority: P0)

**Task 2.1: JWT Token Generation and Validation**
- Generate RSA key pair for RS256 signing (2048-bit minimum)
- Implement GenerateAccessToken(user, tenant) returning signed JWT
- Implement GenerateRefreshToken(user, session) returning signed JWT
- Implement ValidateToken(tokenString) with signature + expiry verification
- Estimated Complexity: High (2-3 files + key management)

**Task 2.2: bcrypt Password Hashing**
- Implement HashPassword(plaintext) using bcrypt.GenerateFromPassword with cost 12
- Implement ComparePassword(hash, plaintext) using bcrypt.CompareHashAndPassword
- Add password strength validation (min 12 chars, complexity requirements)
- Estimated Complexity: Low (1 file + validation)

**Task 2.3: User Login Endpoint**
- Implement POST /api/v1/auth/login handler
- Validate email + password against users table
- Generate access + refresh tokens on success
- Create session record in sessions table
- Implement exponential backoff after 5 failed attempts
- Estimated Complexity: High (3-4 files: handler, service, repository)

**Task 2.4: Token Refresh Endpoint**
- Implement POST /api/v1/auth/refresh handler
- Validate refresh token signature and expiry
- Check session existence in sessions table
- Generate new access token
- Optionally rotate refresh token if > 24 hours old
- Estimated Complexity: Medium (2-3 files)

**Task 2.5: Logout Endpoint**
- Implement POST /api/v1/auth/logout handler
- Delete session record from sessions table by refresh token hash
- Invalidate client-side tokens (return 204 No Content)
- Estimated Complexity: Low (1-2 files)

---

### Phase 3: SMTP Authentication (Priority: P0)

**Task 3.1: SMTP AUTH PLAIN Mechanism**
- Parse AUTH PLAIN command with base64-encoded credentials
- Decode to extract username and password
- Validate against accounts table with ComparePassword
- Respond with 235 (success) or 535 (failure)
- Log authentication attempts to audit_logs
- Estimated Complexity: Medium (2 files)

**Task 3.2: SMTP AUTH LOGIN Mechanism**
- Handle AUTH LOGIN command with interactive challenge-response
- Send 334 VXNlcm5hbWU6 (base64 "Username:")
- Receive base64-encoded username
- Send 334 UGFzc3dvcmQ6 (base64 "Password:")
- Receive base64-encoded password
- Validate and respond with 235 or 535
- Estimated Complexity: Medium (2 files)

**Task 3.3: TLS Enforcement for SMTP AUTH**
- Reject AUTH commands on unencrypted connections with 530 Must issue STARTTLS first
- Ensure STARTTLS completion before accepting AUTH
- Estimated Complexity: Low (1 file)

---

### Phase 4: Middleware Chain (Priority: P0)

**Task 4.1: JWT Authentication Middleware**
- Extract JWT from Authorization: Bearer header
- Validate token signature and expiry with ValidateToken
- Extract user_id, tenant_id, role from claims
- Attach to request context
- Return 401 Unauthorized on validation failure
- Estimated Complexity: Medium (1-2 files)

**Task 4.2: Tenant Context Middleware**
- Retrieve tenant_id from JWT claims in context
- Execute SET LOCAL app.current_tenant_id = ? for RLS enforcement
- Estimated Complexity: Low (1 file)

**Task 4.3: RBAC Authorization Middleware**
- Define route-role mapping (e.g., DELETE /tenants requires owner)
- Check user role from JWT claims against required role
- Return 403 Forbidden if insufficient privileges
- Estimated Complexity: Medium (1-2 files)

**Task 4.4: Rate Limiting Middleware**
- Implement sliding window algorithm using Redis
- Key format: ratelimit:{tenant_id}:{account_id}:{window}
- Increment counter on each request
- Return 421 with Retry-After header on limit exceeded
- Estimated Complexity: High (2-3 files + Redis integration)

---

### Phase 5: RBAC Enforcement (Priority: P1)

**Task 5.1: Tenant Management Endpoints**
- POST /api/v1/tenants (create tenant, requires system admin or public during signup)
- DELETE /api/v1/tenants/{id} (requires owner role)
- PATCH /api/v1/tenants/{id} (requires owner/admin)
- Estimated Complexity: Medium (3-4 files)

**Task 5.2: Account Management Endpoints**
- GET /api/v1/accounts (requires admin/owner)
- POST /api/v1/accounts (requires admin/owner)
- PATCH /api/v1/accounts/{id} (requires admin/owner)
- DELETE /api/v1/accounts/{id} (requires admin/owner)
- Estimated Complexity: Medium (4-5 files)

**Task 5.3: User Role Updates**
- PATCH /api/v1/users/{id}/role (requires owner/admin)
- Invalidate all sessions for target user after role change
- Estimated Complexity: Low (2 files)

---

### Phase 6: Audit Logging (Priority: P1)

**Task 6.1: Audit Logging Service**
- Implement LogAuditEvent(tenant_id, user_id, action, result, metadata)
- Insert into audit_logs table with timestamp, IP address, resource info
- Async logging to avoid blocking request handlers
- Estimated Complexity: Medium (2 files)

**Task 6.2: Audit Log Retrieval**
- GET /api/v1/audit-logs with pagination, filtering by date range, user, action
- Requires admin/owner role
- Estimated Complexity: Low (2 files)

---

### Phase 7: Testing (Priority: P0)

**Task 7.1: Unit Tests**
- JWT generation/validation tests with mock keys
- bcrypt hashing tests with known inputs/outputs
- Middleware tests with mock request contexts
- Target: 85%+ coverage for auth/authz code
- Estimated Complexity: High (10+ test files)

**Task 7.2: Integration Tests**
- End-to-end login flow with database
- SMTP AUTH PLAIN/LOGIN with test accounts
- Rate limiting enforcement tests with Redis
- RLS policy verification with multi-tenant queries
- Target: 90%+ coverage for integration paths
- Estimated Complexity: High (8+ test files)

**Task 7.3: Security Tests**
- SQL injection attempts on login endpoints
- JWT tampering detection tests
- Cross-tenant access attempt tests (should fail)
- Brute force protection tests (exponential backoff)
- Estimated Complexity: Medium (4-5 test files)

---

### Phase 8: Documentation and Deployment (Priority: P2)

**Task 8.1: API Documentation**
- OpenAPI 3.0 specification for all endpoints
- Authentication flow diagrams
- RBAC role matrix table
- Estimated Complexity: Low (1-2 files)

**Task 8.2: Deployment Configuration**
- Generate RSA key pair for production (store in secrets manager)
- Configure Redis connection for rate limiting
- Set up PostgreSQL RLS policies in production migrations
- Environment variable documentation
- Estimated Complexity: Medium (configuration files)

---

## Technology Stack

### Core Dependencies

**Authentication & Security:**
- golang-jwt/jwt v5 - JWT generation and validation with RS256 signing
- golang.org/x/crypto/bcrypt - Password hashing with configurable cost factor
- crypto/rand - Cryptographically secure random number generation for tokens

**Web Framework:**
- go-chi/chi v5 - Lightweight HTTP router with middleware support
- go-chi/cors - CORS middleware for cross-origin API access

**Database:**
- lib/pq - PostgreSQL driver for database/sql interface
- jackc/pgx v5 - High-performance PostgreSQL driver with connection pooling
- golang-migrate/migrate v4 - Database migration management

**Rate Limiting:**
- go-redis/redis v9 - Redis client for rate limiting storage
- sliding-window algorithm implementation (custom)

**Testing:**
- testify/assert - Assertion library for test readability
- testify/mock - Mock generation for interfaces
- httptest - HTTP server testing utilities

---

## Architecture Decisions

### Decision 1: JWT with RS256 Asymmetric Signing

**Rationale:**
RS256 (RSA Signature with SHA-256) provides asymmetric key cryptography where:
- Private key signs tokens (API server only)
- Public key verifies tokens (can be distributed to other services)
- Enables token verification without shared secrets

**Alternatives Considered:**
- HS256 (HMAC-SHA256): Simpler but requires shared secret, not suitable for distributed verification
- ES256 (ECDSA): Smaller keys but less widely supported in Go ecosystem

**Trade-offs:**
- Pros: Enhanced security, public key distribution, better for microservices
- Cons: Larger key size (2048-bit RSA), slightly slower than HMAC

**Implementation:**
```go
// Generate RSA key pair (development only, production uses managed keys)
privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
publicKey := &privateKey.PublicKey

// Sign token
token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
tokenString, _ := token.SignedString(privateKey)

// Verify token
parsedToken, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
    return publicKey, nil
})
```

---

### Decision 2: PostgreSQL Row-Level Security for Tenant Isolation

**Rationale:**
RLS enforces tenant isolation at the database layer, preventing application-level bugs from causing cross-tenant data leaks. Every query automatically includes `WHERE tenant_id = current_tenant_id` without developer intervention.

**Alternatives Considered:**
- Application-level filtering: Manual WHERE clauses in every query (error-prone)
- Separate schemas per tenant: Complex migrations, not scalable
- Separate databases per tenant: Operational nightmare, expensive

**Trade-offs:**
- Pros: Defense-in-depth security, impossible to bypass, automatic enforcement
- Cons: Query performance overhead (mitigated by indexing), added complexity

**Implementation:**
```sql
-- Enable RLS on table
ALTER TABLE users ENABLE ROW LEVEL SECURITY;

-- Create policy enforcing tenant_id match
CREATE POLICY tenant_isolation ON users
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
```

```go
// Set tenant context before queries
_, err := db.Exec("SET LOCAL app.current_tenant_id = $1", tenantID)
```

---

### Decision 3: Sliding Window Rate Limiting with Redis

**Rationale:**
Sliding window algorithm provides smoother rate limiting than fixed windows:
- Fixed window: Allows 2x limit burst at window boundaries
- Sliding window: Distributes requests evenly over time window

**Alternatives Considered:**
- Fixed window: Simpler but bursty traffic patterns
- Token bucket: More complex, unnecessary for SMTP use case
- Database-based: Too slow for high-frequency rate limit checks

**Trade-offs:**
- Pros: Accurate rate limiting, high performance, distributed state
- Cons: Redis dependency, requires cache invalidation strategy

**Implementation:**
```go
// Sliding window key: ratelimit:{tenant_id}:{account_id}:{minute}
key := fmt.Sprintf("ratelimit:%s:%s:%d", tenantID, accountID, time.Now().Unix()/60)
count, _ := redisClient.Incr(ctx, key).Result()
if count == 1 {
    redisClient.Expire(ctx, key, time.Minute)
}
if count > limit {
    return ErrRateLimitExceeded
}
```

---

### Decision 4: Middleware Chain Ordering

**Rationale:**
Middleware execution order is critical for correct auth/authz flow:
1. CORS: Enable cross-origin requests before auth checks
2. Logging: Record all requests including failed auth attempts
3. JWT Auth: Extract and validate token early
4. Tenant Context: Set RLS context after auth succeeds
5. RBAC: Check role permissions after user identified
6. Rate Limit: Enforce limits after user authenticated

**Trade-offs:**
- Pros: Clear separation of concerns, composable middleware
- Cons: Order dependency (must document carefully)

---

## Database Schema Design

### Design Principles

1. **Tenant-First Design:** Every tenant-scoped table has tenant_id foreign key
2. **Cascade Deletes:** ON DELETE CASCADE for tenant deletion cleanup
3. **UUID Primary Keys:** Prevents enumeration attacks, distributed generation
4. **Indexed Foreign Keys:** tenant_id indexed on all tables for RLS performance
5. **Audit Trail:** created_at, updated_at timestamps on all mutable tables

### Indexing Strategy

**Critical Indexes:**
```sql
-- Users table
CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status) WHERE status = 'active';

-- Accounts table
CREATE INDEX idx_accounts_tenant_id ON accounts(tenant_id);
CREATE INDEX idx_accounts_username ON accounts(tenant_id, username);

-- Sessions table
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at) WHERE expires_at > NOW();

-- Audit Logs table
CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
```

---

## Security Considerations

### OWASP Top 10 Compliance Mapping

**A01: Broken Access Control**
- Mitigation: PostgreSQL RLS policies + RBAC middleware
- Testing: Cross-tenant access attempt tests, privilege escalation tests

**A02: Cryptographic Failures**
- Mitigation: bcrypt (cost 12+) for passwords, TLS for SMTP, RS256 for JWT
- Testing: Password hash strength tests, TLS enforcement tests

**A03: Injection**
- Mitigation: Parameterized queries with sqlx, input validation middleware
- Testing: SQL injection attempts on login, account creation endpoints

**A04: Insecure Design**
- Mitigation: SPEC-first design review, security expert consultation
- Testing: Threat modeling, attack surface analysis

**A05: Security Misconfiguration**
- Mitigation: Secure defaults (TLS required, RLS enabled), configuration validation
- Testing: Configuration audit, default credential checks

**A06: Vulnerable and Outdated Components**
- Mitigation: Dependency scanning with `govulncheck`, automated updates
- Testing: CI/CD pipeline vulnerability scans

**A07: Identification and Authentication Failures**
- Mitigation: JWT with short expiry, bcrypt with high cost, rate limiting
- Testing: Brute force protection tests, session fixation tests

**A08: Software and Data Integrity Failures**
- Mitigation: Signed JWTs (RS256), audit logging, database constraints
- Testing: Token tampering detection tests

**A09: Security Logging and Monitoring Failures**
- Mitigation: Comprehensive audit_logs table, structured logging
- Testing: Audit log completeness tests, monitoring alert tests

**A10: Server-Side Request Forgery (SSRF)**
- Mitigation: SMTP provider URL validation, allowlist-based forwarding
- Testing: Malicious URL injection tests

---

## Risk Analysis and Mitigation

### Risk 1: JWT Token Theft

**Probability:** Medium
**Impact:** High (account takeover)
**Mitigation:**
- Short access token expiry (15 minutes)
- Refresh token rotation on reuse
- HttpOnly cookies for web clients (future enhancement)
- IP address validation in sessions table (optional enhancement)

**Monitoring:**
- Alert on unusual geographic login patterns
- Alert on concurrent sessions from different IPs

---

### Risk 2: Rate Limit Bypass

**Probability:** Medium
**Impact:** Medium (quota exhaustion, DoS)
**Mitigation:**
- Redis-based distributed rate limiting
- Fallback to database-backed limits if Redis unavailable
- Multiple limit tiers (per-account, per-tenant, per-IP)

**Monitoring:**
- Alert on rapid account creation within tenant
- Alert on Redis connection failures

---

### Risk 3: PostgreSQL RLS Performance Degradation

**Probability:** Medium
**Impact:** High (slow queries, customer dissatisfaction)
**Mitigation:**
- Comprehensive indexing strategy on tenant_id columns
- Query performance benchmarking in CI/CD pipeline
- Connection pooling with pgx (200 max connections)
- Read replicas for analytics queries

**Monitoring:**
- Alert on P95 query latency > 100ms
- Alert on connection pool exhaustion

---

### Risk 4: bcrypt Cost Factor Too Low

**Probability:** Low
**Impact:** High (password cracking)
**Mitigation:**
- Minimum cost factor 12 (enforced in code)
- Periodic cost factor increase as hardware improves
- Migration script to rehash passwords on user login

**Monitoring:**
- Audit password hashes for cost factor < 12
- Alert on hash verification latency > 500ms (indicates cost too high)

---

### Risk 5: Session Fixation Attack

**Probability:** Low
**Impact:** Medium (session hijacking)
**Mitigation:**
- Generate new refresh token on login (never accept client-provided tokens)
- Delete all existing sessions on password change
- Concurrent session limit (default 5 per user)

**Monitoring:**
- Alert on session creation without corresponding login event
- Alert on session reuse after logout

---

## Implementation Sequence

### Week 1: Foundation
- Phase 1 (Database Schema) - All tasks
- Phase 2 (Authentication Core) - Tasks 2.1, 2.2

### Week 2: Core Authentication
- Phase 2 - Tasks 2.3, 2.4, 2.5
- Phase 3 (SMTP Auth) - Tasks 3.1, 3.2

### Week 3: Security and Middleware
- Phase 3 - Task 3.3
- Phase 4 (Middleware) - All tasks

### Week 4: RBAC and Testing
- Phase 5 (RBAC) - All tasks
- Phase 7 (Testing) - Task 7.1 (Unit Tests)

### Week 5: Integration and Security Testing
- Phase 6 (Audit Logging) - All tasks
- Phase 7 - Tasks 7.2, 7.3

### Week 6: Documentation and Deployment
- Phase 8 - All tasks
- Production readiness review

---

## Success Criteria

**Functional Completeness:**
- ✅ All EARS requirements implemented
- ✅ API endpoints return correct HTTP status codes
- ✅ SMTP AUTH PLAIN/LOGIN work with major email clients
- ✅ RBAC correctly enforces owner/admin/member privileges

**Security Validation:**
- ✅ Zero OWASP Top 10 vulnerabilities in security scan
- ✅ Cross-tenant access attempts blocked 100% of the time
- ✅ All passwords hashed with bcrypt cost >= 12
- ✅ JWT signature validation prevents tampering

**Performance Benchmarks:**
- ✅ Login endpoint P95 latency < 200ms
- ✅ JWT validation latency < 10ms
- ✅ RLS query overhead < 5% vs non-RLS queries
- ✅ Rate limiting check latency < 20ms

**Quality Gates:**
- ✅ 85%+ overall test coverage
- ✅ 95%+ coverage for auth/authz code paths
- ✅ Zero linter warnings (golangci-lint)
- ✅ Zero security warnings (gosec)
- ✅ All integration tests passing

---

## Dependencies

**External Services:**
- PostgreSQL 14+ with RLS support
- Redis 7+ for rate limiting
- Email service for welcome emails (Task 6.6)

**Internal Dependencies:**
- SPEC-SMTP-001 depends on SMTP AUTH implementation (Phase 3)
- SPEC-API-001 depends on JWT middleware (Phase 4)

**Blocking Issues:**
- RSA key pair generation for production (requires secrets management solution)
- Redis deployment strategy (standalone vs cluster)

---

## Future Enhancements (Out of Scope)

1. **OAuth 2.0 Integration:** Social login with Google, GitHub, Microsoft
2. **API Key Management:** Long-lived API keys for CI/CD and automation
3. **Multi-Factor Authentication:** TOTP-based MFA with backup codes
4. **IP Allowlisting:** Per-tenant CIDR-based access control
5. **Session Analytics:** Dashboard showing active sessions, geographic distribution
6. **Anomaly Detection:** ML-based detection of suspicious login patterns
7. **Password Policies:** Configurable complexity requirements, expiration
8. **Single Sign-On (SSO):** SAML/OIDC integration for enterprise customers
