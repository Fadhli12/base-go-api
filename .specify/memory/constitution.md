<!--
SYNC IMPACT REPORT
==================
Version change: 1.0 → 2.0
Modified principles:
  - I. Production-Ready Foundation (expanded: added context propagation, graceful shutdown ordering, worker pools)
  - II. RBAC is Mandatory, Not Hardcoded (expanded: added organization/domain scoping)
  - IV. Stateless JWT + Revocation (expanded: added org context in token claims)
  - V. PostgreSQL + Versioned Migrations (unchanged)
  - VI. Multi-Instance Permission Consistency (expanded: added cache warm on permission:sync)
  - VII. Audit Logging Non-Negotiable (expanded: added webhook event audit trail)
Added sections:
  - VIII. Organization-Scoped Multi-Tenancy (NEW)
  - IX. Event-Driven Architecture (NEW)
  - X. Background Job Processing (NEW)
Removed sections: None
Templates requiring updates:
  - .specify/templates/constitution-template.md ⚠ pending (still has placeholder tokens)
  - .specify/templates/plan-template.md ✅ aligned (constitution check section is dynamic)
  - .specify/templates/spec-template.md ✅ aligned (no constitution-specific content)
  - .specify/templates/tasks-template.md ✅ aligned (no constitution-specific content)
Follow-up TODOs: None
-->

# Go API Base — Project Constitution

**Version**: 2.0 | **Ratified**: 2026-04-22 | **Last Amended**: 2026-05-08 | **Status**: Active

---

## Core Principles

### I. Production-Ready Foundation

Every deliverable MUST be suitable for production deployment without
modification. Code MUST handle edge cases, graceful degradation, and
operational concerns (logging, metrics, health checks).

- **Test Coverage**: Minimum 80% unit + integration test coverage on business logic
- **Error Handling**: All external calls wrapped; standardized error envelopes
- **Observability**: Structured logging (`slog`) on all critical paths;
  correlation IDs and context fields (`request_id`, `user_id`, `org_id`,
  `trace_id`) propagated via `internal/logger` and HTTP middleware
- **Graceful Shutdown**: Signal handling (SIGTERM) with 30-second timeout
  context cancellation. Shutdown order: server → eventBus → workers →
  enforcer → db → redis. Existing requests complete; new requests rejected.
- **Logger Interface**: Use `internal/logger.Logger` abstraction (not raw
  `slog`) for context propagation, field chaining, and testability. Mock
  logger available via `logger.NewMockLogger()`

### II. RBAC is Mandatory, Not Hardcoded

Permission system MUST be **customizable at runtime** without code changes
or redeployment. Use Casbin + GORM to store permissions, roles, and user
grants as database records.

- **No Permission Enum**: Permissions defined as data (resource:action pairs),
  not hardcoded constants
- **Runtime Grant/Revoke**: Admin endpoints to create roles, attach
  permissions, grant users without deploys
- **System Roles Protected**: Core roles (super_admin, admin) marked
  `is_system=true`; deletion prevented
- **Permission Naming Manifest**: `permission:sync` CLI command upserts known
  permissions from manifest on deploy
- **Organization Scoping**: Casbin domain model (`sub, dom, obj, act`)
  enables organization-scoped permissions alongside global permissions.
  `enforcer.Enforce(userID, orgID, resource, action)` checks permissions
  within org context; `uuid.Nil` represents global scope

### III. Soft Deletes for Audit & Compliance

All user-generated data (users, roles, permissions, organizations,
documents, webhooks) uses soft deletes (`deleted_at` timestamp). No hard
deletion of audit-relevant records.

- **Audit Trail Immutable**: Once logged, audit records cannot be deleted or
  modified; DB trigger raises error on UPDATE/DELETE attempts
- **Undo Capable**: Soft deletes enable row restoration if needed
- **Compliance**: Retain historical data for legal/regulatory requirements
- **Query Scoping**: All queries automatically exclude soft-deleted rows via
  GORM scopes

### IV. Stateless JWT + Revocation

Authentication uses stateless JWT tokens for scalability. Access tokens
expire in exactly **15 minutes** (900 seconds) with refresh tokens in
PostgreSQL for revocation capability.

