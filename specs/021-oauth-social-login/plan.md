# Implementation Plan: OAuth 2.0 Social Login

**Branch**: `021-oauth-social-login` | **Date**: 2026-05-18 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/021-oauth-social-login/spec.md`

## Summary

Implement OAuth 2.0 Social Login enabling users to sign in with Google, GitHub, or Microsoft, plus link/unlink OAuth accounts for existing users. The system uses PKCE (S256) for all flows, AES-256-GCM encryption for client secrets, Redis state for CSRF protection, and fragment-based token delivery (RFC 9269). Follows existing repository/service/handler patterns, EventBus integration for linked/unlinked events, and Casbin RBAC for permission enforcement.

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: Echo v4, GORM, Casbin, Redis (go-redis/v9), PostgreSQL, testcontainers-go, golang.org/x/oauth2  
**Storage**: PostgreSQL (oauth_providers, oauth_accounts), Redis (state management, rate limiting)  
**Testing**: testify (require/assert), testcontainers-go for integration, mockery for unit mocks  
**Target Platform**: Linux server (Docker)  
**Project Type**: Web service (REST API)  
**Performance Goals**: 1000 req/s sustained, <200ms p95 callback processing, async provider token exchange  
**Constraints**: 10-min state TTL, PKCE S256 mandatory, client secrets encrypted at rest, fragment-based token delivery  
**Scale/Scope**: Multi-tenant (organization-scoped providers), 3 providers at launch (Google, GitHub, Microsoft)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready Foundation | ✅ PASS | Structured logging, error handling, graceful shutdown, context propagation |
| II. RBAC Runtime-Configurable | ✅ PASS | `oauth:view`, `oauth:link`, `oauth:manage` permissions synced via `permission:sync` |
| III. Soft Deletes for Audit | ✅ PASS | `oauth_providers` has soft delete; `oauth_accounts` has soft delete; deletions audit-logged |
| IV. Stateless JWT + Revocation | ✅ PASS | No JWT changes; social login reuses existing TokenService (access 15m, refresh 30d) |
| V. SQL Migrations (Not AutoMigrate) | ✅ PASS | Migration 000027_oauth_providers + 000028_oauth_accounts with partial unique indexes |
| VI. Multi-Instance Consistency | ✅ PASS | Redis state uses atomic SET NX with TTL; client secret encryption key shared across instances |
| VII. Audit Logging Non-Negotiable | ✅ PASS | All OAuth provider CRUD, link/unlink actions, and social login events audit-logged with before/after JSONB |
| VIII. Organization-Scoped Multi-Tenancy | ✅ PASS | Providers can be global (organization_id=NULL) or org-scoped; X-Organization-ID middleware extracts org context |
| IX. Event-Driven Architecture | ✅ PASS | `auth.oauth.linked` and `auth.oauth.unlinked` events published via EventBus |
| X. Background Job Processing | ✅ PASS | No background workers needed for OAuth; token exchange is synchronous in callback handler |

## Project Structure

### Documentation (this feature)

```text
specs/021-oauth-social-login/
├── plan.md              # This file
├── spec.md              # Feature specification (already complete)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── api-v1.md        # REST API contracts
├── checklists/
│   └── requirements.md  # Quality checklist (already exists)
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── oauth_provider.go           # OAuthProvider entity, DTOs, business methods
│   ├── oauth_account.go            # OAuthAccount entity, DTOs, business methods
│   └── oauth_events.go            # EventBus event definitions (auth.oauth.linked/unlinked)
├── repository/
│   ├── oauth_provider.go           # OAuthProviderRepository interface + GORM impl
│   └── oauth_account.go           # OAuthAccountRepository interface + GORM impl
├── service/
│   ├── oauth_provider.go           # OAuthProviderService (CRUD, config management)
│   ├── oauth_login.go             # OAuthLoginService (login flow, callback, account creation)
│   ├── oauth_link.go              # OAuthLinkService (link/unlink flows)
│   ├── oauth_state.go             # OAuthStateManager (Redis state management, PKCE)
│   └── oauth_encryption.go        # Client secret encryption (HKDF-SHA256 → AES-256-GCM)
├── http/
│   ├── handler/
│   │   ├── oauth_provider.go       # Admin CRUD endpoints (5 endpoints)
│   │   ├── oauth_login.go          # Public auth endpoints (3 endpoints)
│   │   └── oauth_account.go        # Authenticated user endpoints (3 endpoints)
│   └── request/
│       └── oauth.go               # Request DTOs with validation
├── config/
│   └── oauth.go                    # OAuthConfig (env vars, provider defaults)
└── permission/
    └── (existing)                  # Add oauth:view, oauth:link, oauth:manage to manifest

migrations/
├── 000027_oauth_providers.up.sql   # Create oauth_providers table
├── 000027_oauth_providers.down.sql # Rollback
├── 000028_oauth_accounts.up.sql    # Create oauth_accounts table
└── 000028_oauth_accounts.down.sql  # Rollback

config/
└── permissions.yaml                # Add oauth:view, oauth:link, oauth:manage

tests/
├── integration/
│   ├── oauth_provider_test.go      # Provider CRUD integration tests
│   ├── oauth_login_test.go         # Login flow integration tests
│   └── oauth_account_test.go       # Link/unlink integration tests
└── unit/
    ├── oauth_provider_test.go       # Provider service unit tests
    ├── oauth_login_test.go          # Login service unit tests
    ├── oauth_link_test.go           # Link/unlink service unit tests
    ├── oauth_state_test.go          # State manager unit tests
    └── oauth_encryption_test.go     # Encryption/decryption unit tests
```

**Structure Decision**: Follows existing project layout — domain entities in `internal/domain/`, repositories in `internal/repository/`, services in `internal/service/`, handlers in `internal/http/handler/`, request DTOs in `internal/http/request/`. Migration files in `migrations/`. Splits OAuth into 4 service files for separation of concerns (provider management, login flow, link/unlink flow, state management).

## Complexity Tracking

> No constitution violations. All patterns follow established conventions.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none) | (N/A) | (N/A) |