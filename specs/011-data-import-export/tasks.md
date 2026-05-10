# Implementation Tasks: Data Import/Export System

**Feature**: Data Import/Export System
**Branch**: `011-data-import-export`
**Generated**: 2026-05-10
**Source**: [spec.md](./spec.md) | [plan.md](./plan.md) | [data-model.md](./data-model.md)

---

## Task Summary

| # | Task | Priority | Depends On | Est. |
|---|------|----------|------------|------|
| T001 | SQL migration: export_jobs, import_jobs, import_id_maps | P1 | — | 1h |
| T002 | Domain entities: ExportJob, ImportJob, ImportIDMap | P1 | T001 | 1.5h |
| T003 | Domain: EntityExportabilityRegistry + Exportable/Importable interfaces | P1 | — | 1h |
| T004 | Domain: DataPortability events (EventBus integration) | P1 | — | 30m |
| T005 | Repository: ExportJobRepository | P1 | T002 | 1.5h |
| T006 | Repository: ImportJobRepository | P1 | T002 | 1.5h |
| T007 | Repository: ImportIDMapRepository | P1 | T002 | 1h |
| T008 | Service: HMAC-SHA256 signing (data_portability_sign.go) | P1 | — | 45m |
| T009 | Service: DataPortabilityRateLimiter | P1 | — | 1h |
| T010 | Service: FormatEncoder/FormatDecoder — JSON (NDJSON) | P1 | T003 | 2h |
| T011 | Service: FormatEncoder/FormatDecoder — CSV | P2 | T010 | 1.5h |
| T012 | Service: ExportService — sync + async paths | P1 | T005, T008, T009, T010 | 3h |
| T013 | Service: ExportWorker (Asynq task handler) | P1 | T012 | 1.5h |
| T014 | Service: ImportService — validation, batch processing, UUID mapping | P1 | T006, T007, T008, T009, T010 | 3.5h |
| T015 | Service: ImportWorker (Asynq task handler) | P1 | T014 | 1.5h |
| T016 | Service: DataPortabilityValidator (file validation) | P1 | T008 | 1h |
| T017 | HTTP Handler: DataPortabilityHandler — export endpoints (5) | P1 | T012, T009 | 2h |
| T018 | HTTP Handler: DataPortabilityHandler — import endpoints (5) | P1 | T014, T016 | 2h |
| T019 | Middleware: data_portability rate limiting + file size check | P1 | T009 | 45m |
| T020 | Permission sync: 5 new data_portability permissions | P1 | — | 30m |
| T021 | Startup wiring: cmd/api/main.go integration | P1 | T012, T013, T014, T015, T017, T018 | 1h |
| T022 | EventBus integration: export/import event publishing | P1 | T004, T012, T014 | 45m |
| T023 | Export file TTL cleanup: Asynq scheduled task | P2 | T013 | 1h |
| T024 | Unit tests: domain entities + registry | P1 | T002, T003 | 1h |
| T025 | Unit tests: repositories | P1 | T005, T006, T007 | 1.5h |
| T026 | Unit tests: FormatEncoder/FormatDecoder (JSON + CSV) | P1 | T010, T011 | 1.5h |
| T027 | Unit tests: ExportService | P1 | T012 | 1.5h |
| T028 | Unit tests: ImportService | P1 | T014 | 2h |
| T029 | Unit tests: HMAC signing, rate limiter, validator | P1 | T008, T009, T016 | 1.5h |
| T030 | Integration tests: full round-trip (export → import) | P1 | T021 | 3h |

---

## Phase 1: Foundation (P1 — Must-Ship)

These tasks form the minimum viable implementation. No feature is complete without all P1 tasks.

### T001 — SQL Migration: export_jobs, import_jobs, import_id_maps

**Priority**: P1 | **Depends On**: — | **Est.**: 1h

**Goal**: Create migration 000020 with three new tables: `export_jobs`, `import_jobs`, `import_id_maps` per data-model.md schema. All tables must have `deleted_at` for soft deletes, `created_at`/`updated_at` timestamps, and proper indexes. `org_id` is NULLABLE with FK reference to `organizations(id)`. `import_id_maps` has UNIQUE constraint on `(job_id, entity_type, external_id)`.

