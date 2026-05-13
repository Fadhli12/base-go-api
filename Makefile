# Go API Base Makefile
# Project: github.com/example/go-api-base

# Load environment variables from .env file (if it exists)
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Build DATABASE_URL from individual components if not already set
ifeq ($(DATABASE_URL),)
DATABASE_URL := postgres://$(DATABASE_USER):$(DATABASE_PASSWORD)@$(DATABASE_HOST):$(DATABASE_PORT)/$(DATABASE_NAME)?sslmode=$(DATABASE_SSL_MODE)
endif

.PHONY: migrate migrate-up migrate-down migrate-rollback migrate-status migrate-refresh migrate-fresh seed serve test test-all test-integration lint swagger permission-sync help

# ============================================================================
# Migration commands (Laravel-style)
# ============================================================================

# Run all pending migrations (Laravel: php artisan migrate)
migrate: migrate-up

# Run all pending migrations (explicit)
migrate-up:
	@echo "Running migrations..."
	go run ./cmd/api/main.go migrate

# Rollback the last batch of migrations (Laravel: php artisan migrate:rollback)
migrate-down:
	@echo "Rolling back last migration..."
	go run ./cmd/api/main.go migrate:rollback --step=1

# Rollback with step control (Laravel: php artisan migrate:rollback --step=N)
migrate-rollback:
	@echo "Rolling back migrations..."
	go run ./cmd/api/main.go migrate:rollback --step=$(or $(STEP),1)

# Show migration status (Laravel: php artisan migrate:status)
migrate-status:
	@echo "Checking migration status..."
	go run ./cmd/api/main.go migrate:status

# Rollback all + re-migrate (Laravel: php artisan migrate:refresh)
migrate-refresh:
	@echo "Refreshing migrations (rollback all + re-run)..."
	go run ./cmd/api/main.go migrate:refresh --force

# Drop all tables + re-migrate (Laravel: php artisan migrate:fresh)
migrate-fresh:
	@echo "Dropping all tables and re-running migrations..."
	go run ./cmd/api/main.go migrate:fresh --force

# ============================================================================
# Database commands
# ============================================================================

# Seed database with initial data
seed:
	@echo "Running database seed..."
	go run ./cmd/api/main.go seed

# ============================================================================
# Server commands
# ============================================================================

# Run API server
serve:
	@echo "Starting API server..."
	go run ./cmd/api/main.go serve

# ============================================================================
# Test commands
# ============================================================================

# Run unit tests (no Docker required)
test:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.txt ./tests/unit/...

# Run integration tests (requires Docker)
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./tests/integration/... -timeout 5m

# Run all tests (unit only - integration requires Docker separately)
test-all: test
	@echo ""
	@echo "Note: Integration tests require Docker. Run 'make test-integration' separately."

# ============================================================================
# Code quality commands
# ============================================================================

# Run linter
lint:
	@echo "Running golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "ERROR: golangci-lint is not installed."; \
		echo "Install it with: go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	golangci-lint run ./...

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	@if ! command -v swag >/dev/null 2>&1; then \
		echo "WARNING: swag is not installed. Installing..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal

# Sync permissions from manifest to Casbin
permission-sync:
	@echo "Syncing permissions to Casbin..."
	go run ./cmd/api/main.go permission:sync

# ============================================================================
# Docker commands
# ============================================================================

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t go-api-base:latest .

# Run Docker Compose services
docker-run:
	@echo "Starting Docker services..."
	docker-compose up -d

# Stop and clean Docker
clean:
	@echo "Stopping containers and removing volumes..."
	docker-compose down -v

# ============================================================================
# Help
# ============================================================================

help:
	@echo "Go API Base - Available Commands"
	@echo ""
	@echo "Migration (Laravel-style):"
	@echo "  make migrate           Run all pending migrations"
	@echo "  make migrate-up        Same as 'migrate'"
	@echo "  make migrate-down       Rollback last migration step"
	@echo "  make migrate-rollback  Rollback with STEP=N (STEP=1 default)"
	@echo "  make migrate-status    Show migration status table"
	@echo "  make migrate-refresh   Rollback all + re-migrate"
	@echo "  make migrate-fresh     Drop all tables + re-migrate + seed"
	@echo ""
	@echo "Database:"
	@echo "  make seed              Seed database with initial data"
	@echo ""
	@echo "Server:"
	@echo "  make serve             Start the API server"
	@echo ""
	@echo "Testing:"
	@echo "  make test              Run unit tests (no Docker required)"
	@echo "  make test-integration  Run integration tests (requires Docker)"
	@echo "  make test-all          Run unit tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint              Run golangci-lint"
	@echo "  make swagger           Generate Swagger documentation"
	@echo ""
	@echo "Permissions:"
	@echo "  make permission-sync   Sync permissions to Casbin"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build      Build Docker image"
	@echo "  make docker-run        Run Docker Compose services"
	@echo "  make clean             Stop containers and remove volumes"