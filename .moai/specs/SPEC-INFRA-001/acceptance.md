# Acceptance Criteria: SPEC-INFRA-001

## Overview

This document defines comprehensive acceptance criteria for the Infrastructure, Deployment, and Scaling specification (SPEC-INFRA-001). All criteria are written in Given-When-Then (Gherkin) format for clarity and testability.

## Test Scenarios

### 1. Docker Compose Full Stack Startup

**Scenario 1.1: First-time Docker Compose startup**

```gherkin
Given a fresh clone of the smtp-proxy repository
And Docker and Docker Compose are installed
And the .env file is created from .env.example
When I run `docker-compose up`
Then all services should start successfully
And self-signed TLS certificates should be generated automatically
And PostgreSQL should be healthy within 30 seconds
And Redis should be healthy within 30 seconds
And SMTP server should be listening on port 587 and 465 within 60 seconds
And API server should be healthy on port 8080 within 60 seconds
And queue worker should start without errors
And frontend should be accessible on port 3000 within 60 seconds
```

**Scenario 1.2: Subsequent Docker Compose startup**

```gherkin
Given Docker Compose has been run at least once before
And self-signed certificates already exist in /certs directory
When I run `docker-compose up`
Then all services should start successfully
And certificate generation should be skipped
And all services should be healthy within 60 seconds
```

**Scenario 1.3: Service dependency ordering**

```gherkin
Given Docker Compose is starting from a stopped state
When I run `docker-compose up`
Then PostgreSQL should start first
And Redis should start first
And SMTP server should wait for PostgreSQL and Redis health checks
And API server should wait for PostgreSQL and Redis health checks
And queue worker should wait for PostgreSQL and Redis health checks
And no service should start before its dependencies are healthy
```

---

### 2. Self-Signed TLS Certificate Generation

**Scenario 2.1: Certificate generation on first run**

```gherkin
Given the /certs directory does not exist
When I run the certificate generation script
Then a CA certificate should be created at /certs/ca.crt
And a CA private key should be created at /certs/ca.key with permissions 0600
And a server certificate should be created at /certs/server.crt
And a server private key should be created at /certs/server.key with permissions 0600
And the server certificate should include SAN for "localhost" and "127.0.0.1"
And the server certificate should be valid for 365 days
```

**Scenario 2.2: Certificate validation**

```gherkin
Given self-signed certificates have been generated
When I inspect the server certificate using `openssl x509 -in server.crt -text`
Then the certificate should show Subject Alternative Names for localhost and 127.0.0.1
And the certificate should be issued by the local CA
And the certificate should have a validity period of 365 days from creation
And the certificate should use a 2048-bit RSA key
```

**Scenario 2.3: Certificate skip when already exists**

```gherkin
Given self-signed certificates already exist in /certs directory
When I run the certificate generation script
Then the script should detect existing certificates
And the script should skip generation
And the existing certificates should remain unchanged
```

---

### 3. Test Client Connects to Local SMTP with TLS

**Scenario 3.1: STARTTLS connection with insecure flag**

```gherkin
Given the SMTP server is running on localhost:587
And self-signed certificates are in use
When I run `./test-client send --host localhost --port 587 --tls starttls --insecure --from test@example.com --to user@example.com --subject "Test" --body "Hello"`
Then the client should establish a TCP connection to localhost:587
And the client should issue STARTTLS command
And the TLS handshake should complete successfully
And the client should skip certificate verification due to --insecure flag
And the email should be sent successfully
And the command should exit with status code 0
```

**Scenario 3.2: Implicit TLS connection with insecure flag**

```gherkin
Given the SMTP server is running on localhost:465
And self-signed certificates are in use
When I run `./test-client send --host localhost --port 465 --tls implicit --insecure --from test@example.com --to user@example.com --subject "Test" --body "Hello"`
Then the client should establish a TLS connection immediately
And the TLS handshake should complete successfully
And the client should skip certificate verification due to --insecure flag
And the email should be sent successfully
And the command should exit with status code 0
```

**Scenario 3.3: STARTTLS connection failure without insecure flag**

