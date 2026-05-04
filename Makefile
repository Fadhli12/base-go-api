# Go API Base Makefile
# Project: github.com/example/go-api-base

# Load environment variables from .env file (if it exists)
-include .env
export

# Build DATABASE_URL from individual components if not already set
ifeq ($(DATABASE_URL),)
DATABASE_URL := postgres://$(DATABASE_USER):$(DATABASE_PASSWORD)@$(DATABASE_HOST):$(DATABASE_PORT)/$(DATABASE_NAME)?sslmode=$(DATABASE_SSL_MODE)
endif

.PHONY: migrate migrate-down seed serve test test-all test-integration lint docker-build docker-run clean swagger permission-sync mempalace-status mempalace-search mempalace-mcp mempalace-context help

# Migration commands
migrate:
	@echo "Running migrations up..."
	go run ./cmd/api/main.go migrate

migrate-down:
	@echo "Running migrations down..."
	go run ./cmd/api/main.go migrate --down

# Seed database
seed:
	@echo "Running database seed..."
	go run ./cmd/api/main.go seed

# Run API server
serve:
	@echo "Starting API server..."
	go run ./cmd/api/main.go serve

# Run unit tests (excludes integration tests which require Docker)
test:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.txt ./tests/unit/...

# Run integration tests (requires Docker running)
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./tests/integration/... -timeout 5m

# Run all tests (unit + integration)
test-all:
	@echo "Running all tests..."
	go test -v -race -coverprofile=coverage.txt ./tests/unit/...
	@echo ""
	@echo "Note: Integration tests require Docker. Run 'make test-integration' separately."

# Run linter
lint:
	@echo "Running golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "ERROR: golangci-lint is not installed."; \
		echo "Install it with: go install github.com/golangci-lint/golangci-lint@latest"; \
		exit 1; \
	fi
	golangci-lint run ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t go-api-base:latest .

# Run docker-compose
docker-run:
	@echo "Starting Docker services..."
	docker-compose up -d

# Stop and clean Docker
clean:
	@echo "Stopping containers and removing volumes..."
	docker-compose down -v

# Generate Swagger docs
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

# MemPalace integration commands
mempalace-status:
	@echo "Checking MemPalace status..."
	@mempalace status 2>/dev/null || (echo "ERROR: mempalace is not installed." && echo "Install it from: https://mempalaceofficial.com" && exit 1)

mempalace-search:
	@echo "MemPalace semantic search"
	@if [ -z "$(QUERY)" ]; then \
		echo "Usage: make mempalace-search QUERY='your search query'"; \
		echo "Examples:"; \
		echo "  make mempalace-search QUERY='webhook retry'"; \
		echo "  make mempalace-search QUERY='permission check'"; \
		echo "  make mempalace-search QUERY='event bus'"; \
		exit 1; \
	fi
	@powershell -NoProfile -Command "[System.Environment]::SetEnvironmentVariable('PYTHONIOENCODING','utf-8'); mempalace search '$(QUERY)'"

mempalace-mcp:
	@echo "Starting MemPalace MCP server (port 3000)..."
	@if ! command -v mempalace-mcp >/dev/null 2>&1; then \
		echo "ERROR: mempalace-mcp is not installed."; \
		echo "Install it from: https://mempalaceofficial.com"; \
		exit 1; \
	fi
	mempalace-mcp

mempalace-context:
	@echo "Loading MemPalace context for agent task"
	@if [ -z "$(TASK_CONTEXT)" ]; then \
		echo "Usage: make mempalace-context TASK_CONTEXT='your task description'"; \
		echo "Example: make mempalace-context TASK_CONTEXT='webhook delivery retry'"; \
		exit 1; \
	fi
	@echo "Task context: $(TASK_CONTEXT)"
	@powershell -NoProfile -Command ".\.sisyphus\hooks\pre-agent.ps1 -TaskContext '$(TASK_CONTEXT)'"

mempalace-post-decision:
	@if [ -z "$(TASK_STATUS)" ]; then \
		echo "Error: TASK_STATUS is required"; \
		echo "Example: make mempalace-post-decision TASK_STATUS='success' DECISION='Implemented webhook retry' FILES_MODIFIED='internal/service/webhook_worker.go'"; \
		exit 1; \
	fi
	@if [ -z "$(DECISION)" ]; then \
		echo "Error: DECISION is required"; \
		echo "Example: make mempalace-post-decision TASK_STATUS='success' DECISION='Implemented webhook retry' FILES_MODIFIED='internal/service/webhook_worker.go'"; \
		exit 1; \
	fi
	@echo "Filing decision to MemPalace..."
	@echo "Status: $(TASK_STATUS)"
	@echo "Decision: $(DECISION)"
	@echo "Files: $(FILES_MODIFIED)"
	@powershell -NoProfile -Command ".\.sisyphus\hooks\post-agent.ps1 -TaskStatus '$(TASK_STATUS)' -Decision '$(DECISION)' -FilesModified '$(FILES_MODIFIED)'"

# Help
help:
	@echo "Available targets:"
	@echo "  migrate           - Run database migrations up"
	@echo "  migrate-down      - Run database migrations down"
	@echo "  seed              - Run database seed"
	@echo "  serve             - Run API server"
	@echo "  test              - Run unit tests (no Docker required)"
	@echo "  test-all          - Run unit tests"
	@echo "  test-integration  - Run integration tests (requires Docker)"
	@echo "  lint              - Run golangci-lint"
	@echo "  docker-build      - Build Docker image"
	@echo "  docker-run        - Run Docker Compose"
	@echo "  clean             - Stop containers and remove volumes"
	@echo "  swagger           - Generate Swagger documentation"
	@echo "  permission-sync   - Sync permissions to Casbin"
	@echo ""
	@echo "MemPalace integration:"
	@echo "  mempalace-status  - Check MemPalace palace status"
	@echo "  mempalace-search  - Search MemPalace (QUERY='...')"
	@echo "  mempalace-mcp     - Start MCP server for Claude Code"
	@echo "  mempalace-context - Load context for agent task (TASK_CONTEXT='...')"
