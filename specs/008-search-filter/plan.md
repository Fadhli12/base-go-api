# Implementation Plan: Search & Filtering

**Branch**: `008-search-filter` | **Date**: 2026-05-06 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/008-search-filter/spec.md`

## Summary

Implement full-text search capabilities using PostgreSQL tsvector/tsquery with faceted filtering, saved searches, and autocomplete. The feature enables users to search news articles with relevance ranking, apply multiple filters, save searches for later, and get autocomplete suggestions.

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: Echo v4, GORM, PostgreSQL 12+ (tsvector generated columns), Redis (autocomplete cache)  
**Storage**: PostgreSQL with GIN indexes for full-text search  
**Testing**: go test, testcontainers-go for integration tests  
**Target Platform**: Linux server REST API  
**Project Type**: REST API web-service  
**Performance Goals**: Search <500ms p95, 100 concurrent searches, autocomplete <200ms  
**Constraints**: Soft deletes must exclude results, must use existing repository patterns  
**Scale/Scope**: Support for multiple entity types (news first, extensible), 50 saved searches per user max

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready Foundation | ✅ PASS | 80%+ test coverage, structured logging, graceful shutdown |
| II. RBAC is Mandatory, Not Hardcoded | ✅ PASS | Permissions: search:read, saved_search:create/read/delete |
| III. Soft Deletes for Audit & Compliance | ✅ PASS | All queries exclude soft-deleted via GORM DeletedAt scope |
| IV. Stateless JWT + Revocation | ✅ PASS | Existing JWT pattern unchanged |
| V. PostgreSQL + Versioned Migrations | ✅ PASS | SQL migrations for search_vector and saved_searches |
| VI. Multi-Instance Permission Consistency | ✅ PASS | Existing Redis pub/sub pattern unchanged |
| VII. Audit Logging Non-Negotiable | ✅ PASS | All mutating operations logged via existing AuditService |

**Constitution Gate Result**: ✅ PASS - No violations detected

## Project Structure

### Documentation (this feature)

```text
specs/008-search-filter/
├── plan.md              # This file
├── research.md          # Phase 0 output - PostgreSQL FTS patterns
├── data-model.md       # Phase 1 output - entities, relationships
├── quickstart.md       # Phase 1 output - usage guide
├── contracts/
│   └── search-api.md   # Phase 1 output - API contract
└── spec.md             # Feature specification
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── news.go          # Extended with search_vector annotation
│   └── saved_search.go  # NEW - SavedSearch entity
├── http/
│   ├── handler/
│   │   └── search.go    # NEW - Search & saved-search endpoints
│   ├── request/
│   │   └── search.go    # NEW - Request DTOs
│   └── response/
│       └── search.go    # NEW - Response DTOs
├── repository/
│   ├── search.go       # NEW - SearchRepository interface
│   └── saved_search.go  # NEW - SavedSearchRepository interface
└── service/
    └── search.go       # NEW - SearchService

migrations/
├── 000016_add_news_search.up.sql   # Add search_vector to news
├── 000016_add_news_search.down.sql # Remove search_vector
├── 000017_saved_searches.up.sql   # Create saved_searches table
└── 000017_saved_searches.down.sql  # Drop saved_searches table

tests/
├── unit/
│   └── search_service_test.go  # NEW - SearchService unit tests
└── integration/
    └── search_test.go          # NEW - Search integration tests