```gherkin
Given the SMTP server is running on localhost:587
And self-signed certificates are in use
When I run `./test-client send --host localhost --port 587 --tls starttls --from test@example.com --to user@example.com --subject "Test" --body "Hello"`
Then the client should establish a TCP connection
And the client should issue STARTTLS command
And the TLS handshake should fail with certificate verification error
And the client should display a clear error message about certificate trust
And the command should exit with non-zero status code
```

**Scenario 3.4: SMTP AUTH with valid credentials**

```gherkin
Given the SMTP server is running on localhost:587
And a tenant exists with username "tenant1@example.com" and password "secret123"
When I run `./test-client send --host localhost --port 587 --tls starttls --insecure --user tenant1@example.com --password secret123 --from sender@example.com --to recipient@example.com --subject "Auth Test" --body "Authenticated"`
Then the client should complete TLS handshake
And the client should issue AUTH PLAIN command
And the authentication should succeed
And the email should be sent successfully
And the command should exit with status code 0
```

**Scenario 3.5: SMTP AUTH with invalid credentials**

```gherkin
Given the SMTP server is running on localhost:587
When I run `./test-client send --host localhost --port 587 --tls starttls --insecure --user tenant1@example.com --password wrongpassword --from sender@example.com --to recipient@example.com --subject "Auth Fail" --body "Should fail"`
Then the client should complete TLS handshake
And the client should issue AUTH PLAIN command
And the authentication should fail with 535 error
And the client should display "SMTP AUTH failed" error message
And the command should exit with non-zero status code
```

---

### 4. Test Client Sends Email Through Full Pipeline

**Scenario 4.1: End-to-end email delivery**

```gherkin
Given the full Docker Compose stack is running
And the queue worker is processing messages
And a tenant account exists
When I run `./test-client send --host localhost --port 587 --tls starttls --insecure --user tenant1@example.com --password secret123 --from sender@example.com --to recipient@example.com --subject "E2E Test" --body "End to end test"`
Then the email should be accepted by the SMTP server
And the email should be enqueued in Redis within 1 second
And the queue worker should pick up the email within 5 seconds
And the email delivery should be attempted within 10 seconds
And the delivery status should be recorded in PostgreSQL
And I should be able to query the API for email status
And the API should return delivery status as "delivered" or "pending"
```

**Scenario 4.2: Batch email sending**

```gherkin
Given the full Docker Compose stack is running
And a tenant account exists
When I run `./test-client batch --host localhost --port 587 --tls starttls --insecure --user tenant1@example.com --password secret123 --count 100 --rate 10`
Then the client should send 100 emails at a rate of 10 per second
And all emails should be accepted by the SMTP server
And the client should report progress every 10 emails
And the client should complete within 15 seconds (100 emails / 10 per second + overhead)
And all 100 emails should be enqueued in Redis
And the queue worker should process all emails within 60 seconds
```

---

### 5. Let's Encrypt Certificate Acquisition

**Scenario 5.1: Initial certificate acquisition (staging)**

```gherkin
Given the SMTP server is deployed to AWS ECS
And the domain "smtp.example.com" points to the NLB
And Let's Encrypt staging environment is configured
And no certificate exists for smtp.example.com
When the SMTP server starts
Then the autocert manager should initiate ACME challenge
And the HTTP-01 challenge should be served on port 80
And Let's Encrypt should validate the challenge
And a staging certificate should be issued within 60 seconds
And the certificate should be stored in /var/lib/smtp-proxy/certs/
And a backup copy should be stored in S3 at s3://smtp-proxy-certs/smtp.example.com/
And the SMTP server should start using the new certificate
```

**Scenario 5.2: Initial certificate acquisition (production)**

```gherkin
Given the SMTP server is deployed to AWS ECS
And the domain "smtp.example.com" points to the NLB
And Let's Encrypt production environment is configured
And the staging certificate test has passed
When the SMTP server starts with production configuration
Then the autocert manager should initiate ACME challenge
And the HTTP-01 challenge should be served on port 80
And Let's Encrypt should validate the challenge
And a production certificate should be issued within 60 seconds
And the certificate should be valid for 90 days
And the certificate should be trusted by standard browsers
And the certificate should be stored locally and backed up to S3
```

**Scenario 5.3: Certificate acquisition failure and retry**

