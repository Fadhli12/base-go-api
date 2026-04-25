# Implementation Tasks: Structured Logging System (005-log-writer)

**Branch**: 005-log-writer  
**Feature**: Structured Logging System  
**Generated**: 2026-04-25

---

## Overview

This document breaks down the 7-phase implementation plan into atomic, executable tasks. Each task:
- Takes 1-2 hours maximum
- Has clear success criteria
- Lists exact files to create/modify
- Specifies dependencies (must complete before dependent tasks)
- Includes verification steps

**Total Tasks**: 25  
**Estimated Duration**: 28-30 hours  
**Phases**: 7

---

## Phase 1: Logger Interface & slog Implementation

### T1.1: Create Logger Interface Core

**ID**: T1.1  
**Phase**: 1  
**Estimated Duration**: 1.5 hours  
**Dependencies**: None

**Description**:
Create the core Logger interface with level enum and field structure. This is the foundation that all other components depend on.

**Files to Create**:
- `internal/logger/logger.go` - Logger interface, Level enum, ParseLevel function

**Content Requirements**:
```go
// Logger interface with Debug, Info, Warn, Error methods
// All methods accept context.Context as first parameter
// Level enum: LevelDebug, LevelInfo, LevelWarn, LevelError
// ParseLevel function for string to Level conversion
// Level.String() method for Level to string conversion
```

**Imports Required**:
```go
import (
    "context"
    "fmt"
    "time"
)
```

**Success Criteria**:
- [ ] Logger interface defined with all 4 level methods
- [ ] Level enum with proper iota values
- [ ] ParseLevel function handles all valid level strings
- [ ] Level.String() returns uppercase strings
- [ ] Package-level documentation added

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go build ./internal/logger/...
go test ./internal/logger/... -v -run TestLevel
golangci-lint run ./internal/logger/...
```

**Patterns from Codebase**:
- Follow pattern from `internal/permission/enforcer.go` for interface design
- Similar to `internal/cache/driver.go` Cache interface pattern

---

### T1.2: Implement Field Structure and Constructors

**ID**: T1.2  
**Phase**: 1  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T1.1

**Description**:
Create the Field struct and all field constructors. Fields are the building blocks of structured logging.

**Files to Create**:
- `internal/logger/field.go` - Field struct and constructors

**Content Requirements**:
```go
// Field struct with Key and Value fields
// Field.ToSlogAttr() method for slog compatibility
// Package-level constructors:
//   - String(key, value string) Field
//   - Int(key string, value int) Field
//   - Int64(key string, value int64) Field
//   - Float64(key string, value float64) Field
//   - Bool(key string, value bool) Field
//   - Duration(key string, value time.Duration) Field
//   - Time(key string, value time.Time) Field
//   - Any(key string, value interface{}) Field
//   - Err(err error) Field
```

**Imports Required**:
```go
import (
    "log/slog"
    "time"
)
```

**Success Criteria**:
- [ ] Field struct defined with proper fields
- [ ] ToSlogAttr() converts all supported types
- [ ] All 9 field constructors implemented
- [ ] Err() handles nil errors gracefully
- [ ] All constructors return Field by value (not pointer)

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestField
go test ./internal/logger/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep field.go
```

---

### T1.3: Implement Context Operations

**ID**: T1.3  
**Phase**: 1  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T1.1, T1.2

**Description**:
Create context keys and operations for propagating logger and context fields through request lifecycle.

**Files to Create**:
- `internal/logger/context.go` - Context keys and helper functions

**Content Requirements**:
```go
// Custom contextKey type to avoid collisions
// Context keys:
//   - LoggerContextKey
//   - RequestIDContextKey
//   - UserIDContextKey
//   - OrgIDContextKey
//   - TraceIDContextKey
//   - StartTimeContextKey
// Header constants:
//   - RequestIDHeader = "X-Request-ID"
//   - TraceIDHeader = "X-Trace-ID"
//   - OrgIDHeader = "X-Organization-ID"
// Functions:
//   - WithLogger/GetLogger
//   - WithRequestID/GetRequestID
//   - WithUserID/GetUserID
//   - WithOrgID/GetOrgID
//   - WithTraceID/GetTraceID
//   - WithStartTime/GetStartTime
//   - ExtractContextFields (returns []Field)
//   - GenerateRequestID/GenerateTraceID (UUID)
```

**Imports Required**:
```go
import (
    "context"
    "time"
    
    "github.com/google/uuid"
)
```

