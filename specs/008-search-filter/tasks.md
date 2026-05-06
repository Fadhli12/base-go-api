---

description: "Task list for Search & Filtering feature implementation"
---

# Tasks: Search & Filtering

**Input**: Design documents from `/specs/008-search-filter/`  
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/  
**Branch**: `008-search-filter`

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database migrations and repository interfaces that all user stories depend on

- [ ] T001 [P] Create migration `migrations/000016_add_news_search.up.sql` - adds tsvector generated column with GIN index to news table
- [ ] T002 [P] Create migration `migrations/000016_add_news_search.down.sql` - removes search_vector column and index
- [ ] T003 [P] Create migration `migrations/000017_saved_searches.up.sql` - creates saved_searches table with soft delete
- [ ] T004 [P] Create migration `migrations/000017_saved_searches.down.sql` - drops saved_searches table
- [ ] T005 [P] Create domain entity `internal/domain/saved_search.go` - SavedSearch struct with soft delete
- [ ] T006 [P] Create repository interfaces `internal/repository/search.go` - SearchRepository and SavedSearchRepository interfaces

**Checkpoint**: Migrations run, domain entities ready, repository interfaces defined

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core service layer that MUST be complete before ANY user story handler implementation

**⚠️ CRITICAL**: No handler work can begin until this phase is complete

- [ ] T007 [P] Implement SearchService in `internal/service/search.go` - full-text search with tsvector/tsquery, relevance ranking, highlighting, structured logging
- [ ] T008 [P] Implement SavedSearchService in `internal/service/saved_search.go` - CRUD operations with user ownership validation
- [ ] T009 [P] Create request DTOs `internal/http/request/search.go` - SearchRequest, SavedSearchCreateRequest structs with validation
- [ ] T010 [P] Create response DTOs `internal/http/response/search.go` - SearchResponse, SavedSearchResponse, SearchResult structs

**Checkpoint**: Service layer complete and independently testable

---

## Phase 3: User Story 1 - Full-Text Search (Priority: P1) 🎯 MVP

**Goal**: User can search news articles using natural language queries and receive ranked, highlighted results

**Independent Test**: User enters a search query and receives ranked results with highlighted matches

### Implementation for User Story 1

- [ ] T011 [US1] Create SearchHandler in `internal/http/handler/search.go` with GET /api/v1/search endpoint
- [ ] T012 [US1] Implement query parsing with `websearch_to_tsquery()` for safe user input
- [ ] T013 [US1] Implement prefix matching using `:*` suffix in tsquery
- [ ] T014 [US1] Implement relevance ranking with `ts_rank_cd()` and normalization
- [ ] T015 [US1] Implement result highlighting with `ts_headline()` after LIMIT (not before)
- [ ] T016 [US1] Add query truncation at 500 characters (per edge case spec)
- [ ] T017 [US1] Add soft delete exclusion via GORM DeletedAt scope

**Checkpoint**: User Story 1 functional - search returns ranked, highlighted results

---

## Phase 4: User Story 2 - Faceted Filtering (Priority: P1)

**Goal**: User can filter search results by status, author, date range with AND logic and facet counts

**Independent Test**: User applies multiple filters (status, date range, author) and sees results update simultaneously

### Implementation for User Story 2

- [ ] T018 [P] [US2] Implement status filter - WHERE status IN (...)
- [ ] T019 [P] [US2] Implement author_id filter - WHERE author_id = UUID
- [ ] T020 [P] [US2] Implement date range filter - WHERE created_at >= date_from AND created_at <= date_to
- [ ] T021 [US2] Implement AND logic for multiple filters (all filters applied simultaneously)
- [ ] T022 [US2] Implement pagination with page/per_page, total count, total_pages in response
- [ ] T023 [US2] Implement sorting options: relevance, created_at, updated_at, title
- [ ] T024 [US2] Implement facet counts for filtering options (NFR-005)

**Checkpoint**: User Story 2 functional - filters work independently and together

---

## Phase 5: User Story 3 - Saved Searches (Priority: P2)

**Goal**: User can save, list, and delete their search queries

**Independent Test**: User saves a search with filters, later retrieves it, and sees identical results

### Implementation for User Story 3

