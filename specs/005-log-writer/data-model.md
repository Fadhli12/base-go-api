# Data Model: Structured Logging System

**Feature**: 005-log-writer - Structured Logging System  
**Date**: 2026-04-25  
**Status**: Ready for Implementation

---

## Entity Relationship Diagram

```go
// High-level structure
┌─────────────────────────────────────────────────────────────┐
│                     Logger Interface                        │
├─────────────────────────────────────────────────────────────┤
│  + Debug(ctx, msg, fields)                                  │
│  + Info(ctx, msg, fields)                                   │
│  + Warn(ctx, msg, fields)                                   │
│  + Error(ctx, msg, fields)                                  │
│  + WithFields(fields) Logger                                │
│  + WithError(err) Logger                                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   SlogLogger (Implementation)               │
├─────────────────────────────────────────────────────────────┤
│  - handler slog.Handler                                     │
│  - level slog.Level                                         │
│  - fields []Field                                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     MultiWriter                             │
├─────────────────────────────────────────────────────────────┤
│  + Write(ctx, entry) error                                  │
│  + Close() error                                            │
└─────────────────────────────────────────────────────────────┘
        │              │              │
        ▼              ▼              ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│StdoutWriter  │ │FileWriter    │ │SyslogWriter  │
└──────────────┘ └──────────────┘ └──────────────┘
```

---

## Core Entities

### 1. Logger Interface

**Location**: `internal/logger/logger.go`

```go
// Package logger provides structured logging abstractions.
package logger

import (
    "context"
    "time"
)

// Logger is the primary interface for structured logging.
// It provides level-based logging with context propagation and field support.
type Logger interface {
    // Core logging methods - MUST be implemented
    Debug(ctx context.Context, msg string, fields ...Field)
    Info(ctx context.Context, msg string, fields ...Field)
    Warn(ctx context.Context, msg string, fields ...Field)
    Error(ctx context.Context, msg string, fields ...Field)
    
    // Context-aware logging - MUST be implemented
    // Creates a new logger with additional fields that are included in all subsequent logs
    WithFields(fields ...Field) Logger
    
    // Creates a new logger with error context
    WithError(err error) Logger
    
    // Field factory methods - convenience helpers
    String(key, value string) Field
    Int(key string, value int) Field
    Int64(key string, value int64) Field
    Float64(key string, value float64) Field
    Bool(key string, value bool) Field
    Duration(key string, value time.Duration) Field
    Time(key string, value time.Time) Field
    Any(key string, value interface{}) Field
}

// Level represents logging levels compatible with slog
type Level int

const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
)

func (l Level) String() string {
    switch l {
    case LevelDebug:
        return "DEBUG"
    case LevelInfo:
        return "INFO"
    case LevelWarn:
        return "WARN"
    case LevelError:
        return "ERROR"
    default:
        return "UNKNOWN"
    }
}

// ParseLevel converts a string level to Level
type ParseLevel func(string) (Level, error)
```

### 2. Field Structure

**Location**: `internal/logger/field.go`

```go
// Field represents a structured logging field (key-value pair).
// It provides type-safe field creation with slog compatibility.
type Field struct {
    Key   string
    Value interface{}
}

// ToSlogAttr converts Field to slog.Attr for handler compatibility
func (f Field) ToSlogAttr() slog.Attr {
    switch v := f.Value.(type) {
    case string:
        return slog.String(f.Key, v)
    case int:
        return slog.Int(f.Key, v)
    case int64:
        return slog.Int64(f.Key, v)
    case float64:
        return slog.Float64(f.Key, v)
    case bool:
        return slog.Bool(f.Key, v)
    case time.Duration:
        return slog.Duration(f.Key, v)
    case time.Time:
        return slog.Time(f.Key, v)
    case error:
        return slog.String(f.Key, v.Error())
    default:
        return slog.Any(f.Key, v)
    }
}

// Field constructors

func String(key, value string) Field {
    return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
    return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
    return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
    return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
    return Field{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
    return Field{Key: key, Value: value}
}

func Time(key string, value time.Time) Field {
    return Field{Key: key, Value: value}
}

func Any(key string, value interface{}) Field {
    return Field{Key: key, Value: value}
}

func Err(err error) Field {
    if err == nil {
        return Field{Key: "error", Value: nil}
    }
    return Field{Key: "error", Value: err.Error()}
}
```