```

**Structure Decision**: Following established Go API Base patterns - service layer, repository pattern, GORM entities, Echo handlers.

## Implementation Phases

### Phase 1: Database & Infrastructure

**Tasks**:
- [ ] Create migration 000016_add_news_search.up/down.sql
- [ ] Create migration 000017_saved_searches.up/down.sql
- [ ] Run migrations against development database
- [ ] Create domain entity saved_search.go
- [ ] Create repository interfaces (SearchRepository, SavedSearchRepository)

**Deliverables**:
- `migrations/000016_add_news_search.up.sql` - Adds tsvector generated column with GIN index
- `migrations/000017_saved_searches.up.sql` - Creates saved_searches table
- `internal/domain/saved_search.go` - SavedSearch entity
- `internal/repository/search.go` - Repository interfaces

**Verification**:
- `psql $DATABASE_URL -c "\\d news"` shows search_vector column
- `psql $DATABASE_URL -c "\\d saved_searches"` shows table structure

---

### Phase 2: Service Layer

**Tasks**:
- [ ] Create SearchService with full-text search logic
- [ ] Create SavedSearchService for CRUD operations
- [ ] Implement relevance ranking with ts_rank_cd
- [ ] Implement faceted filtering
- [ ] Implement pagination
- [ ] Apply ts_headline() to generate highlighted excerpts (FR-010)
- [ ] Add structured logging for search operations (FR-011)
- [ ] Add permission checks

**Deliverables**:
- `internal/service/search.go` - SearchService (full-text search, filters, pagination, highlighting, logging)
- `internal/service/saved_search.go` - SavedSearchService (CRUD)

**Verification**:
- Unit tests pass: `go test ./internal/service/... -v -run Search`
- Service methods follow context-first pattern
- Error handling uses apperrors package
- Search query logs appear in structured logs with query text, filters, and user_id

---

### Phase 3: HTTP Handler Layer

**Tasks**:
- [ ] Create search handler with endpoints:
  - GET /api/v1/search - Full-text search
  - GET /api/v1/search/suggestions - Autocomplete
- [ ] Create saved-search handler with endpoints:
  - GET /api/v1/saved-searches - List
  - POST /api/v1/saved-searches - Create
  - GET /api/v1/saved-searches/:id - Get
  - DELETE /api/v1/saved-searches/:id - Delete
- [ ] Create request DTOs with validation
- [ ] Create response DTOs

**Deliverables**:
- `internal/http/handler/search.go` - SearchHandler
- `internal/http/handler/saved_search.go` - SavedSearchHandler
- `internal/http/request/search.go` - Request DTOs
- `internal/http/response/search.go` - Response DTOs

**Verification**:
- `curl -X GET "localhost:8080/api/v1/search?q=test"` returns valid JSON
- Response envelope follows existing pattern
- Input validation returns proper errors

---

### Phase 4: Wire-up & Integration

**Tasks**:
- [ ] Register search handlers in server.go
- [ ] Add new permissions (search:read, saved_search:create, saved_search:read, saved_search:delete) to the DefaultPermissions() function in cmd/api/main.go
- [ ] Run permission:sync to verify new permissions appear in DB
- [ ] Create integration tests

**Deliverables**:
- Updated `cmd/api/main.go` with search routes and new permissions in DefaultPermissions()
- `tests/integration/search_test.go` - Integration tests

**Verification**:
- `go run ./cmd/api permission:sync` shows new permissions (search:read, saved_search:*)
- Integration tests pass: `go test -tags=integration ./tests/integration/... -run Search -timeout 5m`

**Note**: `config/permissions.yaml` does not exist in this project. Permissions are managed via the `DefaultPermissions()` function in `cmd/api/main.go`. The new permissions should be added there directly.

---

### Phase 5: Autocomplete (P3 - Deferred)

**Tasks**:
- [ ] Implement autocomplete suggestions endpoint
- [ ] Add Redis caching for suggestions
- [ ] Track search history for suggestions

**Deliverables**:
- `GET /api/v1/search/suggestions` implementation
- Redis-based suggestion cache

**Status**: Deferred - focus on core search first

---

## Dependencies & Risks

| Dependency | Impact | Mitigation |
|------------|--------|------------|
| PostgreSQL 12+ | Required for generated columns | Check version in setup; fallback to triggers for <12 |
| Redis availability | Autocomplete caching | Graceful degradation - query DB directly if Redis unavailable |

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| FTS performance on large datasets | Low | High | GIN indexes, partial indexes, connection pooling |
| Sync lag between data and search_vector | Very Low | Medium | Generated columns are ACID; zero lag |
| Query injection via special characters | Low | High | Use websearch_to_tsquery() which sanitizes input |

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| SavedSearch uses soft delete (Constitution III) | User preferences need soft delete for undo capability and compliance | Hard delete rejected - users may want to recover deleted saved searches |

**Constitutional Note**: SavedSearch is soft-deleted per Constitution III ("All user-generated data uses soft deletes"). The `deleted_at` column has been added to the migration and entity. This enables potential "undo" functionality and maintains consistency with other user-generated data.