**Files**:
- `migrations/000020_create_data_portability.up.sql`
- `migrations/000020_create_data_portability.down.sql`

**Acceptance Criteria**:
- [ ] `make migrate` succeeds
- [ ] Tables match data-model.md schema exactly (column types, constraints, indexes)
- [ ] `org_id` allows NULL with FK to `organizations(id)`
- [ ] `down.sql` drops all three tables in correct order (import_id_maps first, then import_jobs, then export_jobs)
- [ ] Indexes on status, created_by, org_id (with WHERE deleted_at IS NULL partial indexes)

---

### T002 — Domain Entities: ExportJob, ImportJob, ImportIDMap

**Priority**: P1 | **Depends On**: T001 | **Est.**: 1.5h

**Goal**: Create GORM domain entities with `ToResponse()` methods, DTO structs (CreateExportRequest, ExportJobResponse, etc.), and table name methods. Follow project conventions: UUID PKs, soft deletes, `pq.StringArray` for `entity_types` and `text[]`.

**Files**:
- `internal/domain/export_job.go`
- `internal/domain/import_job.go`
- `internal/domain/import_id_map.go`

**Acceptance Criteria**:
- [ ] All entities follow project conventions (UUID PK, timestamps, soft delete, `TableName()`)
- [ ] `ExportJob.OrgID` is `*uuid.UUID` (nullable pointer)
- [ ] `ImportJob.OrgID` is `*uuid.UUID` (nullable pointer)
- [ ] `ToResponse()` methods strip internal fields, return API-safe DTOs
- [ ] DTO request structs have validate tags (binding, required, etc.)
- [ ] `ConflictStrategy` type defined with constants: `skip`, `overwrite`, `fail`
- [ ] `ExportStatus` / `ImportStatus` enum types defined

---

### T003 — Domain: EntityExportabilityRegistry + Exportable/Importable Interfaces

**Priority**: P1 | **Depends On**: — | **Est.**: 1h

**Goal**: Create the entity registry that defines which entity types can be exported/imported, and the `Exportable` and `Importable` interfaces. The registry is a hardcoded MAC (mandatory access control) list — restricted entities are always blocked regardless of Casbin policy. MVP entities: users, roles, permissions, organizations, org_members, user_roles, user_permissions. `ImportOrder` defines topological sorting for import.

**Files**:
- `internal/domain/data_portability.go`

**Acceptance Criteria**:
- [ ] `Exportable` interface with `ToExportRecord() map[string]interface{}` method
- [ ] `Importable` interface with `FromImportRecord() error` method
- [ ] `EntityRegistry` struct with `IsExportable(entityType string) bool`, `IsImportable(entityType string) bool`, `GetExportOrder() []string`, `GetImportOrder() []string`
- [ ] `restrictedEntities` map blocked at service layer (api_keys, two_factor, audit_logs, sessions, etc.)
- [ ] `ImportOrder` constant slice: organizations → roles → permissions → users → org_members → user_roles → user_permissions
- [ ] `ExportableEntities` map: entity_type → domain entity reference
- [ ] Unit test coverage ≥80%

---

### T004 — Domain: DataPortability Events (EventBus Integration)

**Priority**: P1 | **Depends On**: — | **Est.**: 30m

**Goal**: Define EventBus event types for export/import lifecycle: `export.created`, `export.completed`, `export.failed`, `import.created`, `import.completed`, `import.failed`. Follow existing `webhook_events.go` pattern with typed event structs.

**Files**:
- `internal/domain/data_portability_events.go`

**Acceptance Criteria**:
- [ ] Event constants defined for all 6 event types
- [ ] Event structs carry `OrgID` for org-scoped dispatch (following webhook pattern)
- [ ] Events carry job ID, status, and entity types as payload
- [ ] Follows `webhook_events.go` pattern exactly (typed struct, `data_portability` prefix)

---

### T005 — Repository: ExportJobRepository

**Priority**: P1 | **Depends On**: T002 | **Est.**: 1.5h