- **Access Token**: 15 minutes (900 seconds), contains user_id, email,
  and optionally org_id
- **Refresh Token**: 30 days (2592000 seconds), stored in DB, rotated on
  each use
- **Logout**: Mark refresh token as revoked; client discards access token
- **Token Blacklist Optional**: Use Redis blacklist if early revocation
  needed (logout seconds)

### V. PostgreSQL + Versioned Migrations (Not AutoMigrate)

Database schema managed via `golang-migrate/migrate` with versioned SQL
files committed to repo. Never use GORM AutoMigrate in production.

- **Migration as Code**: All schema changes in
  `migrations/*.up.sql` / `*.down.sql`
- **Version Control**: Migrations tracked in git; reproducible across
  environments
- **CI/CD Friendly**: Migrations run as first step in deployment; no schema
  drift
- **No AutoMigrate**: Explicitly disable GORM AutoMigrate; it silently
  drifts schemas

### VI. Multi-Instance Permission Consistency

When running multiple API instances, permission cache invalidation MUST be
**sub-second** and **consistent**. Use Redis pub/sub to notify all instances
of permission changes.

- **Redis Pub/Sub**: Permission change events published to all subscribers
- **Cache TTL**: 30–60 second Redis cache TTL with immediate invalidation on
  change
- **No Eventual Consistency**: Invalidation MUST happen before response sent
  to user
- **Offline Graceful**: If pub/sub fails, fall back to TTL; no hard failure
- **Cache Warm**: `permission:sync` CLI command MUST warm the permission
  cache after upserting the manifest, ensuring freshly started instances
  have correct policy data without waiting for first request

### VII. Audit Logging Non-Negotiable

All user actions (CRUD, permission changes, admin operations, webhook
deliveries) logged with actor, action, resource, before/after snapshot,
timestamp, IP, user agent.

- **Before/After JSONB**: Capture state delta for all sensitive operations
- **Actor ID Always**: Who performed the action, even for system operations
- **Resource & Resource ID**: What was changed and its identifier
- **Compliance Ready**: Audit logs immutable; suitable for security reviews
- **Performance**: Async audit logging to avoid blocking user request
- **Webhook Event Trail**: Webhook creation, update, deletion, and delivery
  status changes MUST be audit-logged as distinct events

### VIII. Organization-Scoped Multi-Tenancy

Resources MUST support organization scoping for multi-tenant use cases.
Organization context is **optional but explicitly modeled** — global
resources (org_id = NULL) coexist with org-scoped resources.

- **X-Organization-ID Header**: Middleware extracts org context from
  `X-Organization-ID` request header; absent header means global scope
- **Domain Model**: Casbin domain model uses organization ID as the domain
  parameter: `enforcer.Enforce(userID, orgID, resource, action)`
- **Repository Scoping**: Repositories MUST support optional org_id filter;
  when org_id is `uuid.Nil`, queries return global (non-org-scoped) results
- **Organization Roles**: owner (full access), admin (manage + invite),
  member (view only)
- **Backward Compatibility**: Organization context is optional; existing
  resources remain accessible without organization scoping
- **Global Visibility**: Global webhooks and resources MUST be visible
  across all organizations; org-scoped resources only visible within
  their org

### IX. Event-Driven Architecture

System events (user created, invoice paid, news published, etc.) propagate
via an in-process EventBus using Go channels. Services publish events;
subscribers react asynchronously.

- **EventBus Interface**: `internal/domain/webhook_events.go` —
  `Subscribe(topic)`, `Publish(event)`, `Start(ctx)`, `Stop()`
- **Setter Injection**: Services receive EventBus via `SetEventBus()` setter
  to avoid constructor signature changes across features
- **Event Structure**: Events carry `Type`, `Payload`, `OrgID` (optional),
  and `Timestamp`
- **Graceful Shutdown**: EventBus.Stop() drains buffered channels before
  closing; workers stopped before event bus
- **No External Broker Required**: EventBus uses Go channels (not Kafka/
  RabbitMQ) for simplicity; external broker integration is a future
  enhancement
- **Asynchronous Processing**: Event delivery to webhook endpoints and
  other consumers MUST NOT block the publishing service

### X. Background Job Processing

Long-running or async operations (webhook delivery, email sending, etc.)
MUST be processed by background workers, never inline in HTTP request
handlers.

