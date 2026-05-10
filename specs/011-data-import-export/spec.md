# Feature Specification: Data Import/Export System

**Feature Branch**: `011-data-import-export`
**Created**: 2026-05-10
**Status**: Draft (Reviewed by Momus — 1 CRITICAL, 2 HIGH, 4 MEDIUM fixes applied)
**Input**: User description: "Data Import/Export system for base-go-api with JSON/CSV format support, async processing via Asynq, RBAC enforcement, UUID mapping, and dry-run validation. Derived from adversarial multi-agent planning (Hyperplan) with input from Pragmatist, Architect, Strategist, Innovator, Metis, and Momus perspectives."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Export Organization Data (Priority: P1)

As an org admin, I want to export my organization's data (users, roles, permissions, members) in JSON format so that I can back it up, migrate it, or provide it for GDPR compliance.

**Why this priority**: Export is the foundation — without it, import has nothing to work with. The most common use case is data backup and portability.

**Independent Test**: Can be fully tested by creating an org with users/roles/permissions, calling the export endpoint, and verifying the output contains all expected data in valid JSON format.

**Acceptance Scenarios**:

1. **Given** an org admin with `export:create` permission, **When** they POST `/api/v1/exports` with `entity_types: ["users", "roles", "permissions"]` and `format: "json"`, **Then** a background job is created and they receive a job ID
2. **Given** a completed export job, **When** the admin calls GET `/api/v1/exports/:id/download`, **Then** they receive a signed URL to download the NDJSON file
3. **Given** a small export (<10K rows), **When** the admin requests with `Accept: application/x-ndjson`, **Then** the response streams directly without creating a background job
4. **Given** a non-admin user without `export:create` permission, **When** they attempt to create an export, **Then** they receive a 403 Forbidden

---

### User Story 2 - Import Data with Dry-Run Preview (Priority: P1)

As an org admin, I want to import data from a JSON file with a dry-run preview so I can see what would change before committing.

**Why this priority**: Import is the most dangerous operation (creates/modifies data). Dry-run validation is essential for user confidence and data safety.

**Independent Test**: Can be fully tested by uploading a JSON file, calling the preview endpoint, and verifying the response shows expected changes without modifying the database.

**Acceptance Scenarios**:

1. **Given** a superadmin with `import:create` permission, **When** they POST `/api/v1/imports/preview` with a valid JSON file, **Then** they receive a preview report showing record counts, validation errors, and conflicts
2. **Given** a preview that shows no errors, **When** they POST `/api/v1/imports` with the same file and `dry_run: false`, **Then** a background import job is created
3. **Given** a JSON file with duplicate emails, **When** they call the preview endpoint with `on_duplicate: "skip"`, **Then** the preview shows the duplicates would be skipped
4. **Given** a JSON file with 50K+ records, **When** they attempt to import, **Then** they receive a 422 error indicating the file exceeds the 10K record limit

---

### User Story 3 - Import Data with Conflict Resolution (Priority: P2)

As a superadmin, I want to import data and choose how conflicts are handled (skip, overwrite, or fail) so I can control data integrity during migration.

**Why this priority**: Conflict resolution is essential for real migration scenarios but builds on top of the basic import flow.

**Independent Test**: Can be tested by importing data with conflicting UUIDs/emails and verifying each conflict strategy behaves correctly.

**Acceptance Scenarios**:

1. **Given** an import with `on_duplicate: "skip"`, **When** a record with an existing email exists, **Then** the existing record is left unchanged and the import continues
2. **Given** an import with `on_duplicate: "overwrite"`, **When** a record with an existing UUID has different data, **Then** the existing record is updated with the imported data
3. **Given** an import with `on_duplicate: "fail"`, **When** any conflict is detected, **Then** the import job is marked as failed with conflict details

---

### User Story 4 - Export/Import with Organization Scoping (Priority: P2)

As a superadmin, I want to export/import data scoped to a specific organization so that multi-tenant data stays isolated.

**Why this priority**: Org scoping is critical for security but adds complexity. Core export/import should work first.

**Independent Test**: Create two orgs with different users, export org A, and verify org B data is not included.

**Acceptance Scenarios**:

