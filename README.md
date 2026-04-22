# Go API Base

A production-ready Go REST API foundation with RBAC, JWT authentication, and comprehensive permission management.

## Features

- **JWT Authentication** - Secure token-based authentication with access and refresh tokens
- **Role-Based Access Control (RBAC)** - Flexible permission system powered by Casbin
- **Permission Management** - Fine-grained permission grants with resource/action model
- **Audit Logging** - Comprehensive audit trail for compliance and security
- **Rate Limiting** - Built-in protection against abuse
- **Health Checks** - `/healthz` and `/readyz` endpoints for orchestration
- **Graceful Shutdown** - 30-second timeout for clean connections drain
- **API Documentation** - Auto-generated Swagger/OpenAPI docs

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Make (optional)

### Using Docker (Recommended)

```bash
# Start all services (PostgreSQL, Redis, API)
make docker-run

# Or with docker-compose directly
docker-compose up -d
```

The API will be available at `http://localhost:8080`.

### Local Development

1. **Clone and install dependencies:**
   ```bash
   git clone https://github.com/example/go-api-base.git
   cd go-api-base
   go mod download
   ```

2. **Start dependencies:**
   ```bash
   docker-compose up -d postgres redis
   ```

3. **Configure environment:**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Run migrations:**
   ```bash
   make migrate
   ```

5. **Start the server:**
   ```bash
   make serve
   ```

## API Documentation

- **Swagger UI**: `http://localhost:8080/swagger/index.html`
- **OpenAPI Spec**: `http://localhost:8080/swagger/doc.json`

## Project Structure

```
go-api-base/
├── cmd/
│   └── api/           # Application entrypoint
│       └── main.go    # CLI commands and server setup
├── internal/
│   ├── config/       # Configuration management
│   ├── database/     # Database connections (PostgreSQL, Redis)
│   ├── http/         # HTTP handlers, middleware, routing
│   ├── models/       # Domain entities (User, Role, Permission)
│   ├── permission/   # Casbin integration, caching, invalidation
│   ├── repository/   # Data access layer
│   └── service/      # Business logic layer
├── migrations/       # SQL migration files
├── docs/
│   └── swagger/      # Auto-generated Swagger docs
├── config/
│   └── permissions.yaml  # Permission manifest
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `JWT_SECRET` | JWT signing secret (min 32 chars) | Required |
| `ACCESS_TOKEN_TTL` | Access token lifetime | `15m` |
| `REFRESH_TOKEN_TTL` | Refresh token lifetime | `168h` |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `GIN_MODE` | Gin mode (debug/release) | `debug` |

See `.env.example` for full configuration options.

## Available Make Commands

```bash
make serve             # Start the API server
make migrate           # Run database migrations up
make migrate-down      # Run database migrations down
make seed              # Seed database with initial data
make test              # Run all tests
make test-integration  # Run integration tests
make lint              # Run golangci-lint
make docker-build      # Build Docker image
make docker-run        # Start Docker Compose services
make clean             # Stop containers and remove volumes
make swagger           # Generate Swagger documentation
make help              # Show all available commands
```

## Health Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Simple liveness check (always returns 200) |
| `GET /readyz` | Readiness check (verifies DB and Redis connections) |

## Deployment

### Docker

```bash
# Build the image
docker build -t go-api-base:latest .

# Run with custom config
docker run -d \
  -p 8080:8080 \
  -e DATABASE_URL=postgres://... \
  -e REDIS_URL=redis://... \
  -e JWT_SECRET=your-secret \
  go-api-base:latest
```

### Production Checklist

- [ ] Set `GIN_MODE=release`
- [ ] Set `LOG_LEVEL=info`
- [ ] Generate secure `JWT_SECRET` (min 32 chars)
- [ ] Enable SSL for database connections
- [ ] Configure rate limiting
- [ ] Set up log aggregation
- [ ] Configure monitoring and alerting

## Security

- Non-root container user
- Bcrypt password hashing
- JWT token rotation
- Rate limiting
- Input validation
- SQL injection prevention via GORM

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Run tests and linting
4. Submit a pull request