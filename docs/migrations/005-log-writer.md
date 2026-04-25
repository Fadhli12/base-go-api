# Migration Guide: 005-Log-Writer (Structured Logging System)

**Feature**: Structured Logging System  
**Status**: ✅ Complete  
**Version**: 1.0.0  
**Date**: 2026-04-25

---

## Overview

The 005-log-writer feature introduces a comprehensive structured logging system to the Go API Base. This guide helps you migrate from the existing logging approach to the new structured logging system.

### What's New

- **Structured Logging Interface** - Clean abstraction over Go's `log/slog`
- **Context Propagation** - Automatic extraction of request_id, user_id, org_id, trace_id
- **Multiple Output Writers** - Stdout, file with rotation, syslog support
- **Middleware Integration** - Automatic request/response logging
- **Zero Breaking Changes** - Existing slog calls continue to work

---

## Migration Path

### Phase 1: No Action Required (Backward Compatible)

The new logging system is **fully backward compatible**. Your existing code continues to work without changes:

```go
// This still works - no changes needed
slog.Info("user created", slog.String("id", userID))
```

### Phase 2: Gradual Adoption (Recommended)

Migrate handlers incrementally to use the new logger interface:

**Before:**
```go
func (h *Handler) GetUser(c echo.Context) error {
    userID := c.Param("id")
    slog.Info("fetching user", slog.String("id", userID))
    
    user, err := h.service.GetUser(c.Request().Context(), userID)
    if err != nil {
        slog.Error("failed to fetch user", slog.String("id", userID), slog.Any("error", err))
        return echo.NewHTTPError(http.StatusNotFound, "Not found")
    }
    
    return c.JSON(http.StatusOK, user)
}
```

**After:**
```go
func (h *Handler) GetUser(c echo.Context) error {
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    userID := c.Param("id")
    
    log.Info(ctx, "fetching user",
        log.String("id", userID),
    )
    
    user, err := h.service.GetUser(ctx, userID)
    if err != nil {
        log.Error(ctx, "failed to fetch user",
            log.String("id", userID),
            logger.Err(err),
        )
        return echo.NewHTTPError(http.StatusNotFound, "Not found")
    }
    
    return c.JSON(http.StatusOK, user)
}
```

### Phase 3: Full Migration (Optional)

Once all handlers are migrated, you can deprecate direct slog usage and standardize on the logger interface.

---

## Configuration

### Environment Variables

Add these to your `.env` file:

```bash
# Logging Configuration
LOG_LEVEL=info                          # debug, info, warn, error
LOG_FORMAT=json                         # json or text
LOG_OUTPUTS=stdout                      # stdout, file, syslog (comma-separated)
LOG_FILE_PATH=/var/log/api.log         # Path for file output
LOG_FILE_MAX_SIZE=100                   # MB before rotation
LOG_FILE_MAX_BACKUPS=10                 # Number of backup files
LOG_FILE_MAX_AGE=30                     # Days to keep backups
LOG_FILE_COMPRESS=true                  # Compress rotated files
LOG_SYSLOG_NETWORK=                     # tcp, udp, or empty for local
LOG_SYSLOG_ADDRESS=                     # Syslog server address
LOG_SYSLOG_TAG=go-api                   # Syslog tag
LOG_ADD_SOURCE=false                    # Include file:line in logs
```

### Update .env.example

Ensure `.env.example` includes all logging variables:

```bash
# Logging
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUTS=stdout
LOG_FILE_PATH=/var/log/api.log
LOG_FILE_MAX_SIZE=100
LOG_FILE_MAX_BACKUPS=10
LOG_FILE_MAX_AGE=30
LOG_FILE_COMPRESS=true
LOG_SYSLOG_NETWORK=
LOG_SYSLOG_ADDRESS=
LOG_SYSLOG_TAG=go-api
LOG_ADD_SOURCE=false
```

---

## Handler Migration Examples

### Example 1: User Handler

**Before:**
```go
func (h *UserHandler) CreateUser(c echo.Context) error {
    var req CreateUserRequest
    if err := c.BindJSON(&req); err != nil {
        slog.Error("invalid request", slog.Any("error", err))
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
    }
    
    user, err := h.service.CreateUser(c.Request().Context(), req)
    if err != nil {
        slog.Error("failed to create user", slog.Any("error", err))
        return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user")
    }
    
    slog.Info("user created", slog.String("id", user.ID.String()))
    return c.JSON(http.StatusCreated, user)
}
```

**After:**
```go
func (h *UserHandler) CreateUser(c echo.Context) error {
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    
    var req CreateUserRequest
    if err := c.BindJSON(&req); err != nil {
        log.Error(ctx, "invalid request", logger.Err(err))
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
    }
    
    user, err := h.service.CreateUser(ctx, req)
    if err != nil {
        log.Error(ctx, "failed to create user", logger.Err(err))
        return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user")
    }
    
    log.Info(ctx, "user created",
        log.String("user_id", user.ID.String()),
    )
    return c.JSON(http.StatusCreated, user)
}
```

### Example 2: Organization Handler

