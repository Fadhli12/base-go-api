# internal/http - Echo HTTP Layer

**Generated:** 2026-04-22 | **Commit:** HEAD | **Branch:** main

---

## OVERVIEW

HTTP layer using Echo v4 framework. Handles routing, middleware chain, request validation, and response formatting. All responses use a standardized JSON envelope.

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Server setup | `server.go:49` | `NewServer()` creates Echo instance |
| Middleware chain | `server.go:68-78` | Order matters: Recover first |
| Route registration | `server.go:240` | `RegisterRoutes()` wires everything |
| Error handling | `server.go:100` | `HTTPErrorHandler` wraps all errors |
| JWT middleware | `middleware/jwt.go:32` | `JWT()` extracts Bearer token |
| Permission middleware | `middleware/permission.go:49` | `Permission()` checks access |
| Audit middleware | `middleware/audit.go:59` | `Audit()` captures before/after |
| Request DTOs | `request/*.go` | Validation with go-playground |
| Response envelope | `response/envelope.go:25` | `Envelope` struct |

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `Server` | struct | `server.go:28` | Echo wrapper with dependencies |
| `NewServer` | func | `server.go:49` | Server init, middleware chain |
| `RegisterRoutes` | func | `server.go:240` | Route group setup |
| `HTTPErrorHandler` | func | `server.go:100` | Error → JSON envelope |
| `JWT` | func | `middleware/jwt.go:32` | Bearer token extraction |
| `Permission` | func | `middleware/permission.go:49` | Casbin RBAC check |
| `Audit` | func | `middleware/audit.go:59` | Async audit logging |
| `Envelope` | struct | `response/envelope.go:25` | Standard response format |
| `ErrorDetail` | struct | `response/envelope.go:32` | Error info container |
| `GetUserID` | func | `middleware/jwt.go:118` | Extract UUID from claims |
| `RequirePermission` | func | `middleware/permission.go:161` | Permission middleware helper |

## MIDDLEWARE CHAIN

```text
Request → Recover → RequestID → CORS → RateLimit → JWT → Permission → Audit → Handler → Response
```

**Order is CRITICAL:**
1. **Recover** (first) - Catches panics in all downstream middleware
2. **RequestID** - Generates unique ID for logging/tracing
3. **CORS** - Handles preflight before auth checks
4. **RateLimit** - Protects against abuse (requires Redis)
5. **JWT** - Extracts user identity, populates context
6. **Permission** - RBAC check (requires JWT claims)
7. **Audit** - Logs mutations (requires actor from JWT)

**Applied per-route-group** in `RegisterRoutes()`:
```go
// Example: Role routes
roles := api.Group("/roles")
roles.Use(middleware.JWT(...))          // Layer 1: Auth
roles.Use(middleware.RequirePermission(...))  // Layer 2: RBAC
roles.Use(middleware.Audit(...))        // Layer 3: Audit
```

## RESPONSE ENVELOPE

All API responses use the standard envelope:

```go
type Envelope struct {
    Data  interface{}   `json:"data,omitempty"`
    Error *ErrorDetail  `json:"error,omitempty"`
    Meta  *Meta         `json:"meta,omitempty"`
}

type Meta struct {
    RequestID string `json:"request_id"`
    Timestamp string `json:"timestamp"`
}
```

**Success response:**
```json
{
  "data": { ... },
  "meta": {
    "request_id": "abc-123",
    "timestamp": "2026-04-22T10:00:00Z"
  }
}
```

**Error response:**
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": "..."
  },
  "meta": {
    "request_id": "abc-123",
    "timestamp": "2026-04-22T10:00:00Z"
  }
}
```

## REQUEST VALIDATION

Request DTOs use `go-playground/validator` with struct tags:

```go
type RegisterRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=72"`
}

