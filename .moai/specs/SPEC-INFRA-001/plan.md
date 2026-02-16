# Implementation Plan: SPEC-INFRA-001

## Overview

This document outlines the implementation plan for the Infrastructure, Deployment, and Scaling specification (SPEC-INFRA-001). The plan is organized into 7 phases over 10 weeks, with clear milestones, task decomposition, technology choices, and risk mitigation strategies.

## Executive Summary

**Goal**: Deliver production-ready infrastructure for smtp-proxy with local development environment, TLS certificate management, AWS ECS deployment, and auto-scaling capabilities.

**Duration**: 10 weeks (estimated)

**Team Size**: 1-2 developers + 1 DevOps engineer

**Key Deliverables**:
1. Docker Compose local development environment
2. Test SMTP client for validation
3. AWS ECS Fargate infrastructure (Terraform)
4. Let's Encrypt certificate automation
5. Auto-scaling policies for all services
6. Blue/green deployment pipeline
7. CloudWatch monitoring and alerting

## Technology Stack

### Infrastructure as Code

- **Terraform 1.9+**: Infrastructure provisioning and management
- **Terraform Cloud**: State management and collaboration
- **AWS Provider**: Version ~> 5.0

**Rationale**: Terraform is industry-standard for IaC with strong AWS support. Declarative syntax enables predictable deployments and easy rollbacks.

### Container Orchestration

- **Docker 24.x+**: Container runtime
- **Docker Compose 2.x+**: Local development orchestration
- **AWS ECS Fargate**: Production container orchestration

**Rationale**: Docker Compose provides simple local development. ECS Fargate offers serverless container management without Kubernetes complexity.

### TLS Certificate Management

- **Let's Encrypt**: Free automated certificate authority
- **Go autocert**: `golang.org/x/crypto/acme/autocert` for production
- **OpenSSL**: Self-signed certificate generation for development

**Rationale**: Let's Encrypt provides free, automated, and widely-trusted certificates. Go autocert integrates seamlessly with Go SMTP server.

### Load Balancing

- **AWS Network Load Balancer (NLB)**: SMTP traffic (TCP passthrough)
- **AWS Application Load Balancer (ALB)**: HTTP/HTTPS traffic

**Rationale**: NLB supports TCP passthrough preserving TLS end-to-end. ALB provides HTTP-specific features for web traffic.

### Database and Caching

- **Amazon RDS PostgreSQL 17**: Managed relational database
- **Amazon ElastiCache Redis 7.4**: Managed in-memory cache

**Rationale**: Managed services reduce operational burden. Multi-AZ deployment ensures high availability.

### Monitoring and Deployment

- **AWS CloudWatch**: Metrics, logs, and alarms
- **AWS CodeDeploy**: Blue/green deployment automation
- **AWS Cloud Map**: Service discovery

**Rationale**: Native AWS services provide tight integration and reduced third-party dependencies.

### Testing

- **Go 1.23+**: Test SMTP client implementation
- **net/smtp**: Standard library SMTP client
- **crypto/tls**: TLS configuration

**Rationale**: Standard library provides robust SMTP and TLS support without external dependencies.

## File Structure

```
smtp-proxy/
├── infra/
│   ├── docker/
│   │   ├── docker-compose.yml           # Local development compose file
│   │   ├── docker-compose.prod.yml      # Production-like local testing
│   │   ├── .env.example                 # Environment variable template
│   │   └── scripts/
│   │       ├── generate-dev-certs.sh    # Self-signed cert generation
│   │       └── init-db.sh               # Database initialization
│   ├── terraform/
│   │   └── aws/
│   │       ├── main.tf                  # Root module
│   │       ├── variables.tf             # Input variables
│   │       ├── outputs.tf               # Output values
│   │       ├── terraform.tfvars.example # Variable values template
│   │       └── modules/
│   │           ├── vpc/                 # VPC and networking
│   │           │   ├── main.tf
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           ├── rds/                 # PostgreSQL RDS
│   │           │   ├── main.tf
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           ├── elasticache/         # Redis cluster
│   │           │   ├── main.tf
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           ├── ecs/                 # ECS cluster and services
│   │           │   ├── main.tf
│   │           │   ├── task-definitions/
│   │           │   │   ├── smtp-server.json
│   │           │   │   ├── api-server.json
│   │           │   │   ├── queue-worker.json
│   │           │   │   └── frontend.json
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           ├── nlb/                 # Network Load Balancer
│   │           │   ├── main.tf
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           ├── alb/                 # Application Load Balancer
│   │           │   ├── main.tf
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           ├── cloudwatch/          # Monitoring and alarms
│   │           │   ├── main.tf
│   │           │   ├── dashboards/
│   │           │   │   ├── smtp-server.json
│   │           │   │   ├── api-server.json
│   │           │   │   └── queue-worker.json
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           ├── codedeploy/          # Blue/green deployment
│   │           │   ├── main.tf
│   │           │   ├── variables.tf
│   │           │   └── outputs.tf
│   │           └── security/            # IAM roles, security groups
│   │               ├── main.tf
│   │               ├── iam-roles.tf
│   │               ├── security-groups.tf
│   │               ├── variables.tf
│   │               └── outputs.tf
│   └── scripts/
│       ├── deploy.sh                    # Deployment automation script
│       ├── rollback.sh                  # Rollback automation
│       └── validate-infra.sh            # Infrastructure validation
├── backend/
│   ├── cmd/
│   │   ├── smtp-server/
│   │   │   ├── Dockerfile               # SMTP server container
│   │   │   └── main.go
│   │   ├── api-server/
│   │   │   ├── Dockerfile               # API server container
│   │   │   └── main.go
│   │   ├── queue-worker/
│   │   │   ├── Dockerfile               # Queue worker container
│   │   │   └── main.go
│   │   └── test-client/                 # Test SMTP client
│   │       ├── main.go                  # CLI entry point
│   │       ├── client.go                # SMTP client implementation
│   │       ├── tls.go                   # TLS configuration
│   │       └── README.md                # Usage documentation
│   └── internal/
│       └── certmanager/                 # Certificate management
│           ├── acme.go                  # Let's Encrypt ACME client
│           ├── reload.go                # Hot-reload mechanism
│           └── storage.go               # Certificate storage
├── frontend/
│   └── Dockerfile                       # Frontend container
└── docs/
    ├── deployment/
    │   ├── local-development.md         # Docker Compose setup guide
    │   ├── aws-deployment.md            # AWS ECS deployment guide
    │   ├── tls-certificates.md          # Certificate management guide
    │   └── auto-scaling.md              # Scaling configuration guide
    └── operations/
        ├── monitoring.md                # CloudWatch monitoring guide
        ├── troubleshooting.md           # Common issues and solutions
        └── runbooks/
            ├── scale-out.md             # Manual scale-out procedure
            ├── certificate-renewal.md   # Manual cert renewal
            └── rollback.md              # Deployment rollback procedure
```

