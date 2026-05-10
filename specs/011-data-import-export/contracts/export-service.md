# Export Service Contract

## ExportService Interface

```go
type ExportService interface {
    // CreateExport creates an export job (async) or streams directly (sync).
    // For sync exports: returns the job with Status="completed" and data streamed to ResponseWriter.
    // For async exports: returns the job with Status="queued" and job ID.
    CreateExport(ctx context.Context, req *ExportRequest) (*ExportJob, error)
    
    // GetExport retrieves an export job by ID.
    // Returns ErrNotFound if job doesn't exist or user lacks access.
    GetExport(ctx context.Context, id uuid.UUID) (*ExportJob, error)
    
    // ListExports lists export jobs for the current user (optionally filtered by org).
    ListExports(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*ExportJob, int64, error)
    
    // DownloadExport returns a signed URL or direct stream for the completed export.
    // Returns ErrNotFound if job not completed or file expired.
    DownloadExport(ctx context.Context, id uuid.UUID) (*ExportDownload, error)
    
    // CancelExport cancels a queued or processing export job.
    // Returns ErrNotFound or ErrInvalidState if job cannot be cancelled.
    CancelExport(ctx context.Context, id uuid.UUID) error
    
    // StreamExport writes export data directly to ResponseWriter for sync exports.
    // Uses ExportCursor for memory-safe streaming.
    StreamExport(ctx context.Context, req *ExportRequest, w io.Writer) error
}
```

## ExportDownload DTO

```go
type ExportDownload struct {
    URL         string    // Signed URL for file download (expires in 24h)
    ContentType string    // MIME type of the file
    Filename    string    // Suggested filename for download
    ExpiresAt   time.Time // When the signed URL expires
    HMACSignature string   // SHA-256 HMAC for file integrity verification
}
```

## Error Types

```go
var (
    ErrExportNotFound    = errors.NewAppError("NOT_FOUND", "export job not found", 404)
    ErrExportNotReady    = errors.NewAppError("EXPORT_NOT_READY", "export is still processing", 409)
    ErrExportExpired     = errors.NewAppError("EXPORT_EXPIRED", "export file has expired", 410)
    ErrExportRateLimited = errors.NewAppError("RATE_LIMITED", "export rate limit exceeded", 429)
    ErrEntityNotExportable = errors.NewAppError("FORBIDDEN", "entity type not exportable", 403)
    ErrInvalidEntityType   = errors.NewAppError("BAD_REQUEST", "invalid or unsupported entity type", 400)
)
```

## Invariants

1. **Permission check MUST happen before export creation**: `enforcer.Enforce(userID, orgID, "data_portability", "export:create")`
2. **Entity registry check MUST happen before any data access**: `EntityRegistry.IsExportable(entityType)` — if false, return 403 immediately.
3. **PII hashing**: Non-superadmin exports MUST hash user email and MUST NOT include password_hash, two_factor_secret.
4. **Rate limiting**: Checked BEFORE job creation. 10 exports/hour/user, 50K records/hour/org.
5. **Sync threshold**: If estimated row count < EXPORT_SYNC_THRESHOLD (default 10K), stream directly. Otherwise, create async job.
6. **File expiration**: Export files auto-delete after EXPORT_FILE_TTL (default 24h).
7. **HMAC signature**: Every export file MUST be signed with HMAC-SHA256 using the STORAGE_SIGNING_KEY.
8. **Soft deletes**: By default, soft-deleted records are excluded. Include only if `include_deleted=true` AND user is superadmin.
9. **Audit**: Single audit_log entry per export job with action="data_portability.export", resource=entity_type, resource_id=job_id.