func (r *RegisterRequest) Validate() error {
    return validate.Struct(r)  // Shared validator instance
}
```

**Handler pattern:**
```go
func (h *AuthHandler) Register(c echo.Context) error {
    var req request.RegisterRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(400, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
    }
    if err := req.Validate(); err != nil {
        return c.JSON(400, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
    }
    // ... business logic
}
```

## PERMISSION MIDDLEWARE

Two-tier permission model:

**Tier 1: Route-level middleware**
```go
// Applied to route group
roles.Use(middleware.RequirePermission(enforcer, "roles", "manage"))
```

**Tier 2: Handler-level scope check**
```go
// Inside handler (e.g., invoice ownership)
func (h *Handler) GetByID(c echo.Context) error {
    userID, _ := middleware.GetUserID(c)
    resourceID := c.Param("id")
    
    // Check ownership scope
    if !h.enforcer.EnforceScope(ctx, userID, "invoices", resourceID, "view") {
        return echo.NewHTTPError(403, "Access denied")
    }
    // ...
}
```

**Context helpers:**
```go
// middleware/jwt.go
GetUserID(c) → uuid.UUID
GetUserClaims(c) → *auth.Claims
GetUserEmail(c) → string

// middleware/permission.go
GetUserIDFromContext(c) → uuid.UUID
GetPermissionResult(c) → *PermissionResult
```

## ERROR HANDLING

`HTTPErrorHandler` converts all errors to envelope format:

| Error Type | HTTP Status | Code |
|------------|-------------|------|
| `*echo.HTTPError` | `echoErr.Code` | `HTTP_ERROR` / `NOT_FOUND` |
| `*apperrors.AppError` | `appErr.HTTPStatus` | `appErr.Code` |
| Unknown | 500 | `INTERNAL_ERROR` |

**Usage in handlers:**
```go
// Return AppError - HTTPErrorHandler converts it
if errors.Is(err, errors.ErrNotFound) {
    return echo.NewHTTPError(404, "Not found")  // Handled by Echo
}

// Or use response helpers directly
return c.JSON(code, response.ErrorWithContext(c, code, message))
```

## ROUTE PATTERNS

**Public routes (no auth):**
```go
auth := v1.Group("/auth")
auth.POST("/register", authHandler.Register)
auth.POST("/login", authHandler.Login)
auth.POST("/refresh", authHandler.Refresh)
```

**Protected routes (JWT required):**
```go
protected := v1.Group("")
protected.Use(middleware.JWT(...))
protected.GET("/me", userHandler.GetCurrentUser)
```

**Resource routes (JWT + permission):**
```go
roles := api.Group("/roles")
roles.Use(middleware.JWT(...))
roles.Use(middleware.RequirePermission(enforcer, "roles", "manage"))
roles.Use(middleware.Audit(...))  // Mutations only

roles.POST("", roleHandler.Create)
roles.GET("", roleHandler.GetAll)
roles.PUT("/:id", roleHandler.Update)
roles.DELETE("/:id", roleHandler.SoftDelete)
```

## ANTI-PATTERNS (THIS PROJECT)

### FORBIDDEN

| Pattern | Reason | Enforcement |
|---------|--------|-------------|
| Printf in handlers | Unstructured | `slog` key-value |
| Bare `c.JSON(status, data)` | No envelope | Use `response.SuccessWithContext()` |
| Skipping audit | Compliance | Required for mutations |
| Permission check only in handler | Inconsistent | Route middleware + scope check |
| Direct error returns | No context | Use AppError or response helpers |
| Missing request ID | Traceability | Always use response.WithContext() |

### MANDATORY

- Context propagation (`c.Request().Context()`)
- Request validation before business logic
- Permission middleware on protected routes
- Audit middleware on mutating operations
- Request ID in all responses

## NOTES

### Middleware Skipper Pattern

```go
// Define skipper for health checks
skipHealth := func(c echo.Context) bool {
    return c.Path() == "/health" || c.Path() == "/ready"
}

// Apply to middleware
e.Use(middleware.JWTWithSkipper(secret, skipHealth))
```

### Audit Middleware Behavior

| Method | Before State | After State |
|--------|-------------|-------------|
| POST | `nil` | Response data |
| PUT/PATCH | Request body | Response data |
| DELETE | `nil` | `nil` |

Audit logs are **async** - don't block on errors.

### Rate Limiter Behavior

- **Sliding window**: 60-second TTL
- **Default limit**: 100 requests/minute/IP
- **Redis required**: Applied only if Redis client exists
- **Fail open**: Allows requests on Redis errors

### Gotchas

1. **Audit skipper** - Default skips `/health`, `/ready`, `/metrics`, `/auth/login`, `/auth/refresh`
2. **Permission domain** - Uses `X-Tenant-ID` header for multi-tenant (fallback: `"default"`)
3. **JWT context key** - Default is `"user"`, claims stored at `c.Get("user")`
4. **Rate limit header** - `X-RateLimit-Remaining` shows remaining requests
5. **Handler order** - Rate limit before JWT (fail fast on abuse)

## CRITICAL CONSTRAINTS

From constitution & patterns:

1. **Audit required** - All mutating operations (POST, PUT, PATCH, DELETE)
2. **Context propagation** - All handlers use `c.Request().Context()`
3. **Two-tier permissions** - Route middleware + handler scope checks
4. **Response envelope** - All responses wrapped in `Envelope`
5. **Error codes** - AppError codes for business errors, not HTTP status codes

## TROUBLESHOOTING

| Issue | Solution |
|-------|----------|
| 401 on all routes | Check JWT middleware applied, token valid |
| Permission denied | `permission:sync` to reload Casbin |
| Missing audit logs | Check AuditService not nil, Skipper config |
| Rate limit 429 | Wait 60s or check Redis connectivity |
| No request ID | Verify RequestID middleware applied |
| CORS failures | Check CORS config, allowed origins |

## FILES

- `server.go` - Server struct, route registration
- `middleware/jwt.go` - JWT extraction, claims helpers
- `middleware/permission.go` - RBAC middleware
- `middleware/audit.go` - Audit logging
- `middleware/rate_limit.go` - Redis rate limiting
- `middleware/cors.go` - CORS configuration
- `middleware/recover.go` - Panic recovery
- `middleware/request_id.go` - Request ID generation
- `middleware/logging.go` - Request logging
- `handler/auth.go` - Auth endpoints
- `handler/user.go` - User management
- `handler/role.go` - Role management
- `handler/permission.go` - Permission management
- `handler/health.go` - Health checks
- `request/auth.go` - Auth request DTOs
- `request/user.go` - User request DTOs
- `request/role.go` - Role request DTOs
- `request/permission.go` - Permission request DTOs
- `request/invoice.go` - Invoice request DTOs
- `response/envelope.go` - Response format