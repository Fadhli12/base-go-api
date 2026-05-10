# Research: Data Import/Export System

## Existing Patterns Research

### Asynq Integration

**Source**: `internal/service/job.go`, `internal/domain/job.go`

The existing job system uses Asynq with Redis. Key patterns:
- `domain.Job` entity with status, payload, result fields
- Jobs are enqueued via `asynq.Client` with task type + payload
- Workers implement `asynq.HandlerFunc` with `HandleTask(ctx, task)`
- Task types are string constants (e.g., `TaskTypeWebhookDelivery`)
- Job status tracked in DB: `queued`, `processing`, `completed`, `failed`, `retrying`
- Worker pool configured via `WEBHOOK_WORKER_CONCURRENCY` env var
- **Reuse pattern**: Create `TaskTypeExport` and `TaskTypeImport` task types, follow same worker pattern

### StorageDriver Interface

**Source**: `internal/service/media.go`, `internal/service/media_version.go`

- `StorageDriver` interface with `Save`, `Get`, `Delete`, `GetSignedURL` methods
- Two implementations: `LocalStorageDriver` and `S3StorageDriver`
- Signed URLs use `STORAGE_SIGNING_KEY` (falls back to JWT secret)
- **Reuse pattern**: Export files stored via same `StorageDriver` interface. GetSignedURL for download.

### EventBus Pattern

**Source**: `internal/domain/webhook_events.go`

- `EventBus` struct with Go channels (buffered, 256)
- `Publish(event)` method, `Subscribe()` returns channel
- Services call `SetEventBus()` setter (avoids constructor changes)
- Events carry `OrgID` for org-scoped dispatch
- **Reuse pattern**: Add export/import events: `export.completed`, `import.completed`, `import.failed`

### Permission Sync Manifest

**Source**: `cmd/api/main.go` (runSeed + permission:sync)

- Permissions defined as `[]domain.PermissionSync` slice
- `permission:sync` CLI command upserts from manifest
- Format: `{Resource: "data_portability", Action: "export:create", Description: "..."}`
- **Reuse pattern**: Add 5 new permissions to the manifest slice in main.go

### Rate Limiter

**Source**: `internal/service/webhook_rate_limiter.go`, `webhook_rate_limiter_iface.go`

- `WebhookRateLimiterInterface` with `Allow(webhookID string) (bool, error)`
- Redis ZSET sliding window implementation
- Falls open on Redis failure (fails open, not closed)
- Configurable: `WEBHOOK_RATE_LIMIT` (default 100/min)
- **Reuse pattern**: Create `DataPortabilityRateLimiter` with same pattern, different keys and limits

### HMAC Signing

**Source**: `internal/service/webhook_sign.go`

- `signWebhookPayload(payload, secret)` → `sha256=<hex-hmac>`
- Strips `whsec_` prefix from secret before HMAC computation
- **Reuse pattern**: Sign export files with same HMAC-SHA256, add `X-Export-Signature` header on download

### Repository Pattern

**Source**: `internal/repository/*.go`

All repositories follow:
1. Interface definition in `repository/` package
2. GORM implementation struct with `db *gorm.DB`
3. All methods take `context.Context` as first param
4. Soft delete via `db.WithContext(ctx).Where("deleted_at IS NULL")`
5. `ToResponse()` methods on domain entities for API response transformation
6. **Reuse pattern**: Follow exact same pattern for ExportJobRepository, ImportJobRepository, ImportIDMapRepository

### Handler Pattern

**Source**: `internal/http/handler/*.go`

- `RegisterRoutes(g *echo.Group)` method on handler struct
- JWT auth middleware applied per route group
- Permission checks via handler-level middleware: `v1.GET("", h.List, middleware.RequirePermission(enforcer, "data_portability", "export:create"))`
- Request validation with `echo.Bind()` and custom validators
- `middleware.GetLogger(c)` for structured logging
- `middleware.GetOrganizationID(c)` for org context
- **Reuse pattern**: Same handler pattern, same route registration, same middleware chain

## Open Questions (Resolved)

| Question | Resolution |
|----------|-----------|
| Should export sync threshold be configurable? | Yes, via `EXPORT_SYNC_THRESHOLD` env var (default 10K) |
| Can we reuse the StorageDriver for export files? | Yes, same interface, different base path |
| Should import_id_maps be cleaned up after import? | No — kept for audit traceability. TTL cleanup via background job if needed later |
| Should export files be stored in DB or filesystem? | Filesystem (via StorageDriver). DB stores the path reference only |
| Should dry-run hit the database? | No — metadata validation only (headers, types, count, size) |
| Who can import? | Superadmin only for `import:create`. Org admin can view (`import:view`). |