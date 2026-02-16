# Acceptance Criteria: SPEC-MULTITENANT-001

## Test Scenarios (Given/When/Then Format)

### Scenario 1: User Login with Valid Credentials

**Given** a user exists in the database with:
- Email: alice@example.com
- Password hash: bcrypt hash of "SecurePass123!"
- Role: admin
- Tenant ID: tenant-uuid-1
- Status: active

**When** the client sends POST /api/v1/auth/login with:
```json
{
  "email": "alice@example.com",
  "password": "SecurePass123!"
}
```

**Then** the API SHALL:
- Return HTTP 200 OK
- Return JSON body containing:
  - access_token: Valid JWT signed with RS256, expiry 15 minutes from now
  - refresh_token: Valid JWT signed with RS256, expiry 7 days from now
  - token_type: "Bearer"
  - expires_in: 900 (seconds)
- Create session record in sessions table with refresh_token_hash
- Update users.last_login to current timestamp
- Insert audit_log entry with action="login", result="success"

**Verification:**
```go
// Decode access token and verify claims
claims := parseJWT(response.AccessToken)
assert.Equal(t, "alice@example.com", claims.Email)
assert.Equal(t, "admin", claims.Role)
assert.Equal(t, "tenant-uuid-1", claims.TenantID)
assert.WithinDuration(t, time.Now().Add(15*time.Minute), claims.ExpiresAt, 5*time.Second)

// Verify session created
session, _ := db.Query("SELECT * FROM sessions WHERE user_id = $1", aliceUserID)
assert.NotNil(t, session)
```

---

### Scenario 2: User Login with Invalid Password

**Given** a user exists with email alice@example.com and correct password "SecurePass123!"

**When** the client sends POST /api/v1/auth/login with:
```json
{
  "email": "alice@example.com",
  "password": "WrongPassword"
}
```

**Then** the API SHALL:
- Return HTTP 401 Unauthorized
- Return JSON body:
```json
{
  "error": "invalid_credentials",
  "message": "Invalid email or password"
}
```
- Increment users.failed_attempts by 1
- NOT create a session record
- Insert audit_log entry with action="login", result="failure"

**Verification:**
```go
assert.Equal(t, 401, response.StatusCode)
user, _ := db.Query("SELECT failed_attempts FROM users WHERE email = $1", "alice@example.com")
assert.Equal(t, 1, user.FailedAttempts)
```

---

### Scenario 3: User Login with Brute Force Protection

**Given** a user exists with 5 consecutive failed login attempts in the last 5 minutes

**When** the client sends POST /api/v1/auth/login with valid credentials

**Then** the API SHALL:
- Return HTTP 429 Too Many Requests
- Return JSON body:
```json
{
  "error": "rate_limit_exceeded",
  "message": "Too many failed login attempts. Try again in 5 minutes.",
  "retry_after": 300
}
```
- NOT validate password (return before bcrypt comparison)
- Insert audit_log entry with action="login", result="failure", metadata={"reason": "rate_limited"}

**Verification:**
```go
assert.Equal(t, 429, response.StatusCode)
assert.Equal(t, "300", response.Headers.Get("Retry-After"))
```

---

### Scenario 4: JWT Token Validation on Protected Endpoint

**Given** a valid access token with:
- Claims: {sub: user-uuid-1, tenant_id: tenant-uuid-1, role: admin}
- Expiry: 10 minutes from now
- Signature: Valid RS256 signature

**When** the client sends GET /api/v1/accounts with:
```
Authorization: Bearer <valid-access-token>
```

**Then** the API SHALL:
- Validate JWT signature using public key
- Verify expiry timestamp > current time
- Extract tenant_id and set PostgreSQL session variable
- Execute query with RLS enforcement
- Return HTTP 200 with accounts belonging to tenant-uuid-1 only

**Verification:**
```go
assert.Equal(t, 200, response.StatusCode)
accounts := parseJSON(response.Body)
for _, account := range accounts {
    assert.Equal(t, "tenant-uuid-1", account.TenantID)
}
```

---

### Scenario 5: Expired Access Token with Valid Refresh Token

**Given** a user has:
- Expired access token (exp claim 5 minutes ago)
- Valid refresh token (exp claim 5 days from now)
- Active session record in sessions table

**When** the client sends GET /api/v1/accounts with expired access token

