# Go API Base Makefile
# Project: github.com/example/go-api-base

.PHONY: migrate migrate-down seed serve test test-integration lint docker-build docker-run clean swagger

# Migration commands
migrate:
	@echo "Running migrations up..."
	migrate -path ./migrations -database "${DATABASE_URL}" up

migrate-down:
	@echo "Running migrations down..."
	migrate -path ./migrations -database "${DATABASE_URL}" down

# Seed database
seed:
	@echo "Running database seed..."
	go run ./cmd/api/main.go seed

# Run API server
serve:
	@echo "Starting API server..."
	go run ./cmd/api/main.go serve

# Run tests
test:
	@echo "Running all tests..."
	go test -v -race -coverprofile=coverage.txt ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v ./tests/integration/...

# Run linter
lint:
	@echo "Running golangci-lint..."
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
	swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal

# Help
help:
	@echo "Available targets:"
	@echo "  migrate          - Run database migrations up"
	@echo "  migrate-down     - Run database migrations down"
	@echo "  seed             - Run database seed"
	@echo "  serve            - Run API server"
	@echo "  test             - Run all tests"
	@echo "  test-integration - Run integration tests"
	@echo "  lint             - Run golangci-lint"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Run Docker Compose"
	@echo "  clean            - Stop containers and remove volumes"
	@echo "  swagger          - Generate Swagger documentation"
