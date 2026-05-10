# Advanced Testing Guide

**Project:** Go API Base
**Last Updated:** 2026-05-08
**Target Audience:** Backend engineers, QA engineers, and dev leads responsible for test strategy and quality assurance.

This guide covers the complete testing approach for the Go API Base: unit testing with mocks, integration testing against real databases, performance and security testing, contract testing, and CI/CD pipeline integration. It assumes you have already completed the [First Setup](FIRST_SETUP.md) and are familiar with the project structure.

---

## Table of Contents

1. [Testing Philosophy and Strategy](#1-testing-philosophy-and-strategy)
2. [Test Environment Setup](#2-test-environment-setup)
3. [Unit Testing](#3-unit-testing)
4. [Integration Testing](#4-integration-testing)
5. [Performance and Load Testing](#5-performance-and-load-testing)
6. [Security Testing](#6-security-testing)
7. [API Contract Testing](#7-api-contract-testing)
8. [Test Data Management](#8-test-data-management)
9. [Coverage Requirements](#9-coverage-requirements)
10. [Continuous Integration Pipeline](#10-continuous-integration-pipeline)
11. [Troubleshooting and FAQ](#11-troubleshooting-and-faq)

---

## 1. Testing Philosophy and Strategy

### 1.1 The Testing Pyramid

The project follows a pragmatic testing pyramid with three layers:

```
       /\
      /  \        E2E / Smoke (select scenarios)
     /    \
    /------\      Integration (real DB + Redis, httptest)
   /        \
  /----------\    Unit (mocked dependencies, testify/mock)
 /            \
```

**Unit tests** verify individual functions and service methods in isolation. All external dependencies (repositories, external APIs, caches) are replaced with mocks. These tests run fast, require no infrastructure, and provide quick feedback during development.

**Integration tests** verify that the full stack works together: HTTP handlers, middleware, services, repositories, and real PostgreSQL/Redis connections. These tests prove that your wiring, SQL queries, permission checks, and response serialization are all correct. They require local PostgreSQL and Redis instances but add Docker.

**E2E / smoke tests** cover the most critical user journeys end to end. These are typically run less frequently, such as before deployment. The project includes selected E2E flows (for example, the webhook lifecycle test in `tests/integration/webhook_e2e_test.go`) that exercise the full event publication through signed HTTP delivery pipeline.

### 1.2 When to Test What

| Scenario | Layer | Reason |
|----------|-------|--------|
| Business logic: password validation, slug generation, permissions math | Unit | Deterministic, fast, no I/O needed |
| Repository methods: complex queries, soft delete scoping | Integration | Need real SQL execution to verify |
| HTTP handler: status codes, response envelopes, validation errors | Integration | Middleware chain matters, real serialization |
| Email provider integration: SMTP/SendGrid/SES specific behavior | Unit + Integration | Unit for logic, integration for real protocol |
| Multi-service coordination: webhook dispatch, job queue processing | Integration | Multiple components interacting |
| Permission matrix: admin vs user vs anonymous access | Integration | Real Casbin enforcer with DB-backed policies |

### 1.3 Anti-Patterns to Avoid

- **Mocking the wrong layer.** Mock at the repository interface boundary, not inside services. If a service calls another service, mock the callee's interface.
- **Testing framework internals.** Do not test GORM, Echo, Casbin, or other framework code. Test your integration with them.
- **Flaky tests from shared state.** Every integration test runs with a clean database (all tables truncated, Redis flushed). If a test relies on data from another test, it is broken.
- **Testing the mock instead of the real thing.** Verify that mocks are used correctly with `mock.AssertExpectations(t)`, but do not over-specify how a mock was called when the outcome is what matters.
- **Skipping race detection.** Always use `-race` for unit tests. The project has goroutine-based components (EventBus, WebhookWorker, async audit logging) where data races are easy to introduce.

---

## 2. Test Environment Setup

### 2.1 Unit Test Environment

Unit tests require no external services. Everything is mocked.

```bash
# Run all unit tests
make test

# Equivalent manual command
go test -v -race -coverprofile=coverage.txt ./tests/unit/...

# Run a specific unit test
go test -v -race ./tests/unit/ -run TestUserService_CreateUser
```

The `-race` flag is mandatory. It enables Go's built-in race detector, which is critical for catching goroutine-level concurrency bugs in async components like the EventBus, WebhookWorker, and async audit logging.

The `-coverprofile=coverage.txt` flag produces a coverage report file you can inspect:

```bash
go tool cover -html=coverage.txt  # Open coverage report in browser
go tool cover -func=coverage.txt  # Show per-function coverage percentages
```

### 2.2 Integration Test Environment

Integration tests need real PostgreSQL and Redis. The project uses two approaches, depending on how you run them:

#### Option A: Local Services (recommended)

Start PostgreSQL and Redis locally (or via Docker Compose), then run tests directly:

```bash
# Start only the databases
docker-compose up -d postgres redis

# Run integration tests
make test-integration

# Equivalent manual command
go test -v -tags=integration ./tests/integration/... -timeout 5m
```

The `-tags=integration` build tag ensures integration tests are never picked up by `go test ./...` without explicitly requesting them. This is enforced by the `//go:build integration` directive at the top of every integration test file.

The test suite automatically loads credentials from your `.env` file (see `testsuite.go:loadEnvFile()`), so no manual environment configuration is needed for local development.

#### Option B: Serial Execution for Stability

When running the full integration suite, serial execution (`-p 1`) avoids contention and flakiness:

```bash
go test -tags=integration -p 1 -count=1 -timeout 10m ./tests/integration/...
```

| Flag | Purpose |
|------|---------|
| `-p 1` | Single package at a time (no parallel packages) |
| `-count=1` | Disable test caching (always re-run) |
| `-tags=integration` | Required build tag |
| `-timeout 10m` | Generous timeout for full suite |

#### Configuration Overrides

The test suite resolves database and Redis configuration in this priority order:

1. `TEST_DATABASE_URL` environment variable (for CI/test isolation)
2. `DATABASE_URL` environment variable
3. Individual `DATABASE_HOST`, `DATABASE_PORT`, etc. variables
4. Values from `../.env` (auto-loaded by `loadEnvFile()`)
5. Defaults: `localhost:5432` / `localhost:6379`

For CI environments, set `TEST_DATABASE_URL` to a dedicated test database to prevent accidental data loss from the truncation step.

### 2.3 Docker-Based Testing

The `Dockerfile` uses a multi-stage build suitable for running tests in containers:

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go test -v -race ./tests/unit/...
```

Running tests inside Docker ensures a consistent Go version and eliminates host environment differences. For integration tests in Docker, use `docker-compose` to spin up the full stack:

```yaml
# Example docker-compose.test.yml
services:
  test:
    build: .
    command: go test -v -tags=integration ./tests/integration/... -timeout 10m
    environment:
      - DATABASE_URL=postgres://test:test@postgres:5432/testdb?sslmode=disable
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis
```

---

## 3. Unit Testing

### 3.1 Mocking Pattern

The project uses `github.com/stretchr/testify/mock` for all unit test mocks. Every repository interface gets a corresponding mock struct in the test file that uses it.

The standard mock pattern:

```go
// Define mock struct embedding mock.Mock
type MockUserRepositoryForUserService struct {
    mock.Mock
}

// Implement each method using args.Get / args.Error
func (m *MockUserRepositoryForUserService) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*domain.User), args.Error(1)
}
```

This pattern appears throughout the codebase. See `tests/unit/user_service_test.go`, `tests/unit/auth_service_test.go`, and `tests/unit/email_service_test.go` for worked examples with different repository interfaces.

**Key conventions:**
- Mock type names include the context: `MockUserRepositoryForUserService`, not just `MockUserRepository`
- `args.Get(0)` is the first return value, `args.Error(1)` is the second
- Nil-check on `args.Get(0)` before type assertion to avoid panics
- Compile-time interface conformance check: `var _ repository.NewsRepository = (*MockNewsRepository)(nil)`

### 3.2 Compile-Time Interface Checks

All mock implementations should include a compile-time check that they satisfy the interface:

```go
var _ repository.NewsRepository = (*MockNewsRepository)(nil)
```

This line produces zero runtime overhead. If the interface changes and the mock falls out of sync, you get a clear compile error instead of a confusing runtime panic. See `tests/unit/service/news_test.go` for an example.

### 3.3 Table-Driven Tests

Table-driven tests are the standard approach for testing multiple input/output combinations. This pattern is used across both unit and integration tests.

**Simple table-driven test (unit)** from `tests/unit/email_provider_test.go`:

```go
func TestEmailMessage_Validation(t *testing.T) {
    tests := []struct {
        name        string
        email       *service.EmailMessage
        expectError bool
    }{
        {
            name: "valid email with HTML content",
            email: &service.EmailMessage{
                To: "test@example.com", Subject: "Test", HTMLContent: "<p>Hello</p>",
            },
            expectError: false,
        },
        {
            name: "valid email with text content",
            email: &service.EmailMessage{
                To: "test@example.com", Subject: "Test", TextContent: "Hello",
            },
            expectError: false,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.email)
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

**Table-driven test with setup (integration)** from `tests/integration/auth_handler_test.go`:

```go
func TestAuthHandler_Register(t *testing.T) {
    suite := NewTestSuite(t)
    defer suite.Cleanup()
    suite.RunMigrations(t)
    suite.SetupTest(t)
    server := helpers.NewTestServer(t, suite)

    t.Run("happy path - creates user and returns 201", func(t *testing.T) {
        // test body
    })
    t.Run("validation - missing email returns 400", func(t *testing.T) {
        // test body
    })
    t.Run("validation - missing password returns 400", func(t *testing.T) {
        // test body
    })
}
```

### 3.4 Mock Expectation Setup

Set up expectations before calling the method under test, then verify:

```go
func TestUserService_CreateUser(t *testing.T) {
    ctx := context.Background()
    mockRepo := new(MockUserRepositoryForUserService)
    mockRoleRepo := new(MockUserRoleRepositoryForUserService)

    userService := service.NewUserService(mockRepo, mockRoleRepo)

    // Arrange: set up expectations
    mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

    // Act
    user, err := userService.Create(ctx, "test@example.com", "Password123!")

    // Assert
    require.NoError(t, err)
    assert.NotNil(t, user)
    mockRepo.AssertExpectations(t) // Verify mock was called as expected
}
```

**Important:** Use `mock.AnythingOfType("*domain.User")` rather than `mock.Anything` when you want to validate the type but not the exact value. This catches type mismatches while keeping tests focused on behavior.

### 3.5 Testing Service Methods

Service tests focus on business logic, not data access. The pattern:

1. Create mocks for all repository dependencies
2. Instantiate the service with those mocks
3. Set up mock expectations (return values, errors)
4. Call the service method
5. Assert on the return value
6. Verify mock expectations were met

**Testing error paths:**

```go
t.Run("returns error when repository fails", func(t *testing.T) {
    mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
        Return(errors.New("db error"))

    _, err := userService.Create(ctx, "test@example.com", "Password123!")
    assert.Error(t, err)
})
```

**Testing permission logic in services:**

```go
t.Run("rejects when user lacks permission", func(t *testing.T) {
    // Service methods check permissions via enforcer before proceeding
    // Arrange: user has no permissions
    // Act: call protected method
    // Assert: returns ErrForbidden or similar
})
```

### 3.6 Testing Providers and External Integrations

The email provider pattern demonstrates how to test pluggable integrations:

```go
type MockEmailProvider struct {
    SendFunc  func(ctx context.Context, email *service.EmailMessage) (string, error)
    NameFunc  func() string
    messageID string
    sendError error
}

func (m *MockEmailProvider) Send(ctx context.Context, email *service.EmailMessage) (string, error) {
    if m.SendFunc != nil {
        return m.SendFunc(ctx, email)
    }
    return m.messageID, m.sendError
}
```

See `tests/unit/email_provider_test.go` for the full implementation, including tests for SMTP, SendGrid, and SES specific behavior.

---

## 4. Integration Testing

### 4.1 TestSuite Architecture

All integration tests share a single `TestSuite` instance. This avoids the cost of creating new database connections and running migrations for every test.

**Initialization flow:**

1. **First `NewTestSuite(t)` call:** Loads `.env`, connects to PostgreSQL and Redis, drops all existing tables, recreates the full schema from embedded migrations, truncates all tables
2. **Subsequent calls:** Return the existing suite (mutex-protected double-check) and truncate tables for isolation

```go
func TestMyFeature(t *testing.T) {
    suite := NewTestSuite(t)    // Creates or reuses shared suite
    defer suite.Cleanup()       // No-op for shared suite (cleanup in TestMain)
    suite.RunMigrations(t)      // Idempotent (skips if already run)
    suite.SetupTest(t)          // Truncate tables + flush Redis

    // ... test code
}
```

The global `testSuite` variable is protected by `sync.Mutex` to ensure thread-safe initialization. The mutex is released before any `require.NoError` call to avoid deadlocks (since `t.FailNow` calls `runtime.Goexit` which can deadlock under a held mutex).

### 4.2 Database Schema Management

Migrations are embedded directly in `testsuite.go` rather than read from files. This design choice ensures:
- Tests always run against a known, versioned schema
- No dependency on the `migrations/` directory at test time
- Schema changes are explicit and reviewed as part of the test suite

The migration functions create the entire schema from scratch: users, roles, permissions, pivot tables, refresh tokens, invoices, news, audit logs, organizations, media, email, API keys, notifications, webhooks, password reset tokens, and the Casbin rule table.

If you add a new domain entity with its own migration, add the corresponding DDL to `runMigrations()` in `testsuite.go`.

### 4.3 Test Isolation

Each test starts with absolutely clean state:

```go
func (s *TestSuite) SetupTest(t *testing.T) {
    // 1. Disable audit_logs immutability trigger
    s.DB.Exec("ALTER TABLE audit_logs DISABLE TRIGGER audit_logs_immutable")

    // 2. Truncate all tables in FK-safe order (children first)
    tables := []string{
        "notification_preferences", "notifications",
        "email_bounces", "email_queue", "email_templates",
        "password_reset_tokens",
        "media_downloads", "media_conversions", "media",
        "api_keys", "audit_logs",
        "user_permissions", "role_permissions", "user_roles",
        "refresh_tokens", "invoices", "news",
        "organization_members", "organizations",
        "webhook_deliveries", "webhooks",
        "casbin_rule", "permissions", "roles", "users",
    }
    for _, table := range tables {
        s.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
    }

    // 3. Re-enable audit trigger
    s.DB.Exec("ALTER TABLE audit_logs ENABLE TRIGGER audit_logs_immutable")

    // 4. Reset serial sequences
    s.DB.Exec("ALTER SEQUENCE casbin_rule_id_seq RESTART WITH 1")

    // 5. Flush Redis database
    s.RedisClient.FlushDB(ctx)
}
```

The audit logs table has an immutability trigger that blocks DELETE and TRUNCATE operations. The trigger is temporarily disabled during cleanup and re-enabled afterward.

### 4.4 HTTP Test Server

Integration HTTP tests use a fully-wired server mirroring the production `runServer()` dependency graph.

**Server creation** from `tests/integration/helpers/handler_test_helpers.go`:

```go
func NewTestServer(t *testing.T, suite SuiteProvider) *Server {
    cfg := DefaultTestConfig()

    // Initialize logger
    log, _ := logger.NewLogger(logger.Config{
        Level: "debug", Format: "json", Outputs: "stdout",
    })

    // Initialize cache driver with Redis
    cacheDriver, _ := appcache.NewDriver(appcache.Config{
        Driver: "redis", DefaultTTL: 300, PermissionTTL: 300,
    }, suite.GetRedisClient())

    // Initialize Casbin enforcer with permission cache
    enf, _ := permission.NewEnforcer(suite.GetDB())
    if cacheDriver != nil {
        enf.SetCache(permission.NewCache(cacheDriver, 300*time.Second))
    }

    // Wire everything into the server
    server := apphttp.NewServer(cfg, suite.GetDB(), suite.GetRedisClient(), cacheDriver, log)
    server.SetEnforcer(enf)
    // ... additional wiring

    return &Server{server}
}
```

This gives you the full middleware chain: RequestID, CORS, RateLimit, JWT, StructuredLogging, and all registered routes. Your tests exercise real handler code, not stubs.

### 4.5 Making HTTP Requests in Tests

The `helpers.MakeRequest` function handles request construction, JSON marshaling, and response capture:

```go
// Authenticated GET
rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users", nil, adminToken)

// POST with JSON body
payload := map[string]interface{}{"name": "Test Role", "description": "A test role"}
rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", payload, adminToken)

// Parse response envelope
env := helpers.ParseEnvelope(t, rec)
assert.Nil(t, env.Error, "response should have no error")
dataMap := env.Data.(map[string]interface{})
```

The response envelope structure matches the production format:

```json
{
    "data": { ... },
    "error": { "code": "VALIDATION_ERROR", "message": "..." },
    "meta": { "request_id": "uuid", "timestamp": "2026-05-08T00:00:00Z" }
}
```

### 4.6 Creating Test Users and Roles

Helper functions in `tests/integration/helpers/fixtures.go` provide reusable test data:

```go
const (
    TestPassword     = "TestPassword123!"
    TestEmailDomain  = "@test.example.com"
    TestAdminRoleName = "admin"
    TestUserRoleName  = "user"
)

// Generate unique email for each test
func UniqueEmail(t *testing.T) string {
    return fmt.Sprintf("test-%d-%s%s", time.Now().UnixNano(), randomString(8), TestEmailDomain)
}
```

For creating authenticated users in tests:

```go
// Create an admin user with full permissions
adminToken, adminID := helpers.CreateAdminUser(t, suite, server.Enforcer())

// Create a regular user with basic permissions
email := helpers.UniqueEmail(t)
userToken := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)
```

### 4.7 Webhook E2E Testing

The webhook E2E test (`tests/integration/webhook_e2e_test.go`) demonstrates a complete multi-component integration test pattern:

```go
func TestWebhookE2E_FullLifecycle(t *testing.T) {
    suite := SetupIntegrationTest(t)
    defer suite.TeardownTest(t)
    ctx := context.Background()

    // 1. Create a real HTTP server to receive webhook calls
    var receivedReq *http.Request
    var receivedBody []byte
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        receivedReq = r
        body, _ := io.ReadAll(r.Body)
        receivedBody = body
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    // 2. Set up webhook service, EventBus, and worker
    webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)
    eventBus := domain.NewEventBus(10)
    webhookSvc.SubscribeToEventBus(eventBus)
    eventBus.Start(ctx)
    worker := newWebhookWorkerWithDeps(suite.DB, suite.RedisClient)
    worker.Start(ctx)

    // 3. Create webhook and publish an event
    wh := createTestWebhookForEvents(t, suite.DB, serverURL, []string{"user.created"}, true)
    eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})

    // 4. Assert the webhook was delivered with correct HMAC signature
    assert.Equal(t, "user.created", receivedReq.Header.Get("X-Webhook-Event"))
    // ... verify HMAC signature, payload, delivery status
}
```

This test verifies the entire pipeline: event publication, EventBus routing, webhook dispatch, worker queue processing, HTTP delivery with HMAC-SHA256 signing, and delivery status updates.

### 4.8 Permission Matrix Testing

The permission matrix test (`tests/integration/permission_matrix_test.go`) systematically verifies access control:

```go
func TestPermissionMatrix(t *testing.T) {
    suite := NewTestSuite(t)
    server := helpers.NewTestServer(t, suite)

    t.Run("admin user has full access to all endpoints", func(t *testing.T) {
        adminToken, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())
        // Test all endpoints with admin token
    })

    t.Run("unauthenticated requests are rejected", func(t *testing.T) {
        // Test protected endpoints without token
    })

    t.Run("regular user has limited access", func(t *testing.T) {
        // Test with user-scoped permissions only
    })
}
```

### 4.9 Soft Delete Verification

Soft delete testing (`tests/integration/soft_delete_test.go`) verifies that GORM's soft delete mechanism works correctly:

```go
t.Run("delete sets deleted_at not removes row", func(t *testing.T) {
    // DELETE via API
    deleteRec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)
    assert.Equal(t, http.StatusOK, deleteRec.Code)

    // Verify row still exists in DB with deleted_at set
    var count int64
    suite.DB.Raw("SELECT COUNT(*) FROM roles WHERE id = ?", roleID).Scan(&count)
    assert.Equal(t, int64(1), count)

    var deletedCount int64
    suite.DB.Raw("SELECT COUNT(*) FROM roles WHERE id = ? AND deleted_at IS NOT NULL", roleID).Scan(&deletedCount)
    assert.Equal(t, int64(1), deletedCount)
})

t.Run("soft deleted resource returns 404 on GET", func(t *testing.T) {
    // After soft delete, GET returns 404
    rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/roles/"+roleID, nil, token)
    assert.Equal(t, http.StatusNotFound, rec.Code)
})
```

This pattern proves that:
- The row is not physically removed from the database
- `deleted_at` is set to a non-null timestamp
- GORM's default scope correctly excludes soft-deleted rows from normal queries
- The API returns appropriate 404 responses for soft-deleted resources

### 4.10 Error Scenario Coverage

The error scenario tests (`tests/integration/error_scenario_test.go`) cover the standard HTTP error states across endpoints:

```go
t.Run("404 for non-existent resource", func(t *testing.T) { ... })
t.Run("400 for invalid request body", func(t *testing.T) { ... })
t.Run("401 for missing auth token", func(t *testing.T) { ... })
t.Run("403 for insufficient permissions", func(t *testing.T) { ... })
t.Run("409 for duplicate resource", func(t *testing.T) { ... })
t.Run("422 for business rule violation", func(t *testing.T) { ... })
```

Every new handler should have corresponding error scenario tests for at least the 400, 401, 403, and 404 states.

---

## 5. Performance and Load Testing

### 5.1 Benchmark Tests in Go

Go's built-in benchmarking framework is useful for measuring specific function performance:

```go
// Place in tests/unit/ alongside regular tests
func BenchmarkUserService_CreateUser(b *testing.B) {
    mockRepo := new(MockUserRepositoryForUserService)
    svc := service.NewUserService(mockRepo, nil)
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        svc.Create(ctx, "test@example.com", "Password123!")
    }
}
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./tests/unit/...
```

The `-benchmem` flag reports memory allocations per operation, which is essential for identifying allocation-heavy code paths.

### 5.2 Load Testing Tools

For HTTP-level load testing, use external tools:

**Vegeta (recommended for Go projects):**

```bash
# Install
go install github.com/tsenart/vegeta@latest

# Attack with target rate
echo "POST http://localhost:8080/api/v1/auth/login" | \
  vegeta attack -rate=100/s -duration=30s -body=login.json | \
  vegeta report

# Output: latencies (min, mean, 50th, 95th, 99th, max), success rate, throughput
```

**hey (simple alternative):**

```bash
go install github.com/rakyll/hey@latest

hey -n 10000 -c 100 -m POST \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"TestPassword123!"}' \
  http://localhost:8080/api/v1/auth/login
```

### 5.3 Key Metrics to Track

| Metric | Target | Why |
|--------|--------|-----|
| P95 latency | < 200ms | 95% of requests should feel fast to users |
| P99 latency | < 500ms | Outliers should not degrade experience |
| Error rate | < 0.1% | Production reliability expectation |
| Throughput | Depends on resources | Establish baseline, alert on degradation |
| Database connection utilization | < 80% | Headroom for traffic spikes |

### 5.4 Profiling

Go's built-in profiler helps identify bottlenecks:

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=. ./tests/unit/...
go tool pprof cpu.prof

# Memory profile
go test -memprofile=mem.prof -bench=. ./tests/unit/...
go tool pprof mem.prof

# HTTP server profiling (in production-like setup)
# Add to main.go: import _ "net/http/pprof"
# Then: go tool pprof http://localhost:8080/debug/pprof/profile
```

### 5.5 Database Query Performance

For integration test suites, you can enable slow query logging to catch N+1 problems:

```go
// In testsuite.go - enable query logging for performance debugging
db, _ := gorm.Open(postgres.Open(connStr), &gorm.Config{
    Logger: logger.Default.LogMode(logger.Info), // Logs all SQL
    // Or use custom logger with slow query threshold:
    // SlowThreshold: 200 * time.Millisecond,
})
```

---

## 6. Security Testing

### 6.1 Static Analysis (gosec)

The project's linting configuration includes `gosec` (Go Security Checker). It catches common security issues at compile time:

```bash
make lint

# gosec checks for:
# - Hardcoded credentials (G101)
# - SQL injection via string concatenation (G201)
# - Weak cryptographic primitives (G401, G402, G403)
# - Unsafe file operations (G304, G305)
# - And more
```

The `.golangci.yml` configuration enables gosec alongside other linters:

```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gosec       # Security-focused linter
    - gocritic    # Code quality linter
```

Run gosec standalone for a focused security report:

```bash
go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
gosec ./...
```

### 6.2 Authentication and Authorization Test Patterns

These are tested extensively in the integration suite. Key scenarios that must pass:

**Token validation:**
- Expired tokens return 401
- Malformed tokens return 401
- Tokens with wrong signing key return 401
- Refresh token rotation invalidates previous tokens (token family revocation)

**Permission enforcement:**
- Non-admin users cannot access admin-only endpoints (403)
- Users cannot access resources belonging to other users
- API keys with restricted scopes cannot access endpoints outside their scope
- X-Organization-ID based role checks correctly scope permissions

**Test location:** `tests/integration/auth_handler_test.go`, `tests/integration/permission_test.go`, `tests/integration/permission_matrix_test.go`

### 6.3 Input Validation Tests

Every handler must have tests for:

- **Missing required fields:** 400 with field-level error messages
- **Invalid formats:** malformed emails, UUIDs, URLs
- **Boundary values:** empty strings, maximum length, negative numbers
- **Type mismatches:** string where number expected, array where object expected
- **SQL injection attempts:** special characters in string fields
- **XSS payloads:** script tags in user-provided content

Example from `tests/integration/auth_handler_test.go`:

```go
t.Run("validation - missing email returns 400", func(t *testing.T) { ... })
t.Run("validation - missing password returns 400", func(t *testing.T) { ... })
t.Run("validation - invalid email format returns 400", func(t *testing.T) { ... })
t.Run("validation - weak password returns 400", func(t *testing.T) { ... })
```

### 6.4 Dependency Vulnerability Scanning

```bash
# Install govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# Scan for known vulnerabilities in dependencies
govulncheck ./...
```

Run this in CI and before every release. The tool checks against Go's vulnerability database and reports CVEs in direct and transitive dependencies.

### 6.5 Secrets Detection

Before committing, scan for accidentally committed secrets:

```bash
# Using gitleaks
brew install gitleaks
gitleaks detect --source . --verbose

# Using truffleHog
brew install trufflehog
trufflehog filesystem .
```

The project's `.golangci.yml` gosec integration catches hardcoded credentials in Go source. These external tools add protection for configuration files, environment files, and other non-Go sources.

### 6.6 OWASP ZAP Scanning

For running API security scans:

```bash
# Start the API locally
make serve

# Run ZAP baseline scan
docker run -t owasp/zap2docker-stable zap-baseline.py \
  -t http://host.docker.internal:8080/swagger/doc.json \
  -r zap_report.html
```

This identifies common vulnerabilities: missing security headers, information disclosure, CSRF, insecure cookie settings, and more.

---

## 7. API Contract Testing

### 7.1 OpenAPI / Swagger Validation

The project auto-generates Swagger documentation via `make swagger`. Tests validate that the Swagger spec is correct:

**Swagger endpoint test** from `tests/integration/swagger_test.go`:

```go
func TestSwaggerDocs(t *testing.T) {
    // Verify swagger.json is served
    // Verify swagger UI renders
    // Validate OpenAPI spec structure
}
```

### 7.2 Response Envelope Contract

All API responses must follow the envelope contract. Tests verify this:

```go
func TestResponseEnvelope(t *testing.T) {
    rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)

    env := helpers.ParseEnvelope(t, rec)

    // All successful responses have these fields
    assert.NotNil(t, env.Data)
    assert.Nil(t, env.Error)
    assert.NotNil(t, env.Meta)
    assert.NotEmpty(t, env.Meta.RequestID)
    assert.NotZero(t, env.Meta.Timestamp)
}
```

### 7.3 Consumer-Driven Contract Testing

For services that consume this API, consider adding Pact contract tests:

```bash
# Install Pact Go
go get github.com/pact-foundation/pact-go/v2

# Define consumer expectations
# Verify against provider
```

This approach is valuable if the API has known external consumers who depend on specific response shapes.

### 7.4 Schema Validation

For critical endpoints, add explicit schema validation in integration tests:

```go
func TestUserResponseSchema(t *testing.T) {
    rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
    env := helpers.ParseEnvelope(t, rec)
    dataMap := env.Data.(map[string]interface{})

    // Required fields
    requiredFields := []string{"id", "email", "created_at", "updated_at"}
    for _, field := range requiredFields {
        assert.Contains(t, dataMap, field, "response must contain '%s'", field)
    }

    // No sensitive fields
    assert.NotContains(t, dataMap, "password_hash")
    assert.NotContains(t, dataMap, "deleted_at")
}
```

---

## 8. Test Data Management

### 8.1 Unique Test Data Generation

Every test that creates database records should use unique identifiers to prevent collisions:

```go
// Unique email (timestamp + random suffix)
func UniqueEmail(t *testing.T) string {
    return fmt.Sprintf("test-%d-%s%s",
        time.Now().UnixNano(), randomString(8), TestEmailDomain)
}

// Unique name for roles, organizations, templates
func UniqueName(prefix string) string {
    return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), randomString(6))
}
```

### 8.2 Test Fixtures

Shared fixtures are defined in `tests/integration/helpers/fixtures.go`:

```go
var CommonValidUserPayload = map[string]interface{}{
    "email":    "test@example.com",
    "password": TestPassword,
}

var CommonInvalidPayloads = map[string]struct {
    Payload       map[string]interface{}
    ExpectedField string
}{
    "missing_email":    {Payload: map[string]interface{}{"password": TestPassword}, ExpectedField: "email"},
    "missing_password": {Payload: map[string]interface{}{"email": "test@example.com"}, ExpectedField: "password"},
    "invalid_email":    {Payload: map[string]interface{}{"email": "not-an-email", "password": TestPassword}, ExpectedField: "email"},
    "weak_password":    {Payload: map[string]interface{}{"email": "test@example.com", "password": "123"}, ExpectedField: "password"},
}
```

### 8.3 Seeding in Integration Tests

Integration tests that need roles, permissions, or Casbin policies should seed them explicitly within the test, rather than relying on a pre-seeded database:

```go
func TestWithRoles(t *testing.T) {
    suite := NewTestSuite(t)
    defer suite.Cleanup()
    suite.RunMigrations(t)
    suite.SetupTest(t)

    // Seed roles and permissions needed for this test
    seedRolesAndPermissions(t, suite.DB)
    syncCasbinPolicies(t, suite.DB)
    // ... test logic
}
```

This keeps tests self-contained and independent of global database state.

### 8.4 Cleaning Up

Integration test cleanup happens automatically via `SetupTest(t)`, which is called at the start of every test (not the end). This "clean before, not after" approach means:
- A failed test leaves its data intact for debugging
- The next test still starts with a clean state
- No cleanup code runs in `defer` blocks (simpler reasoning)

The `TestMain` in `main_test.go` handles final cleanup after all tests finish: closing Redis and database connections.

---

## 9. Coverage Requirements

### 9.1 Coverage Targets

From the project constitution (`.specify/memory/constitution.md`):

| Layer | Target | Measurement |
|-------|--------|-------------|
| Unit tests | 80% | `go test -coverprofile=coverage.txt ./tests/unit/...` |
| Integration tests | 80% | Combined with unit tests for overall coverage |
| Critical paths | 100% | Auth, permission checks, data mutation endpoints |

### 9.2 Generating Coverage Reports

```bash
# Unit test coverage
make test
go tool cover -func=coverage.txt

# Integration test coverage (requires build tag)
go test -v -tags=integration -coverprofile=integration_coverage.txt \
  -coverpkg=./internal/... ./tests/integration/... -timeout 5m

# Merge unit and integration coverage
go run github.com/wadey/gocovmerge@latest \
  coverage.txt integration_coverage.txt > merged_coverage.txt
go tool cover -html=merged_coverage.txt -o coverage.html
```

The `-coverpkg=./internal/...` flag for integration tests ensures coverage is measured for all `internal/` packages, not just the packages imported by test files. Without this, only packages imported by tests appear in the report.

### 9.3 Coverage by Package

Review coverage per package to identify gaps:

```bash
go tool cover -func=coverage.txt | sort -t: -k2 -n
```

Example output:
```
internal/service/auth_service.go:45:  Login           85.7%
internal/service/auth_service.go:92:  Register        92.3%
internal/service/auth_service.go:145: RefreshToken    78.2%  <-- needs attention
```

Focus additional tests on:
- Error handling branches (especially in repository calls)
- Edge cases (empty inputs, boundary values, nil pointers)
- Permission-checking code paths
- Async code paths (goroutines, channel operations)

### 9.4 Uncovered Code Patterns

When you find uncovered code, ask:
1. Is it dead code? Remove it.
2. Is it error handling? Add a test that triggers the error.
3. Is it an edge case? Add a table-driven test entry.
4. Is it infrastructure code (DB connection, server setup)? Consider integration tests.

---

## 10. Continuous Integration Pipeline

> **Note:** The project does not currently include a GitHub Actions or CI configuration file. This section describes the recommended CI/CD testing pipeline that should be implemented.

### 10.1 Recommended Pipeline Stages

```
Push → Lint → Unit Tests → Build → Integration Tests → Security Scan → Deploy
```

Each stage should run in parallel where possible and halt the pipeline on failure.

### 10.2 Lint Stage (fast, runs first)

```yaml
# .github/workflows/lint.yml
- name: Lint
  run: |
    go install github.com/golangci-lint/golangci-lint@latest
    golangci-lint run --timeout 5m ./...
```

### 10.3 Unit Test Stage (parallel)

```yaml
# .github/workflows/test.yml
- name: Unit Tests
  run: go test -v -race -coverprofile=coverage.txt ./tests/unit/...

- name: Coverage Check
  run: |
    coverage=$(go tool cover -func=coverage.txt | grep total | awk '{print $3}' | sed 's/%//')
    if (( $(echo "$coverage < 80" | bc -l) )); then
      echo "Coverage $coverage% is below 80% threshold"
      exit 1
    fi
```

### 10.4 Integration Test Stage

Integration tests need real PostgreSQL and Redis. Use GitHub Actions service containers:

```yaml
# .github/workflows/integration.yml
services:
  postgres:
    image: postgres:15-alpine
    env:
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
      POSTGRES_DB: testdb
    ports:
      - 5432:5432
    options: >-
      --health-cmd pg_isready
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5

  redis:
    image: redis:7-alpine
    ports:
      - 6379:6379
    options: >-
      --health-cmd "redis-cli ping"
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5

steps:
  - name: Integration Tests
    env:
      TEST_DATABASE_URL: postgres://test:test@localhost:5432/testdb?sslmode=disable
      REDIS_URL: redis://localhost:6379
    run: go test -v -tags=integration -p 1 -count=1 -timeout 10m ./tests/integration/...
```

Using `TEST_DATABASE_URL` ensures integration tests use the CI database, not your local one.

### 10.5 Security Scan Stage

```yaml
# .github/workflows/security.yml
- name: Dependency Vulnerability Check
  run: |
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./...

- name: Secrets Detection
  run: |
    brew install gitleaks || true
    gitleaks detect --source . --verbose
```

### 10.6 Docker Build and Test

The multi-stage Dockerfile supports building and testing in Docker:

```yaml
- name: Docker Build & Test
  run: |
    docker build --target builder -t go-api-test .
    docker run go-api-test go test -v -race ./tests/unit/...
```

### 10.7 Pre-Push Hook (local)

For local development, add a pre-push hook to run tests before pushing:

```bash
# .git/hooks/pre-push
#!/bin/sh
echo "Running pre-push checks..."
make lint || exit 1
make test || exit 1
echo "All checks passed!"
```

---

## 11. Troubleshooting and FAQ

### Why do my integration tests fail with "relation does not exist"?

The database schema is created by `runMigrations()` on the first `NewTestSuite()` call. If your test uses a table that is not in the embedded migrations, add the DDL to `testsuite.go:runMigrations()`.

### Why do I get "Failed to connect GORM to PostgreSQL after 10 retries"?

- Verify PostgreSQL is running: `pg_isready -h localhost -p 5432`
- Check your `.env` file has correct `DATABASE_USER` and `DATABASE_PASSWORD`
- If using Docker, confirm the service is healthy: `docker-compose ps`

### Why do integration tests seem flaky?

Run with `-p 1` for serial execution. Parallel test execution can cause contention on shared database sequences and Redis keys. Also, verify `SetupTest(t)` is being called at the start of each test to get clean state.

### How do I debug a failing integration test?

Since `SetupTest` cleans before each test (not after), a failed test leaves its data in the database. Connect to the test database directly:

```bash
psql -h localhost -U postgres -d go_api_base
# Query the tables that were modified by the failing test
SELECT * FROM users;
SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 10;
```

### Why does the linter fail after I add a new interface method?

Update the corresponding mock implementation in the test file. Use the compile-time check pattern (`var _ Interface = (*Mock)(nil)`) to catch these mismatches early.

### Why are my new tests not being discovered?

- Unit tests: Must be in `tests/unit/` or subdirectories, file names ending in `_test.go`
- Integration tests: Must have `//go:build integration` at the top and be in `tests/integration/`
- Test functions: Must start with `Test` and take `*testing.T`

### How do I test async operations?

- **Unit tests:** Mock the channel or goroutine launch point. Use `sync.WaitGroup` or a buffered channel to synchronize with the async work in the mock.
- **Integration tests:** For operations like async audit logging, use the sync variant (`LogActionSync` instead of `LogAction`). For webhook dispatch, use the EventBus and Worker patterns which can be started and stopped deterministically in tests.
- **Never use `time.Sleep`** for synchronization in tests. It makes tests slow and flaky.