**Goal**: Create `ExportJobRepository` interface and GORM implementation with CRUD + domain-specific queries: `FindByStatus`, `FindByOrgID`, `FindByCreatedBy`, `UpdateStatus`, `UpdateFilePath`, etc. All methods take `context.Context` as first param, use `db.WithContext(ctx)`, respect soft deletes.

**Files**:
- `internal/repository/export_job.go`

**Acceptance Criteria**:
- [ ] Interface + GORM implementation following `webhook.go` repository pattern
- [ ] Methods: `Create`, `FindByID`, `FindByStatus`, `FindByOrgID`, `FindByCreatedBy`, `UpdateStatus`, `UpdateFilePath`, `SoftDelete`
- [ ] All methods use `context.Context` as first param
- [ ] Soft delete scopes applied via `WHERE deleted_at IS NULL`
- [ ] `ListExports` supports pagination (page, pageSize)

---

### T006 — Repository: ImportJobRepository

**Priority**: P1 | **Depends On**: T002 | **Est.**: 1.5h

**Goal**: Create `ImportJobRepository` interface and GORM implementation. CRUD + `FindByIdempotencyKey`, `UpdateResult`, `UpdateStatus`, `UpdateSourceFilePath`. Same patterns as T005.

**Files**:
- `internal/repository/import_job.go`

**Acceptance Criteria**:
- [ ] Interface + GORM implementation following `webhook.go` repository pattern
- [ ] Methods: `Create`, `FindByID`, `FindByIdempotencyKey`, `UpdateStatus`, `UpdateResult`, `UpdateSourceFilePath`, `SoftDelete`
- [ ] `ListImports` supports pagination
- [ ] All methods use `context.Context`
- [ ] Soft delete scopes applied

---

### T007 — Repository: ImportIDMapRepository

**Priority**: P1 | **Depends On**: T002 | **Est.**: 1h

**Goal**: Create `ImportIDMapRepository` for UUID resolution mapping. Methods: `Create`, `FindByJobAndEntityType`, `FindByExternalID`, `BatchCreate`, `ResolveUUID`. This is the core of import UUID mapping.

**Files**:
- `internal/repository/import_id_map.go`

**Acceptance Criteria**:
- [ ] Interface + GORM implementation
- [ ] Methods: `Create`, `FindByJobAndEntityType`, `FindByExternalID`, `BatchCreate` (batch insert), `ResolveUUID(jobID, entityType, externalID)`
- [ ] `ResolveUUID` returns the internal UUID that maps to an external UUID within the import context
- [ ] All methods use `context.Context`

---

### T008 — Service: HMAC-SHA256 Signing (data_portability_sign.go)

**Priority**: P1 | **Depends On**: — | **Est.**: 45m

**Goal**: Create `signDataPortabilityFile()` function that generates HMAC-SHA256 signatures for export files. Follow pattern from `webhook_sign.go` exactly. Strip `whsec_` prefix from signing key. Sign file content, return `sha256=<hex>` format. Include timestamp in signature payload.

**Files**:
- `internal/service/data_portability_sign.go`

**Acceptance Criteria**:
- [ ] `signDataPortabilityFile(payload []byte, secret string) string` function
- [ ] `verifyDataPortabilitySignature(payload []byte, signature string, secret string) bool` function
- [ ] Strips `whsec_` prefix from secret (same as webhook_sign.go)
- [ ] Returns `sha256=<hex-hmac>` format
- [ ] Unit tests for sign/verify round-trip

---

### T009 — Service: DataPortabilityRateLimiter

**Priority**: P1 | **Depends On**: — | **Est.**: 1h

**Goal**: Create `DataPortabilityRateLimiter` interface and Redis sliding-window implementation following `webhook_rate_limiter.go` pattern. Two rate limits: per-user (10/hr export, 5/hr import) and per-org (50K records/hr). Falls open on Redis failure.

**Files**:
- `internal/service/data_portability_rate_limiter.go`
- `internal/service/data_portability_rate_limiter_iface.go`

