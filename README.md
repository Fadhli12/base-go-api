# Go API Base

A production-ready Go REST API foundation with RBAC, JWT authentication, and comprehensive permission management.

## Features

- **JWT Authentication** - Secure token-based authentication with access and refresh tokens
- **Role-Based Access Control (RBAC)** - Flexible permission system powered by Casbin
- **Permission Management** - Fine-grained permission grants with resource/action model
- **Audit Logging** - Comprehensive audit trail for compliance and security
- **Structured Logging** - Context-aware logging with slog, multiple output writers, and request tracing
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

## Dynamic Configuration

This project supports dynamic configuration via environment variables. Copy `.env.example` to `.env` and configure:

### Storage Configuration
- `STORAGE_DRIVER` - Storage backend: `local`, `s3`, or `minio`
- `STORAGE_LOCAL_PATH` - Local storage path (default: ./storage/uploads)
- `STORAGE_BASE_URL` - Public URL for storage (default: http://localhost:8080/storage)
- S3/MinIO: Set `STORAGE_S3_BUCKET`, `STORAGE_S3_ACCESS_KEY`, `STORAGE_S3_SECRET_KEY`

### Image Compression
- `IMAGE_COMPRESSION_ENABLED` - Enable/disable compression (default: true)
- `IMAGE_COMPRESSION_THUMBNAIL_QUALITY` - Thumbnail quality 1-100 (default: 85)
- `IMAGE_COMPRESSION_THUMBNAIL_WIDTH/HEIGHT` - Thumbnail dimensions (default: 300x300)

### Cache Configuration
- `CACHE_DRIVER` - Cache backend: `redis`, `memory`, or `none`
- `CACHE_PERMISSION_TTL` - Permission cache TTL in seconds (default: 300)

### Swagger
- `SWAGGER_ENABLED` - Enable Swagger UI (default: true)
- `SWAGGER_PATH` - Path for Swagger UI (default: /swagger)

### Structured Logging
- `LOG_LEVEL` - Log level: `debug`, `info`, `warn`, `error` (default: `info`)
- `LOG_FORMAT` - Log format: `json` or `text` (default: `json`)
- `LOG_OUTPUTS` - Output destinations: comma-separated list of `stdout`, `file`, `syslog` (default: `stdout`)
- `LOG_FILE_PATH` - Path to log file when using file output (default: `/var/log/api.log`)
- `LOG_FILE_MAX_SIZE` - Max log file size in MB before rotation (default: `100`)
- `LOG_FILE_MAX_BACKUPS` - Number of backup log files to keep (default: `10`)
- `LOG_FILE_MAX_AGE` - Days to keep backup log files (default: `30`)
- `LOG_FILE_COMPRESS` - Compress rotated log files (default: `true`)
- `LOG_SYSLOG_NETWORK` - Syslog network: `tcp`, `udp`, or empty for local (default: empty)
- `LOG_SYSLOG_ADDRESS` - Syslog server address (default: empty)
- `LOG_SYSLOG_TAG` - Syslog tag/identifier (default: `go-api`)
- `LOG_ADD_SOURCE` - Include file:line in log output (default: `false`)

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

## Structured Logging

The API includes a comprehensive structured logging system with context propagation, multiple output writers, and automatic field extraction.

### Basic Usage

```go
// In HTTP handlers
func (h *Handler) GetUser(c echo.Context) error {
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    
    log.Info(ctx, "fetching user",
        log.String("user_id", userID),
    )
    
    user, err := h.service.GetUser(ctx, userID)
    if err != nil {
        log.Error(ctx, "failed to fetch user",
            log.String("user_id", userID),
            logger.Err(err),
        )
        return echo.NewHTTPError(http.StatusNotFound, "User not found")
    }
    
    return c.JSON(http.StatusOK, user)
}
```

### Automatic Context Fields

The middleware automatically extracts and logs:
- `request_id` - From X-Request-ID header or generated UUID
- `user_id` - From JWT claims
- `org_id` - From X-Organization-ID header
- `trace_id` - From X-Trace-ID header or generated

### Field Types

```go
log.String("key", "value")
log.Int("count", 42)
log.Int64("timestamp", time.Now().Unix())
log.Float64("percentage", 99.9)
log.Bool("active", true)
log.Duration("elapsed", time.Second)
log.Time("created_at", time.Now())
log.Any("metadata", map[string]interface{}{...})
logger.Err(err)
```

### Configuration Examples

**JSON output to stdout (default):**
```bash
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUTS=stdout
```

**Multiple outputs with file rotation:**
```bash
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUTS=stdout,file
LOG_FILE_PATH=/var/log/api.log
LOG_FILE_MAX_SIZE=100
LOG_FILE_MAX_BACKUPS=10
LOG_FILE_MAX_AGE=30
```

**Syslog integration:**
```bash
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUTS=syslog
LOG_SYSLOG_NETWORK=tcp
LOG_SYSLOG_ADDRESS=logs.example.com:514
LOG_SYSLOG_TAG=go-api
```

### Performance

- Log call overhead: < 1ms
- Middleware overhead: < 2ms per request
- Zero-allocation for constant keys
- Goroutine-safe implementations

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