1. **Given** an org admin with `export:create` permission, **When** they export data from their org, **Then** only data belonging to their org is included
2. **Given** a superadmin, **When** they export with `org_id=all`, **Then** data from all organizations is included
3. **Given** an import into org A, **When** the import creates users, **Then** all imported data is scoped to org A

---

### User Story 5 - CSV Format Support (Priority: P3)

As a business user, I want to export and import data in CSV format so I can work with it in spreadsheets.

**Why this priority**: CSV is the human-friendly format. JSON covers the API integration use case first; CSV adds spreadsheet usability.

**Independent Test**: Export users as CSV, verify it opens correctly in a spreadsheet, modify it, and re-import.

**Acceptance Scenarios**:

1. **Given** an export request with `format: "csv"`, **When** the export completes, **Then** the file is a valid CSV with headers matching the entity schema
2. **Given** a CSV file with proper headers, **When** imported, **Then** records are created correctly
3. **Given** a CSV file with CSV injection attempts (`=cmd(...)`, `+cmd(...)`), **When** imported, **Then** malicious cell values are stripped of leading special characters

---

### Edge Cases

- What happens when an import file contains UUIDs that collide with soft-deleted records? → New UUIDs are generated; soft-deleted records are never resurrected
- What happens when an export or import rate limit is exceeded? → Returns 429 Too Many Requests with retry-after header
- What happens when an import file exceeds 50MB? → Returns 413 Payload Too Large
- What happens when an import file contains more than 10,000 entities? → Returns 422 Unprocessable Entity with record count
- What happens when a user tries to export a restricted entity (api_keys, two_factor, etc.)? → Returns 403 Forbidden; entity is in the hardcoded blocklist
- What happens when an async export job times out? → Job is marked as failed after configurable timeout; signed URL is never generated
- What happens when an import job is cancelled mid-processing? → Partially imported records are committed per-entity-type; remaining entity types are skipped
- What happens when an export file's HMAC signature doesn't match on import? → Import is rejected with 400 Bad Request and integrity error details
- What happens when two concurrent imports try to create the same user? → First import wins; second import respects conflict strategy (skip/overwrite/fail)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide async export functionality that streams data in NDJSON format for datasets ≥10K rows via Asynq background jobs
- **FR-002**: System MUST provide sync export for datasets <10K rows that responds directly with Transfer-Encoding: chunked
- **FR-003**: System MUST provide async import functionality via Asynq background jobs with batch processing (500 records per transaction)
- **FR-004**: System MUST support JSON (primary) and CSV (secondary) export/import formats via FormatEncoder/FormatDecoder strategy pattern
- **FR-005**: System MUST process imported entities in topological dependency order: organizations → roles → permissions → users → organization_members → user_roles → user_permissions
- **FR-006**: System MUST maintain an import_id_maps table for UUID resolution during import, mapping external UUIDs to internal UUIDs
- **FR-007**: System MUST support three conflict resolution strategies: skip (default), overwrite, and fail — configurable per import session
- **FR-008**: System MUST never resurrect soft-deleted records during import — new UUIDs are generated for collisions with soft-deleted rows
- **FR-009**: System MUST provide dry-run (preview) import that validates metadata (headers, field names, types, row count) without database writes
- **FR-010**: System MUST enforce RBAC with 5 Casbin permissions: export:create, export:download, import:create, import:view, import:cancel — all org-scoped
- **FR-011**: System MUST maintain a hardcoded entity exportability registry that blocks restricted entities (api_keys, two_factor, refresh_tokens, password_reset_tokens, audit_logs, system_settings) from export/import regardless of Casbin policy
- **FR-012**: System MUST enforce rate limits: 10/hr/user for exports, 5/hr/user for imports, 50K/hr/org for records processed — adapted from webhook_rate_limiter.go pattern
- **FR-013**: System MUST validate import files for: size ≤50MB, entity count ≤10K, JSON depth ≤32 levels, ZIP bombs (max 1000 entries, 100MB uncompressed), CSV injection (strip leading =, +, -, @, tab)
- **FR-014**: System MUST sign export files with HMAC-SHA256 (reusing webhook_sign.go pattern) for integrity verification on import
- **FR-015**: System MUST auto-delete export files after 24 hours (ephemeral storage)
- **FR-016**: System MUST create a single audit_log entry per import/export job (not per record) with job ID reference for detailed traceability
- **FR-017**: System MUST validate permissions per-entity during import processing (NOT just handler-level); import does NOT bypass entity-level access controls
- **FR-018**: System MUST hash PII fields (user email) in exports for non-superadmin users; MUST NEVER export password_hash or two_factor_secret
- **FR-019**: System MUST use idempotency keys (SHA-256 of file content + timestamp) stored in Redis with 24h TTL to prevent duplicate imports
- **FR-020**: System MUST implement ExportCursor interface with keyset pagination (WHERE id > lastID ORDER BY id LIMIT batchSize) for memory-safe streaming