### 3. LogEntry Structure

**Location**: `internal/logger/entry.go`

```go
// LogEntry represents a single log entry for writer implementations.
// This is the internal structure passed to writers.
type LogEntry struct {
    // Core fields (always present)
    Timestamp time.Time              `json:"timestamp"`
    Level     string                 `json:"level"`
    Message   string                 `json:"message"`
    
    // Context fields (populated from context, may be empty)
    RequestID string                 `json:"request_id,omitempty"`
    UserID    string                 `json:"user_id,omitempty"`
    OrgID     string                 `json:"org_id,omitempty"`
    TraceID   string                 `json:"trace_id,omitempty"`
    
    // Structured fields (custom key-value pairs)
    Fields    map[string]interface{} `json:"fields,omitempty"`
    
    // Error information
    Error     string                 `json:"error,omitempty"`
    StackTrace string               `json:"stack_trace,omitempty"`
    
    // Source information (optional, for debugging)
    Source    *SourceInfo            `json:"source,omitempty"`
}

// SourceInfo contains source code location
type SourceInfo struct {
    File string `json:"file"`
    Line int    `json:"line"`
    Function string `json:"function,omitempty"`
}
```

---

## Context Keys

**Location**: `internal/logger/context.go`

```go
// Package logger provides structured logging abstractions.
package logger

import (
    "context"
    
    "github.com/google/uuid"
)

// Context keys for logger-related values
// Using custom types to avoid collisions with other packages
type contextKey string

const (
    // LoggerContextKey is the key for storing logger in context
    LoggerContextKey contextKey = "logger"
    
    // RequestIDContextKey is the key for request ID
    RequestIDContextKey contextKey = "request_id"
    
    // UserIDContextKey is the key for user ID (from JWT)
    UserIDContextKey contextKey = "user_id"
    
    // OrgIDContextKey is the key for organization ID
    OrgIDContextKey contextKey = "org_id"
    
    // TraceIDContextKey is the key for trace ID (OpenTelemetry compatible)
    TraceIDContextKey contextKey = "trace_id"
    
    // StartTimeContextKey is the key for request start time (for duration calculation)
    StartTimeContextKey contextKey = "start_time"
)

// Header names for incoming requests
const (
    RequestIDHeader = "X-Request-ID"
    TraceIDHeader   = "X-Trace-ID"
    OrgIDHeader     = "X-Organization-ID"
)

// Context operations

// WithLogger injects a logger into the context
func WithLogger(ctx context.Context, logger Logger) context.Context {
    return context.WithValue(ctx, LoggerContextKey, logger)
}

// FromContext extracts the logger from context.
// Returns a no-op logger if not found (never returns nil)
func FromContext(ctx context.Context) Logger {
    if logger, ok := ctx.Value(LoggerContextKey).(Logger); ok {
        return logger
    }
    return NewNopLogger()
}

// WithRequestID injects request ID into context
func WithRequestID(ctx context.Context, requestID string) context.Context {
    return context.WithValue(ctx, RequestIDContextKey, requestID)
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value(RequestIDContextKey).(string); ok {
        return id
    }
    return ""
}

// WithUserID injects user ID into context
func WithUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, UserIDContextKey, userID)
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
    if id, ok := ctx.Value(UserIDContextKey).(string); ok {
        return id
    }
    return ""
}

// WithOrgID injects organization ID into context
func WithOrgID(ctx context.Context, orgID string) context.Context {
    return context.WithValue(ctx, OrgIDContextKey, orgID)
}

// GetOrgID extracts organization ID from context
func GetOrgID(ctx context.Context) string {
    if id, ok := ctx.Value(OrgIDContextKey).(string); ok {
        return id
    }
    return ""
}

// WithTraceID injects trace ID into context
func WithTraceID(ctx context.Context, traceID string) context.Context {
    return context.WithValue(ctx, TraceIDContextKey, traceID)
}

// GetTraceID extracts trace ID from context
func GetTraceID(ctx context.Context) string {
    if id, ok := ctx.Value(TraceIDContextKey).(string); ok {
        return id
    }
    return ""
}

// WithStartTime injects request start time into context
func WithStartTime(ctx context.Context) context.Context {
    return context.WithValue(ctx, StartTimeContextKey, time.Now())
}

// GetStartTime extracts start time from context
func GetStartTime(ctx context.Context) (time.Time, bool) {
    if t, ok := ctx.Value(StartTimeContextKey).(time.Time); ok {
        return t, true
    }
    return time.Time{}, false
}

// ExtractContextFields extracts all context fields into a slice of Fields
// Used by logger implementations to automatically include context
func ExtractContextFields(ctx context.Context) []Field {
    var fields []Field
    
    if requestID := GetRequestID(ctx); requestID != "" {
        fields = append(fields, String("request_id", requestID))
    }
    
    if userID := GetUserID(ctx); userID != "" {
        fields = append(fields, String("user_id", userID))
    }
    
    if orgID := GetOrgID(ctx); orgID != "" {
        fields = append(fields, String("org_id", orgID))
    }
    
    if traceID := GetTraceID(ctx); traceID != "" {
        fields = append(fields, String("trace_id", traceID))
    }
    
    return fields
}

// GenerateRequestID generates a new request ID
func GenerateRequestID() string {
    return uuid.New().String()
}

// GenerateTraceID generates a new trace ID (OpenTelemetry compatible)
func GenerateTraceID() string {
    return uuid.New().String()
}
```