**Then** the API SHALL:
- Return HTTP 401 Unauthorized
- Return JSON body:
```json
{
  "error": "token_expired",
  "message": "Access token expired. Refresh required."
}
```

**When** the client sends POST /api/v1/auth/refresh with:
```json
{
  "refresh_token": "<valid-refresh-token>"
}
```

**Then** the API SHALL:
- Validate refresh token signature and expiry
- Verify session exists in sessions table
- Generate new access token (expiry 15 minutes from now)
- Optionally rotate refresh token if > 24 hours old
- Return HTTP 200 with new tokens

**Verification:**
```go
// First request fails
assert.Equal(t, 401, response1.StatusCode)
assert.Equal(t, "token_expired", response1.JSON["error"])

// Refresh succeeds
assert.Equal(t, 200, response2.StatusCode)
newAccessToken := response2.JSON["access_token"]
assert.NotEmpty(t, newAccessToken)

// Verify new access token works
response3 := client.Get("/api/v1/accounts", newAccessToken)
assert.Equal(t, 200, response3.StatusCode)
```

---

### Scenario 6: SMTP AUTH PLAIN Mechanism with Valid Credentials

**Given** an SMTP account exists with:
- Tenant ID: tenant-uuid-1
- Username: smtp-user-1
- Password hash: bcrypt hash of "SmtpPassword123"
- Status: active

**When** the SMTP client sends:
```
AUTH PLAIN AHNtdHAtdXNlci0xAFNtdHBQYXNzd29yZDEyMw==
```
(base64 encoding of "\0smtp-user-1\0SmtpPassword123")

**Then** the SMTP server SHALL:
- Decode base64 credentials
- Split by NULL bytes to extract username and password
- Query accounts table for username within tenant
- Validate password with bcrypt.CompareHashAndPassword
- Respond with: `235 2.7.0 Authentication successful`
- Update accounts.last_auth_at timestamp
- Insert audit_log entry with action="smtp_auth", result="success"

**Verification:**
```go
conn := connectSMTP("localhost:2525")
conn.StartTLS()
response := conn.Send("AUTH PLAIN AHNtdHAtdXNlci0xAFNtdHBQYXNzd29yZDEyMw==\r\n")
assert.Contains(t, response, "235 2.7.0 Authentication successful")
```

---

### Scenario 7: SMTP AUTH LOGIN Mechanism with Valid Credentials

**Given** an SMTP account exists with username smtp-user-1 and password SmtpPassword123

**When** the SMTP client sends interactive AUTH LOGIN sequence:
```
C: AUTH LOGIN
S: 334 VXNlcm5hbWU6
C: c210cC11c2VyLTE=  (base64 "smtp-user-1")
S: 334 UGFzc3dvcmQ6
C: U210cFBhc3N3b3JkMTIz  (base64 "SmtpPassword123")
S: 235 2.7.0 Authentication successful
```

**Then** the SMTP server SHALL:
- Respond with 334 username prompt after AUTH LOGIN
- Decode base64 username
- Respond with 334 password prompt
- Decode base64 password
- Validate credentials against accounts table
- Respond with 235 on success
- Insert audit_log entry

**Verification:**
```go
conn := connectSMTP("localhost:2525")
conn.StartTLS()
assert.Contains(t, conn.Send("AUTH LOGIN\r\n"), "334 VXNlcm5hbWU6")
assert.Contains(t, conn.Send("c210cC11c2VyLTE=\r\n"), "334 UGFzc3dvcmQ6")
assert.Contains(t, conn.Send("U210cFBhc3N3b3JkMTIz\r\n"), "235")
```

---

### Scenario 8: SMTP AUTH PLAIN with Invalid Credentials

**Given** an SMTP account exists with username smtp-user-1 and password SmtpPassword123

**When** the SMTP client sends:
```
AUTH PLAIN AHNtdHAtdXNlci0xAFdyb25nUGFzc3dvcmQ=
```
(base64 encoding of "\0smtp-user-1\0WrongPassword")

**Then** the SMTP server SHALL:
- Decode credentials
- Validate against database
- Respond with: `535 5.7.8 Authentication credentials invalid`
- Increment failed_attempts for account
- Insert audit_log entry with action="smtp_auth", result="failure"