- **Worker Pools**: Each job type uses a configurable goroutine pool
  (`WEBHOOK_WORKER_CONCURRENCY`, etc.) with graceful start/stop
- **Queue Abstraction**: `WebhookQueue` interface abstracts Redis sorted
  sets; fallback in-memory queue for testing without Redis
- **Rate Limiting**: Sliding window rate limiting per webhook (default:
  100/min); fails open if Redis unavailable
- **Retry with Backoff**: Failed deliveries retry with exponential backoff
  (1min → 5min → 30min); maximum 3 retries
- **Stuck Delivery Recovery**: Reaper goroutine resets deliveries stuck in
  `processing` status for >5 minutes
- **Delivery Retention**: Completed/failed delivery records retained for
  configurable period (default: 90 days)
- **HMAC-SHA256 Signing**: All webhook payloads signed with
  `X-Webhook-Signature` header; secret prefix `whsec_` stripped before
  HMAC computation
- **Interface-Based Testability**: Queue, RateLimiter, and external HTTP
  clients abstracted behind interfaces with setter injection
  (`SetQueue()`, `SetRateLimiter()`)

---

## Quality Gates

### Phase 0 → Phase 1 Gate

**Must pass before design:**
- [ ] Constitution ratified by team
- [ ] All NEEDS CLARIFICATION items researched (research.md complete)
- [ ] No unresolved blocker dependencies

### Phase 1 → Phase 2 Gate

**Must pass before implementation:**
- [ ] data-model.md entity diagram approved
- [ ] contracts/api-v1.md endpoint specs signed off
- [ ] Casbin RBAC model config finalized
- [ ] No architecture violations detected
- [ ] Constitution re-check: no new principle violations

### Per-Phase Gate (Phase 2.1–2.10)

**After each phase implementation:**
- [ ] Unit tests: ≥80% coverage on business logic
- [ ] Integration tests: Real Postgres + Redis containers
- [ ] Linting: `golangci-lint` passes (or project-defined linter)
- [ ] No new principle violations
- [ ] Code review: At least one peer approval

### Final Release Gate

**Before production deployment:**
- [ ] All phases complete + gates passed
- [ ] End-to-end test: Full auth → permission → CRUD → audit flow
- [ ] Load test: ≥1000 req/s sustained for 5 minutes with p99 latency
  < 200ms, error rate < 0.1%
- [ ] Security review: JWT (HS256, 256-bit secret), bcrypt (cost ≥12),
  SQL injection (parameterized queries), CORS (explicit origin whitelist)
- [ ] Docker image built, health checks verified (database: 1s timeout,
  redis: 1s timeout, total: 2s timeout)
- [ ] Runbook written (deployment, rollback, scaling)

---

## Development Workflow

### Branching & Commits

- **Feature branches**: `feature/<phase>-<summary>`
  (e.g., `feature/006-webhook-system`)
- **Commits**: Atomic, descriptive; reference phase/task number
- **PRs**: Require one peer review before merge; gates must pass

### Code Review Checklist

Reviewers verify:
- ✅ **Principle compliance**: No RBAC enum, soft deletes used, JWT pattern
  correct, org scoping applied where needed
- ✅ **Test coverage**: New code has unit + integration tests
- ✅ **Error handling**: All external calls wrapped; standardized errors
- ✅ **Observability**: Structured logging, correlation IDs, context
  propagation
- ✅ **Documentation**: Godoc comments on public functions
- ✅ **No bloat**: Minimal dependencies, clean architecture
- ✅ **Background processing**: Long-running work uses worker pools,
  not inline in handlers

### Deployment Process

1. Run migrations in staging (verify rollback script)
2. Run tests in CI (unit, integration, linting)
3. Build Docker image with git SHA tag
4. Deploy to staging; validate health checks + basic flows
5. Manual sign-off for production
6. Blue-green deploy; monitor metrics for 15 min
7. Rollback plan ready (previous image, migration rollback)

---

## Technical Standards

### Logging

- **Structured**: Use `internal/logger.Logger` abstraction with key-value
  pairs (no printf-style)
- **Levels**: DEBUG (dev only), INFO (state changes), WARN (degraded),
  ERROR (action failed)
