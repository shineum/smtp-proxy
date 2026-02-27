# =============================================================================
# Multi-stage Dockerfile for smtp-proxy services
# Builds all binaries in a single builder stage, then creates minimal runtime
# images per service using named targets.
# =============================================================================

# ---------------------------------------------------------------------------
# Stage 1: Builder - compiles all Go binaries
# ---------------------------------------------------------------------------
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Cache dependency downloads separately from source code.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and ensure dependencies are resolved.
COPY . .
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
COPY config/ /app/config/

RUN mkdir -p /data/messages && chown nobody:nobody /data/messages

WORKDIR /app

EXPOSE 587 465

USER nobody:nobody

ENTRYPOINT ["smtp-server"]

# ---------------------------------------------------------------------------
# Stage 2b: API Server runtime
# ---------------------------------------------------------------------------
FROM alpine:3.21 AS api-server

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /bin/api-server /usr/local/bin/
COPY config/ /app/config/

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
COPY config/ /app/config/

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
