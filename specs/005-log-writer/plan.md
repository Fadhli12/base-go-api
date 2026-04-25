# Implementation Plan: Structured Logging System

**Feature**: 005-log-writer - Structured Logging System  
**Branch**: 005-log-writer  
**Date**: 2026-04-25  
**Spec**: specs/005-log-writer/spec.md  

---

## Summary

Implement a structured logging system with a clean abstraction layer (`Logger` interface), request context propagation (request ID, user ID, org ID, trace ID), multiple output writers (stdout, file, syslog), and Echo middleware integration. The system will use Go 1.22's `log/slog` as the foundation while providing a flexible interface for testability and extensibility.

**Key Requirements:**
- Zero breaking changes to existing code
- Performance overhead < 2ms per request
- Support multiple output writers with log rotation
- Automatic context field extraction from JWT and headers
- Comprehensive test coverage (80%+)

---

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: 
- `log/slog` (standard library) - Structured logging foundation
- `github.com/labstack/echo/v4` - HTTP framework and middleware
- `gopkg.in/natefinch/lumberjack.v2` - File rotation
- `log/syslog` (standard library) - Syslog integration  
**Storage**: N/A (logs go to writers, not database)  
**Testing**: `go test`, `testify`, `testcontainers-go`  
**Target Platform**: Linux server (Docker)  
**Project Type**: Web service (REST API)  
**Performance Goals**: < 2ms middleware overhead, < 1ms per log call  
**Constraints**: 
- Zero breaking changes (backward compatible with slog)
- Goroutine-safe implementations
- Graceful shutdown with log flushing
- Environment variable configuration  
**Scale/Scope**: Production API serving 1000+ req/s

---

## Constitution Check

**Status**: ✅ PASSED

| Principle | Requirement | Compliance |
|-----------|-------------|------------|
| **I. Production-Ready Foundation** | Structured logging with correlation IDs | ✅ Structured slog with request_id, user_id, org_id, trace_id |
| **II. RBAC is Mandatory** | No hardcoded permissions | ✅ N/A - Logging doesn't bypass RBAC |
| **III. Soft Deletes** | Soft deletes on all entities | ✅ N/A - No database storage for logs |
| **IV. Stateless JWT** | JWT access tokens 15m, refresh 30d | ✅ N/A - Uses existing JWT middleware |
| **V. SQL Migrations** | Versioned migrations, no AutoMigrate | ✅ N/A - No database schema changes |
| **VI. Permission Consistency** | Sub-second cache invalidation | ✅ N/A - No caching in logging |
| **VII. Audit Logging** | Comprehensive audit trail | ✅ Complements existing audit system |

**Additional Checks:**
- ✅ Zero breaking changes to existing handlers
- ✅ Backward compatible with existing slog calls
- ✅ Test coverage target 80%+
- ✅ Performance targets < 2ms per request

---

## Project Structure

### Documentation (this feature)

```text
specs/005-log-writer/
├── plan.md              # This file - Complete implementation plan
├── research.md          # Phase 0 - Research findings on slog, Echo patterns, writers
├── data-model.md        # Phase 1 - Logger interface, Field struct, context keys, config
├── quickstart.md        # Phase 1 - Usage examples and configuration guide
├── contracts/           # Phase 1 - Interface contracts
│   └── logger.md        # Logger interface contract
└── spec.md              # Original feature specification
```

### Source Code (repository root)