### Non-Functional Requirements

- **NFR-001**: Export performance: <5 seconds for 10K records, <60 seconds for 100K records
- **NFR-002**: Import performance: 500 records/second batch processing rate
- **NFR-003**: Memory safety: Export cursor holds max batchSize (500) records in memory at any time
- **NFR-004**: File storage: Export files stored via existing StorageDriver interface (local/S3)
- **NFR-005**: Format extensibility: Adding a new format requires implementing FormatEncoder + FormatDecoder + registering in factory — zero service changes

## Technical Design *(mandatory)*

### Architecture

```
Export Flow:
  Request → Handler (permission check, rate limit, estimate size)
    → Small (<10K): Sync stream (ExportCursor → FormatEncoder → ResponseWriter)
    → Large (≥10K): Create ExportJob (Asynq) → Worker processes → StorageDriver → Signed URL → EventBus notification

Import Flow:
  Upload → Handler (validate metadata, size, format, HMAC signature) → Create ImportJob (Asynq)
    → Worker: parse → count entities → validate schema
    → process in topological order (per entity type, per batch of 500)
    → UUID resolution via import_id_maps
    → per-entity permission validation
    → job result with success/failure counts per entity type

Dry-Run Flow:
  Upload → Handler (validate metadata) → Create DryRunJob (Asynq)
    → Worker: parse → validate all records WITHOUT writing to DB
    → return preview report (counts, conflicts, validation errors)
```

### Key Abstractions

```go
// Format strategy pattern
type FormatEncoder interface {
    ContentType() string
    FileExtension() string
    Encode(ctx context.Context, cursor ExportCursor, w io.Writer) error
}

type FormatDecoder interface {
    ContentType() string
    FileExtension() string
    CanValidate() bool
    Validate(ctx context.Context, r io.Reader) ValidationResult
    Decode(ctx context.Context, r io.Reader) (<-chan Importable, error)
}

// Streaming cursor
type ExportCursor interface {
    Next(ctx context.Context, batchSize int) ([]Exportable, error)
    HasMore() bool
    Close() error
}

// Entity registry
type EntityExportabilityRegistry struct {
    exportable  map[string]bool    // which entities can be exported
    importable  map[string]bool    // which entities can be imported
    sensitivity map[string]string   // "public", "pii", "restricted", "system"
}
```

### Database Schema

```sql
-- Migration: 000020_create_data_portability.up.sql

CREATE TABLE export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) NOT NULL DEFAULT 'queued', -- queued, processing, completed, failed
    entity_types TEXT[] NOT NULL,
    format VARCHAR(10) NOT NULL DEFAULT 'json', -- json, csv
    org_id UUID,
    created_by UUID NOT NULL,
    file_path VARCHAR(500),
    file_expires_at TIMESTAMP,
    record_count INTEGER,
    error_message TEXT,
    hmac_signature VARCHAR(128),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP -- soft delete
);

CREATE TABLE import_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) NOT NULL DEFAULT 'queued', -- queued, validating, processing, completed, failed, cancelled
    entity_types TEXT[] NOT NULL,
    format VARCHAR(10) NOT NULL DEFAULT 'json',
    org_id UUID,
    created_by UUID NOT NULL,
    conflict_strategy VARCHAR(10) NOT NULL DEFAULT 'skip', -- skip, overwrite, fail
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    source_file_path VARCHAR(500),
    idempotency_key VARCHAR(64) UNIQUE, -- SHA-256 hash for dedup
    result JSONB, -- {entity_type: {created: N, skipped: N, failed: N, overwritten: N}}
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE TABLE import_id_maps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES import_jobs(id),
    entity_type VARCHAR(50) NOT NULL,
    external_id UUID NOT NULL, -- UUID from the import file
    internal_id UUID NOT NULL, -- UUID generated or matched in our DB
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(job_id, entity_type, external_id)
);

CREATE INDEX idx_export_jobs_status ON export_jobs(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_export_jobs_created_by ON export_jobs(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_export_jobs_org_id ON export_jobs(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_import_jobs_status ON import_jobs(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_import_jobs_created_by ON import_jobs(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_import_jobs_idempotency ON import_jobs(idempotency_key);
CREATE INDEX idx_import_id_maps_job ON import_id_maps(job_id);
CREATE INDEX idx_import_id_maps_external ON import_id_maps(entity_type, external_id);
```

