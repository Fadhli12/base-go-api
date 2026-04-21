# Go API Base — Project Constitution

**Version**: 1.0 | **Ratified**: 2026-04-22 | **Status**: Active

---

## Core Principles

### I. Production-Ready Foundation

Every deliverable must be suitable for production deployment without modification. Code must handle edge cases, graceful degradation, and operational concerns (logging, metrics, health checks).

- **Test Coverage**: Minimum 80% unit + integration test coverage on business logic
- **Error Handling**: All external calls wrapped; standardized error envelopes
- **Observability**: Structured logging (slog) on all critical paths; correlation IDs propagated
- **Graceful Shutdown**: Signal handling (SIGTERM) with 30-second timeout context cancellation. Existing requests complete, new requests rejected.

### II. RBAC is Mandatory, Not Hardcoded

Permission system must be **customizable at runtime** without code changes or redeployment. Use Casbin + GORM to store permissions, roles, and user grants as database records.

- **No Permission Enum**: Permissions defined as data (resource:action pairs), not hardcoded constants
- **Runtime Grant/Revoke**: Admin endpoints to create roles, attach permissions, grant users without deploys
- **System Roles Protected**: Core roles (super_admin, admin) marked `is_system=true`; deletion prevented
- **Permission Naming Manifest**: `permission:sync` CLI command upserts known permissions from manifest on deploy

### III. Soft Deletes for Audit & Compliance

All user-generated data (users, roles, permissions, documents) uses soft deletes (`deleted_at` timestamp). No hard deletion of audit-relevant records.

- **Audit Trail Immutable**: Once logged, audit records cannot be deleted or modified
- **Undo Capable**: Soft deletes enable row restoration if needed
- **Compliance**: Retain historical data for legal/regulatory requirements
- **Query Scoping**: All queries automatically exclude soft-deleted rows via GORM scopes

### IV. Stateless JWT + Revocation

Authentication uses stateless JWT tokens for scalability. Access tokens expire in exactly **15 minutes** (900 seconds) with refresh tokens in PostgreSQL for revocation capability.

- **Access Token**: 15 minutes (900 seconds), contains user_id and email
- **Refresh Token**: 30 days (2592000 seconds), stored in DB, rotated on each use
- **Logout**: Mark refresh token as revoked; client discards access token
- **Token Blacklist Optional**: Use Redis blacklist if early revocation needed (logout seconds)

### V. PostgreSQL + Versioned Migrations (Not AutoMigrate)

Database schema managed via `golang-migrate/migrate` with versioned SQL files committed to repo. Never use GORM AutoMigrate in production.

- **Migration as Code**: All schema changes in `migrations/*.up.sql` / `*.down.sql`
- **Version Control**: Migrations tracked in git; reproducible across environments
- **CI/CD Friendly**: Migrations run as first step in deployment; no schema drift
- **No AutoMigrate**: Explicitly disable GORM AutoMigrate; it silently drifts schemas

### VI. Multi-Instance Permission Consistency

When running multiple API instances, permission cache invalidation must be **sub-second** and **consistent**. Use Redis pub/sub to notify all instances of permission changes.

- **Redis Pub/Sub**: Permission change events published to all subscribers
- **Cache TTL**: 30–60 second Redis cache TTL with immediate invalidation on change
- **No Eventual Consistency**: Invalidation must happen before response sent to user
- **Offline Graceful**: If pub/sub fails, fall back to TTL; no hard failure

### VII. Audit Logging Non-Negotiable

All user actions (CRUD, permission changes, admin operations) logged with actor, action, resource, before/after snapshot, timestamp, IP, user agent.

- **Before/After JSONB**: Capture state delta for all sensitive operations
- **Actor ID Always**: Who performed the action, even for system operations
- **Resource & Resource ID**: What was changed and its identifier
- **Compliance Ready**: Audit logs immutable; suitable for security reviews
- **Performance**: Async audit logging to avoid blocking user request

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
- [ ] All 10 phases complete + gates passed
- [ ] End-to-end test: Full auth → permission → CRUD → audit flow
- [ ] Load test: ≥1000 req/s sustained for 5 minutes with p99 latency < 200ms, error rate < 0.1%
- [ ] Security review: JWT (HS256, 256-bit secret), bcrypt (cost ≥12), SQL injection (parameterized queries), CORS (explicit origin whitelist)
- [ ] Docker image built, health checks verified (database: 1s timeout, redis: 1s timeout, total: 2s timeout)
- [ ] Runbook written (deployment, rollback, scaling)

---

## Development Workflow

### Branching & Commits

- **Feature branches**: `feature/<phase>-<summary>` (e.g., `feature/phase-2-auth`)
- **Commits**: Atomic, descriptive; reference phase/task number
- **PRs**: Require one peer review before merge; gates must pass

### Code Review Checklist

Reviewers verify:
- ✅ **Principle compliance**: No RBAC enum, soft deletes used, JWT pattern correct
- ✅ **Test coverage**: New code has unit + integration tests
- ✅ **Error handling**: All external calls wrapped; standardized errors
- ✅ **Observability**: Structured logging, correlation IDs
- ✅ **Documentation**: Godoc comments on public functions
- ✅ **No bloat**: Minimal dependencies, clean architecture

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

- **Structured**: Use `log/slog` with key-value pairs (no printf-style)
- **Levels**: DEBUG (dev only), INFO (state changes), WARN (degraded), ERROR (action failed)
- **Correlation ID**: Every request tagged with request_id; propagate to logs and downstream calls
- **Sensitive Data**: Never log passwords, tokens, PII

### Error Handling

- **Custom Types**: Define domain-specific errors (e.g., `ErrUserNotFound`, `ErrPermissionDenied`)
- **HTTP Mapping**: Map to standard codes (400, 401, 403, 500)
- **Standardized Envelope**: All errors return `{error: {code, message, details}}`
- **Cause Chain**: Include wrapped error for debugging (but don't expose to client)

### Testing Strategy

- **Unit Tests**: Fast, isolated, ≥80% coverage on services + handlers
- **Integration Tests**: Real Postgres + Redis (testcontainers-go)
- **Contract Tests**: HTTP API responses match contracts/api-v1.md
- **CI/CD**: All tests run in pipeline; fail fast
- **Data-Driven**: Use table-driven tests for edge cases

### Naming Conventions

- **Permissions**: `resource:action` (e.g., `invoice:create`, `user:read`, `invoice:*`)
- **Roles**: PascalCase (e.g., `SuperAdmin`, `Manager`, `Viewer`)
- **Endpoints**: REST conventions (POST for create, PUT for update, DELETE for delete)
- **Tables**: snake_case plural (e.g., `users`, `audit_logs`, `user_roles`)
- **Functions**: camelCase, descriptive (e.g., `canUserEditInvoice`, `invalidatePermissionCache`)

---

## Governance

### Constitution Amendments

Changes to this constitution require:
1. **Proposal**: Document rationale and impact
2. **Review**: Team discussion (minimum 3 days)
3. **Approval**: Unanimous consent (if blocking principle) or lead approval (if clarification)
4. **Migration**: Update project guidance; plan migration for affected code

### Violations & Waivers

Principle violations must be **explicitly justified** in PR description:
- **Why violated**: What requirement forced the deviation
- **Alternatives rejected**: Why simpler alternatives insufficient
- **Mitigation**: How risk is managed
- **Time-bound**: If temporary, clear exit criteria

Waivers must be approved by project lead + one peer reviewer.

### Documentation Maintenance

Constitution is living document. Update when:
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

**Team Sign-Off**: (Pending team review)

