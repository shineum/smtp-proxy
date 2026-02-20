# =============================================================================
# smtp-proxy — Developer Workflow
#
# Quick start:
#   make dev          Start everything + seed dev account
#   make test-email   Send a test email with dev credentials
#   make stop         Shut down all services
# =============================================================================

.PHONY: dev stop logs test-email test lint certs

## Start all services and seed the dev account
dev: certs
	docker compose up -d --build

## Send a test email using the dev account
test-email:
	docker compose run --rm test-client

## Stop all services
stop:
	docker compose down

## Follow service logs
logs:
	docker compose logs -f

## Run Go tests
test:
	$(MAKE) -C server test

## Run linter
lint:
	$(MAKE) -C server lint

## Generate self-signed TLS certificates (idempotent)
certs:
	@if [ ! -f certs/server.crt ]; then \
		echo "Generating dev TLS certificates..."; \
		./scripts/generate-dev-certs.sh; \
	fi
