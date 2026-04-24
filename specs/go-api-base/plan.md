# Implementation Plan: Team/Organization Support

**Branch**: `003-organization-support` | **Date**: 2026-04-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/go-api-base/spec.md`

## Summary

Add multi-tenant organization support to enable organization-scoped resources with Casbin RBAC domain integration. Users can create organizations, manage memberships (owner/admin/member roles), and scope all resources via X-Organization-ID header. Implementation extends existing permission system without breaking changes using nullable foreign keys for backward compatibility.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Echo v4, GORM, Casbin, PostgreSQL 15, Redis 7, testcontainers-go
**Storage**: PostgreSQL 15 (GORM ORM), Redis (permission cache)
**Testing**: Go testing package + testify + testcontainers-go for integration tests
**Target Platform**: Linux server (Docker container)
**Project Type**: Web service (REST API)
**Performance Goals**: 1000 req/s sustained with <200ms p99 latency
**Constraints**: <15ms permission check with Redis cache, <5ms org context extraction
**Scale/Scope**: Multi-tenant SaaS with thousands of organizations, millions of resources

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Design Check

✅ **I. Production-Ready Foundation**
- [x] Test coverage: Will add unit + integration tests (80%+ coverage target)
- [x] Error handling: All external calls wrapped with standardized errors
- [x] Observability: Structured logging via slog, correlation IDs
- [x] Graceful shutdown: Maintained (no changes to shutdown logic)

✅ **II. RBAC is Mandatory, Not Hardcoded**
- [x] Using Casbin with domains for organization-scoped permissions
- [x] No permission enums - permissions stored as data (resource:action pairs)
- [x] Organization permissions seeded via permission:sync
- [x] Roles stored in database, assignable at runtime

✅ **III. Soft Deletes for Audit & Compliance**
- [x] Organizations use `deleted_at` for soft deletes
- [x] Organization members deleted via CASCADE (purge on org delete)
- [x] All mutations logged to audit trail

✅ **IV. Stateless JWT + Revocation**
- [x] No changes to JWT system
- [x] Organization context extracted from header (not JWT)
- [x] Existing token format preserved

✅ **V. PostgreSQL + Versioned Migrations**
- [x] Schema changes via SQL migrations (golang-migrate)
- [x] No AutoMigrate used
- [x] Migration files in `migrations/000003_organizations.*.sql`

✅ **VI. Multi-Instance Permission Consistency**
- [x] Organization permissions cached in Redis (existing pattern)
- [x] Pub/sub invalidation on membership changes
- [x] Sub-second propagation across instances

✅ **VII. Audit Logging Non-Negotiable**
- [x] All organization mutations logged (before/after JSONB)
- [x] Actor ID tracked
- [x] Immutable audit log

### Gate Status: PASSED ✓

No constitution violations detected. Design aligns with all core principles.

## Project Structure

### Documentation (this feature)

```text
specs/go-api-base/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output - Casbin domains, context extraction, soft deletes
├── data-model.md        # Phase 1 output - Entity diagrams, relationships
├── quickstart.md        # Phase 1 output - Usage examples
├── contracts/           # Phase 1 output - API contracts
│   └── api-v1.md        # REST endpoint specifications
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
go-api-base/
├── migrations/
│   ├── 000003_organizations.up.sql    # Create organizations + members
│   └── 000003_organizations.down.sql  # Rollback
├── internal/
│   ├── domain/
│   │   ├── organization.go            # Organization entity
│   │   └── organization_member.go     # OrganizationMember entity
│   ├── repository/
│   │   └── organization.go            # OrganizationRepository interface
│   ├── service/
│   │   └── organization.go            # OrganizationService
│   └── http/
│       ├── handler/
│       │   └── organization.go         # HTTP handlers
│       ├── middleware/
│       │   └── organization.go         # Context extraction middleware
│       └── request/
│           └── organization.go         # Request/response DTOs
└── tests/
    └── integration/
        └── organization_test.go       # Integration tests
```

**Structure Decision**: Following existing Go API Base structure with repository pattern, service layer, Echo handlers. No new directories needed - all additions integrate with existing `internal/` structure.

## Complexity Tracking

> **No violations justify complexity tracking.** Implementation follows existing patterns.