**Success Criteria**:
- [ ] contextKey type defined with underlying string
- [ ] All 6 context keys defined as constants
- [ ] 3 header constants defined
- [ ] All With*/Get* function pairs implemented
- [ ] ExtractContextFields returns []Field with all set context values
- [ ] GenerateRequestID/GenerateTraceID use uuid.New().String()
- [ ] Get functions return zero values (not panic) when key not found

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestContext
go test ./internal/logger/... -race -run TestContext
```

**Patterns from Codebase**:
- Similar to `middleware/request_id.go` RequestIDContextKey pattern
- Follow `middleware/jwt.go` GetUserClaims pattern for context extraction

---

### T1.4: Implement slog-based Logger

**ID**: T1.4  
**Phase**: 1  
**Estimated Duration**: 2 hours  
**Dependencies**: T1.1, T1.2, T1.3

**Description**:
Create the SlogLogger implementation that wraps slog.Handler with our Logger interface.

**Files to Create**:
- `internal/logger/slog_logger.go` - SlogLogger implementation
- `internal/logger/entry.go` - LogEntry struct

**Content Requirements**:
```go
// LogEntry struct with all log entry fields
// SlogLogger struct with handler, level, fields
// NewSlogLogger(handler slog.Handler, level Level) *SlogLogger
// All Logger interface methods implemented:
//   - Debug/Info/Warn/Error with level checking
//   - WithFields returns new SlogLogger with merged fields
//   - WithError returns new SlogLogger with error field
//   - All field factory methods (String, Int, etc.)
// Helper: contextToSlogAttrs converts context fields to slog.Attr
```

**Imports Required**:
```go
import (
    "context"
    "log/slog"
    "time"
)
```

**Success Criteria**:
- [ ] LogEntry struct defined with JSON tags
- [ ] SlogLogger implements Logger interface
- [ ] Level filtering works (Debug skipped when level=Info)
- [ ] WithFields creates new instance, doesn't modify original
- [ ] Context fields automatically extracted and logged
- [ ] Field factory methods return Field (not call slog directly)

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestSlogLogger
go test ./internal/logger/... -v -run TestLevelFiltering
go test ./internal/logger/... -race
```

---

## Phase 2: Output Writers & Configuration

### T2.1: Create Writer Interface and MultiWriter

**ID**: T2.1  
**Phase**: 2  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T1.4

**Description**:
Create the Writer interface and MultiWriter implementation for supporting multiple output destinations.

**Files to Create**:
- `internal/logger/writer.go` - Writer interface and MultiWriter

**Content Requirements**:
```go
// Writer interface with Write(ctx, entry) and Close() methods
// WriterFunc adapter type
// MultiWriter struct with []Writer slice
// NewMultiWriter(writers ...Writer) *MultiWriter
// MultiWriter.Write - iterates all writers, logs errors to stderr
// MultiWriter.Close - closes all writers, aggregates errors
```

**Imports Required**:
```go
import (
    "context"
    "fmt"
    "os"
)
```

**Success Criteria**:
- [ ] Writer interface defined
- [ ] WriterFunc implements Writer interface
- [ ] MultiWriter created with variadic constructor
- [ ] MultiWriter.Write calls all writers, continues on error
- [ ] MultiWriter.Close aggregates errors from all writers
- [ ] Thread-safe (no data races)

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestWriter
go test ./internal/logger/... -v -run TestMultiWriter
go test ./internal/logger/... -race -run TestMultiWriter
```

---

### T2.2: Implement Stdout Writer

**ID**: T2.2  
**Phase**: 2  
**Estimated Duration**: 1 hour  
**Dependencies**: T2.1

**Description**:
Create the StdoutWriter for console output with JSON/text format support.

**Files to Create**:
- `internal/logger/writer_stdout.go` - Stdout writer

**Content Requirements**:
```go
// StdoutWriter struct with encoder
// Encoder interface for JSON/text encoding
// NewStdoutWriter(format string) (*StdoutWriter, error)
// StdoutWriter.Write encodes entry and writes to stdout
// StdoutWriter.Close is no-op (returns nil)
// JSON encoder uses json.Marshal
// Text encoder uses fmt.Fprintf with template
```

**Imports Required**:
```go
import (
    "context"
    "encoding/json"
    "fmt"
    "os"
)
```

**Success Criteria**:
- [ ] StdoutWriter implements Writer interface
- [ ] Supports "json" and "text" formats
- [ ] JSON output is valid JSON with all fields
- [ ] Text output is human-readable
- [ ] Errors return error (don't panic)

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestStdoutWriter
go test ./internal/logger/... -v -run TestJSONFormat
go test ./internal/logger/... -v -run TestTextFormat
```

---

### T2.3: Implement File Writer with Rotation

**ID**: T2.3  
**Phase**: 2  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T2.1

**Description**:
Create FileWriter with automatic log rotation using lumberjack.

**Files to Create**:
- `internal/logger/writer_file.go` - File writer with rotation

