# Tasks: Webhook System

**Input**: Design documents from `/specs/006-webhook-system/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/api-v1.md, contracts/env-vars.md, quickstart.md
**Branch**: `006-webhook-system`

**Tests**: Integration tests using testcontainers per existing project pattern. Unit tests with testify/mock for service/repo layer.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- Domain entities: `internal/domain/`
- Repositories: `internal/repository/`
- Services: `internal/service/`
- HTTP handlers: `internal/http/handler/`
- HTTP middleware: `internal/http/middleware/`
- Request DTOs: `internal/http/request/`
- Config: `internal/config/`
- Migrations: `migrations/`
- Unit tests: `tests/unit/`
- Integration tests: `tests/integration/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project structure, config, and database migration for webhooks.

- [ ] T001 [P] Create `internal/config/webhook.go` — WebhookConfig struct with Viper bindings for all 7 env vars from contracts/env-vars.md (WEBHOOK_WORKER_CONCURRENCY, WEBHOOK_RETRY_MAX, WEBHOOK_RATE_LIMIT, WEBHOOK_ALLOW_HTTP, WEBHOOK_DELIVERY_TIMEOUT, WEBHOOK_DELIVERY_RETENTION_DAYS, WEBHOOK_MAX_PAYLOAD_SIZE)
- [ ] T002 [P] Create `internal/domain/webhook.go` — Webhook + WebhookDelivery entities, WebhookDeliveryStatus constants, ValidWebhookEvents map, ToResponse()/ToCreateResponse() methods, IsSubscribedTo()/IsActive()/IsGlobal()/CanRetry()/CanReplay()/IsTerminal()/Replay() methods per data-model.md
- [ ] T003 [P] Create `internal/domain/webhook_events.go` — EventBus struct with buffered channel and subscriber pattern per research.md R3
- [ ] T004 Create migration `migrations/000014_webhooks.up.sql` — webhooks and webhook_deliveries tables with all indexes, constraints, and CHECK per data-model.md SQL schema
- [ ] T005 [P] Create migration `migrations/000014_webhooks.down.sql` — DROP TABLE webhook_deliveries, DROP TABLE webhooks (order matters: deliveries first due to FK)
- [ ] T006 Add `webhooks:view` and `webhooks:manage` permissions to seed data/permission sync command in `cmd/api/main.go` and run `permission:sync`
- [ ] T007 Wire WebhookConfig into top-level Config struct in `internal/config/config.go` and add Viper defaults

**Checkpoint**: Config loads, migration runs, permissions sync — `make migrate && go run ./cmd/api permission:sync` succeeds

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Repository and event bus layer that ALL user stories depend on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T008 Create `internal/repository/webhook.go` — WebhookRepository interface and GORM implementation with: Create, FindByID, FindByOrgID (paginated), Update, SoftDelete, FindActiveByEvent (for dispatch), plus WebhookDeliveryRepository with: Create, FindByID, FindByWebhookID (paginated filtered), UpdateStatus, FindStuckProcessing, DeleteOlderThan for retention
- [ ] T009 Create `internal/http/request/webhook.go` — Request DTOs: CreateWebhookRequest (name, url, events, active, rate_limit), UpdateWebhookRequest (all optional), ListWebhooksQuery (limit, offset, active, event), ListDeliveriesQuery (limit, offset, status, event) with Echo validator tags
- [ ] T010 Create `internal/http/middleware/webhook_signature.go` — HMAC-SHA256 signature generation utility: SignWebhookPayload(secret string, timestamp int64, body []byte) string producing `t=...,v1=...` format, and VerifyWebhookSignature(secret string, sigHeader string, body []byte) bool per research.md R1

**Checkpoint**: Repos compile, DTOs validate, signature utility works — `go build ./...` succeeds

---

## Phase 3: User Story 1 — Webhook CRUD (Priority: P1) 🎯 MVP

**Goal**: Users can create, list, get, update, and soft-delete webhooks via REST API.

**Independent Test**: CRUD operations on `/api/v1/webhooks` return correct status codes and data; `secret` only appears on creation response.

### Implementation for User Story 1

