---
id: SPEC-RESTRUCTURE-001
title: Full Architecture Restructuring - Acceptance Criteria
version: 0.2.0
status: draft
created: 2026-02-25
updated: 2026-02-25
author: sungwon
---

## Acceptance Criteria

All scenarios use Given-When-Then format. Minimum 2 scenarios per module with edge cases.

---

## Module 1: Message Storage Interface

### Scenario MSI-1.1: Store and retrieve message body via LocalFileStore

```gherkin
Given the STORAGE_TYPE is set to "local"
  And the STORAGE_PATH is set to "/tmp/test-messages"
  And the LocalFileStore is initialized
When Put is called with messageID "msg-001" and data "Hello World email body"
Then the file "/tmp/test-messages/msg-001" shall exist on disk
  And Get called with messageID "msg-001" shall return "Hello World email body"
```

### Scenario MSI-1.2: Retrieve non-existent message from LocalFileStore

```gherkin
Given the LocalFileStore is initialized with STORAGE_PATH "/tmp/test-messages"
  And no file exists for messageID "msg-nonexistent"
When Get is called with messageID "msg-nonexistent"
Then the operation shall return ErrNotFound
  And no panic or unexpected error shall occur
```

### Scenario MSI-1.3: Delete message body via LocalFileStore

```gherkin
Given the LocalFileStore contains a file for messageID "msg-002"
When Delete is called with messageID "msg-002"
Then the file for "msg-002" shall no longer exist on disk
  And subsequent Get for "msg-002" shall return ErrNotFound
```

### Scenario MSI-1.4: Delete non-existent message is idempotent

```gherkin
Given the LocalFileStore does not contain a file for messageID "msg-ghost"
When Delete is called with messageID "msg-ghost"
Then the operation shall return nil (no error)
```

### Scenario MSI-1.5: LocalFileStore auto-creates base directory

```gherkin
Given the STORAGE_PATH is set to "/tmp/test-nested/deep/path"
  And the directory "/tmp/test-nested/deep/path" does not exist
When the LocalFileStore is initialized
Then the directory "/tmp/test-nested/deep/path" shall be created
  And Put operations shall succeed without error
```

### Scenario MSI-1.6: S3Store stores and retrieves message body

```gherkin
Given the STORAGE_TYPE is set to "s3"
  And the S3_BUCKET is "test-bucket"
  And the S3_PREFIX is "messages/"
  And the S3_ENDPOINT points to a MinIO instance
When Put is called with messageID "msg-s3-001" and data containing email body
Then the object "messages/msg-s3-001" shall exist in "test-bucket"
  And Get called with messageID "msg-s3-001" shall return the stored data
```

### Scenario MSI-1.7: S3Store handles non-existent object

```gherkin
Given the S3Store is initialized with a valid bucket
  And no object exists for key "messages/msg-s3-missing"
When Get is called with messageID "msg-s3-missing"
Then the operation shall return ErrNotFound
```

### Scenario MSI-1.8: Default to local storage when STORAGE_TYPE is unset

```gherkin
Given the STORAGE_TYPE environment variable is not set
When the MessageStore factory function is called
Then a LocalFileStore instance shall be created
  And a warning log message shall be emitted indicating default storage type
```

### Scenario MSI-1.9: Reject unsupported STORAGE_TYPE

```gherkin
Given the STORAGE_TYPE is set to "gcs"
When the MessageStore factory function is called
Then a LocalFileStore instance shall be created as fallback
  And a warning log message shall be emitted: "unsupported storage type 'gcs', defaulting to local"
```

---

## Module 2: SMTP Server Refactoring

### Scenario SMTP-2.1: Successful message submission with storage

```gherkin
Given the SMTP server is running with LocalFileStore configured
  And a valid account "user1" with password "pass1" exists
  And the account has allowed sender domain "example.com"
When an SMTP client connects and authenticates as "user1"/"pass1"
  And sends MAIL FROM:<sender@example.com>
  And sends RCPT TO:<recipient@test.com>
  And sends DATA with a valid email message body
Then the server shall:
  1. Insert a metadata record into PostgreSQL with status "queued"
  2. Store the full message body in MessageStore
  3. Enqueue only the message_id to Redis Streams
  4. Return SMTP 250 OK with the message_id
  And the PostgreSQL messages record shall NOT contain the message body
  And the Redis queue entry shall contain only id, account_id, and tenant_id
```

### Scenario SMTP-2.2: Storage failure during DATA processing

