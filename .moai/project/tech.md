# SMTP Proxy - Technology Stack

## Technology Overview

This document details the complete technology stack for the smtp-proxy project, including backend, frontend, infrastructure, and development tools.

---

## Backend Stack (Go)

### Language and Runtime

**Go Version**: 1.21+ (latest stable)

**Rationale**:
- Native concurrency with goroutines for high-throughput SMTP handling
- Excellent standard library with net/smtp and net/mail packages
- Fast compilation and single-binary deployment
- Memory efficiency and low overhead for long-running services
- Strong static typing with excellent tooling

**Recommended Upgrade Path**: Track Go releases and upgrade within 3 months of stable release for security and performance improvements.

### Core Libraries

**HTTP and API Framework**

**chi (go-chi/chi)** - Version 5.0+

Rationale: Lightweight, idiomatic HTTP router with excellent middleware support. No external dependencies beyond standard library. Superior performance over heavier frameworks.

Key features: Context-based routing, middleware composition, route grouping, URL parameter parsing.

Alternative considered: gorilla/mux (more features but heavier), echo (faster but non-idiomatic).

**SMTP Protocol**

**net/smtp** - Go standard library

Rationale: Production-ready SMTP client in standard library. Used for outbound connections to ESP providers.

**go-smtp** - github.com/emersion/go-smtp

Rationale: Full-featured SMTP server implementation supporting SMTP AUTH, STARTTLS, and custom backends. Active maintenance and RFC compliance.

Key features: AUTH mechanisms, TLS support, custom storage backends, connection hooks.

**Database Access**

**pgx** - github.com/jackc/pgx/v5

Rationale: Native PostgreSQL driver with superior performance over database/sql. Connection pooling built-in. Supports PostgreSQL-specific features.

Key features: Prepared statements, batch operations, LISTEN/NOTIFY, efficient binary protocol.

Migration tool: golang-migrate/migrate for SQL migrations with up/down support.

**Message Queue**

**Redis** (Primary recommendation) - github.com/redis/go-redis/v9

Rationale: Simple, fast, proven queue solution. Stream support for reliable delivery. Low operational complexity.

Key features: Redis Streams for queues, consumer groups, persistence, pub/sub for real-time updates.

**NATS** (Alternative for higher throughput) - github.com/nats-io/nats.go

Rationale: Cloud-native messaging with JetStream for persistence. Excellent for distributed systems.

Key features: JetStream persistence, exactly-once delivery, horizontal scaling.

Decision guide: Use Redis for simplicity (up to 10K messages/minute). Use NATS for scale (beyond 10K messages/minute or distributed deployment).

**Configuration Management**

**viper** - github.com/spf13/viper

Rationale: Complete configuration solution supporting multiple sources. Environment variables, config files, remote config.

Key features: Automatic environment variable binding, live config reload, multiple format support (YAML, JSON, TOML).

**envconfig** - github.com/kelseyhightower/envconfig (Lightweight alternative)

Rationale: Simple environment variable parsing. Minimal dependencies. Struct-tag based configuration.

**Validation**

**validator** - github.com/go-playground/validator/v10

Rationale: Comprehensive struct validation with tag-based rules. Custom validation functions. Industry standard.

Key features: Struct validation, cross-field validation, custom error messages, internationalization.

**Logging**

**zerolog** - github.com/rs/zerolog

Rationale: Zero-allocation JSON logger. Excellent performance. Structured logging with type safety.

Key features: Zero allocation, chained API, log levels, context-aware logging, pretty printing for development.

Alternative considered: zap (faster but more complex), logrus (slower, older).

**Metrics and Monitoring**

**prometheus/client_golang** - github.com/prometheus/client_golang

Rationale: Industry-standard metrics collection. Native Prometheus exposition format. Rich metric types.

Key features: Counter, Gauge, Histogram, Summary metrics. HTTP handler for scraping.

**OpenTelemetry** (For distributed tracing) - go.opentelemetry.io/otel

Rationale: Vendor-neutral observability framework. Distributed tracing across services.