```text
internal/
├── logger/                      # NEW: Structured logging package
│   ├── logger.go               # Logger interface definition
│   ├── field.go                # Field struct and constructors
│   ├── entry.go                # LogEntry struct
│   ├── context.go              # Context keys and operations
│   ├── config.go               # Configuration structures
│   ├── slog_logger.go          # slog-based implementation
│   ├── nop_logger.go           # No-op implementation
│   ├── writer.go               # Writer interface and MultiWriter
│   ├── writer_stdout.go        # Stdout writer
│   ├── writer_file.go          # File writer with lumberjack
│   ├── writer_syslog.go        # Syslog writer
│   ├── factory.go              # Logger factory from config
│   └── mock.go                 # Mock logger for tests
├── http/
│   └── middleware/
│       ├── structured_logging.go    # NEW: Structured logging middleware
│       └── logging.go               # EXISTING: Keep for backward compat (deprecated)
├── config/
│   └── config.go               # MODIFIED: Add logger config
└── ...

tests/
├── unit/
│   └── logger/                 # NEW: Logger unit tests
│       ├── logger_test.go
│       ├── field_test.go
│       ├── context_test.go
│       ├── slog_logger_test.go
│       └── writer_test.go
└── integration/
    └── logging_test.go         # NEW: Integration tests

```

**Structure Decision**: Single project with logger package following existing patterns (similar to `internal/permission/`, `internal/cache/`).

---

## Complexity Tracking

No constitution violations requiring justification.

---

## Implementation Phases

### Phase 0: Research ✅ COMPLETE

**Status**: Complete | **Duration**: 2 hours

**Deliverables:**
- [x] `research.md` - Research findings on Go 1.22+ slog best practices, Echo middleware patterns, log writer implementations, performance optimization, audit logging integration

**Key Decisions:**
| Decision | Choice | Rationale |
|----------|--------|-----------|
| Base logger | `log/slog` | Standard library, stable, performant |
| Abstraction | `Logger` interface | Testability, flexibility, swappable implementations |
| Context propagation | Go context | Enables propagation to repositories/services |
| Writers | Interface-based | Swappable, testable, follows Go patterns |
| Performance | Sync default, async optional | Balance safety and throughput |

### Phase 1: Design ✅ COMPLETE

**Status**: Complete | **Duration**: 3 hours

**Deliverables:**
- [x] `data-model.md` - Logger interface, Field struct, context keys, configuration structures
- [x] `contracts/logger.md` - Interface contracts, implementation requirements
- [x] `quickstart.md` - Usage examples, configuration guide, testing patterns

### Phase 2: Logger Interface & Implementation (Day 1 PM)

**Status**: Ready | **Duration**: 4 hours | **Dependencies**: Phase 1

#### 2.1 Create Package Structure

```bash
mkdir -p internal/logger
touch internal/logger/{logger,field,entry,context,config,slog_logger,nop_logger,writer,writer_stdout,writer_file,writer_syslog,factory,mock}.go
```

#### 2.2 Implement Core Interface (`internal/logger/logger.go`)

```go
// Tasks:
// [ ] Define Logger interface with Debug, Info, Warn, Error methods
// [ ] Define Level enum with String() method
// [ ] Define ParseLevel function
// [ ] Add package-level documentation
```

#### 2.3 Implement Field Support (`internal/logger/field.go`)

```go
// Tasks:
// [ ] Define Field struct
// [ ] Implement ToSlogAttr() method for slog compatibility
// [ ] Implement field constructors: String, Int, Int64, Float64, Bool, Duration, Time, Any, Err
// [ ] Add validation for key non-empty
```

#### 2.4 Implement Context Operations (`internal/logger/context.go`)

```go
// Tasks:
// [ ] Define contextKey type for collision avoidance
// [ ] Define context constants (LoggerContextKey, RequestIDContextKey, UserIDContextKey, OrgIDContextKey, TraceIDContextKey, StartTimeContextKey)
// [ ] Implement WithLogger / FromContext
// [ ] Implement WithRequestID / GetRequestID
// [ ] Implement WithUserID / GetUserID
// [ ] Implement WithOrgID / GetOrgID
// [ ] Implement WithTraceID / GetTraceID
// [ ] Implement ExtractContextFields
// [ ] Implement GenerateRequestID / GenerateTraceID (UUID)
```

#### 2.5 Implement slog-based Logger (`internal/logger/slog_logger.go`)

