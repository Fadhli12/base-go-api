# Integration Tests

This package provides integration test infrastructure using [testcontainers-go](https://golang.testcontainers.org/) for isolated, ephemeral PostgreSQL and Redis containers.

## Prerequisites

- **Docker**: Must be running locally (testcontainers creates ephemeral containers)
- **Go 1.22+**: Required for testcontainers-go
- **Test dependencies**: Already added to `go.mod`

## Running Tests

Integration tests are tagged with `//go:build integration` to prevent them from running during normal test execution. To run integration tests:

```bash
# Run all integration tests
go test ./tests/integration/... -tags=integration -v

# Run specific test
go test ./tests/integration/... -tags=integration -v -run TestInfrastructure_Initialization

# Run with timeout (containers take time to start)
go test ./tests/integration/... -tags=integration -v -timeout 5m
```

## Architecture

### TestSuite

The `TestSuite` struct manages the complete test lifecycle:

```go
type TestSuite struct {
    DB             *gorm.DB              // GORM database connection
    RedisClient    *redis.Client         // Redis client
    PGContainer    *postgres.PostgresContainer
    RedisContainer testcontainers.Container
    Cleanup        func()
}
```

### Initialization

The test suite is initialized lazily on first use via `SetupIntegrationTest()`:

```go
func TestMyFeature(t *testing.T) {
    suite := SetupIntegrationTest(t)
    defer suite.TeardownTest(t)
    
    // Use suite.DB for database operations
    // Use suite.RedisClient for Redis operations
}
```

### Test Isolation

Each test starts with a clean state:

1. **Database**: All tables are truncated before each test
2. **Redis**: Database is flushed before each test
3. **Containers**: Single container instance shared across tests (created once)

### Migration Handling

Migrations are executed once during initial setup:

- `000001_init.up.sql`: Base schema (users, roles, permissions, pivots, refresh_tokens)
- `000002_audit_logs.up.sql`: Audit logging table

## Package Structure

```
tests/integration/
├── testsuite.go          # TestSuite implementation, container setup, migrations
├── main_test.go          # TestMain entry point, lifecycle management
├── infrastructure_test.go # Infrastructure validation tests
└── README.md             # This file
```

## Example Tests

### Basic Database Test

```go
func TestDatabaseInsert(t *testing.T) {
    suite := SetupIntegrationTest(t)
    defer suite.TeardownTest(t)

    // Insert test user
    result := suite.DB.Exec(`
        INSERT INTO users (email, password_hash) 
        VALUES ('test@example.com', 'hash123')
    `)
    require.NoError(t, result.Error)

    // Verify insertion
    var count int64
    suite.DB.Raw("SELECT COUNT(*) FROM users").Scan(&count)
    require.Equal(t, int64(1), count)
}
```

### Redis Operations Test

```go
func TestRedisCaching(t *testing.T) {
    suite := SetupIntegrationTest(t)
    defer suite.TeardownTest(t)

    ctx := context.Background()
    
    // Set value
    err := suite.RedisClient.Set(ctx, "key", "value", 10*time.Second).Err()
    require.NoError(t, err)

    // Get value
    val, err := suite.RedisClient.Get(ctx, "key").Result()
    require.NoError(t, err)
    require.Equal(t, "value", val)
}
```

## Container Lifecycle

1. **First Test**: Containers start, migrations run
2. **Each Test**: Tables truncated, Redis flushed
3. **All Tests Complete**: Containers terminate

Containers are shared across tests for performance, but data is isolated.

## Debugging

### Enable Container Logs

```go
// In testsuite.go, modify container creation:
pgContainer, err := postgrescontainer.Run(ctx, "postgres:15-alpine",
    postgrescontainer.WithDatabase("testdb"),
    postgrescontainer.WithUsername("test"),
    postgrescontainer.WithPassword("test"),
    testcontainers.WithLogger(testcontainers.TestLogger(t)), // Enable logs
)
```

### Inspect Container State

```go
// Get Redis address
addr := suite.GetRedisAddr()
t.Logf("Redis running at: %s", addr)

// Check database connectivity
var result int
suite.DB.Raw("SELECT 1").Scan(&result)
t.Logf("Database responsive: %d", result)
```

## Troubleshooting

### "rootless Docker is not supported on Windows"

**Solution**: Ensure Docker Desktop is running and you're using Docker Engine (not rootless mode).

```bash
# Check Docker status
docker ps

# If not running, start Docker Desktop
```

### Container Startup Timeout

**Solution**: Increase test timeout:

```bash
go test ./tests/integration/... -tags=integration -v -timeout 10m
```

### Port Conflicts

**Solution**: Testcontainers uses random ports, but if conflicts occur:

1. Stop existing containers: `docker ps -a | grep test | docker rm -f`
2. Verify ports not in use: `netstat -an | grep 5432`, `netstat -an | grep 6379`

## Dependencies Added

```go
// go.mod additions
require (
    github.com/testcontainers/testcontainers-go v0.42.0
    github.com/testcontainers/testcontainers-go/modules/postgres v0.42.0
    github.com/testcontainers/testcontainers-go/modules/redis v0.42.0
    github.com/stretchr/testify v1.10.0
)
```

## Best Practices

1. **Use `SetupIntegrationTest(t)`** for automatic test isolation
2. **Always defer `TeardownTest(t)`** for extensibility
3. **Don't share test suite across packages** - each test file should call `SetupIntegrationTest`
4. **Keep tests independent** - each test should pass in isolation
5. **Use `require` for fatal assertions** - fail fast on critical errors

## Related Documentation

- [Testcontainers-go Documentation](https://golang.testcontainers.org/)
- [PostgreSQL Module](https://golang.testcontainers.org/modules/postgres/)
- [Redis Module](https://golang.testcontainers.org/modules/redis/)
- [Testify Documentation](https://github.com/stretchr/testify)