## Implementation Phases

### Phase 1: Docker Compose Local Development (Week 1-2)

**Priority**: P0 (Critical)

**Goal**: Enable local development with all services running via Docker Compose.

#### Tasks

1. **Create docker-compose.yml**
   - Duration: 1 day
   - Define all services (postgres, redis, smtp-server, api-server, queue-worker, frontend)
   - Configure health checks for each service
   - Setup volume mounts for hot-reload
   - Configure network isolation

2. **Implement certificate generation script**
   - Duration: 1 day
   - Write `generate-dev-certs.sh` using OpenSSL
   - Generate CA certificate for local trust
   - Generate server certificates with SAN for localhost and 127.0.0.1
   - Document manual certificate trust process for different OSes

3. **Create .env.example template**
   - Duration: 0.5 day
   - Document all required environment variables
   - Provide secure defaults for development
   - Include comments explaining each variable

4. **Configure service dependencies**
   - Duration: 0.5 day
   - Ensure PostgreSQL starts before application services
   - Ensure Redis starts before queue worker
   - Configure wait-for scripts if needed

5. **Write Dockerfiles for each service**
   - Duration: 2 days
   - Multi-stage builds for minimal image size
   - Non-root user for security
   - Health check commands
   - Optimize layer caching for Go dependencies

6. **Test and document local setup**
   - Duration: 1 day
   - Write `docs/deployment/local-development.md`
   - Test on Linux, macOS, and Windows WSL2
   - Document common issues and solutions

#### Acceptance Criteria

- ✅ `docker-compose up` starts all services successfully
- ✅ Self-signed certificates generated automatically on first run
- ✅ All health checks pass within 60 seconds
- ✅ Hot-reload works for Go and Next.js code changes
- ✅ Services can communicate via Docker network
- ✅ Database migrations run automatically
- ✅ Documentation covers setup for all supported OSes

#### Risks and Mitigation

**Risk**: Certificate trust issues on different operating systems
- **Mitigation**: Provide clear OS-specific instructions; use `--insecure` flag for test client

**Risk**: Port conflicts on developer machines
- **Mitigation**: Use non-standard ports (e.g., 5870 instead of 587); document port configuration

**Risk**: Slow initial startup due to image builds
- **Mitigation**: Pre-build images; provide `docker-compose pull` command in docs

### Phase 2: Test SMTP Client (Week 3)

**Priority**: P0 (Critical)

**Goal**: Build standalone test client for SMTP server validation.

#### Tasks

1. **Implement basic SMTP client**
   - Duration: 2 days
   - Use `net/smtp` standard library
   - Support STARTTLS and implicit TLS modes
   - Implement SMTP AUTH (PLAIN and LOGIN)
   - CLI argument parsing with `flag` or `cobra`

2. **Add TLS configuration options**
   - Duration: 1 day
   - Support self-signed certificate trust
   - Implement `--insecure` flag for development
   - Certificate validation and error reporting

3. **Implement batch sending**
   - Duration: 1 day
   - Rate limiting using `time.Ticker`
   - Progress reporting
   - Error handling and retry logic

4. **Write usage documentation**
   - Duration: 0.5 day
   - CLI help text
   - Example commands in README
   - Common use cases and troubleshooting

