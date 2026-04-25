# Contract: Logger Interface

**Feature**: 005-log-writer - Structured Logging System  
**Contract Type**: Interface Definition  
**Date**: 2026-04-25  
**Status**: Final

---

## Overview

This document defines the contract for the `Logger` interface, which provides a structured logging abstraction layer. The contract ensures:

- **Backward Compatibility**: Existing code continues working
- **Testability**: Easy to mock for unit tests
- **Flexibility**: Swappable implementations (slog, custom, mock)
- **Performance**: Minimal overhead with context propagation

---

## Interface Definition

### File Location
`internal/logger/logger.go`

### Contract

```go
// Logger is the primary interface for structured logging.
//
// CONTRACT:
// - All methods accept context.Context as first parameter
// - Field methods return Field (not pointer) for stack allocation
// - WithFields returns a NEW logger instance (immutable)
// - WithError returns a NEW logger with error field attached
// - Implementations must be goroutine-safe
// - Implementations must never panic on nil context (use background context)
// - Implementations must handle missing context fields gracefully
//
// USAGE:
//   logger := logger.FromContext(ctx)
//   logger.Info(ctx, "request processed",
//       logger.String("method", "GET"),
//       logger.Duration("duration", elapsed),
//   )
type Logger interface {
    // Debug logs a message at DEBUG level.
    // Messages at this level are development-only and may be expensive.
    // CONTRACT: No-op if level > DEBUG
    Debug(ctx context.Context, msg string, fields ...Field)
    
    // Info logs a message at INFO level.
    // Use for state changes and significant events.
    // CONTRACT: No-op if level > INFO
    Info(ctx context.Context, msg string, fields ...Field)
    
    // Warn logs a message at WARN level.
    // Use for degraded conditions or suspicious behavior.
    // CONTRACT: No-op if level > WARN
    Warn(ctx context.Context, msg string, fields ...Field)
    
    // Error logs a message at ERROR level.
    // Use for failures that affect request processing.
    // CONTRACT: Always logs (unless level > ERROR)
    Error(ctx context.Context, msg string, fields ...Field)
    
    // WithFields returns a new Logger with additional fields.
    // Fields are included in all subsequent log calls.
    // CONTRACT: Returns new instance, original logger unchanged
    WithFields(fields ...Field) Logger
    
    // WithError returns a new Logger with error context.
    // Error is added to fields with key "error".
    // CONTRACT: Returns new instance, original logger unchanged
    WithError(err error) Logger
    
    // Field factory methods
    // CONTRACT: All return Field by value (not pointer)
    String(key, value string) Field
    Int(key string, value int) Field
    Int64(key string, value int64) Field
    Float64(key string, value float64) Field
    Bool(key string, value bool) Field
    Duration(key string, value time.Duration) Field
    Time(key string, value time.Time) Field
    Any(key string, value interface{}) Field
}
```

---

## Field Contract

### File Location
`internal/logger/field.go`

### Contract

```go
// Field represents a structured logging field.
//
// CONTRACT:
// - Immutable once created
// - Key must be non-empty (enforced by implementations)
// - Value must be JSON-serializable
// - Safe for concurrent use
// - Stack-allocated (not heap) for performance
type Field struct {
    Key   string
    Value interface{}
}

// ToSlogAttr converts Field to slog.Attr.
// CONTRACT: Handles all standard types; complex types via reflection
func (f Field) ToSlogAttr() slog.Attr

// Package-level field constructors (free functions)
// CONTRACT: All return Field by value
func String(key, value string) Field
func Int(key string, value int) Field
func Int64(key string, value int64) Field
func Float64(key string, value float64) Field
func Bool(key string, value bool) Field
func Duration(key string, value time.Duration) Field
func Time(key string, value time.Time) Field
func Any(key string, value interface{}) Field
func Err(err error) Field
```

### Supported Types

