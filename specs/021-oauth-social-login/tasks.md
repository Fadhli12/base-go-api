# Tasks: OAuth 2.0 Social Login

**Input**: Design documents from `/specs/021-oauth-social-login/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/api-v1.md

**Tests**: Not explicitly requested in the spec. Test tasks are omitted. Integration tests will be covered implicitly by the project's 80% coverage requirement.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US5)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, migrations, config, and domain entities needed by all stories.

- [ ] T001 Create OAuth config struct in `internal/config/oauth.go` with env var mapping (OAUTH_ENCRYPTION_KEY, OAUTH_ALLOW_HTTP, OAUTH_FRONTEND_CALLBACK_URL, OAUTH_STATE_TTL) and provider default scopes (Google: openid/email/profile, GitHub: user:email/read:user, Microsoft: openid/email/profile)
- [ ] T002 Create OAuthProvider domain entity in `internal/domain/oauth_provider.go` with GORM model (UUID PK, name, display_name, client_id, client_secret_encrypted, redirect_url, additional_scopes, config JSONB, is_enabled, is_system, organization_id, timestamps, soft delete), business methods (EncryptSecret, DecryptSecret, GetEffectiveScopes, GetAuthorizationURL, GetTokenURL, GetUserInfoURL, ToResponse), and DTOs (CreateOAuthProviderRequest, UpdateOAuthProviderRequest, OAuthProviderResponse)
- [ ] T003 [P] Create OAuthAccount domain entity in `internal/domain/oauth_account.go` with GORM model (UUID PK, user_id FK, provider_id FK, provider_user_id string, email, email_verified, display_name, avatar_url, timestamps, soft delete), unique constraints documentation, and ToResponse DTO (OAuthAccountResponse)
- [ ] T004 [P] Create OAuth event definitions in `internal/domain/oauth_events.go` with EventBus event types (auth.oauth.linked, auth.oauth.unlinked) following the existing webhook_events.go pattern (buffered channel, Subscribe/Publish, Start/Stop)
- [ ] T005 Create SQL migration `migrations/000027_oauth_providers.up.sql` and `000027_oauth_providers.down.sql` with oauth_providers table (id UUID PK, name VARCHAR50 UNIQUE WHERE deleted_at IS NULL, display_name, client_id, client_secret_encrypted, redirect_url, additional_scopes TEXT[] DEFAULT '{}', config JSONB DEFAULT '{}', is_enabled bool DEFAULT true, is_system bool DEFAULT false, organization_id UUID FK, timestamps, deleted_at, partial unique indexes, column comments)
- [ ] T006 Create SQL migration `migrations/000028_oauth_accounts.up.sql` and `000028_oauth_accounts.down.sql` with oauth_accounts table (id UUID PK, user_id UUID FK, provider_id UUID FK, provider_user_id VARCHAR255, email VARCHAR255, email_verified bool DEFAULT false, display_name, avatar_url, timestamps, deleted_at, partial unique indexes on provider_user_id and user_provider, column comments)
- [ ] T007 Add OAuth permissions to `config/permissions.yaml` under a new `oauth` section: oauth:view (resource: oauth, action: view, scope: own), oauth:link (resource: oauth, action: link, scope: own), oauth:manage (resource: oauth, action: manage, scope: all)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core service infrastructure that MUST be complete before ANY user story can be implemented.

**CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T008 Create OAuthProviderRepository interface and GORM implementation in `internal/repository/oauth_provider.go` with methods: Create, FindByID, FindByName, FindEnabled (global + org-scoped), FindAll (with pagination + org filter), Update, SoftDelete, and org-scoped query helper
- [ ] T009 Create OAuthAccountRepository interface and GORM implementation in `internal/repository/oauth_account.go` with methods: Create, FindByID, FindByUserAndProvider, FindByProviderAndProviderUserID, FindByProviderID, CountByUserID (for auth method check), SoftDelete, and helper for counting auth methods (password + linked providers)
- [ ] T010 Create OAuthStateManager in `internal/service/oauth_state.go` with interface (CreateState, ValidateState) and Redis implementation using `oauth:state:{nonce}` key, 600s TTL, JSON value containing callback_url/provider/intent/user_id/code_verifier/org_id/created_at, atomic GET+DEL on validation
- [ ] T011 Create client secret encryption service in `internal/service/oauth_encryption.go` with DeriveEncryptionKey (HKDF-SHA256, info="oauth-encryption-v1"), Encrypt (AES-256-GCM, format v1:base64(IV+ciphertext+tag)), Decrypt (reverse), key override via OAUTH_ENCRYPTION_KEY env var
- [ ] T012 Create OAuthConfig loader in `internal/config/oauth.go` with OAuthConfig struct loaded from environment variables (OAUTH_ENCRYPTION_KEY, OAUTH_ALLOW_HTTP, OAUTH_FRONTEND_CALLBACK_URL, OAUTH_STATE_TTL), provider default scope maps (Google/GitHub/Microsoft), and SSRF-safe HTTP client construction following existing `internal/ssrf/` patterns
- [ ] T013 Create ProviderProfile normalization struct in `internal/domain/oauth_provider.go` (or separate `internal/service/oauth_provider_profile.go`) with ProviderProfile{ProviderID, Email, EmailVerified, DisplayName, AvatarURL} and NormalizeGoogleProfile, NormalizeGitHubProfile, NormalizeMicrosoftProfile functions that map raw provider JSON to the unified struct
- [ ] T014 Add OAuth service and handler wiring to `cmd/api/main.go` following existing patterns: create OAuthProviderRepository, OAuthAccountRepository, repositories → create OAuthProviderService, OAuthLoginService, OAuthLinkService, OAuthStateManager → create handlers → register routes on Echo v1 group → inject EventBus setters → wire into graceful shutdown

**Checkpoint**: Foundation ready — repositories, state manager, encryption, config, and server wiring complete. User story implementation can begin.

---

## Phase 3: User Story 1 — Sign In with Social Provider (Priority: P1) 🎯 MVP

**Goal**: Users can click "Sign in with Google/GitHub/Microsoft", authenticate, and receive JWT tokens.

**Independent Test**: Initiate OAuth flow with a configured provider, complete consent screen, verify tokens in URL fragment redirect.

- [ ] T015 [US1] Create OAuthLoginService in `internal/service/oauth_login.go` with InitiateLogin (generates state + PKCE code_verifier, stores Redis state, builds authorization URL with code_challenge, returns redirect URL), HandleCallback (validates state from Redis, exchanges authorization code with PKCE code_verifier, fetches provider user info, normalizes profile, handles login/link intent routing, creates/finds user, generates JWT tokens via TokenService, builds fragment redirect URL with tokens or error)
- [ ] T016 [US1] Create OAuthLoginHandler in `internal/http/handler/oauth_login.go` with GET /api/v1/auth/oauth/:provider (initiate login, 302 redirect) and GET /api/v1/auth/oauth/:provider/callback (handle callback, validate state, process login, 302 redirect to frontend with fragment)
- [ ] T017 [US1] Implement email handling logic in OAuthLoginService: verified email → create/find user, unverified email → redirect with #error=email_not_verified, missing email → create user with placeholder email oauth-{uuid}@social.placeholder and set requires_email_update=true on User entity
- [ ] T018 [US1] Implement password hash placeholder for social-only users: when auto-creating a user via social login (no password), set password_hash to a random 64-char hex string (unmatchable) so the user must use OAuth to authenticate
- [ ] T019 [US1] Add requires_email_update field support: add X-Requires-Email-Update response header in login callback fragment when the user has requires_email_update=true, and add email_update_required boolean to UserResponse DTO

**Checkpoint**: User Story 1 complete — users can sign in with a configured OAuth provider and receive JWT tokens.

---

## Phase 4: User Story 4 — Manage OAuth Providers (Admin) (Priority: P1) 🎯 MVP

**Goal**: Admins can create, update, enable/disable, and delete OAuth provider configurations.

**Independent Test**: Create a Google provider config via admin endpoint, verify it appears in public providers list, disable it, verify it disappears.

- [ ] T020 [US4] Create OAuthProviderService in `internal/service/oauth_provider.go` with Create (validate name, encrypt client_secret, store), FindByID, FindByName, FindEnabled (for public listing), FindAll (with pagination + org filter for admin), Update (partial update, re-encrypt client_secret if changed), Delete (soft delete, prevent deletion of is_system=true), ToggleEnabled, and audit logging for all CRUD operations
- [ ] T021 [US4] Create OAuthProviderHandler in `internal/http/handler/oauth_provider.go` with POST /api/v1/oauth-providers (create, 201), GET /api/v1/oauth-providers (list with pagination), GET /api/v1/oauth-providers/:id (get by ID), PUT /api/v1/oauth-providers/:id (update), DELETE /api/v1/oauth-providers/:id (soft delete), and GET /api/v1/auth/oauth/providers (public list — no auth required, returns name/display_name/scopes only)
- [ ] T022 [US4] Add handler-level permission enforcement: admin endpoints require oauth:manage via Casbin Enforce(userID, orgID, "oauth", "manage"), public providers endpoint requires no auth
- [ ] T023 [US4] Implement callback URL allowlist validation: when creating/updating a provider, validate redirect_url against OAUTH_FRONTEND_CALLBACK_URL (or an allowlist). If OAUTH_ALLOW_HTTP=false, reject HTTP URLs. SSRF protection on outbound HTTP to provider endpoints using internal/ssrf package

**Checkpoint**: User Story 4 complete — admins can manage OAuth provider configurations and public endpoint lists enabled providers.

---

## Phase 5: User Story 2 — Link an OAuth Provider to Existing Account (Priority: P2)

**Goal**: Authenticated users can link a social account to their existing account.

**Independent Test**: Authenticate with password, call POST /link with provider authorization, verify social account appears in linked accounts.

- [ ] T024 [US2] Create OAuthLinkService in `internal/service/oauth_link.go` with InitiateLink (generates state + PKCE code_verifier with intent=link and user_id, stores Redis state, builds authorization URL, returns redirect_url as JSON), HandleLinkCallback (validates state, exchanges code, normalizes profile, checks provider not already linked, checks provider_user_id not linked to different user, creates OAuthAccount, publishes auth.oauth.linked event, builds success fragment)
- [ ] T025 [US2] Add link endpoint to OAuthLoginHandler (or create OAuthAccountHandler) with POST /api/v1/auth/oauth/:provider/link (requires JWT + oauth:link permission, returns JSON {redirect_url}) — this stores state with intent=link and user_id in Redis, then returns the provider authorization URL as JSON (NOT a 302 redirect)
- [ ] T026 [US2] Add shared callback handler routing: the GET /api/v1/auth/oauth/:provider/callback handler reads intent from Redis state and routes to HandleLogin (intent=login) or HandleLink (intent=link)
- [ ] T027 [US2] Implement duplicate link prevention: check UNIQUE(user_id, provider_id) WHERE deleted_at IS NULL before creating OAuthAccount, return 409 if already linked; check UNIQUE(provider_id, provider_user_id) WHERE deleted_at IS NULL, return 409 if linked to different user

**Checkpoint**: User Story 2 complete — authenticated users can link social providers to their accounts.

---

## Phase 6: User Story 3 — Unlink an OAuth Provider from Account (Priority: P2)

**Goal**: Authenticated users can unlink a social account, with last-auth-method protection.

**Independent Test**: Link then unlink a provider, verify it no longer appears in linked accounts. Verify last-auth-method protection blocks unlink when only method remains.

- [ ] T028 [US3] Add Unlink method to OAuthLinkService (or create dedicated method) that: verifies user has the provider linked, counts user auth methods (password hash is not placeholder + linked providers count ≥ 2), if only one method remains return 409 LAST_AUTH_METHOD, soft-delete the OAuthAccount, publishes auth.oauth.unlinked event, creates audit log entry
- [ ] T029 [US3] Add unlink endpoint DELETE /api/v1/auth/oauth/:provider/unlink (requires JWT + oauth:link permission) to OAuthAccountHandler, calls Unlink service method, returns 200 {message: "Provider unlinked successfully"} or appropriate error
- [ ] T030 [US3] Implement audit logging for unlink: create audit entry with action=oauth_account.unlinked, resource_type=oauth_account, before/after JSONB capturing provider name, user ID, provider user ID

**Checkpoint**: User Story 3 complete — users can unlink providers with last-auth-method protection.

---

## Phase 7: User Story 5 — View Linked Accounts (Priority: P3)

**Goal**: Authenticated users can view which social accounts are linked to their profile.

**Independent Test**: Link multiple providers, call GET /accounts, verify all linked accounts are returned with correct details.

- [ ] T031 [US5] Add ListLinkedAccounts method to OAuthAccountRepository that finds all non-deleted OAuthAccounts by user_id with provider eager loading, maps to OAuthAccountResponse DTOs (provider name, email, display_name, avatar_url, linked_at)
- [ ] T032 [US5] Add GET /api/v1/auth/oauth/accounts endpoint to OAuthAccountHandler (requires JWT + oauth:view permission) that returns {accounts: [OAuthAccountResponse...]}

**Checkpoint**: User Story 5 complete — users can view their linked accounts.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: EventBus integration, SSRF hardening, structured logging, and documentation.

- [ ] T033 [P] Wire EventBus integration in `cmd/api/main.go`: create EventBus, call SetEventBus on OAuthProviderService (for provider change events), call SubscribeToEventBus on OAuthLinkService (for link/unlink events), start EventBus in background, add EventBus.Stop() to graceful shutdown order (between server and workers per existing pattern)
- [ ] T034 [P] Add structured logging throughout OAuth services using `internal/logger.Logger` with context propagation (request_id, user_id, org_id, provider) — use log.WithFields for scoped loggers (e.g., orderLog := log.WithFields(log.String("provider", "google")))
- [ ] T035 [P] Ensure SSRF-safe HTTP clients for all outbound requests to OAuth providers (token exchange, user info fetch) using `internal/ssrf.NewClient()` — never use http.DefaultClient or raw http.Client{}
- [ ] T036 [P] Add audit logging for all OAuth provider CRUD operations (oauth_provider.created, oauth_provider.updated, oauth_provider.deleted) and link/unlink operations (oauth_account.linked, oauth_account.unlinked) and social login (auth.oauth.login) with before/after JSONB, excluding client_secret from after-state
- [ ] T037 [P] Verify all OAuth endpoints suppress logging of Location headers containing token fragments (StructuredLogging middleware config or custom log filter)
- [ ] T038 Update `docs/FEATURE_STATUS.md` OAuth Social Login row: status → ✅ IMPLEMENTED, completion → 100%, notes → PKCE+S256, HKDF+AES-GCM, fragment encoding RFC 9269
- [ ] T039 Add integration tests in `tests/integration/oauth_test.go` covering: full login flow (state → redirect → callback → tokens), link/unlink flow, admin CRUD, error scenarios (invalid state, expired state, provider disabled, last auth method), and fragment URL verification
- [ ] T040 Run `permission:sync` command and verify oauth:view, oauth:link, oauth:manage permissions appear in database with correct resource/action/scope

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational — core login flow
- **US4 (Phase 4)**: Depends on Foundational — admin CRUD (can run in parallel with US1 since they touch different files)
- **US2 (Phase 5)**: Depends on US1 (callback handler from US1 is shared) — link flow extends login callback
- **US3 (Phase 6)**: Depends on US2 (needs OAuthAccount entity and linked accounts) — unlink uses same service
- **US5 (Phase 7)**: Depends on US2 (needs OAuthAccount repository) — view uses same query
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (Sign In)**: Depends on Foundational only — can start immediately after Phase 2
- **US4 (Admin CRUD)**: Depends on Foundational only — can run in parallel with US1
- **US2 (Link)**: Depends on US1 (shared callback handler) — must start after US1's callback is complete
- **US3 (Unlink)**: Depends on US2 (OAuthAccount entity and link service) — must start after US2
- **US5 (View)**: Depends on US2 (OAuthAccount repository) — can start after US2 but parallel with US3

### Within Each User Story

- Domain entities before repositories (Phase 1 entities → Phase 2 repos)
- Repositories before services (Phase 2 repos → Phase 3+ services)
- Services before handlers (Phase 3+ services → Phase 3+ handlers)
- Handler routing before integration

### Parallel Opportunities

- T002 + T003 + T004 (domain entities) — all different files
- T005 + T006 (migrations) — all different files
- T008 + T009 (repositories) — different files
- T010 + T011 + T012 (state, encryption, config) — different files
- US1 + US4 (Phase 3 + Phase 4) — different handler/service files, can run in parallel
- T033 through T037 (polish tasks) — all different files

---

## Parallel Example: Phase 1 + Phase 2

```text
# Phase 1 (all can run in parallel):
Task T001: Create config/oauth.go
Task T002: Create domain/oauth_provider.go
Task T003: Create domain/oauth_account.go
Task T004: Create domain/oauth_events.go

# Phase 2 (after Phase 1, some can run in parallel):
Task T005: Create migration 000027        (parallel with T006)
Task T006: Create migration 000028        (parallel with T005)
Task T008: Create repository/oauth_provider.go   (parallel with T009)
Task T009: Create repository/oauth_account.go   (parallel with T008)
Task T010: Create service/oauth_state.go         (parallel with T011, T012, T013)
Task T011: Create service/oauth_encryption.go    (parallel with T010, T012, T013)
Task T012: Create config/oauth.go (env vars)     (parallel with T010, T011, T013)
Task T013: Create ProviderProfile normalization  (parallel with T010, T011, T012)
Task T014: Wire in cmd/api/main.go               (after T008-T013)
```

## Parallel Example: User Stories 1 + 4

```text
# These can run in parallel since they touch different files:
Task T015-T019: US1 (oauth_login.go, handler/oauth_login.go)
Task T020-T023: US4 (oauth_provider.go service, handler/oauth_provider.go)
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 4 Only)

1. Complete Phase 1: Setup (T001–T007)
2. Complete Phase 2: Foundational (T008–T014)
3. Complete Phase 3: US1 — Sign In (T015–T019)
4. Complete Phase 4: US4 — Admin CRUD (T020–T023)
5. **STOP and VALIDATE**: Test social login end-to-end + admin provider management
6. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 (Sign In) → Test end-to-end OAuth login → Deploy (MVP!)
3. Add US4 (Admin) → Test admin CRUD → Deploy
4. Add US2 (Link) → Test linking flow → Deploy
5. Add US3 (Unlink) → Test unlinking + last-auth protection → Deploy
6. Add US5 (View Accounts) → Test account listing → Deploy
7. Add Polish (Phase 8) → Full production readiness

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (T001–T014)
2. Once Foundational is done:
   - Developer A: US1 (T015–T019) — login flow
   - Developer B: US4 (T020–T023) — admin CRUD
3. Then sequentially:
   - Developer A: US2 (T024–T027) — link flow
   - Developer B: US5 (T031–T032) — view accounts
4. Then:
   - Developer A: US3 (T028–T030) — unlink flow
   - Developer B: Polish (T033–T040)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- US1 and US4 can be developed in parallel (different handler/service files, shared foundation)
- US2 depends on US1's callback handler (shared GET /callback endpoint)
- The requires_email_update field on User entity may need a separate migration — check if users table already has it or add it to migration 000027
- Provider names are strings ("google", "github", "microsoft") matching event type pattern — NOT Go enums/iota
- Client secrets are NEVER returned in API responses — OAuthProviderResponse strips client_secret_encrypted