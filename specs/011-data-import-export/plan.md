# Implementation Plan: Data Import/Export System

**Branch**: `011-data-import-export` | **Date**: 2026-05-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/011-data-import-export/spec.md`

## Summary

Build a data import/export system for the Go REST API supporting JSON (primary) and CSV (secondary) formats with FormatEncoder/FormatDecoder strategy pattern. Exports stream synchronously for <10K rows and async via Asynq for ≥10K rows. Imports are always async with topological dependency ordering, batch processing (500/transaction), and UUID resolution via `import_id_maps` table. Features 5 Casbin permissions, hardcoded entity exportability registry, rate limiting, dry-run preview, and HMAC-SHA256 file signatures.

## Technical Context

**Language/Version**: Go 1.23+
**Primary Dependencies**: Echo v4, GORM, Asynq (Redis), Casbin, PostgreSQL
**Storage**: PostgreSQL (primary), Redis (queue, rate limits, idempotency cache), local/S3 (export files)
**Testing**: testify (unit), testcontainers-go (integration)
**Target Platform**: Linux server (Docker)
**Project Type**: REST API web service
**Performance Goals**: <5s for 10K record export, <60s for 100K, 500 records/s import throughput
**Constraints**: 50MB max file size, 10K max entities per import, <2s handler response time
**Scale/Scope**: 7 MVP entity types, ~29 total entities (v2+)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready | ✅ PASS | Error handling, logging, graceful shutdown all required |
| II. RBAC Not Hardcoded | ✅ PASS | 5 permissions added to DB via permission:sync, entity registry is configurable |
| III. Soft Deletes | ✅ PASS | Import never resurrects soft-deleted records; new UUIDs generated |
| IV. Stateless JWT | ✅ PASS | No JWT changes needed; existing auth flows used |
| V. PostgreSQL + Migrations | ✅ PASS | SQL migrations only (000020_*.sql), no AutoMigrate |
| VI. Perm Consistency | ✅ PASS | 5 new permissions added to sync manifest |
| VII. Audit Logging | ✅ PASS | 1 audit_log per job, not per record; references job_id |
| VIII. Org-Scoped Multi-Tenancy | ✅ PASS | Exports filtered by org_id; imports scoped to target org |
| IX. Event-Driven | ✅ PASS | EventBus integration for export/import completion events |
| X. Background Processing | ✅ PASS | Asynq workers for async jobs, reusing existing job infrastructure |

**Gate: PASS** — All constitutional principles satisfied.

## Project Structure

### Documentation (this feature)

```text
specs/011-data-import-export/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── encoder.md       # FormatEncoder interface contract
│   ├── decoder.md       # FormatDecoder interface contract
│   ├── export-service.md
│   ├── import-service.md
│   └── handler.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── export_job.go           # ExportJob entity, DTOs, ToResponse()
│   ├── import_job.go           # ImportJob entity, DTOs, ToResponse()
│   ├── import_id_map.go        # ImportIDMap entity
│   ├── data_portability.go     # EntityExportabilityRegistry, Exportable interface, conflict strategies
│   └── data_portability_events.go  # EventBus events (export.completed, import.completed, import.failed)
├── repository/
│   ├── export_job.go           # ExportJobRepository interface + GORM impl
│   ├── import_job.go           # ImportJobRepository interface + GORM impl
│   └── import_id_map.go        # ImportIDMapRepository interface + GORM impl
├── service/
│   ├── export_service.go       # ExportService (sync + async paths, FormatEncoder dispatch)
│   ├── import_service.go       # ImportService (batch processing, UUID mapping, conflict resolution)
│   ├── import_worker.go        # Asynq worker for import processing
│   ├── export_worker.go        # Asynq worker for export processing
│   ├── data_portability_rate_limiter.go  # Rate limiter (adapted from webhook_rate_limiter.go)
│   ├── data_portability_sign.go           # HMAC-SHA256 signing (adapted from webhook_sign.go)
│   ├── format_json.go          # JSON FormatEncoder + FormatDecoder (NDJSON streaming)
│   ├── format_csv.go           # CSV FormatEncoder + FormatDecoder
│   └── data_portability_validator.go     # File validation (size, format, count, injection)
├── http/
│   ├── handler/
│   │   └── data_portability.go  # Export + Import HTTP handlers (9 endpoints)
│   └── middleware/
│       └── data_portability.go  # Rate limiting middleware, file size check
migrations/
├── 000020_create_data_portability.up.sql    # export_jobs, import_jobs, import_id_maps
└── 000020_create_data_portability.down.sql
tests/
├── unit/
│   ├── service/
│   │   ├── export_service_test.go
│   │   ├── import_service_test.go
│   │   ├── format_json_test.go
│   │   ├── format_csv_test.go
│   │   ├── data_portability_rate_limiter_test.go
│   │   ├── data_portability_sign_test.go
│   │   └── data_portability_validator_test.go
│   └── domain/
│       └── data_portability_test.go  # EntityExportabilityRegistry tests
└── integration/
    └── data_portability_test.go  # Full round-trip tests (testcontainers)
