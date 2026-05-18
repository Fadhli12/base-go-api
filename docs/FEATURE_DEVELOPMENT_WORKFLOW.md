# Feature Development Workflow

**Last Updated:** 2026-05-18  
**Applies to:** Go API Base project  
**Workflow:** SpecKit → Implementation → Review → Merge

---

## Overview

This document describes the complete end-to-end workflow for developing a new feature in the Go API Base project, from initial specification through code review and merge to master. The workflow uses [SpecKit](https://github.com/speckit) for structured feature planning and a multi-agent orchestration system for implementation.

The canonical example throughout this document is the **OAuth 2.0 Social Login** feature (branch `021-oauth-social-login`), which was developed following this exact workflow.

---

## Phase 0: Branch Setup

Before any work begins, create a numbered feature branch:

```bash
# Branch naming convention: {NNN}-{kebab-case-feature-name}
git checkout -b 021-oauth-social-login
```

**Branch numbering** comes from the feature number in `docs/FEATURE_RECOMMENDATIONS.md` or `FEATURE_RECOMMENDATIONS_V2.md`. The number determines the migration file prefix (e.g., `000027_oauth_providers.up.sql`).

---

## Phase 1: Specification (`/speckit.specify`)

### Purpose
Transform a feature idea into a structured specification document that captures requirements, constraints, and acceptance criteria.

### Command
```
/speckit.specify
```

### What it produces
- `specs/{NNN}-{feature-name}/spec.md` — Full feature specification

### How to invoke
```
"lets work with oauth-social-login with branch number 021, create it using /speckit.specify"
```

### Specification structure
The generated spec includes:
- **Overview** — Feature summary and motivation
- **Requirements** — Functional and non-functional requirements
- **Acceptance criteria** — Testable conditions for "done"
- **Constraints** — Architecture rules, security requirements, performance targets
- **Out of scope** — Explicitly excluded items

### Key decisions captured
- RFC references (e.g., RFC 9269 for fragment encoding, RFC 7636 for PKCE)
- Security model (HKDF-SHA256 key derivation, AES-256-GCM encryption)
- API design (endpoint list, request/response shapes)
- Integration points (EventBus, audit logging, SSRF protection)

---

## Phase 2: Clarification (`/speckit.clarify`)

### Purpose
Identify and resolve ambiguity, underspecification, and hidden risks before planning begins.

### Command
```
/speckit.clarify
```

### What it produces
- Updates `spec.md` with clarified requirements
- Asks up to 5 targeted clarification questions

### When to use
After specification but **before** planning. This catches:
- Missing edge cases (e.g., "What happens if a user unlinks their last auth method?")
- Ambiguous API contracts (e.g., "Should the callback redirect or return JSON?")
- Security gaps (e.g., "How are client secrets stored?")
- Integration conflicts (e.g., "Does this use the existing EventBus pattern?")

### Example clarifications (OAuth)
| Question | Resolution |
|----------|------------|
| How to deliver tokens in callback? | Fragment encoding per RFC 9269 (not query params) |
| Token storage for linked accounts? | Don't return tokens for link flows, only for login flows |
| Client secret storage? | HKDF-SHA256 key derivation + AES-256-GCM encryption at rest |
| Unlink last auth method? | Reject with 409 — must have at least one auth method |

---

## Phase 2.5: Pre-Planning Review (Metis + Momus)

### Purpose
Validate the specification's completeness and identify hidden intention traps before investing implementation time.

### Metis (Plan Consultant)
Analyzes the request to identify hidden intentions, ambiguities, and AI failure points.

```
"continue with plan, consult with @Metis (Plan Consultant)"
```

### Momus (Plan Critic)
Reviews the work plan for rigor, verifiability, and completeness. **Blocks plans with critical issues.**

```
"and review with @Momus (Plan Critic)"
```

### Flow
1. **Metis** identifies hidden risks and ambiguities
2. **Momus** reviews the plan — can REJECT (blocking issues) or APPROVE
3. If rejected: fix issues, re-submit
4. If approved: proceed to task generation

---

## Phase 3: Planning (`/speckit.plan`)

### Purpose
Transform the specification into a detailed implementation plan with phases, tasks, and dependencies.

### Command
```
/speckit.plan
```

### What it produces
- `specs/{NNN}-{feature-name}/plan.md` — Implementation plan
- `specs/{NNN}-{feature-name}/research.md` — Technical research
- `specs/{NNN}-{feature-name}/data-model.md` — Data model design
- `specs/{NNN}-{feature-name}/contracts/` — Interface contracts
- `specs/{NNN}-{feature-name}/quickstart.md` — Quick start guide

### Plan structure
- **Phases** — Ordered groups of related tasks
- **Dependencies** — What must be complete before each phase
- **Architecture decisions** — Key technical choices with rationale
- **Integration points** — How the feature connects to existing systems

### Example phases (OAuth)
| Phase | Content | Commit |
|-------|---------|--------|
| Phase 1 | Domain entities, config, migrations, permissions | `83c90ad` |
| Phase 2 | Repositories, state manager, encryption service | `b58e1ad` |
| Phase 3-4 | Login service, provider service, handler, request DTOs | `b98c509` |
| Phase 3-7+wiring | Login handler, account handler, server wiring, EventBus | `a019ee6` |
| Phase 8 polish | Build fix, integration tests, docs update | `655cd2d` |

---

## Phase 4: Task Generation (`/speckit.tasks`)

### Purpose
Break the plan into actionable, ordered tasks with dependencies.

### Command
```
/speckit.tasks
```

### What it produces
- `specs/{NNN}-{feature-name}/tasks.md` — Ordered task list

### Task format
Each task includes:
- Task ID (e.g., US1, US2, T001)
- Description
- Dependencies (which tasks must complete first)
- Acceptance criteria
- Estimated effort

---

## Phase 5: Implementation (`/speckit.implement`)

### Purpose
Execute tasks from `tasks.md` in order, producing working code.

### Command
```
/speckit.implement
```

### Orchestration model
Implementation uses the **Sisyphus orchestration system** with specialized subagents:

| Agent Type | Role | When to use |
|-----------|------|-------------|
| `explore` | Contextual codebase grep | Find existing patterns, file locations |
| `librarian` | External documentation search | Look up API references, library docs |
| `oracle` | High-reasoning consultation | Architecture decisions, debugging |
| `metis` | Pre-planning analysis | Scope clarification |
| `momus` | Plan criticism | Quality gate before implementation |
| `deep` (Sisyphus-Junior) | Focused task execution | Most implementation work |

### Delegation pattern

```typescript
// CORRECT: Delegate with full context
task(
  category="deep",
  load_skills=["clean-code-principles", "security-review"],
  run_in_background=true,
  description="Implement OAuth domain entities",
  prompt="1. TASK: Create OAuth provider and account domain entities
2. EXPECTED OUTCOME: Files in internal/domain/ with GORM models, DTOs, business methods
3. REQUIRED TOOLS: Write, Edit, Read, Bash
4. MUST DO: Follow existing domain entity patterns (UUID PKs, soft delete, ToResponse methods)
5. MUST NOT DO: Use AutoMigrate, skip soft delete, use int IDs
6. CONTEXT: See internal/domain/webhook.go for entity pattern reference"
)
```

### Key principles
- **DELEGATE, don't implement directly** — subagents produce measurably better code
- **Parallelize everything** — fire multiple agents simultaneously for independent work
- **Use `session_id`** for follow-up — preserves full context, saves tokens
- **Verify after delegation** — always check results against MUST DO/MUST NOT DO

### Commit strategy
Each logical unit of work gets its own commit:
```
feat(021): add OAuth domain entities, config, and migrations (Phase 1)
feat(021): add OAuth repositories, state manager, and encryption (Phase 2 partial)
feat(021): add OAuth login service, provider service, handler, and request DTOs (US1+US4)
feat(021): add OAuth login handler, account handler, server wiring, and service methods
feat(oauth): phase 8 polish — fix build, integration tests, docs update
```

---

## Phase 6: Code Review

### Purpose
Validate implemented code against project conventions, security standards, and architectural patterns.

### Review dimensions

#### 6.1 Security Review
Run via Oracle or security-review skill:
- Secret handling (client_secret encryption)
- Token delivery mechanism (fragment vs query params)
- SSRF protection on outbound requests
- PKCE implementation correctness
- State management Redis atomicity

#### 6.2 Pattern Adherence Review
Compare implementation against established project patterns:
- **Permission enforcement**: Services should NOT hold `enforcer` — handlers/middleware do permission checks
- **Soft delete**: All entities must have `DeletedAt gorm.DeletedAt`
- **UUID primary keys**: `uuid.UUID` with `gen_random_uuid()`
- **Context propagation**: All repo/service methods take `context.Context` as first param
- **Migration down files**: Must include `DROP TRIGGER` for auto-update triggers
- **EventBus integration**: Use `SetEventBus()` setter, not constructor injection
- **Response envelope**: Always use `response.SuccessWithContext()` / `response.ErrorWithContext()`

#### 6.3 Build Verification
```bash
go build ./...   # Must pass with zero errors
go vet ./...    # Must pass with zero warnings
```

### Example review findings (OAuth)
| Severity | Finding | Resolution |
|----------|---------|------------|
| CRIT | Services hold unused `enforcer` field | Removed from `OAuthLoginService` and `OAuthProviderService` |
| WARN | Handler uses inline `enforcer.Enforce()` calls | Refactored to `middleware.RequirePermission()` on route groups |
| WARN | Migration down files missing `DROP TRIGGER` | Added trigger cleanup to both down files |
| INFO | PKCE modulo bias (`int(byte) % 66`) | Acknowledged — ~1.5% bias, negligible in practice |

### Fix-and-commit workflow
```
1. Identify findings
2. Create todo list for each fix
3. Apply fixes sequentially
4. Build + vet verification after each fix
5. Commit all fixes together
```

---

## Phase 7: Documentation Update

### Purpose
Update `docs/FEATURE_STATUS.md` to reflect the completed feature.

### What to update
- Status: Change from `❌ NOT IMPLEMENTED` to `✅ FULLY IMPLEMENTED`
- Components table: Add all files created/modified
- Endpoints: List all new API endpoints
- Permissions: Document new permission entries
- Key design decisions: Note important architectural choices
- EventBus events: List new event types if applicable
- Build status: Update to `✅ PASSES`

### Example entry structure
```markdown
### 4.3 OAuth 2.0 Social Login

**Status:** ✅ FULLY IMPLEMENTED

**What Exists:**
| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/oauth_provider.go`, ... |
| Repository | `internal/repository/oauth_provider.go`, ... |
| Service | `internal/service/oauth_login.go`, ... |
| Handler | `internal/http/handler/oauth_provider.go`, ... |
| ... | ... |

**Endpoints:**
- `GET /api/v1/auth/oauth/:provider` — Initiate OAuth login
- ...

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `oauth:view` | oauth | view | View providers and accounts |
| ...

**Key Design Decisions:**
- Fragment encoding for token delivery (RFC 9269)
- PKCE S256 for all OAuth flows
- ...
```

### Also update
- `AGENTS.md` — Add the new feature to the WHERE TO LOOK table and CODE MAP
- `cmd/api/main.go` references — Ensure wiring is documented

---

## Phase 8: Merge to Master

### Purpose
Integrate the feature branch into master with conflict resolution.

### Step-by-step

```bash
# 1. Ensure all changes are committed on the feature branch
git add -A
git commit -m "docs: update FEATURE_STATUS with review fix notes"

# 2. Switch to master
git checkout master

# 3. Merge (may produce conflicts if master has diverged)
git merge 021-oauth-social-login --no-edit

# 4. Resolve conflicts if any
# Common conflict files:
#   - AGENTS.md (speckit section)
#   - docs/FEATURE_STATUS.md (status updates)
#   - internal/http/server.go (new routes from different features)

# 5. Verify build passes after conflict resolution
go build ./...
go vet ./...

# 6. Commit merge
git add -A
git commit -m "Merge 021-oauth-social-login into master"
```

### Conflict resolution strategy
| File | Strategy |
|------|----------|
| `AGENTS.md` | Keep the newest feature's speckit section |
| `FEATURE_STATUS.md` | Merge both — include all feature status entries |
| `server.go` | Combine routes from both branches (idempotency + OAuth) |
| `cmd/api/main.go` | Combine wiring from both branches |

### Common merge conflict pattern
When two feature branches add routes in the same `RegisterRoutes()` method, the conflict shows both additions. **Resolve by including BOTH sets of routes** — each feature adds its own independent route group.

---

## Complete Workflow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    FEATURE DEVELOPMENT FLOW                   │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. CREATE BRANCH                                             │
│     git checkout -b 021-oauth-social-login                   │
│                                                               │
│  2. SPECIFY  ─────────►  specs/021-.../spec.md               │
│     /speckit.specify                                          │
│                                                               │
│  3. CLARIFY  ─────────►  Updated spec.md                     │
│     /speckit.clarify                                          │
│                                                               │
│  4. PRE-REVIEW                                               │
│     Metis (consult) ──► Momus (critique) ──► APPROVE/REJECT   │
│                                                               │
│  5. PLAN ──────────────►  plan.md, research.md,               │
│     /speckit.plan          data-model.md, contracts/,         │
│                             quickstart.md                      │
│                                                               │
│  6. TASKS ─────────────►  tasks.md                            │
│     /speckit.tasks                                            │
│                                                               │
│  7. IMPLEMENT                                                  │
│     /speckit.implement                                        │
│     │                                                          │
│     ├── Phase 1: Domain + Config + Migrations                 │
│     ├── Phase 2: Repositories + Services                      │
│     ├── Phase 3+: Handlers + Wiring                           │
│     └── Phase N: Polish + Tests + Docs                         │
│                                                               │
│  8. CODE REVIEW                                               │
│     Security ──► Pattern ──► Build ──► Fix ──► Commit         │
│                                                               │
│  9. UPDATE DOCS                                               │
│     FEATURE_STATUS.md ──► AGENTS.md ──► Commit               │
│                                                               │
│  10. MERGE TO MASTER                                          │
│      git checkout master                                      │
│      git merge 021-oauth-social-login                          │
│      (resolve conflicts)                                      │
│      go build && go vet                                       │
│      git commit -m "Merge 021-oauth-social-login into master"│
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Conventions Reference

### Branch naming
```
{NNN}-{kebab-case-feature-name}
```
Examples: `019-idempotency-keys`, `020-ssrf-protection`, `021-oauth-social-login`

### Commit message format
```
feat({NNN}): {what was added} ({phase/context})
fix({scope}): {what was fixed} ({detail})
docs: {what was documented}
```

### Migration numbering
```
migrations/{NNNNNN}_{descriptive_name}.up.sql
migrations/{NNNNNN}_{descriptive_name}.down.sql
```
The feature number determines the migration prefix. Each feature may use multiple sequential files.

### Permission naming
```
{resource}:{action}
```
Examples: `oauth:view`, `oauth:link`, `oauth:manage`

### EventBus event naming
```
{domain}.{entity}.{action}
```
Examples: `auth.oauth.linked`, `auth.oauth.unlinked`

---

## Troubleshooting

### SpecKit commands not found
Ensure `.opencode/command/` contains the speckit command files:
```bash
ls .opencode/command/speckit.*.md
```

### Build fails after merge conflict resolution
1. Check `server.go` for duplicate route registrations
2. Verify all imports are present after conflict resolution
3. Run `go build ./...` before committing

### Integration tests fail
Integration tests require Docker (testcontainers). Run with:
```bash
go test -v -tags=integration ./tests/integration/... -timeout 5m
```

### Review findings keep appearing
Address each finding individually, build+vet after each fix, then commit all fixes together. Don't batch unrelated fixes.

---

## Related Documents

- [FEATURE_STATUS.md](FEATURE_STATUS.md) — Current feature implementation status
- [FEATURE_RECOMMENDATIONS.md](FEATURE_RECOMMENDATIONS.md) — V1 feature backlog
- [FEATURE_RECOMMENDATIONS_V2.md](FEATURE_RECOMMENDATIONS_V2.md) — V2 feature backlog
- [AGENTS.md](../AGENTS.md) — Agent instructions and codebase map
- [CONTRIBUTING.md](../README.md) — Contributing guidelines (if exists)