- [ ] T011 [P] [US1] Create `internal/service/webhook.go` — WebhookService with Create, GetByID, ListByOrg, Update, SoftDelete methods. Service does NOT call enforcer (handler-level ownership check per R10). Includes: secret generation (whsec_ prefix + 32 bytes hex), URL validation (HTTPS enforcement unless WEBHOOK_ALLOW_HTTP, SSRF protection blocking RFC1918/localhost/link-local), events validation against ValidWebhookEvents map
- [ ] T012 [US1] Create `internal/http/handler/webhook.go` — WebhookHandler with: Create (201 + WebhookCreateResponse with secret), GetByID (200 + WebhookResponse without secret), List (200 + paginated), Update (200 + WebhookResponse), SoftDelete (204). All endpoints require JWT auth. List/Get require `webhooks:view` permission. Create/Update/Delete require `webhooks:manage` permission. Handler extracts orgID via `middleware.GetOrganizationID(c)` and passes to service
- [ ] T013 [US1] Register webhook routes in `internal/http/server.go` RegisterRoutes — Group under `/api/v1/webhooks` with JWT and permission middleware. Add route-specific permission checks: POST/PUT/DELETE require `webhooks:manage`, GET requires `webhooks:view`
- [ ] T014 [US1] Create `tests/integration/webhook_handler_test.go` — Integration tests for all 5 CRUD endpoints: create webhook (verify secret in response), create duplicate URL (verify 409), get webhook (verify secret NOT in response), list webhooks (paginated), update webhook, delete webhook (soft delete), permission enforcement (verify 403 without proper permission)

**Checkpoint**: Webhook CRUD fully functional — can create, list, get, update, delete webhooks via API with correct permissions

---

## Phase 4: User Story 2 — Delivery Tracking & Replay (Priority: P2)

**Goal**: Users can view delivery history and replay failed/rate-limited/delivered deliveries.

**Independent Test**: Delivery records appear for webhooks; replay resets status to queued with attempt_number=1.

### Implementation for User Story 2

- [ ] T015 [P] [US2] Add delivery listing and replay methods to `internal/service/webhook.go` — ListDeliveries(webhookID, orgID, query), GetDelivery(webhookID, deliveryID, orgID), ReplayDelivery(webhookID, deliveryID, orgID). Replay enforces: CanReplay() check (409 if queued/processing), resets delivery via Replay() method, saves to DB
- [ ] T016 [US2] Add delivery endpoints to `internal/http/handler/webhook.go` — GET /webhooks/:id/deliveries (paginated with status/event filters), GET /webhooks/:id/deliveries/:delivery_id (full payload+response), POST /webhooks/:id/deliveries/:delivery_id/replay (200 + reset delivery). All require JWT auth. View requires `webhooks:view`; replay requires `webhooks:manage`. Handler verifies org ownership before proceeding
- [ ] T017 [US2] Register delivery routes in `internal/http/server.go` — Add under existing webhook route group
- [ ] T018 [US2] Create `tests/integration/webhook_delivery_test.go` — Integration tests: list deliveries, get delivery detail (verify payload/response_body present), replay delivery (verify status=queued, attempt_number=1, next_retry_at=null), replay queued delivery (verify 409), list with status filter

**Checkpoint**: Delivery tracking and replay functional — can view delivery history and replay failed deliveries

---

## Phase 5: User Story 3 — Async Delivery Worker (Priority: P3)

**Goal**: Webhooks receive HTTP callbacks with HMAC signatures, retry on failure with exponential backoff, per-webhook rate limiting, and stuck-delivery recovery.

**Independent Test**: Active webhooks receive signed POST requests; failed deliveries retry with backoff; rate-limited deliveries are marked; stuck deliveries are recovered.

### Implementation for User Story 3