**Content Requirements**:
```go
// FileWriter struct with *lumberjack.Logger and encoder
// FileConfig struct (already in config.go, reuse)
// NewFileWriter(config FileConfig, format string) (*FileWriter, error)
// Creates parent directories if needed
// FileWriter.Write encodes and writes to lumberjack
// FileWriter.Close closes lumberjack logger
```

**Imports Required**:
```go
import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    
    "gopkg.in/natefinch/lumberjack.v2"
)
```

**Success Criteria**:
- [ ] FileWriter implements Writer interface
- [ ] Creates parent directories on init
- [ ] Uses lumberjack for rotation
- [ ] Respects FileConfig (MaxSize, MaxBackups, MaxAge, Compress)
- [ ] Thread-safe via lumberjack

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestFileWriter
go test ./internal/logger/... -v -run TestFileRotation
# Manual: Check temp file created and rotated
```

**Dependencies External**:
- `gopkg.in/natefinch/lumberjack.v2` (already in go.mod)

---

### T2.4: Implement Configuration and Factory

**ID**: T2.4  
**Phase**: 2  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T2.1, T2.2, T2.3

**Description**:
Create configuration structures and factory for creating loggers from config.

**Files to Create/Modify**:
- `internal/logger/config.go` - Logger configuration structs
- `internal/logger/factory.go` - Logger factory
- `internal/config/config.go` - Modify to add logger config (add fields only)

**Content Requirements**:
```go
// Config struct with Level, Format, Outputs, File, Syslog, AddSource
// FileConfig struct with Path, MaxSize, MaxBackups, MaxAge, Compress
// SyslogConfig struct with Network, Address, Tag
// DefaultConfig() returns sensible defaults
// Config.Validate() validates all fields
// Config.ParseLevel() converts level string to Level

// Factory:
// NewFromConfig(config Config) (Logger, error)
// createWriters(outputs []string, config Config) ([]Writer, error)
// createHandler(writer Writer, format string, addSource bool) slog.Handler
```

**Imports Required**:
```go
import (
    "fmt"
    "log/slog"
    "os"
)
```

**Success Criteria**:
- [ ] Config struct defined with all fields
- [ ] DefaultConfig returns sensible defaults
- [ ] Validate returns error for invalid level/format/outputs
- [ ] ParseLevel returns LevelInfo for unknown values
- [ ] NewFromConfig creates logger with multiple writers
- [ ] Supports "stdout", "file", "syslog" outputs

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestConfig
go test ./internal/logger/... -v -run TestFactory
go test ./internal/logger/... -v -run TestValidation
```

