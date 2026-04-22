# Integration Tests - testcontainers-go

**Location:** tests/integration | **Purpose:** Ephemeral container-based integration tests

---

## OVERVIEW

Integration test suite using testcontainers-go for real PostgreSQL and Redis containers. Each test runs in isolation with clean database state.

## TEST STRUCTURE

```
tests/integration/
├── testsuite.go     # Container setup, DB/Redis clients
├── main_test.go     # TestMain (optional)
├── auth_*.go        # Auth flow tests
├── role_*.go        # Role management tests
├── permission_*.go  # Permission enforcement tests
├── invoice_*.go    # Domain module tests
└── README.md
```

## RUNNING

```bash
# Requires Docker running
go test -v -tags=integration ./tests/integration/... -timeout 5m

# Or via make
make test-integration
```

## TESTSUITE PATTERN

### Creation

```go
//go:build integration

func TestMyFeature(t *testing.T) {
    suite := NewTestSuite(t)
    defer suite.Cleanup()
    
    // Run migrations once
    suite.RunMigrations(t)
    
    // Use suite.DB and suite.RedisClient
}
```

### TestSuite struct

```go
type TestSuite struct {
    DB             *gorm.DB
    RedisClient    *redis.Client
    PGContainer    *postgrescontainer.PostgresContainer
    RedisContainer testcontainers.Container
    Cleanup        func()
}
```

### Cleanup Pattern

```go
func TestSomething(t *testing.T) {
    suite := NewTestSuite(t)
    defer suite.Cleanup()  // Always defer cleanup
    
    t.Run("subtest", func(t *testing.T) {
        suite.SetupTest(t)  // Clean before each subtest
        // test code
    })
}
```

## CONTAINER DETAILS

### PostgreSQL

```go
pgContainer, _ := postgrescontainer.Run(ctx, "postgres:15-alpine",
    postgrescontainer.WithDatabase("testdb"),
    postgrescontainer.WithUsername("test"),
    postgrescontainer.WithPassword("test"),
)
connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
```

### Redis

```go
redisContainer, _ := rediscontainer.Run(ctx, "redis:7-alpine")
redisHost, _ := redisContainer.Host(ctx)
redisPort, _ := redisContainer.MappedPort(ctx, "6379")
```

## MIGRATIONS

Embedded directly in testsuite.go (not file-based):

```go
func (s *TestSuite) RunMigrations(t *testing.T) {
    // Migration 000001: Base schema
    migration001 := `
        CREATE EXTENSION IF NOT EXISTS "pgcrypto";
        CREATE TABLE users (...);
        -- full schema DDL
    `
    s.DB.Exec(migration001)
    
    // Migration 000002: Audit logs
    migration002 := `
        CREATE TABLE audit_logs (...);
        -- audit trigger
    `
    s.DB.Exec(migration002)
}
```

## TEST ISOLATION

### SetupTest (per-test cleanup)

```go
func (s *TestSuite) SetupTest(t *testing.T) {
    // Truncate ALL tables (order matters for FKs)
    tables := []string{
        "user_permissions",
        "role_permissions", 
        "user_roles",
        "refresh_tokens",
        "audit_logs",
        "invoices",
        "permissions",
        "roles",
        "users",
    }
    for _, table := range tables {
        s.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
    }
    
    // Flush Redis
    s.RedisClient.FlushDB(ctx)
}
```

## TEST PATTERNS

### Creating Test User

```go
func createTestUser(t *testing.T, db *gorm.DB) *domain.User {
    user := &domain.User{
        Email:        "test@example.com",
        PasswordHash: "$2a$12$...", // bcrypt hash
    }
    require.NoError(t, db.Create(user).Error)
    return user
}
```

### Testing Permissions

```go
func TestPermissionEnforcement(t *testing.T) {
    suite := NewTestSuite(t)
    defer suite.Cleanup()
    suite.RunMigrations(t)
    
    // Create user, role, permission
    user := createTestUser(t, suite.DB)
    role := createTestRole(t, suite.DB)
    
    // Enforce permission
    enforcer, _ := permission.NewEnforcer(suite.DB)
    allowed, _ := enforcer.Enforce(user.ID, "default", "invoices", "view")
    assert.True(t, allowed)
}
```

### Table-Driven Tests

```go
func TestAuthLogin(tt *testing.T) {
    suite := NewTestSuite(tt)
    defer suite.Cleanup()
    
    tests := []struct {
        name    string
        setup   func(t *testing.T)
        request *request.LoginRequest
        wantErr bool
    }{
        {"valid credentials", func(t *testing.T) {
            createTestUser(t, suite.DB)
        }, &request.LoginRequest{...}, false},
        // more cases
    }
    
    for _, tc := range tests {
        tt.Run(tc.name, func(t *testing.T) {
            suite.SetupTest(t)  // Clean DB
            tc.setup(t)
            // test logic
        })
    }
}
```

## ASSERTIONS

```go
require.NoError(t, err)       // Fatal, stops test
assert.Equal(t, expected, actual)  // Non-fatal, continues
assert.NotNil(t, user)
assert.Empty(t, list)
assert.Contains(t, "msg", "substring")
```

## TROUBLESHOOTING

| Issue | Solution |
|-------|----------|
| "rootless Docker not supported" | Start Docker Desktop |
| Timeout | Add `-timeout 10m` |
| Port conflicts | testcontainers uses random ports |
| Slow startup | First run pulls images, cache after |

## NOTES

- Containers created once per test function
- Random ports prevent conflicts
- Cleanup terminates containers
- No state persists between tests
- Real DB/Redis, not mocks