- [ ] T019 [US3] Create `internal/service/webhook_worker.go` — WebhookWorker struct mirroring EmailWorker pattern: goroutine pool (WebhookConfig.WorkerConcurrency), Start/Stop with 30s graceful shutdown, atomic metrics (delivered, failed, rate_limited counts), delivery HTTP client (10s timeout, 100 max idle conns), HMAC-SHA256 signing, SSDRF validation, payload truncation (1MB max, response body 1KB max). Process loop: dequeue from Redis ZSET, set processing_started_at, execute delivery, update status, schedule retry with exponential backoff (+1min, +5min, +30min)
- [ ] T020 [US3] Create `internal/service/webhook_queue_redis.go` — Redis-based delivery queue mirroring EmailQueue pattern: Enqueue (ZADD with score=timestamp), Dequeue (ZPOPMIN), EnqueueRetry (ZADD with future score for backoff), GetNextBatch (paginated scan of due deliveries). Key: `webhook:deliveries` ZSET
- [ ] T021 [US3] Create `internal/service/webhook_rate_limiter.go` — Per-webhook rate limiter using Redis sliding window: Allow(webhookID) returns (allowed bool, remaining int, retryAfter time.Duration). Key: `webhook:ratelimit:{id}`, window: 60s, max: WebhookConfig.RateLimit. When exceeded, create delivery with status=rate_limited and schedule backoff
- [ ] T022 [US3] Implement event dispatch in `internal/service/webhook.go` — DispatchEvent(ctx, event string, payload interface{}) method: (1) find active webhooks subscribed to event via repo, (2) check org scope, (3) check rate limit per webhook, (4) create WebhookDelivery record, (5) enqueue to Redis ZSET. Register EventBus subscription in wire-up
- [ ] T023 [US3] Add stuck-delivery reaper goroutine to `internal/service/webhook_worker.go` — Periodic (every 60s) query for `status='processing' AND processing_started_at < now() - timeout*2`. Mark as failed with `last_error='processing timeout: worker recovery'`, schedule retry if CanRetry(), re-enqueue
- [ ] T024 [US3] Add delivery retention cleanup to `internal/service/webhook_worker.go` — Periodic task using WEBHOOK_DELIVERY_RETENTION_DAYS config to delete old delivery records from webhook_deliveries
- [ ] T025 [US3] Wire worker lifecycle in `cmd/api/main.go` — Initialize WebhookWorker, call Start() after server init, add to graceful shutdown order (server → worker → enforcer → db → redis)
- [ ] T026 [US3] Create `tests/unit/webhook_worker_test.go` — Unit tests: HMAC signature generation/verification, URL validation (HTTPS, SSRF protection), payload truncation, delivery status transitions, CanRetry/CanReplay logic, Replay() field reset, rate limiter Allow/Deny logic, exponential backoff calculation
- [ ] T027 [US3] Create `tests/integration/webhook_worker_test.go` — Integration tests: end-to-end delivery (create webhook → emit event → verify HTTP POST received by test server), retry on failure (test server returns 500 → verify backoff and retry), rate limiting (exceed rate_limit → verify rate_limited status), stuck recovery (manually set processing_started_at in past → verify reaper marks as failed)

**Checkpoint**: Async delivery fully functional — webhooks receive signed POSTs with automatic retry and rate limiting

---

## Phase 6: User Story 4 — Event Integration (Priority: P4)

**Goal**: Domain events (user.created, invoice.paid, etc.) automatically trigger webhook dispatch.

**Independent Test**: Creating a user or invoice emits an event, and subscribed webhooks receive deliveries.

### Implementation for User Story 4

- [ ] T028 [US4] Add event emission hooks in existing services — Inject EventBus into UserService, InvoiceService, NewsService. Emit events: `user.created`, `user.deleted`, `invoice.created`, `invoice.paid`, `news.published` after successful operations. Use service-level logger for context
- [ ] T029 [US4] Add event emission integration test in `tests/integration/webhook_event_test.go` — Create webhook subscribing to `user.created`, create a user via API, verify webhook delivery record appears with correct event, payload contains user data
- [ ] T030 [US4] Create `tests/integration/webhook_e2e_test.go` — Full end-to-end: create webhook → trigger event → verify signed delivery to test server → verify delivery record with response_code — covers the complete webhook lifecycle

**Checkpoint**: Events flow end-to-end — domain operations trigger webhook deliveries

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, configuration validation, and quickstart verification.

