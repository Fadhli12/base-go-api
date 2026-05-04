# Integration Tests

Integration tests that run against local PostgreSQL and Redis instances. No Docker required.

## Prerequisites

- **PostgreSQL**: Running locally (default: `localhost:5432`)
- **Redis**: Running locally (default: `localhost:6379`)
- **Go 1.22+**: Required for test execution
- **Test dependencies**: Already added to `go.mod`

## Running Tests

```bash
# Run all integration tests
go test -tags=integration ./tests/integration/... -v -timeout 10m

# Run specific test
go test -tags=integration ./tests/integration/... -v -timeout 5m -run TestAuthHandler_Register

# Run with serial execution (recommended for stability)
go test -tags=integration -p 1 -count=1 -timeout 10m ./tests/integration/...
```

## Configuration

The test suite automatically loads database and Redis configuration from:

1. **Environment variables** (highest priority):
   - `TEST_DATABASE_URL` — Full connection string (for CI/test isolation)
   - `DATABASE_URL` — Full connection string
   - `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`
   - `REDIS_URL` or `REDIS_HOST`, `REDIS_PORT`

2. **`.env` file** (auto-loaded from project root):
   - The test suite walks up from the working directory to find and load `.env`
   - Environment variables take precedence over `.env` values
   - This means your local `.env` credentials are automatically used

3. **Defaults** (if nothing is set):
   - PostgreSQL: `host=localhost port=5432 user=postgres password=postgres dbname=go_api_base sslmode=disable`
   - Redis: `localhost:6379`

## Architecture

### Shared Test Suite

All tests share a single `TestSuite` instance for performance:

- **First test**: Initializes DB connection, Redis connection, drops all tables, runs migrations
- **Each test**: Truncates all tables and flushes Redis for isolation
- **After all tests**: Closes connections in `TestMain`

```go
// Shared suite pattern — used automatically by NewTestSuite()
func TestMyFeature(t *testing.T) {
    suite := NewTestSuite(t)
    // suite.SetupTest(t) is called automatically
    // Use suite.DB and suite.RedisClient
}

// Alternative: SetupIntegrationTest() (same behavior)
func TestMyFeature2(t *testing.T) {
    suite := SetupIntegrationTest(t)
    defer suite.TeardownTest(t)
    // Use suite.DB and suite.RedisClient
}
```

### Test Isolation

Each test starts with a clean state:

1. **Database**: All tables are truncated before each test
2. **Redis**: Database is flushed before each test
3. **Schema**: On first run, all tables are dropped and recreated from migrations

### Migration Handling

Migrations are embedded directly in `testsuite.go`:

- **First run**: Drops all existing tables, then creates from scratch
- **Subsequent runs**: Skipped (idempotent guard)
- This ensures the schema always matches test expectations, even if the dev database has different column types

## Package Structure

```
tests/integration/
├── testsuite.go          # TestSuite, NewTestSuite, migrations, .env loader
├── main_test.go          # TestMain, SetupIntegrationTest, GetTestSuite
├── helpers/              # Test helper functions
├── auth_handler_test.go  # Auth endpoint tests
├── audit_log_test.go     # Audit logging tests
├── email_test.go         # Email service tests
├── media_test.go         # Media/upload tests
├── notification_test.go  # Notification preference tests
├── organization_test.go  # Organization service tests
├── role_test.go          # Role CRUD tests
├── permission_test.go    # Permission enforcement tests
└── ...
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "Failed to connect GORM to PostgreSQL" | Verify PostgreSQL is running: `pg_isready -h localhost -p 5432` |
| "Failed to ping Redis" | Verify Redis is running: `redis-cli ping` |
| Wrong password error | Check `.env` file has correct `DATABASE_PASSWORD` |
| Schema mismatch errors | Tests drop and recreate all tables on first run — restart if needed |
| Slow tests | Use `-p 1` flag for serial execution to avoid contention |

## Key Design Decisions

1. **No Docker containers** — Tests run against local PostgreSQL and Redis for speed and simplicity
2. **Shared suite** — Single DB/Redis connection pool shared across all tests for performance
3. **`.env` auto-loading** — Credentials are picked up automatically from project `.env` file
4. **Drop-and-recreate migrations** — Tests always start with a known schema state
5. **Serial execution recommended** — Use `-p 1` flag to avoid test interference