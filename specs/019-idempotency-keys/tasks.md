# Tasks: Idempotency Keys

**Input**: Design documents from `/specs/019-idempotency-keys/`
**Prerequisites**: plan.md (required), spec.md (required for user stories)

**Tests**: Not explicitly requested in spec, but constitution requires 80%+ coverage on business logic. Test tasks are included for each phase.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Extend cache interface, create config struct, domain entities, and database migration

- [x] T001 Extend `cache.Driver` interface with `SetNX(ctx, key, value, ttlSeconds) (bool, error)` method in `internal/cache/cache.go`
- [x] T002 [P] Implement `SetNX` on `redisCache` using `redis.Client.SetNX()` in `internal/cache/redis.go`
- [x] T003 [P] Implement `SetNX` on `memoryCache` using mutex-protected check-then-set in `internal/cache/memory.go`
- [x] T004 [P] Implement `SetNX` on `NoopCache` returning `(false, nil)` in `internal/cache/noop.go`
- [x] T005 [P] Create `IdempotencyConfig` struct with `mapstructure` tags and `DefaultIdempotencyConfig()` in `internal/config/idempotency.go`
- [x] T006 [P] Add `Idempotency IdempotencyConfig` field to `Config` struct and environment variable parsing in `internal/config/config.go`
- [x] T007 Create `IdempotencyRecord` entity, DTOs (`IdempotencyRecordResponse`, `IdempotencyListResponse`), and status constants (`StatusProcessing`, `StatusCompleted`, `StatusConflict`) in `internal/domain/idempotency.go`
- [x] T008 Create migration `000027_create_idempotency_keys.up.sql` with `idempotency_keys` table, partial unique index (WHERE deleted_at IS NULL), and indexes on `expires_at` and `user_id` in `migrations/`
- [x] T009 [P] Create migration `000027_create_idempotency_keys.down.sql` with DROP INDEX and DROP TABLE in `migrations/`

**Checkpoint**: Cache interface extended, config struct created, domain entity defined, migration ready

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Repository and service layers that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T010 Create `IdempotencyRepository` interface with `Create`, `FindByKey`, `FindByID`, `UpdateStatus`, `CleanupExpired`, `ListByUser` methods in `internal/repository/idempotency.go`
- [x] T011 Implement `GORMIdempotencyRepository` with soft-delete scopes and partial unique index queries in `internal/repository/idempotency.go`
- [x] T012 Create `IdempotencyService` struct with `CreateRecord`, `FindRecord`, `FindRecordByID`, `UpdateRecord`, `ListByUser` methods and embedded reaper goroutine (`Start`/`Stop`/`run`) in `internal/service/idempotency.go`
- [x] T013 Add permission seeds `idempotency:view` and `idempotency:manage` to permission manifest in `cmd/api/main.go`

**Checkpoint**: Foundation ready — repository, service, and permissions exist; user story implementation can now begin

---

## Phase 3: User Story 1 — Duplicate Request Protection (Priority: P1) 🎯 MVP

**Goal**: Core idempotency middleware that prevents duplicate operations from retry mechanisms, returning cached responses for requests with the same `Idempotency-Key` header.

**Independent Test**: Send two identical POST requests with the same `Idempotency-Key` header. The second request must return the same response (status code and body) without re-executing business logic.

### Tests for User Story 1

- [x] T014 [P] [US1] Unit test for `IdempotencyRepository` CRUD operations in `tests/unit/repository/idempotency_test.go`
- [x] T015 [P] [US1] Unit test for `IdempotencyService` methods (Create, Find, Update, List) in `tests/unit/service/idempotency_test.go`
- [x] T016 [P] [US1] Unit test for `IdempotencyMiddleware` — key validation, skip GET/DELETE, Redis fail-open, cache hit, in-flight conflict, payload mismatch in `tests/unit/middleware/idempotency_test.go`
- [x] T017 [US1] Integration test — full request lifecycle with idempotency header (create invoice with key, replay with same key, verify identical response) in `tests/integration/idempotency_test.go`

### Implementation for User Story 1

