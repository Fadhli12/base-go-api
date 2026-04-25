# Feature Specification: Structured Logging System

**Version**: 1.0 | **Date**: 2026-04-25 | **Status**: Draft

---

## Overview

### Problem Statement

The Go API Base currently uses basic `slog` logging without a proper abstraction layer. This creates several issues:

1. **No log writer abstraction** - Logging is tightly coupled to `slog` package
2. **No structured context propagation** - Request IDs, user IDs, org IDs not consistently logged
3. **No log filtering/routing** - All logs go to stdout; no ability to route by level or component
4. **No performance metrics** - No built-in request duration, error tracking
5. **No log formatting flexibility** - Hard to switch between JSON, text, or custom formats

### Proposed Solution

Implement a **structured logging system** with:
- Clean abstraction layer (`Logger` interface)
- Request context propagation (request ID, user ID, org ID, trace ID)
- Structured field support (key-value pairs)
- Multiple output writers (stdout, file, syslog)
- Log level filtering and routing
- Performance metrics (request duration, error counts)
- Middleware integration for automatic request/response logging

### Success Criteria

- ✅ All handlers use structured logger via middleware
- ✅ Request ID, user ID, org ID automatically included in logs
- ✅ Logger interface allows swapping implementations
- ✅ Support for multiple output writers (stdout, file, syslog)
- ✅ Request/response logging middleware captures duration and status
- ✅ Error logging includes stack traces and context
- ✅ Zero breaking changes to existing code
- ✅ Integration tests verify log output

---

## Requirements

### Functional Requirements

#### FR-1: Logger Interface

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1.1 | Define `Logger` interface with Debug, Info, Warn, Error methods | MUST |
| FR-1.2 | Support structured fields (key-value pairs) | MUST |
| FR-1.3 | Support field chaining for context building | MUST |
| FR-1.4 | Implement `slog`-based logger as default | MUST |
| FR-1.5 | Logger methods accept `context.Context` as first parameter | MUST |

#### FR-2: Context Propagation

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-2.1 | Extract request ID from context or generate UUID | MUST |
| FR-2.2 | Extract user ID from JWT claims | MUST |
| FR-2.3 | Extract organization ID from X-Organization-ID header | MUST |
| FR-2.4 | Extract trace ID from X-Trace-ID header or generate | MUST |
| FR-2.5 | Store context fields in `context.Context` for retrieval | MUST |

#### FR-3: Middleware Integration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-3.1 | Request logging middleware captures method, path, query params | MUST |
| FR-3.2 | Response logging middleware captures status, duration, bytes | MUST |
| FR-3.3 | Error logging middleware captures error type and stack trace | MUST |
| FR-3.4 | Middleware injects logger into context for handler use | MUST |
| FR-3.5 | Request ID propagated through entire request lifecycle | MUST |

#### FR-4: Output Writers

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-4.1 | Support stdout writer (default) | MUST |
| FR-4.2 | Support file writer with rotation | SHOULD |
| FR-4.3 | Support syslog writer | SHOULD |
| FR-4.4 | Support custom writer interface | MUST |
| FR-4.5 | Multiple writers can be chained | SHOULD |

#### FR-5: Log Levels

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-5.1 | Support DEBUG, INFO, WARN, ERROR levels | MUST |
| FR-5.2 | Configurable minimum log level | MUST |
| FR-5.3 | Different levels for different components | SHOULD |
| FR-5.4 | Environment variable to set log level | MUST |

### Non-Functional Requirements

#### NFR-1: Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1.1 | Logger overhead per log call | < 1ms |
| NFR-1.2 | Context extraction overhead | < 0.5ms |
| NFR-1.3 | Middleware overhead per request | < 2ms |

#### NFR-2: Reliability

| ID | Requirement |
|----|-------------|
| NFR-2.1 | Failed log writes don't crash application |
| NFR-2.2 | Log buffering prevents blocking on I/O |
| NFR-2.3 | Graceful shutdown flushes pending logs |

#### NFR-3: Compatibility

| ID | Requirement |
|----|-------------|
| NFR-3.1 | No breaking changes to existing handlers |
| NFR-3.2 | Existing `slog` calls continue working |
| NFR-3.3 | Backward compatible with current logging |

---

## Data Model

### Logger Interface