5. **Add logging and metrics**
   - Duration: 0.5 day
   - Detailed connection logs
   - Success/failure counters
   - Latency measurements

#### Acceptance Criteria

- ✅ Client successfully sends single email via STARTTLS
- ✅ Client successfully sends single email via implicit TLS
- ✅ Client supports both PLAIN and LOGIN authentication
- ✅ Batch mode sends 100 emails at configurable rate
- ✅ `--insecure` flag works with self-signed certificates
- ✅ Clear error messages for common failure scenarios
- ✅ README includes usage examples

#### Risks and Mitigation

**Risk**: SMTP protocol edge cases not handled
- **Mitigation**: Test against known SMTP servers (Gmail, Mailgun) for compatibility

**Risk**: Performance issues with large batch sends
- **Mitigation**: Implement connection pooling; add connection reuse option

### Phase 3: Terraform Infrastructure Modules (Week 4-5)

**Priority**: P0 (Critical)

**Goal**: Create Terraform modules for all AWS infrastructure.

#### Tasks

1. **VPC Module**
   - Duration: 1 day
   - Create VPC with public and private subnets across 3 AZs
   - Configure NAT gateways for private subnet internet access
   - Setup route tables and security groups

2. **RDS Module**
   - Duration: 1.5 days
   - Create PostgreSQL 17 instance with multi-AZ
   - Configure parameter group for performance
   - Setup read replica
   - Enable automated backups and point-in-time recovery
   - Create security group allowing access from ECS tasks only

3. **ElastiCache Module**
   - Duration: 1 day
   - Create Redis cluster with 3 nodes (1 primary + 2 replicas)
   - Enable cluster mode for sharding
   - Configure automatic failover
   - Setup security group for ECS task access

4. **ECS Module**
   - Duration: 2 days
   - Create ECS cluster
   - Define task definitions for all services (smtp-server, api-server, queue-worker, frontend)
   - Configure ECR repositories for container images
   - Setup IAM roles (execution role, task role)
   - Define ECS services with desired count

5. **NLB Module**
   - Duration: 1 day
   - Create Network Load Balancer for SMTP traffic
   - Configure listeners for ports 587 and 465
   - Setup target groups with health checks
   - Configure cross-zone load balancing

6. **ALB Module**
   - Duration: 1 day
   - Create Application Load Balancer for HTTP traffic
   - Configure HTTPS listener with ACM certificate
   - Setup target groups for API server and frontend
   - Configure path-based routing rules

7. **CloudWatch Module**
   - Duration: 1 day
   - Create log groups for all services
   - Define custom metrics for queue depth and connection count
   - Create CloudWatch dashboards
   - Setup basic alarms (high error rate, service unavailable)

8. **Security Module**
   - Duration: 1 day
   - Define all IAM roles and policies
   - Create security groups with least privilege
   - Setup AWS Secrets Manager secrets
   - Configure KMS keys for encryption

9. **Root Module Integration**
   - Duration: 1 day
   - Wire all modules together in `main.tf`
   - Define input variables with validation
   - Create output values for important resources
   - Write terraform.tfvars.example

10. **Testing and Documentation**
    - Duration: 1 day
    - Test infrastructure provisioning in dev account
    - Validate network connectivity between components
    - Write `docs/deployment/aws-deployment.md`
    - Document variable configuration

#### Acceptance Criteria

- ✅ `terraform plan` executes without errors
- ✅ `terraform apply` creates all infrastructure in ~15 minutes
- ✅ All modules are self-contained with clear interfaces
- ✅ Security groups follow least privilege principle
- ✅ Secrets are stored in AWS Secrets Manager (no hardcoded values)
- ✅ Infrastructure can be destroyed cleanly with `terraform destroy`
- ✅ Documentation covers all configuration variables

#### Risks and Mitigation

**Risk**: Terraform state conflicts in team environment
- **Mitigation**: Use Terraform Cloud or S3 backend with state locking (DynamoDB)

**Risk**: AWS service quotas exceeded
- **Mitigation**: Request quota increases proactively; document required quotas

**Risk**: High AWS costs during development
- **Mitigation**: Use smaller instance types (t4g family); implement auto-shutdown for dev environments

**Risk**: Complex module dependencies causing circular references
- **Mitigation**: Use data sources for cross-module references; careful module design

### Phase 4: Let's Encrypt Integration (Week 6)

**Priority**: P1 (High)

**Goal**: Implement automatic certificate management with Let's Encrypt.

#### Tasks

1. **Implement ACME client in Go**
   - Duration: 2 days
   - Use `golang.org/x/crypto/acme/autocert` package
   - Implement HTTP-01 challenge handler
   - Configure certificate storage (filesystem + S3 backup)
   - Setup automatic renewal scheduler

2. **Implement certificate hot-reload**
   - Duration: 1.5 days
   - File watcher for certificate changes (using `fsnotify`)
   - TLS config reload without server restart
   - Atomic certificate replacement
   - Graceful handling of reload failures

3. **Add SNI support**
   - Duration: 1 day
   - Multi-domain certificate cache
   - Dynamic certificate selection based on SNI
   - Fallback certificate for unknown domains