- [ ] T025 [P] [US3] Create SavedSearchHandler in `internal/http/handler/saved_search.go`
- [ ] T026 [P] [US3] Implement GET /api/v1/saved-searches - List user's saved searches
- [ ] T027 [P] [US3] Implement POST /api/v1/saved-searches - Create new saved search with name, query_text, filters
- [ ] T028 [P] [US3] Implement GET /api/v1/saved-searches/:id - Get specific saved search
- [ ] T029 [US3] Implement DELETE /api/v1/saved-searches/:id - Soft delete saved search
- [ ] T030 [US3] Add 50 saved search limit per user check (return 409 LIMIT_EXCEEDED)
- [ ] T031 [US3] Add audit logging for create/delete operations via existing AuditService

**Checkpoint**: User Story 3 functional - saved searches CRUD complete

---

## Phase 6: User Story 4 - Search Suggestions/Autocomplete (Priority: P3)

**Goal**: User sees autocomplete suggestions as they type

**Independent Test**: User types part of a search query and sees dropdown suggestions

### Implementation for User Story 4 (Deferred)

- [ ] T032 [P] [US4] Implement GET /api/v1/search/suggestions endpoint - returns autocomplete suggestions
- [ ] T033 [US4] Implement Redis caching for suggestion cache (graceful degradation if Redis unavailable)
- [ ] T034 [US4] Track search history for suggestions based on user's recent searches

**Checkpoint**: User Story 4 functional - autocomplete suggestions appear

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Integration, testing, and improvements that affect multiple user stories

- [ ] T035 [P] Register search routes in `cmd/api/main.go` - wire up SearchHandler and SavedSearchHandler
- [ ] T036 [P] Add new permissions to `cmd/api/main.go` DefaultPermissions() - search:read, saved_search:create, saved_search:read, saved_search:delete
- [ ] T037 [P] Create unit tests `tests/unit/search_service_test.go` - test SearchService methods
- [ ] T038 [P] Create integration tests `tests/integration/search_test.go` - full search flow with testcontainers
- [ ] T039 Run permission:sync and verify new permissions appear in database
- [ ] T040 [P] Update AGENTS.md - add search & filtering entries to WHERE TO LOOK table
- [ ] T041 Run quickstart.md validation - verify search functionality end-to-end
- [ ] T042 Build and verify no compilation errors: `go build ./...`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 completion - BLOCKS all handler work
- **Phase 3-6 (User Stories)**: All depend on Phase 2 completion
  - US1, US2, US3, US4 can proceed in parallel after Phase 2
- **Phase 7 (Polish)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 2 - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Phase 2 - No dependencies on other stories
- **User Story 3 (P2)**: Can start after Phase 2 - No dependencies on other stories
- **User Story 4 (P3)**: Can start after Phase 2 - Deferred by plan

### Within Each User Story

- Models before services (Phase 1 for domain entities)
- Services before handlers (Phase 2 for services)
- Core implementation before integration

### Parallel Opportunities

- All Phase 1 tasks marked [P] can run in parallel (migrations, domain entity, repository interfaces)
- All Phase 2 tasks marked [P] can run in parallel (services, request/response DTOs)
- US1, US2, US3, US4 can be worked on in parallel by different developers after Phase 2
- All Phase 7 tasks marked [P] can run in parallel (registration, permissions, tests)

---

## Implementation Strategy

### MVP First (User Story 1 + 2)

1. Complete Phase 1: Setup (migrations, domain, repository interfaces)
2. Complete Phase 2: Foundational (services, DTOs)
3. Complete Phase 3: User Story 1 (full-text search with ranking/highlighting)
4. Complete Phase 4: User Story 2 (faceted filtering)
5. **STOP and VALIDATE**: Test search + filter independently
6. Deploy/demo if ready

### Incremental Delivery

1. Complete Phase 1 + Phase 2 → Foundation ready
2. Add US1 → Test independently → Deploy/Demo (MVP!)
3. Add US2 → Test independently → Deploy/Demo
4. Add US3 → Test independently → Deploy/Demo
5. Add US4 (deferred) → Test independently → Deploy/Demo

### Parallel Team Strategy

With multiple developers:

1. Team completes Phase 1 + Phase 2 together (2-3 days)
2. Once Phase 2 is done:
   - Developer A: User Story 1 (full-text search)
   - Developer B: User Story 2 (faceted filtering)
   - Developer C: User Story 3 (saved searches)
3. Stories complete and integrate independently

---

## Notes

- **[P]** tasks = different files, no dependencies, can run in parallel
- **[Story]** label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- T032-T034 (US4) are deferred per plan.md Phase 5
- Verify tests fail before implementing (TDD if tests requested)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