```gherkin
Given the SMTP server is deployed to AWS ECS
And the domain "smtp.example.com" points to the NLB
And Let's Encrypt is temporarily unreachable
When the SMTP server attempts to acquire a certificate
Then the initial ACME request should fail
And the autocert manager should log the failure
And the autocert manager should retry after 30 seconds
And the autocert manager should retry with exponential backoff (30s, 60s, 120s, 300s)
And a CloudWatch alarm should trigger after 3 consecutive failures
And an SNS notification should be sent to the operations team
```

---

### 6. Certificate Hot-Reload Without Downtime

**Scenario 6.1: Certificate renewal triggers hot-reload**

```gherkin
Given the SMTP server has an active certificate
And the certificate is set to expire in 29 days
And there are 100 active SMTP connections
When the certificate renewal process completes
Then the new certificate should be stored atomically (rename operation)
And the file watcher should detect the certificate change within 1 second
And the SMTP server should reload the TLS configuration
And all 100 existing connections should remain active
And new connections should use the new certificate
And no connection should be dropped during the reload
And the reload should complete within 1 second
```

**Scenario 6.2: Certificate reload without file change**

```gherkin
Given the SMTP server is running with a valid certificate
And there are active connections
When the certificate file timestamp is updated but content is unchanged
Then the file watcher should detect the modification
And the SMTP server should skip reloading (identical certificate detected)
And no connections should be affected
And a log message should indicate "Certificate unchanged, skipping reload"
```

**Scenario 6.3: Certificate reload with invalid certificate**

```gherkin
Given the SMTP server is running with a valid certificate
And there are active connections
When a new certificate file is written but is invalid (corrupted or expired)
Then the file watcher should detect the change
And the SMTP server should attempt to load the certificate
And certificate validation should fail
And the SMTP server should log an error "Failed to load certificate"
And the SMTP server should continue using the old certificate
And a CloudWatch alarm should trigger "Certificate reload failed"
And an SNS notification should be sent to operations
And all connections should remain active
```

---

### 7. ECS Fargate Deployment (All Services Running)

**Scenario 7.1: Initial infrastructure deployment**

```gherkin
Given I have configured terraform.tfvars with required variables
And I am in the infra/terraform/aws/ directory
When I run `terraform init`
And I run `terraform plan`
And I run `terraform apply -auto-approve`
Then Terraform should create VPC with 3 availability zones
And Terraform should create RDS PostgreSQL instance in multi-AZ mode
And Terraform should create ElastiCache Redis cluster with 3 nodes
And Terraform should create ECS cluster
And Terraform should create 4 ECS task definitions (smtp-server, api-server, queue-worker, frontend)
And Terraform should create 4 ECS services
And Terraform should create Network Load Balancer with listeners on ports 587 and 465
And Terraform should create Application Load Balancer with HTTPS listener
And the entire deployment should complete within 20 minutes
And all resources should be tagged with "Environment: production"
```

**Scenario 7.2: ECS services healthy after deployment**

```gherkin
Given Terraform has successfully applied the infrastructure
When I check the ECS cluster status
Then the smtp-server service should have 2 running tasks
And the api-server service should have 2 running tasks
And the queue-worker service should have 1 running task
And the frontend service should have 2 running tasks
And all tasks should pass health checks within 120 seconds
And all tasks should be registered with their respective target groups
And the NLB health checks should show 2/2 healthy targets for SMTP
And the ALB health checks should show 2/2 healthy targets for API
```

**Scenario 7.3: Service connectivity validation**

```gherkin
Given all ECS services are running and healthy
When I run connectivity tests
Then the SMTP server should be accessible via NLB DNS on port 587
And the SMTP server should be accessible via NLB DNS on port 465
And the API server should be accessible via ALB DNS on port 443
And the frontend should be accessible via ALB DNS on port 443
And the SMTP server should be able to connect to RDS PostgreSQL
And the SMTP server should be able to connect to ElastiCache Redis
And the queue worker should be able to read from Redis Streams
And the API server should be able to query PostgreSQL
```

---

### 8. Auto-Scaling: SMTP Server Scales on Connection Count

**Scenario 8.1: Scale-out on connection threshold breach**