4. **Configure DNS for ACME challenges**
   - Duration: 0.5 day
   - Route 53 configuration for domain
   - DNS-01 challenge support (if needed for wildcard certs)

5. **Testing and monitoring**
   - Duration: 1 day
   - Test certificate acquisition in staging environment
   - Test automatic renewal (mock expiry date)
   - Add CloudWatch metrics for certificate expiry
   - Create alarm for renewal failures

#### Acceptance Criteria

- ✅ Certificate automatically acquired from Let's Encrypt on first run
- ✅ Certificate renews automatically 30 days before expiry
- ✅ Certificate reload happens without SMTP connection drops
- ✅ SNI works for multiple domains
- ✅ CloudWatch alarm triggers on renewal failure
- ✅ Certificate backup stored in S3
- ✅ Documentation covers certificate management

#### Risks and Mitigation

**Risk**: Let's Encrypt rate limits during testing
- **Mitigation**: Use Let's Encrypt staging environment; implement exponential backoff

**Risk**: Certificate reload causes brief connection failures
- **Mitigation**: Implement atomic TLS config replacement; thorough testing

**Risk**: DNS propagation delays for ACME challenges
- **Mitigation**: Use HTTP-01 challenge; increase challenge timeout

### Phase 5: Auto-Scaling Policies (Week 7-8)

**Priority**: P1 (High)

**Goal**: Implement auto-scaling for all ECS services.

#### Tasks

1. **Implement custom CloudWatch metrics**
   - Duration: 1.5 days
   - Publish active connection count from SMTP server
   - Publish queue depth from queue worker
   - Add metric publishing to application code
   - Test metric ingestion in CloudWatch

2. **Configure SMTP server scaling**
   - Duration: 1 day
   - Create Application Auto Scaling target
   - Define target tracking policy for connection count
   - Define CPU utilization policy
   - Test scaling behavior with synthetic load

3. **Configure queue worker scaling**
   - Duration: 1 day
   - Create Application Auto Scaling target
   - Define target tracking policy for queue depth
   - Test scaling with simulated queue growth

4. **Configure API server scaling**
   - Duration: 0.5 day
   - Create Application Auto Scaling target
   - Use ALB request count per target metric
   - Test scaling with load testing tool (k6 or wrk)

5. **Implement connection draining**
   - Duration: 1.5 days
   - Add graceful shutdown to SMTP server
   - Track active connections in-memory
   - Wait for connection completion before shutdown
   - Configure deregistration delay in target groups

6. **Implement backpressure mechanism**
   - Duration: 1 day
   - Check queue depth before accepting emails
   - Return 421 SMTP response when queue is full
   - Add CloudWatch metric for backpressure events

7. **Database connection pooling**
   - Duration: 1 day
   - Configure pgx connection pool parameters
   - Calculate optimal pool size based on task count
   - Test pool exhaustion scenarios
   - Add connection pool metrics

8. **Load testing and capacity validation**
   - Duration: 2 days
   - Create load testing scenarios (gradual ramp, sudden spike)
   - Validate auto-scaling behavior
   - Measure scale-out response time
   - Test scale-in with connection draining
   - Document capacity planning formulas

#### Acceptance Criteria

- ✅ SMTP server scales out when connection count > 500 per task
- ✅ Queue worker scales out when queue depth > 1000 per worker
- ✅ API server scales out based on ALB request count
- ✅ Scale-out completes within 30 seconds of threshold breach
- ✅ Scale-in drains connections gracefully (no dropped connections)
- ✅ Backpressure activates when queue depth > 50,000
- ✅ System handles 10,000 emails/min at full scale
- ✅ Database connection pool never exhausts

#### Risks and Mitigation

**Risk**: Auto-scaling too aggressive causing cost overruns
- **Mitigation**: Set conservative max capacity; implement cost alerts

**Risk**: Slow scale-out leads to service degradation
- **Mitigation**: Use aggressive scale-out thresholds; implement pre-warming for known peaks

**Risk**: Connection draining timeout too short
- **Mitigation**: Analyze typical connection duration; set timeout to P99 duration + buffer

**Risk**: Database connection pool exhaustion during scale events
- **Mitigation**: Over-provision pool size; implement connection retry logic

### Phase 6: Blue/Green Deployment (Week 9)

**Priority**: P2 (Medium)

**Goal**: Implement zero-downtime deployments with automatic rollback.

#### Tasks

1. **Configure CodeDeploy**
   - Duration: 1.5 days
   - Create CodeDeploy application and deployment groups
   - Configure blue/green deployment settings
   - Setup ALB for traffic shifting
   - Define deployment lifecycle hooks

2. **Implement health check endpoints**
   - Duration: 1 day
   - Add `/health` endpoint to all services
   - Include dependency checks (database, Redis)
   - Return appropriate HTTP status codes
   - Add version information to health response

3. **Create deployment automation scripts**
   - Duration: 1.5 days
   - Write `deploy.sh` script
   - Build and push container images to ECR
   - Update ECS task definitions
   - Trigger CodeDeploy deployment
   - Monitor deployment progress