---

## Writer Interface

**Location**: `internal/logger/writer.go`

```go
// Writer is the interface for log output destinations.
// Implementations must be goroutine-safe.
type Writer interface {
    // Write writes a log entry to the destination.
    // Implementations should not block indefinitely.
    Write(ctx context.Context, entry LogEntry) error
    
    // Close closes the writer and releases resources.
    // Should flush any buffered data.
    Close() error
}

// WriterFunc is an adapter to allow the use of ordinary functions as Writers
type WriterFunc func(ctx context.Context, entry LogEntry) error

func (f WriterFunc) Write(ctx context.Context, entry LogEntry) error {
    return f(ctx, entry)
}

func (f WriterFunc) Close() error {
    return nil
}

// MultiWriter writes to multiple writers
type MultiWriter struct {
    writers []Writer
}

// NewMultiWriter creates a new multi-writer
func NewMultiWriter(writers ...Writer) *MultiWriter {
    return &MultiWriter{writers: writers}
}

// Write writes to all writers. Errors are collected but not returned
// to prevent one failing writer from breaking logging.
func (m *MultiWriter) Write(ctx context.Context, entry LogEntry) error {
    for _, w := range m.writers {
        if err := w.Write(ctx, entry); err != nil {
            // Log error to stderr but don't fail
            fmt.Fprintf(os.Stderr, "log writer error: %v\n", err)
        }
    }
    return nil
}

// Close closes all writers
func (m *MultiWriter) Close() error {
    var errs []error
    for _, w := range m.writers {
        if err := w.Close(); err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return fmt.Errorf("close errors: %v", errs)
    }
    return nil
}
```

---

## Configuration Structure

**Location**: `internal/logger/config.go`

