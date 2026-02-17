.PHONY: build test lint clean migrate-up migrate-down sqlc docker-up docker-down

# Build
build:
	go build -o bin/smtp-server ./cmd/smtp-server
	go build -o bin/api-server ./cmd/api-server
	go build -o bin/queue-worker ./cmd/queue-worker

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

# Docker
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Clean
clean:
	rm -rf bin/ coverage.out coverage.html
