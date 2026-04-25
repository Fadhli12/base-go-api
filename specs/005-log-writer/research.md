# Structured Logging System - Research Findings

**Feature**: 005-log-writer - Structured Logging System  
**Date**: 2026-04-25  
**Status**: ✅ Complete

---

## Executive Summary

Research confirms that Go 1.22+ `log/slog` is the optimal foundation for structured logging. The implementation should leverage Go's standard library while providing an abstraction layer for multi-writer support, context propagation, and middleware integration.

---

## Research Topics

### 1. Go 1.22+ slog Best Practices

#### Key Findings

- **Standard Library Stability**: `log/slog` was introduced in Go 1.21 and stabilized in Go 1.22 with enhanced text handler performance
- **Structured Logging**: Native support for key-value pairs via `slog.Attr` and `slog.Record`
- **Handler Interface**: Clean separation between log creation and log output via `slog.Handler`
- **Context Integration**: Built-in `slog.NewContext()` and `slog.FromContext()` for request-scoped logging

#### Best Practice Patterns

```go
// Context-aware logger (slog native)
ctx = slog.WithContext(ctx, logger)
logger = slog.FromContext(ctx)
logger.Info("message", slog.String("key", "value"))

// Handler customization
handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
    AddSource: true, // Include file:line
})
```

#### Performance Characteristics
- **Zero-allocation logging**: When using compile-time constant keys
- **Buffer reuse**: JSONHandler uses internal buffer pool
- **Async by default**: slog handlers are synchronous, but can be wrapped

### 2. Echo Middleware Context Propagation

#### Current Project Patterns

The project already has robust middleware patterns:

| Context Key | Source | Location |
|------------|--------|----------|
| `request_id` | `X-Request-ID` header or generated UUID | `middleware/request_id.go:12` |
| `user` (claims) | JWT Bearer token | `middleware/jwt.go:72` |
| `organization_id` | `X-Organization-ID` header | `middleware/organization.go:43` |

#### Context Propagation Strategy

```go
// Logger middleware should:
// 1. Extract context from Echo
// 2. Inject into context.Context
// 3. Store logger reference for handlers

type loggerContextKey struct{}

func WithLogger(ctx context.Context, logger Logger) context.Context {
    return context.WithValue(ctx, loggerContextKey{}, logger)
}

func FromContext(ctx context.Context) Logger {
    if logger, ok := ctx.Value(loggerContextKey{}).(Logger); ok {
        return logger
    }
    return DefaultLogger()
}
```

#### Echo Context vs Go Context

| Aspect | Echo Context | Go Context |
|--------|--------------|------------|
| Storage | `c.Set(key, val)` | `context.WithValue()` |
| Retrieval | `c.Get(key)` | `ctx.Value(key)` |
| Request lifecycle | Per-request | Per-request + cancel |
| Best for | Request-scoped data | Context propagation |

**Recommendation**: Use Echo context for request metadata, Go context for logger propagation (enables passing to repositories/services).

### 3. Log Writer Implementations

#### Multi-Writer Pattern

```go
// Writer interface (custom abstraction)
type Writer interface {
    Write(ctx context.Context, entry LogEntry) error
    Close() error
}

// Multi-writer composition
type MultiWriter struct {
    writers []Writer
}

func (m *MultiWriter) Write(ctx context.Context, entry LogEntry) error {
    for _, w := range m.writers {
        if err := w.Write(ctx, entry); err != nil {
            // Log errors shouldn't crash app
            // But log to stderr if possible
            fmt.Fprintf(os.Stderr, "log write error: %v\n", err)
        }
    }
    return nil
}
```

#### File Rotation with lumberjack.v2

```go
import "gopkg.in/natefinch/lumberjack.v2"

func NewFileWriter(path string) Writer {
    return &fileWriter{
        logger: &lumberjack.Logger{
            Filename:   path,
            MaxSize:    100, // MB
            MaxBackups: 10,
            MaxAge:     30,  // days
            Compress:   true,
        },
    }
}
```

#### Syslog Writer

```go
import "log/syslog"

func NewSyslogWriter(network, addr, tag string) (Writer, error) {
    writer, err := syslog.Dial(network, addr, syslog.LOG_INFO, tag)
    if err != nil {
        return nil, err
    }
    return &syslogWriter{writer: writer}, nil
}
```

### 4. Performance Optimization

#### Benchmark Requirements

| Metric | Target | Measurement |
|--------|--------|-------------|
| Log call overhead | < 1ms | `BenchmarkLoggerInfo` |
| Context extraction | < 0.5ms | `BenchmarkContextExtraction` |
| Middleware overhead | < 2ms | `BenchmarkRequestLogging` |

#### Optimization Strategies

1. **Lazy Evaluation**: Only extract context fields when actually logging
2. **Buffer Pooling**: Reuse buffers for JSON encoding (slog does this)
3. **Async Writers**: Use channels for non-blocking writes
4. **Sampling**: Skip logging for high-frequency, low-value events

#### Implementation Pattern