```gherkin
Given the SMTP server is running with 2 tasks (min capacity)
And each task is configured to target 500 connections
And auto-scaling is enabled with target tracking policy
When the total connection count reaches 1200 (600 per task)
Then CloudWatch should receive the custom metric "ActiveConnections"
And the average connections per task should be calculated as 600
And the target tracking policy should trigger scale-out
And ECS should launch 1 additional SMTP server task
And the new task should be healthy within 30 seconds
And the new task should be registered with the NLB target group
And the connection count should rebalance to ~400 per task (1200 / 3)
```

**Scenario 8.2: Scale-out on CPU threshold breach**

```gherkin
Given the SMTP server is running with 2 tasks
And the CPU utilization is below 70%
When I simulate high CPU load (e.g., heavy TLS encryption)
And the average CPU utilization exceeds 70% for 2 consecutive minutes
Then the CPU-based auto-scaling policy should trigger scale-out
And ECS should launch 1 additional SMTP server task
And the new task should be healthy within 30 seconds
And CPU utilization should drop below 70% after load rebalancing
```

**Scenario 8.3: Scale-in after load decrease**

```gherkin
Given the SMTP server is running with 4 tasks
And the connection count per task is 200 (below 500 target)
And the CPU utilization is 30% (below threshold)
When the low load condition persists for 5 minutes
Then the target tracking policy should trigger scale-in
And ECS should deregister 1 task from the NLB target group
And the deregistered task should enter connection draining mode
And the draining delay should be 300 seconds
And ECS should wait for all connections to close gracefully
And if connections remain open after 300 seconds, ECS should force terminate
And the final task count should be 3
```

**Scenario 8.4: Maximum capacity limit enforcement**

```gherkin
Given the SMTP server is running with 20 tasks (max capacity)
And auto-scaling is configured with max_capacity = 20
When the connection count continues to increase
And the scale-out policy is triggered
Then ECS should not launch additional tasks beyond max capacity
And a CloudWatch alarm "SMTP server at max capacity" should trigger
And an SNS notification should be sent to operations
And the SMTP server should continue accepting connections up to its capacity
And if queue depth exceeds threshold, backpressure should activate (421 responses)
```

---

### 9. Auto-Scaling: Queue Worker Scales on Queue Depth

**Scenario 9.1: Scale-out on queue depth increase**

```gherkin
Given the queue worker is running with 1 task (min capacity)
And the queue depth is 500 messages
And the target queue depth is 1000 messages per worker
When the queue depth increases to 3500 messages
Then the queue worker should publish the custom metric "QueueDepth" to CloudWatch
And the average queue depth per worker should be calculated as 3500
And the target tracking policy should trigger scale-out
And ECS should launch 2 additional queue worker tasks (total 3, targeting ~1167 per worker)
And the new tasks should be healthy within 30 seconds
And the queue processing rate should increase proportionally
```

**Scenario 9.2: Scale-out to max capacity**

```gherkin
Given the queue worker is running with 5 tasks
And the queue depth is 50,000 messages
And max capacity is 10 tasks
When the queue depth continues to grow
Then the target tracking policy should trigger scale-out
And ECS should launch 5 additional tasks (total 10)
And all 10 tasks should be processing messages
And the queue processing rate should be ~10,000 messages per minute (estimated)
And if the queue depth continues to grow beyond 50,000, backpressure should activate
```

**Scenario 9.3: Scale-in after queue drains**

```gherkin
Given the queue worker is running with 10 tasks
And the queue depth drops to 2,000 messages
When the low queue depth persists for 3 minutes
Then the target tracking policy should trigger scale-in
And ECS should reduce the desired count to 2 tasks
And 8 tasks should be terminated gracefully
And in-flight messages should be processed before termination
And the final task count should be 2 (handling 1000 messages each)
```

---

### 10. Scale-In with Connection Draining

**Scenario 10.1: SMTP server connection draining**

```gherkin
Given the SMTP server is running with 4 tasks
And task "smtp-server-abc123" has 50 active connections
When ECS triggers scale-in for this task
Then the NLB should deregister the task from the target group
And the task should stop accepting new connections
And the task should continue processing existing 50 connections
And the task should send a log message "Entering connection draining mode"
And the task should wait up to 300 seconds for connections to close
And if all connections close within 60 seconds, the task should terminate immediately
And if connections remain after 300 seconds, the task should force close and terminate
And no connections should be abruptly dropped during draining
```