```gherkin
Given the SMTP server is running
  And the MessageStore is configured but the storage backend is unavailable (disk full / S3 down)
  And a valid authenticated SMTP session exists
When the client sends DATA with a valid email message
  And the metadata is successfully inserted into PostgreSQL
  And the MessageStore Put operation fails
Then the server shall:
  1. Mark the metadata record as "failed" or delete it
  2. Return SMTP 451 "Requested action aborted: local error in processing"
  3. Log the storage failure with message_id and error details
  And no message_id shall be enqueued to Redis
```

### Scenario SMTP-2.3: Redis enqueue failure after successful storage

```gherkin
Given the SMTP server is running
  And the MessageStore is operational
  And Redis is temporarily unavailable
  And a valid authenticated SMTP session exists
When the client sends DATA with a valid email message
  And the metadata is inserted into PostgreSQL
  And the message body is stored in MessageStore
  And the Redis XADD operation fails
Then the server shall:
  1. Update the message status to "enqueue_failed" in PostgreSQL
  2. Return SMTP 451 temporary error
  3. Log the enqueue failure for manual recovery
  And the message body shall remain in storage (not deleted)
```

### Scenario SMTP-2.4: Sync delivery mode is removed

```gherkin
Given the SMTP server codebase
When inspecting the delivery package
Then the SyncService implementation shall not exist
  And no configuration option for sync/inline delivery shall be available
  And only async (queue-based) delivery shall be supported
```

### Scenario SMTP-2.5: Message metadata does not contain body

```gherkin
Given a message has been successfully submitted via SMTP
When querying the PostgreSQL messages table for the submitted message_id
Then the result shall contain: message_id, account_id, tenant_id, sender, recipients, subject, headers, status, created_at, updated_at
  And the result shall NOT contain a body/content column with email content
  And a storage_ref column shall reference the storage location
```

---

## Module 3: Queue Worker Refactoring

### Scenario QW-3.1: Successful message delivery via storage fetch

```gherkin
Given the queue worker is running with LocalFileStore configured
  And a message_id "msg-deliver-001" exists in PostgreSQL with status "queued"
  And the message body for "msg-deliver-001" exists in MessageStore
  And a valid ESP provider is configured for the message's account
When the worker dequeues "msg-deliver-001" from Redis Streams
Then the worker shall:
  1. Fetch metadata from PostgreSQL
  2. Fetch message body from MessageStore
  3. Resolve the ESP provider for the account
  4. Deliver the message via the provider
  5. Update status to "delivered" in PostgreSQL
  6. Log a delivery record in delivery_logs
```

### Scenario QW-3.2: Storage read failure with retry and DLQ

```gherkin
Given the queue worker is running
  And a message_id "msg-fail-storage" exists in PostgreSQL
  And the message body for "msg-fail-storage" is NOT available in MessageStore
When the worker dequeues "msg-fail-storage" from Redis Streams
Then the worker shall:
  1. Attempt to fetch from MessageStore (attempt 1) - fail
  2. Wait 1 second and retry (attempt 2) - fail
  3. Wait 2 seconds and retry (attempt 3) - fail
  4. Wait 4 seconds and retry (attempt 4 - final) - fail
  5. Mark the message status as "storage_error" in PostgreSQL
  6. Route the message to the dead letter queue
  7. Log the storage failure with full context including all retry attempts
```

### Scenario QW-3.3: Orphaned message_id in queue

```gherkin
Given the queue worker is running
  And a message_id "msg-orphan-001" is in the Redis queue
  And NO record exists for "msg-orphan-001" in PostgreSQL
When the worker dequeues "msg-orphan-001"
Then the worker shall:
  1. Log a warning: "orphaned message_id msg-orphan-001 not found in database"
  2. Acknowledge the Redis message (remove from queue)
  3. NOT attempt storage fetch or ESP delivery
  And no error shall be thrown
```

### Scenario QW-3.4: Backward compatibility with old queue format

```gherkin
Given the queue worker is running with backward compatibility enabled
  And a message in old format exists in Redis with fields: id, tenant_id, from, to, subject, headers, body
When the worker dequeues this old-format message
Then the worker shall:
  1. Detect the presence of the "body" field in the message
  2. Use the inline body directly (skip MessageStore fetch)
  3. Deliver via ESP provider as before
  4. Update delivery status in PostgreSQL
  And the delivery shall succeed identically to the new flow
```

### Scenario QW-3.5: ESP delivery failure (existing behavior preserved)

