# =============================================================================
# Multi-stage Dockerfile for smtp-proxy services
# Build context: project root (.)
# Builds frontend assets first, then embeds them into the Go binary.
# =============================================================================

# ---------------------------------------------------------------------------
# Stage 0: Frontend - builds React SPA with Vite
# ---------------------------------------------------------------------------
FROM node:22-alpine AS frontend

WORKDIR /web

# Cache dependency installation
COPY web/package.json web/package-lock.json ./
RUN npm ci --legacy-peer-deps

# Build production bundle
COPY web/ .
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 1: Builder - compiles all Go binaries
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Cache dependency downloads separately from source code.
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy source and ensure dependencies are resolved.
COPY server/ .

# Copy frontend build output into the embed directory
COPY --from=frontend /web/dist ./internal/web/dist/

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/smtp-server ./cmd/smtp-server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api-server ./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/queue-worker ./cmd/queue-worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/test-client ./cmd/test-client

# ---------------------------------------------------------------------------
# Stage 2a: SMTP Server runtime
# ---------------------------------------------------------------------------
FROM alpine:3.21 AS smtp-server

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/smtp-server /usr/local/bin/
COPY server/config/ /app/config/

RUN mkdir -p /data/messages && chown nobody:nobody /data/messages

WORKDIR /app

EXPOSE 587 465

USER nobody:nobody

ENTRYPOINT ["smtp-server"]

# ---------------------------------------------------------------------------
# Stage 2b: API Server runtime (serves both API and embedded SPA)
# ---------------------------------------------------------------------------
FROM alpine:3.21 AS api-server

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /bin/api-server /usr/local/bin/
COPY server/config/ /app/config/
COPY server/migrations/ /app/migrations/

WORKDIR /app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

USER nobody:nobody

ENTRYPOINT ["api-server"]

# ---------------------------------------------------------------------------
# Stage 2c: Queue Worker runtime
# ---------------------------------------------------------------------------
FROM alpine:3.21 AS queue-worker

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/queue-worker /usr/local/bin/
COPY server/config/ /app/config/

RUN mkdir -p /data/messages && chown nobody:nobody /data/messages

WORKDIR /app

USER nobody:nobody

ENTRYPOINT ["queue-worker"]

# ---------------------------------------------------------------------------
# Stage 2d: Test Client (development utility)
# ---------------------------------------------------------------------------
FROM alpine:3.21 AS test-client

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/test-client /usr/local/bin/

WORKDIR /app

USER nobody:nobody

ENTRYPOINT ["test-client"]