### Permissions

```sql
-- New permissions for permission:sync
('data_portability', 'export', 'create'),
('data_portability', 'export', 'download'),
('data_portability', 'import', 'create'),
('data_portability', 'import', 'view'),
('data_portability', 'import', 'cancel');
```

### API Endpoints

| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| POST | `/api/v1/exports` | Create export job (async) or stream (sync) | `data_portability:export:create` |
| GET | `/api/v1/exports` | List user's export jobs | `data_portability:export:download` |
| GET | `/api/v1/exports/:id` | Get export job status | `data_portability:export:download` |
| GET | `/api/v1/exports/:id/download` | Download export file (signed URL) | `data_portability:export:download` |
| DELETE | `/api/v1/exports/:id` | Cancel queued export | `data_portability:import:cancel` |
| POST | `/api/v1/imports/preview` | Dry-run import validation | `data_portability:import:create` |
| POST | `/api/v1/imports` | Create import job | `data_portability:import:create` |
| GET | `/api/v1/imports` | List user's import jobs | `data_portability:import:view` |
| GET | `/api/v1/imports/:id` | Get import job status/result | `data_portability:import:view` |
| POST | `/api/v1/imports/:id/cancel` | Cancel queued/processing import | `data_portability:import:cancel` |

### MVP Entity Scope

**Exportable/Importable (v1)**:
- organizations, roles, permissions, users, organization_members, user_roles, user_permissions

**Blocked from export (hardcoded)**:
- api_keys, two_factor, two_factor_recovery_codes, refresh_tokens, password_reset_tokens

**Blocked from import (hardcoded)**:
- audit_logs (immutable, DB trigger), system_settings (system-managed), email_queue, email_bounces, email_templates

**Out of scope (v2+)**:
- media, media_versions (binary files require different handling), news, webhooks, webhook_deliveries, notifications, notification_preferences, saved_searches, jobs

## Out of Scope *(mandatory)*

- Binary/media file export (different magnitude — requires ZIP archives with file storage)
- Audit log import (blocked by DB trigger, system-managed)
- Multi-entity ZIP bundle with manifest (v2 — dependency DAG complexity)
- Differential/incremental exports (v2 — requires `updated_at` cursor tracking)
- WebSocket export delivery (v2 — signed URL polling is sufficient for v1)
- Cross-system UUID mapping beyond import_id_maps (v2 — single-system migration focus first)
- Protocol Buffers format (v3 — JSON + CSV sufficient for v1)
- XLSX format (v3 — requires additional dependency)
- Import-from-URL (v2 — file upload only for v1)

## Implementation Phases *(mandatory)*

### Phase 1: Foundation (Export + Import Core)
- Migration files (export_jobs, import_jobs, import_id_maps tables)
- Permission sync additions
- Domain entities (ExportJob, ImportJob, ImportIDMap)
- Repository interfaces and GORM implementations
- FormatEncoder/FormatDecoder interfaces + JSON encoder/decoder implementations
- ExportCursor interface with keyset pagination
- ExportService (sync + async paths)
- ImportService (async path only, batch processing)
- Asynq worker integration (export worker, import worker)
- Rate limiter (adapted from webhook_rate_limiter.go)
- HMAC-SHA256 file signing (reusing webhook_sign.go pattern)
- Handler endpoints (9 endpoints)

### Phase 2: CSV Format + Dry-Run
- CSV FormatEncoder and FormatDecoder implementations
- CSV injection sanitization
- Import preview (dry-run) endpoint — metadata validation only
- Import commit endpoint

### Phase 3: Organization Scoping + PII Protection
- Org-scoped export filtering (WHERE organization_id = ?)
- Superadmin `org_id=all` export capability
- PII field hashing in exports for non-superadmin
- Import org-scoping enforcement