```go
// Config holds logger configuration
type Config struct {
    // Level is the minimum log level (debug, info, warn, error)
    Level string `json:"level" yaml:"level" env:"LOG_LEVEL"`
    
    // Format is the output format (json, text)
    Format string `json:"format" yaml:"format" env:"LOG_FORMAT"`
    
    // Outputs is the list of output destinations (stdout, file, syslog)
    Outputs []string `json:"outputs" yaml:"outputs" env:"LOG_OUTPUTS" envSeparator:","`
    
    // File configuration (when outputs includes "file")
    File FileConfig `json:"file" yaml:"file"`
    
    // Syslog configuration (when outputs includes "syslog")
    Syslog SyslogConfig `json:"syslog" yaml:"syslog"`
    
    // AddSource includes source file:line in logs
    AddSource bool `json:"add_source" yaml:"add_source" env:"LOG_ADD_SOURCE"`
}

// FileConfig holds file writer configuration
type FileConfig struct {
    // Path is the log file path
    Path string `json:"path" yaml:"path" env:"LOG_FILE_PATH"`
    
    // MaxSize is the maximum file size in MB before rotation
    MaxSize int `json:"max_size" yaml:"max_size" env:"LOG_FILE_MAX_SIZE"`
    
    // MaxBackups is the maximum number of backup files to keep
    MaxBackups int `json:"max_backups" yaml:"max_backups" env:"LOG_FILE_MAX_BACKUPS"`
    
    // MaxAge is the maximum age of backup files in days
    MaxAge int `json:"max_age" yaml:"max_age" env:"LOG_FILE_MAX_AGE"`
    
    // Compress determines if rotated files should be compressed
    Compress bool `json:"compress" yaml:"compress" env:"LOG_FILE_COMPRESS"`
}

// SyslogConfig holds syslog writer configuration
type SyslogConfig struct {
    // Network is the network type (tcp, udp, or empty for local)
    Network string `json:"network" yaml:"network" env:"LOG_SYSLOG_NETWORK"`
    
    // Address is the syslog server address
    Address string `json:"address" yaml:"address" env:"LOG_SYSLOG_ADDRESS"`
    
    // Tag is the syslog tag
    Tag string `json:"tag" yaml:"tag" env:"LOG_SYSLOG_TAG"`
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
    return Config{
        Level:   "info",
        Format:  "json",
        Outputs: []string{"stdout"},
        File: FileConfig{
            Path:       "/var/log/api.log",
            MaxSize:    100,
            MaxBackups: 10,
            MaxAge:     30,
            Compress:   true,
        },
        Syslog: SyslogConfig{
            Network: "",
            Address: "",
            Tag:     "go-api",
        },
        AddSource: false,
    }
}

// Validate validates the configuration
func (c Config) Validate() error {
    validLevels := map[string]bool{
        "debug": true,
        "info":  true,
        "warn":  true,
        "error": true,
    }
    
    if !validLevels[c.Level] {
        return fmt.Errorf("invalid log level: %s", c.Level)
    }
    
    validFormats := map[string]bool{
        "json": true,
        "text": true,
    }
    
    if !validFormats[c.Format] {
        return fmt.Errorf("invalid log format: %s", c.Format)
    }
    
    validOutputs := map[string]bool{
        "stdout": true,
        "file":   true,
        "syslog": true,
    }
    
    for _, output := range c.Outputs {
        if !validOutputs[output] {
            return fmt.Errorf("invalid output: %s", output)
        }
    }
    
    return nil
}

// ParseLevel converts config level string to Level
func (c Config) ParseLevel() Level {
    switch c.Level {
    case "debug":
        return LevelDebug
    case "info":
        return LevelInfo
    case "warn":
        return LevelWarn
    case "error":
        return LevelError
    default:
        return LevelInfo
    }
}
```

---

## Middleware Integration Model

**Location**: `internal/http/middleware/structured_logging.go` (planned)

```go
// LoggingMiddlewareConfig holds configuration for structured logging middleware
type LoggingMiddlewareConfig struct {
    // Logger is the logger instance to use
    Logger logger.Logger
    
    // Skipper defines a function to skip logging for certain requests
    Skipper func(c echo.Context) bool
    
    // LogRequestBody determines if request body should be logged (careful with PII)
    LogRequestBody bool
    
    // LogResponseBody determines if response body should be logged
    LogResponseBody bool
    
    // BodyMaxSize is the maximum body size to log (in bytes)
    BodyMaxSize int
}

// DefaultLoggingMiddlewareConfig returns sensible defaults
func DefaultLoggingMiddlewareConfig(logger logger.Logger) LoggingMiddlewareConfig {
    return LoggingMiddlewareConfig{
        Logger:          logger,
        Skipper:         DefaultLoggingSkipper(),
        LogRequestBody:  false,
        LogResponseBody: false,
        BodyMaxSize:     1024,
    }
}

// DefaultLoggingSkipper skips health checks and metrics
func DefaultLoggingSkipper() func(c echo.Context) bool {
    return func(c echo.Context) bool {
        path := c.Request().URL.Path
        return path == "/healthz" || 
               path == "/readyz" || 
               path == "/metrics" ||
               path == "/swagger/*"
    }
}
```