```go
// Tasks:
// [ ] Define SlogLogger struct with handler, level, fields
// [ ] Implement NewSlogLogger constructor
// [ ] Implement Debug method (level check)
// [ ] Implement Info method (level check)
// [ ] Implement Warn method (level check)
// [ ] Implement Error method (level check, stack trace)
// [ ] Implement WithFields method (immutable)
// [ ] Implement WithError method (immutable)
// [ ] Implement field factory methods
// [ ] Add level filtering via slog.Handler
// [ ] Add automatic context field extraction
```

#### 2.6 Implement Writers

**`internal/logger/writer.go`:**
- [ ] Define Writer interface (Write, Close)
- [ ] Define WriterFunc adapter
- [ ] Implement MultiWriter with goroutine safety

**`internal/logger/writer_stdout.go`:**
- [ ] Define StdoutWriter struct
- [ ] Implement NewStdoutWriter with JSON/text encoding
- [ ] Implement Write method
- [ ] Implement Close method

**`internal/logger/writer_file.go`:**
- [ ] Define FileWriter struct with lumberjack.Logger
- [ ] Implement NewFileWriter with directory creation
- [ ] Integrate lumberjack for rotation
- [ ] Implement Write method
- [ ] Implement Close method

**`internal/logger/writer_syslog.go`:**
- [ ] Define SyslogWriter struct
- [ ] Implement NewSyslogWriter with connection
- [ ] Map log levels to syslog severity
- [ ] Implement Write method
- [ ] Implement Close method

#### 2.7 Implement NopLogger (`internal/logger/nop_logger.go`)

```go
// Tasks:
// [ ] Define NopLogger struct
// [ ] Implement all Logger methods as no-ops
// [ ] Implement NewNopLogger constructor
```

#### 2.8 Unit Tests

**`internal/logger/logger_test.go`, `*_test.go`:**
- [ ] Test Field constructors
- [ ] Test ToSlogAttr conversion
- [ ] Test context operations (With*, Get*)
- [ ] Test SlogLogger all levels
- [ ] Test level filtering
- [ ] Test WithFields immutability
- [ ] Test WithError
- [ ] Test stdout writer
- [ ] Test file writer (with temp files)
- [ ] Test nop logger
- [ ] Benchmark logger methods (target: <1ms)
- [ ] Race detector: `go test -race ./internal/logger/...`

#### Phase 2 Success Criteria
- [ ] `go test ./internal/logger/... -v` passes
- [ ] All methods have >80% coverage
- [ ] Benchmarks show <1ms per log call
- [ ] Race detector passes
- [ ] `go build ./...` succeeds

---

### Phase 3: Context Propagation & Middleware (Day 2 AM)

**Status**: Ready | **Duration**: 4 hours | **Dependencies**: Phase 2

#### 3.1 Create Middleware (`internal/http/middleware/structured_logging.go`)

```go
// Tasks:
// [ ] Define LoggingMiddlewareConfig struct
// [ ] Implement DefaultLoggingMiddlewareConfig
// [ ] Implement DefaultLoggingSkipper (skip health checks, metrics)
// [ ] Implement StructuredLogging middleware
// [ ] Implement GetLogger helper for handlers
// [ ] Add request ID extraction from existing middleware
// [ ] Add user ID extraction from JWT claims
// [ ] Add org ID extraction from X-Organization-ID header
// [ ] Add trace ID extraction from X-Trace-ID header
// [ ] Generate IDs if not present
// [ ] Log request start (DEBUG level)
// [ ] Log request complete (INFO level)
// [ ] Calculate and log duration
// [ ] Log errors with stack traces (ERROR level)
// [ ] Inject logger into Echo context
```

#### 3.2 Integration with Existing Middleware

```go
// Tasks:
// [ ] Extract request_id from RequestID middleware (already in Echo context)
// [ ] Extract user from JWT middleware claims
// [ ] Extract org_id from organization middleware
// [ ] Inject logger and context fields into Go context
// [ ] Ensure middleware ordering: RequestID -> JWT -> Organization -> StructuredLogging
```