4. **Implement automatic rollback**
   - Duration: 1 day
   - Configure CodeDeploy rollback triggers
   - Define rollback on health check failure
   - Define rollback on CloudWatch alarm
   - Test rollback scenarios

5. **Testing and documentation**
   - Duration: 1 day
   - Test successful deployment
   - Test rollback on health check failure
   - Write deployment runbook
   - Document rollback procedure

#### Acceptance Criteria

- ✅ New deployment completes without downtime
- ✅ Traffic gradually shifts from blue to green
- ✅ Automatic rollback on health check failure
- ✅ Deployment script automates entire process
- ✅ Rollback completes within 5 minutes
- ✅ Documentation includes deployment and rollback procedures

#### Risks and Mitigation

**Risk**: Health checks too strict causing false positive rollbacks
- **Mitigation**: Tune health check thresholds; use grace period

**Risk**: Rollback fails leaving system in partial state
- **Mitigation**: Thoroughly test rollback scenarios; implement manual rollback procedure

**Risk**: Long deployment time due to traffic shift duration
- **Mitigation**: Optimize traffic shift interval; allow manual approval for critical deployments

### Phase 7: Load Testing and Validation (Week 10)

**Priority**: P2 (Medium)

**Goal**: Validate system performance and scaling under production-like load.

#### Tasks

1. **Create load testing scenarios**
   - Duration: 1 day
   - Scenario 1: Gradual ramp (0 to 10K emails/min over 30 minutes)
   - Scenario 2: Sudden spike (0 to 10K emails/min in 1 minute)
   - Scenario 3: Sustained high load (10K emails/min for 2 hours)
   - Scenario 4: Queue backpressure (saturate queue to trigger 421 responses)

2. **Execute load tests**
   - Duration: 2 days
   - Run each scenario multiple times
   - Monitor CloudWatch metrics during tests
   - Record scale-out and scale-in timing
   - Measure email delivery latency

3. **Analyze results and tune**
   - Duration: 1.5 days
   - Analyze auto-scaling behavior
   - Tune scaling thresholds if needed
   - Optimize database queries if bottlenecks found
   - Adjust connection pool sizes

4. **Validate capacity planning formulas**
   - Duration: 0.5 day
   - Compare actual vs predicted task counts
   - Update capacity formulas in documentation
   - Create capacity planning spreadsheet

5. **Create operational dashboards**
   - Duration: 1 day
   - Build CloudWatch dashboard for operations team
   - Include key metrics (throughput, latency, error rate, queue depth)
   - Add cost tracking widgets
   - Share dashboard with team

#### Acceptance Criteria

- ✅ System handles 10,000 emails/min sustained load
- ✅ Scale-out completes within 30 seconds
- ✅ P95 email delivery latency < 5 seconds
- ✅ Zero dropped connections during scale events
- ✅ Backpressure activates correctly at queue threshold
- ✅ Capacity planning formulas accurate within 10%
- ✅ Operational dashboard provides actionable insights

#### Risks and Mitigation

**Risk**: Load testing causes AWS cost spike
- **Mitigation**: Use small instance types for testing; set cost alerts; clean up resources immediately after tests

**Risk**: Load testing impacts production if using shared infrastructure
- **Mitigation**: Use dedicated test environment; isolate network and database

**Risk**: Test results not representative of production traffic patterns
- **Mitigation**: Analyze expected production patterns; design realistic test scenarios

## Architecture Decisions

### Decision 1: ECS Fargate vs ECS EC2

**Context**: Need container orchestration platform for production.

**Options**:
1. ECS Fargate (serverless)
2. ECS EC2 (self-managed instances)
3. Kubernetes (EKS or self-hosted)

**Decision**: ECS Fargate

**Rationale**:
- **Pros**: No instance management, automatic scaling, pay-per-task, strong AWS integration
- **Cons**: Higher per-task cost at very high scale, less control over instance types

**Trade-offs**:
- Accept ~20% higher cost for significantly reduced operational burden
- Sacrifice fine-grained instance control for simplicity
- ECS Fargate aligns with team's AWS expertise (no Kubernetes knowledge)

**Validation**: Monitor costs after launch; migrate to EC2-based ECS if cost becomes prohibitive

### Decision 2: Let's Encrypt vs Purchased Certificates

**Context**: Need TLS certificates for production SMTP server.

**Options**:
1. Let's Encrypt (free, automated)
2. Commercial CA (DigiCert, Sectigo)
3. AWS Certificate Manager (free, but limited to AWS resources)

**Decision**: Let's Encrypt with `autocert`

**Rationale**:
- **Pros**: Zero cost, automatic renewal, widely trusted, programmatic integration
- **Cons**: 90-day expiry requires automation, rate limits, HTTP-01 challenge requires port 80 access

**Trade-offs**:
- Accept 90-day renewal cycle for zero cost
- Implement robust renewal automation to mitigate short expiry
- Cannot use ACM because NLB requires TCP passthrough (no TLS termination at LB)

**Validation**: Test renewal automation thoroughly; set up alerts for renewal failures

### Decision 3: Database Connection Pooling Strategy

**Context**: ECS tasks need database connections; need to avoid exhausting RDS connections.