```gherkin
Given the queue worker is running
  And a message_id exists in PostgreSQL and MessageStore
  And the resolved ESP provider returns a transient error
When the worker attempts delivery
Then the worker shall:
  1. Apply existing exponential backoff retry policy
  2. Retry delivery according to configured max_retries
  3. If all retries fail, route to dead letter queue
  4. Update message status appropriately at each step
  And the retry behavior shall be identical to pre-restructure behavior
```

---

## Module 4: Admin Interface (FastAPI + React)

### Scenario ADMIN-4.1: Successful admin login

```gherkin
Given the FastAPI admin backend is running
  And a user "admin@example.com" with role "admin" and valid password exists
When a POST request is sent to /auth/login with email "admin@example.com" and correct password
Then the response shall be HTTP 200
  And the response body shall contain:
    - access_token (JWT with user_id, role, exp claims)
    - refresh_token
    - token_type "bearer"
  And the access_token shall be valid for the configured expiration period
```

### Scenario ADMIN-4.2: Unauthorized access without token

```gherkin
Given the FastAPI admin backend is running
When a GET request is sent to /accounts without an Authorization header
Then the response shall be HTTP 401 Unauthorized
  And the response body shall contain error message "Not authenticated"
```

### Scenario ADMIN-4.3: RBAC enforcement - viewer cannot create

```gherkin
Given the FastAPI admin backend is running
  And a user with role "viewer" has a valid JWT token
When a POST request is sent to /tenants with tenant creation data and the viewer's token
Then the response shall be HTTP 403 Forbidden
  And the response body shall contain error message "Insufficient permissions"
  And no tenant shall be created in the database
```

### Scenario ADMIN-4.4: CRUD operations for accounts

```gherkin
Given the FastAPI admin backend is running
  And an authenticated user with role "owner"
When the user sends POST /accounts with name "test-account" and domain "test.com"
Then the response shall be HTTP 201 with the created account data

When the user sends GET /accounts
Then the response shall be HTTP 200 with a list containing "test-account"
  And the response shall include pagination metadata (total, page, per_page)

When the user sends PUT /accounts/{id} with updated name "updated-account"
Then the response shall be HTTP 200 with the updated account data

When the user sends DELETE /accounts/{id}
Then the response shall be HTTP 204
  And subsequent GET /accounts/{id} shall return HTTP 404
```

### Scenario ADMIN-4.5: Delivery logs with filtering and pagination

```gherkin
Given the FastAPI admin backend is running
  And 150 delivery log records exist in the database
  And an authenticated user with role "admin"
When GET /delivery-logs?page=1&per_page=20&status=delivered is sent
Then the response shall be HTTP 200
  And the response body shall contain at most 20 records
  And all returned records shall have status "delivered"
  And pagination metadata shall indicate total matching records
```

### Scenario ADMIN-4.6: Token refresh flow

```gherkin
Given the FastAPI admin backend is running
  And a user has an expired access_token but valid refresh_token
When POST /auth/refresh is sent with the refresh_token
Then the response shall be HTTP 200
  And a new access_token shall be returned
  And the old access_token shall no longer be accepted
```

### Scenario ADMIN-4.7: React frontend login and dashboard

```gherkin
Given the React admin frontend is served and accessible at port 3000
  And the FastAPI backend is running at port 8000
When a user navigates to the login page
  And enters valid credentials and clicks Login
Then the user shall be redirected to the Dashboard page
  And the Dashboard shall display:
    - Total messages sent (metric card)
    - Delivery success rate (metric card)
    - Queue depth (metric card)
    - Delivery trend chart (line chart)
  And the navigation sidebar shall show links based on the user's role
```

### Scenario ADMIN-4.8: React frontend role-based navigation

```gherkin
Given a user with role "viewer" is logged into the React frontend
When the user views the navigation sidebar
Then the following items shall be visible: Dashboard, Tenants (read-only), Delivery Logs
  And the following items shall be hidden or disabled: Accounts, Users, Providers, Routing Rules (write actions)
  And attempting to navigate to /accounts directly shall show "Access Denied" or redirect to Dashboard
```

### Scenario ADMIN-4.9: Admin backend does not send emails

```gherkin
Given the FastAPI admin backend is running
When any CRUD operation is performed on any resource
Then no email shall be sent via any ESP provider
  And no interaction with Redis Streams shall occur
  And only PostgreSQL read/write operations shall be performed
```

---

## Module 7: Delivery Logging, Retry & Observability

### Scenario LOG-7.1: Enqueue retry succeeds on second attempt