- [ ] T031 [P] Update `AGENTS.md` with webhook system section: WHERE TO LOOK table entry, Commands section, Conventions entry, Organization Feature section for webhook permissions
- [ ] T032 [P] Update `.env.example` with all 7 webhook environment variables from contracts/env-vars.md
- [ ] T033 Run `make lint` and fix all issues in webhook files
- [ ] T034 Run quickstart.md validation — follow each step in quickstart.md against a running server and verify all curl commands succeed
- [ ] T035 [P] Add WebhookService and WebhookWorker audit logging — all CRUD operations emit audit logs (before/after JSONB) using existing audit middleware pattern

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 completion — BLOCKS all user stories
- **Phase 3 (US1 - CRUD)**: Depends on Phase 2 — MVP milestone
- **Phase 4 (US2 - Delivery Tracking)**: Depends on Phase 3 (needs webhook records)
- **Phase 5 (US3 - Worker)**: Depends on Phase 2 (needs repo + queue). Can develop in parallel with US1/US2 but needs both for integration tests
- **Phase 6 (US4 - Events)**: Depends on Phase 5 (needs worker for delivery). Also needs existing services for event emission
- **Phase 7 (Polish)**: Depends on all user stories

### User Story Dependencies

- **US1 (CRUD)**: → Phase 2 complete → Independently testable
- **US2 (Delivery Tracking)**: → US1 complete (needs webhook records) → Independently testable
- **US3 (Worker)**: → Phase 2 complete → Needs US1 for integration tests → Independently testable for unit
- **US4 (Events)**: → US3 complete (needs worker) → Depends on existing services

### Within Each User Story

- Models/DTOs before services
- Services before handlers
- Handlers before routes
- Core implementation before tests
- Story complete before moving to next priority

### Parallel Opportunities

- T001, T002, T003 (config, domain, events) — all parallel
- T004, T005 (up/down migrations) — parallel
- T011, T009 (service + DTOs can start together if repo interface defined)
- T026, T027 (unit + integration worker tests) — parallel
- T028 event emission hooks across different services — parallel
- T031, T032, T035 (docs, env, audit) — all parallel

---

## Parallel Example: Phase 1

```bash
# Launch all setup tasks together:
Task: "Create internal/config/webhook.go"
Task: "Create internal/domain/webhook.go"
Task: "Create internal/domain/webhook_events.go"
Task: "Create migrations/000014_webhooks.up.sql"
Task: "Create migrations/000014_webhooks.down.sql"

# After T006-T007 complete, verify:
Task: "Run make migrate && go run ./cmd/api permission:sync"
```

## Parallel Example: Phase 5 (Worker)

```bash
# Launch worker components together:
Task: "Create internal/service/webhook_worker.go"
Task: "Create internal/service/webhook_queue_redis.go"
Task: "Create internal/service/webhook_rate_limiter.go"

# After worker components complete:
Task: "Add event dispatch to internal/service/webhook.go"
Task: "Create tests/unit/webhook_worker_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (CRUD)
4. **STOP and VALIDATE**: Create/list/get/update/delete webhooks via API
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 (CRUD) → Test independently → Demo webhook management (MVP!)
3. Add US2 (Delivery Tracking) → Test independently → View delivery history
4. Add US3 (Worker) → Test independently → Webhooks fire on events with retry
5. Add US4 (Events) → Test independently → Domain events trigger webhooks
6. Polish → Documentation and audit logging complete

### Sequential (Team Size: 1)

Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6 → Phase 7

Each phase validated by tests before proceeding.

---

## Notes

- [P] tasks operate on different files with no shared dependencies
- Each user story is independently testable at its checkpoint
- All HTTP endpoints follow existing project conventions (Echo handler, envelope response, JWT auth, permission middleware)
- Service layer does NOT enforce permissions — handler layer does ownership checks
- webhook_deliveries has NO soft delete (immutable audit trail)
- Migration number is 000014 (existing migrations end at 000013)
- The `whsec_` prefix on secrets must be stripped before HMAC computation
- EventBus uses Go channels (in-process, not Redis pub/sub)
- WebhookWorker mirrors EmailWorker pattern (goroutine pool, atomic metrics, Start/Stop)