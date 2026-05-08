# Tasks: File Versioning

**Input**: Design documents from `/specs/010-file-versioning/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/api-v1.md

**Tests**: This feature requires 80%+ unit + integration test coverage per constitution. Test tasks are included.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4, US5)
- Include exact file paths in descriptions

## Path Conventions

This project uses the Go standard layout:
- Domain entities: `internal/domain/`
- Repositories: `internal/repository/`
- Services: `internal/service/`
- HTTP handlers: `internal/http/handler/`
- Request DTOs: `internal/http/request/`
- Migrations: `migrations/`
- Permissions config: `config/permissions.yaml`
- Unit tests: `tests/unit/`
- Integration tests: `tests/integration/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database migration and permissions foundation that all user stories depend on.

- [ ] T001 Create database migration `migrations/000019_create_media_versions.up.sql` with `media_versions` table (all columns, constraints, indexes from data-model.md) and `ALTER TABLE media ADD COLUMN current_version INTEGER NOT NULL DEFAULT 1`
- [ ] T002 Create rollback migration `migrations/000019_create_media_versions.down.sql` with `DROP TABLE IF EXISTS media_versions` and `ALTER TABLE media DROP COLUMN IF EXISTS current_version`
- [ ] T003 [P] Add `media_version:upload`, `media_version:view`, `media_version:download`, `media_version:restore`, `media_version:delete` permissions to the permissions manifest and verify they sync via `permission:sync`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core domain entity and repository that MUST be complete before ANY user story implementation.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T004 Create `MediaVersion` domain entity in `internal/domain/media_version.go` with all fields from data-model.md (ID, MediaID, Media, Version, Filename, OriginalFilename, MimeType, Size, FilePath, Checksum, UploadedByID, UploadedBy, CreatedAt, UpdatedAt, DeletedAt), `TableName()` method, `IsCurrent()` business method, `IsDeleted()` business method, `MediaVersionResponse` DTO, `ToResponse()` method, and `VersionHistoryResponse` DTO
- [ ] T005 Extend `Media` entity in `internal/domain/media.go` to add `CurrentVersion int` field with `gorm:"default:1;not null"` tag, `Versions []*MediaVersion` relation with `gorm:"foreignKey:MediaID;constraint:OnDelete:CASCADE"`, and update `ToResponse()` to include `CurrentVersion`, `VersionCount`, and `Versions`
- [ ] T006 Create `MediaVersionRepository` interface and GORM implementation in `internal/repository/media_version.go` with methods: `Create`, `FindByID`, `FindByMediaIDAndVersion`, `FindByMediaID` (paginated, excludes soft-deleted), `SoftDelete`, `CountByMediaID`, `FindCurrentVersion` (finds version matching media.current_version), `FindByChecksum` (for duplicate detection)
- [ ] T007 Write SHA-256 checksum utility in `internal/service/media_version.go` (or a dedicated helper) that computes `crypto/sha256` hex digest from an `io.Reader`, to be reused by version upload logic; include unit test in `tests/unit/media_version_service_test.go`

**Checkpoint**: Foundation ready — `MediaVersion` entity, `Media` extension, repository, and checksum utility all exist. User story implementation can now begin.

---

## Phase 3: User Story 1 — Upload New Version (Priority: P1) 🎯 MVP

**Goal**: Allow users to upload a new version of an existing media file, creating a `MediaVersion` record with SHA-256 checksum detection and optimistic locking.

**Independent Test**: Upload a file, then upload a new version. Verify `current_version` increments, both versions accessible, duplicate content rejected with 409 Conflict.

### Implementation for User Story 1