#### 3.3 Integration Tests (`tests/integration/logging_test.go`)

```go
// Tasks:
// [ ] Test middleware captures request start/complete
// [ ] Test context propagation through handlers
// [ ] Test error logging with stack traces
// [ ] Test skipper functionality (health checks skipped)
// [ ] Test multiple writers simultaneously
// [ ] Test with real Echo server (testcontainers not required for logging tests)
```

#### Phase 3 Success Criteria
- [ ] Middleware logs all requests
- [ ] Request ID appears in all logs
- [ ] User ID appears when authenticated
- [ ] Org ID appears when present
- [ ] Duration is accurate
- [ ] Errors include stack traces
- [ ] Skipper works for health checks
- [ ] Integration tests pass

---

### Phase 4: Output Writers & Configuration (Day 2 PM)

**Status**: Ready | **Duration**: 4 hours | **Dependencies**: Phase 2

#### 4.1 Configuration Integration

**`internal/config/config.go` (modifications):**
```go
// Tasks:
// [ ] Add LogConfig struct to main Config
// [ ] Add environment variable mapping
// [ ] Implement validation
// [ ] Add defaults
```

#### 4.2 Logger Factory (`internal/logger/factory.go`)

```go
// Tasks:
// [ ] Implement NewFromConfig constructor
// [ ] Implement writer creation from config
// [ ] Implement level parsing from string
// [ ] Implement format selection (json/text)
// [ ] Add error handling
```

#### 4.3 Application Integration (`cmd/api/main.go` modifications)

```go
// Tasks:
// [ ] Load logger config from environment in runServer
// [ ] Create logger in server initialization
// [ ] Pass logger to middleware setup
// [ ] Replace/adjust existing logging setup
// [ ] Add graceful shutdown for writers (flush on shutdown)
```

#### 4.4 Environment Variable Documentation

Update `.env.example`:
```bash
# Logging Configuration
LOG_LEVEL=info                    # debug, info, warn, error
LOG_FORMAT=json                   # json, text
LOG_OUTPUTS=stdout                # stdout, file, syslog (comma-separated)
LOG_FILE_PATH=/var/log/api.log   # Required if outputs includes "file"
LOG_FILE_MAX_SIZE=100             # MB
LOG_FILE_MAX_BACKUPS=10
LOG_FILE_MAX_AGE=30                 # days
LOG_FILE_COMPRESS=true
LOG_SYSLOG_NETWORK=tcp            # tcp, udp, or "" for local
LOG_SYSLOG_ADDRESS=localhost:514
LOG_SYSLOG_TAG=go-api
LOG_ADD_SOURCE=false              # Include file:line in logs
```

#### Phase 4 Success Criteria
- [ ] Environment variables configure logger
- [ ] Multiple outputs work simultaneously
- [ ] File rotation works (test with small MaxSize)
- [ ] Syslog output works (if configured)
- [ ] Configuration validates on startup (fail fast)
- [ ] Application starts with logger configured

---

### Phase 5: Handler Integration (Day 3 AM-PM)

**Status**: Ready | **Duration**: 6 hours | **Dependencies**: Phase 3, Phase 4

#### 5.1 Update Handlers (Gradual Migration)

Priority order (highest traffic first):

**1. Auth Handlers** (`internal/http/handler/auth.go`):
```go
// Tasks:
// [ ] Add logger extraction from context
// [ ] Replace slog calls with structured logger
// [ ] Add structured fields (user_id, email, action)
// [ ] Update error handling
// [ ] Security: Don't log passwords/tokens
```

**2. User Handlers** (`internal/http/handler/user.go`):
```go
// Tasks:
// [ ] Add logger extraction
// [ ] Log CRUD operations with user_id
// [ ] Log role/permission changes
// [ ] Update error handling
```