**Scenario 10.2: API server connection draining**

```gherkin
Given the API server is running with 3 tasks
And task "api-server-def456" is handling ongoing HTTP requests
When ECS triggers scale-in for this task
Then the ALB should deregister the task from the target group
And the ALB should stop routing new requests to the task
And the task should continue processing in-flight requests
And the task should respond with "Connection: close" header for new requests
And the task should wait up to 120 seconds for requests to complete
And if all requests complete within 30 seconds, the task should terminate immediately
And if requests remain after 120 seconds, the task should force terminate
```

**Scenario 10.3: Queue worker graceful shutdown**

```gherkin
Given the queue worker is running with 5 tasks
And task "queue-worker-ghi789" is processing 10 messages
When ECS triggers scale-in for this task
Then the task should receive SIGTERM signal
And the task should stop pulling new messages from Redis
And the task should complete processing the current 10 messages
And the task should log "Graceful shutdown initiated, processing remaining messages"
And the task should wait up to 180 seconds for message processing
And if all messages are processed within 30 seconds, the task should exit with code 0
And if messages remain after 180 seconds, the task should re-enqueue them and exit
```

---

### 11. Blue/Green Deployment with Zero Downtime

**Scenario 11.1: Successful blue/green deployment**

```gherkin
Given the current deployment is running (blue environment)
And all services are healthy
And a new Docker image has been pushed to ECR with tag "v2.0.0"
When I run `./deploy.sh v2.0.0`
Then CodeDeploy should create a new deployment
And ECS should launch new tasks with the v2.0.0 image (green environment)
And the green environment should pass health checks within 120 seconds
And CodeDeploy should begin shifting traffic from blue to green
And traffic should shift in increments: 10%, 50%, 100% over 15 minutes
And health checks should pass at each traffic shift stage
And the blue environment should remain running during traffic shift
And after 100% traffic shift, the blue environment should be terminated
And the deployment should complete within 20 minutes
And no requests should result in errors during the deployment
```

**Scenario 11.2: Automatic rollback on health check failure**

```gherkin
Given the current deployment is running (blue environment)
And a new Docker image has been pushed to ECR with tag "v2.1.0" (contains a bug)
When I run `./deploy.sh v2.1.0`
Then CodeDeploy should create a new deployment
And ECS should launch new tasks with the v2.1.0 image (green environment)
And the green environment tasks should start but fail health checks
And CodeDeploy should detect health check failures
And CodeDeploy should automatically trigger rollback
And traffic should remain on the blue environment (100%)
And the green environment should be terminated
And a CloudWatch alarm "Deployment failed" should trigger
And an SNS notification should be sent to operations
And the rollback should complete within 5 minutes
```

**Scenario 11.3: Manual rollback via script**

```gherkin
Given the current deployment v2.0.0 has been live for 1 hour
And a critical bug is discovered in production
When I run `./rollback.sh`
Then the script should identify the previous stable deployment (v1.9.0)
And the script should trigger a new CodeDeploy deployment with v1.9.0
And ECS should launch tasks with v1.9.0 image
And traffic should shift back to v1.9.0 within 10 minutes
And the v2.0.0 environment should be terminated after rollback
And the rollback should complete successfully
And a log entry should record "Manual rollback to v1.9.0 completed"
```

---

## Performance Requirements

### Throughput and Latency

**PR-1: Email Processing Throughput**

```gherkin
Given the system is fully scaled (20 SMTP tasks, 10 queue workers)
When load testing sends a sustained rate of 10,000 emails per minute
Then the system should accept and queue all emails without dropping any
And the queue depth should remain below 10,000 messages
And the P95 email acceptance latency should be less than 100ms
And the P95 end-to-end delivery latency should be less than 5 seconds
```

**PR-2: SMTP Connection Latency**

```gherkin
Given the SMTP server is running with 2 tasks
When a client establishes a TLS connection
Then the TCP handshake should complete within 50ms
And the TLS handshake should complete within 100ms
And the EHLO response should be received within 10ms
And the total connection establishment time should be less than 200ms (P95)
```

**PR-3: API Response Time**

```gherkin
Given the API server is running with 2 tasks
When load testing sends 1000 requests per second to the API
Then the P50 response time should be less than 100ms
And the P95 response time should be less than 200ms
And the P99 response time should be less than 500ms
And the error rate should be less than 0.1%
```

