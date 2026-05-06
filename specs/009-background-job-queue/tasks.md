# Tasks: Background Job Queue

**Input**: Design documents from `/specs/009-background-job-queue/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Go project paths (per plan.md):
- Domain entities: `internal/domain/`
- Config: `internal/config/`
- Repository: `internal/repository/`
- Services: `internal/service/`
- HTTP handlers: `internal/http/handler/`
- HTTP middleware: `internal/http/middleware/`
- Tests: `tests/unit/`, `tests/integration/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create `internal/domain/job.go` with Job entity, JobStatus enum, and constants per data-model.md
- [ ] T002 Create `internal/config/job.go` with JobConfig struct (worker count, retry backoff, timeout, TTL) with viper env binding

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 Create `internal/repository/job.go` with JobRepository interface and Redis implementation
  - Methods: Submit, GetByID, Update, ListByUser, Enqueue, Dequeue, Requeue, SetProcessing, SetCompleted, SetFailed, GetStuckJobs
  - Redis sorted set `jobs:queue` for queue
  - Redis hash `jobs:data:{id}` for job data
  - Redis secondary indexes `jobs:user:{user_id}` for list queries
- [ ] T004 Create `internal/service/job_handler.go` with JobHandler interface and built-in handlers
  - `JobHandler` interface: `JobType()` string, `Handle(ctx, payload)` (result, error)
  - Built-in: `EmailJobHandler`, `ExportJobHandler`
  - Handler registry map[string]JobHandler with mutex
- [ ] T005 [P] Create `internal/service/job_callback.go` for webhook callback delivery
  - `DeliverCallback(ctx, url, payload)` with 5s timeout, best-effort HTTP POST
  - Log failures but don't fail the job

**Checkpoint**: Foundation ready - repository, handler registry, and callback delivery are functional

---

## Phase 3: User Story 1 - Submit Background Jobs (Priority: P1) 🎯 MVP

**Goal**: Jobs can be submitted via HTTP API and queued in Redis

**Independent Test**: POST /api/v1/jobs returns 202 with job_id, job appears in Redis queue within 100ms

### Implementation for User Story 1

- [ ] T006 [US1] Create `internal/http/handler/job.go` with `SubmitJob` handler
  - POST /api/v1/jobs - validate type + payload, create job, enqueue, return 202
  - Request struct: `{type, payload, callback_url, max_retries}`
  - Response: `{job_id, status, created_at}`
- [ ] T007 [US1] Create `internal/http/request/job.go` with request DTOs
  - `JobSubmitRequest` with validation tags
- [ ] T008 [US1] Create `internal/http/response/job.go` with response DTOs
  - `JobResponse`, `JobListResponse`
- [ ] T009 [US1] Create `internal/http/middleware/job.go` with job ownership middleware
  - Verify requesting user owns the job or is admin
- [ ] T010 [US1] Wire job routes in `internal/http/server.go`
  - POST /api/v1/jobs → JobHandler.SubmitJob
  - Register routes in RegisterRoutes()

**Checkpoint**: Job submission works end-to-end

---

## Phase 4: User Story 2 - Monitor Job Status (Priority: P2)

**Goal**: Job status and results can be retrieved via HTTP API

**Independent Test**: GET /api/v1/jobs/:id returns correct status with timestamps

### Implementation for User Story 2

- [ ] T011 [US2] Add `GetJob` handler to `internal/http/handler/job.go`
  - GET /api/v1/jobs/:id - get job by ID, check ownership, return status
  - Include result payload when completed
  - Include next_retry_at when failed with retries remaining
- [ ] T012 [US2] Add `ListJobs` handler to `internal/http/handler/job.go`
  - GET /api/v1/jobs - list jobs for user with filters (status, type) and pagination (limit, offset)
  - Use Redis secondary index for filtering
- [ ] T013 [US2] Wire status and list routes in `internal/http/server.go`
  - GET /api/v1/jobs/:id → JobHandler.GetJob
  - GET /api/v1/jobs → JobHandler.ListJobs

**Checkpoint**: Job status monitoring works end-to-end

---

## Phase 5: User Story 3 - Retry Failed Jobs (Priority: P3)

**Goal**: Failed jobs automatically retry with exponential backoff; dead jobs can be resubmitted

**Independent Test**: Failed job with retries transitions to failed+next_retry_at, then reprocesses

### Implementation for User Story 3

- [ ] T014 [US3] Implement retry logic in `internal/service/job.go`
  - On handler error: check attempt_count vs max_retries
  - If retries remain: calculate backoff (1m/5m/30m cap), set next_retry_at, requeue in Redis
  - If exhausted: set status to dead
