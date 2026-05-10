# File Versioning Implementation - Learnings

## 2026-05-08 Initial Context Gathering

### Project Patterns
- **Module**: `github.com/example/go-api-base`
- **Framework**: Echo v4
- **ORM**: GORM with UUID primary keys and soft deletes
- **Errors**: `pkg/errors` with `AppError` (code, message, HTTPStatus), `ErrNotFound`, `ErrConflict`, `WrapInternal()`
- **Repository pattern**: Interface + private impl + `NewXxxRepository(db)`, all methods use `ctx context.Context` first, `r.db.WithContext(ctx)` for GORM ops
- **Service pattern**: Concrete struct (not interface), constructor `NewXxxService(...)`, wraps repo errors with `errors.WrapInternal()`, returns `errors.NewAppError()` for business errors
- **Handler pattern**: Struct with service + auditSvc + enforcer fields, `RegisterRoutes(g *echo.Group, jwtSecret)`, extract userID via `middleware.GetUserID(c)`, permission checks via `middleware.RequirePermission(enforcer, resource, action)`
- **Response envelope**: `response.SuccessWithContext(c, code, data)`, `response.ErrorWithContext(c, code, msg)` — all responses wrapped in Envelope struct
- **Audit logging**: `AuditService.LogAction(ctx, actorID, action, resource, resourceID, before, after)` — 7 param, async via channel
- **Permissions**: `DefaultPermissions()` in `cmd/api/main.go` — `PermissionManifest` struct with `Permissions []PermissionEntry{Name, Description, Resource, Action}` and `Roles []RoleEntry{Name, Description, Permissions}`, synced via `permission:sync` command to Casbin

### Media System Architecture
- `Media` entity in `internal/domain/media.go` — has `UploadedByID`, `Conversions`, `ToResponse()`, etc.
- `MediaRepository` interface in `internal/repository/media.go` — CRUD + downloads
- `MediaService` in `internal/service/media.go` — `Upload()`, `List()`, `Get()`, `Delete()`, `GetSignedURL()`, `ValidateSignedURL()`, `CheckPermission()`, uses `storage.Driver`
- `MediaHandler` in `internal/http/handler/media.go` — 949 lines, `RegisterRoutes()`, multipart upload, signed URLs
- Routes: `POST /models/:model_type/:model_id/media`, `GET /media/:media_id`, `GET /media/:media_id/download`, etc.
- Server wiring in `internal/http/server.go:362-537` — creates repos, services, handlers

### Key Files to Modify/Create
- **Create**: `internal/domain/media_version.go` — MediaVersion entity + DTOs
- **Modify**: `internal/domain/media.go` — add `CurrentVersion`, `Versions`, update `ToResponse()`
- **Create**: `internal/repository/media_version.go` — MediaVersionRepository interface + GORM impl
- **Create**: `internal/service/media_version.go` — VersionService (or UploadVersion, ListVersions, etc.)
- **Create**: `internal/http/handler/media_version.go` — MediaVersionHandler with 7 endpoints
- **Create**: `internal/http/request/media_version.go` — Request DTOs
- **Create**: `migrations/000019_create_media_versions.up.sql` + `.down.sql`
- **Modify**: `cmd/api/main.go` — add media_version:* permissions + DI wiring
- **Modify**: `internal/http/server.go` — add VersionService/Handler wiring + routes

### Database Conventions
- Last migration: `000018_refresh_token_is2fa_pending`
- Next: `000019_create_media_versions`
- Uses `gen_random_uuid()` for UUIDs
- Uses `update_updated_at_column()` trigger function
- Soft deletes with `deleted_at TIMESTAMPTZ DEFAULT NULL`
- All FK references use UUID type

## 2026-05-08 Phase 1 Completed — Migrations + Permissions

### T001-T002: Migration Files Created
- `migrations/000019_create_media_versions.up.sql` — creates `media_versions` table with 6 constraints/indexes + trigger + ALTER TABLE on media
- `migrations/000019_create_media_versions.down.sql` — drops trigger, table, and column