### Phase 4: Integration Tests + Hardening
- Integration tests (testcontainers)
- Concurrent import testing
- Rate limit testing
- Security testing (CSV injection, zip bombs, JSON depth bombs)
- Performance testing (10K, 50K, 100K record exports)
- Idempotency key testing

## Test Plan *(mandatory)*

### Unit Tests
- FormatEncoder/FormatDecoder for JSON and CSV
- ExportCursor keyset pagination
- ImportService batch processing and UUID mapping
- Entity exportability registry (blocked entities)
- Conflict resolution strategies (skip, overwrite, fail)
- Rate limiter (adapted from webhook tests)
- HMAC signature generation and verification
- CSV injection sanitization
- JSON depth limit enforcement

### Integration Tests
- Full export → import round-trip (all 7 entity types)
- Export with org scoping
- Import with conflict strategies
- Import preview (dry-run) validation
- Rate limiting enforcement
- Permission enforcement (blocked entities, non-admin access)
- Soft-delete handling (never resurrect)
- PII field hashing
- File size and count limits
- Import idempotency (duplicate file rejection)
- Large dataset export (10K+ records)
- Large dataset import (5K+ records with batch processing)
- Concurrent import race conditions (two imports creating same user)
- Import cancellation semantics (partial commit verification)
- Export file auto-cleanup after 24h TTL
- Cross-entity-type export ordering verification
- NDJSON import file ordering enforcement (out-of-order rejection)

## Momus Review Fixes *(applied 2026-05-10)*

### CRITICAL Fixes

1. **import_jobs.org_id FK with NULL** — org_id is NULLABLE. NULL indicates a global (non-org-scoped) import. PostgreSQL FK allows NULL per SQL standard. Migration must use `org_id UUID REFERENCES organizations(id)` (no NOT NULL constraint).

### HIGH Fixes

2. **ExportCursor is per-entity-type** — ExportCursor is instantiated per-entity-type, not across all types simultaneously. The ExportService iterates entity types sequentially in topological order, creating one cursor per type. The outer loop is entity type order; the inner loop is keyset-paginated within that type.

3. **NDJSON import file ordering** — NDJSON and JSON import files MUST list entity types in topological order (organizations → roles → permissions → users → org_members → user_roles → user_permissions). The decoder validates ordering at parse time and rejects out-of-order files with a descriptive error. Alternative: service buffers all records and reorders, but this has memory implications for large imports (documented).

### MEDIUM Fixes

4. **Dry-run reads DB but doesn't write** — Dry-run performs read-only validation against the database (conflict detection via SELECT queries, schema checks) but makes no writes. Previous wording "metadata-only validation" was misleading — it should say "read-only validation."

5. **Per-entity permission validation** — For imports, Enforce() is called per-entity-type (not per-record). Before processing each entity type batch, a single Enforce() call validates the user can import that entity type. This limits Casbin calls to ~7 per import (one per entity type), not N per record.

6. **CSV format for junction tables** — CSV import supports flat entity types only (users, roles, permissions, organizations). Junction tables (user_roles, user_permissions, organization_members) are CSV-unsupported in v1 and require JSON format. The CSV decoder rejects these entity types with a clear error message.

7. **Export file cleanup** — Export files are auto-cleaned by an Asynq scheduled task that runs hourly, deleting files where file_expires_at < NOW(). This reuses the existing Asynq scheduler pattern.

8. **Concurrent import race conditions** — import_id_maps has UNIQUE(job_id, entity_type, external_id) which is per-job. Cross-import races on unique constraints (e.g., email) are handled by the conflict_strategy: "skip" ignores the error, "overwrite" updates, "fail" aborts. For truly concurrent imports creating the same user, PostgreSQL's unique constraint prevents duplicates at the DB level regardless of the application-level check.

9. **CancelImport semantics** — Cancelling a processing import stops after the current entity type batch completes. Partially committed entity types remain. The ImportResult reports exactly which types succeeded and which were skipped. This is documented behavior, not a bug — the per-entity-type transaction model means partial success is by design.

10. **Exportable interface requirement** — All 7 MVP entity types MUST implement the Exportable interface (GetEntityType(), ToExportResponse(piiHash bool)) before the export feature ships. This is a prerequisite, not optional.