**Verification:**
```go
conn := connectSMTP("localhost:2525")
conn.StartTLS()
response := conn.Send("AUTH PLAIN AHNtdHAtdXNlci0xAFdyb25nUGFzc3dvcmQ=\r\n")
assert.Contains(t, response, "535 5.7.8 Authentication credentials invalid")
```

---

### Scenario 9: SMTP AUTH Rejected on Unencrypted Connection

**Given** an SMTP client connected without TLS

**When** the client sends:
```
AUTH PLAIN AHNtdHAtdXNlci0xAFNtdHBQYXNzd29yZDEyMw==
```

**Then** the SMTP server SHALL:
- Check TLS status before processing AUTH
- Respond with: `530 5.7.0 Must issue STARTTLS first`
- NOT process credentials
- Insert audit_log entry with action="smtp_auth", result="failure", metadata={"reason": "no_tls"}

**Verification:**
```go
conn := connectSMTP("localhost:2525") // No TLS
response := conn.Send("AUTH PLAIN AHNtdHAtdXNlci0xAFNtdHBQYXNzd29yZDEyMw==\r\n")
assert.Contains(t, response, "530 5.7.0 Must issue STARTTLS first")
```

---

### Scenario 10: RBAC Enforcement - Owner Role Operations

**Given** a user with role="owner" and tenant_id="tenant-uuid-1"

**When** the user sends DELETE /api/v1/tenants/tenant-uuid-1 with valid JWT

**Then** the API SHALL:
- Extract role from JWT claims
- Verify role == "owner"
- Check tenant_id match
- Execute tenant deletion with CASCADE
- Return HTTP 204 No Content

**Verification:**
```go
assert.Equal(t, 204, response.StatusCode)
_, err := db.Query("SELECT * FROM tenants WHERE id = $1", "tenant-uuid-1")
assert.Error(t, err) // Tenant deleted
```

---

### Scenario 11: RBAC Enforcement - Admin Role Denied Tenant Deletion

**Given** a user with role="admin" and tenant_id="tenant-uuid-1"

**When** the user sends DELETE /api/v1/tenants/tenant-uuid-1 with valid JWT

**Then** the API SHALL:
- Extract role from JWT claims
- Verify role != "owner"
- Return HTTP 403 Forbidden
- Return JSON body:
```json
{
  "error": "insufficient_privileges",
  "message": "Only tenant owners can delete tenants",
  "required_role": "owner",
  "current_role": "admin"
}
```
- NOT execute deletion
- Insert audit_log entry with result="failure"

**Verification:**
```go
assert.Equal(t, 403, response.StatusCode)
tenant, _ := db.Query("SELECT * FROM tenants WHERE id = $1", "tenant-uuid-1")
assert.NotNil(t, tenant) // Tenant still exists
```

---

### Scenario 12: RBAC Enforcement - Member Role Read-Only Access

**Given** a user with role="member" and tenant_id="tenant-uuid-1"

**When** the user sends POST /api/v1/accounts with valid JWT

**Then** the API SHALL:
- Extract role from JWT claims
- Verify role in ["owner", "admin"]
- Return HTTP 403 Forbidden

**When** the user sends GET /api/v1/dashboard with valid JWT

**Then** the API SHALL:
- Extract role from JWT claims
- Allow access (no role restriction on read-only endpoints)
- Return HTTP 200 with dashboard data

**Verification:**
```go
// Write operation denied
postResponse := client.Post("/api/v1/accounts", validJWT, accountData)
assert.Equal(t, 403, postResponse.StatusCode)

// Read operation allowed
getResponse := client.Get("/api/v1/dashboard", validJWT)
assert.Equal(t, 200, getResponse.StatusCode)
```

---

### Scenario 13: Cross-Tenant Isolation Verification

**Given** two tenants exist:
- Tenant A (tenant-uuid-1) with user Alice (user-uuid-1)
- Tenant B (tenant-uuid-2) with user Bob (user-uuid-2)

**When** Alice sends GET /api/v1/accounts with her valid JWT (tenant_id="tenant-uuid-1")

**Then** the API SHALL:
- Set PostgreSQL session variable: `app.current_tenant_id = tenant-uuid-1`
- Execute query with RLS enforcement
- Return only accounts where tenant_id = "tenant-uuid-1"
- NOT return accounts from tenant-uuid-2