**3. Organization Handlers** (`internal/http/handler/organization.go`):
```go
// Tasks:
// [ ] Add logger extraction
// [ ] Log organization operations with org_id
// [ ] Log member operations
// [ ] Update error handling
```

**4. Invoice Module** (`internal/module/invoice/handler.go`):
```go
// Tasks:
// [ ] Example module pattern integration
// [ ] Log invoice operations
```

**5. Email Handlers** (`internal/http/handler/email*.go`):
```go
// Tasks:
// [ ] Log template operations
// [ ] Log queue operations
```

**6. Other Handlers** (Media, News, etc.):
```go
// Tasks:
// [ ] Apply same pattern
```

#### 5.2 Service Layer Updates (Key Services)

**`internal/service/` modifications:**
```go
// Tasks:
// [ ] Pass context to service methods (already done)
// [ ] Use logger.FromContext in services
// [ ] Add structured logging for business operations
// [ ] Integrate with existing audit logging
```

#### 5.3 Migration Pattern

```go
// Gradual migration approach:

// Step 1: Add structured logger alongside existing
func (h *Handler) Action(c echo.Context) error {
    log := middleware.GetLogger(c)  // NEW
    ctx := c.Request().Context()
    
    slog.Info("action called")  // OLD (still works)
    log.Info(ctx, "action called", log.String("key", "value"))  // NEW
    
    // ... handler logic
}

// Step 2: Eventually remove old slog calls
```

#### Phase 5 Success Criteria
- [ ] All handlers use structured logger
- [ ] All log calls include context fields
- [ ] Error logging includes request context
- [ ] Existing tests still pass (backward compatibility)
- [ ] No breaking changes to API responses
- [ ] Security: No PII logged (passwords, tokens)

---

### Phase 6: Testing & Quality Assurance (Day 3 PM)

**Status**: Ready | **Duration**: 4 hours | **Dependencies**: Phase 4, Phase 5

#### 6.1 Unit Tests Completion

**Coverage targets:**
- [ ] `internal/logger/*.go` - >90% coverage
- [ ] All field types tested
- [ ] All context operations tested
- [ ] All writer types tested
- [ ] Level filtering tested
- [ ] Error handling tested

#### 6.2 Integration Tests

**`tests/integration/logging_test.go`:**
```go
// Test scenarios:
// [ ] Full request/response logging flow
// [ ] Context propagation through middleware chain
// [ ] Multiple writers (stdout + file)
// [ ] Error logging with stack traces
// [ ] Configuration from environment variables
// [ ] Health check skipping
// [ ] JWT claims extraction
// [ ] Organization ID extraction
```

#### 6.3 Performance Tests

**`tests/benchmark/logger_bench_test.go`:**
```go
// Benchmarks:
// [ ] BenchmarkLoggerInfo - target <1ms
// [ ] BenchmarkLoggerWithFields - target <1ms
// [ ] BenchmarkContextExtraction - target <500µs
// [ ] BenchmarkMiddleware - target <2ms
// [ ] Memory allocations per log call
```

Run benchmarks:
```bash
go test -bench=. -benchmem ./tests/benchmark/...
```

#### 6.4 Load Testing

```bash
# Run load test
go run tests/load/logging_load.go

# Verify:
# - No goroutine leaks
# - Memory usage stable
# - No dropped logs
# - p99 latency < 2ms for middleware
```

#### 6.5 Quality Gates

- [ ] `go test ./... -v` passes
- [ ] `go test -race ./...` passes
- [ ] 80%+ unit test coverage
- [ ] All integration tests pass
- [ ] Performance benchmarks pass
- [ ] Load test passes (1000 req/s, p99 < 2ms)
- [ ] `golangci-lint` passes

**Coverage Verification Command:**
```bash
# Generate coverage report
go test -cover -coverprofile=coverage.out ./internal/logger/...

# View coverage summary
go tool cover -func=coverage.out | grep total

# Expected output: total: (v)% of statements
# Must be >= 80%

# Optional: Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html
```