```go
type Logger interface {
    // Core logging methods
    Debug(ctx context.Context, msg string, fields ...Field)
    Info(ctx context.Context, msg string, fields ...Field)
    Warn(ctx context.Context, msg string, fields ...Field)
    Error(ctx context.Context, msg string, fields ...Field)
    
    // Context-aware logging
    WithFields(fields ...Field) Logger
    WithError(err error) Logger
    
    // Structured field support
    String(key, value string) Field
    Int(key string, value int) Field
    Int64(key string, value int64) Field
    Float64(key string, value float64) Field
    Bool(key string, value bool) Field
    Duration(key string, value time.Duration) Field
    Any(key string, value interface{}) Field
}

type Field struct {
    Key   string
    Value interface{}
}
```

### Context Keys

```go
const (
    ContextKeyRequestID    = "request_id"
    ContextKeyUserID       = "user_id"
    ContextKeyOrgID        = "org_id"
    ContextKeyTraceID      = "trace_id"
    ContextKeyLogger       = "logger"
    ContextKeyStartTime    = "start_time"
)
```

### Log Entry Structure

```go
type LogEntry struct {
    Timestamp   time.Time              `json:"timestamp"`
    Level       string                 `json:"level"`
    Message     string                 `json:"message"`
    RequestID   string                 `json:"request_id,omitempty"`
    UserID      string                 `json:"user_id,omitempty"`
    OrgID       string                 `json:"org_id,omitempty"`
    TraceID     string                 `json:"trace_id,omitempty"`
    Fields      map[string]interface{} `json:"fields,omitempty"`
    Error       string                 `json:"error,omitempty"`
    StackTrace  string                 `json:"stack_trace,omitempty"`
}
```

---

## API/Integration Points

### Logger Usage in Handlers

```go
func (h *Handler) GetUser(c echo.Context) error {
    logger := middleware.GetLogger(c)
    userID := c.Param("id")
    
    logger.Info(c.Request().Context(), "fetching user",
        logger.String("user_id", userID),
    )
    
    user, err := h.service.GetUser(c.Request().Context(), userID)
    if err != nil {
        logger.Error(c.Request().Context(), "failed to fetch user",
            logger.String("user_id", userID),
            logger.String("error", err.Error()),
        )
        return echo.NewHTTPError(http.StatusNotFound, "User not found")
    }
    
    logger.Info(c.Request().Context(), "user fetched successfully",
        logger.String("user_id", userID),
    )
    return c.JSON(http.StatusOK, user)
}
```

### Middleware Integration

```go
// Request logging middleware
func LoggingMiddleware(logger Logger) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Extract/generate context fields
            requestID := c.Request().Header.Get("X-Request-ID")
            if requestID == "" {
                requestID = uuid.New().String()
            }
            
            // Create context with logger and fields
            ctx := context.WithValue(c.Request().Context(), ContextKeyRequestID, requestID)
            ctx = context.WithValue(ctx, ContextKeyLogger, logger)
            
            // Log request
            logger.Info(ctx, "request started",
                logger.String("method", c.Request().Method),
                logger.String("path", c.Request().URL.Path),
            )
            
            // Execute handler
            start := time.Now()
            err := next(c)
            duration := time.Since(start)
            
            // Log response
            logger.Info(ctx, "request completed",
                logger.Int("status", c.Response().Status),
                logger.Duration("duration", duration),
            )
            
            return err
        }
    }
}
```

---

## Implementation Phases

### Phase 1: Logger Interface & Implementation (Day 1 AM)

- Define `Logger` interface in `internal/logger/logger.go`
- Implement `slog`-based logger in `internal/logger/slog_logger.go`
- Define context keys in `internal/logger/context.go`
- Add field helpers (String, Int, Duration, etc.)
- Unit tests for logger interface

### Phase 2: Context Propagation (Day 1 PM)

- Create context extraction helpers in `internal/logger/context.go`
- Extract request ID, user ID, org ID, trace ID
- Add context setters for middleware
- Unit tests for context extraction

### Phase 3: Middleware Integration (Day 1 PM)

- Create logging middleware in `internal/http/middleware/logging.go`
- Integrate into server middleware chain
- Add request/response logging
- Add error logging middleware
- Integration tests for middleware

### Phase 4: Output Writers (Day 2 AM)

- Implement file writer with rotation
- Implement syslog writer
- Create writer interface for extensibility
- Configuration for multiple writers
- Tests for each writer type

### Phase 5: Handler Integration (Day 2 PM)