- [ ] T008 [US1] Create `VersionService` in `internal/service/media_version.go` with `UploadVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, file io.Reader, filename string, fileSize int64) (*domain.MediaVersion, error)` method that: (1) finds media by ID, (2) retroactively creates v1 MediaVersion if none exist, (3) detects MIME type and validates against parent media's MIME type, (4) computes SHA-256 checksum, (5) compares checksum against current version (reject 409 if match), (6) generates versioned storage path `{media_id}/v{N}/{filename}`, (7) stores file via storage driver, (8) creates MediaVersion record in DB transaction with optimistic lock on `current_version`, (9) updates media's `current_version`, (10) audit-logs the operation
- [ ] T009 [US1] Create `UploadVersionRequest` DTO in `internal/http/request/media_version.go` with file validation (max size, blocked extensions, MIME type check)
- [ ] T010 [US1] Create `MediaVersionHandler` in `internal/http/handler/media_version.go` with `Upload` handler method that: extracts user from JWT, parses multipart form, validates request, calls `VersionService.UploadVersion`, maps errors, returns 201 with version response; includes `UploadVersion` route registration in `RegisterRoutes` method
- [ ] T011 [US1] Wire `VersionService` and `MediaVersionHandler` into the DI container in `internal/http/server.go`; add route group `POST /api/v1/media/:media_id/versions` with JWT auth and `media_version:upload` permission middleware
- [ ] T012 [US1] Write unit tests for `VersionService.UploadVersion` in `tests/unit/media_version_service_test.go` covering: successful upload, retroactive v1 creation, checksum duplicate rejection (409), MIME type mismatch (400), media not found (404), optimistic lock conflict (409), storage failure rollback
- [ ] T013 [US1] Write integration test for upload version endpoint in `tests/integration/media_version_test.go` covering: upload first version, upload second version, upload duplicate content, upload wrong MIME type

**Checkpoint**: User Story 1 complete — users can upload new versions with checksum deduplication and optimistic locking.

---

## Phase 4: User Story 2 — View Version History (Priority: P1)

**Goal**: Allow users to view the complete version history of a media file with pagination.

**Independent Test**: Upload 3 versions, then GET version history. Verify all 3 versions returned in descending order with correct metadata.

### Implementation for User Story 2

- [ ] T014 [US2] Add `ListVersions(ctx context.Context, mediaID uuid.UUID, limit, offset int) ([]*domain.MediaVersion, int64, error)` method to `VersionService` that: (1) finds media by ID (404 if not found or soft-deleted for non-admins), (2) calls repository to get paginated versions excluding soft-deleted, (3) returns `VersionHistoryResponse` with `current_version`, `versions`, and `total`
- [ ] T015 [P] [US2] Add `GetVersion(ctx context.Context, mediaID uuid.UUID, version int) (*domain.MediaVersion, error)` method to `VersionService` that: (1) finds media by ID, (2) finds version by media_id and version number, (3) returns 404/410 for not found / soft-deleted versions
- [ ] T016 [US2] Add `ListVersions` and `GetVersion` handler methods to `MediaVersionHandler` in `internal/http/handler/media_version.go` with permission checks (`media_version:view`), pagination query params (`limit`, `offset`), error mapping
- [ ] T017 [US2] Register routes in `RegisterRoutes`: `GET /api/v1/media/:media_id/versions` (list), `GET /api/v1/media/:media_id/versions/:version` (get by version number); add JWT auth and `media_version:view` permission middleware
- [ ] T018 [US2] Update `MediaHandler.GetByID` response in `internal/http/handler/media.go` to include `current_version` and `version_count` fields when the media has versions (preload `Versions` relation conditionally)
- [ ] T019 [US2] Write unit tests for `ListVersions` and `GetVersion` in `tests/unit/media_version_service_test.go` covering: successful listing with pagination, media not found, version not found, soft-deleted version return code 410
- [ ] T020 [US2] Write integration tests for version history endpoint in `tests/integration/media_version_test.go` covering: list versions with pagination, get specific version, get version of soft-deleted media (404), get non-existent version (404)

**Checkpoint**: User Stories 1 AND 2 complete — users can upload versions and view history.

---

## Phase 5: User Story 3 — Download Specific Version (Priority: P2)

**Goal**: Allow users to download a specific version of a file (not just the current one) and generate signed URLs for version downloads.

**Independent Test**: Upload 2 versions, then download version 1. Verify correct file content served and audit log entry created.

### Implementation for User Story 3

- [ ] T021 [US3] Add `DownloadVersion(ctx context.Context, mediaID uuid.UUID, version int, ipAddress, userAgent string) (io.ReadCloser, *domain.MediaVersion, error)` method to `VersionService` that: (1) finds media by ID, (2) finds version by media_id and version number (404 if not found, 410 if soft-deleted), (3) retrieves file from storage driver, (4) creates `MediaDownload` audit record for the version, (5) audit-logs the download, (6) returns file stream and version metadata
- [ ] T022 [P] [US3] Add `GetVersionSignedURL(ctx context.Context, mediaID uuid.UUID, version int, expiresIn int) (string, time.Time, error)` method to `VersionService` that: (1) validates media and version exist, (2) reuses existing `storage.Signer` for signed URL generation with version-specific path, (3) returns signed URL and expiry
- [ ] T023 [US3] Add `DownloadVersion` and `GetVersionSignedURL` handler methods to `MediaVersionHandler` in `internal/http/handler/media_version.go` with: `DownloadVersion` streams file with `Content-Disposition` and `Content-Type` headers, `GetVersionSignedURL` returns JSON with `url`, `expires_at`, `expires_in`; both check `media_version:download` permission
- [ ] T024 [US3] Register routes: `GET /api/v1/media/:media_id/versions/:version/download` and `GET /api/v1/media/:media_id/versions/:version/url`; add JWT auth and `media_version:download` permission middleware
- [ ] T025 [US3] Write unit tests for `DownloadVersion` and `GetVersionSignedURL` covering: successful download, version not found (404), soft-deleted version (410), file missing from storage (404), signed URL generation and expiry validation
- [ ] T026 [US3] Write integration tests for download endpoint and signed URL endpoint