#### Phase 6 Success Criteria
- [ ] All tests pass
- [ ] Race detector clean
- [ ] Coverage >80%
- [ ] Performance targets met
- [ ] No breaking changes verified

---

### Phase 7: Documentation & Deployment (Day 4 AM)

**Status**: Ready | **Duration**: 3 hours | **Dependencies**: Phase 6

#### 7.1 Update AGENTS.md

Add section to `AGENTS.md`:
```markdown
## LOGGING FEATURE

### Structured Logging System
- **Location**: `internal/logger/`
- **Middleware**: `internal/http/middleware/structured_logging.go`
- **Usage in Handlers**: 
  ```go
  log := middleware.GetLogger(c)
  log.Info(ctx, "message", log.String("key", "value"))
  ```

### Configuration
- `LOG_LEVEL` - debug, info, warn, error
- `LOG_FORMAT` - json, text
- `LOG_OUTPUTS` - stdout, file, syslog (comma-separated)
- `LOG_FILE_*` - File rotation settings
- `LOG_SYSLOG_*` - Syslog settings

### Patterns
- **Field logging**: `log.Info(ctx, "msg", log.String("key", "val"))`
- **Error logging**: `log.Error(ctx, "msg", log.Err(err))`
- **Field chaining**: `log.WithFields(...).Info(ctx, "msg")`
- **Context propagation**: `logger.FromContext(ctx)`

### Migration from slog
```go
// Old
slog.Info("user created", slog.String("id", userID))

// New
log := middleware.GetLogger(c)
log.Info(ctx, "user created", log.String("id", userID))
```
```

#### 7.2 Update README.md

Add section:
```markdown
## Logging

The API uses structured logging with the following features:
- Structured JSON/text output
- Multiple writers (stdout, file, syslog)
- Automatic context propagation (request_id, user_id, org_id, trace_id)
- Request/response logging middleware
- Configurable via environment variables

See [quickstart.md](specs/005-log-writer/quickstart.md) for usage examples.
```

#### 7.3 Update `.env.example`

Add logging configuration variables (see Phase 4.4).

#### 7.4 Create Migration Guide

**`docs/migrations/005-log-writer.md`:**
```markdown
# Migration Guide: Structured Logging

## Pre-deployment
1. Review current logging patterns
2. Set LOG_LEVEL=info (default)
3. Configure LOG_OUTPUTS=stdout initially

## Deployment
1. Deploy new code (backward compatible)
2. Verify logs appear with request_id
3. Monitor for errors
4. Gradually enable file/syslog output

## Post-deployment Handler Updates
1. Update handlers to use structured logger
2. Add structured fields to logs
3. Enable file rotation if needed
4. Configure syslog for centralized logging

## Rollback
- Safe to rollback - existing slog calls still work
- Logger configuration optional - defaults safe

### Explicit Rollback Steps (if needed)
1. **Code Rollback**: `git revert <commit-hash>` or `git checkout <previous-branch>`
2. **Configuration Rollback**: Remove `LOG_*` environment variables from deployment
3. **Middleware Removal**: Comment out `middleware.StructuredLogging()` in `internal/http/server.go`
4. **Handler Cleanup**: Remove logger parameter from service constructors (optional - backward compatible)
5. **Verification**: Restart API, verify logs still appear via existing slog calls
6. **No Data Loss**: No database changes - zero risk of data corruption
```

#### 7.5 Deployment Checklist

- [ ] Update `.env.example`
- [ ] Update `docker-compose.yml` (add log volumes if needed)
- [ ] Update README.md
- [ ] Update AGENTS.md with logging reference
- [ ] Create migration guide
- [ ] Update deployment runbook
- [ ] Add log rotation cron job (if not using lumberjack)
- [ ] Configure log aggregation (if using external system)