**Options**:
1. PgBouncer sidecar container
2. Application-level pooling (pgx pool)
3. RDS Proxy (AWS managed connection pooling)

**Decision**: Application-level pooling with pgx

**Rationale**:
- **Pros**: No additional infrastructure, lower latency, simpler deployment
- **Cons**: Per-task pools (not shared across tasks), requires careful sizing

**Trade-offs**:
- Accept higher total connection count for simpler architecture
- Calculate max connections: tasks × pool_size must not exceed RDS max_connections
- Formula: 20 SMTP + 10 API + 10 Worker = 40 tasks × 25 connections = 1000 (within RDS limit of 1200)

**Validation**: Monitor connection usage; switch to PgBouncer or RDS Proxy if exhaustion occurs

### Decision 4: Blue/Green vs Rolling Deployment

**Context**: Need deployment strategy for zero-downtime releases.

**Options**:
1. Blue/green (CodeDeploy)
2. Rolling update (native ECS)
3. Canary deployment

**Decision**: Blue/green with CodeDeploy

**Rationale**:
- **Pros**: Instant rollback, full environment testing, clear traffic shift control
- **Cons**: Requires 2× resources during deployment, more complex setup

**Trade-offs**:
- Accept temporary cost increase (2× tasks for ~10 minutes) for safety
- Gain instant rollback capability (critical for email infrastructure)
- Simplified validation (green environment fully running before traffic shift)

**Validation**: Test rollback scenarios; measure deployment duration; document manual approval process

### Decision 5: Auto-Scaling Metrics

**Context**: Need metrics to drive auto-scaling decisions.

**Options**:
1. CPU/memory utilization only
2. Custom metrics (connection count, queue depth)
3. Combination approach

**Decision**: Combination approach with custom metrics

**Rationale**:
- **Pros**: More accurate scaling triggers, better resource utilization, prevents overload
- **Cons**: More complex setup, custom metric publishing required

**Metrics**:
- SMTP server: Connection count (primary) + CPU (secondary)
- Queue worker: Queue depth (primary) + CPU (secondary)
- API server: ALB request count (primary) + CPU (secondary)

**Trade-offs**:
- Accept complexity for better scaling accuracy
- Custom metrics provide domain-specific scaling signals
- CPU is fallback metric when custom metric collection fails

**Validation**: Load testing to validate scaling behavior; compare vs CPU-only scaling

## Scaling Formulas

### SMTP Server Capacity

**Formula**:
```
Required tasks = (target_emails_per_min × avg_processing_time_sec / 60) / connections_per_task

Where:
- target_emails_per_min = 10,000
- avg_processing_time_sec = 2
- connections_per_task = 2,500

Result: (10,000 × 2 / 60) / 2,500 = 0.133 tasks

For headroom: min_capacity = 2, max_capacity = 20
```

**Capacity Table**:

| Emails/Min | Tasks (min) | Tasks (actual) | Headroom |
|------------|-------------|----------------|----------|
| 1,000      | 0.013       | 2              | 150x     |
| 5,000      | 0.067       | 2              | 30x      |
| 10,000     | 0.133       | 2              | 15x      |
| 25,000     | 0.333       | 3              | 9x       |
| 50,000     | 0.667       | 4              | 6x       |
| 100,000    | 1.333       | 5              | 3.75x    |

**Auto-Scaling Trigger**: Connection count > 500 per task

### Queue Worker Capacity

**Formula**:
```
Required workers = queue_depth / messages_per_worker_target

Where:
- messages_per_worker_target = 1,000
- queue_depth = dynamic (measured via CloudWatch)

Result: Scales proportionally to queue depth
```

**Capacity Table**:

| Queue Depth | Workers (min) | Workers (actual) |
|-------------|---------------|------------------|
| 500         | 0.5           | 1                |
| 1,000       | 1             | 1                |
| 5,000       | 5             | 5                |
| 10,000      | 10            | 10 (max)         |
| 50,000      | 50            | 10 (max, backpressure active) |

**Auto-Scaling Trigger**: Queue depth > 1,000 per worker

**Backpressure Threshold**: Queue depth > 50,000 (SMTP server returns 421)

### Database Connection Pool

**Formula**:
```
Total connections = (num_smtp_tasks × pool_size) + (num_api_tasks × pool_size) + (num_worker_tasks × pool_size)

Where:
- pool_size = 25 (per task)
- num_smtp_tasks_max = 20
- num_api_tasks_max = 10
- num_worker_tasks_max = 10

Result: (20 + 10 + 10) × 25 = 1,000 connections

RDS max_connections: 1,200 (200 buffer for admin/monitoring)
```

**Pool Configuration** (per task):
```
max_conns = 25
min_conns = 5
max_conn_lifetime = 1 hour
max_conn_idle_time = 30 minutes
health_check_period = 1 minute
```

## Risk Analysis

### Technical Risks

#### Risk 1: Certificate Renewal Failure

**Severity**: High

**Impact**: SMTP server unable to accept TLS connections after certificate expiry

**Likelihood**: Low (with proper automation)