**Acceptance Criteria**:
- [ ] `DataPortabilityRateLimiter` interface with `Allow(userID string, action string) (bool, error)` and `AllowRecords(orgID string, count int) (bool, error)`
- [ ] Redis ZSET sliding window implementation (same pattern as webhook)
- [ ] Fails open on Redis error (returns true, nil)
- [ ] Configurable limits via env vars: `IMPORT_RATE_LIMIT_EXPORT`, `IMPORT_RATE_LIMIT_IMPORT`, `IMPORT_RATE_LIMIT_RECORDS`
- [ ] Unit tests with mock Redis

---

### T010 — Service: FormatEncoder/FormatDecoder — JSON (NDJSON)

**Priority**: P1 | **Depends On**: T003 | **Est.**: 2h

**Goal**: Implement `FormatEncoder` and `FormatDecoder` for JSON/NDJSON format. JSON encoder streams NDJSON (one JSON object per line) using `ExportCursor`. JSON decoder reads NDJSON input, validates topological ordering, and yields `Importable` records via channel. Follow `contracts/encoder.md` interface contracts exactly.

**Files**:
- `internal/service/format_json.go`

**Acceptance Criteria**:
- [ ] `JSONEncoder` implements `FormatEncoder` interface
- [ ] `ContentType()` returns `"application/x-ndjson"`
- [ ] `FileExtension()` returns `"json"`
- [ ] `Encode()` streams NDJSON line-by-line using `ExportCursor.Next()` (keyset pagination)
- [ ] `JSONDecoder` implements `FormatDecoder` interface
- [ ] `CanValidate()` returns `true`
- [ ] `Validate()` checks format correctness, required fields, row count (≤10K), nesting depth
- [ ] `Decode()` yields `Importable` records via channel in topological order, validates at parse time
- [ ] Context cancellation respected in both Encode and Decode
- [ ] Unit tests for encode/decode round-trip, error cases, context cancellation

---

### T011 — Service: FormatEncoder/FormatDecoder — CSV

**Priority**: P2 | **Depends On**: T010 | **Est.**: 1.5h

**Goal**: Implement `FormatEncoder` and `FormatDecoder` for CSV format. CSV supports flat entities only (users, roles, permissions, organizations). Junction tables (user_roles, user_permissions, org_members) are NOT supported in CSV format. Follow `contracts/encoder.md`.

**Files**:
- `internal/service/format_csv.go`

**Acceptance Criteria**:
- [ ] `CSVEncoder` implements `FormatEncoder`
- [ ] `ContentType()` returns `"text/csv"`
- [ ] `CSVDecoder` implements `FormatDecoder`
- [ ] `CanValidate()` returns `true`
- [ ] CSV format rejects junction entities (user_roles, user_permissions, org_members) with clear error
- [ ] CSV encoder writes header line + data rows
- [ ] CSV decoder reads header + rows, validates column names match entity fields
- [ ] Unit tests for CSV encode/decode round-trip

---

### T012 — Service: ExportService — Sync + Async Paths

**Priority**: P1 | **Depends On**: T005, T008, T009, T010 | **Est.**: 3h

**Goal**: Implement `ExportService` with sync streaming (<10K rows) and async (≥10K rows via Asynq) export paths. Follow `contracts/export-service.md` exactly. Key methods: `CreateExport`, `GetExport`, `ListExports`, `DownloadExport`, `CancelExport`, `StreamExport`.

**Files**:
- `internal/service/export_service.go`

**Acceptance Criteria**:
- [ ] `ExportService` implements all 6 methods from contract
- [ ] Sync path: entity count < threshold → stream directly via `FormatEncoder.Encode()` with `ExportCursor`
- [ ] Async path: entity count ≥ threshold → create `ExportJob` (status=queued), enqueue Asynq task
- [ ] Permission check via `enforcer.Enforce(userID, orgID, "data_portability", "export:create")` before export
- [ ] Entity registry check: `IsExportable()` blocks restricted entities at service layer (MAC)
- [ ] HMAC signature computed on file content after export completion
- [ ] `ExportCursor` uses keyset pagination (`WHERE id > lastID ORDER BY id LIMIT batchSize`)
- [ ] Rate limiting via `DataPortabilityRateLimiter`
- [ ] Error types defined: `ErrExportNotFound`, `ErrExportNotReady`, `ErrExportExpired`, `ErrExportRateLimited`, `ErrEntityNotExportable`, `ErrInvalidEntityType`
- [ ] Unit tests for sync, async, cancel, rate-limit, permission-denied flows