- **Correlation ID**: Every request tagged with request_id; propagate to
  logs and downstream calls
- **Context Fields**: `user_id`, `org_id`, `trace_id` extracted
  automatically by middleware
- **Sensitive Data**: Never log passwords, tokens, PII
- **Field Chaining**: Use `log.WithFields()` for scoped loggers with
  common fields (e.g., order_id, webhook_id)
- **Multiple Writers**: Support stdout, file (rotation via lumberjack),
  and syslog; configurable via `LOG_OUTPUTS` env var

### Error Handling

- **Custom Types**: Define domain-specific errors
  (e.g., `ErrNotFound`, `ErrPermissionDenied`) in `pkg/errors/`
- **HTTP Mapping**: Map to standard codes (400, 401, 403, 500)
- **Standardized Envelope**: All errors return
  `{error: {code, message, details}}`
- **Cause Chain**: Include wrapped error for debugging (but don't expose
  to client)

### Testing Strategy

- **Unit Tests**: Fast, isolated, ≥80% coverage on services + handlers;
  mock logger and external dependencies
- **Integration Tests**: Real Postgres + Redis (testcontainers-go)
- **Contract Tests**: HTTP API responses match contracts/api-v1.md
- **CI/CD**: All tests run in pipeline; fail fast
- **Data-Driven**: Use table-driven tests for edge cases
- **Background Jobs**: Test workers with in-memory queue and mock HTTP
  clients; verify retry logic, rate limiting, and stuck recovery

### Naming Conventions

- **Permissions**: `resource:action` (e.g., `invoice:create`, `user:read`,
  `webhook:manage`)
- **Roles**: PascalCase (e.g., `SuperAdmin`, `Manager`, `Viewer`)
- **Endpoints**: REST conventions (POST for create, PUT for update,
  DELETE for delete)
- **Tables**: snake_case plural (e.g., `users`, `audit_logs`,
  `webhook_deliveries`)
- **Functions**: camelCase, descriptive (e.g., `canUserEditInvoice`,
  `invalidatePermissionCache`, `dispatchEvent`)

### Architectural Patterns

- **Repository Pattern**: All data access through interface repos;
  `Repository` suffix (e.g., `WebhookRepository`)
- **Service Layer**: Business logic in `internal/service/`; handlers
  delegate to services
- **Setter Injection**: For cross-cutting concerns (EventBus, Queue,
  RateLimiter) to avoid constructor explosion
- **Domain Entities**: GORM models in `internal/domain/`; DTOs and
  response types co-located
- **Worker Pattern**: Background processors in `internal/service/`
  with `*_worker.go` naming; Start/Stop lifecycle

---

## Governance

### Constitution Amendments

Changes to this constitution require:
1. **Proposal**: Document rationale and impact
2. **Review**: Team discussion (minimum 3 days)
3. **Approval**: Unanimous consent (if blocking principle) or lead
   approval (if clarification)
4. **Migration**: Update project guidance; plan migration for affected
   code

### Violations & Waivers

Principle violations MUST be **explicitly justified** in PR description:
- **Why violated**: What requirement forced the deviation
- **Alternatives rejected**: Why simpler alternatives insufficient
- **Mitigation**: How risk is managed
- **Time-bound**: If temporary, clear exit criteria

Waivers must be approved by project lead + one peer reviewer.

### Documentation Maintenance

Constitution is a living document. Update when:
- New architectural decision made
- Violation pattern emerges
- Technology choice evolves
- Team learns from incident

Review quarterly or before major phases.

---

## Ratification & History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-04-22 | (Project Init) | Foundation ratified: RBAC mandatory, soft deletes, JWT + refresh, migrations, pub/sub invalidation, audit logging |
| 2.0 | 2026-05-08 | Atlas (Constitution Update) | MAJOR: Added Principle VIII (Organization-Scoped Multi-Tenancy), Principle IX (Event-Driven Architecture), Principle X (Background Job Processing). Expanded Principles I, II, IV, VI, VII to reflect evolved architecture (structured logging, org scoping, cache warm, webhook audit trail). Expanded Technical Standards with logging patterns, architectural patterns, and background job testing. Updated Naming Conventions with webhook examples. |

**Team Sign-Off**: (Pending team review)