### T003: Permissions Added
- Added 5 `media_version:*` permissions to `DefaultPermissions()`:
  - `media_version:upload` — Upload new media versions
  - `media_version:view` — View media version history
  - `media_version:download` — Download specific media versions
  - `media_version:restore` — Restore previous media versions
  - `media_version:delete` — Delete specific media versions (admin only)
- All 5 added to admin role's Permissions list

### Verification
- `go build ./...` passes with zero errors

## 2026-05-08 Phase 2 Completed — Domain Entity + Repository + Checksum

### T004: MediaVersion Domain Entity
- Created `internal/domain/media_version.go` with:
  - `MediaVersion` struct — UUID PK, soft delete, FK to Media (CASCADE) and User (RESTRICT)
  - `MediaVersionResponse` — DTO without `FilePath` (internal), with `IsCurrent` computed
  - `VersionHistoryResponse` — version history wrapper with pagination total
  - `IsCurrent(version int)` — checks if this version is the current one
  - `IsDeleted()` — checks `DeletedAt.Valid`
  - `ToResponse(currentVersion int)` — converts entity to response DTO

### T005: Media Entity Extension
- Modified `internal/domain/media.go` (4 atomic edits):
  1. Added `CurrentVersion int` field (gorm default:1, after `Size`)
  2. Added `Versions []*MediaVersion` relation (after `Conversions`, CASCADE delete)
  3. Added `CurrentVersion int`, `VersionCount int`, `Versions []*MediaVersionResponse` to `MediaResponse`
  4. Updated `ToResponse()` to populate new fields + version DTOs (similar to conversions pattern)
- `URL` field kept empty in ToResponse with comment noting handler population

### T006: MediaVersionRepository
- Created `internal/repository/media_version.go` with:
  - `MediaVersionRepository` interface — 8 methods
  - GORM implementation following exact project patterns:
    - `FindByMediaID` — paginated with WHERE `deleted_at IS NULL`, ORDER BY version DESC
    - `FindCurrentVersion` — finds by mediaID + currentVersion + not soft-deleted
    - `FindByChecksum` — finds by mediaID + checksum + not soft-deleted (duplicate detection)
    - `CountByMediaID` — counts non-deleted versions
    - All methods use `r.db.WithContext(ctx)`, map `gorm.ErrRecordNotFound` to `errors.ErrNotFound`
    - SoftDelete checks `RowsAffected == 0` for ErrNotFound

### T007: SHA-256 Checksum Utility
- Created `internal/service/media_version.go` with `ComputeSHA256Checksum(io.Reader) (string, error)`
- Created `tests/unit/media_version_service_test.go` with 3 test cases:
  - Empty string (known SHA-256: e3b0c442...)
  - "hello world" (known SHA-256: b94d27b9...)
  - Large data (1MB, verifies 64-char hex output)

### Verification
- `go build ./...` passes with zero errors
- `go vet ./internal/domain/ ./internal/repository/ ./internal/service/` passes
- LSP diagnostics clean on all 5 changed files
- Pre-existing `job_worker_test.go` compilation issue blocks unit test execution (unrelated)
## Bug Fixes Applied (2026-05-08)

### media_version.go Upload handler bugs fixed

**Bug 1 - Missing return on error (CRITICAL)**
- Line 91: `h.mapServiceError(c, svcErr)` was called without return
- Result: Error response written, then success response also written (duplicate response)
- Fix: Added `return` before `h.mapServiceError(c, svcErr)`

**Bug 2 - Audit log called on error**
- Line 94: `h.logUploadAudit()` was called even when `version` is nil on error path
- Fix: Moved audit logging AFTER the error check block (now only called on success)

**Bug 3 - Wrong currentVersion in response**
- Line 96: `version.ToResponse(0)` passed `0` as currentVersion, making `is_current` always false
- Fix: Changed to `version.ToResponse(version.Version)` since this IS the current version

**Bug 4 - mapServiceError return type**
- `mapServiceError` was `void` function, causing bug 1
- Fix: Changed return type to `error` so handler can return the result

All 3 bugs fixed, project builds successfully.
