# Tasks: Activity Feed / Timeline

**Input**: Design documents from `/specs/016-activity-feed/`
**Prerequisites**: plan.md (required), spec.md (required), data-model.md, contracts/api-v1.md, contracts/event-bus.md, quickstart.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create domain entities and database migration — the foundation everything depends on.

- [ ] T001 Create Activity, ActivityRead, ActivityFollow entities and DTOs in `internal/domain/activity.go` — Activity (id, actor_id, action_type, resource_type, resource_id, organization_id, metadata JSONB, archived_at, deleted_at, created_at, updated_at), ActivityRead (id, user_id, activity_id, read_at, created_at), ActivityFollow (id, user_id, resource_type, resource_id, created_at). Include ActionType/ResourceType constants, ToResponse() methods, IsValidActionType(), IsValidResourceType(). Follow pattern in `internal/domain/comment.go` and `internal/domain/notification.go`.
- [ ] T002 Create ActivityEventType constants and event-to-activity mapping in `internal/domain/activity_events.go` — Map from EventBus event types (user.created, invoice.paid, etc.) to Activity ActionType and ResourceType. Include fallback derivation (dot-split). Follow pattern in `internal/domain/webhook_events.go`.
- [ ] T003 Create database migration `migrations/000023_create_activities.up.sql` and `000023_create_activities.down.sql` — Three tables (activities, activity_reads, activity_follows) with all indexes from data-model.md: partial indexes (WHERE archived_at IS NULL), composite unique constraints, foreign keys with ON DELETE CASCADE, and update_updated_at trigger on activities. Follow pattern in `migrations/000022_create_comments.up.sql`.
- [ ] T004 Create ActivityConfig in `internal/config/activity.go` — RetentionDays (default 90), DefaultPageSize (default 20), MaxPageSize (default 100), ReaperInterval (default 60s). Follow existing config pattern (e.g., webhook config).

**Checkpoint**: Domain entities, event mappings, migration, and config are complete. Foundation ready for repository layer.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Repository and service layers that ALL user stories depend on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T005 Create ActivityRepository interface and GORM implementation in `internal/repository/activity.go` and `internal/repository/activity_repo.go` — Interface methods: Create, FindByID, FindByOrganization (paginated, filtered, excludes archived+deleted), FindByResource (paginated, filtered), ArchiveOlderThan (for reaper), SoftDelete. Follow repository pattern in `internal/repository/comment.go`. Queries MUST use batch is_read/is_following computation (no LEFT JOIN on activity_reads/activity_follows in main query).
- [ ] T006 [P] Create ActivityReadRepository interface and GORM implementation in `internal/repository/activity.go` and `internal/repository/activity_repo.go` — Interface methods: Upsert (create or update read_at), FindByUserAndActivityIDs (batch read status), DeleteByUserAndActivity, CountUnreadByUser, MarkAllReadByUser. Unique constraint on (user_id, activity_id).
- [ ] T007 [P] Create ActivityFollowRepository interface and GORM implementation in `internal/repository/activity.go` and `internal/repository/activity_repo.go` — Interface methods: Create, Delete, FindByUser, FindByUserAndResource, FindByUserAndResourceIDs (batch follow status), ExistsByUserAndResource. Unique constraint on (user_id, resource_type, resource_id).

**Checkpoint**: All repository interfaces and implementations complete. Service layer can now begin.

---

## Phase 3: User Story 1 — View Personal Activity Feed (Priority: P1) 🎯 MVP

**Goal**: Users can view a paginated, org-scoped activity feed with filtering by resource_type, action_type, and read/unread status. EventBus integration automatically creates activities.

**Independent Test**: Create activities via EventBus.Publish(), verify they appear in GET /api/v1/activities, verify org scoping, verify pagination.

### Implementation for User Story 1