```go
type asyncWriter struct {
    ch     chan LogEntry
    writer Writer
}

func (w *asyncWriter) Write(ctx context.Context, entry LogEntry) error {
    select {
    case w.ch <- entry:
        return nil
    default:
        // Channel full - drop log (better than blocking)
        return nil
    }
}
```

### 5. Audit Logging Integration

#### Current Audit System

Located at `internal/service/audit.go`:
- Captures before/after JSONB state
- Async logging to database
- Actor ID, resource ID tracking

#### Integration Points

| Audit Requirement | Logging Solution |
|------------------|------------------|
| Actor tracking | Extract user_id from JWT claims |
| Resource tracking | Extract from handler parameters |
| Before/after state | Pass as fields to logger |
| Compliance | Structured logs to append-only storage |

#### Integration Pattern

```go
// Logger captures same context as audit
logger.Info(ctx, "user created",
    logger.String("user_id", userID),
    logger.String("email", email),
    logger.Any("before", nil),        // nil = created
    logger.Any("after", userResponse),
)

// Audit service captures separately for compliance
auditSvc.Log(ctx, service.AuditEntry{
    Action:     "user:create",
    ActorID:    userID,
    Resource:   "users",
    ResourceID: userID,
    Before:     nil,
    After:      userJSON,
})
```

### 6. Trace ID Support

#### OpenTelemetry Compatibility

```go
// Context keys for trace propagation
const (
    TraceIDHeader = "X-Trace-ID"
    SpanIDHeader  = "X-Span-ID"
)

// Extract trace context from headers or generate
traceID := c.Request().Header.Get(TraceIDHeader)
if traceID == "" {
    traceID = generateTraceID() // 16-byte hex
}

// Store in context for downstream propagation
ctx = context.WithValue(ctx, traceIDContextKey, traceID)
```

---

## Technical Decisions

### Decision 1: slog vs Custom Logger

**Chosen**: slog-based with abstraction wrapper

**Rationale**:
- Standard library = no external dependency
- Industry standard (replaced all major logging libraries)
- Handler interface allows custom writers
- Context integration built-in

**Trade-off**: Slightly more verbose API, but performance and stability win.

### Decision 2: Context Propagation Method

**Chosen**: Custom context keys + Echo middleware

**Rationale**:
- Echo context = request metadata storage
- Go context = propagation to repositories/services
- Consistent with existing JWT/organization middleware patterns

### Decision 3: Writer Composition

**Chosen**: Interface-based with multi-writer support

**Rationale**:
- Clean separation of concerns
- Easy to add new writers (file, syslog, cloud)
- Testable with mock writers
- Aligns with Go io.Writer pattern

### Decision 4: Performance Strategy

**Chosen**: Sync writes with optional async wrapper

**Rationale**:
- Default: Synchronous (simpler, no data loss)
- Opt-in: Async for high-throughput scenarios
- Failed writes logged to stderr (don't crash app)

---

## Constitution Compliance Analysis

| Principle | Logging Impact | Compliance |
|-----------|----------------|------------|
| **Observability** | Structured logs with correlation IDs | ✅ Required |
| **Soft deletes** | N/A for logging system | ✅ N/A |
| **SQL migrations** | N/A (no DB storage for logs) | ✅ N/A |
| **RBAC** | Logging doesn't bypass permissions | ✅ Check |
| **Audit logging** | Complements existing audit system | ✅ Compatible |
| **No breaking changes** | Backward compatible with slog | ✅ Required |

---

## Implementation Recommendations

### Phase 1: Core Logger Interface

1. Define `Logger` interface in `internal/logger/logger.go`
2. Implement `slog` wrapper in `internal/logger/slog_logger.go`
3. Add field helpers for type safety

### Phase 2: Context Propagation

1. Create `internal/logger/context.go` with context keys
2. Extract request_id, user_id, org_id, trace_id
3. Inject logger into context

### Phase 3: Middleware Integration

1. Create `internal/http/middleware/logging_v2.go`
2. Replace existing logging middleware
3. Add request/response capture with duration

### Phase 4: Output Writers

1. Implement `StdoutWriter`, `FileWriter`, `SyslogWriter`
2. Add writer configuration
3. Support writer chaining

### Phase 5: Configuration

1. Add log level, format, output settings
2. Environment variable support
3. Config validation

---

## References

- [Go log/slog Package](https://pkg.go.dev/log/slog)
- [slog.Handler Interface](https://pkg.go.dev/log/slog#Handler)
- [lumberjack.v2 File Rotation](https://github.com/natefinch/lumberjack)
- [Echo Middleware Documentation](https://echo.labstack.com/docs/middleware)
- Project: `internal/http/middleware/request_id.go`
- Project: `internal/http/middleware/jwt.go`

---

## Success Criteria Verification

| Criteria | Status | Evidence |
|----------|--------|----------|
| Research complete | ✅ | This document |
| slog best practices | ✅ | Section 1 |
| Echo patterns | ✅ | Section 2 |
| Writer implementations | ✅ | Section 3 |
| Performance targets | ✅ | Section 4 |
| Audit integration | ✅ | Section 5 |
| Constitution check | ✅ | Section compliance |

**All research complete. Ready for Phase 1 design.**
