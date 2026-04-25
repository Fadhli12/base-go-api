# Quickstart: Structured Logging System

**Feature**: 005-log-writer - Structured Logging System  
**Date**: 2026-04-25  
**Status**: Ready for Implementation

---

## Table of Contents

1. [Installation](#installation)
2. [Basic Usage](#basic-usage)
3. [Configuration](#configuration)
4. [Context Propagation](#context-propagation)
5. [Middleware Integration](#middleware-integration)
6. [Handler Usage](#handler-usage)
7. [Testing](#testing)
8. [Advanced Topics](#advanced-topics)

---

## Installation

The structured logging system is part of the core API. No additional installation required.

### Import Path

```go
import "github.com/example/go-api-base/internal/logger"
```

### Verify Installation

```bash
# Run logger tests
go test ./internal/logger/... -v

# Run integration tests
go test -tags=integration ./tests/integration/... -run Logging
```

---

## Basic Usage

### 1. Creating a Logger

```go
package main

import (
    "os"
    "github.com/example/go-api-base/internal/logger"
)

func main() {
    // Create with configuration
    config := logger.Config{
        Level:   "info",
        Format:  "json",
        Outputs: []string{"stdout"},
    }
    
    log, err := logger.New(config)
    if err != nil {
        panic(err)
    }
    
    // Use the logger
    ctx := context.Background()
    log.Info(ctx, "application started")
}
```

### 2. Logging with Fields

```go
import (
    "context"
    "time"
    "github.com/example/go-api-base/internal/logger"
)

func processOrder(ctx context.Context, log logger.Logger, orderID string) {
    // Log with structured fields
    log.Info(ctx, "processing order",
        log.String("order_id", orderID),
        log.Int("items", 5),
        log.Float64("total", 99.99),
        log.Duration("timeout", 30*time.Second),
    )
    
    // Log errors with context
    if err := doSomething(); err != nil {
        log.Error(ctx, "order processing failed",
            log.String("order_id", order_id),
            log.Err(err),
        )
    }
}
```

### 3. Log Levels

```go
// Debug - Development only, may be verbose
log.Debug(ctx, "entering function", log.String("function", "processOrder"))

// Info - State changes, significant events
log.Info(ctx, "order created", log.String("order_id", orderID))

// Warn - Degraded conditions, suspicious behavior
log.Warn(ctx, "slow database query",
    log.Duration("duration", 5*time.Second),
    log.String("query", "SELECT..."),
)

// Error - Failures affecting request processing
log.Error(ctx, "payment failed",
    log.String("order_id", orderID),
    log.Err(err),
)
```

---

## Configuration

### Environment Variables

```bash
# Set log level (debug, info, warn, error)
export LOG_LEVEL=info

# Set output format (json, text)
export LOG_FORMAT=json

# Set outputs (comma-separated: stdout, file, syslog)
export LOG_OUTPUTS=stdout,file

# File writer configuration
export LOG_FILE_PATH=/var/log/api.log
export LOG_FILE_MAX_SIZE=100        # MB
export LOG_FILE_MAX_BACKUPS=10
export LOG_FILE_MAX_AGE=30          # days
export LOG_FILE_COMPRESS=true

# Syslog configuration
export LOG_SYSLOG_NETWORK=tcp
export LOG_SYSLOG_ADDRESS=localhost:514
export LOG_SYSLOG_TAG=go-api

# Include source file:line
export LOG_ADD_SOURCE=true
```

### Programmatic Configuration

```go
// Load from environment
config := logger.LoadConfigFromEnv()

// Or create manually
config := logger.Config{
    Level:     "info",
    Format:    "json",
    Outputs:   []string{"stdout", "file"},
    AddSource: true,
    File: logger.FileConfig{
        Path:       "/var/log/api.log",
        MaxSize:    100,
        MaxBackups: 10,
        MaxAge:     30,
        Compress:   true,
    },
}

// Validate
if err := config.Validate(); err != nil {
    log.Fatal(err)
}

// Create logger
log, err := logger.New(config)
```

### Configuration Files

```yaml
# config/logger.yaml
level: info
format: json
outputs:
  - stdout
  - file
file:
  path: /var/log/api.log
  max_size: 100
  max_backups: 10
  max_age: 30
  compress: true
syslog:
  network: tcp
  address: localhost:514
  tag: go-api
add_source: false
```

```go
// Load from file
config, err := logger.LoadConfig("config/logger.yaml")
```

---

## Context Propagation

### Automatic Context Extraction

The logger automatically extracts these fields from context:

| Field | Source | Header |
|-------|--------|--------|
| `request_id` | Context / Auto-generated | `X-Request-ID` |
| `user_id` | JWT claims | `Authorization` |
| `org_id` | Organization middleware | `X-Organization-ID` |
| `trace_id` | Context / Auto-generated | `X-Trace-ID` |

### Setting Context Fields

```go
import (
    "context"
    "github.com/example/go-api-base/internal/logger"
)

func middlewareExample(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        ctx := c.Request().Context()
        
        // Add request ID
        requestID := c.Request().Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = logger.GenerateRequestID()
        }
        ctx = logger.WithRequestID(ctx, requestID)
        
        // Add user ID from JWT
        if claims := getClaims(c); claims != nil {
            ctx = logger.WithUserID(ctx, claims.UserID)
        }
        
        // Add organization ID
        if orgID := c.Request().Header.Get("X-Organization-ID"); orgID != "" {
            ctx = logger.WithOrgID(ctx, orgID)
        }
        
        // Add trace ID
        traceID := c.Request().Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = logger.GenerateTraceID()
        }
        ctx = logger.WithTraceID(ctx, traceID)
        
        // Update request context
        c.SetRequest(c.Request().WithContext(ctx))
        
        return next(c)
    }
}
```

### Getting Context Fields

```go
func handlerExample(c echo.Context) error {
    ctx := c.Request().Context()
    
    // Extract individual fields
    requestID := logger.GetRequestID(ctx)
    userID := logger.GetUserID(ctx)
    orgID := logger.GetOrgID(ctx)
    traceID := logger.GetTraceID(ctx)
    
    // Extract all fields at once
    fields := logger.ExtractContextFields(ctx)
    // Returns: [request_id, user_id, org_id, trace_id] as []Field
    
    return c.JSON(200, map[string]string{
        "request_id": requestID,
        "user_id":    userID,
    })
}
```

### Logger from Context

```go
func serviceMethod(ctx context.Context) {
    // Get logger from context (never returns nil)
    log := logger.FromContext(ctx)
    
    // All context fields automatically included
    log.Info(ctx, "service method called")
    // Output: {"level":"INFO","message":"service method called",
    //          "request_id":"...","user_id":"...",...}
}
```

---

## Middleware Integration

### Using Structured Logging Middleware

```go
import (
    "github.com/example/go-api-base/internal/http/middleware"
    "github.com/example/go-api-base/internal/logger"
)

func setupServer() *echo.Echo {
    e := echo.New()
    
    // Initialize logger
    log, _ := logger.New(logger.DefaultConfig())
    
    // Apply structured logging middleware
    e.Use(middleware.StructuredLogging(middleware.LoggingMiddlewareConfig{
        Logger:          log,
        Skipper:         middleware.DefaultLoggingSkipper(),
        LogRequestBody:  false,
        LogResponseBody: false,
    }))
    
    return e
}
```

### Middleware Configuration

```go
config := middleware.LoggingMiddlewareConfig{
    // Required: Logger instance
    Logger: log,
    
    // Optional: Skip certain paths (health checks, etc.)
    Skipper: func(c echo.Context) bool {
        path := c.Request().URL.Path
        return path == "/healthz" || path == "/readyz"
    },
    
    // Optional: Log request body (disabled by default for PII safety)
    LogRequestBody: false,
    
    // Optional: Log response body
    LogResponseBody: false,
    
    // Optional: Max body size to log (bytes)
    BodyMaxSize: 1024,
}
```

### Log Output Example

**Request Start (DEBUG level):**
```json
{
  "timestamp": "2026-04-25T10:30:00Z",
  "level": "DEBUG",
  "message": "request started",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "user-123",
  "org_id": "org-456",
  "trace_id": "trace-789",
  "method": "POST",
  "path": "/api/v1/users",
  "remote_addr": "192.168.1.1",
  "user_agent": "Mozilla/5.0..."
}
```

**Request Complete (INFO level):**
```json
{
  "timestamp": "2026-04-25T10:30:00.045Z",
  "level": "INFO",
  "message": "request completed",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "user-123",
  "org_id": "org-456",
  "trace_id": "trace-789",
  "method": "POST",
  "path": "/api/v1/users",
  "status": 201,
  "duration_ms": 45,
  "response_size": 256
}
```

**Error Logging (ERROR level):**
```json
{
  "timestamp": "2026-04-25T10:30:00.050Z",
  "level": "ERROR",
  "message": "request failed",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "user-123",
  "error": "database connection timeout",
  "stack_trace": "github.com/example/.../handler.go:45\ngithub.com/labstack/echo/...",
  "status": 500,
  "duration_ms": 5000
}
```

---

## Handler Usage

### Basic Handler Example

```go
package handler

import (
    "net/http"
    "github.com/labstack/echo/v4"
    "github.com/example/go-api-base/internal/http/middleware"
    "github.com/example/go-api-base/internal/logger"
)

type UserHandler struct {
    service UserService
}

func NewUserHandler(service UserService) *UserHandler {
    return &UserHandler{service: service}
}

func (h *UserHandler) Create(c echo.Context) error {
    // Get logger from context (injected by middleware)
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    
    // Parse request
    var req CreateUserRequest
    if err := c.Bind(&req); err != nil {
        log.Error(ctx, "failed to bind request",
            log.Err(err),
        )
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
    }
    
    // Log operation start
    log.Info(ctx, "creating user",
        log.String("email", req.Email),
        log.String("role", req.Role),
    )
    
    // Call service
    user, err := h.service.Create(ctx, req)
    if err != nil {
        log.Error(ctx, "user creation failed",
            log.String("email", req.Email),
            log.Err(err),
        )
        return echo.NewHTTPError(http.StatusInternalServerError, "Creation failed")
    }
    
    // Log success
    log.Info(ctx, "user created successfully",
        log.String("user_id", user.ID),
        log.String("email", user.Email),
    )
    
    return c.JSON(http.StatusCreated, user)
}
```

### Field Chaining

```go
func (h *UserHandler) ProcessOrder(c echo.Context) error {
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    
    // Create scoped logger with common fields
    orderLog := log.WithFields(
        log.String("order_id", c.Param("id")),
        log.String("user_id", getUserID(c)),
    )
    
    // All logs include order_id and user_id
    orderLog.Info(ctx, "validating order")
    
    if err := validate(); err != nil {
        orderLog.Error(ctx, "validation failed", log.Err(err))
        return err
    }
    
    orderLog.Info(ctx, "processing payment")
    
    if err := processPayment(); err != nil {
        // Add error to logger
        orderLog.WithError(err).Error(ctx, "payment failed")
        return err
    }
    
    orderLog.Info(ctx, "order completed")
    return c.JSON(200, result)
}
```

### Error Handling

```go
func (h *UserHandler) GetUser(c echo.Context) error {
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    userID := c.Param("id")
    
    log.Info(ctx, "fetching user", log.String("user_id", userID))
    
    user, err := h.service.GetByID(ctx, userID)
    if err != nil {
        // Check error type for appropriate logging
        if errors.Is(err, ErrNotFound) {
            log.Warn(ctx, "user not found",
                log.String("user_id", userID),
            )
            return echo.NewHTTPError(http.StatusNotFound, "User not found")
        }
        
        log.Error(ctx, "database error",
            log.String("user_id", userID),
            log.Err(err),
        )
        return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
    }
    
    log.Info(ctx, "user fetched", log.String("user_id", userID))
    return c.JSON(http.StatusOK, user)
}
```

---

## Testing

### Unit Test with Mock Logger

```go
package handler

import (
    "testing"
    "github.com/example/go-api-base/internal/logger"
    "github.com/stretchr/testify/assert"
)

func TestUserHandler_Create(t *testing.T) {
    // Create mock logger
    mockLog := logger.NewMockLogger()
    
    // Create handler with mock
    handler := NewUserHandler(mockService, mockLog)
    
    // Execute request
    // ... setup echo context, call handler.Create(c)
    
    // Assert logged messages
    assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "creating user"))
    assert.True(t, mockLog.AssertField("email", "test@example.com"))
    assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "user created successfully"))
}
```

### Integration Test

```go
func TestLoggingMiddleware(t *testing.T) {
    // Create test server with logging middleware
    e := echo.New()
    log, _ := logger.New(logger.Config{Level: "debug"})
    
    e.Use(middleware.StructuredLogging(middleware.LoggingMiddlewareConfig{
        Logger: log,
    }))
    
    e.GET("/test", func(c echo.Context) error {
        return c.String(200, "OK")
    })
    
    // Make request
    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)
    
    // Assert logs captured
    // (using log capture writer in test setup)
}
```

### Benchmark

```go
func BenchmarkLoggerInfo(b *testing.B) {
    log, _ := logger.New(logger.Config{Level: "info"})
    ctx := context.Background()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        log.Info(ctx, "benchmark message",
            log.String("key1", "value1"),
            log.Int("key2", 42),
        )
    }
}
```

---

## Advanced Topics

### Custom Writers

```go
// Implement custom writer
type DatabaseWriter struct {
    db *gorm.DB
}

func (w *DatabaseWriter) Write(ctx context.Context, entry logger.LogEntry) error {
    // Store log in database
    return w.db.WithContext(ctx).Create(&LogRecord{
        Timestamp: entry.Timestamp,
        Level:     entry.Level,
        Message:   entry.Message,
        Fields:    entry.Fields,
    }).Error
}

func (w *DatabaseWriter) Close() error {
    return nil
}

// Use custom writer
config := logger.Config{
    Outputs: []string{"custom"},
}

customWriter := &DatabaseWriter{db: db}
log, _ := logger.NewWithWriters(config, customWriter)
```

### Sampling

```go
// Log 1% of debug messages to reduce volume
config := logger.Config{
    Level: "debug",
    Sampling: &logger.SamplingConfig{
        Initial:    100,
        Thereafter: 100, // Log 1 out of 100
    },
}
```

### Redacting Sensitive Data

```go
// Custom field redaction
type RedactingLogger struct {
    logger.Logger
    sensitiveFields []string
}

func (l *RedactingLogger) Info(ctx context.Context, msg string, fields ...logger.Field) {
    // Redact sensitive fields
    for i, f := range fields {
        if l.isSensitive(f.Key) {
            fields[i] = logger.String(f.Key, "[REDACTED]")
        }
    }
    l.Logger.Info(ctx, msg, fields...)
}

func (l *RedactingLogger) isSensitive(key string) bool {
    for _, s := range l.sensitiveFields {
        if s == key {
            return true
        }
    }
    return false
}
```

### Context Propagation to External Services

```go
func callExternalAPI(ctx context.Context, url string) error {
    // Get trace ID from context
    traceID := logger.GetTraceID(ctx)
    
    // Propagate to external service
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("X-Trace-ID", traceID)
    
    // External service receives same trace ID
    resp, err := http.DefaultClient.Do(req)
    // ...
}
```

---

## Troubleshooting

### Common Issues

#### 1. No Logs Appearing

```bash
# Check log level
export LOG_LEVEL=debug

# Verify logger initialization
log, err := logger.New(config)
if err != nil {
    panic(err) // Never ignore initialization errors
}
```

#### 2. Context Fields Not Appearing

```go
// Ensure middleware injects logger
log := middleware.GetLogger(c)
if log == nil {
    // Logger not injected - check middleware order
}

// Use FromContext as fallback
log := logger.FromContext(c.Request().Context())
```

#### 3. Performance Issues

```bash
# Enable sampling for high-volume debug logs
export LOG_SAMPLING_INITIAL=100
export LOG_SAMPLING_THEREAFTER=100

# Reduce file writer sync frequency
export LOG_FILE_SYNC_INTERVAL=5s
```

### Debug Mode

```go
// Enable debug logging
config := logger.Config{
    Level:     "debug",
    AddSource: true, // Include file:line
}
```

---

## Migration from Existing Logging

### Gradual Migration

```go
// Step 1: Add structured logger alongside existing
func (h *Handler) Action(c echo.Context) error {
    // Old logging (still works)
    slog.Info("action called")
    
    // New structured logging
    log := middleware.GetLogger(c)
    log.Info(c.Request().Context(), "action called",
        log.String("action", "user_create"),
    )
    
    // ... handler logic
}

// Step 2: Replace old logging gradually
// Step 3: Remove old logging when all migrated
```

### Bulk Migration Script

```bash
# Find all slog calls
grep -r "slog\." --include="*.go" internal/

# Replace patterns (example)
# Before: slog.Info("user created", slog.String("id", id))
# After: log.Info(ctx, "user created", log.String("id", id))
```

---

## Examples

### Complete Handler Example

```go
package handler

import (
    "net/http"
    "github.com/labstack/echo/v4"
    "github.com/example/go-api-base/internal/http/middleware"
    "github.com/example/go-api-base/internal/logger"
)

type OrderHandler struct {
    service OrderService
}

func (h *OrderHandler) Create(c echo.Context) error {
    ctx := c.Request().Context()
    log := middleware.GetLogger(c)
    
    // Parse and validate
    var req CreateOrderRequest
    if err := c.Bind(&req); err != nil {
        log.Error(ctx, "invalid request", log.Err(err))
        return echo.NewHTTPError(400, "Invalid request")
    }
    
    // Create scoped logger
    orderLog := log.WithFields(
        log.String("order_ref", req.Reference),
        log.String("customer_id", req.CustomerID),
    )
    
    // Process
    orderLog.Info(ctx, "creating order")
    
    order, err := h.service.Create(ctx, req)
    if err != nil {
        orderLog.Error(ctx, "order creation failed", log.Err(err))
        return echo.NewHTTPError(500, "Creation failed")
    }
    
    orderLog.Info(ctx, "order created",
        log.String("order_id", order.ID),
        log.Float64("total", order.Total),
    )
    
    return c.JSON(201, order)
}
```

### Service Layer Example

```go
package service

import (
    "context"
    "github.com/example/go-api-base/internal/logger"
)

type OrderService struct {
    repo   OrderRepository
    // No logger field - use context propagation
}

func (s *OrderService) Create(ctx context.Context, req CreateOrderRequest) (*Order, error) {
    // Get logger from context
    log := logger.FromContext(ctx)
    
    log.Info(ctx, "validating order")
    
    if err := s.validate(ctx, req); err != nil {
        log.Warn(ctx, "validation failed", log.Err(err))
        return nil, err
    }
    
    log.Info(ctx, "saving order")
    
    order, err := s.repo.Create(ctx, req)
    if err != nil {
        log.Error(ctx, "database error", log.Err(err))
        return nil, err
    }
    
    log.Info(ctx, "order saved", log.String("order_id", order.ID))
    
    return order, nil
}
```

---

## Summary

| Task | Command |
|------|---------|
| Basic logging | `log.Info(ctx, "message", log.String("key", "value"))` |
| Get logger from context | `log := logger.FromContext(ctx)` |
| Add context fields | `ctx = logger.WithRequestID(ctx, id)` |
| Chain fields | `log.WithFields(...).Info(ctx, "msg")` |
| Error logging | `log.Error(ctx, "msg", log.Err(err))` |
| Configuration | `logger.New(config)` |
| Testing | `logger.NewMockLogger()` |

---

## Support

For issues or questions:
1. Check this guide
2. Review `internal/logger/` package documentation
3. See example implementations in `internal/module/invoice/`
4. Run tests: `go test ./internal/logger/... -v`

---

**Next Steps**: See [Implementation Plan](plan.md) for phase-by-phase development guide.