Key features: Trace context propagation, span creation, exporter support (Jaeger, Zipkin).

**Testing**

**testify** - github.com/stretchr/testify

Rationale: Rich assertion library and mocking framework. Reduces boilerplate in tests.

Key features: Assert and require packages, mock generation, suite support.

**gomock** (For advanced mocking) - github.com/golang/mock

Rationale: Official Go mocking framework. Interface-based mocking. Code generation from interfaces.

**Security**

**bcrypt** - golang.org/x/crypto/bcrypt

Rationale: Secure password hashing. Adaptive cost factor. Standard library quality.

**jwt-go** - github.com/golang-jwt/jwt/v5

Rationale: JWT token generation and validation for API authentication.

Key features: Multiple signing algorithms, claims validation, token refresh.

**ESP Provider SDKs**

**SendGrid**: github.com/sendgrid/sendgrid-go

**AWS SDK Go v2**: github.com/aws/aws-sdk-go-v2/service/ses

**Mailgun**: github.com/mailgun/mailgun-go/v4

Rationale: Official provider SDKs offer best compatibility and feature support.

---

## Frontend Stack (Next.js)

### Framework and Runtime

**Next.js Version**: 14+ with App Router

Rationale: React framework with server-side rendering, static generation, API routes. App Router provides improved routing, layouts, and server components.

Key features: File-based routing, server components, streaming, built-in optimization.

**Node.js Version**: 20 LTS

Rationale: Latest LTS with long-term support. Performance improvements over previous versions.

**React Version**: 18+

Rationale: Concurrent rendering, automatic batching, transitions API. Required for Next.js 14.

### Core Libraries

**UI Component Library**

**shadcn/ui** - shadcn-ui.com

Rationale: Copy-paste component library built on Radix UI primitives. Full customization without library lock-in. Accessible by default.

Key components: Button, Input, Card, Dialog, Table, Select, Dropdown Menu, Toast.

Styling: Tailwind CSS for utility-first styling. Full design system control.

**Radix UI** (Underlying primitives) - @radix-ui/react-*

Rationale: Unstyled, accessible components. ARIA-compliant. Keyboard navigation built-in.

**State Management**

**TanStack Query (React Query)** - @tanstack/react-query v5

Rationale: Server state management with caching, background updates, optimistic updates. Eliminates Redux boilerplate for API data.

Key features: Automatic caching, refetch on focus, invalidation, infinite queries, mutations.

**Zustand** (For client state) - zustand

Rationale: Minimal, unopinionated state management. No providers required. Simple API.

Use case: UI state, user preferences, temporary form state.

**Form Handling**

**React Hook Form** - react-hook-form v7

Rationale: Performant forms with minimal re-renders. Built-in validation. TypeScript support.

Key features: Uncontrolled components, validation schemas, error handling, watch fields.

**Zod** (Schema validation) - zod

Rationale: TypeScript-first schema validation. Type inference. Runtime type checking.

Use case: Form validation schemas, API response validation.

**HTTP Client**

**axios** - axios

Rationale: Full-featured HTTP client with interceptors. Request/response transformation. Automatic JSON handling.

Key features: Interceptors for auth tokens, timeout configuration, retry logic, request cancellation.

Alternative: fetch API with custom wrapper (lighter, native).

**Data Visualization**

**Recharts** - recharts

Rationale: Composable chart library built on React components. Responsive. Wide variety of chart types.

Key charts: Line, Bar, Area, Pie, Scatter charts for metrics visualization.

**Chart.js + react-chartjs-2** (Alternative for advanced features)

Rationale: More chart types, better performance for large datasets. Plugin ecosystem.

**Date and Time**

**date-fns** - date-fns

Rationale: Modular date utility library. Tree-shakeable. Immutable. Simpler than moment.js.

Key features: Date formatting, parsing, manipulation, timezone support via date-fns-tz.

**UI Utilities**

**clsx** - clsx

Rationale: Tiny utility for constructing className strings conditionally. Tailwind CSS companion.