**Mitigation**:
1. Implement robust retry logic with exponential backoff
2. Setup CloudWatch alarm for renewal failures
3. Send SNS notification to operations team
4. Store backup certificates in S3
5. Document manual renewal procedure

**Contingency**: Manual certificate renewal using `certbot` if automation fails

#### Risk 2: ECS Task Launch Delays

**Severity**: Medium

**Impact**: Slow auto-scaling response during traffic spikes

**Likelihood**: Medium (Fargate cold start ~15-30 seconds)

**Mitigation**:
1. Set aggressive scale-out thresholds (trigger early)
2. Maintain higher min_capacity during peak hours
3. Implement backpressure to protect system
4. Consider pre-warming tasks before known traffic events

**Contingency**: Manually increase desired count before expected traffic spike

#### Risk 3: Database Connection Pool Exhaustion

**Severity**: High

**Impact**: Application errors, failed email processing

**Likelihood**: Medium (at high scale)

**Mitigation**:
1. Calculate pool sizes conservatively
2. Monitor connection usage via CloudWatch
3. Implement connection retry logic in application
4. Set RDS max_connections to 1,200 (exceeding calculated need by 200)

**Contingency**: Reduce ECS task count or implement PgBouncer

#### Risk 4: Redis Cluster Failover Latency

**Severity**: Medium

**Impact**: Brief queue processing delays during failover

**Likelihood**: Low (Redis automatic failover typically < 30 seconds)

**Mitigation**:
1. Enable automatic failover in ElastiCache
2. Implement exponential backoff retry in workers
3. Monitor failover events via CloudWatch
4. Multi-AZ deployment reduces failover need

**Contingency**: Queue workers will automatically retry; no manual intervention needed

### Operational Risks

#### Risk 5: AWS Service Quota Limits

**Severity**: High

**Impact**: Unable to scale beyond quota limits

**Likelihood**: Medium (new AWS account)

**Mitigation**:
1. Request quota increases proactively:
   - ECS tasks per service: 100 (default 10)
   - Fargate vCPUs: 200 (default 50)
   - NLB listeners: 10 (default 5)
2. Document required quotas in deployment guide
3. Monitor quota usage via Service Quotas console

**Contingency**: Submit emergency quota increase request (AWS typically responds within 24 hours)

#### Risk 6: Deployment Rollback Failure

**Severity**: High

**Impact**: System in broken state, manual intervention required

**Likelihood**: Low (with thorough testing)

**Mitigation**:
1. Thoroughly test rollback scenarios in staging
2. Document manual rollback procedure
3. Implement health check grace period
4. Keep previous task definition versions for manual rollback

**Contingency**: Manual rollback via AWS console or CLI

#### Risk 7: Cost Overrun

**Severity**: Medium

**Impact**: Budget exceeded, potential service reduction

**Likelihood**: Medium (unpredictable scaling)

**Mitigation**:
1. Set conservative max_capacity limits
2. Implement cost alerts (AWS Budgets)
3. Monitor cost per email metric
4. Use ARM-based instances (t4g, r7g) for 20% cost savings

**Contingency**: Reduce min_capacity; optimize instance types; implement stricter scaling policies

### Business Risks

#### Risk 8: Let's Encrypt Rate Limit Exhaustion

**Severity**: Medium

**Impact**: Unable to acquire certificates for new domains

**Likelihood**: Low (50 certs/week limit sufficient for expected usage)

**Mitigation**:
1. Use staging environment for testing (unlimited rate limit)
2. Monitor certificate issuance count
3. Plan certificate requests (batch if possible)
4. Document rate limits in operations guide

**Contingency**: Purchase commercial certificates or wait for rate limit reset

#### Risk 9: Performance Below SLA

**Severity**: High

**Impact**: Customer dissatisfaction, SLA penalties

**Likelihood**: Low (with thorough load testing)

**Mitigation**:
1. Comprehensive load testing before production launch
2. Establish performance baselines
3. Continuous monitoring with CloudWatch dashboards
4. Capacity planning with 2× headroom

**Contingency**: Scale out manually; optimize bottlenecks; add caching layer

## Testing Strategy

### Unit Testing

**Components to Test**:
1. Certificate manager (renewal logic, storage, reload)
2. SMTP client (TLS configuration, authentication, batch sending)
3. Auto-scaling metric publishing
4. Connection draining logic

**Framework**: Go standard `testing` package

**Coverage Target**: 85% code coverage

### Integration Testing

**Scenarios**:
1. Docker Compose full stack startup
2. Certificate hot-reload without connection drops
3. Queue worker processing pipeline
4. Database connection pool behavior
5. ECS service communication via Cloud Map

**Environment**: Dedicated test AWS account

### Load Testing

**Tools**:
- Test SMTP client (batch mode)
- k6 or wrk for HTTP load testing
- Custom Go script for queue saturation

**Scenarios**:
1. Gradual ramp: 0 to 10K emails/min over 30 minutes
2. Sudden spike: 0 to 10K emails/min in 1 minute
3. Sustained load: 10K emails/min for 2 hours
4. Backpressure activation: Queue depth > 50,000

**Metrics to Capture**:
- Email delivery latency (P50, P95, P99)
- Connection establishment time
- Queue processing lag
- Auto-scaling response time
- Database connection pool utilization
- Error rate

### Chaos Testing (Optional, Post-Launch)

**Scenarios**:
1. AZ failure simulation
2. RDS failover during peak load
3. Redis cluster failover
4. ECS task sudden termination
5. Network partition between services

**Framework**: AWS Fault Injection Simulator (FIS)

## Rollout Plan

### Pre-Production Checklist

- [ ] All Terraform modules tested in dev account
- [ ] Docker Compose works on all developer machines
- [ ] Test SMTP client validated against local and staging environments
- [ ] Let's Encrypt staging certificates acquired successfully
- [ ] Load testing completed with satisfactory results
- [ ] Auto-scaling thresholds tuned
- [ ] Blue/green deployment tested
- [ ] Rollback procedure validated
- [ ] Documentation complete and reviewed
- [ ] Operations team trained
- [ ] Cost monitoring dashboards created
- [ ] AWS service quotas increased

### Production Launch

**Week 1**: Soft launch
- Deploy to production with traffic from internal testing only
- Monitor all metrics closely
- Validate auto-scaling behavior
- Test certificate renewal (mock expiry)
- Perform controlled load tests

**Week 2-3**: Beta customers
- Invite 5-10 beta customers
- Monitor performance and errors
- Collect feedback
- Fix critical issues
- Optimize based on real traffic patterns

**Week 4**: General availability
- Open to all customers
- Announce via marketing channels
- Monitor scale and performance
- Establish on-call rotation
- Document lessons learned

## Success Metrics

### Technical Metrics

- **Deployment Frequency**: Weekly deployments without downtime
- **MTTR (Mean Time To Recovery)**: < 15 minutes via automatic rollback
- **Infrastructure Provisioning Time**: < 20 minutes for full stack
- **Auto-Scaling Response Time**: < 30 seconds from trigger to new task
- **Certificate Renewal Success Rate**: > 99.9%

### Performance Metrics

- **Email Throughput**: 10,000 emails/min sustained
- **SMTP Connection Latency**: P95 < 100ms
- **Email Delivery Latency**: P95 < 5 seconds
- **API Response Time**: P95 < 200ms, P99 < 500ms
- **Uptime**: > 99.9%

### Cost Metrics

- **Monthly Infrastructure Cost**: < $1,200 at full scale
- **Cost Per Email**: < $0.0001 (1/100th of a cent)
- **Cost Efficiency**: < 20% waste from over-provisioning

### Operational Metrics

- **On-Call Alert Volume**: < 5 alerts per week
- **Manual Intervention Frequency**: < 1 per month
- **Documentation Coverage**: 100% of operational procedures

## Maintenance Plan

### Ongoing Maintenance Tasks

**Weekly**:
- Review CloudWatch dashboards for anomalies
- Check certificate expiry dates
- Review AWS costs vs budget
- Check for pending Terraform updates

**Monthly**:
- Review and update auto-scaling thresholds based on traffic patterns
- Update Terraform modules to latest AWS provider version
- Patch Docker base images for security updates
- Review and prune old CloudWatch logs

**Quarterly**:
- Load test to validate capacity
- Review and update capacity planning formulas
- Review AWS service quotas and request increases if needed
- Update disaster recovery runbooks

**Annually**:
- Major version upgrades (Go, PostgreSQL, Redis)
- Review and optimize AWS costs
- Architecture review for potential improvements

### Monitoring and Alerting

**Critical Alarms**:
1. ECS service unhealthy (> 50% tasks failing health checks)
2. Certificate renewal failure
3. Queue depth > 50,000 (backpressure active)
4. RDS storage < 10% free
5. Error rate > 5% for any service

**Warning Alarms**:
1. Auto-scaling near max capacity (> 80%)
2. Database connection pool > 80% utilized
3. SSL/TLS certificate expiry < 7 days
4. API P95 latency > 300ms
5. Cost > 110% of budget

## Next Steps

After completing this SPEC implementation:

1. **SPEC-CI-CD-001**: Implement CI/CD pipeline for automated testing and deployment
2. **SPEC-MONITORING-001**: Enhance observability with distributed tracing and advanced metrics
3. **SPEC-DR-001**: Implement disaster recovery and backup procedures
4. **SPEC-MULTI-REGION-001**: Multi-region deployment for global availability
5. **SPEC-COST-OPT-001**: Cost optimization strategies (reserved instances, spot instances, right-sizing)

## Conclusion

This implementation plan provides a structured approach to delivering production-ready infrastructure for smtp-proxy. The 7-phase plan spans 10 weeks and covers all aspects from local development to production deployment and auto-scaling.

Key success factors:
- **Incremental delivery**: Each phase produces working artifacts
- **Testing-first approach**: Load testing validates capacity before production
- **Automation**: Minimize manual operations through Terraform and auto-scaling
- **Safety**: Blue/green deployment and automatic rollback protect production
- **Observability**: CloudWatch monitoring provides visibility into system behavior

The plan balances speed of delivery with operational safety, enabling the team to deliver high-quality infrastructure while maintaining flexibility to adapt based on real-world usage patterns.