| Type | Example | JSON Output | Notes |
|------|---------|-------------|-------|
| string | `"hello"` | `"hello"` | Safe for all values |
| int | `42` | `42` | 32-bit on 32-bit arch |
| int64 | `int64(42)` | `42` | Explicit width |
| float64 | `3.14` | `3.14` | IEEE 754 double |
| bool | `true` | `true` | |
| time.Duration | `time.Second` | `"1s"` | Human-readable |
| time.Time | `time.Now()` | `"2026-01-15T10:30:00Z"` | RFC3339 format |
| error | `err` | `"error message"` | nil = omitted |
| any | `struct{...}` | JSON-encoded | Uses reflection |

---

## Context Contract

### File Location
`internal/logger/context.go`

### Contract

```go
// Context operations CONTRACT:
// - Keys use custom types to avoid collisions
// - Values stored as interface{}
// - Extraction returns zero values if not found
// - Never return nil logger (return NopLogger instead)

// WithLogger injects logger into context.
// CONTRACT: Returns new context, original unchanged
func WithLogger(ctx context.Context, logger Logger) context.Context

// FromContext extracts logger from context.
// CONTRACT: Never returns nil; returns NopLogger if not found
func FromContext(ctx context.Context) Logger

// Context field operations
// CONTRACT: All With* functions return new context
// CONTRACT: All Get* functions return zero value if not found
func WithRequestID(ctx context.Context, requestID string) context.Context
func GetRequestID(ctx context.Context) string

func WithUserID(ctx context.Context, userID string) context.Context
func GetUserID(ctx context.Context) string

func WithOrgID(ctx context.Context, orgID string) context.Context
func GetOrgID(ctx context.Context) string

func WithTraceID(ctx context.Context, traceID string) context.Context
func GetTraceID(ctx context.Context) string

// ExtractContextFields gathers all context fields.
// CONTRACT: Returns slice (may be empty, never nil)
func ExtractContextFields(ctx context.Context) []Field
```

---

## Writer Contract

### File Location
`internal/logger/writer.go`

### Contract

```go
// Writer is the interface for log output destinations.
//
// CONTRACT:
// - Implementations must be goroutine-safe
// - Write should not block indefinitely (use timeouts)
// - Errors are logged to stderr but don't crash application
// - Close must flush any buffered data
// - Close may be called multiple times (idempotent)
type Writer interface {
    // Write outputs a log entry.
    // CONTRACT: Context used for timeouts/cancellation
    Write(ctx context.Context, entry LogEntry) error
    
    // Close releases resources and flushes buffers.
    // CONTRACT: Safe to call multiple times
    Close() error
}

// MultiWriter writes to multiple writers.
// CONTRACT: Errors from individual writers don't stop others
// CONTRACT: Close errors are aggregated
func NewMultiWriter(writers ...Writer) *MultiWriter
```

---

## Configuration Contract

### File Location
`internal/logger/config.go`

### Contract

```go
// Config holds logger configuration.
//
// CONTRACT:
// - All fields have sensible defaults
// - Validate() must be called before use
// - Environment variable mapping documented
// - Changes require logger re-initialization
type Config struct {
    Level       string   // "debug", "info", "warn", "error"
    Format      string   // "json", "text"
    Outputs     []string // "stdout", "file", "syslog"
    File        FileConfig
    Syslog      SyslogConfig
    AddSource    bool
}

// Validate checks configuration validity.
// CONTRACT: Returns error for invalid values
func (c Config) Validate() error

// ParseLevel converts level string to Level.
// CONTRACT: Returns LevelInfo for unknown values (safe default)
func (c Config) ParseLevel() Level
```

### Environment Variables

| Variable | Config Field | Default | Description |
|----------|--------------|---------|-------------|
| `LOG_LEVEL` | `Level` | `info` | Minimum log level |
| `LOG_FORMAT` | `Format` | `json` | Output format |
| `LOG_OUTPUTS` | `Outputs` | `stdout` | Comma-separated destinations |
| `LOG_FILE_PATH` | `File.Path` | `/var/log/api.log` | Log file path |
| `LOG_FILE_MAX_SIZE` | `File.MaxSize` | `100` | Max file size (MB) |
| `LOG_FILE_MAX_BACKUPS` | `File.MaxBackups` | `10` | Backup file count |
| `LOG_FILE_MAX_AGE` | `File.MaxAge` | `30` | Max age (days) |
| `LOG_FILE_COMPRESS` | `File.Compress` | `true` | Compress backups |
| `LOG_SYSLOG_NETWORK` | `Syslog.Network` | `""` | Network type |
| `LOG_SYSLOG_ADDRESS` | `Syslog.Address` | `""` | Server address |
| `LOG_SYSLOG_TAG` | `Syslog.Tag` | `go-api` | Syslog tag |
| `LOG_ADD_SOURCE` | `AddSource` | `false` | Include file:line |