**tailwind-merge** - tailwind-merge

Rationale: Merge Tailwind CSS classes without style conflicts. Combine with clsx for cn utility.

**lucide-react** (Icons) - lucide-react

Rationale: Beautiful, consistent icon library. Tree-shakeable. React components.

**Authentication**

**NextAuth.js** - next-auth

Rationale: Authentication for Next.js with multiple providers. Session management. JWT and database sessions.

Key features: OAuth providers, credentials provider, session handling, CSRF protection.

**Testing**

**Jest** - jest

Rationale: Testing framework with zero config for Next.js. Snapshot testing. Mocking support.

**React Testing Library** - @testing-library/react

Rationale: User-centric testing. Encourages accessibility. Queries by role, label, text.

**Playwright** (E2E testing) - @playwright/test

Rationale: Modern E2E testing with multi-browser support. Fast, reliable. Visual comparisons.

---

## Database

### Primary Database

**PostgreSQL Version**: 15+

Rationale: Robust relational database with excellent JSON support. ACID compliance. Rich indexing. Proven at scale.

Key features:
- JSONB for flexible metadata storage
- Full-text search for message content
- Triggers for audit logging
- Row-level security for multi-tenancy
- Partitioning for message history tables

**Extensions**:
- pg_stat_statements: Query performance monitoring
- pgcrypto: Encryption functions for sensitive data
- uuid-ossp: UUID generation

### Migration Tool

**golang-migrate** - github.com/golang-migrate/migrate

Rationale: Database-agnostic migration tool. Up/down migrations. CLI and library support.

Migration files: SQL format with numeric versioning (001_initial_schema.up.sql, 001_initial_schema.down.sql).

---

## Message Queue

### Primary Choice: Redis

**Redis Version**: 7.0+

**Deployment**: Redis with persistence (AOF + RDS) or managed service (AWS ElastiCache, Redis Cloud)

Rationale: Simple operational model. Excellent performance. Redis Streams provide queue with consumer groups.

**Redis Streams features**:
- Message persistence with configurable retention
- Consumer groups for load distribution
- Pending message tracking
- Message acknowledgment
- Dead letter queue implementation

**go-redis client**: github.com/redis/go-redis/v9 with connection pooling and cluster support.

### Alternative: NATS with JetStream

**NATS Version**: 2.9+

Rationale: Better for distributed deployments. Higher throughput. Built-in clustering.

**JetStream features**:
- Exactly-once delivery semantics
- Message replay
- Horizontal scaling
- Stream replication
- Key-value store

Use case: Choose NATS when scaling beyond single Redis instance or requiring multi-region deployment.

### Alternative: RabbitMQ

**RabbitMQ Version**: 3.12+

Rationale: Enterprise-grade message broker. Rich routing features. Wide adoption.

Use case: Legacy integration requirements or complex routing patterns.

Decision deferred: Evaluate based on throughput requirements during implementation phase.

---

## Infrastructure and DevOps

### Containerization

**Docker Version**: 24+

Rationale: Standard containerization platform. Wide tooling support. Multi-stage builds for optimization.

**Docker Compose**: Local development environment orchestration.

**Container Registry**:
- GitHub Container Registry (ghcr.io) for open source
- AWS ECR or GCP Artifact Registry for cloud deployment

### Orchestration

**Kubernetes**: 1.28+ for production

Rationale: Industry-standard orchestration. Horizontal scaling. Self-healing. Rich ecosystem.

**Key resources**:
- Deployments for stateless services
- StatefulSets for queue workers requiring ordered shutdown
- ConfigMaps and Secrets for configuration
- Horizontal Pod Autoscaler for dynamic scaling
- Ingress for SMTP and HTTP traffic routing

**Managed Kubernetes**:
- AWS EKS
- Google GKE
- Azure AKS

**Alternative for small deployments**: Docker Swarm or AWS ECS for lower operational complexity.

### Infrastructure as Code

**Terraform**: 1.6+

Rationale: Multi-cloud IaC with state management. Rich provider ecosystem. Strong community.

