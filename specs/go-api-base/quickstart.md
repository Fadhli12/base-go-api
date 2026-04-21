# Quickstart: Go API Base

**Last Updated**: 2026-04-22 | **Status**: Phase 1 Design

Get the Go API Base running locally in 5 minutes.

---

## Prerequisites

- **Go 1.22+**: [Install Go](https://golang.org/doc/install)
- **Docker & Docker Compose**: [Install Docker Desktop](https://www.docker.com/products/docker-desktop)
- **Git**: [Install Git](https://git-scm.com/)
- **Make**: Usually pre-installed on macOS/Linux; on Windows, install [GNU Make](http://gnuwin32.sourceforge.net/packages/make.htm) or use WSL

---

## Step 1: Clone & Setup

```bash
# Clone repository
git clone https://github.com/your-org/go-api.git
cd go-api

# Copy example env file
cp config/.env.example config/.env

# Install Go dependencies
go mod download
```

**`.env` defaults** (edit if needed):
```bash
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/go_api_dev
REDIS_URL=redis://localhost:6379/0
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRY=900
REFRESH_TOKEN_EXPIRY=2592000
API_PORT=8080
LOG_LEVEL=info
```

---

## Step 2: Start Services (Postgres + Redis)

```bash
# Start Docker Compose (postgres:15, redis:7)
docker-compose up -d

# Verify services are running
docker-compose ps
```

**Output**:
```
NAME                COMMAND                  STATUS
go-api-postgres     docker-entrypoint.s…     Up 2 seconds
go-api-redis        redis-server             Up 2 seconds
```

Wait ~5 seconds for Postgres to be ready.

---

## Step 3: Run Migrations

```bash
# Install golang-migrate if not present
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations from migrations/ directory
make migrate
# OR manually:
migrate -path migrations -database "$DATABASE_URL" up
```

**Expected output**:
```
1/u init (16.123ms)
2/u audit_logs (8.456ms)
```

This creates base schema (users, roles, permissions, pivots, audit_logs).

---

## Step 4: Seed Initial Data

```bash
# Run seed command (creates system roles + permissions)
make seed
# OR:
go run cmd/api/main.go seed
```

**Created**:
- Roles: `SuperAdmin`, `Admin`, `Viewer`
- Permissions: `user:*`, `role:*`, `permission:*`, `invoice:*`, `audit_log:read`, etc.

---

## Step 5: Start API Server

```bash
# Run API in foreground (Ctrl+C to stop)
make serve
# OR:
go run cmd/api/main.go serve
```

**Expected output**:
```
2026-04-22T10:30:00.000Z INFO    Server starting on :8080
2026-04-22T10:30:00.123Z INFO    Database connected: postgresql://...
2026-04-22T10:30:00.456Z INFO    Redis connected: redis://...
```

API is now running at `http://localhost:8080`.

---

## Step 6: Test the API

### 6.1 Health Checks

```bash
# Liveness
curl http://localhost:8080/healthz
# Response: {"status":"healthy"}

# Readiness (checks DB + Redis)
curl http://localhost:8080/readyz
# Response: {"status":"ready","checks":{"database":"ok","redis":"ok"}}
```

### 6.2 Register User

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@example.com",
    "password": "SecurePass123!"
  }'
```

**Response** (201):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "alice@example.com",
    "created_at": "2026-04-22T10:30:00Z"
  },
  "error": null,
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440001",
    "timestamp": "2026-04-22T10:30:00Z"
  }
}
```

### 6.3 Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@example.com",
    "password": "SecurePass123!"
  }'
```

**Response** (200):
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "alice@example.com",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "access_token_expires_in": 900,
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token_expires_in": 2592000
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Save the tokens**:
```bash
export ACCESS_TOKEN="<access_token_from_response>"
export USER_ID="<user_id_from_response>"
```

### 6.4 Get Effective Permissions

```bash
curl -X GET http://localhost:8080/api/v1/users/$USER_ID/effective-permissions \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "permissions": [
      {
        "id": "...",
        "name": "user:create",
        "resource": "user",
        "action": "create",
        "effect": "allow"
      }
    ]
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

(Will show whatever permissions alice has via roles.)

### 6.5 Create Invoice

```bash
curl -X POST http://localhost:8080/api/v1/invoices \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 15000
  }'
```

**Response** (201):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "amount": 15000,
    "status": "draft",
    "created_at": "2026-04-22T10:30:00Z",
    "updated_at": "2026-04-22T10:30:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

### 6.6 List Invoices

```bash
curl -X GET http://localhost:8080/api/v1/invoices \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "invoices": [
      { /* invoice created above */ }
    ],
    "total": 1,
    "limit": 20,
    "offset": 0
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

### 6.7 Refresh Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<refresh_token_from_login>"
  }'
```

**Response** (200): New access_token + new refresh_token (rotation).

### 6.8 Logout

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "message": "Successfully logged out"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

After logout, refresh tokens are marked revoked; access_token still valid until expiry (expires in 15 min by default).

---

## Useful Commands

```bash
# Run tests
make test

# Run integration tests (requires Docker)
make test-integration

# Check code
make lint

# Build Docker image
make docker-build

# View logs (API)
docker-compose logs api

# View logs (Postgres)
docker-compose logs postgres

# Stop all services
docker-compose down

# Full reset (⚠️ deletes data)
docker-compose down -v
make migrate
make seed
make serve
```

---

## Directory Structure

```
.
├── cmd/api/main.go                 # Entrypoint
├── internal/
│   ├── config/                     # Viper config
│   ├── database/                   # Postgres + Redis connections
│   ├── http/
│   │   ├── server.go               # Echo server setup
│   │   ├── middleware/             # JWT, permission, audit, etc.
│   │   ├── handler/                # HTTP handlers (auth, role, invoice, etc.)
│   │   ├── request/                # DTOs + validation
│   │   └── response/               # Envelope types
│   ├── domain/                     # Entities
│   ├── repository/                 # GORM queries
│   ├── service/                    # Business logic
│   ├── permission/                 # Casbin, cache, sync
│   ├── auth/                       # JWT, password, tokens
│   └── module/invoice/             # Example domain module
├── migrations/                     # SQL migration files
├── config/
│   ├── .env.example                # Environment variables
│   └── rbac_model.conf             # Casbin model
├── tests/                          # Unit + integration tests
├── docker-compose.yml              # Local dev services
├── Makefile                        # Common commands
├── go.mod                          # Go modules
└── README.md                       # Full documentation
```

---

## Troubleshooting

### "Connection refused" on Postgres

```bash
# Postgres not ready yet; wait 10s and retry
docker-compose logs postgres

# Force restart Postgres
docker-compose restart postgres
```

### "Permission denied" on create invoice

User doesn't have `invoice:create` permission. Check effective permissions:
```bash
curl -X GET http://localhost:8080/api/v1/users/$USER_ID/effective-permissions \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Assign role with permission:
```bash
# Admin endpoint to grant role to user
curl -X POST http://localhost:8080/api/v1/users/$USER_ID/roles \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "role_id": "<SuperAdmin role_id>"
  }'
```

### Migration fails

```bash
# Check migration status
migrate -path migrations -database "$DATABASE_URL" version

# Force to version (use with caution)
migrate -path migrations -database "$DATABASE_URL" force 2
```

### Port 8080 already in use

```bash
# Find process on port 8080
lsof -i :8080

# Kill process (macOS/Linux)
kill -9 <PID>

# Or use different port in .env
API_PORT=8081
```

---

## Next Steps

1. **Explore API docs**: Open Swagger UI at `http://localhost:8080/swagger/index.html` (requires Phase 2 Swagger setup)
2. **Import Postman collection**: Use `docs/postman.json` (generated in Phase 2)
3. **Read data-model.md**: Understand entity relationships
4. **Read contracts/api-v1.md**: Full endpoint specification
5. **Run integration tests**: `make test-integration`

---

**Questions?** See `README.md` or consult `IMPLEMENTATION_PLAN.md` for architecture details.