- Update existing handlers to use logger
- Add structured fields to all log calls
- Update error handling to use logger
- Verify no breaking changes

### Phase 6: Configuration (Day 2 PM)

- Add log level configuration
- Add writer configuration
- Environment variables for log settings
- Config validation

### Phase 7: Testing & Documentation (Day 3 AM)

- Integration tests for full logging flow
- Performance benchmarks
- Update AGENTS.md with logging patterns
- Add Swagger annotations for log endpoints (if any)

---

## Architecture Patterns

### Logger Injection Pattern

```go
type Handler struct {
    logger  logger.Logger
    service Service
}

func (h *Handler) Operation(c echo.Context) error {
    ctx := c.Request().Context()
    h.logger.Info(ctx, "operation started")
    // ... handler logic
}
```

### Context-Aware Logging

```go
// In middleware
ctx = context.WithValue(ctx, logger.ContextKeyRequestID, requestID)
ctx = context.WithValue(ctx, logger.ContextKeyUserID, userID)

// In handler
logger.Info(ctx, "message") // Automatically includes request_id, user_id
```

### Field Chaining

```go
logger.WithFields(
    logger.String("user_id", userID),
    logger.String("action", "create"),
).Info(ctx, "user created")
```

---

## Testing Strategy

### Unit Tests

- Logger interface implementation
- Context extraction functions
- Field helpers
- Writer implementations

### Integration Tests

- Full request logging flow
- Context propagation through middleware
- Multiple writers
- Error logging with stack traces

### Performance Tests

- Logger overhead benchmarks
- Context extraction performance
- Middleware overhead

---

## Configuration

### Environment Variables

```bash
LOG_LEVEL=info                    # DEBUG, INFO, WARN, ERROR
LOG_FORMAT=json                   # json or text
LOG_OUTPUT=stdout                 # stdout, file, syslog, or comma-separated
LOG_FILE_PATH=/var/log/api.log   # For file writer
LOG_FILE_MAX_SIZE=100             # MB
LOG_FILE_MAX_BACKUPS=10           # Number of backup files
LOG_FILE_MAX_AGE=30               # Days
```

### Configuration Structure

```go
type Config struct {
    Level       string   // DEBUG, INFO, WARN, ERROR
    Format      string   // json, text
    Outputs     []string // stdout, file, syslog
    FilePath    string   // For file writer
    MaxSize     int      // MB
    MaxBackups  int
    MaxAge      int      // Days
}
```

---

## Success Metrics

- [ ] Logger interface defined and implemented
- [ ] All handlers use structured logger
- [ ] Request ID, user ID, org ID in all logs
- [ ] Middleware captures request/response metrics
- [ ] Multiple output writers supported
- [ ] Zero breaking changes to existing code
- [ ] Integration tests pass with 80%+ coverage
- [ ] Performance overhead < 2ms per request
- [ ] Documentation complete (Swagger + AGENTS.md)

---

## Dependencies

### Internal Dependencies

- `internal/http` - Middleware integration
- `internal/domain` - Entity logging
- `internal/service` - Service layer logging

### External Dependencies

- `log/slog` - Standard library logging
- `gopkg.in/natefinch/lumberjack.v2` - File rotation (optional)

---

## Risks and Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Performance degradation | Low | Medium | Benchmark and optimize hot paths |
| Breaking existing code | Low | High | Gradual migration, backward compatibility |
| Log file disk space | Medium | Medium | Implement rotation and cleanup |
| Context propagation failures | Low | High | Comprehensive testing, fallback values |

---

## Timeline

**Total Effort**: 2-3 days

| Phase | Duration | Dependencies |
|-------|-----------|--------------|
| Phase 1: Logger Interface | 3 hours | None |
| Phase 2: Context Propagation | 2 hours | Phase 1 |
| Phase 3: Middleware Integration | 3 hours | Phase 2 |
| Phase 4: Output Writers | 3 hours | Phase 1 |
| Phase 5: Handler Integration | 4 hours | Phase 3 |
| Phase 6: Configuration | 2 hours | Phase 4 |
| Phase 7: Testing & Documentation | 3 hours | All phases |

---

## References

- [Go log/slog Package](https://pkg.go.dev/log/slog)
- [Structured Logging Best Practices](https://www.kartar.net/2015/12/structured-logging/)
- [AGENTS.md](../../AGENTS.md)
