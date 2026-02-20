.PHONY: build test lint clean migrate-up migrate-down sqlc \
       dev-certs docker-build docker-up docker-down docker-logs test-email

# Build
build:
	go build -o bin/smtp-server ./cmd/smtp-server
	go build -o bin/api-server ./cmd/api-server
	go build -o bin/queue-worker ./cmd/queue-worker
	go build -o bin/test-client ./cmd/test-client

# Test
test:
	go test -race -cover ./...

test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Database
migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down

# sqlc
sqlc:
	sqlc generate

# TLS Certificates
dev-certs:
	../scripts/generate-dev-certs.sh

# Docker
docker-build:
	docker compose -f ../docker-compose.yml build

docker-up:
	docker compose -f ../docker-compose.yml up -d

docker-down:
	docker compose -f ../docker-compose.yml down

docker-logs:
	docker compose -f ../docker-compose.yml logs -f

# Test email via test-client (local Go run, requires dev account)
test-email:
	go run ./cmd/test-client \
		--host localhost \
		--port 587 \
		--tls starttls \
		--insecure \
		--user dev \
		--password dev \
		--from dev@example.com \
		--to recipient@example.com \
		--subject "Test Email" \
		--body "Hello from smtp-proxy test client."

# Clean
clean:
	rm -rf bin/ coverage.out coverage.html