#### Phase 7 Success Criteria
- [ ] AGENTS.md updated with logging reference
- [ ] README.md updated with logging section
- [ ] `.env.example` updated with new variables
- [ ] Migration guide complete
- [ ] All documentation reviewed and accurate

---

## Testing Strategy

### Unit Test Patterns

```go
// Table-driven tests
func TestLoggerLevels(t *testing.T) {
    tests := []struct {
        name     string
        level    Level
        logFunc  func(Logger, context.Context, string)
        expected bool
    }{
        {"debug logs at debug", LevelDebug, Logger.Debug, true},
        {"info logs at debug", LevelDebug, Logger.Info, true},
        {"debug no-op at info", LevelInfo, Logger.Debug, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Test Patterns

```go
// Test with real Echo server
func TestLoggingMiddleware(t *testing.T) {
    e := echo.New()
    log, _ := logger.New(logger.Config{Level: "debug"})
    
    e.Use(middleware.StructuredLogging(middleware.LoggingMiddlewareConfig{
        Logger: log,
    }))
    
    e.GET("/test", func(c echo.Context) error {
        return c.String(200, "OK")
    })
    
    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)
    
    assert.Equal(t, 200, rec.Code)
    // Verify logs captured
}
```

### Benchmark Patterns

```go
func BenchmarkLogger(b *testing.B) {
    log, _ := logger.New(logger.Config{Level: "info"})
    ctx := context.Background()
    
    b.ReportAllocs()
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        log.Info(ctx, "benchmark",
            logger.String("key1", "value1"),
            logger.Int("key2", 42),
        )
    }
}
```

---

## Quality Gates

### Per-Phase Gates

| Phase | Test Coverage | Lint | Integration | Performance |
|-------|---------------|------|-------------|-------------|
| Phase 2 | >80% | ✅ | N/A | Benchmarks |
| Phase 3 | >80% | ✅ | Middleware | <2ms |
| Phase 4 | >80% | ✅ | Config | <2ms |
| Phase 5 | >80% | ✅ | End-to-end | <2ms |
| Phase 6 | >80% | ✅ | All | Load test |
| Phase 7 | N/A | ✅ | N/A | N/A |

### Final Release Gate

- [ ] All phases complete
- [ ] 80%+ test coverage
- [ ] All integration tests pass
- [ ] Load test: 1000 req/s sustained for 5 minutes, p99 < 2ms
- [ ] No breaking changes verified
- [ ] Documentation complete
- [ ] Code review approved

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Performance degradation | Low | Medium | Benchmarks, async option, optimization |
| Breaking existing code | Low | High | Backward compatibility, slog fallback, gradual migration |
| Log disk space exhaustion | Medium | Medium | File rotation (lumberjack), monitoring, alerts |
| Context propagation failures | Low | High | Comprehensive testing, fallback values, safe defaults |
| Memory leaks | Low | High | Race detector, load testing, profiling |
| Concurrent access bugs | Low | High | Goroutine-safe implementations, mutexes, race detector |

---

## Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 0: Research | 2h | - |
| Phase 1: Design | 3h | Phase 0 |
| Phase 2: Logger Interface | 4h | Phase 1 |
| Phase 3: Middleware | 4h | Phase 2 |
| Phase 4: Writers & Config | 4h | Phase 2 |
| Phase 5: Handler Integration | 6h | Phase 3, 4 |
| Phase 6: Testing | 4h | Phase 4, 5 |
| Phase 7: Documentation | 3h | Phase 6 |
| **Total** | **30h** | |

**Estimated Duration**: 3-4 days (with parallel work possible)

---

## Dependencies

### External Dependencies

| Package | Purpose | Version |
|---------|---------|---------|
| log/slog | Standard logging | Go 1.22+ |
| gopkg.in/natefinch/lumberjack.v2 | File rotation | v2.2.1 |
| log/syslog | Syslog support | Standard |

### Internal Dependencies

| Package | Purpose |
|---------|---------|
| internal/http/middleware | Middleware integration |
| internal/config | Configuration loading |
| internal/auth | JWT claims extraction |

---

## Environment Variables Reference

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LOG_LEVEL` | string | info | Minimum log level (debug, info, warn, error) |
| `LOG_FORMAT` | string | json | Output format (json, text) |
| `LOG_OUTPUTS` | []string | stdout | Comma-separated destinations |
| `LOG_FILE_PATH` | string | /var/log/api.log | Log file path |
| `LOG_FILE_MAX_SIZE` | int | 100 | Max file size (MB) |
| `LOG_FILE_MAX_BACKUPS` | int | 10 | Backup file count |
| `LOG_FILE_MAX_AGE` | int | 30 | Max age (days) |
| `LOG_FILE_COMPRESS` | bool | true | Compress backups |
| `LOG_SYSLOG_NETWORK` | string | "" | Network type (tcp, udp) |
| `LOG_SYSLOG_ADDRESS` | string | "" | Server address |
| `LOG_SYSLOG_TAG` | string | go-api | Syslog tag |
| `LOG_ADD_SOURCE` | bool | false | Include file:line |

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Log call overhead | <1ms | Benchmark |
| Middleware overhead | <2ms | Benchmark |
| Unit test coverage | >80% | `go test -cover` |
| Integration tests | 100% pass | `go test -tags=integration` |
| Breaking changes | 0 | API compatibility check |
| Handler adoption | 100% | Code review |
| Documentation | Complete | AGENTS.md, quickstart.md, README.md |
| Performance (load) | 1000 req/s, p99 < 2ms | Load test |
| Race conditions | 0 | `go test -race` |