- [x] T018 [US1] Create `IdempotencyMiddleware` struct with `NewIdempotencyMiddleware(cache, service, config, logger)` constructor and `Middleware()` method returning `echo.MiddlewareFunc` in `internal/http/middleware/idempotency.go`
- [x] T019 [US1] Implement key validation: skip if no header, skip GET/DELETE/OPTIONS/HEAD, validate format regex `^[a-zA-Z0-9_-]+$`, max 128 chars, return 400 if invalid in `internal/http/middleware/idempotency.go`
- [x] T020 [US1] Implement user scoping: extract `user_id` and `org_id` from JWT context; skip if no user_id (unauthenticated); build Redis key `idem:{orgID}:{userID}:{method}:{path}:{key}` in `internal/http/middleware/idempotency.go`
- [x] T021 [US1] Implement Redis check flow: check lock key → if exists with different hash return 409 (payload mismatch); if processing return 409 + Retry-After; if completed check record key → replay ≤4KB from Redis, >4KB from DB in `internal/http/middleware/idempotency.go`
- [x] T022 [US1] Implement Redis SETNX lock: acquire in-flight guard with `cache.SetNX(key, hash, guardTTL)`; if SetNX returns false, return 409 Conflict (race condition) in `internal/http/middleware/idempotency.go`
- [x] T023 [US1] Implement response capture: wrap `http.ResponseWriter` with `idempotencyResponseWriter` (following `audit.go:48-57` pattern) to capture status code and body; skip for WebSocket upgrades in `internal/http/middleware/idempotency.go`
- [x] T024 [US1] Implement response storage: after handler executes, check response body size → if ≤4KB store full JSON in Redis record key; if >4KB store reference `{status, refID, hash, size}` in Redis and full record in DB via async goroutine; set response headers `X-Idempotency-Key` and `X-Idempotency-Replayed` in `internal/http/middleware/idempotency.go`
- [x] T025 [US1] Implement fail-open: if `cache.Driver` is nil or any Redis operation fails, log warning and pass through to next handler in `internal/http/middleware/idempotency.go`
- [x] T026 [US1] Wire middleware into `Server.RegisterRoutes()` — add `idempotencyService` field + setter + apply middleware after JWT in protected route groups in `internal/http/server.go`
- [x] T027 [US1] Wire config parsing, service creation, reaper start/stop in `cmd/api/main.go` — add to graceful shutdown order after `analyticsReaper`

**Checkpoint**: At this point, User Story 1 should be fully functional — duplicate requests with the same idempotency key return cached responses without re-executing business logic

---

## Phase 4: User Story 2 — Automatic Key Expiration & Cleanup (Priority: P2)

**Goal**: Expired idempotency records are automatically cleaned up via a background reaper, preventing unbounded storage growth.

**Independent Test**: Create an idempotency record, set TTL to expire (or use a short TTL for testing), then send a request with the same key. The system should treat it as a new request rather than returning a cached response.

### Tests for User Story 2

- [x] T028 [P] [US2] Unit test for `IdempotencyService.CleanupExpired()` — verify soft-delete of records past retention days in `tests/unit/service/idempotency_test.go`
- [x] T029 [P] [US2] Unit test for reaper `Start`/`Stop` lifecycle — verify goroutine starts and stops cleanly in `tests/unit/service/idempotency_test.go`
- [x] T030 [US2] Integration test — create record with short TTL, verify reaper cleans it up, verify new request with same key is treated as fresh in `tests/integration/idempotency_reaper_test.go`

### Implementation for User Story 2

- [x] T031 [US2] Implement `CleanupExpired(ctx, retentionDays)` in `IdempotencyRepository` — soft-delete records where `expires_at < NOW() - retentionDays` using `deleted_at` in `internal/repository/idempotency.go`
- [x] T032 [US2] Implement reaper goroutine in `IdempotencyService` — `Start(ctx)` launches goroutine with configurable ticker, `Stop()` cancels context, `run()` calls `CleanupExpired` and logs results in `internal/service/idempotency.go`
- [x] T033 [US2] Add `IDEMPOTENCY_REAPER_INTERVAL` and `IDEMPOTENCY_RETENTION_DAYS` environment variable parsing to config in `internal/config/idempotency.go`
- [x] T034 [US2] Wire reaper start/stop in `cmd/api/main.go` — start after analytics reaper, stop in graceful shutdown order

**Checkpoint**: At this point, expired idempotency records are automatically cleaned up by the background reaper

---

## Phase 5: User Story 3 — Admin Visibility & Monitoring (Priority: P3)

**Goal**: Administrators can monitor idempotency key usage — active keys, hit rates, expired counts — via admin API endpoints.

**Independent Test**: Make several idempotent requests, then query the admin endpoint to verify the counts and statuses match expectations.

### Tests for User Story 3

- [x] T035 [P] [US3] Unit test for `IdempotencyHandler` — list, get by ID, trigger cleanup in `tests/unit/handler/idempotency_test.go`
- [x] T036 [US3] Integration test — create multiple idempotency records, verify list endpoint returns correct pagination and data, verify cleanup endpoint triggers reaper in `tests/integration/idempotency_handler_test.go`

### Implementation for User Story 3