```

## Phase 0: Research

### Key Research Questions

1. **Asynq integration**: How does the existing job system wire workers? What's the handler signature?
2. **StorageDriver interface**: How does the existing media storage driver work? Can we reuse it for export file storage?
3. **EventBus pattern**: How do services publish/subscribe to events? What's the event struct format?
4. **Permission sync manifest**: Where is it? How do we add new permissions?
5. **Existing rate limiter**: How does webhook_rate_limiter.go work? What's the interface?
6. **Export format**: Do we need a streaming JSON encoder or can we use `json.Encoder` with newline delimiting?

### Research Outputs

All research will be documented in `research.md`.

## Phase 1: Design

### Data Model

See `data-model.md` for full entity definitions.

### Key Design Decisions

1. **FormatEncoder/FormatDecoder strategy pattern** — Zero cost abstraction. Adding a format = implement 2 interfaces + register in factory map. No service changes.

2. **import_id_maps table over per-entity external_id** — 1 migration vs 29. Full provenance (job_id, entity_type, external_id, internal_id). Queryable. No domain model pollution.

3. **Always-async import** — Even small imports risk handler blocking (1K enforce calls = 1s). Handler validates metadata, enqueues job, returns job ID. Worker processes in background.

4. **Sync+async export** — Small exports (<10K) stream directly. Large exports go through Asynq. Threshold configurable via env var.

5. **Hardcoded entity exportability registry** — Not in Casbin. Security-sensitive entities (api_keys, two_factor, etc.) are blocked at the service layer regardless of policy. This is MAC (mandatory access control), not DAC (discretionary).

6. **Topological import order** — Hardcoded for v1: organizations → roles → permissions → users → org_members → user_roles → user_permissions. No DAG engine needed.

7. **Per-entity-type transactions** — Each entity type is committed in its own transaction. If users fail, organizations remain valid. Import result reports per-entity-type success/failure.

8. **Dry-run = metadata validation only** — No database queries during preview. Checks headers, field names, types, row count. Full validation happens during actual commit.

9. **HMAC-SHA256 file signatures** — Reuse pattern from webhook_sign.go. Sign on export, verify on import. No Merkle trees.

### Contracts

See `contracts/` directory for interface definitions.

### API Endpoints

| Method | Path | Description | Auth | Permission |
|--------|------|-------------|------|------------|
| POST | `/api/v1/exports` | Create export job | JWT | `data_portability:export:create` |
| GET | `/api/v1/exports` | List user's exports | JWT | `data_portability:export:download` |
| GET | `/api/v1/exports/:id` | Get export status | JWT | `data_portability:export:download` |
| GET | `/api/v1/exports/:id/download` | Download export file | JWT | `data_portability:export:download` |
| DELETE | `/api/v1/exports/:id` | Cancel queued export | JWT | `data_portability:import:cancel` |
| POST | `/api/v1/imports/preview` | Dry-run validation | JWT | `data_portability:import:create` |
| POST | `/api/v1/imports` | Create import job | JWT | `data_portability:import:create` |
| GET | `/api/v1/imports` | List user's imports | JWT | `data_portability:import:view` |
| GET | `/api/v1/imports/:id` | Get import status/result | JWT | `data_portability:import:view` |
| POST | `/api/v1/imports/:id/cancel` | Cancel processing import | JWT | `data_portability:import:cancel` |

### EventBus Integration

```go
// New events
const (
    EventExportCreated  = "export.created"
    EventExportCompleted = "export.completed"
    EventExportFailed    = "export.failed"
    EventImportCreated  = "import.created"
    EventImportCompleted = "import.completed"
    EventImportFailed    = "import.failed"
)
```

### Configuration (Environment Variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `EXPORT_SYNC_THRESHOLD` | 10000 | Row count threshold for sync vs async export |
| `IMPORT_BATCH_SIZE` | 500 | Records per transaction during import |
| `IMPORT_MAX_FILE_SIZE` | 52428800 | 50MB max upload size |
| `IMPORT_MAX_ENTITY_COUNT` | 10000 | Max entities per import file |
| `IMPORT_RATE_LIMIT_EXPORT` | 10 | Max exports per hour per user |
| `IMPORT_RATE_LIMIT_IMPORT` | 5 | Max imports per hour per user |
| `IMPORT_RATE_LIMIT_RECORDS` | 50000 | Max records per hour per org |
| `EXPORT_FILE_TTL` | 24h | Time before auto-deleting export files |
| `IMPORT_IDEMPOTENCY_TTL` | 24h | Redis TTL for import idempotency keys |
| `IMPORT_CONFLICT_STRATEGY` | skip | Default conflict resolution: skip, overwrite, fail |

## Phase 2: Task Breakdown

*Generated by `/speckit.tasks` command — not part of /speckit.plan output.*