**Modules**:
- VPC and networking
- RDS PostgreSQL with replicas
- ElastiCache Redis cluster
- EKS cluster with node groups
- Application Load Balancer
- CloudWatch alarms and dashboards

**Alternative**: Pulumi for programming language-based IaC (TypeScript, Go support).

### CI/CD

**GitHub Actions**

Rationale: Native GitHub integration. Free for public repos. Reasonable pricing. Matrix builds. Self-hosted runners support.

**Workflows**:
- Backend CI: Go linting, testing, coverage, Docker build
- Frontend CI: TypeScript checking, ESLint, Prettier, Jest tests
- Security: Dependency scanning with Dependabot, SAST with CodeQL
- Deployment: Automated deploy to staging, manual approval for production

**Alternative**: GitLab CI/CD for self-hosted requirements.

### Monitoring and Observability

**Prometheus**: Metrics collection and alerting

**Grafana**: Metrics visualization and dashboards

**Loki** (Logging): Log aggregation with Grafana integration

**Tempo** (Tracing): Distributed tracing backend

**Sentry** (Error tracking): Frontend and backend error monitoring

**Managed alternatives**:
- Datadog for unified monitoring platform
- New Relic for APM
- AWS CloudWatch for AWS-native monitoring

### Load Balancing

**NGINX**: Reverse proxy and load balancer for SMTP and HTTP traffic

**AWS Application Load Balancer**: Managed L7 load balancing with health checks

**HAProxy**: Alternative for advanced routing and TCP load balancing

---

## Development Tools

### Backend Development

**Go Linting and Formatting**

**golangci-lint**: Meta-linter running 50+ linters. Configuration-driven. Fast.

Enabled linters: gofmt, goimports, govet, errcheck, staticcheck, gosec, revive.

**gofumpt**: Stricter formatting than gofmt for consistency.

**Code Quality**

**SonarQube** (Optional): Code quality and security analysis. Technical debt tracking.

**GoReleaser**: Automated binary releases with cross-compilation. Changelog generation.

### Frontend Development

**TypeScript**: 5.0+

Rationale: Static typing for JavaScript. Catches errors at compile time. Excellent IDE support.

**ESLint**: JavaScript/TypeScript linting

Config: Next.js recommended config with TypeScript rules. Import ordering rules. Accessibility checks.

**Prettier**: Code formatting

Rationale: Opinionated formatting eliminates style discussions. Automatic formatting on save.

**Tailwind CSS**: 3.4+

Rationale: Utility-first CSS framework. No custom CSS files. Responsive design utilities. JIT compilation.

### Version Control

**Git**: 2.40+

**Branching Strategy**: GitHub Flow (main + feature branches) or GitFlow for larger teams

**Commit Convention**: Conventional Commits for semantic versioning and changelog generation

Format: type(scope): description

Types: feat, fix, docs, style, refactor, test, chore

### Package Management

**Backend**: Go modules (go.mod, go.sum) with vendoring for reproducible builds

**Frontend**: npm or pnpm

Recommendation: pnpm for faster installs and disk space efficiency. Strict dependency resolution.

### Documentation

**API Documentation**: OpenAPI 3.0 specification with Swagger UI

Tool: swag for generating OpenAPI from Go comments.

**Architecture Diagrams**: Mermaid for version-controlled diagrams in Markdown

**Documentation Site**: Nextra (Next.js-based) for developer docs

### Local Development

**Hot Reload**:
- Backend: air (cosmtrek/air) for Go hot reload
- Frontend: Built-in Next.js fast refresh

**Database Admin**: pgAdmin or DBeaver for PostgreSQL management

**Redis Admin**: RedisInsight for Redis debugging and monitoring

**API Testing**:
- Postman or Insomnia for manual API testing
- bruno for Git-friendly API collections

**SMTP Testing**: MailHog or MailCatcher for local SMTP server during development

---

## Security Tools

### Dependency Scanning

**Dependabot**: Automated dependency updates with security vulnerability alerts

