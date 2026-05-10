# Import Service Contract

## ImportService Interface

```go
type ImportService interface {
    // PreviewImport validates import metadata without committing changes.
    // Checks: format, headers, field names, row count, file size, HMAC signature.
    // Does NOT: check database constraints, unique violations, or FK existence.
    PreviewImport(ctx context.Context, file io.Reader, format string) (*ImportPreviewResponse, error)
    
    // CreateImport creates an import job for async processing.
    // Validates: file size, format, entity count, idempotency key.
    // Enqueues Asynq task for background processing.
    CreateImport(ctx context.Context, req *ImportRequest) (*ImportJob, error)
    
    // GetImport retrieves an import job by ID.
    GetImport(ctx context.Context, id uuid.UUID) (*ImportJob, error)
    
    // ListImports lists import jobs for the current user.
    ListImports(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*ImportJob, error)
    
    // CancelImport cancels a queued or processing import job.
    CancelImport(ctx context.Context, id uuid.UUID) error
    
    // ProcessImport is called by the Asynq worker to process an import job.
    // Processes entities in topological order, in batches of IMPORT_BATCH_SIZE.
    // Creates import_id_maps entries for UUID resolution.
    ProcessImport(ctx context.Context, jobID uuid.UUID) error
}
```

## Import Processing Flow

```
1. Worker picks up Asynq task
2. Load ImportJob from DB
3. Validate HMAC signature of source file
4. Read source file via StorageDriver
5. Create FormatDecoder for format
6. Call decoder.Validate() for metadata check
7. For each entity type in ImportOrder:
   a. Begin transaction
   b. Decode records in batches of 500
   c. For each record:
      - Look up external_id in import_id_maps (resolve UUID)
      - Check entity exportability registry
      - Check per-entity permission (enforce)
      - Apply conflict strategy (skip/overwrite/fail)
      - Never resurrect soft-deleted records (generate new UUID)
      - Insert/update record
   d. Commit transaction
   e. Update ImportJob result with per-entity counts
8. Mark ImportJob as completed
9. Publish EventImportCompleted to EventBus
10. Create single audit_log entry
```

## Error Types

```go
var (
    ErrImportNotFound      = errors.NewAppError("NOT_FOUND", "import job not found", 404)
    ErrImportRateLimited   = errors.NewAppError("RATE_LIMITED", "import rate limit exceeded", 429)
    ErrFileTooLarge        = errors.NewAppError("PAYLOAD_TOO_LARGE", "import file exceeds 50MB limit", 413)
    ErrEntityCountExceeded = errors.NewAppError("UNPROCESSABLE", "import exceeds 10K entity limit", 422)
    ErrInvalidFormat       = errors.NewAppError("BAD_REQUEST", "unsupported import format", 400)
    ErrHmacValidationFailed = errors.NewAppError("BAD_REQUEST", "file integrity check failed", 400)
    ErrDuplicateImport     = errors.NewAppError("CONFLICT", "duplicate import (same idempotency key)", 409)
    ErrEntityNotImportable = errors.NewAppError("FORBIDDEN", "entity type not importable", 403)
    ErrJsonDepthExceeded   = errors.NewAppError("BAD_REQUEST", "JSON nesting exceeds 32 levels", 400)
    ErrCsvInjection        = errors.NewAppError("BAD_REQUEST", "CSV injection detected", 400)
)
```

## Invariants

1. **Permission check MUST happen at handler AND service layer**: Handler checks `data_portability:import:create`, service checks per-entity during processing.
2. **All imports are async**: No synchronous import path. Handler validates metadata and returns job ID.
3. **Topological order**: Entities MUST be processed in `ImportOrder` sequence to resolve FK dependencies.
4. **Per-entity-type transactions**: Each entity type is committed in its own transaction. If users fails, organizations remain valid.
5. **UUID mapping via import_id_maps**: All FK references within the import file MUST be resolved through the mapping table before INSERT.
6. **Soft-deleted records**: If external UUID matches a soft-deleted record, generate a new UUID and map it. NEVER resurrect soft-deleted records.
7. **Conflict strategy**: Default is "skip". "overwrite" updates existing records. "fail" aborts the entire entity type on first conflict.
8. **Idempotency**: SHA-256 of file content + timestamp stored in Redis with 24h TTL prevents duplicate imports.
9. **Rate limiting**: 5 imports/hour/user, 50K records/hour/org.
10. **Audit**: Single audit_log entry per import job with action="data_portability.import", resource=entity_type, resource_id=job_id.
11. **Security**: CSV injection check (strip leading =, +, -, @, tab), JSON depth limit (32 levels), ZIP bomb check (max 1000 entries, 100MB uncompressed).
12. **PII import**: Imported users MUST NOT set password_hash. Email is validated for format. two_factor fields are silently stripped.