---

## Appendix A: Logger Interface Reference

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| Debug | ctx, msg, fields... | - | Development logging |
| Info | ctx, msg, fields... | - | State changes |
| Warn | ctx, msg, fields... | - | Degraded conditions |
| Error | ctx, msg, fields... | - | Failures with stack trace |
| WithFields | fields... | Logger | Create scoped logger (immutable) |
| WithError | err | Logger | Add error context (immutable) |
| String | key, value | Field | Create string field |
| Int | key, value | Field | Create int field |
| Int64 | key, value | Field | Create int64 field |
| Float64 | key, value | Field | Create float64 field |
| Bool | key, value | Field | Create bool field |
| Duration | key, value | Field | Create duration field |
| Time | key, value | Field | Create time field |
| Any | key, value | Field | Create any field |

## Appendix B: Context Keys Reference

| Key | Type | Source | Description |
|-----|------|--------|-------------|
| logger | Logger | Middleware | Logger instance |
| request_id | string | Header/Generated | Request correlation ID |
| user_id | string | JWT claims | Authenticated user ID |
| org_id | string | Header | Organization ID |
| trace_id | string | Header/Generated | Distributed tracing ID |
| start_time | time.Time | Middleware | Request timing |

## Appendix C: Configuration Structures

```go
// Config holds logger configuration
type Config struct {
    Level       string       // DEBUG, INFO, WARN, ERROR
    Format      string       // json, text
    Outputs     []string     // stdout, file, syslog
    File        FileConfig   // File writer settings
    Syslog      SyslogConfig // Syslog settings
    AddSource   bool         // Include file:line
}

type FileConfig struct {
    Path       string // Log file path
    MaxSize    int    // MB before rotation
    MaxBackups int    // Number of backups
    MaxAge     int    // Days to keep backups
    Compress   bool   // Compress backups
}

type SyslogConfig struct {
    Network string // tcp, udp, or "" for local
    Address string // Server address
    Tag     string // Syslog tag
}
```

---

## Sign-off

- [ ] Architecture Review: ___________
- [ ] Security Review: ___________
- [ ] Performance Review: ___________
- [ ] Documentation Review: ___________

---

**END OF PLAN - READY FOR IMPLEMENTATION**