```gherkin
Given the SMTP server is running
  And a valid authenticated session has persisted metadata and stored body
  And Redis is temporarily slow (first XADD times out)
When the SMTP server attempts to enqueue the message_id
Then the first enqueue attempt shall fail
  And the system shall wait 500ms and retry (attempt 2)
  And the second attempt shall succeed
  And the SMTP server shall return 250 OK
  And the application log shall contain 2 entries:
    - attempt 1: WARN with message_id, error reason
    - attempt 2: INFO with message_id, "enqueue succeeded on retry 2"
```

### Scenario LOG-7.2: Enqueue retry exhausted after 3 attempts

```gherkin
Given the SMTP server is running
  And Redis is completely unavailable
When the SMTP server attempts to enqueue the message_id
Then the system shall:
  1. Attempt 1 - fail, wait 500ms
  2. Attempt 2 - fail, wait 1s
  3. Attempt 3 - fail, wait 2s (all retries exhausted)
  4. Mark message as "enqueue_failed" in PostgreSQL
  5. Return SMTP 451 to client
  And 3 WARN log entries and 1 ERROR log entry shall be recorded
  And each log entry shall include: message_id, attempt_number, error_reason
```

### Scenario LOG-7.3: Delivery success logged to DB and application log

```gherkin
Given the queue worker delivers a message successfully via SendGrid
  And the delivery took 1200ms
When the delivery completes
Then a record shall be inserted into delivery_logs with:
  - message_id: the delivered message ID
  - provider_name: "sendgrid"
  - status: "delivered"
  - duration_ms: 1200
  - attempt_number: 1
  And a structured JSON log entry shall be written at INFO level with:
    - message_id, account_id, provider: "sendgrid", status: "delivered", duration_ms: 1200
```

### Scenario LOG-7.4: Delivery failure logged with full error context

```gherkin
Given the queue worker attempts to deliver a message via SES
  And the SES API returns a 503 Service Unavailable error
When the delivery attempt fails
Then a record shall be inserted into delivery_logs with:
  - status: "failed"
  - provider_name: "ses"
  - error_message: "503 Service Unavailable"
  - attempt_number: (current attempt number)
  And a structured JSON log entry shall be written at ERROR level with:
    - message_id, account_id, provider: "ses", status: "failed"
    - error_type, error_message, attempt_number, duration_ms
```

### Scenario LOG-7.5: DLQ routing logged with complete retry history

```gherkin
Given the queue worker has exhausted all 5 retries for a message
  And each retry failed with different errors
When the message is routed to the dead letter queue
Then a delivery_logs record shall be inserted with:
  - status: "dlq"
  - attempt_number: 5
  - error_message: "max retries exhausted"
  And a WARN log entry shall include:
    - message_id, total_retries: 5, all error reasons from each attempt
```

### Scenario LOG-7.6: Log output to file with rotation

```gherkin
Given LOG_OUTPUT is set to "file"
  And LOG_FILE_PATH is set to "/var/log/smtp-proxy.log"
  And LOG_MAX_SIZE_MB is set to 100
  And LOG_MAX_FILES is set to 10
When the application starts and processes messages
Then logs shall be written to "/var/log/smtp-proxy.log" in JSON format
  And when the file reaches 100MB, it shall be rotated
  And at most 10 rotated files shall be retained
  And no logs shall be written to stdout
```

### Scenario LOG-7.7: Log output to CloudWatch

```gherkin
Given LOG_OUTPUT is set to "cloudwatch"
  And LOG_CW_GROUP is set to "/smtp-proxy/production"
  And LOG_CW_STREAM is set to "smtp-server-001"
  And valid AWS credentials are available
When the application starts and processes messages
Then structured JSON log entries shall be sent to CloudWatch Logs
  And the log group "/smtp-proxy/production" shall receive entries
  And the log stream "smtp-server-001" shall contain the entries
  And logs shall also be viewable via AWS CloudWatch console
```

### Scenario LOG-7.8: Default log output is stdout

```gherkin
Given LOG_OUTPUT is not set in the environment
When the application starts
Then logs shall be written to stdout in JSON format (default behavior)
  And no log files shall be created
  And no CloudWatch calls shall be made
```

### Scenario LOG-7.9: Statistics not stored as application logs

```gherkin
Given the system is processing messages
When delivery results are recorded
Then delivery statistics (counts, rates, durations) shall be stored in PostgreSQL delivery_logs table
  And aggregate statistics shall be queryable via SQL
  And full application logs (debug, info, warn, error traces) shall NOT be stored in PostgreSQL
  And application logs shall only go to the configured LOG_OUTPUT target
```

---

## Module 5: Service Boundaries and Deployment