---

## Middleware Contract

### File Location
`internal/http/middleware/structured_logging.go`

### Contract

```go
// LoggingMiddlewareConfig holds middleware configuration.
//
// CONTRACT:
// - Logger must be non-nil
// - Skipper may be nil (logs all requests)
// - Body logging disabled by default (PII safety)
type LoggingMiddlewareConfig struct {
    Logger          Logger
    Skipper         func(c echo.Context) bool
    LogRequestBody  bool
    LogResponseBody bool
    BodyMaxSize     int
}

// StructuredLogging returns Echo middleware.
//
// CONTRACT:
// - Injects logger into request context
// - Extracts/exposes request_id, user_id, org_id, trace_id
// - Logs request start (DEBUG) and completion (INFO)
// - Duration calculated from middleware entry
// - Errors logged at ERROR level with stack trace
func StructuredLogging(config LoggingMiddlewareConfig) echo.MiddlewareFunc

// GetLogger retrieves logger from Echo context.
// CONTRACT: Never returns nil (returns NopLogger if not found)
func GetLogger(c echo.Context) Logger
```

---

## Implementation Requirements

### SlogLogger Implementation

**File**: `internal/logger/slog_logger.go`

```go
// SlogLogger wraps slog.Logger with our interface.
//
// CONTRACT:
// - Implements Logger interface
// - Thread-safe (delegates to slog which is thread-safe)
// - Context fields extracted automatically
// - Level filtering via slog.Handler
type SlogLogger struct {
    handler slog.Handler
    level   slog.Level
    fields  []Field // Pre-attached fields
}

// NewSlogLogger creates a new slog-based logger.
// CONTRACT: Returns non-nil logger
func NewSlogLogger(handler slog.Handler) *SlogLogger

// NewNopLogger returns a no-op logger for safe defaults.
// CONTRACT: All methods are no-ops
func NewNopLogger() Logger
```

### StdoutWriter Implementation

**File**: `internal/logger/writer_stdout.go`

```go
// StdoutWriter writes logs to stdout.
//
// CONTRACT:
// - Uses JSON or text format based on config
// - Thread-safe via underlying writer
// - No buffering (immediate output)
type StdoutWriter struct {
    encoder Encoder
}

// NewStdoutWriter creates stdout writer.
// CONTRACT: Returns initialized writer
func NewStdoutWriter(format string) (*StdoutWriter, error)
```

### FileWriter Implementation

**File**: `internal/logger/writer_file.go`

```go
// FileWriter writes logs to file with rotation.
//
// CONTRACT:
// - Uses lumberjack for rotation
// - Thread-safe via lumberjack
// - Automatic rotation based on size/age
// - Compressed backups
//
// DEPENDENCY: gopkg.in/natefinch/lumberjack.v2
type FileWriter struct {
    logger *lumberjack.Logger
    encoder Encoder
}

// NewFileWriter creates file writer with rotation.
// CONTRACT: Creates parent directories if needed
func NewFileWriter(config FileConfig, format string) (*FileWriter, error)
```

### SyslogWriter Implementation

**File**: `internal/logger/writer_syslog.go`

```go
// SyslogWriter writes logs to syslog.
//
// CONTRACT:
// - Maps log levels to syslog severity
// - Reconnects on connection failure
// - Thread-safe via syslog.Writer
//
// DEPENDENCY: log/syslog
type SyslogWriter struct {
    writer *syslog.Writer
}

// NewSyslogWriter creates syslog writer.
// CONTRACT: Returns error if connection fails
func NewSyslogWriter(config SyslogConfig) (*SyslogWriter, error)
```

---

## Testing Contract

### Mock Logger

**File**: `internal/logger/mock.go` (test helper)