- [ ] T015 [US3] Add `ResubmitJob` handler to `internal/http/handler/job.go`
  - POST /api/v1/jobs/:id/resubmit - only for dead status, reset attempt_count to 1
- [ ] T016 [US3] Wire resubmit route in `internal/http/server.go`
  - POST /api/v1/jobs/:id/resubmit → JobHandler.ResubmitJob

**Checkpoint**: Retry mechanism works end-to-end with backoff

---

## Phase 6: User Story 4 - Webhook Notifications (Priority: P4)

**Goal**: Jobs can register callback URLs, callbacks delivered on completion/failure

**Independent Test**: Job with callback_url receives HTTP POST to callback URL on completion

### Implementation for User Story 4

- [ ] T017 [US4] Integrate callback delivery into job completion flow
  - After job completes or fails with exhausted retries: call DeliverCallback
  - Callback payload: `{job_id, type, status, result, attempt_count, completed_at}`
- [ ] T018 [US4] Add callback headers in `internal/service/job_callback.go`
  - Headers: Content-Type, X-Job-ID, X-Job-Status

**Checkpoint**: Webhook notifications delivered on job completion

---

## Phase 7: Worker Pool & Stuck Job Recovery

**Purpose**: Background job processing with automatic stuck job recovery

- [ ] T019 Create `internal/service/job_worker.go` with worker pool
  - N poller goroutines using BZPOPMIN on `jobs:queue`
  - N worker goroutines consuming from buffered channel
  - Graceful shutdown with drain
- [ ] T020 Create `internal/service/job_reaper.go` with stuck job recovery
  - Run every 60s
  - Find jobs in "processing" > 5 min
  - Reset to pending with incremented attempt_count

**Checkpoint**: Jobs process automatically in background; stuck jobs recover

---

## Phase 8: Unit Tests

**Purpose**: Verify core job queue functionality

- [ ] T021 [P] Create `tests/unit/job_service_test.go` for JobService
  - Test submit: job created with correct status
  - Test status: returns correct job state
  - Test retry: retries when failed with attempts remaining
  - Test dead: transitions to dead when retries exhausted
- [ ] T022 [P] Create `tests/unit/job_worker_test.go` for worker pool
  - Test job processing: status transitions to completed
  - Test job failure: retry scheduled correctly
  - Test graceful shutdown: workers drain in-flight jobs

---

## Phase 9: Integration Tests

**Purpose**: End-to-end verification with real Redis

- [ ] T023 Create `tests/integration/job_test.go`
  - Test full job lifecycle: submit → process → complete
  - Test retry lifecycle: fail → retry → complete
  - Test dead letter: max retries → dead → resubmit
  - Test callback delivery (mock server)

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T024 [P] Add integration with existing email service (EmailJobHandler)
- [ ] T025 [P] Add logging for job lifecycle events in all service files
- [ ] T026 Verify graceful shutdown integration with cmd/api/main.go
- [ ] T027 Run quickstart.md validation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 - BLOCKS all user stories
- **Phases 3-6 (User Stories)**: All depend on Phase 2
  - US1 (submit) → US2 (status) → US3 (retry) → US4 (callbacks) sequential but each independently testable
- **Phase 7 (Worker)**: Depends on Phase 2 - workers need queue infrastructure
- **Phases 8-9 (Tests)**: Depend on implementation phases
- **Phase 10 (Polish)**: Depends on all user stories

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 - No dependencies on other stories
- **US2 (P2)**: Can start after Phase 2 and US1 - May use submit but independently testable
- **US3 (P3)**: Can start after Phase 2 and US1 - May use status but independently testable
- **US4 (P4)**: Can start after Phase 2 - Uses callback delivery (foundational)

### Within Each User Story

- Domain model before repository
- Repository before service
- Service before handler
- Handler before wiring

### Parallel Opportunities

- Phase 1 tasks (T001, T002): Can run in parallel
- Phase 2 tasks (T003, T004, T005): T004 and T005 can run in parallel (no shared files)
- Within US1: T006, T007, T008, T009 can start together but T010 (wiring) depends on all
- Within US2: T011, T012 can start together but T013 (wiring) depends on both
- Unit tests (T021, T022): Can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (submit jobs)
4. **STOP and VALIDATE**: Test job submission works
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Phase 1 + Phase 2 → Foundation ready
2. Add Phase 3 (US1) → Test job submission → Deploy (MVP!)
3. Add Phase 4 (US2) → Test status API → Deploy
4. Add Phase 5 (US3) → Test retry → Deploy
5. Add Phase 6 (US4) → Test callbacks → Deploy
6. Add Phase 7 (Worker) → Background processing works
7. Add tests → Verify everything
8. Polish → Production ready

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Go project: Use Echo context for request IDs, slog for structured logging, viper for config
