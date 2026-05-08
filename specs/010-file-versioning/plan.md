# Implementation Plan: File Versioning

**Branch**: `010-file-versioning` | **Date**: 2026-05-08 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/010-file-versioning/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add version history tracking to the existing media upload system. Each file upload creates a `MediaVersion` record with version number, SHA-256 checksum, and dedicated storage path (`{media_id}/v{N}/{filename}`). Users can view version history, download specific versions, restore previous versions as current, and (admins) delete specific versions. The existing `Media` entity gains a `current_version` field tracking the active version. Built on the existing storage driver abstraction (local/S3/MinIO), audit service, and Casbin RBAC system.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Echo v4, GORM, golang-migrate, `crypto/sha256` (stdlib), `crypto/hmac` (stdlib)
**Storage**: PostgreSQL (metadata), Local/S3/MinIO (files via existing `storage.Driver` interface)
**Testing**: `testify` + `testcontainers-go` for integration tests, `testify/mock` for unit tests
**Target Platform**: Linux server (Docker)
**Project Type**: web-service (REST API)
**Performance Goals**: Version upload within 2s, version history under 1s for 100 versions, download latency unchanged from existing media download
**Constraints**: ≤200ms p95 on version history listing, optimistic locking prevents concurrent upload race conditions, no hard limit on version count per media
**Scale/Scope**: Extends existing media entity; new `media_versions` table; 7 new API endpoints; migration #000019

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready Foundation | ✅ PASS | Uses existing logger, error handling, graceful shutdown patterns. All operations follow existing handler→service→repository flow. |
| II. RBAC Mandatory, Not Hardcoded | ✅ PASS | New `media_version:*` permissions added to manifest. Route-level `RequirePermission` + handler-level ownership/org checks. No hardcoded permission enums. |
| III. Soft Deletes | ✅ PASS | `media_versions` has `deleted_at`. Version metadata retained on delete; only file removed from storage. |
| IV. Stateless JWT + Revocation | ✅ PASS | All version endpoints use existing JWT middleware. No new token logic. |
| V. PostgreSQL + Versioned Migrations | ✅ PASS | Migration `000019_create_media_versions.up.sql` + `.down.sql`. No GORM AutoMigrate. |
| VI. Multi-Instance Permission Consistency | ✅ PASS | Uses existing Casbin enforcer and pub/sub invalidation. New permissions synced via `permission:sync`. |
| VII. Audit Logging Non-Negotiable | ✅ PASS | All version operations (upload, download, restore, delete) use 9-param `LogAction` with before/after snapshots. |
| VIII. Organization-Scoped Multi-Tenancy | ✅ PASS | Version operations inherit org context from parent media. `X-Organization-ID` header respected at route level. |
| IX. Event-Driven Architecture | ⬜ N/A | No new events emitted. Versioning is a media-internal concern. |
| X. Background Job Processing | ⬜ N/A | All operations are synchronous (upload, download, restore, delete). No background workers needed. |

**Gate Result**: ✅ ALL PASS — No constitution violations. Proceed to implementation.

## Project Structure

### Documentation (this feature)

```text
specs/010-file-versioning/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api-v1.md
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# New files (created by this feature)
internal/
├── domain/
│   └── media_version.go              # MediaVersion entity + response DTOs
├── repository/
│   └── media_version.go              # MediaVersionRepository interface + GORM impl
├── service/
│   └── media_version.go              # VersionService: upload, list, get, download, restore, delete
├── http/
│   ├── handler/
│   │   └── media_version.go          # VersionHandler: 7 endpoints
│   └── request/
│       └── media_version.go          # Version request DTOs + validation

# Modified files (extended by this feature)
internal/
├── domain/
│   └── media.go                      # +CurrentVersion field, +Versions relation, +updated ToResponse()
├── repository/
│   └── media.go                      # +FindByIDWithVersion (or preload Versions)
├── service/
│   └── media.go                      # +Upload creates MediaVersion on first upload
├── http/
│   └── handler/
│       └── media.go                  # +RegisterVersionRoutes, +extended GetByID response
├── http/
│   └── server.go                     # +MediaVersionService DI, +route registration

# Database
migrations/
├── 000019_create_media_versions.up.sql
└── 000019_create_media_versions.down.sql

config/
└── permissions.yaml                   # +media_version:* permissions

# Tests
tests/
├── unit/
│   ├── repository/
│   │   └── media_version_repository_test.go
│   └── service/
│       └── media_version_service_test.go
└── integration/
    └── media_version_test.go          # testcontainers suite
```

**Structure Decision**: Single project — the existing Go API structure is followed. Versioning extends the media module with co-located domain, repository, service, handler files. No new package or module — all files live in existing `internal/*` directories.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations detected. All constitution principles are satisfied.