- [ ] T008 Create ActivityService in `internal/service/activity.go` — Methods: ListByOrganization (paginated feed with batch is_read/is_following computation), FindByID, SubscribeToEventBus (buffered channel pattern from webhook), processEvents, handleEvent (event-to-activity mapping + metadata construction). Extract actor_id, org_id from WebhookEvent. Follow service pattern in `internal/service/comment.go`. Use `archived_at` for 90-day filter (NOT DeletedAt).
- [ ] T009 Create ActivityHandler in `internal/http/handler/activity.go` — GET /api/v1/activities (list with filters: resource_type, resource_id, action_type, unread, following, since, limit, offset). Permission: activity:view. Extract userID and orgID from middleware. Follow handler pattern in `internal/http/handler/comment.go`.
- [ ] T010 Create request DTOs in `internal/http/request/activity.go` — ListActivitiesRequest with validation (limit max 100, resource_type validation, action_type validation). Follow pattern in `internal/http/request/comment.go`.
- [ ] T011 Create response DTOs in `internal/http/response/activity.go` — ActivityResponse (with is_read, is_following computed fields, actor_name), ActivityListResponse (activities, total, unread_count, limit, offset). Follow pattern in `internal/http/response/comment.go`.
- [ ] T012 Register ActivityRoutes on Server in `internal/http/server.go` — Add RegisterActivityRoutes method. Route group under /api/v1/activities with JWT and permission middleware (activity:view). Follow pattern of existing route registrations.
- [ ] T013 Wire ActivityService DI and EventBus subscription in `cmd/api/main.go` — Create repos, service, handler. Subscribe to EventBus with `activityService.SubscribeToEventBus(eventBus)`. Add activity:view and activity:manage to DefaultPermissions and runSeed. Follow wiring pattern for webhook service.

**Checkpoint**: At this point, User Story 1 should be fully functional — activities appear in feed, org-scoped, filterable, paginated.

---

## Phase 4: User Story 2 — View Resource Activity History (Priority: P2)

**Goal**: Users can view activity history for a specific resource (e.g., an invoice, a news article) and follow/unfollow resources.

**Independent Test**: Create activities for a specific resource, verify GET /activities?resource_type=invoice&resource_id=X returns only those activities. Follow a resource, verify ?following=true filter works.

### Implementation for User Story 2

- [ ] T014 Add ListByResource method to ActivityService in `internal/service/activity.go` — Filter activities by resource_type and resource_id, paginated. Reuse batch is_read/is_following computation from ListByOrganization.
- [ ] T015 Create ActivityFollowService in `internal/service/activity_follow.go` — Methods: Follow (create ActivityFollow), Unfollow (delete ActivityFollow), ListFollows (paginated), IsFollowing (check status). Follow service pattern in `internal/service/comment.go`.
- [ ] T016 Add follow/unfollow endpoints to ActivityHandler in `internal/http/handler/activity.go` — POST /api/v1/activities/follow (create follow), DELETE /api/v1/activities/follow/:id (unfollow), GET /api/v1/activities/follows (list follows). Add ?following=true filter to list endpoint. Permission: activity:view. Audit log follow/unfollow mutations.
- [ ] T017 Add follow request/response DTOs in `internal/http/request/activity.go` and `internal/http/response/activity.go` — FollowResourceRequest (resource_type, resource_id with validation), FollowResponse, FollowListResponse.

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently — resource-scoped history and follow/unfollow are functional.

---

## Phase 5: User Story 3 — Mark Activities as Read and Manage Feed (Priority: P3)

**Goal**: Users can mark activities as read, count unread, mark all as read, and the 90-day reaper auto-archives old activities.

**Independent Test**: Create activities, mark some as read, verify unread count decreases. Mark all as read, verify count goes to 0. Verify reaper archives activities older than retention period.

### Implementation for User Story 3

- [ ] T018 Add read tracking methods to ActivityService in `internal/service/activity.go` — MarkAsRead (upsert ActivityRead), MarkAllAsRead (batch upsert for all visible unread activities), CountUnread (batch query via ActivityReadRepository). Audit log mark-read mutations.
- [ ] T019 Create ActivityReaper in `internal/service/activity_reaper.go` — Background goroutine with configurable interval (default 60s). Queries activities WHERE created_at < NOW() - retention_days AND archived_at IS NULL AND deleted_at IS NULL. Sets archived_at in batches of 1000. Uses context cancellation for graceful shutdown. Follow pattern of webhook stuck-delivery reaper.
- [ ] T020 Add read-tracking and admin endpoints to ActivityHandler in `internal/http/handler/activity.go` — GET /api/v1/activities/count-unread, PUT /api/v1/activities/read-all, PUT /api/v1/activities/:id/read, DELETE /api/v1/activities/:id (admin, permission: activity:manage). Audit log all mutations.
- [ ] T021 Add CountUnread request/response and read-all response DTOs in `internal/http/request/activity.go` and `internal/http/response/activity.go` — CountUnreadResponse, MarkAllReadResponse, MarkReadResponse, DeleteActivityRequest.
- [ ] T022 Start/stop ActivityReaper in `cmd/api/main.go` — Start reaper goroutine after EventBus subscription. Add to shutdown order: server → eventBus → activityReaper → workers → enforcer → db → redis. Wire ActivityReadRepository into ActivityService.

