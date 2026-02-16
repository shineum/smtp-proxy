---
id: SPEC-INFRA-001
version: "1.0.0"
status: approved
created: "2026-02-16"
updated: "2026-02-16"
author: sungwon
priority: P1
lifecycle: spec-anchored
domains:
  - infrastructure
  - deployment
  - scaling
  - operations
tags:
  - docker
  - aws-ecs
  - tls
  - certificates
  - auto-scaling
  - terraform
---

# SPEC-INFRA-001: Infrastructure, Deployment, and Scaling

## HISTORY

### Version 1.0.0 (2026-02-16)
- Initial specification for complete infrastructure and operations
- Covers TLS certificate management (Let's Encrypt + self-signed)
- Docker Compose local development environment
- Test SMTP client for validation
- AWS ECS Fargate deployment with auto-scaling
- Comprehensive scaling strategy for all components

## OVERVIEW

This SPEC defines the complete infrastructure, deployment, and scaling architecture for the smtp-proxy system. It encompasses local development setup, TLS certificate management, testing tools, cloud deployment, and auto-scaling strategies to support production workloads.

## SCOPE

### In Scope

1. **TLS Certificate Management**
   - Let's Encrypt integration for production certificates
   - Self-signed certificate generation for local development
   - Automatic certificate renewal without service interruption
   - Hot-reload mechanism for certificate updates
   - SNI (Server Name Indication) support for multi-domain hosting

2. **Docker Compose Local Development**
   - One-command development environment setup
   - All services containerized (PostgreSQL, Redis, SMTP, API, Worker, Frontend)
   - Volume mounts for hot-reload during development
   - Environment variable management with .env.example template
   - Automatic self-signed certificate generation on first run
   - Health checks and dependency ordering
   - Network isolation for security

3. **Test SMTP Client**
   - Go-based CLI client for sending test emails
   - TLS and STARTTLS protocol support
   - SMTP AUTH mechanisms (PLAIN, LOGIN)
   - Runs on host machine (outside Docker)
   - Supports both self-signed and valid certificates
   - Batch sending capability for load testing

4. **AWS ECS Fargate Deployment**
   - ECS task definitions for each service
   - Network Load Balancer (NLB) for SMTP traffic
   - Application Load Balancer (ALB) for HTTP traffic
   - RDS PostgreSQL with read replicas
   - ElastiCache Redis in cluster mode
   - Service discovery via AWS Cloud Map
   - Blue/green deployment with CodeDeploy
   - CloudWatch monitoring and alarms

5. **Auto-Scaling Strategy**
   - Per-service scaling policies
   - Queue-based backpressure mechanisms
   - Connection draining during scale-in
   - Database connection pooling
   - Capacity planning formulas

### Out of Scope

- Multi-region deployment (future SPEC)
- Disaster recovery and backup procedures (separate SPEC)
- Cost optimization strategies (separate SPEC)
- CI/CD pipeline configuration (separate SPEC)
- Security hardening and compliance (covered in SPEC-SECURITY-001)

## CONSTITUTION ALIGNMENT

This SPEC aligns with project constitution defined in `.moai/project/tech.md`:

- **Technology Stack**: Go 1.23+, Next.js 15+, PostgreSQL 17, Redis 7.4, Docker Compose 2.x
- **Infrastructure**: AWS ECS Fargate, RDS, ElastiCache, NLB/ALB
- **IaC**: Terraform for infrastructure as code
- **Naming Conventions**: Kebab-case for infrastructure resources, snake_case for environment variables
- **Security Standards**: TLS 1.3 minimum, Let's Encrypt for production, no hardcoded credentials

## ENVIRONMENT

### Development Environment

- **OS**: Linux, macOS, Windows with WSL2
- **Docker**: Docker Engine 24.x+, Docker Compose 2.x+
- **Go**: 1.23+ for test client development
- **Tools**: curl, openssl, aws-cli, terraform

### Production Environment

- **Cloud Provider**: AWS
- **Compute**: ECS Fargate 1.4.0+ (arm64 and x86_64)
- **Database**: RDS PostgreSQL 17
- **Cache**: ElastiCache Redis 7.4 in cluster mode
- **Load Balancers**: NLB for SMTP (TCP), ALB for HTTP/HTTPS
- **DNS**: Route 53 for domain management
- **Monitoring**: CloudWatch, CloudWatch Logs, CloudWatch Alarms

### External Dependencies

- **Let's Encrypt**: ACME protocol for certificate issuance
- **OpenSSL**: For self-signed certificate generation
- **AWS Services**: ECS, RDS, ElastiCache, NLB, ALB, Cloud Map, CodeDeploy, CloudWatch

## ASSUMPTIONS

### Technical Assumptions

1. **TLS Certificate Management**
   - ASSUMPTION: Let's Encrypt rate limits (50 certificates per domain per week) are sufficient
   - CONFIDENCE: High
   - EVIDENCE: Production will use 1-3 certificates maximum
   - RISK IF WRONG: Need to use alternative CA or purchase certificates
   - VALIDATION: Monitor Let's Encrypt API quota during deployment

2. **Docker Compose Performance**
   - ASSUMPTION: Docker Compose can handle local development load (< 100 emails/min)
   - CONFIDENCE: High
   - EVIDENCE: Docker Compose standard practice for microservices development
   - RISK IF WRONG: Need to use Kubernetes locally (minikube/kind)
   - VALIDATION: Performance testing during development

3. **AWS ECS Fargate Compatibility**
   - ASSUMPTION: All services can run in Fargate without privileged mode or host networking
   - CONFIDENCE: High
   - EVIDENCE: SMTP server, API, worker are stateless applications
   - RISK IF WRONG: Need to use EC2-based ECS
   - VALIDATION: Test deployments during implementation

4. **Auto-Scaling Response Time**
   - ASSUMPTION: ECS auto-scaling can respond within 30 seconds to traffic spikes
   - CONFIDENCE: Medium
   - EVIDENCE: ECS task launch time typically 15-30 seconds
   - RISK IF WRONG: Need pre-warming or larger minimum task count
   - VALIDATION: Load testing with gradual and sudden traffic increases

### Business Assumptions

1. **Traffic Patterns**
   - ASSUMPTION: Email traffic is generally predictable with gradual increases
   - CONFIDENCE: Medium
   - EVIDENCE: B2B email traffic patterns are typically business-hours focused
   - RISK IF WRONG: Need burst capacity or queuing strategy
   - VALIDATION: Monitor production traffic patterns after launch

2. **Cost Budget**
   - ASSUMPTION: AWS costs are acceptable for target scale (10K emails/min)
   - CONFIDENCE: Medium
   - EVIDENCE: Preliminary cost estimation shows reasonable monthly spend
   - RISK IF WRONG: Need to optimize resource usage or revise pricing
   - VALIDATION: Cost monitoring during beta testing

### Root Cause Analysis

**Why do we need this infrastructure?**

- **Surface Problem**: Need to deploy smtp-proxy to production
- **First Why**: Manual deployment is error-prone and not scalable
- **Second Why**: No standardized deployment process exists
- **Third Why**: Infrastructure requirements were not defined upfront
- **Fourth Why**: Project started with feature development before operations
- **Root Cause**: SPEC-First approach was not applied to infrastructure from the beginning

**Lesson**: Infrastructure and deployment should be specified as early as feature development.

## REQUIREMENTS

### TLS Certificate Management Requirements

#### Ubiquitous Requirements

**REQ-INFRA-TLS-001**: The system **shall** support TLS 1.3 and TLS 1.2 for SMTP connections.

**REQ-INFRA-TLS-002**: The system **shall** provide both STARTTLS (port 587) and implicit TLS (port 465) modes.

**REQ-INFRA-TLS-003**: The system **shall** validate client certificates when mutual TLS is enabled.

#### Event-Driven Requirements

**REQ-INFRA-TLS-004**: **WHEN** a certificate renewal is triggered, **THEN** the system **shall** request a new certificate from Let's Encrypt via ACME protocol.

**REQ-INFRA-TLS-005**: **WHEN** a new certificate is obtained, **THEN** the system **shall** reload the certificate without restarting the SMTP server.

**REQ-INFRA-TLS-006**: **WHEN** certificate renewal fails, **THEN** the system **shall** retry with exponential backoff and alert administrators.

**REQ-INFRA-TLS-007**: **WHEN** Docker Compose starts for the first time, **THEN** the system **shall** automatically generate self-signed certificates for local development.

#### State-Driven Requirements

**REQ-INFRA-TLS-008**: **IF** running in production mode, **THEN** the system **shall** use Let's Encrypt certificates.

**REQ-INFRA-TLS-009**: **IF** running in development mode, **THEN** the system **shall** use self-signed certificates.

**REQ-INFRA-TLS-010**: **IF** a certificate expires within 30 days, **THEN** the system **shall** initiate automatic renewal.

#### Unwanted Requirements

**REQ-INFRA-TLS-011**: The system **shall not** accept TLS 1.1 or earlier protocol versions.

**REQ-INFRA-TLS-012**: The system **shall not** store private keys in plain text in version control.

**REQ-INFRA-TLS-013**: The system **shall not** restart the SMTP server during certificate renewal.

#### Optional Requirements

**REQ-INFRA-TLS-014**: **Where possible**, the system **should** support OCSP stapling for improved TLS handshake performance.

**REQ-INFRA-TLS-015**: **Where possible**, the system **should** support certificate pinning for enhanced security.

### Docker Compose Local Development Requirements

#### Ubiquitous Requirements

**REQ-INFRA-DOCKER-001**: The system **shall** provide a `docker-compose.yml` file that starts all services with a single command.

**REQ-INFRA-DOCKER-002**: The system **shall** mount source code directories as volumes for hot-reload during development.

**REQ-INFRA-DOCKER-003**: The system **shall** provide health checks for all services.

#### Event-Driven Requirements

**REQ-INFRA-DOCKER-004**: **WHEN** `docker-compose up` is executed, **THEN** the system **shall** start services in dependency order (database, cache, then application services).

**REQ-INFRA-DOCKER-005**: **WHEN** a service health check fails, **THEN** the system **shall** restart the failing service automatically.

**REQ-INFRA-DOCKER-006**: **WHEN** environment variables change in `.env`, **THEN** the system **shall** reload affected services on restart.

#### State-Driven Requirements

**REQ-INFRA-DOCKER-007**: **IF** PostgreSQL is not ready, **THEN** application services **shall** wait before starting.

**REQ-INFRA-DOCKER-008**: **IF** Redis is not ready, **THEN** the queue worker **shall** wait before starting.

#### Unwanted Requirements

**REQ-INFRA-DOCKER-009**: The system **shall not** expose database ports to the host machine by default.

**REQ-INFRA-DOCKER-010**: The system **shall not** run services as root inside containers.

**REQ-INFRA-DOCKER-011**: The system **shall not** include production credentials in `.env.example`.

#### Optional Requirements

**REQ-INFRA-DOCKER-012**: **Where possible**, the system **should** use multi-stage Docker builds to minimize image size.

**REQ-INFRA-DOCKER-013**: **Where possible**, the system **should** cache Go dependencies in Docker layers.

### Test SMTP Client Requirements

#### Ubiquitous Requirements

**REQ-INFRA-CLIENT-001**: The test client **shall** be implemented as a standalone Go binary.

**REQ-INFRA-CLIENT-002**: The test client **shall** support both STARTTLS and implicit TLS modes.

**REQ-INFRA-CLIENT-003**: The test client **shall** support SMTP AUTH PLAIN and LOGIN mechanisms.

#### Event-Driven Requirements

**REQ-INFRA-CLIENT-004**: **WHEN** the `--send` command is invoked, **THEN** the client **shall** send a single test email.

**REQ-INFRA-CLIENT-005**: **WHEN** the `--batch` command is invoked, **THEN** the client **shall** send multiple emails for load testing.

**REQ-INFRA-CLIENT-006**: **WHEN** TLS verification fails, **THEN** the client **shall** provide a clear error message with certificate details.

#### State-Driven Requirements

**REQ-INFRA-CLIENT-007**: **IF** `--insecure` flag is set, **THEN** the client **shall** skip TLS certificate verification.

**REQ-INFRA-CLIENT-008**: **IF** authentication credentials are provided, **THEN** the client **shall** authenticate before sending.

#### Unwanted Requirements

**REQ-INFRA-CLIENT-009**: The test client **shall not** require Docker to run.

**REQ-INFRA-CLIENT-010**: The test client **shall not** store sent emails persistently.

#### Optional Requirements

**REQ-INFRA-CLIENT-011**: **Where possible**, the client **should** support custom email templates.

**REQ-INFRA-CLIENT-012**: **Where possible**, the client **should** report detailed timing metrics.

### AWS ECS Fargate Deployment Requirements

#### Ubiquitous Requirements

**REQ-INFRA-ECS-001**: The system **shall** deploy all services to AWS ECS Fargate.

**REQ-INFRA-ECS-002**: The system **shall** use Terraform for infrastructure as code.

**REQ-INFRA-ECS-003**: The system **shall** implement blue/green deployments via AWS CodeDeploy.

**REQ-INFRA-ECS-004**: The system **shall** use AWS Secrets Manager for credential storage.

#### Event-Driven Requirements

**REQ-INFRA-ECS-005**: **WHEN** a new task definition is deployed, **THEN** CodeDeploy **shall** execute blue/green deployment.

**REQ-INFRA-ECS-006**: **WHEN** health checks fail during deployment, **THEN** the system **shall** automatically roll back.

**REQ-INFRA-ECS-007**: **WHEN** a task fails, **THEN** ECS **shall** automatically restart it.

#### State-Driven Requirements

**REQ-INFRA-ECS-008**: **IF** traffic exceeds threshold, **THEN** the system **shall** scale out additional tasks.

**REQ-INFRA-ECS-009**: **IF** traffic drops below threshold, **THEN** the system **shall** scale in excess tasks with connection draining.

#### Unwanted Requirements

**REQ-INFRA-ECS-010**: The system **shall not** store secrets in environment variables.

**REQ-INFRA-ECS-011**: The system **shall not** use EC2-based ECS (Fargate only).

**REQ-INFRA-ECS-012**: The system **shall not** deploy without health checks.

#### Optional Requirements

**REQ-INFRA-ECS-013**: **Where possible**, the system **should** use ARM64 Fargate tasks for cost savings.

**REQ-INFRA-ECS-014**: **Where possible**, the system **should** enable ECS Exec for debugging.

### Auto-Scaling Requirements

#### Ubiquitous Requirements

**REQ-INFRA-SCALE-001**: Each service **shall** have independent auto-scaling policies.

**REQ-INFRA-SCALE-002**: The system **shall** implement connection draining during scale-in events.

**REQ-INFRA-SCALE-003**: The system **shall** use database connection pooling (PgBouncer or pgx pool).

#### Event-Driven Requirements

**REQ-INFRA-SCALE-004**: **WHEN** SMTP connection count exceeds threshold, **THEN** the system **shall** scale out SMTP server tasks.

**REQ-INFRA-SCALE-005**: **WHEN** Redis queue depth exceeds threshold, **THEN** the system **shall** scale out queue worker tasks.

**REQ-INFRA-SCALE-006**: **WHEN** API request latency exceeds threshold, **THEN** the system **shall** scale out API server tasks.

**REQ-INFRA-SCALE-007**: **WHEN** queue depth exceeds critical threshold, **THEN** SMTP server **shall** return 421 (service unavailable) to apply backpressure.

#### State-Driven Requirements

**REQ-INFRA-SCALE-008**: **IF** CPU utilization exceeds 70% for 2 minutes, **THEN** the system **shall** trigger scale-out.

**REQ-INFRA-SCALE-009**: **IF** CPU utilization drops below 30% for 5 minutes, **THEN** the system **shall** trigger scale-in.

#### Unwanted Requirements

**REQ-INFRA-SCALE-010**: The system **shall not** terminate tasks with active connections during scale-in.

**REQ-INFRA-SCALE-011**: The system **shall not** scale beyond maximum capacity limits.

#### Optional Requirements

**REQ-INFRA-SCALE-012**: **Where possible**, the system **should** use predictive scaling based on historical patterns.

**REQ-INFRA-SCALE-013**: **Where possible**, the system **should** implement pre-warming during known traffic peaks.

## SPECIFICATIONS

### TLS Certificate Architecture

#### Let's Encrypt Integration (Production)

**Component**: Certificate Manager Service

**Technology Choice**:
- Go-based: Use `golang.org/x/crypto/acme/autocert` for automatic certificate management
- Alternative: Use `certbot` in Docker sidecar container

**Implementation Pattern**:

```
CertificateManager
├── ACME Client (Let's Encrypt)
├── Certificate Storage (filesystem or S3)
├── Renewal Scheduler (cron-based)
├── Hot-Reload Notifier (file watcher)
└── SNI Handler (multi-domain support)
```

**Certificate Renewal Process**:

1. Check certificate expiry daily at 2 AM UTC
2. If expiry within 30 days, initiate renewal
3. Use HTTP-01 or DNS-01 challenge (DNS-01 for wildcard)
4. Store new certificate atomically
5. Notify SMTP server via IPC (Unix socket or HTTP)
6. SMTP server reloads TLS config without restart

**Storage Strategy**:
- Local filesystem: `/var/lib/smtp-proxy/certs/`
- Backup to S3: `s3://smtp-proxy-certs/{domain}/`
- Permissions: 0600 for private keys, 0644 for certificates

#### Self-Signed Certificates (Development)

**Component**: Certificate Generation Script

**Technology**: OpenSSL via shell script

**Script Functionality**:

```bash
#!/bin/bash
# scripts/generate-dev-certs.sh

# Generate CA certificate
openssl req -x509 -new -nodes -days 365 \
  -keyout ca.key -out ca.crt \
  -subj "/C=US/O=SMTP-Proxy-Dev/CN=SMTP-Proxy-CA"

# Generate server certificate signed by CA
openssl req -new -nodes \
  -keyout server.key -out server.csr \
  -subj "/C=US/O=SMTP-Proxy-Dev/CN=localhost"

# Sign with CA and create certificate
openssl x509 -req -days 365 \
  -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1")
```

**Integration with Docker Compose**:

```yaml
services:
  smtp-server:
    volumes:
      - ./certs:/certs:ro
    environment:
      - TLS_CERT_FILE=/certs/server.crt
      - TLS_KEY_FILE=/certs/server.key
```

**First-Run Automation**:
- `docker-compose.yml` includes init container that runs cert generation script
- Check if `/certs/server.crt` exists; skip if present
- Generate and save to mounted volume

#### SNI Support Architecture

**Multi-Domain Certificate Management**:

```go
type CertificateManager struct {
    certCache map[string]*tls.Certificate
    mu        sync.RWMutex
}

func (cm *CertificateManager) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
    cm.mu.RLock()
    cert, exists := cm.certCache[clientHello.ServerName]
    cm.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("no certificate for domain: %s", clientHello.ServerName)
    }

    return cert, nil
}
```

**Configuration**:
- Support multiple domains via configuration file
- Each domain has separate Let's Encrypt certificate
- Automatic SNI selection based on EHLO/HELO domain

### Docker Compose Architecture

#### Service Composition

**File**: `infra/docker/docker-compose.yml`

```yaml
version: '3.9'

services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER}"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - backend

  redis:
    image: redis:7.4-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - backend

  smtp-server:
    build:
      context: ../..
      dockerfile: backend/cmd/smtp-server/Dockerfile
    ports:
      - "587:587"   # STARTTLS
      - "465:465"   # Implicit TLS
    volumes:
      - ./certs:/certs:ro
      - ../../backend:/app/backend:ro
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - REDIS_URL=redis://redis:6379
      - TLS_CERT_FILE=/certs/server.crt
      - TLS_KEY_FILE=/certs/server.key
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "nc", "-zv", "localhost", "587"]
      interval: 10s
      timeout: 5s
      retries: 3
    networks:
      - backend

  api-server:
    build:
      context: ../..
      dockerfile: backend/cmd/api-server/Dockerfile
    ports:
      - "8080:8080"
    volumes:
      - ../../backend:/app/backend:ro
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - REDIS_URL=redis://redis:6379
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    networks:
      - backend

  queue-worker:
    build:
      context: ../..
      dockerfile: backend/cmd/queue-worker/Dockerfile
    volumes:
      - ../../backend:/app/backend:ro
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - REDIS_URL=redis://redis:6379
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - backend

  frontend:
    build:
      context: ../..
      dockerfile: frontend/Dockerfile
    ports:
      - "3000:3000"
    volumes:
      - ../../frontend:/app/frontend:ro
      - /app/frontend/node_modules
    environment:
      - NEXT_PUBLIC_API_URL=http://api-server:8080
    depends_on:
      api-server:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/api/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    networks:
      - frontend
      - backend

networks:
  backend:
    driver: bridge
  frontend:
    driver: bridge

volumes:
  postgres_data:
  redis_data:
```

#### Environment Variable Management

**File**: `.env.example`

```bash
# Database Configuration
DB_USER=smtp_proxy
DB_PASSWORD=changeme_dev_password
DB_NAME=smtp_proxy_dev

# Redis Configuration
REDIS_URL=redis://redis:6379

# SMTP Configuration
SMTP_STARTTLS_PORT=587
SMTP_TLS_PORT=465

# API Configuration
API_PORT=8080

# Frontend Configuration
NEXT_PUBLIC_API_URL=http://localhost:8080
```

**Usage Instructions**:
```bash
# Copy example to actual .env file
cp .env.example .env

# Edit .env with your local values
# Then start services
docker-compose up
```

#### Hot-Reload Configuration

**Backend Services** (Go):
- Use `air` for live reload
- Mount source directories as read-only volumes
- Use `.air.toml` configuration for watch patterns

**Frontend** (Next.js):
- Next.js dev server has built-in hot-reload
- Mount `frontend` directory with `node_modules` exclusion

### Test SMTP Client Specification

#### CLI Interface

**File**: `backend/cmd/test-client/main.go`

**Command Structure**:

```bash
# Send single test email
./test-client send \
  --host localhost \
  --port 587 \
  --tls starttls \
  --user tenant1@example.com \
  --password secret \
  --from sender@example.com \
  --to recipient@example.com \
  --subject "Test Email" \
  --body "This is a test email"

# Send batch of emails
./test-client batch \
  --host localhost \
  --port 587 \
  --tls starttls \
  --user tenant1@example.com \
  --password secret \
  --count 100 \
  --rate 10  # 10 emails per second

# Test with implicit TLS
./test-client send \
  --host localhost \
  --port 465 \
  --tls implicit \
  --insecure  # Skip cert verification for self-signed certs

# Test authentication failure
./test-client send \
  --host localhost \
  --port 587 \
  --user tenant1@example.com \
  --password wrongpassword  # Expect auth failure
```

#### Implementation Pattern

**Core Functionality**:

```go
package main

import (
    "crypto/tls"
    "fmt"
    "net/smtp"
)

type SMTPClient struct {
    host     string
    port     int
    tlsMode  string  // "starttls" or "implicit"
    insecure bool
    username string
    password string
}

func (c *SMTPClient) Send(from, to, subject, body string) error {
    // Build TLS config
    tlsConfig := &tls.Config{
        ServerName:         c.host,
        InsecureSkipVerify: c.insecure,
    }

    addr := fmt.Sprintf("%s:%d", c.host, c.port)

    if c.tlsMode == "implicit" {
        // Implicit TLS: dial with TLS from start
        conn, err := tls.Dial("tcp", addr, tlsConfig)
        if err != nil {
            return fmt.Errorf("TLS dial failed: %w", err)
        }
        defer conn.Close()

        client, err := smtp.NewClient(conn, c.host)
        if err != nil {
            return fmt.Errorf("SMTP client creation failed: %w", err)
        }
        defer client.Quit()

        return c.sendMail(client, from, to, subject, body)
    }

    // STARTTLS: dial plain, upgrade to TLS
    client, err := smtp.Dial(addr)
    if err != nil {
        return fmt.Errorf("SMTP dial failed: %w", err)
    }
    defer client.Quit()

    if err := client.StartTLS(tlsConfig); err != nil {
        return fmt.Errorf("STARTTLS failed: %w", err)
    }

    return c.sendMail(client, from, to, subject, body)
}

func (c *SMTPClient) sendMail(client *smtp.Client, from, to, subject, body string) error {
    // Authenticate if credentials provided
    if c.username != "" {
        auth := smtp.PlainAuth("", c.username, c.password, c.host)
        if err := client.Auth(auth); err != nil {
            return fmt.Errorf("SMTP AUTH failed: %w", err)
        }
    }

    // Send mail
    if err := client.Mail(from); err != nil {
        return err
    }
    if err := client.Rcpt(to); err != nil {
        return err
    }

    wc, err := client.Data()
    if err != nil {
        return err
    }
    defer wc.Close()

    msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)
    if _, err := wc.Write([]byte(msg)); err != nil {
        return err
    }

    return nil
}
```

#### Batch Testing Implementation

```go
func (c *SMTPClient) SendBatch(from string, recipients []string, count int, rate int) error {
    rateLimiter := time.NewTicker(time.Second / time.Duration(rate))
    defer rateLimiter.Stop()

    for i := 0; i < count; i++ {
        <-rateLimiter.C

        to := recipients[i%len(recipients)]
        subject := fmt.Sprintf("Batch Test Email %d", i+1)
        body := fmt.Sprintf("This is batch test email number %d", i+1)

        if err := c.Send(from, to, subject, body); err != nil {
            return fmt.Errorf("batch email %d failed: %w", i+1, err)
        }

        if (i+1)%10 == 0 {
            fmt.Printf("Sent %d/%d emails\n", i+1, count)
        }
    }

    return nil
}
```

### AWS ECS Fargate Deployment Architecture

#### Infrastructure Components

**Terraform Module Structure**:

```
infra/terraform/aws/
├── main.tf                 # Root module
├── variables.tf            # Input variables
├── outputs.tf              # Output values
├── modules/
│   ├── vpc/                # VPC, subnets, NAT gateway
│   ├── rds/                # PostgreSQL RDS
│   ├── elasticache/        # Redis cluster
│   ├── ecs/                # ECS cluster and services
│   ├── nlb/                # Network Load Balancer for SMTP
│   ├── alb/                # Application Load Balancer for HTTP
│   ├── cloudwatch/         # Monitoring and alarms
│   └── codedeploy/         # Blue/green deployment
```

#### ECS Task Definitions

**SMTP Server Task**:

```hcl
resource "aws_ecs_task_definition" "smtp_server" {
  family                   = "smtp-proxy-smtp-server"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "512"   # 0.5 vCPU
  memory                   = "1024"  # 1 GB
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.smtp_server_task.arn

  container_definitions = jsonencode([
    {
      name      = "smtp-server"
      image     = "${aws_ecr_repository.smtp_server.repository_url}:latest"
      essential = true

      portMappings = [
        {
          containerPort = 587
          protocol      = "tcp"
        },
        {
          containerPort = 465
          protocol      = "tcp"
        }
      ]

      environment = [
        {
          name  = "DB_HOST"
          value = aws_db_instance.postgresql.address
        },
        {
          name  = "REDIS_URL"
          value = "redis://${aws_elasticache_replication_group.redis.primary_endpoint_address}:6379"
        }
      ]

      secrets = [
        {
          name      = "DB_PASSWORD"
          valueFrom = aws_secretsmanager_secret.db_password.arn
        }
      ]

      healthCheck = {
        command     = ["CMD-SHELL", "nc -zv localhost 587"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = "/ecs/smtp-proxy/smtp-server"
          "awslogs-region"        = "us-east-1"
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])
}
```

**API Server Task**:

```hcl
resource "aws_ecs_task_definition" "api_server" {
  family                   = "smtp-proxy-api-server"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"   # 0.25 vCPU
  memory                   = "512"   # 512 MB
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.api_server_task.arn

  container_definitions = jsonencode([
    {
      name      = "api-server"
      image     = "${aws_ecr_repository.api_server.repository_url}:latest"
      essential = true

      portMappings = [
        {
          containerPort = 8080
          protocol      = "tcp"
        }
      ]

      healthCheck = {
        command     = ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = "/ecs/smtp-proxy/api-server"
          "awslogs-region"        = "us-east-1"
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])
}
```

**Queue Worker Task**:

```hcl
resource "aws_ecs_task_definition" "queue_worker" {
  family                   = "smtp-proxy-queue-worker"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "512"   # 0.5 vCPU
  memory                   = "1024"  # 1 GB
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.queue_worker_task.arn

  container_definitions = jsonencode([
    {
      name      = "queue-worker"
      image     = "${aws_ecr_repository.queue_worker.repository_url}:latest"
      essential = true

      # No port mappings - worker doesn't expose ports

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = "/ecs/smtp-proxy/queue-worker"
          "awslogs-region"        = "us-east-1"
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])
}
```

#### Load Balancer Configuration

**Network Load Balancer (SMTP)**:

```hcl
resource "aws_lb" "smtp_nlb" {
  name               = "smtp-proxy-nlb"
  internal           = false
  load_balancer_type = "network"
  subnets            = aws_subnet.public[*].id

  enable_cross_zone_load_balancing = true

  tags = {
    Name = "smtp-proxy-nlb"
  }
}

# STARTTLS port 587
resource "aws_lb_listener" "smtp_starttls" {
  load_balancer_arn = aws_lb.smtp_nlb.arn
  port              = "587"
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.smtp_starttls.arn
  }
}

# Implicit TLS port 465
resource "aws_lb_listener" "smtp_tls" {
  load_balancer_arn = aws_lb.smtp_nlb.arn
  port              = "465"
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.smtp_tls.arn
  }
}
```

**Application Load Balancer (HTTP)**:

```hcl
resource "aws_lb" "http_alb" {
  name               = "smtp-proxy-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  enable_http2                     = true
  enable_cross_zone_load_balancing = true

  tags = {
    Name = "smtp-proxy-alb"
  }
}

# HTTPS listener
resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.http_alb.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate.main.arn

  default_action {
    type = "forward"
    target_group_arn = aws_lb_target_group.api_server.arn
  }
}
```

#### RDS PostgreSQL Configuration

```hcl
resource "aws_db_instance" "postgresql" {
  identifier              = "smtp-proxy-db"
  engine                  = "postgres"
  engine_version          = "17.2"
  instance_class          = "db.t4g.medium"  # ARM-based for cost savings
  allocated_storage       = 100
  storage_type            = "gp3"
  storage_encrypted       = true

  db_name  = "smtp_proxy"
  username = "smtp_admin"
  password = random_password.db_password.result

  multi_az               = true
  backup_retention_period = 7
  backup_window          = "03:00-04:00"
  maintenance_window     = "mon:04:00-mon:05:00"

  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  # Read replicas for scaling reads
  replicate_source_db = null  # Set for read replica

  tags = {
    Name = "smtp-proxy-postgresql"
  }
}

# Read replica for scaling
resource "aws_db_instance" "postgresql_read_replica" {
  identifier          = "smtp-proxy-db-replica"
  replicate_source_db = aws_db_instance.postgresql.identifier
  instance_class      = "db.t4g.medium"

  publicly_accessible = false

  tags = {
    Name = "smtp-proxy-postgresql-replica"
  }
}
```

#### ElastiCache Redis Configuration

```hcl
resource "aws_elasticache_replication_group" "redis" {
  replication_group_id       = "smtp-proxy-redis"
  replication_group_description = "Redis cluster for SMTP proxy queue"

  engine               = "redis"
  engine_version       = "7.1"
  node_type            = "cache.t4g.medium"  # ARM-based
  num_cache_clusters   = 3  # 1 primary + 2 replicas

  parameter_group_name = "default.redis7.cluster.on"
  port                 = 6379

  subnet_group_name          = aws_elasticache_subnet_group.redis.name
  security_group_ids         = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true

  automatic_failover_enabled = true
  multi_az_enabled           = true

  snapshot_retention_limit = 5
  snapshot_window          = "03:00-05:00"

  tags = {
    Name = "smtp-proxy-redis"
  }
}
```

### Auto-Scaling Strategy

#### SMTP Server Scaling Policy

**Metric**: Connection Count + CPU Utilization

```hcl
resource "aws_appautoscaling_target" "smtp_server" {
  max_capacity       = 20
  min_capacity       = 2
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.smtp_server.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

# Scale out on connection count
resource "aws_appautoscaling_policy" "smtp_connections_scale_out" {
  name               = "smtp-connections-scale-out"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.smtp_server.resource_id
  scalable_dimension = aws_appautoscaling_target.smtp_server.scalable_dimension
  service_namespace  = aws_appautoscaling_target.smtp_server.service_namespace

  target_tracking_scaling_policy_configuration {
    target_value = 500.0  # Target 500 connections per task

    customized_metric_specification {
      metric_name = "ActiveConnections"
      namespace   = "SMTP-Proxy"
      statistic   = "Average"

      dimensions {
        name  = "ServiceName"
        value = "smtp-server"
      }
    }

    scale_in_cooldown  = 300  # 5 minutes
    scale_out_cooldown = 60   # 1 minute
  }
}

# Scale out on CPU
resource "aws_appautoscaling_policy" "smtp_cpu_scale_out" {
  name               = "smtp-cpu-scale-out"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.smtp_server.resource_id
  scalable_dimension = aws_appautoscaling_target.smtp_server.scalable_dimension
  service_namespace  = aws_appautoscaling_target.smtp_server.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }

    target_value       = 70.0
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
}
```

**Capacity Formula**:

```
Connection capacity per task: ~2500 concurrent connections
Target throughput: 10,000 emails/min
Average email processing time: 2 seconds

Required tasks = (10,000 emails/min * 2 sec / 60 sec) / 2500 = 1.33 ≈ 2 tasks minimum

For headroom: min_capacity = 2, max_capacity = 20
```

#### Queue Worker Scaling Policy

**Metric**: Redis Queue Depth

```hcl
resource "aws_appautoscaling_target" "queue_worker" {
  max_capacity       = 10
  min_capacity       = 1
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.queue_worker.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "queue_depth_scale" {
  name               = "queue-depth-scale"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.queue_worker.resource_id
  scalable_dimension = aws_appautoscaling_target.queue_worker.scalable_dimension
  service_namespace  = aws_appautoscaling_target.queue_worker.service_namespace

  target_tracking_scaling_policy_configuration {
    target_value = 1000.0  # Target 1000 messages per worker

    customized_metric_specification {
      metric_name = "QueueDepth"
      namespace   = "SMTP-Proxy"
      statistic   = "Average"

      dimensions {
        name  = "QueueName"
        value = "email-processing"
      }
    }

    scale_in_cooldown  = 180  # 3 minutes
    scale_out_cooldown = 60   # 1 minute
  }
}
```

**CloudWatch Custom Metric** (published by queue worker):

```go
func publishQueueDepth(ctx context.Context, svc *cloudwatch.CloudWatch, queueName string, depth int64) error {
    _, err := svc.PutMetricDataWithContext(ctx, &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("SMTP-Proxy"),
        MetricData: []*cloudwatch.MetricDatum{
            {
                MetricName: aws.String("QueueDepth"),
                Value:      aws.Float64(float64(depth)),
                Unit:       aws.String("Count"),
                Timestamp:  aws.Time(time.Now()),
                Dimensions: []*cloudwatch.Dimension{
                    {
                        Name:  aws.String("QueueName"),
                        Value: aws.String(queueName),
                    },
                },
            },
        },
    })
    return err
}
```

#### API Server Scaling Policy

**Metric**: Request Count + Latency

```hcl
resource "aws_appautoscaling_target" "api_server" {
  max_capacity       = 10
  min_capacity       = 2
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.api_server.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

# ALB target tracking (built-in metric)
resource "aws_appautoscaling_policy" "api_request_count" {
  name               = "api-request-count"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.api_server.resource_id
  scalable_dimension = aws_appautoscaling_target.api_server.scalable_dimension
  service_namespace  = aws_appautoscaling_target.api_server.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ALBRequestCountPerTarget"
      resource_label         = "${aws_lb.http_alb.arn_suffix}/${aws_lb_target_group.api_server.arn_suffix}"
    }

    target_value       = 1000.0  # 1000 requests per target
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
}
```

#### Connection Draining

**ECS Service Configuration**:

```hcl
resource "aws_ecs_service" "smtp_server" {
  name            = "smtp-server"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.smtp_server.arn
  desired_count   = 2
  launch_type     = "FARGATE"

  deployment_configuration {
    minimum_healthy_percent = 100
    maximum_percent         = 200

    deployment_circuit_breaker {
      enable   = true
      rollback = true
    }
  }

  # Enable connection draining
  deployment_controller {
    type = "ECS"
  }

  # Graceful shutdown timeout
  health_check_grace_period_seconds = 60

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.smtp_server.id]
    assign_public_ip = false
  }
}
```

**Application-Level Graceful Shutdown**:

```go
func (s *SMTPServer) Shutdown(ctx context.Context) error {
    log.Info("Initiating graceful shutdown...")

    // Stop accepting new connections
    if err := s.listener.Close(); err != nil {
        return fmt.Errorf("failed to close listener: %w", err)
    }

    // Wait for active connections to complete
    done := make(chan struct{})
    go func() {
        s.activeConnections.Wait()
        close(done)
    }()

    select {
    case <-done:
        log.Info("All connections closed gracefully")
        return nil
    case <-ctx.Done():
        log.Warn("Shutdown timeout exceeded, forcing shutdown")
        return ctx.Err()
    }
}
```

#### Database Connection Pooling

**PgBouncer Configuration** (if using PgBouncer):

```ini
[databases]
smtp_proxy = host=smtp-proxy-db.xxxx.us-east-1.rds.amazonaws.com port=5432 dbname=smtp_proxy

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt

pool_mode = transaction
max_client_conn = 1000
default_pool_size = 25
min_pool_size = 5
reserve_pool_size = 5
reserve_pool_timeout = 3

server_lifetime = 3600
server_idle_timeout = 600
```

**Go pgx Connection Pool**:

```go
func NewDatabasePool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, fmt.Errorf("failed to parse connection string: %w", err)
    }

    // Connection pool settings
    config.MaxConns = 25
    config.MinConns = 5
    config.MaxConnLifetime = time.Hour
    config.MaxConnIdleTime = 30 * time.Minute
    config.HealthCheckPeriod = time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("failed to create connection pool: %w", err)
    }

    // Verify connectivity
    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return pool, nil
}
```

**Capacity Formula**:

```
Per ECS task: 25 max connections
Number of tasks (at max scale): 20 SMTP + 10 API + 10 Worker = 40 tasks
Total max connections: 40 * 25 = 1000 connections

RDS max_connections setting: 1200 (with 200 buffer for admin/maintenance)
```

#### Backpressure Mechanism

**SMTP Server Queue Check**:

```go
func (s *SMTPServer) HandleConnection(conn net.Conn) error {
    // Check queue depth before accepting email
    queueDepth, err := s.redis.XLen(ctx, "email-processing").Result()
    if err != nil {
        return fmt.Errorf("failed to check queue depth: %w", err)
    }

    // Critical threshold: 50,000 messages
    const criticalThreshold = 50000

    if queueDepth > criticalThreshold {
        // Return 421 (service unavailable) to apply backpressure
        if err := s.sendSMTPResponse(conn, 421, "Service temporarily unavailable, try again later"); err != nil {
            return err
        }

        // Increment backpressure metric
        s.metrics.IncrementBackpressureCount()

        return nil
    }

    // Normal processing continues...
    return s.processEmail(conn)
}
```

**CloudWatch Alarm for Backpressure**:

```hcl
resource "aws_cloudwatch_metric_alarm" "queue_backpressure" {
  alarm_name          = "smtp-proxy-queue-backpressure"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "QueueDepth"
  namespace           = "SMTP-Proxy"
  period              = 60
  statistic           = "Average"
  threshold           = 50000
  alarm_description   = "Queue depth exceeds critical threshold"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  dimensions = {
    QueueName = "email-processing"
  }
}
```

## TRACEABILITY

This SPEC is referenced by:
- SPEC-SMTP-001: SMTP server implementation depends on TLS certificate management
- SPEC-API-001: API server deployment uses infrastructure defined here
- SPEC-QUEUE-001: Queue worker scaling policies defined here
- SPEC-SECURITY-001: Security requirements reference TLS configuration
- SPEC-MONITORING-001: CloudWatch monitoring integrates with scaling policies

This SPEC references:
- Project Constitution: `.moai/project/tech.md` for technology stack
- SPEC-DATABASE-001: Database schema migrations for RDS setup
- SPEC-FRONTEND-001: Frontend deployment via ECS Fargate

## EXPERT CONSULTATION RECOMMENDATIONS

### Backend Expert Consultation

This SPEC involves significant backend infrastructure and deployment architecture. Consider consulting with `expert-backend` for:

- **ECS Task Definition Optimization**: Review CPU/memory allocations and resource limits
- **Database Connection Pooling Strategy**: Validate PgBouncer vs pgx pool decision
- **Redis Cluster Configuration**: Review ElastiCache cluster mode settings
- **Auto-Scaling Threshold Tuning**: Validate capacity formulas and scaling thresholds
- **Graceful Shutdown Implementation**: Review connection draining patterns

### DevOps Expert Consultation

This SPEC contains extensive deployment and operations requirements. Consider consulting with `expert-devops` for:

- **Terraform Module Architecture**: Review IaC structure and module organization
- **Blue/Green Deployment Strategy**: Validate CodeDeploy configuration
- **CloudWatch Monitoring and Alarms**: Review metric collection and alerting strategy
- **Secret Management**: Validate AWS Secrets Manager integration
- **CI/CD Integration**: Review deployment pipeline requirements

### Security Expert Consultation

This SPEC includes TLS certificate management and AWS security. Consider consulting with `expert-security` for:

- **TLS Configuration Best Practices**: Review cipher suites and protocol versions
- **Certificate Rotation Strategy**: Validate Let's Encrypt renewal automation
- **IAM Role Permissions**: Review task role and execution role policies
- **Network Security Groups**: Validate ingress/egress rules
- **Secret Storage**: Review AWS Secrets Manager vs Parameter Store decision

## NOTES

### Implementation Priority

1. **Phase 1** (Week 1-2): Docker Compose local development environment + self-signed certificates
2. **Phase 2** (Week 3): Test SMTP client implementation
3. **Phase 3** (Week 4-5): Terraform infrastructure modules (VPC, RDS, ElastiCache, ECS)
4. **Phase 4** (Week 6): Let's Encrypt integration and certificate management
5. **Phase 5** (Week 7-8): Auto-scaling policies and CloudWatch monitoring
6. **Phase 6** (Week 9): Blue/green deployment with CodeDeploy
7. **Phase 7** (Week 10): Load testing and capacity validation

### Known Limitations

- Initial implementation will use single-region deployment (multi-region is out of scope)
- Let's Encrypt rate limits may require careful planning for testing
- ECS Fargate cold start latency may affect initial scale-out performance
- Database read replicas introduce eventual consistency (not suitable for all queries)

### Alternative Approaches Considered

1. **Kubernetes instead of ECS**:
   - Pros: More portable, richer ecosystem, better observability tools
   - Cons: Higher operational complexity, team lacks Kubernetes expertise
   - Decision: Stick with ECS Fargate for simplicity and AWS integration

2. **EC2-based ECS instead of Fargate**:
   - Pros: Lower cost at high scale, more control over instance types
   - Cons: Higher operational burden, instance management overhead
   - Decision: Use Fargate for serverless benefits; revisit if cost becomes issue

3. **Application-managed certificates instead of Let's Encrypt**:
   - Pros: More control, no rate limits
   - Cons: Manual renewal, higher cost for commercial CA
   - Decision: Use Let's Encrypt for automation and zero cost

4. **Amazon Aurora instead of RDS PostgreSQL**:
   - Pros: Better scaling, automatic failover, read replicas
   - Cons: Higher cost, vendor lock-in
   - Decision: Start with RDS PostgreSQL; migrate to Aurora if needed

### Performance Targets

- **SMTP Connection Latency**: < 100ms for TLS handshake
- **Email Processing Throughput**: 10,000 emails/min at scale
- **API Response Time**: P95 < 200ms, P99 < 500ms
- **Queue Processing Latency**: < 5 seconds from enqueue to delivery attempt
- **Auto-Scaling Response Time**: < 30 seconds from trigger to new task running
- **Certificate Reload**: < 1 second without connection drops

### Cost Estimation (Monthly)

- **ECS Fargate**: ~$200 (2 SMTP + 2 API + 1 Worker tasks baseline)
- **RDS PostgreSQL**: ~$150 (db.t4g.medium multi-AZ)
- **ElastiCache Redis**: ~$100 (cache.t4g.medium 3-node cluster)
- **NLB**: ~$20 (fixed cost + data transfer)
- **ALB**: ~$25 (fixed cost + LCU charges)
- **Data Transfer**: ~$50 (outbound data)
- **CloudWatch**: ~$30 (logs + metrics)
- **Total Baseline**: ~$575/month

**At Scale (20 tasks total)**:
- ECS Fargate: ~$800
- Total: ~$1,175/month