### Scenario DEPLOY-5.1: Docker Compose full stack startup

```gherkin
Given the docker-compose.yml is configured with all services
  And the .env file contains valid DATABASE_URL and REDIS_URL
When "docker compose up -d --build" is executed
Then all services shall start successfully:
  - postgres: healthy on port 5432
  - redis: healthy on port 6379
  - smtp-server: healthy on port 2525
  - queue-worker: running (no port)
  - admin-api: healthy on port 8000
  - admin-frontend: healthy on port 3000
  And database migrations shall have been applied
  And all health check endpoints shall return 200 OK
```

### Scenario DEPLOY-5.2: End-to-end message flow via Docker Compose

```gherkin
Given all services are running via Docker Compose
  And an account "e2e-test" is configured via the admin API
  And an ESP provider is configured for "e2e-test" (using a mock/test provider)
When an SMTP client connects to port 2525
  And authenticates as "e2e-test"
  And sends a test email
Then the message metadata shall appear in PostgreSQL with status "queued"
  And the message body shall exist in the local file storage volume
  And the queue worker shall pick up the message
  And the message status shall transition to "delivered"
  And the delivery log shall be visible in the admin dashboard
```

### Scenario DEPLOY-5.3: Service isolation - admin failure does not affect SMTP

```gherkin
Given all services are running via Docker Compose
When the admin-api service is stopped
Then the smtp-server shall continue accepting and processing messages
  And the queue-worker shall continue delivering messages
  And the admin-frontend shall show connection errors but not crash
```

### Scenario DEPLOY-5.4: Message storage volume mount (local dev)

```gherkin
Given Docker Compose is configured with STORAGE_TYPE=local
  And STORAGE_PATH is mapped to a Docker volume at /data/messages
When a message is submitted via SMTP
Then the message body file shall be visible in the mounted volume on the host
  And the queue worker shall be able to read the same file
  And restarting the smtp-server container shall not lose stored messages
```

### Scenario DEPLOY-5.5: Independent service configuration

```gherkin
Given each service has its own set of environment variables
When the admin-api DATABASE_URL is changed to point to a read replica
Then only the admin-api shall use the read replica
  And the smtp-server and queue-worker shall continue using the primary database
  And no other service is affected by the configuration change
```

---

## Module 6: API Server Placeholder

### Scenario API-6.1: Health check endpoint

```gherkin
Given the API server placeholder is running
When a GET request is sent to /health
Then the response shall be HTTP 200
  And the response body shall contain status "ok"
```

### Scenario API-6.2: No other endpoints available

```gherkin
Given the API server placeholder is running
When a GET request is sent to /accounts or any other non-health endpoint
Then the response shall be HTTP 404
  And no admin or CRUD functionality shall be available
```

---

## Quality Gates

### Definition of Done

- [ ] All MessageStore interface methods implemented with tests (85%+ coverage)
- [ ] SMTP server refactored: sync mode removed, storage integration complete
- [ ] Queue worker refactored: fetches body from storage, backward compat works
- [ ] FastAPI admin backend: all endpoints functional with RBAC
- [ ] React frontend: login, dashboard, CRUD screens operational
- [ ] Docker Compose: all services start with single command
- [ ] Integration tests: end-to-end flow verified
- [ ] No data in PostgreSQL messages.body column (migration applied)
- [ ] Queue messages contain only ID references (verified)
- [ ] All existing tests pass (no regressions)
- [ ] Enqueue retry logic tested (success on retry, exhaustion after 3 attempts)
- [ ] Delivery result logging: success/failure/DLQ entries in delivery_logs table
- [ ] Log output configurable: stdout (default), file with rotation, CloudWatch
- [ ] Delivery statistics queryable from PostgreSQL (not from app logs)
- [ ] API server stripped to placeholder

### Verification Methods

| Method                   | Tools                                    | Scope              |
|--------------------------|------------------------------------------|---------------------|
| Unit Tests               | `go test -race ./...` (Go), pytest (Python), vitest (React) | All modules |
| Integration Tests        | Docker Compose + test scripts            | Cross-service flows |
| E2E Tests                | SMTP client + API calls + DB checks      | Full pipeline       |
| Load Tests               | k6 or vegeta                             | SMTP throughput     |
| Security Review          | gosec (Go), bandit (Python), npm audit   | All codebases      |
| RBAC Verification        | pytest with role-specific tokens         | Admin API           |
| Storage Reliability      | Fault injection (kill storage mid-write) | MessageStore        |
| Backward Compatibility   | Mixed old/new format queue messages      | Worker              |