**Checkpoint**: User Stories 1, 2, AND 3 complete — users can upload, view history, and download any version.

---

## Phase 6: User Story 4 — Restore a Previous Version (Priority: P2)

**Goal**: Allow users with `media_version:restore` permission to promote a previous version as current without creating a new version.

**Independent Test**: Upload 3 versions, then restore version 1. Verify `current_version` updates to 1, no new version created, audit log captures restore action.

### Implementation for User Story 4

- [ ] T027 [US4] Add `RestoreVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, ipAddress, userAgent string) (*domain.Media, error)` method to `VersionService` that: (1) finds media by ID, (2) finds target version by media_id and version number, (3) validates version is not already current (400 if so), (4) validates version is not soft-deleted (410 if so), (5) checks `media_version:restore` permission via enforcer, (6) updates `media.current_version` in DB transaction with optimistic lock, (7) audit-logs with `before: {current_version: N}` and `after: {current_version: M}`
- [ ] T028 [US4] Add `RestoreVersion` handler method to `MediaVersionHandler` in `internal/http/handler/media_version.go` with: permission check (`media_version:restore`), ownership check for non-admins, parse version from URL param, call service, return 200 with `previous_version`, `restored_version`, `current_version`
- [ ] T029 [US4] Register route: `POST /api/v1/media/:media_id/versions/:version/restore` with JWT auth and `media_version:restore` permission middleware
- [ ] T030 [US4] Write unit tests for `RestoreVersion` covering: successful restore, already-current version (400), version not found (404), soft-deleted version (410), permission denied (403), optimistic lock conflict
- [ ] T031 [US4] Write integration test for restore endpoint in `tests/integration/media_version_test.go`

**Checkpoint**: User Stories 1–4 complete — users can upload, view, download, and restore versions.

---

## Phase 7: User Story 5 — Delete a Specific Version (Priority: P3)

**Goal**: Allow admins to soft-delete a specific version's file from storage while preserving the metadata record for audit.

**Independent Test**: Upload 3 versions, then soft-delete version 2. Verify version 2 has `deleted_at` set, file removed from storage, versions 1 and 3 still accessible.

### Implementation for User Story 5

- [ ] T032 [US5] Add `DeleteVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, isAdmin bool, ipAddress, userAgent string) error` method to `VersionService` that: (1) finds media by ID, (2) finds target version, (3) validates version is not the current version (400 if current), (4) validates user has `media_version:delete` permission (admin only), (5) removes file from storage via `storageDriver.Delete()`, (6) soft-deletes version record (sets `deleted_at`), (7) audit-logs with before snapshot of version metadata; note: if storage deletion fails after DB soft-delete, log error but don't undo the soft-delete
- [ ] T033 [US5] Add `DeleteVersion` handler method to `MediaVersionHandler` in `internal/http/handler/media_version.go` with: admin-only check, parse version from URL param, call service, return 200 with deletion confirmation message
- [ ] T034 [US5] Register route: `DELETE /api/v1/media/:media_id/versions/:version` with JWT auth and `media_version:delete` permission middleware (admin-only)
- [ ] T035 [US5] Write unit tests for `DeleteVersion` covering: successful soft-delete with file removal, current version deletion rejected (400), version not found (404), already-deleted version (409), permission denied for non-admin (403), storage deletion failure (soft-delete persists)
- [ ] T036 [US5] Write integration test for delete version endpoint in `tests/integration/media_version_test.go`

**Checkpoint**: All 5 user stories complete — full versioning lifecycle implemented.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final integration, documentation updates, and quality assurance.