**Snyk**: Continuous security monitoring for dependencies

**Trivy**: Container image vulnerability scanning

### Static Analysis

**gosec**: Go security scanner for common security issues

**CodeQL**: Semantic code analysis for security vulnerabilities

**npm audit**: Frontend dependency vulnerability scanning

### Secrets Management

**HashiCorp Vault**: Secret storage and rotation for production

**AWS Secrets Manager**: AWS-native secrets management

**SOPS** (Secrets OPerationS): Encrypted secrets in Git with age or PGP

Development: .env files with .env.example template (never commit .env)

---

## Performance Tools

### Profiling

**pprof**: Go profiling for CPU, memory, goroutines. Built-in HTTP endpoints.

**go-torch**: Flame graph generation from pprof data

### Load Testing

**k6**: Modern load testing tool with JavaScript scripting. Grafana integration.

**vegeta**: HTTP load testing with constant request rate.

Use case: k6 for scenario-based testing, vegeta for raw throughput measurement.

### Benchmarking

**Go benchmarks**: Built-in benchmarking with go test -bench

**autocannon**: HTTP benchmarking for frontend API

---

## Technology Decision Summary

### Core Technology Choices

| Component | Technology | Version | Rationale |
|-----------|-----------|---------|-----------|
| Backend Language | Go | 1.21+ | Concurrency, performance, single binary |
| Frontend Framework | Next.js | 14+ | SSR, App Router, React ecosystem |
| Database | PostgreSQL | 15+ | ACID, JSONB, scalability |
| Message Queue | Redis Streams | 7.0+ | Simplicity, performance, persistence |
| Container Runtime | Docker | 24+ | Standard, wide support |
| Orchestration | Kubernetes | 1.28+ | Scaling, self-healing, ecosystem |
| Monitoring | Prometheus + Grafana | Latest | Open source, rich ecosystem |
| CI/CD | GitHub Actions | N/A | Native integration, cost-effective |

### Build vs Buy Decisions

**Build**: SMTP server, routing engine, queue workers, admin UI

Rationale: Core differentiators requiring custom logic. No off-the-shelf solutions meet multi-tenant requirements.

**Buy/Use Open Source**: Database, message queue, monitoring, load balancing

Rationale: Commodity infrastructure with excellent open-source solutions. Focus engineering effort on core value.

**Managed Services (Production)**:
- RDS for PostgreSQL (reduced operational burden)
- ElastiCache for Redis (high availability built-in)
- EKS for Kubernetes (managed control plane)

Rationale: Reduce operational complexity. Focus on application development. Managed services provide SLA guarantees.

---

## Migration and Upgrade Strategy

### Database Migrations

- All schema changes via golang-migrate SQL files
- Test migrations on staging before production
- Rollback plan for every migration
- Zero-downtime migrations using dual-write pattern for breaking changes

### Dependency Updates

- Patch versions: Automated via Dependabot with CI validation
- Minor versions: Quarterly review and update cycle
- Major versions: Dedicated testing period, changelog review, migration guide

### Go Version Updates

- Upgrade within 3 months of new stable release
- Test suite validation on new version
- Check compatibility of all dependencies
- Update CI/CD pipeline and Dockerfiles

### Node.js and Next.js Updates

- Node.js LTS: Upgrade within 2 months of new LTS release
- Next.js: Follow stable releases, evaluate App Router improvements
- Test build and runtime on new versions before production deployment

---

## Technology Evaluation Criteria

Future technology additions will be evaluated based on:

1. **Maturity**: Production-ready with stable API
2. **Community**: Active development and support
3. **Performance**: Meets throughput and latency requirements
4. **Maintenance**: Long-term viability and update cadence
5. **Integration**: Compatibility with existing stack
6. **Cost**: Licensing, operational, and development costs
7. **Team Expertise**: Learning curve and available skills

---

**Document Version**: 1.0.0

**Last Updated**: 2026-02-15

**Technology Review Cycle**: Quarterly

**Maintained By**: smtp-proxy Development Team