**Integration Point**:
- Modify `internal/config/config.go` to add LogConfig fields (don't replace existing Log struct)

---

## Phase 3: Middleware Integration

### T3.1: Create Structured Logging Middleware

**ID**: T3.1  
**Phase**: 3  
**Estimated Duration**: 2 hours  
**Dependencies**: T1.3, T2.4

**Description**:
Create the Echo middleware that integrates structured logging into request lifecycle.

**Files to Create**:
- `internal/http/middleware/structured_logging.go` - Middleware implementation

**Content Requirements**:
```go
// LoggingMiddlewareConfig struct with Logger, Skipper, LogRequestBody, LogResponseBody, BodyMaxSize
// DefaultLoggingMiddlewareConfig(logger Logger) LoggingMiddlewareConfig
// DefaultLoggingSkipper() func(echo.Context) bool - skips /healthz, /readyz, /metrics
// StructuredLogging(config LoggingMiddlewareConfig) echo.MiddlewareFunc
// GetLogger(c echo.Context) Logger - retrieves logger from context
// 
// Middleware behavior:
//   - Extract/generate request_id
//   - Extract user_id from JWT claims
//   - Extract org_id from X-Organization-ID header
//   - Extract/generate trace_id
//   - Create context with all fields
//   - Log request start (DEBUG)
//   - Log request complete (INFO) with duration, status
//   - Log errors (ERROR) with stack trace
//   - Inject logger into echo context
```

**Imports Required**:
```go
import (
    "time"
    
    "github.com/example/go-api-base/internal/logger"
    "github.com/labstack/echo/v4"
)
```

**Success Criteria**:
- [ ] LoggingMiddlewareConfig defined
- [ ] DefaultLoggingSkipper skips health/metrics endpoints
- [ ] StructuredLogging middleware implements echo.MiddlewareFunc
- [ ] Request ID extracted from header or generated
- [ ] User ID extracted from JWT claims via GetUserClaims
- [ ] Org ID extracted from X-Organization-ID header
- [ ] Trace ID extracted or generated
- [ ] Request logged at DEBUG level on start
- [ ] Request logged at INFO level on completion with duration
- [ ] Errors logged at ERROR level with stack trace
- [ ] Logger injected into echo context

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/http/middleware/... -v -run TestStructuredLogging
go test ./internal/http/middleware/... -race -run TestStructuredLogging
```

**Patterns from Codebase**:
- Follow `middleware/logging.go` request/response pattern
- Use `middleware/jwt.go` GetUserClaims for user extraction
- Similar to `middleware/request_id.go` for ID generation

---

### T3.2: Implement NopLogger for Safe Defaults

**ID**: T3.2  
**Phase**: 3  
**Estimated Duration**: 1 hour  
**Dependencies**: T1.1

**Description**:
Create NopLogger for use when no logger is configured or for testing.

**Files to Create**:
- `internal/logger/nop_logger.go` - No-op logger implementation

**Content Requirements**:
```go
// NopLogger struct (empty)
// NewNopLogger() Logger - returns nop logger instance
// All Logger methods implemented as no-ops
// WithFields returns self (same instance)
// WithError returns self (same instance)
// Field methods return empty Field{}
```

**Imports Required**:
```go
import (
    "context"
    "time"
)
```

**Success Criteria**:
- [ ] NopLogger implements Logger interface
- [ ] All logging methods are no-ops (no output)
- [ ] WithFields/WithError return same instance
- [ ] Safe to use when logger not configured
- [ ] FromContext returns NopLogger when no logger in context

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestNopLogger
go test ./internal/logger/... -v -run TestFromContext
```

---

### T3.3: Create MockLogger for Testing

**ID**: T3.3  
**Phase**: 3  
**Estimated Duration**: 1 hour  
**Dependencies**: T1.1, T1.2

**Description**:
Create MockLogger for unit testing handlers and services.

**Files to Create**:
- `internal/logger/mock.go` - Mock logger for testing

**Content Requirements**:
```go
// MockLogger struct with:
//   - mu sync.RWMutex
//   - messages []LoggedMessage
// LoggedMessage struct with Level, Message, Fields, Context
// NewMockLogger() *MockLogger
// All Logger methods implemented (records to messages)
// AssertLogged(level Level, msg string) bool
// AssertField(key string, value interface{}) bool
// GetMessages() []LoggedMessage
// Reset() clears messages
```

**Imports Required**:
```go
import (
    "context"
    "sync"
    "time"
)
```

**Success Criteria**:
- [ ] MockLogger implements Logger interface
- [ ] Thread-safe with mutex
- [ ] AssertLogged returns true if message logged
- [ ] AssertField returns true if field found in any message
- [ ] GetMessages returns copy of messages
- [ ] Reset clears all recorded messages

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./internal/logger/... -v -run TestMockLogger
go test ./internal/logger/... -race -run TestMockLogger
```

---

## Phase 4: Application Integration

### T4.1: Update Config for Logger Integration

**ID**: T4.1  
**Phase**: 4  
**Estimated Duration**: 1 hour  
**Dependencies**: T2.4

**Description**:
Extend existing Config struct with logger configuration fields.

**Files to Modify**:
- `internal/config/config.go` - Add logger config fields

**Content Requirements**:
```go
// Extend existing LogConfig in config.go:
type LogConfig struct {
    Level       string       // Existing
    Format      string       // New: json/text
    Outputs     []string     // New: comma-separated list
    FilePath    string       // New: log file path
    MaxSize     int          // New: MB
    MaxBackups  int          // New: count
    MaxAge      int          // New: days
    Compress    bool         // New: compress backups
    SyslogNetwork string     // New: tcp/udp
    SyslogAddress string     // New: host:port
    SyslogTag     string     // New: tag
    AddSource   bool         // New: include file:line
}

// Add to Config struct:
// Log LogConfig (already exists, extend it)

// Add to setDefaults():
// v.SetDefault("log.format", "json")
// v.SetDefault("log.outputs", "stdout")
// etc.

// Add to parseLogConfig():
// Parse all new fields from environment
```

**Success Criteria**:
- [ ] LogConfig extended with all new fields
- [ ] All new fields have defaults in setDefaults()
- [ ] parseLogConfig() parses all new fields
- [ ] Config validation includes logger config
- [ ] Backward compatible (existing code still works)

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go build ./internal/config/...
go test ./internal/config/... -v
go test ./... -run TestConfig 2>&1 | head -20
```

---

### T4.2: Integrate Logger into Server Initialization

**ID**: T4.2  
**Phase**: 4  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T3.1, T4.1

**Description**:
Initialize logger in server startup and integrate into middleware chain.

**Files to Modify**:
- `internal/http/server.go` - Add logger to Server struct and middleware chain
- `cmd/api/main.go` - Create logger and pass to server

**Content Requirements**:
```go
// In internal/http/server.go:
// Add Logger field to Server struct
// Update NewServer to accept Logger parameter
// Add StructuredLogging to middleware chain
// Middleware order: Recover -> RequestID -> StructuredLogging -> CORS -> RateLimit -> JWT -> ...

// In cmd/api/main.go:
// Load logger config from Config.Log
// Create logger using logger.NewFromConfig()
// Pass logger to NewServer()
// Add graceful shutdown for logger (flush writers)
```

**Success Criteria**:
- [ ] Server struct has Logger field
- [ ] NewServer accepts Logger parameter
- [ ] StructuredLogging middleware added to chain
- [ ] Middleware order documented
- [ ] Logger initialized in main.go
- [ ] Graceful shutdown flushes logger

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go build ./cmd/api/...
go build ./internal/http/...
go test ./internal/http/... -v -run TestServer
```

**Patterns from Codebase**:
- Follow `internal/http/server.go` Server struct pattern
- Similar to graceful shutdown in `cmd/api/main.go`

---

### T4.3: Update .env.example with Logger Config

**ID**: T4.3  
**Phase**: 4  
**Estimated Duration**: 0.5 hours  
**Dependencies**: T4.1

**Description**:
Add logger environment variables to .env.example file.

**Files to Modify**:
- `.env.example` - Add logger configuration

**Content Requirements**:
```bash
# Logging Configuration
LOG_LEVEL=info                    # debug, info, warn, error
LOG_FORMAT=json                   # json, text
LOG_OUTPUTS=stdout                # stdout, file, syslog (comma-separated)
LOG_FILE_PATH=/var/log/api.log   # Required if outputs includes "file"
LOG_FILE_MAX_SIZE=100             # MB before rotation
LOG_FILE_MAX_BACKUPS=10
LOG_FILE_MAX_AGE=30               # days
LOG_FILE_COMPRESS=true
LOG_SYSLOG_NETWORK=tcp            # tcp, udp, or "" for local
LOG_SYSLOG_ADDRESS=localhost:514
LOG_SYSLOG_TAG=go-api
LOG_ADD_SOURCE=false              # Include file:line in logs
```

**Success Criteria**:
- [ ] All environment variables documented
- [ ] Default values shown
- [ ] Comments explain valid values
- [ ] File is valid shell syntax

**Verification Steps**:
```bash
cd C:\Development\base\go-api
# Check file syntax
cat .env.example | grep -E "^LOG_" | wc -l  # Should show 11 lines
```

---

## Phase 5: Handler Integration

### T5.1: Update Auth Handler for Structured Logging

**ID**: T5.1  
**Phase**: 5  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T3.1, T4.2

**Description**:
Update AuthHandler to use structured logging (highest traffic endpoint).

**Files to Modify**:
- `internal/http/handler/auth.go` - Add structured logging

**Content Requirements**:
```go
// Import logger package
// Update handler methods to:
//   - Get logger from context: log := middleware.GetLogger(c)
//   - Log Register attempts with email (no password!)
//   - Log Login attempts with email (no password!)
//   - Log errors with context
//   - Log successful operations
// Security: Never log passwords or tokens
```

**Success Criteria**:
- [ ] All auth endpoints use structured logging
- [ ] Login attempts logged with email
- [ ] Registration attempts logged with email
- [ ] Errors logged with context
- [ ] No passwords or tokens in logs
- [ ] Backward compatible (still works with old tests)

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go build ./internal/http/handler/...
go test ./tests/unit/... -run TestAuth -v
```

---

### T5.2: Update User Handler for Structured Logging

**ID**: T5.2  
**Phase**: 5  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T3.1, T4.2

**Description**:
Update UserHandler to use structured logging for CRUD operations.

**Files to Modify**:
- `internal/http/handler/user.go` - Add structured logging

**Content Requirements**:
```go
// Import logger package
// Update handler methods to:
//   - Get logger from context
//   - Log GetCurrentUser with user_id
//   - Log GetByID with user_id
//   - Log Update with user_id and changed fields
//   - Log Delete with user_id
//   - Log errors with context
```

**Success Criteria**:
- [ ] All user endpoints use structured logging
- [ ] User ID logged in all operations
//   - Log Update with user_id and changed fields
//   - Log Delete with user_id
//   - Log errors with context
```

**Success Criteria**:
- [ ] All user endpoints use structured logging
- [ ] User ID logged in all operations
- [ ] Changes logged for updates
- [ ] Errors logged with context
- [ ] Backward compatible

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go build ./internal/http/handler/...
go test ./tests/unit/... -run TestUser -v
```

---

### T5.3: Update Organization Handler for Structured Logging

**ID**: T5.3  
**Phase**: 5  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T3.1, T4.2

**Description**:
Update OrganizationHandler to use structured logging.

**Files to Modify**:
- `internal/http/handler/organization.go` - Add structured logging

**Content Requirements**:
```go
// Import logger package
// Update handler methods to:
//   - Get logger from context
//   - Log Create with org_id
//   - Log GetByID with org_id
//   - Log Update with org_id
//   - Log Delete with org_id
//   - Log member operations with user_id and org_id
//   - Log errors with context
```

**Success Criteria**:
- [ ] All organization endpoints use structured logging
- [ ] Organization ID logged in all operations
- [ ] Member operations logged with both IDs
- [ ] Errors logged with context
- [ ] Backward compatible

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go build ./internal/http/handler/...
go test ./tests/integration/... -run TestOrganization -v
```

---

### T5.4: Update Remaining Handlers for Structured Logging

**ID**: T5.4  
**Phase**: 5  
**Estimated Duration**: 2 hours  
**Dependencies**: T3.1, T4.2

**Description**:
Update remaining handlers (Role, Permission, Media, News, Email, Invoice) to use structured logging.

**Files to Modify**:
- `internal/http/handler/role.go`
- `internal/http/handler/permission.go`
- `internal/http/handler/media.go`
- `internal/http/handler/news.go`
- `internal/http/handler/email.go`
- `internal/http/handler/email_template.go`
- `internal/module/invoice/handler.go`

**Content Requirements**:
```go
// Same pattern for all handlers:
//   - Import logger package
//   - Get logger from context at start of handler
//   - Log operations with relevant IDs
//   - Log errors with context
//   - Follow existing handler patterns
```

**Success Criteria**:
- [ ] All remaining handlers use structured logging
- [ ] Resource IDs logged in operations
- [ ] Errors logged with context
- [ ] Backward compatible
- [ ] No breaking changes to existing tests

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go build ./internal/http/handler/...
go build ./internal/module/invoice/...
go test ./... -v 2>&1 | tail -50
```

---

## Phase 6: Testing & Quality Assurance

### T6.1: Create Comprehensive Unit Tests

**ID**: T6.1  
**Phase**: 6  
**Estimated Duration**: 2 hours  
**Dependencies**: T1.4, T2.3, T3.2, T3.3

**Description**:
Create unit tests for all logger components with >80% coverage target.

**Files to Create**:
- `tests/unit/logger/logger_test.go` - Logger interface tests
- `tests/unit/logger/field_test.go` - Field tests
- `tests/unit/logger/context_test.go` - Context tests
- `tests/unit/logger/slog_logger_test.go` - SlogLogger tests
- `tests/unit/logger/writer_test.go` - Writer tests

**Content Requirements**:
```go
// Test all components:
//   - Level enum conversion
//   - Field constructors and ToSlogAttr
//   - Context operations (With*, Get*)
//   - SlogLogger all levels
//   - Level filtering (Debug skipped at Info level)
//   - WithFields immutability
//   - WithError
//   - StdoutWriter output
//   - FileWriter (with temp files)
//   - MultiWriter
//   - NopLogger
//   - MockLogger assertions
// Use table-driven tests
// Use testify/assert and testify/require
```

**Success Criteria**:
- [ ] All test files created
- [ ] >80% coverage on logger package
- [ ] All level methods tested
- [ ] All field types tested
- [ ] Context operations tested
- [ ] Writer implementations tested
- [ ] Mock assertions tested

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./tests/unit/logger/... -v
go test ./tests/unit/logger/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep "total:"  # Should be >80%
go test ./tests/unit/logger/... -race
```

---

### T6.2: Create Middleware Integration Tests

**ID**: T6.2  
**Phase**: 6  
**Estimated Duration**: 1.5 hours  
**Dependencies**: T3.1, T4.2

**Description**:
Create integration tests for logging middleware with Echo server.

**Files to Create**:
- `tests/integration/logging_test.go` - Middleware integration tests

**Content Requirements**:
```go
// Test scenarios:
//   - Request/response logging flow
//   - Context propagation through middleware chain
//   - Request ID extraction/generation
//   - User ID extraction from JWT
//   - Org ID extraction from header
//   - Duration calculation accuracy
//   - Error logging with stack traces
//   - Skipper functionality (health checks)
//   - Multiple writers (capture both outputs)
// Use httptest for HTTP testing
// Use MockLogger to capture logged messages
```

**Imports Required**:
```go
import (
    "net/http"
    "net/http/httptest"
    "testing"
    
    "github.com/example/go-api-base/internal/http/middleware"
    "github.com/example/go-api-base/internal/logger"
    "github.com/labstack/echo/v4"
    "github.com/stretchr/testify/assert"
)
```

**Success Criteria**:
- [ ] All test scenarios implemented
- [ ] Tests use real Echo server
- [ ] MockLogger captures all log messages
- [ ] Duration calculated correctly
- [ ] All context fields extracted

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./tests/integration/... -tags=integration -run TestLogging -v
go test ./tests/integration/... -tags=integration -race
```

---

### T6.3: Create Performance Benchmarks

**ID**: T6.3  
**Phase**: 6  
**Estimated Duration**: 1 hour  
**Dependencies**: T6.1

**Description**:
Create benchmarks to verify performance targets (<1ms per log call, <2ms middleware overhead).

**Files to Create**:
- `tests/benchmark/logger_bench_test.go` - Logger benchmarks

**Content Requirements**:
```go
// Benchmarks:
//   - BenchmarkLoggerInfo - log.Info call
//   - BenchmarkLoggerWithFields - WithFields + Info
//   - BenchmarkLoggerError - Error with stack trace
//   - BenchmarkContextExtraction - ExtractContextFields
//   - BenchmarkMiddleware - Full middleware chain
//   - Memory allocation benchmarks (b.ReportAllocs())
// 
// Targets:
//   - Logger calls: <1ms (1000µs)
//   - Context extraction: <500µs
//   - Middleware: <2ms (2000µs)
```

**Success Criteria**:
- [ ] All benchmarks implemented
- [ ] BenchmarkLoggerInfo < 1ms
- [ ] BenchmarkContextExtraction < 500µs
- [ ] BenchmarkMiddleware < 2ms
- [ ] Memory allocations measured

**Verification Steps**:
```bash
cd C:\Development\base\go-api
go test ./tests/benchmark/... -bench=. -benchmem -benchtime=5s
go test ./tests/benchmark/... -bench=BenchmarkMiddleware -benchmem
```

---

### T6.4: Run Full Test Suite and Race Detector

**ID**: T6.4  
**Phase**: 6  
**Estimated Duration**: 1 hour  
**Dependencies**: T6.1, T6.2, T6.3

**Description**:
Run complete test suite to verify no regressions and no race conditions.

**Files**: N/A (verification task)

**Success Criteria**:
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Race detector clean (go test -race)
- [ ] Coverage > 80%
- [ ] No breaking changes (existing tests still pass)
- [ ] golangci-lint passes

**Verification Steps**:
```bash
cd C:\Development\base\go-api
# Unit tests
go test ./... -v 2>&1 | grep -E "(PASS|FAIL)"

# Race detector
go test ./internal/logger/... -race
go test ./internal/http/middleware/... -race

# Coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep "total:"

# Lint
golangci-lint run ./internal/logger/... ./internal/http/middleware/...
```

---

## Phase 7: Documentation & Deployment

### T7.1: Update AGENTS.md with Logging Reference

**ID**: T7.1  
**Phase**: 7  
**Estimated Duration**: 1 hour  
**Dependencies**: T6.4

**Description**:
Add comprehensive logging section to AGENTS.md for future developers.

**Files to Modify**:
- `AGENTS.md` - Add logging feature section

**Content Requirements**:
```markdown
## STRUCTURED LOGGING FEATURE

### Overview
- Location: internal/logger/
- Middleware: internal/http/middleware/structured_logging.go
- Usage: middleware.GetLogger(c) in handlers

### Configuration
Environment variables table

### Usage Patterns
Examples of logging in handlers

### Migration from slog
Before/after examples

### Performance
Overhead metrics

### Testing
How to use MockLogger in tests
```

**Success Criteria**:
- [ ] Logging section added to AGENTS.md
- [ ] Configuration documented
- [ ] Usage patterns with examples
- [ ] Migration guide included
- [ ] Testing patterns documented

**Verification Steps**:
```bash
cd C:\Development\base\go-api
grep -A 50 "STRUCTURED LOGGING" AGENTS.md | head -60
```

---

### T7.2: Update README.md and Create Migration Guide

**ID**: T7.2  
**Phase**: 7  
**Estimated Duration**: 1 hour  
**Dependencies**: T6.4

**Description**:
Update README with logging section and create migration guide.

**Files to Modify/Create**:
- `README.md` - Add logging section
- `docs/migrations/005-log-writer.md` - Migration guide
- `specs/005-log-writer/quickstart.md` - Already exists, verify content

**Content Requirements**:
```markdown
// README.md addition:
## Logging

The API uses structured logging with the following features:
- Structured JSON/text output
- Multiple writers (stdout, file, syslog)
- Automatic context propagation
- Request/response logging

See [quickstart.md](specs/005-log-writer/quickstart.md) for details.

// Migration guide:
# Migration Guide: Structured Logging

## Pre-deployment
- Review current logging
- Set LOG_LEVEL=info

## Deployment
- Deploy new code (backward compatible)
- Verify logs with request_id

## Post-deployment
- Update handlers gradually
```

**Success Criteria**:
- [ ] README.md has logging section
- [ ] Migration guide created
- [ ] quickstart.md reviewed and accurate
- [ ] All links work

**Verification Steps**:
```bash
cd C:\Development\base\go-api
grep -A 10 "## Logging" README.md
cat docs/migrations/005-log-writer.md | head -30
```

---

### T7.3: Final Verification and Sign-off

**ID**: T7.3  
**Phase**: 7  
**Estimated Duration**: 0.5 hours  
**Dependencies**: T7.1, T7.2

**Description**:
Final verification that all requirements are met.

**Files**: N/A (verification task)

**Success Criteria**:
- [ ] All phases complete (T1.1 through T7.2)
- [ ] Unit test coverage > 80%
- [ ] All integration tests pass
- [ ] Performance benchmarks pass (<2ms middleware)
- [ ] Race detector clean
- [ ] No breaking changes verified
- [ ] Documentation complete (AGENTS.md, README.md, migration guide)
- [ ] Environment variables documented in .env.example
- [ ] Code review ready

**Final Verification Commands**:
```bash
cd C:\Development\base\go-api

# Build check
go build ./...

# Test check
go test ./... -v 2>&1 | grep -E "^(PASS|FAIL|ok|FAIL)"

# Coverage check
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1

# Race check
go test ./internal/logger/... -race
go test ./internal/http/middleware/... -race

# Lint check
golangci-lint run ./internal/logger/... ./internal/http/middleware/...

# Benchmark check
go test ./tests/benchmark/... -bench=. -benchtime=1s
```

**Sign-off Checklist**:
- [ ] Architecture Review: Complete
- [ ] Security Review: Complete (no PII in logs)
- [ ] Performance Review: Complete (<2ms target met)
- [ ] Documentation Review: Complete

---

## Task Dependency Graph

```
T1.1 ──┬── T1.2 ──┬── T1.3 ──┬── T1.4 ──┬── T2.1 ──┬── T2.2 ──┬── T2.3 ──┬── T2.4 ──┬── T3.1
       │          │          │          │          │          │          │
       └──────────┴──────────┴──────────┘          └──────────┴──────────┘
                                                            │
                                                            ▼
                                                     T4.1 ──┬── T4.2 ──┬── T4.3
                                                           │          │
                                                          T3.2       T5.1
                                                          T3.3       T5.2
                                                                     T5.3
                                                                     T5.4
                                                                       │
                                                                       ▼
                                                                 T6.1 ──┬── T6.2 ──┬── T6.3 ──┬── T6.4
                                                                       │          │          │
                                                                       ▼          ▼          ▼
                                                                     T7.1       T7.2       T7.3
```

---

## Summary

| Phase | Tasks | Duration | Key Deliverables |
|-------|-------|----------|------------------|
| 1: Logger Interface | T1.1-T1.4 | 6 hours | Logger interface, Field, Context, SlogLogger |
| 2: Writers & Config | T2.1-T2.4 | 5.5 hours | Writer interface, Stdout/File writers, Config, Factory |
| 3: Middleware | T3.1-T3.3 | 4 hours | StructuredLogging middleware, NopLogger, MockLogger |
| 4: App Integration | T4.1-T4.3 | 3 hours | Config integration, Server integration, .env.example |
| 5: Handler Updates | T5.1-T5.4 | 6.5 hours | All handlers updated with structured logging |
| 6: Testing | T6.1-T6.4 | 5.5 hours | Unit tests, Integration tests, Benchmarks, Race detector |
| 7: Documentation | T7.1-T7.3 | 2.5 hours | AGENTS.md, README.md, Migration guide |
| **Total** | **25 tasks** | **~30 hours** | **Complete structured logging system** |

---

## Notes for Implementation

1. **Zero Breaking Changes**: All changes must be backward compatible. Existing slog calls continue to work.

2. **Performance Critical**: Each task includes benchmarks. The <2ms middleware target is mandatory.

3. **Security First**: Never log passwords, tokens, or other sensitive data. LogRequestBody is disabled by default.

4. **Test-Driven**: Each task includes verification steps. Tests must pass before marking complete.

5. **Incremental**: Tasks can be implemented incrementally. Each task produces working code.

6. **Code Review Ready**: Follow existing codebase patterns from internal/permission/, internal/cache/, internal/audit/.

---

**END OF TASKS - READY FOR IMPLEMENTATION**