---

### T013 — Service: ExportWorker (Asynq Task Handler)

**Priority**: P1 | **Depends On**: T012 | **Est.**: 1.5h

**Goal**: Create Asynq worker for async export processing. Follow `webhook_worker.go` pattern. Worker picks up `TaskTypeExport` tasks, processes via `ExportService.StreamExport()`, writes to `StorageDriver`, updates `ExportJob` status and file path. Graceful shutdown support.

**Files**:
- `internal/service/export_worker.go`

**Acceptance Criteria**:
- [ ] `ExportWorker` struct with `Start(ctx)` and `Stop()` methods (follow webhook_worker.go pattern)
- [ ] `HandleExportTask` implements `asynq.HandlerFunc`
- [ ] Worker processes: load job → stream export → write to StorageDriver → update status + HMAC → publish EventBus event
- [ ] On failure: update `ExportJob.ErrorMessage`, set status to `failed`
- [ ] Graceful shutdown via context cancellation
- [ ] Configurable concurrency via `EXPORT_WORKER_CONCURRENCY` env var

---

### T014 — Service: ImportService — Validation, Batch Processing, UUID Mapping

**Priority**: P1 | **Depends On**: T006, T007, T008, T009, T010 | **Est.**: 3.5h

**Goal**: Implement `ImportService` with preview, creation, batch processing, conflict resolution, and UUID mapping. Follow `contracts/import-service.md` exactly. This is the most complex service.

**Files**:
- `internal/service/import_service.go`

**Acceptance Criteria**:
- [ ] `ImportService` implements all 6 methods from contract
- [ ] `PreviewImport`: metadata-only validation (format, headers, row count, file size) — reads DB for conflict detection but makes no writes
- [ ] `CreateImport`: validate file size + format + entity count + idempotency key → store file via StorageDriver → create ImportJob (status=queued) → enqueue Asynq task
- [ ] `ProcessImport`: topological order processing (organizations → roles → permissions → users → org_members → user_roles → user_permissions)
- [ ] Per-entity-type transactions: batch size 500, commit after each entity type
- [ ] Conflict resolution: `skip` / `overwrite` / `fail` strategy per job
- [ ] UUID resolution via `ImportIDMapRepository`: external ID → internal ID mapping
- [ ] Never resurrect soft-deleted records: always generate new UUID for soft-deleted matches
- [ ] HMAC signature verification on source file before processing
- [ ] Rate limiting via `DataPortabilityRateLimiter`
- [ ] Cancel via context: stop after current entity type batch completes
- [ ] `Enforce()` per entity type (~7 checks per import), not per record
- [ ] Error types: `ErrImportNotFound`, `ErrImportRateLimited`, `ErrFileTooLarge`, `ErrEntityConflict`, `ErrInvalidImportFormat`, `ErrImportCancelled`

---

### T015 — Service: ImportWorker (Asynq Task Handler)

**Priority**: P1 | **Depends On**: T014 | **Est.**: 1.5h

**Goal**: Create Asynq worker for import processing. Follow `webhook_worker.go` pattern. Handles stuck-job recovery (reaper), similar to webhook worker.

**Files**:
- `internal/service/import_worker.go`

**Acceptance Criteria**:
- [ ] `ImportWorker` struct with `Start(ctx)` and `Stop()` methods
- [ ] `HandleImportTask` implements `asynq.HandlerFunc`
- [ ] Processes: load job → verify HMAC → decode file via FormatDecoder → ProcessImport → update job result → publish EventBus event
- [ ] Stuck-job reaper: find jobs stuck in `processing` for >5min, reset to `queued`
- [ ] On failure: update `ImportJob.ErrorMessage`, set status to `failed`
- [ ] `processing_started_at` field for stuck-delivery recovery
- [ ] Configurable concurrency via `IMPORT_WORKER_CONCURRENCY` env var
- [ ] Graceful shutdown via context cancellation

---

### T016 — Service: DataPortabilityValidator (File Validation)

**Priority**: P1 | **Depends On**: T008 | **Est.**: 1h

