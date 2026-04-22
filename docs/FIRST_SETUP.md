# First-Time Setup Guide

**Go API Base** - Production-ready REST API with JWT auth, RBAC, and audit logging.

This guide will get you from clone to running API in under 5 minutes.

---

## Prerequisites

| Requirement | Version | Install Link |
|-------------|---------|--------------|
| Go | 1.22+ | [golang.org/doc/install](https://golang.org/doc/install) |
| Docker Desktop | Latest | [docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop) |
| Git | Any | [git-scm.com](https://git-scm.com/) |
| Make (optional) | Any | Pre-installed on macOS/Linux; [GNU Make for Windows](http://gnuwin32.sourceforge.net/packages/make.htm) |

**Verify installations:**
```bash
go version        # should show go1.22.x or higher
docker --version  # should show Docker version
docker-compose --version
```

---

## Quick Start (3 Commands)

If you have Make installed:

```bash
# 1. Start services (PostgreSQL + Redis + API)
make docker-run

# 2. Run migrations (in another terminal)
make migrate

# 3. Seed initial data
make seed
```

API runs at **http://localhost:8080**

---

## Detailed Setup

### Step 1: Clone Repository

```bash
git clone https://github.com/example/go-api-base.git
cd go-api-base
```

### Step 2: Configure Environment

```bash
# Copy example environment file
cp config/.env.example .env
```

**Edit `.env` for your environment:**

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `DATABASE_URL` | Yes | PostgreSQL connection | `postgres://postgres:postgres@localhost:5432/go_api_base?sslmode=disable` |
| `REDIS_URL` | Yes | Redis connection | `redis://localhost:6379/0` |
| `JWT_SECRET` | **Yes** | Signing key (min 32 chars) | ⚠️ Change in production! |
| `SERVER_PORT` | No | API port | `8080` |
| `LOG_LEVEL` | No | Log verbosity | `info` |
| `CORS_ALLOWED_ORIGINS` | No | Allowed origins | `http://localhost:3000` |

**⚠️ Production Security:**
```bash
# Generate a secure JWT secret (32+ characters)
openssl rand -base64 32
# Or use: head -c 32 /dev/urandom | base64
```

### Step 3: Start Infrastructure

```bash
# Start PostgreSQL and Redis containers
docker-compose up -d postgres redis

# Wait for services (5-10 seconds)
docker-compose ps
```

**Expected output:**
```
NAME                STATUS              PORTS
go-api-postgres     Up (healthy)        0.0.0.0:5432->5432/tcp
go-api-redis        Up (healthy)         0.0.0.0:6379->6379/tcp
```

### Step 4: Install Migration Tool

```bash
# Install golang-migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

**Verify:**
```bash
migrate -version
# Output: 4.x.x
```

### Step 5: Run Database Migrations

```bash
# Set environment (PowerShell)
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/go_api_base?sslmode=disable"

# Run migrations
make migrate
# OR: migrate -path migrations -database $env:DATABASE_URL up
```

**Expected output:**
```
1/u init (16.123ms)
2/u audit_logs (8.456ms)
```

### Step 6: Seed Initial Data

```bash
make seed
# OR: go run cmd/api/main.go seed
```

**Creates:**
- **Roles**: `SuperAdmin`, `Admin`, `Viewer`
- **Permissions**: `user:*`, `role:*`, `permission:*`, `invoice:*`, `audit_log:read`

### Step 7: Start the API Server

```bash
make serve
# OR: go run cmd/api/main.go serve
```

**Expected output:**
```
2026-04-22T10:30:00Z INFO  Server starting on :8080
2026-04-22T10:30:00Z INFO  Database connected: postgresql://...
2026-04-22T10:30:00Z INFO  Redis connected: redis://...
```

---

## Verify Installation

### Health Checks

```bash
# Liveness check
curl http://localhost:8080/healthz
# Response: {"status":"healthy"}

# Readiness check (verifies DB + Redis)
curl http://localhost:8080/readyz
# Response: {"status":"ready","checks":{"database":"ok","redis":"ok"}}
```

### Create Your First User

```bash
# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecurePass123!"}'

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecurePass123!"}'

# Response includes access_token and refresh_token
```

### Swagger Documentation

Open in browser: **http://localhost:8080/swagger/index.html**

---

## Common Commands

| Command | Description |
|---------|-------------|
| `make serve` | Start API server |
| `make migrate` | Run database migrations |
| `make migrate-down` | Rollback last migration |
| `make seed` | Seed initial data |
| `make test` | Run unit tests |
| `make test-integration` | Run integration tests (requires Docker) |
| `make lint` | Run golangci-lint |
| `make docker-run` | Start all services (Postgres + Redis + API) |
| `make clean` | Stop containers and remove volumes |
| `make help` | Show all commands |

---

## Troubleshooting

### "Connection refused" on PostgreSQL

```bash
# Check if PostgreSQL is running
docker-compose ps postgres

# View logs
docker-compose logs postgres

# Restart
docker-compose restart postgres

# Wait 10 seconds for startup
```

### "Connection refused" on Redis

```bash
# Check if Redis is running
docker-compose ps redis

# Test connection
docker exec go-api-redis redis-cli ping
# Should respond: PONG

# Restart
docker-compose restart redis
```

### Port 8080 Already in Use

```bash
# Find process on port
# Windows:
netstat -ano | findstr :8080

# Stop the process or change port in .env
SERVER_PORT=8081
```

### Migration Fails

```bash
# Check current migration version
migrate -path migrations -database "$DATABASE_URL" version

# Force to specific version (use with caution)
migrate -path migrations -database "$DATABASE_URL" force 2

# Reset everything (DESTRUCTIVE)
docker-compose down -v
docker-compose up -d postgres redis
make migrate
make seed
```

### Permission Denied Errors

After login, if you get `403 Forbidden`:

```bash
# Check your effective permissions
curl -X GET http://localhost:8080/api/v1/users/{user_id}/effective-permissions \
  -H "Authorization: Bearer {access_token}"

# Assign a role (requires admin)
curl -X POST http://localhost:8080/api/v1/users/{user_id}/roles \
  -H "Authorization: Bearer {admin_token}" \
  -H "Content-Type: application/json" \
  -d '{"role_id":"{super_admin_role_id}"}'
```

---

## Development Workflow

### Hot Reload (Recommended)

Install [Air](https://github.com/air-verse/air) for live reload:

```bash
go install github.com/air-verse/air@latest

# Run with hot reload
air
```

### Running Tests

```bash
# Unit tests
make test

# Integration tests (requires Docker)
make test-integration

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Database Management

```bash
# Connect to PostgreSQL
docker exec -it go-api-postgres psql -U postgres -d go_api_base

# Backup database
docker exec go-api-postgres pg_dump -U postgres go_api_base > backup.sql

# Restore database
docker exec -i go-api-postgres psql -U postgres go_api_base < backup.sql
```

---

## Project Structure

```
go-api-base/
├── cmd/api/main.go          # Application entrypoint (Cobra CLI)
├── internal/
│   ├── config/              # Configuration (Viper)
│   ├── database/            # PostgreSQL + Redis connections
│   ├── http/
│   │   ├── server.go        # Echo server setup
│   │   ├── handler/         # HTTP handlers
│   │   ├── middleware/      # Auth, permissions, audit, rate limiting
│   │   ├── request/         # DTOs with validation
│   │   └── response/        # JSON envelope types
│   ├── domain/              # Entity definitions
│   ├── repository/          # Data access layer (GORM)
│   ├── service/             # Business logic layer
│   ├── auth/                # JWT, password hashing, token management
│   ├── permission/          # Casbin enforcement, caching
│   └── module/invoice/      # Example domain module
├── migrations/              # SQL migration files
├── config/
│   ├── .env.example         # Environment template
│   └── rbac_model.conf      # Casbin RBAC model
├── tests/
│   ├── integration/         # Testcontainers integration tests
│   ├── unit/                # Unit tests
│   └── contract/            # Swagger contract tests
├── docs/
│   ├── swagger/             # Generated Swagger docs
│   ├── postman.json         # Postman collection
│   └── FIRST_SETUP.md      # This file
├── docker-compose.yml       # Local development services
├── Dockerfile               # Production image
├── Makefile                 # Common commands
├── go.mod                   # Go modules
└── README.md                # Project overview
```

---

## Next Steps

1. **API Documentation** → Open [Swagger UI](http://localhost:8080/swagger/index.html)
2. **Postman Collection** → Import `docs/postman.json`
3. **Full API Reference** → See `docs/api-examples.md`
4. **Architecture Details** → See `specs/go-api-base/data-model.md`
5. **Deployment Guide** → See `docs/runbook.md`

---

## Getting Help

- **Documentation**: Check `README.md` and `docs/` directory
- **API Contracts**: See `specs/go-api-base/contracts/api-v1.md`
- **Data Model**: See `specs/go-api-base/data-model.md`
- **Issues**: Create an issue in the repository

---

**Need to reset everything?**

```bash
# Stop containers and remove all data
docker-compose down -v

# Start fresh
docker-compose up -d postgres redis
make migrate
make seed
make serve
```