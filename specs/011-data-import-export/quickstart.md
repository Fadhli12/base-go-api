# Quick Start: Data Import/Export System

## Prerequisites

- Go 1.23+
- PostgreSQL (running)
- Redis (running)
- Asynq server (configured via existing setup)

## Setup

### 1. Run Migrations

```bash
make migrate
# Or: go run ./cmd/api migrate
```

### 2. Sync Permissions

```bash
go run ./cmd/api permission:sync
# Adds: data_portability:export:create, data_portability:export:download,
#       data_portability:import:create, data_portability:import:view,
#       data_portability:import:cancel
```

### 3. Start the Server

```bash
make serve
```

## Quick Usage

### Export Users (Async - Large Dataset)

```bash
# Create export job
curl -X POST http://localhost:8080/api/v1/exports \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -H "X-Organization-ID: <org-id>" \
  -d '{
    "entity_types": ["users", "roles", "permissions"],
    "format": "json",
    "include_deleted": false
  }'

# Response: { "data": { "id": "...", "status": "queued", ... } }

# Check status
curl http://localhost:8080/api/v1/exports/<id> \
  -H "Authorization: Bearer <token>"

# Download when completed
curl http://localhost:8080/api/v1/exports/<id>/download \
  -H "Authorization: Bearer <token>"
# Returns signed URL redirect or direct NDJSON stream
```

### Export Users (Sync - Small Dataset)

```bash
# Stream directly (under 10K rows)
curl http://localhost:8080/api/v1/exports?entity_types=users&format=json&sync=true \
  -H "Authorization: Bearer <token>" \
  -H "Accept: application/x-ndjson"
```

### Import Users (Dry-Run Preview)

```bash
# Preview import without committing
curl -X POST http://localhost:8080/api/v1/imports/preview \
  -H "Authorization: Bearer <token>" \
  -H "X-Organization-ID: <org-id>" \
  -F "file=@users_export.ndjson"

# Response: Preview report with counts, conflicts, validation errors
```

### Import Users (Commit)

```bash
# Create import job
curl -X POST http://localhost:8080/api/v1/imports \
  -H "Authorization: Bearer <token>" \
  -H "X-Organization-ID: <org-id>" \
  -F "file=@users_export.ndjson" \
  -F "conflict_strategy=skip"

# Response: { "data": { "id": "...", "status": "queued", ... } }

# Check progress
curl http://localhost:8080/api/v1/imports/<id> \
  -H "Authorization: Bearer <token>"
```

### CSV Export

```bash
# Export as CSV (flat entities only)
curl -X POST http://localhost:8080/api/v1/exports \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_types": ["users"],
    "format": "csv"
  }'
```

## Configuration

Set these environment variables (or add to `.env`):

```env
# Export/Import Settings
EXPORT_SYNC_THRESHOLD=10000       # Rows threshold for sync vs async
IMPORT_BATCH_SIZE=500             # Records per transaction
IMPORT_MAX_FILE_SIZE=52428800     # 50MB max upload
IMPORT_MAX_ENTITY_COUNT=10000     # Max entities per import
IMPORT_RATE_LIMIT_EXPORT=10       # Exports per hour per user
IMPORT_RATE_LIMIT_IMPORT=5        # Imports per hour per user
IMPORT_RATE_LIMIT_RECORDS=50000   # Records per hour per org
EXPORT_FILE_TTL=24h              # Auto-delete export files after 24h
IMPORT_CONFLICT_STRATEGY=skip    # Default: skip, overwrite, or fail
```

## Verifying

```bash
# Run unit tests
go test -v ./tests/unit/service/... -run DataPortability

# Run integration tests (requires Docker)
go test -v -tags=integration ./tests/integration/... -run DataPortability -timeout 5m
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "permission denied" on export | Run `go run ./cmd/api permission:sync` |
| Import file rejected as too large | Check `IMPORT_MAX_FILE_SIZE` (default 50MB) |
| Rate limit exceeded | Wait or increase `IMPORT_RATE_LIMIT_*` values |
| Export job stuck in "processing" | Check Asynq dashboard; worker may have crashed |
| "entity not exportable" error | Check hardcoded blocklist; api_keys, two_factor, etc. are blocked |