**PR-4: Auto-Scaling Response Time**

```gherkin
Given the SMTP server is at min capacity (2 tasks)
When load testing suddenly increases connection count to 2000
Then CloudWatch should receive the custom metric within 5 seconds
And the auto-scaling policy should trigger within 10 seconds
And a new ECS task should launch within 30 seconds
And the new task should be healthy and receiving traffic within 60 seconds from initial trigger
```

---

## Quality Gates

### QG-1: Infrastructure as Code Quality

```gherkin
Given all Terraform modules are implemented
When I run `terraform fmt -check`
Then all .tf files should be properly formatted

When I run `terraform validate`
Then all modules should pass validation with no errors

When I run `tflint`
Then there should be no warnings or errors
And all AWS resources should follow naming conventions
And all resources should have required tags
```

### QG-2: Docker Image Quality

```gherkin
Given all Dockerfiles are implemented
When I build all Docker images
Then each image should be less than 500MB
And each image should use non-root user
And each image should have health check defined
And each image should have minimal attack surface (distroless or alpine base)

When I scan images with `docker scout cves`
Then there should be zero critical vulnerabilities
And there should be zero high vulnerabilities
```

### QG-3: Test Coverage

```gherkin
Given all Go code is implemented
When I run `go test -cover ./...`
Then the overall test coverage should be at least 85%
And all critical paths (certificate management, connection draining) should have 100% coverage
And all tests should pass
```

### QG-4: Documentation Completeness

```gherkin
Given all implementation is complete
When I review the documentation
Then there should be a complete deployment guide for local development
And there should be a complete deployment guide for AWS
And there should be operational runbooks for all common scenarios
And there should be troubleshooting guides for all known issues
And all configuration variables should be documented
And all CloudWatch dashboards should be documented
```

### QG-5: Security Compliance

```gherkin
Given the infrastructure is deployed
When I run a security audit
Then no secrets should be stored in version control
And all secrets should be in AWS Secrets Manager
And all RDS databases should have encryption at rest enabled
And all ElastiCache clusters should have encryption in transit enabled
And all S3 buckets should have encryption enabled
And all security groups should follow least privilege principle
And all IAM roles should have minimal required permissions
```

---

## Non-Functional Requirements

### NFR-1: Reliability

```gherkin
Given the system is running in production
When measured over a 30-day period
Then the uptime should be at least 99.9%
And the mean time between failures (MTBF) should be at least 720 hours
And the mean time to recovery (MTTR) should be less than 15 minutes
```

### NFR-2: Scalability

```gherkin
Given the system is at min capacity
When load increases from 0 to 10,000 emails per minute over 30 minutes
Then the system should scale out automatically
And all emails should be processed successfully
And the P95 latency should not degrade by more than 20%

When load decreases back to 0 over 30 minutes
Then the system should scale in automatically
And the final capacity should return to min capacity
```

### NFR-3: Observability

```gherkin
Given the system is running in production
When I access the CloudWatch dashboard
Then I should see real-time metrics for all services
And I should see current task counts for all services
And I should see queue depth metrics
And I should see error rate metrics
And I should see latency percentiles (P50, P95, P99)
And all metrics should update within 60 seconds
```

### NFR-4: Cost Efficiency

```gherkin
Given the system is running at baseline capacity (min tasks)
When measured over a 30-day period
Then the total AWS infrastructure cost should be less than $600 per month
And the cost per 1000 emails should be less than $0.10

Given the system is running at full scale (max tasks)
When measured over a 24-hour period
Then the total AWS infrastructure cost should be less than $40 per day
And the cost should scale linearly with load
```

---

## Edge Cases and Error Handling

### EC-1: Certificate Renewal During High Load

```gherkin
Given the SMTP server has 1000 active connections
And a certificate renewal is due
When the certificate renewal process starts
Then the renewal should complete without affecting active connections
And new connections should use the new certificate immediately after reload
And zero connections should be dropped during the process
```

### EC-2: Database Connection Pool Exhaustion

