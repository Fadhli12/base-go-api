# Handler Contract

## DataPortabilityHandler

```go
type DataPortabilityHandler struct {
    exportService  service.ExportService
    importService  service.ImportService
    enforcer       *permission.Enforcer
    rateLimiter    service.DataPortabilityRateLimiter
    logger         logger.Logger
}

func (h *DataPortabilityHandler) RegisterRoutes(g *echo.Group, jwtSecret string) {
    // Export routes
    e := g.Group("/exports", middleware.JWT(jwtSecret))
    e.POST("", h.CreateExport, middleware.RequirePermission(h.enforcer, "data_portability", "export:create"))
    e.GET("", h.ListExports, middleware.RequirePermission(h.enforcer, "data_portability", "export:download"))
    e.GET("/:id", h.GetExport, middleware.RequirePermission(h.enforcer, "data_portability", "export:download"))
    e.GET("/:id/download", h.DownloadExport, middleware.RequirePermission(h.enforcer, "data_portability", "export:download"))
    e.DELETE("/:id", h.CancelExport, middleware.RequirePermission(h.enforcer, "data_portability", "import:cancel"))
    
    // Import routes
    i := g.Group("/imports", middleware.JWT(jwtSecret))
    i.POST("/preview", h.PreviewImport, middleware.RequirePermission(h.enforcer, "data_portability", "import:create"))
    i.POST("", h.CreateImport, middleware.RequirePermission(h.enforcer, "data_portability", "import:create"))
    i.GET("", h.ListImports, middleware.RequirePermission(h.enforcer, "data_portability", "import:view"))
    i.GET("/:id", h.GetImport, middleware.RequirePermission(h.enforcer, "data_portability", "import:view"))
    i.POST("/:id/cancel", h.CancelImport, middleware.RequirePermission(h.enforcer, "data_portability", "import:cancel"))
}
```

## Handler Behaviors

### CreateExport
1. Parse `ExportRequest` from JSON body
2. Extract `userID` from JWT claims, `orgID` from `X-Organization-ID` header
3. Check rate limit (10/hr/user for exports)
4. Validate entity types against `EntityRegistry.IsExportable()`
5. If entity not exportable → 403 Forbidden
6. If `sync=true` AND estimated row count < threshold → stream directly via `StreamExport()`
7. Otherwise → create async `ExportJob` via `ExportService.CreateExport()`
8. Return 202 Accepted with job ID

### CreateImport
1. Parse multipart form data (file + metadata)
2. Extract `userID` from JWT claims, `orgID` from header
3. Check rate limit (5/hr/user for imports)
4. Validate file size ≤ 50MB
5. Compute SHA-256 idempotency key, check Redis for duplicates
6. Store file via StorageDriver
7. Create `ImportJob` via `ImportService.CreateImport()`
8. Return 202 Accepted with job ID

### PreviewImport
1. Same as CreateImport steps 1-4
2. Call `ImportService.PreviewImport()` (metadata-only validation, no DB writes)
3. Return preview report (entity counts, conflicts, validation errors)

### DownloadExport
1. Get export job by ID
2. Verify job status is "completed"
3. Verify file has not expired
4. Return signed URL (redirect) or stream directly

### CancelExport / CancelImport
1. Get job by ID
2. Verify job status is "queued" or "processing"
3. Cancel Asynq task if queued
4. Mark job as "cancelled"
5. Return 200 OK

## Middleware Requirements

```go
// Rate limiting middleware adapted from webhook_rate_limiter.go
func DataPortabilityRateLimit(limiter DataPortabilityRateLimiter) echo.MiddlewareFunc {
    // Limits: 10/hr/user for exports, 5/hr/user for imports, 50K/hr/org for records
    // Uses Redis ZSET sliding window (same pattern as webhook rate limiter)
    // Falls open on Redis failure (fails open, not closed)
}

// File size middleware
func MaxImportFileSize(maxBytes int64) echo.MiddlewareFunc {
    // Rejects multipart uploads exceeding maxBytes
    // Applied only to POST /imports and POST /imports/preview
}
```

## Response Format

All responses use the existing Envelope format:
```json
{
    "data": { ... },
    "error": { "code": "...", "message": "..." },
    "meta": { "request_id": "...", "timestamp": "..." }
}
```