**Goal**: Create validator that checks import files for size, format, entity count, and injection attacks. Used by handler before enqueuing import jobs. Stateless validation.

**Files**:
- `internal/service/data_portability_validator.go`

**Acceptance Criteria**:
- [ ] `ValidateImportFile(fileSize int64, format string, entityCount int) error`
- [ ] File size ≤ 50MB (`IMPORT_MAX_FILE_SIZE`)
- [ ] Format must be `json` or `csv`
- [ ] Entity count ≤ 10K (`IMPORT_MAX_ENTITY_COUNT`)
- [ ] Reject ZIP bombs: check actual file size matches Content-Length
- [ ] Reject CSV injection: detect formula patterns (`=`, `+`, `-`, `@` at start of cells)
- [ ] Clear error messages with AppError format
- [ ] Unit tests for all validation rules

---

### T017 — HTTP Handler: DataPortabilityHandler — Export Endpoints (5)

**Priority**: P1 | **Depends On**: T012, T009 | **Est.**: 2h

**Goal**: Create HTTP handler for 5 export endpoints: POST /exports, GET /exports, GET /exports/:id, GET /exports/:id/download, DELETE /exports/:id. Follow `contracts/handler.md` and existing handler patterns (e.g., `webhook.go`).

**Files**:
- `internal/http/handler/data_portability.go` (partial — export endpoints)

**Acceptance Criteria**:
- [ ] 5 export endpoints registered in `RegisterRoutes`
- [ ] Permission middleware: `RequirePermission(enforcer, "data_portability", "export:create")` etc.
- [ ] JWT authentication required for all endpoints
- [ ] Rate limiting middleware integrated
- [ ] `CreateExport`: parse JSON body, validate entity types with EntityRegistry, dispatch sync or async
- [ ] `ListExports`: pagination, org filter from `X-Organization-ID`
- [ ] `GetExport`: return job status + metadata
- [ ] `DownloadExport`: return signed URL or direct stream
- [ ] `CancelExport`: set status to `cancelled` if queued/processing
- [ ] Request/response format follows Envelope pattern
- [ ] Unit tests for all 5 endpoints

---

### T018 — HTTP Handler: DataPortabilityHandler — Import Endpoints (5)

**Priority**: P1 | **Depends On**: T014, T016 | **Est.**: 2h

**Goal**: Create HTTP handler for 5 import endpoints: POST /imports/preview, POST /imports, GET /imports, GET /imports/:id, POST /imports/:id/cancel. Follow `contracts/handler.md`.

**Files**:
- `internal/http/handler/data_portability.go` (add import endpoints to same file)

**Acceptance Criteria**:
- [ ] 5 import endpoints registered in `RegisterRoutes`
- [ ] Permission middleware for import-specific permissions
- [ ] `PreviewImport`: multipart form upload, validate file, call PreviewImport, return conflict report
- [ ] `CreateImport`: multipart form (file + metadata), validate file, compute HMAC + idempotency key, store file, create job
- [ ] `ListImports`: pagination, org filter
- [ ] `GetImport`: return job status + result JSON
- [ ] `CancelImport`: set status to `cancelled` if queued/processing
- [ ] File upload handling: multipart form, size limit enforcement, temp file storage
- [ ] Unit tests for all 5 endpoints

---

### T019 — Middleware: DataPortability Rate Limiting + File Size Check

**Priority**: P1 | **Depends On**: T009 | **Est.**: 45m

**Goal**: Create Echo middleware that enforces rate limits (per-user, per-org, per-action) and file size limits for import endpoints. Integrate `DataPortabilityRateLimiter` into request pipeline.

**Files**:
- `internal/http/middleware/data_portability.go`

**Acceptance Criteria**:
- [ ] Rate limit middleware: 10/hr export per user, 5/hr import per user, 50K records/hr per org
- [ ] File size middleware: reject uploads > 50MB with 413 status
- [ ] Uses `DataPortabilityRateLimiter` interface (testable without Redis)
- [ ] Returns standard error Envelope on rate limit exceeded (429)
- [ ] Returns standard error Envelope on payload too large (413)
- [ ] Configurable limits via env vars

---

### T020 — Permission Sync: 5 New DataPortability Permissions