---

## Entity Relationships

```
┌─────────────────────┐         ┌─────────────────────┐
│   Logger Interface  │◄────────│  SlogLogger (impl)  │
└─────────────────────┘         └─────────────────────┘
          │                              │
          │ uses                         │ uses
          ▼                              ▼
┌─────────────────────┐         ┌─────────────────────┐
│      Field          │         │    slog.Handler     │
└─────────────────────┘         └─────────────────────┘
                                          │
                                          │ writes to
                                          ▼
                                 ┌─────────────────────┐
                                 │   MultiWriter       │
                                 └─────────────────────┘
                                          │
                         ┌────────────────┼────────────────┐
                         ▼                ▼                ▼
                 ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
                 │StdoutWriter  │  │FileWriter    │  │SyslogWriter  │
                 └──────────────┘  └──────────────┘  └──────────────┘
```

---

## Integration Points

### 1. With Echo Context

```go
// In middleware
c.Set("logger", logger)

// In handler
logger := c.Get("logger").(logger.Logger)
```

### 2. With Go Context

```go
// In middleware (Echo → Go context propagation)
ctx := logger.WithRequestID(c.Request().Context(), requestID)
ctx = logger.WithUserID(ctx, userID)
c.SetRequest(c.Request().WithContext(ctx))

// In handler
logger := logger.FromContext(c.Request().Context())
```

### 3. With Existing Audit System

```go
// Both logger and audit capture context
logger.Info(ctx, "user updated", logger.String("user_id", id))
auditSvc.Log(ctx, AuditEntry{Action: "user:update", ...})
```

---

## Data Validation

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| Level | string | debug, info, warn, error | Case-insensitive |
| Format | string | json, text | json recommended for production |
| RequestID | string | UUID v4 | Generated if not provided |
| UserID | string | UUID v4 | From JWT claims |
| OrgID | string | UUID v4 | From X-Organization-ID header |
| TraceID | string | UUID v4 | From X-Trace-ID header or generated |
| Timestamp | time.Time | RFC3339 | UTC timezone |
| Duration | int64 | milliseconds | Calculated from start time |

---

## Migration Path

### From Existing slog

```go
// Before
slog.Info("user created", slog.String("id", userID))

// After (backward compatible)
logger := logger.FromContext(ctx)
logger.Info(ctx, "user created", logger.String("id", userID))
```

### From existing logging middleware

The new middleware will replace `internal/http/middleware/logging.go` while maintaining backward compatibility:
- Same log format
- Same request/response logging
- Enhanced with context fields

---

## Success Criteria Mapping

| Requirement | Data Model Support |
|-------------|-------------------|
| Logger interface | `Logger` interface defined |
| Structured fields | `Field` struct with type safety |
| Context propagation | Context keys + helper functions |
| Request ID extraction | `GetRequestID`, `WithRequestID` |
| User ID extraction | `GetUserID`, `WithUserID` |
| Org ID extraction | `GetOrgID`, `WithOrgID` |
| Trace ID support | `GetTraceID`, `WithTraceID` |
| Multiple writers | `Writer` interface + `MultiWriter` |
| Log levels | `Level` enum with string conversion |
| Configuration | `Config` struct with validation |

---

## Notes

1. **No DB storage**: Logs are output to writers (stdout/file/syslog), not stored in database
2. **Goroutine-safe**: All implementations must be safe for concurrent use
3. **Zero-allocation**: Field creation should minimize allocations where possible
4. **Context-first**: All logger methods accept context as first parameter
5. **Never nil logger**: `FromContext` returns NopLogger if not found (safe default)