**Checkpoint**: All three user stories are now independently functional — read tracking, counts, and 90-day archival are complete.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Unit tests, integration tests, and documentation updates.

- [ ] T023 [P] Write unit tests for ActivityService in `tests/unit/activity_service_test.go` — Test ListByOrganization with filters, ListByResource, MarkAsRead, MarkAllAsRead, CountUnread, handleEvent (event-to-activity mapping), batch is_read/is_following computation. Use mock repos and mock enforcer. Follow pattern in `tests/unit/comment_service_test.go`.
- [ ] T024 [P] Write unit tests for ActivityFollowService in `tests/unit/activity_follow_service_test.go` — Test Follow, Unfollow, ListFollows, IsFollowing. Use mock repos.
- [ ] T025 [P] Write unit tests for ActivityReaper in `tests/unit/activity_reaper_test.go` — Test Start/Stop lifecycle, archival of old activities, batch processing, context cancellation.
- [ ] T026 Write integration tests for ActivityHandler in `tests/integration/activity_handler_test.go` — Full HTTP round-trip tests: create activity via EventBus → verify in feed, list with filters, mark read, mark all read, count unread, follow/unfollow, org scoping, pagination, 90-day archival (reaper test). Follow pattern in `tests/integration/notification_handler_test.go`.
- [ ] T027 Update AGENTS.md with Activity Feed feature documentation — Add Activity Feed section to WHERE TO LOOK, CODE MAP, COMMANDS (if applicable), NOTES, and TROUBLESHOOTING tables. Follow existing pattern.
- [ ] T028 Update docs/FEATURE_STATUS.md — Mark Activity Feed as complete with branch name and date.
- [ ] T029 Run `go build ./...` and `go vet ./...` — Verify no compilation errors or vet warnings.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 — MVP deliverable
- **User Story 2 (Phase 4)**: Depends on Phase 2 — Can start in parallel with Phase 3 after Phase 2
- **User Story 3 (Phase 5)**: Depends on Phase 2 — Can start in parallel with Phase 3/4 after Phase 2
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 2 — No dependencies on other stories
- **User Story 2 (P2)**: Can start after Phase 2 — Adds resource-scoped filtering and follow/unfollow
- **User Story 3 (P3)**: Can start after Phase 2 — Adds read tracking, counts, and reaper

### Within Each User Story

- Domain entities before repositories
- Repositories before services
- Services before handlers
- Handlers before route registration
- DI wiring last

### Parallel Opportunities

- T006 (ActivityReadRepository) and T007 (ActivityFollowRepository) can run in parallel
- T010 (request DTOs) and T011 (response DTOs) can run in parallel
- T023, T024, T025 (unit tests) can all run in parallel
- User Stories 2 and 3 can start in parallel after Phase 2 completes

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Launch repos in parallel (different files):
Task: "Create ActivityReadRepository in internal/repository/activity.go"
Task: "Create ActivityFollowRepository in internal/repository/activity.go"
# Note: All repos are in the same files, so they should be done sequentially
# by the same implementer to avoid merge conflicts.
```

## Parallel Example: Phase 6 (Polish)

```bash
# Launch all unit tests in parallel:
Task: "Write unit tests for ActivityService in tests/unit/activity_service_test.go"
Task: "Write unit tests for ActivityFollowService in tests/unit/activity_follow_service_test.go"
Task: "Write unit tests for ActivityReaper in tests/unit/activity_reaper_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T007)
3. Complete Phase 3: User Story 1 (T008-T013)
4. **STOP and VALIDATE**: Test feed listing, EventBus integration, org scoping
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test follow/unfollow and resource filtering → Deploy/Demo
4. Add User Story 3 → Test read tracking and reaper → Deploy/Demo
5. Polish → Unit tests, integration tests, docs

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Activity entity has BOTH `archived_at` (reaper) and `deleted_at` (admin DELETE) — do NOT use DeletedAt for archival
- All feed queries MUST use batch is_read/is_following computation (no LEFT JOIN)
- EventBus subscription MUST use buffered channel pattern (not synchronous handler)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently