# Tasks: API Key Authentication

**Input**: Design documents from `/specs/api-key-auth/`
**Prerequisites**: plan.md ✅ | spec.md ✅ | data-model.md ✅ | contracts/api-v1.md ✅ | research.md ✅ | quickstart.md ✅

**Tests**: No test tasks included unless explicitly requested. TDD approach can be added later.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

---

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Database & Dependencies)

**Purpose**: Database schema and shared infrastructure for API key feature

- [ ] T001 Create migration file `migrations/000005_api_keys.up.sql` with api_keys table schema
- [ ] T002 Create migration rollback `migrations/000005_api_keys.down.sql` to drop api_keys table
- [ ] T003 Add `api_key:create`, `api_key:read`, `api_key:revoke`, `api_key:manage` permissions to `config/permissions.yaml`
- [ ] T004 Run migration: `make migrate`
- [ ] T005 Run seed: `make seed` (creates new permissions in database)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core domain models and utilities that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T006 [P] Create domain entity `internal/domain/api_key.go` with APIKey struct, APIKeyResponse, ToResponse(), IsActive(), HasScope()
- [ ] T007 [P] Create repository interface `internal/repository/api_key.go` with APIKeyRepository interface
- [ ] T008 [P] Create request DTOs `internal/http/request/api_key.go` with CreateAPIKeyRequest, APIKeyListQuery, validation
- [ ] T009 Implement repository `internal/repository/api_key.go` (Create, FindByID, FindByKeyHash, FindByUserID, Update, Revoke, SoftDelete, SoftDeleteByUserID, UpdateLastUsedAt, CountActiveByUserID)
- [ ] T010 Add key generation utility to `internal/service/api_key.go` (GenerateKey: crypto/rand 32 chars, GeneratePrefix: first 12 chars, HashKey: bcrypt cost 12)

**Checkpoint**: Foundation ready - API key domain, repository, and utilities available

---

## Phase 3: User Story 1 - Create API Key (Priority: P1) 🎯 MVP

**Goal**: User can create named API keys with custom scopes for integrations

**Independent Test**: POST /api-keys returns key value (shown once), key stored securely with bcrypt hash

### Implementation for User Story 1

- [ ] T011 [P] [US1] Implement APIKeyService.Create in `internal/service/api_key.go` with scope validation, key generation, hashing
- [ ] T012 [US1] Implement CreateAPIKey handler in `internal/http/handler/api_key.go` (POST /api-keys)
- [ ] T013 [US1] Add validation middleware for scopes (check user has permissions for requested scopes)
- [ ] T014 [US1] Add audit logging for api_key:create action in `internal/service/api_key.go`
- [ ] T015 [US1] Register route POST /api-keys in `internal/http/server.go` with JWT middleware
- [ ] T016 [US1] Implement ValidateScopes() in `internal/service/api_key.go` (query user permissions from Casbin, validate each scope)

**Checkpoint**: User Story 1 complete - can create API keys via POST /api-keys with JWT authentication

---

## Phase 4: User Story 2 - List API Keys (Priority: P2)

**Goal**: User can list all their API keys to manage and monitor them

**Independent Test**: GET /api-keys returns list with name, prefix, scopes, created_at, last_used_at, is_revoked

### Implementation for User Story 2

- [ ] T017 [P] [US2] Implement APIKeyService.List in `internal/service/api_key.go` with pagination (limit, offset)
- [ ] T018 [US2] Implement ListAPIKeys handler in `internal/http/handler/api_key.go` (GET /api-keys)
- [ ] T019 [US2] Add GetByID method in `internal/service/api_key.go` with ownership check
- [ ] T020 [US2] Implement GetAPIKey handler in `internal/http/handler/api_key.go` (GET /api-keys/:id)
- [ ] T021 [US2] Register routes GET /api-keys and GET /api-keys/:id in `internal/http/server.go`

**Checkpoint**: User Story 2 complete - can list and view API keys

---

## Phase 5: User Story 3 - Revoke API Key (Priority: P3)

**Goal**: User can revoke API keys to prevent further use while retaining audit trail

**Independent Test**: DELETE /api-keys/:id sets revoked_at, subsequent auth with key returns 401

### Implementation for User Story 3

- [ ] T022 [P] [US3] Implement APIKeyService.Revoke in `internal/service/api_key.go` with ownership check
- [ ] T023 [US3] Implement RevokeAPIKey handler in `internal/http/handler/api_key.go` (DELETE /api-keys/:id)
- [ ] T024 [US3] Add audit logging for api_key:revoke action
- [ ] T025 [US3] Implement RevokeSoft in `internal/repository/api_key.go` (sets revoked_at timestamp)
- [ ] T026 [US3] Register route DELETE /api-keys/:id in `internal/http/server.go`

**Checkpoint**: User Story 3 complete - can revoke API keys

---

## Phase 6: User Story 4 - API Key Authentication (Priority: P1) 🎯 MVP

**Goal**: External services can authenticate using X-API-Key header

**Independent Test**: GET /invoices with X-API-Key header authenticates and returns data based on key scopes

### Implementation for User Story 4

- [ ] T027 [P] [US4] Create API key authentication middleware `internal/http/middleware/api_key.go`
- [ ] T028 [US4] Implement APIKeyService.Validate in `internal/service/api_key.go` (validate key not revoked, not expired, user active)
- [ ] T029 [US4] Implement UpdateLastUsedAt in `internal/repository/api_key.go` - use atomic UPDATE to avoid race conditions
- [ ] T030 [US4] Add last_used_at optimization: skip update if updated within last 1 minute (reduce DB writes)
- [ ] T031 [US4] Integrate API key middleware after JWT extraction in auth chain
- [ ] T032 [US4] Update permission middleware `internal/http/middleware/permission.go` to check scopes when present
- [ ] T033 [US4] Implement scope-to-permission mapping in `internal/service/api_key.go` (scopeAllowsAction function)
- [ ] T034 [US4] Add user context population from API key (user_id, scopes, api_key_id)

**Checkpoint**: User Story 4 complete - can authenticate with API keys, scope validation works

---

## Phase 7: User Story 5 - Key Expiration Enforcement (Priority: P2)

**Goal**: Optional expiration ensures keys don't remain active indefinitely

**Independent Test**: Key with expires_at in past returns 401 on authentication

### Implementation for User Story 5

- [ ] T035 [P] [US5] Add expiration check in APIKeyService.Validate (is_expired check)
- [ ] T036 [US5] Add expires_at field validation in CreateAPIKeyRequest (optional, must be future timestamp)
- [ ] T037 [US5] Return 401 with "API key expired" message when expired key used
- [ ] T038 [US5] Update APIKeyResponse to include expires_at field
- [ ] T039 [US5] Update quickstart.md with expiration example

**Checkpoint**: User Story 5 complete - API key expiration enforced

---

## Phase 8: Rate Limiting & Permission Integration

**Purpose**: Cross-cutting concerns that affect all API key operations

- [ ] T040 [P] Add per-API-key rate limiting in `internal/http/middleware/rate_limit.go`
- [ ] T041 [P] Configure rate limit for API keys (default: 60 requests/minute) vs user sessions (100 req/min)
- [ ] T042 Update rate limit identifier to use api_key:{key_id} when API key auth is used
- [ ] T043 [P] Add permission check for api_key:create in create handler
- [ ] T044 [P] Add permission check for api_key:read in list/get handlers
- [ ] T045 [P] Add permission check for api_key:revoke in revoke handler

---

## Phase 9: Polish & Documentation

**Purpose**: Documentation, cleanup, and final touches

- [ ] T046 [P] Add OpenAPI annotations to CreateAPIKey handler
- [ ] T047 [P] Add OpenAPI annotations to ListAPIKeys handler
- [ ] T048 [P] Add OpenAPI annotations to GetAPIKey handler
- [ ] T049 [P] Add OpenAPI annotations to RevokeAPIKey handler
- [ ] T050 Update `README.md` with API key usage section
- [ ] T051 Update `AGENTS.md` with API key references
- [ ] T052 Add API key error codes to error handling (INVALID_SCOPE, SCOPE_NOT_ALLOWED, KEY_NAME_EXISTS, KEY_EXPIRED, KEY_REVOKED, KEY_NOT_FOUND)
- [ ] T053 [P] Add example curl commands to docs/api-examples.md (or similar)
- [ ] T054 Run quickstart.md validation (test all examples work)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - US1, US2, US3 can run in parallel (if staffed)
  - US4 can run in parallel (if staffed)
  - US5 depends on US4 (expiration check in auth flow)