```go
// MockLogger is a test double for Logger.
//
// CONTRACT:
// - Records all logged messages
// - Thread-safe for concurrent tests
// - Provides assertion helpers
type MockLogger struct {
    mu       sync.RWMutex
    messages []LoggedMessage
}

type LoggedMessage struct {
    Level   Level
    Message string
    Fields  []Field
    Context context.Context
}

// AssertLogged checks if message was logged.
// CONTRACT: Returns true if any message matches
func (m *MockLogger) AssertLogged(level Level, msg string) bool

// AssertField checks if field exists in logged messages.
// CONTRACT: Returns true if field found in any message
func (m *MockLogger) AssertField(key string, value interface{}) bool

// Reset clears recorded messages.
func (m *MockLogger) Reset()
```

### Test Patterns

```go
// Pattern 1: Use mock logger in tests
func TestHandler(t *testing.T) {
    mockLogger := logger.NewMockLogger()
    handler := NewHandler(mockLogger, ...)
    
    // ... execute handler
    
    assert.True(t, mockLogger.AssertLogged(logger.LevelInfo, "user created"))
    assert.True(t, mockLogger.AssertField("user_id", userID))
}

// Pattern 2: Use nop logger when logger not needed
type MinimalHandler struct {
    logger logger.Logger // Use NopLogger
}

func NewMinimalHandler() *MinimalHandler {
    return &MinimalHandler{
        logger: logger.NewNopLogger(),
    }
}

// Pattern 3: Extract and verify context fields
func TestContextPropagation(t *testing.T) {
    ctx := context.Background()
    ctx = logger.WithRequestID(ctx, "test-request-id")
    ctx = logger.WithUserID(ctx, "test-user-id")
    
    fields := logger.ExtractContextFields(ctx)
    
    assert.Contains(t, fields, logger.String("request_id", "test-request-id"))
    assert.Contains(t, fields, logger.String("user_id", "test-user-id"))
}
```

---

## Performance Contract

| Operation | Target | Maximum | Notes |
|-----------|--------|---------|-------|
| Debug() call | 50µs | 1ms | With context extraction |
| Info() call | 50µs | 1ms | With context extraction |
| Error() call | 100µs | 2ms | Includes stack trace |
| WithFields() | 10µs | 100µs | Creates new instance |
| Context extraction | 10µs | 500µs | All fields |
| Middleware overhead | 1ms | 2ms | Per request |

**Performance Guarantees:**
- Zero-allocation for constant keys (compiler optimization)
- Buffer pooling for JSON encoding
- Async writer option for high-throughput
- Sampling support for high-frequency logs

---

## Backward Compatibility Contract

### Migration Path

```go
// Phase 1: Existing code (still works)
slog.Info("message", slog.String("key", "value"))

// Phase 2: Mixed usage (both work)
logger := logger.FromContext(ctx)
logger.Info(ctx, "message", logger.String("key", "value"))
// OR: slog.Info(...) still works

// Phase 3: Full migration
// Only structured logger used
```

### Compatibility Guarantees

- Existing `slog` calls continue working
- No breaking changes to handler signatures
- Middleware can coexist with existing logging middleware
- Configuration defaults match existing behavior

---

## Error Handling Contract

| Scenario | Behavior | Rationale |
|----------|----------|-----------|
| nil context | Use context.Background() | Never panic |
| nil logger | Return NopLogger | Safe default |
| Write error | Log to stderr, continue | Don't crash app |
| Invalid config | Return error on init | Fail fast |
| Missing context fields | Omit from log | Graceful degradation |
| Panic in handler | Recover and log | Stability |

---

## Security Contract

| Requirement | Implementation |
|-------------|----------------|
| No PII in logs | LogRequestBody disabled by default |
| Sensitive data redaction | Configurable field redaction |
| Safe serialization | JSON encoding for all values |
| No stack trace leakage | Stack traces only in ERROR logs |
| Context isolation | Each request gets fresh context |

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-04-25 | Initial contract |

---

## Sign-off

- [ ] API Review: ___________
- [ ] Performance Review: ___________
- [ ] Security Review: ___________
- [ ] Implementation: ___________
