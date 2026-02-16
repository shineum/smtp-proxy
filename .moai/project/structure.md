# SMTP Proxy - Project Structure

## Project Organization

This document describes the recommended monorepo structure for the smtp-proxy project, combining Go backend services with a Next.js admin interface.

---

## Directory Tree

```
smtp-proxy/
├── .github/
│   └── workflows/
│       ├── backend-ci.yml
│       ├── frontend-ci.yml
│       └── deploy.yml
├── .moai/
│   ├── config/
│   │   └── sections/
│   │       ├── language.yaml
│   │       ├── quality.yaml
│   │       ├── user.yaml
│   │       └── workflow.yaml
│   ├── docs/
│   ├── project/
│   │   ├── product.md
│   │   ├── structure.md
│   │   └── tech.md
│   └── specs/
├── backend/
│   ├── cmd/
│   │   ├── smtp-server/
│   │   │   └── main.go
│   │   ├── queue-worker/
│   │   │   └── main.go
│   │   └── api-server/
│   │       └── main.go
│   ├── internal/
│   │   ├── smtp/
│   │   │   ├── server.go
│   │   │   ├── handler.go
│   │   │   ├── auth.go
│   │   │   └── session.go
│   │   ├── queue/
│   │   │   ├── queue.go
│   │   │   ├── worker.go
│   │   │   └── retry.go
│   │   ├── provider/
│   │   │   ├── provider.go
│   │   │   ├── sendgrid.go
│   │   │   ├── ses.go
│   │   │   ├── mailgun.go
│   │   │   └── generic.go
│   │   ├── router/
│   │   │   ├── router.go
│   │   │   └── rules.go
│   │   ├── api/
│   │   │   ├── handlers/
│   │   │   ├── middleware/
│   │   │   └── routes.go
│   │   ├── models/
│   │   │   ├── tenant.go
│   │   │   ├── message.go
│   │   │   └── provider.go
│   │   ├── storage/
│   │   │   ├── postgres.go
│   │   │   └── migrations/
│   │   └── monitoring/
│   │       ├── metrics.go
│   │       └── logger.go
│   ├── pkg/
│   │   ├── config/
│   │   │   └── config.go
│   │   └── errors/
│   │       └── errors.go
│   ├── test/
│   │   ├── integration/
│   │   └── e2e/
│   ├── scripts/
│   │   ├── setup-dev.sh
│   │   └── migrate.sh
│   ├── go.mod
│   ├── go.sum
│   └── Makefile
├── frontend/
│   ├── app/
│   │   ├── (auth)/
│   │   │   ├── login/
│   │   │   └── layout.tsx
│   │   ├── (dashboard)/
│   │   │   ├── dashboard/
│   │   │   ├── tenants/
│   │   │   ├── providers/
│   │   │   ├── messages/
│   │   │   ├── analytics/
│   │   │   └── layout.tsx
│   │   ├── api/
│   │   ├── layout.tsx
│   │   └── page.tsx
│   ├── components/
│   │   ├── ui/
│   │   ├── dashboard/
│   │   ├── forms/
│   │   └── charts/
│   ├── lib/
│   │   ├── api-client.ts
│   │   ├── auth.ts
│   │   └── utils.ts
│   ├── hooks/
│   │   ├── use-tenants.ts
│   │   └── use-metrics.ts
│   ├── types/
│   │   └── api.ts
│   ├── public/
│   ├── styles/
│   ├── package.json
│   ├── tsconfig.json
│   ├── next.config.js
│   └── tailwind.config.js
├── infra/
│   ├── docker/
│   │   ├── Dockerfile.smtp-server
│   │   ├── Dockerfile.queue-worker
│   │   ├── Dockerfile.api-server
│   │   └── Dockerfile.frontend
│   ├── k8s/
│   │   ├── smtp-server/
│   │   ├── queue-worker/
│   │   ├── api-server/
│   │   └── frontend/
│   ├── terraform/
│   │   ├── aws/
│   │   └── gcp/
│   └── docker-compose.yml
├── docs/
│   ├── api/
│   │   └── openapi.yaml
│   ├── architecture/
│   │   ├── system-design.md
│   │   └── data-flow.md
│   └── deployment/
│       └── production.md
├── .gitignore
├── README.md
├── CHANGELOG.md
├── LICENSE
└── Makefile
```

---

## Backend Structure (Go)

### cmd/ - Application Entry Points