**Before:**
```go
func (h *OrgHandler) ListMembers(c echo.Context) error {
    orgID := c.Param("id")
    slog.Info("listing organization members", slog.String("org_id", orgID))
    
    members, err := h.service.ListMembers(c.Request().Context(), orgID)
    if err != nil {
        slog.Error("failed to list members", slog.String("org_id", orgID), slog.Any("error", err))
        return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list members")
    }
    
    return c.JSON(http.StatusOK, members)
}
```

**After:**
```go
func (h *OrgHandler) ListMembers(c echo.Context) error {
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    orgID := c.Param("id")
    
    log.Info(ctx, "listing organization members",
        log.String("org_id", orgID),
    )
    
    members, err := h.service.ListMembers(ctx, orgID)
    if err != nil {
        log.Error(ctx, "failed to list members",
            log.String("org_id", orgID),
            logger.Err(err),
        )
        return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list members")
    }
    
    return c.JSON(http.StatusOK, members)
}
```

---

## Service Layer Migration

Services can also use the logger interface for better context propagation:

**Before:**
```go
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
    slog.Info("fetching user", slog.String("id", id.String()))
    
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        slog.Error("user not found", slog.String("id", id.String()))
        return nil, ErrNotFound
    }
    
    return user, nil
}
```

**After:**
```go
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
    log := logger.FromContext(ctx)
    
    log.Info(ctx, "fetching user",
        log.String("user_id", id.String()),
    )
    
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        log.Error(ctx, "user not found",
            log.String("user_id", id.String()),
            logger.Err(err),
        )
        return nil, ErrNotFound
    }
    
    return user, nil
}
```

---

## Testing Migration

### Unit Tests

Update mock logger usage in tests:

**Before:**
```go
func TestGetUser(t *testing.T) {
    // No structured logging in tests
    user, err := handler.GetUser(mockContext)
    assert.NoError(t, err)
}
```

**After:**
```go
func TestGetUser(t *testing.T) {
    mockLog := logger.NewMockLogger()
    c := echo.NewContext(nil, nil)
    c.Set("logger", mockLog)
    
    user, err := handler.GetUser(c)
    assert.NoError(t, err)
    assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "fetching user"))
}
```

### Integration Tests

No changes needed - logging is transparent to integration tests.

---

## Deployment Checklist

- [ ] Update `.env.example` with logging variables
- [ ] Configure `LOG_LEVEL` for your environment (debug/info/warn/error)
- [ ] Set `LOG_OUTPUTS` based on your infrastructure (stdout/file/syslog)
- [ ] If using file output, ensure `/var/log/` directory exists and is writable
- [ ] If using syslog, configure `LOG_SYSLOG_ADDRESS` and `LOG_SYSLOG_NETWORK`
- [ ] Test logging in development environment
- [ ] Verify log output format (JSON or text)
- [ ] Update log aggregation/monitoring to parse new log format
- [ ] Deploy and monitor logs in production

---

## Troubleshooting

### Issue: Logger not available in handler

**Solution**: Ensure middleware is registered in the correct order:

```go
// In server.go
e.Use(middleware.StructuredLogging(logger))  // Must be before handlers
```

### Issue: Context fields not appearing in logs

**Solution**: Ensure you're using the logger from middleware:

```go
// Correct
log := middleware.GetLogger(c)

// Incorrect - creates new logger without context
log := logger.NewDefaultLogger()
```

### Issue: File output not working on Windows

**Solution**: File output tests are skipped on Windows due to file locking. Use stdout or syslog instead.

### Issue: Syslog connection refused

**Solution**: Verify syslog server is running and accessible:

```bash
# Test syslog connectivity
nc -zv logs.example.com 514
```

---

## Performance Impact

- **Log call overhead**: < 1ms per call
- **Middleware overhead**: < 2ms per request
- **Memory**: Zero-allocation for constant keys
- **Concurrency**: Fully goroutine-safe

No performance degradation expected from migration.

---

## Backward Compatibility

✅ **Fully backward compatible**

- Existing slog calls continue to work
- No breaking changes to handler signatures
- No breaking changes to HTTP response formats
- Existing tests continue to pass
- Optional adoption - migrate at your own pace

---

## Support & Documentation

- **Spec**: [specs/005-log-writer/spec.md](../../specs/005-log-writer/spec.md)
- **Quickstart**: [specs/005-log-writer/quickstart.md](../../specs/005-log-writer/quickstart.md)
- **Contracts**: [specs/005-log-writer/contracts/logger.md](../../specs/005-log-writer/contracts/logger.md)
- **AGENTS.md**: [AGENTS.md](../../AGENTS.md) - Structured Logging Feature section

---

## Rollback Plan

If you need to rollback:

1. Revert logging configuration to previous state
2. Handlers will continue to work (backward compatible)
3. Existing slog calls will still function
4. No database migrations needed

---

## Next Steps

1. **Review** this migration guide
2. **Update** `.env.example` with logging variables
3. **Configure** logging for your environment
4. **Migrate** handlers incrementally (recommended)
5. **Test** logging in development
6. **Deploy** with confidence

---

**Migration Guide Version**: 1.0.0  
**Last Updated**: 2026-04-25  
**Status**: ✅ Ready for Production