- [x] T037 [P] [US3] Create `IdempotencyHandler` struct with `List`, `GetByID`, `TriggerCleanup` methods in `internal/http/handler/idempotency.go`
- [x] T038 [US3] Register handler routes — GET `/api/v1/idempotency/keys` (list), GET `/api/v1/idempotency/keys/:id` (get), POST `/api/v1/idempotency/cleanup` (trigger) with JWT + permission middleware (`idempotency:view`, `idempotency:manage`) in `internal/http/handler/idempotency.go`
- [x] T039 [US3] Wire handler into `Server.RegisterRoutes()` in `internal/http/server.go`

**Checkpoint**: All user stories are now independently functional — duplicate protection, automatic cleanup, and admin visibility

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, final wiring validation, edge case testing

- [x] T040 [P] Add `Idempotency-Key` and `X-Idempotency-Replayed` header documentation to API docs in `docs/`
- [x] T041 [P] Create quickstart guide at `specs/019-idempotency-keys/quickstart.md`
- [x] T042 [P] Create interface contracts at `specs/019-idempotency-keys/contracts/idempotency.md`
- [x] T043 Edge case integration tests — 9 tests created (Redis fail-open, 4KB threshold, concurrent same-key, WebSocket skip, unauthenticated skip, GET skip, DELETE skip, invalid key, valid key) in `tests/integration/idempotency_edge_test.go`
- [x] T044 [P] Update `AGENTS.md` with idempotency feature documentation (middleware, config, endpoints, key patterns)
- [x] T045 Run `go build`, `go vet`, and `golangci-lint run` — fix any issues
- [ ] T046 Verify all integration tests pass with `go test -v -tags=integration ./tests/integration/... -timeout 5m`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 — core middleware, MUST complete first (MVP)
- **User Story 2 (Phase 4)**: Depends on Phase 2 — reaper and cleanup, can parallel with US1's service tasks
- **User Story 3 (Phase 5)**: Depends on Phase 2 + US1's service layer — admin visibility endpoints
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 2 — No dependencies on other stories (core middleware)
- **User Story 2 (P2)**: Can start after Phase 2 — Shares `IdempotencyService` with US1 but reaper is independent
- **User Story 3 (P3)**: Depends on US1 service layer for handler — requires `IdempotencyService` methods

### Within Each User Story

- Domain entities before repository
- Repository before service
- Service before middleware/handler
- Core implementation before integration tests
- Story complete before moving to next priority

### Parallel Opportunities

- T001–T009 (Phase 1 setup): All marked [P] can run in parallel — different files
- T014–T016 (US1 tests): All marked [P] can run in parallel — different test files
- T028–T029 (US2 tests): Both marked [P] can run in parallel
- T035 (US3 test): Can run in parallel with T037–T038
- T040–T042 (Phase 6 docs): All marked [P] can run in parallel

---

## Parallel Example: User Story 1

```bash
# Parallel: All cache.Driver implementations + config + domain + migration
Task T002: "Implement SetNX on redisCache"
Task T003: "Implement SetNX on memoryCache"
Task T004: "Implement SetNX on NoopCache"
Task T005: "Create IdempotencyConfig"
Task T007: "Create IdempotencyRecord entity"
Task T008: "Create migration"

# Parallel: All unit tests for US1
Task T014: "Unit test for Repository"
Task T015: "Unit test for Service"
Task T016: "Unit test for Middleware"

# Sequential: Middleware implementation (depends on tests + foundation)
Task T018 → T019 → T020 → T021 → T022 → T023 → T024 → T025 → T026 → T027
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (cache extension, config, domain, migration)
2. Complete Phase 2: Foundational (repository, service, permissions)
3. Complete Phase 3: User Story 1 (middleware — core idempotency)
4. **STOP and VALIDATE**: Test duplicate request protection independently
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test reaper independently → Deploy/Demo
4. Add User Story 3 → Test admin endpoints independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:
1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (middleware — longest, highest priority)
   - Developer B: User Story 2 (reaper — can start service immediately)
   - Developer C: Phase 6 tasks (docs, contracts — can start in parallel)
3. US3 starts when US1's service layer is complete (handler depends on service)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (if following TDD)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- The `cache.Driver.SetNX` extension (T001–T004) is a prerequisite that modifies an existing interface — all implementations must be added before any other cache.Driver usage
- The middleware position is **after JWT, before Permission+Audit** in route groups — this is critical for user scoping
- Fail-open on Redis errors is non-negotiable (FR-012) — must match rate limiter pattern
- The 4KB threshold for Redis caching must be configurable via `IDEMPOTENCY_MAX_CACHED_RESPONSE_SIZE` env var