- [ ] T037 [P] Update `internal/http/server.go` DI wiring to ensure `VersionService` receives `AuditService`, `storage.Driver`, `Enforcer`, and `MediaVersionRepository` dependencies correctly; verify all 7 version routes are registered with proper middleware chains
- [ ] T038 [P] Update `AGENTS.md` with File Versioning feature documentation (WHERE TO LOOK table entry, CODE MAP entries for `MediaVersion`, `VersionService`, `MediaVersionHandler`, key methods, endpoints table, migration reference, configuration)
- [ ] T039 [P] Update `docs/FEATURE_STATUS.md` to mark File Versioning as ✅ FULLY IMPLEMENTED with component locations
- [ ] T040 Run `go build ./...` and ensure zero compilation errors; run `golangci-lint run ./...` and fix any linting issues
- [ ] T041 Run `go test -v -race ./tests/unit/...` and `go test -v -tags=integration ./tests/integration/... -timeout 5m` to verify all tests pass with 80%+ coverage on business logic

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (migration must exist for domain entities)
- **US1 (Phase 3)**: Depends on Phase 2 — core MVP
- **US2 (Phase 4)**: Depends on Phase 2 — can start in parallel with US1 after Phase 2
- **US3 (Phase 5)**: Depends on Phase 2 and US1 (needs upload to have versions to download)
- **US4 (Phase 6)**: Depends on Phase 2 and US2 (needs version listing for restore workflow)
- **US5 (Phase 7)**: Depends on Phase 2 and US2 (needs version listing for delete workflow)
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

```
Phase 1 (Setup) ──► Phase 2 (Foundational) ──┬──► US1 (Upload) ──► US3 (Download)
                                                ├──► US2 (History) ──┬──► US4 (Restore)
                                                │                      └──► US5 (Delete)
                                                └──► (US1 + US2 can proceed in parallel)
```

- **US1 (Upload)**: Foundation only — core MVP, must complete first for meaningful version operations
- **US2 (History)**: Foundation only — listing versions is independent of upload but needs the same MediaVersion entity
- **US3 (Download)**: Depends on US1 (needs versions to exist) — can start after US1
- **US4 (Restore)**: Depends on US2 (needs to see version history) — can start after US2
- **US5 (Delete)**: Depends on US2 (needs version listing for delete context) — can start after US2

### Within Each User Story

- Domain entity/repo before service
- Service before handler
- Handler before route registration
- Test after implementation
- Route registration and DI wiring are the final integration step

### Parallel Opportunities

- T001, T002, T003 can run in parallel (different files)
- T004, T005 can run in parallel (domain entity files)
- US1 and US2 can run in parallel after Phase 2
- US3 and US4 can run in parallel after their respective dependencies
- T037, T038, T039, T040, T041 can all run in parallel during Polish phase

---

## Parallel Example: User Story 1

```bash
# After Phase 2, launch US1 and US2 in parallel:
# Developer A: US1 (Upload New Version)
Task T008: Create VersionService.UploadVersion in internal/service/media_version.go
Task T009: Create UploadVersionRequest DTO in internal/http/request/media_version.go

# Developer B: US2 (View Version History) — can start in parallel
Task T014: Add ListVersions to VersionService
Task T015: Add GetVersion to VersionService (parallel with T014)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (migration + permissions)
2. Complete Phase 2: Foundational (entity, repo, checksum utility)
3. Complete Phase 3: User Story 1 (Upload New Version)
4. **STOP and VALIDATE**: Upload a file → upload new version → verify version created with checksum
5. Deploy/demo if ready — this is the minimum viable feature

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 (Upload) → Test upload + checksum dedup → **MVP!**
3. Add US2 (History) → Test version listing → Deploy/Demo
4. Add US3 (Download) → Test version download → Deploy/Demo
5. Add US4 (Restore) → Test version restore → Deploy/Demo
6. Add US5 (Delete) → Test version delete → Deploy/Demo
7. Polish → Final integration, lint, docs

### Parallel Team Strategy

With multiple developers after Phase 2:

- Developer A: US1 (Upload) → then US3 (Download)
- Developer B: US2 (History) → then US4 (Restore) → then US5 (Delete)

Each story adds value without breaking previous stories.

---

## Notes

- [P] tasks can run in parallel (different files, no shared dependencies)
- [Story] label maps each task to its user story for traceability
- Each user story is independently completable and testable
- Verify tests pass after each phase
- Commit after each task or logical group
- Stop at any checkpoint to validate independently
- The `config/permissions.yaml` file may not exist yet — create it or add to the existing permissions config mechanism used by `permission:sync`
- Existing media handler GET endpoint should be updated (T018) to include version info, but backward compatibility must be maintained (new fields are additive in JSON response)