**Verification:**
```go
aliceAccounts := client.Get("/api/v1/accounts", aliceJWT)
assert.Equal(t, 200, aliceAccounts.StatusCode)
for _, account := range aliceAccounts.JSON {
    assert.Equal(t, "tenant-uuid-1", account["tenant_id"])
}

// Verify Bob's accounts not included
bobAccountIDs := getAccountIDs("tenant-uuid-2")
aliceAccountIDs := extractIDs(aliceAccounts.JSON)
assert.Empty(t, intersection(bobAccountIDs, aliceAccountIDs))
```

---

### Scenario 14: Rate Limiting Enforcement

**Given** a tenant account with rate limit 100 emails/hour

**When** the SMTP client sends 101 MAIL FROM commands within 60 minutes

**Then** the SMTP server SHALL:
- Accept first 100 commands
- Track count in Redis with key: `ratelimit:tenant-uuid-1:account-uuid-1:{hour}`
- Reject 101st command with: `421 4.7.0 Rate limit exceeded. Try again later.`
- Include `Retry-After: 3600` header (seconds until window reset)
- Insert audit_log entry with action="rate_limit", result="exceeded"

**Verification:**
```go
for i := 1; i <= 100; i++ {
    response := smtpClient.Send(fmt.Sprintf("MAIL FROM:<%d@example.com>\r\n", i))
    assert.Contains(t, response, "250 OK")
}

response101 := smtpClient.Send("MAIL FROM:<overflow@example.com>\r\n")
assert.Contains(t, response101, "421 4.7.0 Rate limit exceeded")
assert.Contains(t, response101, "Retry-After: 3600")
```

---

### Scenario 15: Session Invalidation on Role Update

**Given** a user Alice has:
- Role: admin
- Active sessions: 3 (web, mobile, API key)

**When** owner updates Alice's role to "member" via PATCH /api/v1/users/alice-uuid/role

**Then** the API SHALL:
- Update users.role to "member"
- Execute: `DELETE FROM sessions WHERE user_id = 'alice-uuid'`
- Return HTTP 200

**When** Alice tries to use old access token

**Then** the API SHALL:
- Validate token signature (passes)
- Check session existence by refresh token (fails - deleted)
- Return HTTP 401 Unauthorized with error "session_invalidated"

**Verification:**
```go
// Before role update
sessions, _ := db.Query("SELECT COUNT(*) FROM sessions WHERE user_id = $1", aliceUUID)
assert.Equal(t, 3, sessions.Count)

// Update role
response := ownerClient.Patch("/api/v1/users/alice-uuid/role", `{"role": "member"}`)
assert.Equal(t, 200, response.StatusCode)

// After role update
sessions, _ = db.Query("SELECT COUNT(*) FROM sessions WHERE user_id = $1", aliceUUID)
assert.Equal(t, 0, sessions.Count)

// Old token fails
accountsResponse := client.Get("/api/v1/accounts", aliceOldAccessToken)
assert.Equal(t, 401, accountsResponse.StatusCode)
```

---

### Scenario 16: Password Change Flow

**Given** a user Alice with current password "OldPassword123"

**When** Alice sends POST /api/v1/auth/change-password with:
```json
{
  "current_password": "OldPassword123",
  "new_password": "NewSecurePass456!",
  "new_password_confirm": "NewSecurePass456!"
}
```

**Then** the API SHALL:
- Validate current_password with bcrypt
- Validate new_password strength (min 12 chars, complexity requirements)
- Validate new_password == new_password_confirm
- Generate new bcrypt hash with cost 12
- Update users.password_hash
- Delete all sessions for user (force re-authentication)
- Return HTTP 200

**Verification:**
```go
assert.Equal(t, 200, response.StatusCode)

// Verify old password no longer works
loginResponse := client.Post("/api/v1/auth/login", `{"email": "alice@example.com", "password": "OldPassword123"}`)
assert.Equal(t, 401, loginResponse.StatusCode)

// Verify new password works
newLoginResponse := client.Post("/api/v1/auth/login", `{"email": "alice@example.com", "password": "NewSecurePass456!"}`)
assert.Equal(t, 200, newLoginResponse.StatusCode)

// Verify all sessions invalidated
sessions, _ := db.Query("SELECT COUNT(*) FROM sessions WHERE user_id = $1", aliceUUID)
assert.Equal(t, 1, sessions.Count) // Only new session from newLoginResponse
```