**Priority**: P1 | **Depends On**: — | **Est.**: 30m

**Goal**: Add 5 new Casbin permissions to the permission sync manifest in `cmd/api/main.go`. Permissions: `data_portability:export:create`, `data_portability:export:download`, `data_portability:import:create`, `data_portability:import:view`, `data_portability:import:cancel`.

**Files**:
- `cmd/api/main.go` (permission sync manifest)

**Acceptance Criteria**:
- [ ] 5 new `domain.PermissionSync` entries added to manifest slice
- [ ] `go run ./cmd/api permission:sync` creates permissions in DB
- [ ] Permissions follow `{resource}:{action}` naming convention
- [ ] Description fields are human-readable

---

### T021 — Startup Wiring: cmd/api/main.go Integration

**Priority**: P1 | **Depends On**: T012, T013, T014, T015, T017, T018 | **Est.**: 1h

**Goal**: Wire all data portability components into the startup sequence in `cmd/api/main.go`: repositories, services, workers, handlers, EventBus subscriptions, graceful shutdown order.

**Files**:
- `cmd/api/main.go`

**Acceptance Criteria**:
- [ ] New repositories: `ExportJobRepository`, `ImportJobRepository`, `ImportIDMapRepository`
- [ ] New services: `ExportService`, `ImportService`
- [ ] New workers: `ExportWorker`, `ImportWorker`
- [ ] New handler: `DataPortabilityHandler` registered on Echo server
- [ ] `SetEventBus()` wired for export/import event publishing
- [ ] Workers started with `worker.Start(ctx)`
- [ ] Graceful shutdown order: server → eventBus → workers → enforcer → db → redis
- [ ] `make serve` starts successfully with all new components

---

### T022 — EventBus Integration: Export/Import Event Publishing

**Priority**: P1 | **Depends On**: T004, T012, T014 | **Est.**: 45m

**Goal**: Wire export/import services to publish lifecycle events via EventBus. Events: `export.created`, `export.completed`, `export.failed`, `import.created`, `import.completed`, `import.failed`. Follow existing `SetEventBus()` pattern.

**Files**:
- `internal/service/export_service.go` (add EventBus calls)
- `internal/service/import_service.go` (add EventBus calls)
- `cmd/api/main.go` (wire EventBus)

**Acceptance Criteria**:
- [ ] `ExportService` has `SetEventBus(*domain.EventBus)` setter
- [ ] `ImportService` has `SetEventBus(*domain.EventBus)` setter
- [ ] Events published at: job creation, job completion, job failure
- [ ] Events carry `OrgID` for org-scoped dispatch
- [ ] Event payloads include: job ID, status, entity types
- [ ] EventBus integration wired in `cmd/api/main.go`

---

## Phase 2: Enhancement (P2 — Should-Have)

### T023 — Export File TTL Cleanup: Asynq Scheduled Task

**Priority**: P2 | **Depends On**: T013 | **Est.**: 1h

**Goal**: Create an Asynq scheduled task that runs hourly to delete expired export files (whose `file_expires_at` < now). Uses `StorageDriver.Delete()` for physical file removal and updates `ExportJob` status. Configured via `EXPORT_FILE_TTL` env var (default: 24h).

**Files**:
- `internal/service/export_cleanup.go`

**Acceptance Criteria**:
- [ ] Asynq scheduled task runs every hour
- [ ] Finds `ExportJob` records where `file_expires_at < NOW()`
- [ ] Deletes physical file via `StorageDriver.Delete()`
- [ ] Sets `ExportJob.FilePath = nil`, `ExportJob.HmacSignature = nil`
- [ ] Respects soft deletes (skips deleted jobs)
- [ ] Configurable TTL via `EXPORT_FILE_TTL`

---

### T024–T030 — Unit and Integration Tests

**Priority**: P1 | **Various Depends** | **Est.**: 11.5h total

**T024 — Domain Entity + Registry Unit Tests** (P1, depends T002+T003, 1h)
- Test all entity `ToResponse()` methods
- Test `EntityRegistry.IsExportable()` / `IsImportable()` for all entity types
- Test that restricted entities are always blocked
- Test `ImportOrder` correctness