```gherkin
Given the SMTP server is at max capacity (20 tasks)
And each task has a connection pool of 25 connections
When all connection pools reach maximum usage (500 total connections)
Then new queries should wait for available connections
And connection acquire timeout should be 5 seconds
And queries should fail gracefully with "connection pool timeout" error
And a CloudWatch alarm should trigger "Connection pool near exhaustion"
And an SNS notification should be sent to operations
```

### EC-3: Redis Cluster Failover During Active Processing

```gherkin
Given the queue worker is processing 1000 messages
And Redis cluster has 1 primary and 2 replicas
When the Redis primary node fails
Then ElastiCache should promote a replica to primary within 30 seconds
And the queue worker should detect the failover
And the queue worker should reconnect to the new primary automatically
And in-flight messages should be re-queued or retried
And message processing should resume within 60 seconds of failover
And zero messages should be lost during the failover
```

### EC-4: ECS Task Sudden Termination

```gherkin
Given the SMTP server task is processing 100 connections
When AWS terminates the task due to Fargate maintenance
Then the task should receive SIGTERM signal
And the task should stop accepting new connections
And the task should wait up to 300 seconds for connections to close
And ECS should launch a replacement task immediately
And the replacement task should be healthy within 60 seconds
And the total number of healthy tasks should be maintained
```

### EC-5: Let's Encrypt Rate Limit Exceeded

```gherkin
Given the system has requested 50 certificates in the past week
And the Let's Encrypt rate limit is 50 certificates per domain per week
When the system attempts to request the 51st certificate
Then the Let's Encrypt API should return a rate limit error
And the autocert manager should log the rate limit error
And the autocert manager should not retry until the rate limit resets
And the system should continue using the existing certificate
And a CloudWatch alarm "Let's Encrypt rate limit exceeded" should trigger
And an SNS notification should be sent to operations
```

---

## Success Criteria Summary

All acceptance criteria are met when:

- ✅ **Local Development**: Docker Compose starts all services successfully with self-signed certificates
- ✅ **Test Client**: Test SMTP client can send emails via STARTTLS and implicit TLS
- ✅ **End-to-End**: Emails flow through the complete pipeline from SMTP acceptance to delivery
- ✅ **Production Deployment**: All ECS services deploy successfully and pass health checks
- ✅ **Certificate Management**: Let's Encrypt certificates are acquired and renewed automatically
- ✅ **Hot-Reload**: Certificates reload without connection drops or downtime
- ✅ **Auto-Scaling**: All services scale out and in based on their respective metrics
- ✅ **Connection Draining**: Scale-in events drain connections gracefully without drops
- ✅ **Blue/Green Deployment**: Deployments complete with zero downtime and automatic rollback on failure
- ✅ **Performance**: System handles 10,000 emails/min with P95 latency < 5 seconds
- ✅ **Quality Gates**: 85%+ test coverage, zero critical vulnerabilities, complete documentation
- ✅ **Reliability**: 99.9% uptime, MTTR < 15 minutes
- ✅ **Observability**: Real-time metrics and dashboards available for all services
- ✅ **Cost**: Baseline cost < $600/month, full scale cost < $1,200/month

## Testing Checklist

### Pre-Production Testing

- [ ] All Docker Compose scenarios pass on Linux, macOS, and Windows WSL2
- [ ] Test client successfully connects and sends emails in all modes
- [ ] End-to-end email delivery works in local environment
- [ ] Terraform infrastructure deploys successfully in dev AWS account
- [ ] Let's Encrypt staging certificates acquired successfully
- [ ] Certificate hot-reload tested with simulated renewal
- [ ] Auto-scaling tested with load testing tools
- [ ] Connection draining tested during scale-in events
- [ ] Blue/green deployment tested with successful and failed deployments
- [ ] All quality gates pass
- [ ] All documentation reviewed and validated

### Production Validation

- [ ] Let's Encrypt production certificates acquired successfully
- [ ] All services healthy after initial deployment
- [ ] Auto-scaling validated with real production traffic
- [ ] Performance metrics meet SLAs during beta period
- [ ] No critical errors in CloudWatch logs
- [ ] Cost tracking confirms estimates are accurate
- [ ] Operations team trained on runbooks and dashboards

---

**Document Version**: 1.0.0
**Last Updated**: 2026-02-16
**Status**: Approved
**Owner**: Infrastructure Team