---

## Edge Cases

### Edge Case 1: Concurrent Login Sessions

**Given** a user logs in from 3 different devices simultaneously

**When** all 3 login requests arrive within 1 second

**Then** the API SHALL:
- Create 3 separate session records
- Generate 3 unique refresh tokens
- All 3 sessions remain valid concurrently (up to session limit)

**Verification:**
```go
var wg sync.WaitGroup
for i := 0; i < 3; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        response := client.Post("/api/v1/auth/login", credentials)
        assert.Equal(t, 200, response.StatusCode)
    }()
}
wg.Wait()

sessions, _ := db.Query("SELECT COUNT(*) FROM sessions WHERE user_id = $1", userUUID)
assert.Equal(t, 3, sessions.Count)
```

---

### Edge Case 2: Token Expiry Boundary

**Given** an access token with exp claim exactly equal to current timestamp

**When** the client sends request with this token

**Then** the API SHALL:
- Treat token as expired (exp <= now is invalid)
- Return HTTP 401 Unauthorized

**Verification:**
```go
claims := jwt.MapClaims{
    "sub": "user-uuid-1",
    "exp": time.Now().Unix(), // Exactly now
}
token := generateJWT(claims)

response := client.Get("/api/v1/accounts", token)
assert.Equal(t, 401, response.StatusCode)
```

---

### Edge Case 3: SMTP AUTH Base64 Decoding Failure

**Given** an SMTP client sends malformed AUTH PLAIN command with invalid base64

**When** the client sends:
```
AUTH PLAIN InvalidBase64!@#$
```

**Then** the SMTP server SHALL:
- Attempt base64 decoding
- Catch decoding error
- Respond with: `501 5.5.2 Syntax error in authentication credentials`
- Insert audit_log entry with result="failure", metadata={"reason": "base64_decode_error"}

**Verification:**
```go
conn := connectSMTP("localhost:2525")
conn.StartTLS()
response := conn.Send("AUTH PLAIN InvalidBase64!@#$\r\n")
assert.Contains(t, response, "501 5.5.2 Syntax error")
```

---

## Security Test Scenarios

### Security Test 1: SQL Injection Attempt on Login

**Given** an attacker attempts SQL injection

**When** the client sends POST /api/v1/auth/login with:
```json
{
  "email": "admin'--",
  "password": "anything"
}
```

**Then** the API SHALL:
- Use parameterized query (NOT string concatenation)
- Query: `SELECT * FROM users WHERE email = $1` with parameter "admin'--"
- Return 401 Unauthorized (no user found)
- NOT expose database error details
- Insert audit_log with action="login", result="failure"

**Verification:**
```go
response := client.Post("/api/v1/auth/login", maliciousPayload)
assert.Equal(t, 401, response.StatusCode)
assert.NotContains(t, response.Body, "SQL") // No SQL errors leaked
assert.NotContains(t, response.Body, "database")
```

---

### Security Test 2: JWT Token Tampering

**Given** an attacker obtains a valid JWT and modifies the role claim

**When** the attacker changes role from "member" to "owner" and sends request

**Then** the API SHALL:
- Verify JWT signature using public key
- Detect signature mismatch (signature computed with original claims)
- Return HTTP 401 Unauthorized with error "invalid_token_signature"
- NOT process the request
- Insert audit_log with action="jwt_validation", result="failure"

**Verification:**
```go
validToken := generateValidToken(role: "member")
parts := strings.Split(validToken, ".")
header := parts[0]
payload := base64Decode(parts[1])

// Tamper with payload
tamperedPayload := strings.Replace(payload, `"role":"member"`, `"role":"owner"`, 1)
tamperedToken := header + "." + base64Encode(tamperedPayload) + "." + parts[2]

response := client.Delete("/api/v1/tenants/tenant-uuid-1", tamperedToken)
assert.Equal(t, 401, response.StatusCode)
assert.Equal(t, "invalid_token_signature", response.JSON["error"])
```

---

### Security Test 3: Cross-Tenant Access Attempt

**Given** user Alice from tenant-uuid-1 obtains account-uuid-2 belonging to tenant-uuid-2

**When** Alice sends DELETE /api/v1/accounts/account-uuid-2 with her valid JWT