**Purpose**: Main application executables with minimal logic, primarily configuration and startup

**cmd/smtp-server/main.go**
- Initializes SMTP server on port 25/587/465
- Loads configuration from environment and config files
- Sets up signal handling for graceful shutdown
- Initializes logging and monitoring
- Connects to message queue backend

**cmd/queue-worker/main.go**
- Initializes queue consumer workers
- Connects to queue backend (Redis/NATS)
- Initializes ESP provider clients
- Sets up worker pool with configurable concurrency
- Handles message processing and delivery

**cmd/api-server/main.go**
- Initializes HTTP API server for admin interface
- Sets up REST API routes and middleware
- Connects to database for CRUD operations
- Provides metrics endpoint for monitoring
- Handles authentication and authorization

### internal/ - Private Application Code

**Purpose**: Core business logic not intended for external import

**internal/smtp/**
- server.go: SMTP protocol server implementation with RFC compliance
- handler.go: SMTP command handlers (HELO, MAIL FROM, RCPT TO, DATA)
- auth.go: SMTP AUTH mechanisms (PLAIN, LOGIN) with credential validation
- session.go: SMTP session state management and connection pooling

**internal/queue/**
- queue.go: Queue interface abstraction (Redis/NATS/RabbitMQ)
- worker.go: Worker pool implementation with job processing
- retry.go: Retry policy engine with exponential backoff

**internal/provider/**
- provider.go: ESP provider interface definition
- sendgrid.go: SendGrid API v3 client implementation
- ses.go: AWS SES SMTP and API client
- mailgun.go: Mailgun HTTP API client
- generic.go: Generic SMTP relay implementation

**internal/router/**
- router.go: Routing engine for provider selection
- rules.go: Routing rule evaluation (domain, tenant, time-based)

**internal/api/**
- handlers/: HTTP request handlers for REST API endpoints
- middleware/: Authentication, logging, CORS, rate limiting
- routes.go: API route registration and versioning

**internal/models/**
- tenant.go: Tenant data model with CRUD operations
- message.go: Message data model with status tracking
- provider.go: Provider configuration model

**internal/storage/**
- postgres.go: PostgreSQL client with connection pooling
- migrations/: SQL migration files for schema evolution

**internal/monitoring/**
- metrics.go: Prometheus metrics collection and export
- logger.go: Structured logging with correlation IDs

### pkg/ - Public Libraries

**Purpose**: Reusable packages that can be imported by external projects

**pkg/config/**
- config.go: Configuration loading from environment, files, and defaults
- Validation of configuration values
- Configuration hot-reload support

**pkg/errors/**
- errors.go: Custom error types with context
- Error wrapping and unwrapping
- Error code definitions for API responses

### test/ - Testing Infrastructure

**test/integration/**
- Integration tests with real dependencies (database, queue)
- Docker Compose setup for test environment
- Test fixtures and helpers

**test/e2e/**
- End-to-end tests simulating real SMTP clients
- Full workflow tests (accept → queue → deliver)
- Performance and load testing

---

## Frontend Structure (Next.js)

### app/ - Next.js App Router

**Purpose**: Next.js 13+ App Router with file-based routing

**app/(auth)/**
- Route group for authentication pages
- login/: Login page with form validation
- layout.tsx: Auth layout without navigation

**app/(dashboard)/**
- Route group for authenticated dashboard
- dashboard/: Main dashboard with metrics overview
- tenants/: Tenant management CRUD interface
- providers/: ESP provider configuration interface
- messages/: Message history and status tracking
- analytics/: Charts and analytics visualization
- layout.tsx: Dashboard layout with navigation sidebar

**app/api/**
- API route handlers for server-side logic
- Proxy to backend API with authentication
- Server-side data fetching

### components/ - React Components

**components/ui/**
- Reusable UI components from shadcn/ui
- Button, Input, Card, Dialog, Table components
- Design system primitives

**components/dashboard/**
- Dashboard-specific components
- MetricsCard, StatusBadge, TenantTable
- Chart components wrapping recharts

**components/forms/**
- Form components with validation
- TenantForm, ProviderForm
- React Hook Form integration

**components/charts/**
- Data visualization components
- DeliveryRateChart, ErrorRateChart
- Real-time metric updates

### lib/ - Utility Libraries

**lib/api-client.ts**
- HTTP client with axios or fetch
- Authentication token management
- Request/response interceptors
- Error handling

**lib/auth.ts**
- Authentication utilities
- Token storage and refresh
- Protected route helpers

**lib/utils.ts**
- General utility functions
- Date formatting, validation
- Tailwind CSS class helpers (cn)

### hooks/ - Custom React Hooks

**hooks/use-tenants.ts**
- React Query hook for tenant data
- CRUD operations with optimistic updates
- Cache invalidation

**hooks/use-metrics.ts**
- Real-time metrics fetching
- WebSocket or polling updates
- Data aggregation

### types/ - TypeScript Type Definitions

**types/api.ts**
- API request/response types
- Domain model types
- Enum definitions

---

## Infrastructure (infra/)

### docker/ - Docker Configuration

**Purpose**: Container images for all services

**Dockerfile.smtp-server**
- Multi-stage build for Go binary
- Minimal Alpine Linux base
- Health check endpoint
- Non-root user execution

**Dockerfile.queue-worker**
- Similar to smtp-server
- Worker-specific configuration
- Configurable concurrency

**Dockerfile.api-server**
- REST API server container
- Database migration on startup
- Metrics exposure

**Dockerfile.frontend**
- Node.js build stage
- Static file serving with nginx
- Production optimizations

### k8s/ - Kubernetes Manifests

**Purpose**: Production deployment configuration

Each service directory contains:
- deployment.yaml: Pod specification and replica configuration
- service.yaml: Service exposure (ClusterIP/LoadBalancer)
- configmap.yaml: Environment-specific configuration
- secret.yaml: Sensitive credentials (encrypted)
- hpa.yaml: Horizontal Pod Autoscaler rules

### terraform/ - Infrastructure as Code

**terraform/aws/**
- RDS PostgreSQL database
- ElastiCache Redis for queue
- Application Load Balancer
- ECS cluster configuration

**terraform/gcp/**
- Cloud SQL PostgreSQL
- Memorystore Redis
- Cloud Load Balancing
- GKE cluster configuration

### docker-compose.yml - Local Development

**Purpose**: Single-command local environment setup

Services defined:
- postgres: Database with persistent volume
- redis: Message queue backend
- smtp-server: SMTP service on port 2525
- queue-worker: Background worker
- api-server: REST API on port 8080
- frontend: Next.js dev server on port 3000

---

## Shared Directories

### docs/ - Documentation

**docs/api/**
- OpenAPI 3.0 specification
- API endpoint documentation
- Request/response examples

**docs/architecture/**
- System design documents
- Component diagrams
- Data flow diagrams

**docs/deployment/**
- Production deployment guides
- Configuration reference
- Troubleshooting runbooks

### .github/workflows/ - CI/CD Pipelines

**backend-ci.yml**
- Go linting with golangci-lint
- Unit and integration tests
- Test coverage reporting
- Docker image build and push

**frontend-ci.yml**
- TypeScript type checking
- ESLint and Prettier validation
- Unit tests with Jest
- Build verification

**deploy.yml**
- Automated deployment to staging/production
- Database migration execution
- Health check validation
- Rollback on failure

---

## Configuration Files

### Root Level

**.gitignore**
- Go binary exclusions
- Node.js dependencies
- Environment files
- Build artifacts

**README.md**
- Project overview
- Quick start guide
- Development setup
- Contributing guidelines

**CHANGELOG.md**
- Version history
- Feature additions
- Bug fixes
- Breaking changes

**Makefile**
- Common development tasks
- Build targets
- Test execution
- Deployment commands

---

## Directory Organization Principles

### Separation of Concerns

Backend and frontend are completely separated in dedicated directories, allowing independent development and deployment. Infrastructure configuration is isolated in the infra directory for clear operational boundaries.

### Scalability

The structure supports monorepo scaling with additional backend services easily added to cmd and internal directories. Frontend features are organized by route groups enabling parallel team development. Infrastructure as code supports multi-environment deployment without duplication.

### Testability

Test directories at multiple levels support unit tests alongside code in Go packages, integration tests in dedicated test directory, and end-to-end tests covering full workflows.

### Developer Experience

Clear naming conventions make navigation intuitive. Standard Go project layout follows community best practices. Next.js App Router structure aligns with official recommendations. Docker Compose enables one-command local environment setup.

---

**Document Version**: 1.0.0

**Last Updated**: 2026-02-15

**Maintained By**: smtp-proxy Development Team