- **Rate Limiting (Phase 8)**: Depends on US4 (API key middleware)
- **Polish (Phase 9)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (Create)**: Can start after Foundational (Phase 2) - No dependencies
- **User Story 2 (List)**: Can start after Foundational (Phase 2) - No dependencies
- **User Story 3 (Revoke)**: Can start after Foundational (Phase 2) - No dependencies
- **User Story 4 (Authentication)**: Can start after Foundational (Phase 2) - Requires US1 for key creation to test
- **User Story 5 (Expiration)**: Must come after US4 (expiration check in auth flow)

### Within Each User Story

- Models before services (T006-T008 must complete before T011)
- Services before handlers (T011 before T012)
- Handlers before route registration (T012 before T015)
- Audit logging after core implementation

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- Phase 1 tasks (T001-T003) can run in parallel
- Phase 2 tasks (T006-T008) can run in parallel (different files)
- User Stories US1, US2, US3, US4 can run in parallel if team capacity allows
- All OpenAPI annotations (T046-T049) can run in parallel

---

## Parallel Example: Foundation Phase

```bash
# Launch all domain/entity tasks in parallel:
Task: "Create domain entity internal/domain/api_key.go"
Task: "Create repository interface internal/repository/api_key.go"
Task: "Create request DTOs internal/http/request/api_key.go"

# Then sequentially:
Task: "Implement repository internal/repository/api_key.go"
Task: "Add key generation utility to internal/service/api_key.go"
```

## Parallel Example: User Stories 1-4

```bash
# After Foundational phase, all these can run in parallel:
Task: "US1: Implement Create API key flow"
Task: "US2: Implement List API keys flow"
Task: "US3: Implement Revoke API key flow"
Task: "US4: Implement API key authentication middleware"

# US5 must wait for US4
Task: "US5: Implement key expiration enforcement"
```

---

## Implementation Strategy

### MVP First (User Stories 1 & 4 Only)

1. Complete Phase 1: Setup (migration, permissions)
2. Complete Phase 2: Foundational (domain, repository, DTOs)
3. Complete Phase 3: User Story 1 (Create API key)
4. Complete Phase 6: User Story 4 (API key authentication)
5. **STOP and VALIDATE**: Create key → Use key to authenticate
6. Deploy/demo if ready

### Full Feature Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 (Create) → Can create keys → Deploy/Demo (MVP!)
3. Add User Story 2 (List) → Can view keys → Deploy/Demo
4. Add User Story 3 (Revoke) → Can revoke keys → Deploy/Demo
5. Add User Story 4 (Auth) → Can use keys → Deploy/Demo
6. Add User Story 5 (Expiration) → Key expiry → Deploy/Demo
7. Add Rate Limiting → Per-key limits → Deploy
8. Add Polish → Documentation → Final release

### Testing Strategy

**Unit Tests** (if TDD approach is requested):
- Domain: IsActive(), HasScope(), ToResponse()
- Repository: CRUD operations with testcontainers
- Service: Key generation, scope validation, authentication

**Integration Tests** (if TDD approach is requested):
- Full auth flow: Create key → Authenticate → Use → Revoke
- Rate limiting: Per-key rate limit enforcement
- Audit logging: Create/revoke operations logged

---

## Task Summary

| Phase | Tasks | Parallel | Sequential |
|-------|-------|----------|------------|
| Setup | 5 | 3 | 2 |
| Foundational | 5 | 3 | 2 |
| US1: Create | 6 | 1 | 5 |
| US2: List | 5 | 1 | 4 |
| US3: Revoke | 5 | 1 | 4 |
| US4: Authenticate | 8 | 1 | 7 |
| US5: Expiration | 5 | 1 | 4 |
| Rate Limiting | 6 | 4 | 2 |
| Polish | 9 | 4 | 5 |
| **Total** | **54** | **19** | **35** |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Migration number: 000005 (after 000003_news and 000004_media)
- All user context from API key: user_id, scopes, api_key_id
- Rate limits: API keys 60/min vs users 100/min