**Then** the API SHALL:
- Set RLS context: `app.current_tenant_id = tenant-uuid-1`
- Execute DELETE query with RLS enforcement
- PostgreSQL filters account-uuid-2 (different tenant)
- Return HTTP 404 Not Found (account not found in user's tenant)
- NOT execute deletion
- Insert audit_log with action="delete_account", result="failure", metadata={"reason": "not_found"}

**Verification:**
```go
response := aliceClient.Delete("/api/v1/accounts/account-uuid-2", aliceJWT)
assert.Equal(t, 404, response.StatusCode)

// Verify account still exists in tenant B
account, _ := systemDB.Query("SELECT * FROM accounts WHERE id = $1", "account-uuid-2")
assert.NotNil(t, account)
```

---

### Security Test 4: Brute Force Password Guessing

**Given** an attacker attempts to guess passwords

**When** the attacker sends 10 failed login attempts within 1 minute

**Then** the API SHALL:
- Accept first 5 attempts (increment failed_attempts)
- Apply exponential backoff starting at attempt 6:
  - Attempt 6: Wait 1 second before response
  - Attempt 7: Wait 2 seconds
  - Attempt 8: Wait 4 seconds
  - Attempt 9: Wait 8 seconds
  - Attempt 10: Return 429 Too Many Requests
- NOT validate password after threshold
- Require 15-minute cooldown period

**Verification:**
```go
for i := 1; i <= 10; i++ {
    start := time.Now()
    response := client.Post("/api/v1/auth/login", invalidCredentials)
    duration := time.Since(start)

    if i <= 5 {
        assert.Equal(t, 401, response.StatusCode)
        assert.Less(t, duration, 500*time.Millisecond)
    } else if i <= 9 {
        expectedDelay := time.Duration(math.Pow(2, float64(i-6))) * time.Second
        assert.WithinDuration(t, expectedDelay, duration, 100*time.Millisecond)
    } else {
        assert.Equal(t, 429, response.StatusCode)
    }
}
```

---

### Security Test 5: Session Fixation Prevention

**Given** an attacker obtains a pre-authenticated session token

**When** a user logs in with valid credentials

**Then** the API SHALL:
- Generate NEW refresh token (never accept client-provided tokens)
- Create NEW session record with new token
- NOT reuse any existing session tokens
- Delete old sessions if over concurrent limit

**Verification:**
```go
// Attacker creates pre-auth token
preAuthToken := "attacker-controlled-token"

// User logs in (attacker cannot inject their token)
loginRequest := `{"email": "alice@example.com", "password": "SecurePass123!"}`
response := client.Post("/api/v1/auth/login", loginRequest)

newRefreshToken := response.JSON["refresh_token"]
assert.NotEqual(t, preAuthToken, newRefreshToken)

// Verify new token is cryptographically random
assert.Len(t, newRefreshToken, 32) // 256-bit token
assert.Regexp(t, `^[a-f0-9]{64}$`, newRefreshToken) // Hex encoding
```

---

## Quality Gate Criteria

### Code Coverage Requirements

**Overall Coverage:**
- ✅ Minimum 85% line coverage across all packages
- ✅ Minimum 80% branch coverage for conditional logic

**Critical Path Coverage:**
- ✅ 95%+ coverage for authentication handlers (login, refresh, logout)
- ✅ 95%+ coverage for authorization middleware (JWT validation, RBAC)
- ✅ 95%+ coverage for SMTP AUTH mechanisms (PLAIN, LOGIN)
- ✅ 90%+ coverage for RLS context setting middleware
- ✅ 90%+ coverage for rate limiting logic

---

### Security Validation

**Static Analysis:**
- ✅ Zero findings from `gosec` security scanner
- ✅ Zero findings from `govulncheck` vulnerability scanner
- ✅ Zero high/critical findings from Snyk dependency scan

**Penetration Testing:**
- ✅ SQL injection attempts blocked on all input fields
- ✅ JWT tampering detected and rejected
- ✅ Cross-tenant access attempts return 404 (not 403 to avoid enumeration)
- ✅ Brute force protection enforced after 5 attempts
- ✅ TLS enforcement prevents plaintext AUTH

---

### Performance Benchmarks

**API Latency (P95):**
- ✅ POST /api/v1/auth/login: < 200ms (including bcrypt)
- ✅ POST /api/v1/auth/refresh: < 50ms
- ✅ GET /api/v1/accounts (with RLS): < 100ms
- ✅ JWT middleware validation: < 10ms per request

**SMTP Performance:**
- ✅ AUTH PLAIN validation: < 150ms (including bcrypt)
- ✅ AUTH LOGIN validation: < 200ms (2 round-trips + bcrypt)
- ✅ Rate limit check (Redis): < 20ms

**Database Performance:**
- ✅ RLS query overhead: < 5% vs non-RLS equivalent
- ✅ Session lookup by refresh token: < 10ms (indexed)
- ✅ Audit log insertion: < 30ms (async, non-blocking)

---

### Functional Completeness

**Authentication:**
- ✅ User login with bcrypt password validation
- ✅ JWT access token generation (RS256, 15min expiry)
- ✅ JWT refresh token generation (RS256, 7day expiry)
- ✅ Token refresh flow with optional rotation
- ✅ Logout with session deletion
- ✅ SMTP AUTH PLAIN mechanism
- ✅ SMTP AUTH LOGIN mechanism
- ✅ TLS enforcement for SMTP AUTH

**Authorization:**
- ✅ RBAC middleware with owner/admin/member roles
- ✅ Owner-only operations (tenant deletion, billing)
- ✅ Admin operations (account CRUD, provider config)
- ✅ Member read-only access
- ✅ Session invalidation on role change

**Tenant Isolation:**
- ✅ PostgreSQL RLS policies on all tenant-scoped tables
- ✅ Tenant context setting via session variable
- ✅ Cross-tenant access blocked at database level
- ✅ 100% isolation verified with integration tests

**Rate Limiting:**
- ✅ Redis-based sliding window algorithm
- ✅ Per-tenant-account enforcement
- ✅ 421 response with Retry-After header on limit exceeded
- ✅ Rate limit reset after time window

**Audit Logging:**
- ✅ Login attempts (success/failure) logged
- ✅ Authorization failures logged
- ✅ Administrative actions logged (role changes, deletions)
- ✅ SMTP AUTH attempts logged
- ✅ Rate limit violations logged

---

### Error Handling

**Graceful Degradation:**
- ✅ Redis failure: Fallback to database-backed rate limiting
- ✅ Database connection failure: Return 503 Service Unavailable
- ✅ JWT key loading failure: Fail fast on startup with clear error

**User-Friendly Error Messages:**
- ✅ Generic "Invalid email or password" (no username enumeration)
- ✅ Specific error codes for programmatic handling
- ✅ No sensitive data in error responses (passwords, tokens, SQL)

---

## Definition of Done

An implementation is considered **COMPLETE** when:

1. ✅ All 16 test scenarios pass in automated test suite
2. ✅ All 5 edge cases handled correctly
3. ✅ All 5 security tests pass (no vulnerabilities)
4. ✅ Code coverage meets or exceeds thresholds (85% overall, 95% critical paths)
5. ✅ All performance benchmarks met (P95 latencies within targets)
6. ✅ Zero security warnings from gosec, govulncheck, Snyk
7. ✅ Zero linter warnings from golangci-lint
8. ✅ All database migrations run successfully on PostgreSQL 14+
9. ✅ Redis integration tested with actual Redis instance
10. ✅ API documentation (OpenAPI 3.0) generated and accurate
11. ✅ Deployment configuration documented (RSA keys, Redis, PostgreSQL)
12. ✅ Security review completed by security-expert agent
13. ✅ Manual testing with real SMTP clients (Thunderbird, Outlook, Gmail)
14. ✅ Audit logs verified for completeness and accuracy
15. ✅ RBAC roles tested with real user workflows

---

**Final Approval Checklist:**

- [ ] Product Owner sign-off on acceptance criteria
- [ ] Security Expert review completed with no blockers
- [ ] DevOps team confirms deployment readiness
- [ ] QA team confirms all test scenarios passing
- [ ] Documentation review completed
- [ ] Production secrets management strategy defined (RSA keys)
- [ ] Monitoring alerts configured (failed logins, rate limits, errors)
- [ ] Rollback plan documented and tested

---

**Traceability:**
- SPEC-MULTITENANT-001: spec.md (requirements source)
- SPEC-MULTITENANT-001: plan.md (implementation tasks)
- Git Branch: feature/multitenant-auth
- Test Coverage Report: coverage/auth-{date}.html