**T025 — Repository Unit Tests** (P1, depends T005+T006+T007, 1.5h)
- Test CRUD operations for ExportJobRepository, ImportJobRepository, ImportIDMapRepository
- Test soft delete scopes
- Test pagination
- Test `ResolveUUID` mapping
- Use `testify/mock` (no Docker needed)

**T026 — FormatEncoder/FormatDecoder Unit Tests** (P1, depends T010+T011, 1.5h)
- Test JSON encoder stream output (NDJSON line-by-line)
- Test JSON decoder: valid input, invalid format, entity ordering, context cancellation
- Test CSV encoder/decoder round-trip
- Test CSV rejection of junction entities
- Test validation: row count limits, missing required fields

**T027 — ExportService Unit Tests** (P1, depends T012, 1.5h)
- Test sync path (small export streams directly)
- Test async path (large export creates job + enqueues)
- Test cancel: queued → cancelled, processing → cancelled
- Test rate limiting: exceeded → 429
- Test permission denied → 403
- Test restricted entity type → 403

**T028 — ImportService Unit Tests** (P1, depends T014, 2h)
- Test preview validation (metadata checks, no writes)
- Test topological processing order
- Test conflict strategies: skip, overwrite, fail
- Test UUID mapping (external → internal resolution)
- Test HMAC verification failure → rejection
- Test cancel: stops after current entity type batch

**T029 — HMAC, Rate Limiter, Validator Unit Tests** (P1, depends T008+T009+T016, 1.5h)
- Test sign/verify round-trip for HMAC
- Test `whsec_` prefix stripping
- Test rate limiter: allow/deny, Redis failure fallback
- Test validator: file size, format, entity count, CSRF/injection patterns

**T030 — Integration Tests: Full Round-Trip** (P1, depends T021, 3h)
- Test full export → import cycle using testcontainers
- Test: create org with users/roles/permissions → export → download NDJSON → import into new org
- Test dry-run preview returns correct conflict report
- Test rate limiting with real Redis
- Test HMAC signature verification end-to-end
- Use `NewTestSuite(t)` from `tests/integration/testsuite.go`

---

## Dependency Graph

```
T001 (migration)
├── T002 (domain entities)
│   ├── T005 (export repo)
│   ├── T006 (import repo)
│   └── T007 (id map repo)
│       └── (feeds into T014)

T003 (entity registry)
├── T010 (JSON encoder/decoder)
│   └── T011 (CSV encoder/decoder)
│
T004 (events)
└── T022 (event publishing)

T008 (HMAC signing) ───┐
T009 (rate limiter) ───┤
T016 (validator) ──────┘
    └── (no deps among these three)

T012 (export service) ──┬── T013 (export worker)
│                        └── T017 (export handler)
│
T014 (import service) ──┬── T015 (import worker)
│                        └── T018 (import handler)
│
T019 (middleware) ─── T009
T020 (permissions) ─── (standalone)

T021 (wiring) ── T012 + T013 + T014 + T015 + T017 + T018

T023 (cleanup) ── T013
T024–T030 (tests) ── their respective targets
```

## Parallelization Strategy

The following tasks can be implemented in parallel since they have no mutual dependencies:

**Wave 1** (no dependencies):
- T001 (migration), T003 (registry), T004 (events), T008 (HMAC), T009 (rate limiter), T020 (permissions)

**Wave 2** (depends only on Wave 1):
- T002 (entities → T001), T010 (JSON format → T003), T016 (validator → T008)

**Wave 3** (depends on Wave 2):
- T005+T006+T007 (repos → T002), T011 (CSV → T010), T019 (middleware → T009)

**Wave 4** (core services):
- T012 (export service → T005+T008+T009+T010), T014 (import service → T006+T007+T008+T009+T010)

**Wave 5** (workers + handlers):
- T013 (export worker → T012), T015 (import worker → T014), T017 (export handler → T012+T009), T018 (import handler → T014+T016), T022 (events → T004+T012+T014)

**Wave 6** (integration):
- T021 (wiring → all services/handlers), T023 (cleanup → T013)

**Wave 7** (tests — parallel with Wave 5-6):
- T024–T030 can start as their targets become available