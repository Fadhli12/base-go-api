# Feature Specification: OAuth 2.0 Social Login

**Feature Branch**: `021-oauth-social-login`
**Created**: 2026-05-18
**Status**: Draft
**Input**: User description: "Implement OAuth 2.0 Social Login — Sign in with Google/GitHub/Microsoft, plus OAuth account linking/unlinking for existing users. Must follow existing project patterns (soft delete, UUID PKs, Casbin RBAC, context propagation, audit logging, structured logging, EventBus integration). Supports organization scoping."

## User Scenarios & Testing

### User Story 1 - Sign In with Social Provider (Priority: P1)

A new or returning user visits the login page and clicks "Sign in with Google" (or GitHub/Microsoft). If they have an existing linked account, they are authenticated and receive JWT tokens. If they are a new user, an account is automatically created with their profile information from the social provider and they receive JWT tokens.

**Why this priority**: This is the core value proposition — every SaaS user in 2026 expects social login. Without this, the feature does not exist.

**Independent Test**: Can be fully tested by initiating an OAuth flow with a configured provider, completing the provider consent screen, and verifying that the user receives valid JWT access and refresh tokens. Delivers authentication value immediately.

**Acceptance Scenarios**:

1. **Given** a configured Google OAuth provider, **When** a user clicks "Sign in with Google" and completes Google's consent screen, **Then** the system creates or finds the user account and redirects the user to the frontend callback URL with JWT tokens in the URL fragment (`#access_token=...&refresh_token=...`).
2. **Given** a configured Google OAuth provider and an existing linked user, **When** the user signs in via Google again, **Then** the system authenticates the existing user without creating a duplicate and redirects to the frontend callback URL with JWT tokens in the URL fragment.
3. **Given** no configured OAuth provider, **When** a user attempts to initiate an OAuth flow, **Then** the system returns a 404 error indicating the provider is not available.
4. **Given** a user denies consent on the provider's consent screen, **When** the callback is received with an error parameter, **Then** the system redirects to the frontend login page with the error in the URL fragment (e.g., `#error=access_denied&message=User+denied+access`).

---

### User Story 2 - Link an OAuth Provider to Existing Account (Priority: P2)

An authenticated user links a social account (e.g., GitHub) to their existing account via the account settings page. After linking, they can sign in using either their password or the linked social provider.

**Why this priority**: Account linking enables existing users to adopt social login without losing their history, settings, or data. It's essential for migration from password-only auth.

**Independent Test**: Can be fully tested by authenticating with password, calling the link endpoint with a valid provider authorization code, and verifying the social account appears in the user's linked accounts list. Delivers linking value immediately.

**Acceptance Scenarios**:

1. **Given** an authenticated user with no linked GitHub account, **When** the user authorizes GitHub via the link endpoint, **Then** the system creates an OAuth account record linking the GitHub identity to the user and returns a 200 response.
2. **Given** a user who already has a GitHub account linked, **When** they attempt to link GitHub again, **Then** the system returns a 409 Conflict indicating the account is already linked.
3. **Given** a GitHub account that is linked to a different user, **When** another user tries to link the same GitHub account, **Then** the system returns a 409 Conflict indicating the account belongs to another user.
4. **Given** an authenticated user linking a provider, **When** the link succeeds, **Then** an `auth.oauth.linked` event is published to the EventBus.

---

### User Story 3 - Unlink an OAuth Provider from Account (Priority: P2)

An authenticated user removes a linked social account from their profile. After unlinking, they can no longer sign in with that provider.

**Why this priority**: Users must be able to undo a link (privacy/autonomy). Without unlink, linking is a one-way door.

**Independent Test**: Can be fully tested by linking then immediately unlinking a provider and verifying the linked accounts list no longer shows it. Also verifies password requirement enforcement.

**Acceptance Scenarios**:

1. **Given** an authenticated user with a linked Google account and a password set, **When** the user unlinks Google, **Then** the system soft-deletes the OAuth account record and returns a 200 response.
2. **Given** an authenticated user with ONLY a linked Google account (no password), **When** the user attempts to unlink Google, **Then** the system returns a 400 error indicating at least one authentication method must remain.
3. **Given** a user who unlinks a provider, **When** the unlink succeeds, **Then** an `auth.oauth.unlinked` event is published to the EventBus.

---

### User Story 4 - Manage OAuth Providers (Admin) (Priority: P1)

An administrator configures which OAuth providers are available (Google, GitHub, Microsoft) by creating, updating, or disabling provider configurations. Only enabled providers appear in the available providers list and accept authentication attempts.

**Why this priority**: Without provider configuration, no social login is possible. Admin must be able to enable/disable providers (e.g., disable Microsoft if the Azure app is misconfigured).

**Independent Test**: Can be fully tested by creating a Google provider config via the admin endpoint, verifying it appears in the public providers list, then disabling it and verifying it disappears. Delivers admin control value immediately.

**Acceptance Scenarios**:

1. **Given** an admin with `oauth:manage` permission, **When** they create a Google OAuth provider with client ID, client secret, and redirect URL, **Then** the system stores the provider configuration with an encrypted secret and returns 201.
2. **Given** an admin, **When** they disable a provider, **Then** the provider no longer appears in the public providers list and all login attempts with that provider return 404.
3. **Given** an admin, **When** they update a provider's client secret, **Then** the system re-encrypts the secret and subsequent login attempts use the new credentials.

---

### User Story 5 - View Linked Accounts (Priority: P3)

An authenticated user views which social accounts are linked to their profile, seeing the provider name, linked email, and when the link was established.

**Why this priority**: Transparency — users need to see what's connected to manage their account security. Lower priority because linking/unlinking already provides feedback.

**Independent Test**: Can be fully tested by linking multiple providers and calling the list endpoint to verify all linked accounts are returned with correct details.

**Acceptance Scenarios**:

1. **Given** an authenticated user with 2 linked social accounts, **When** they request their linked accounts, **Then** the system returns a list containing both providers with provider name, email, and linked date.
2. **Given** an authenticated user with no linked social accounts, **When** they request their linked accounts, **Then** the system returns an empty list.

---

### Edge Cases

- What happens when the OAuth state parameter expires or is replayed? → State is stored as `oauth:state:{nonce}` in Redis with 10-minute TTL and `{callback_url, provider, intent, user_id, code_verifier, org_id, created_at}`. Expired/invalid state redirects to frontend with `#error=invalid_state`. PKCE code_verifier is verified during token exchange.
- How does the system handle a provider's email that matches an existing user's email but is not linked? → Does NOT auto-link (security-first). Redirects to frontend with `#error=email_already_exists&message=An account with this email already exists. Please log in with your existing account and link this provider in settings.`
- What happens when a user's social token is revoked on the provider side after linking? → Tokens are not stored long-term; only the provider ID is used for identification. Next login re-authenticates with the provider.
- What happens when an admin deletes a provider that users have linked accounts for? → Existing linked accounts remain functional until the provider configuration is fully deleted (soft delete). If provider is disabled, login attempts return 404.
- How does the system handle rate limits on the OAuth callback endpoint? → Rate limiting applies per the existing rate limiter; no special OAuth rate limiting is needed.
- What happens when two users have the same email at a social provider? → Provider ID + email combination is unique per provider. Each provider ID maps to exactly one user account.
- What happens when a user signs up via social but the provider doesn't return an email? → If email is unavailable from the provider, the system creates the user with a placeholder email (e.g., `oauth-{uuid}@social.placeholder`) and sets `requires_email_update=true` on the User entity. On subsequent API requests, the response includes a `X-Requires-Email-Update: true` response header and an `email_update_required` field in the user response DTO, prompting the frontend to show an email collection form.
- What happens when a provider returns an unverified email? → The system does NOT auto-create an account with an unverified email. It redirects to the frontend with `#error=email_not_verified&message=Please verify your email with {provider} first.`

## Clarifications

### Session 2026-05-18

- Q: After the OAuth callback processes the authorization code and creates/authenticates the user, how should the system deliver the JWT tokens to the client? → A: Redirect to frontend URL with tokens in URL fragment (SPA-friendly, RFC 9269 compliant). The callback endpoint redirects to a configurable frontend URL using `#access_token=...&refresh_token=...` (fragment, NOT query parameters). Fragments are not sent in Referer headers or server logs. The frontend parses `window.location.hash` and removes it after storage. Error flows also redirect to the frontend using fragments (e.g., `#error=email_already_exists`). Only the POST link/unlink endpoints return JSON.
- Q: How should OAuth scopes be managed for each provider? → A: Hardcoded sensible defaults per provider (Google: `openid email profile`, GitHub: `read:user user:email`, Microsoft: `openid email profile`). Admins can only add extra scopes, not remove defaults.
- Q: When a new user signs in via a social provider for the first time, what default state should the auto-created user account have? → A: User is created as active with the default/basic role (same as password registration) and all available social profile data (email, name, avatar) — user can immediately use the system.

### Metis Pre-Planning Review (2026-05-18)

Key findings incorporated into the spec:

- **Fragment encoding over query params**: Tokens MUST be delivered in URL fragments (`#access_token=...`), NOT query parameters (`?access_token=...`). Fragments are not sent in Referer headers or server logs (RFC 9269 / BCP 212).
- **PKCE (Proof Key for Code Exchange)**: All OAuth flows MUST implement S256 PKCE. The `code_verifier` is stored in the Redis state entry alongside the nonce.
- **Shared callback handler**: The GET callback endpoint is shared between login and link flows, distinguished by the `intent` field in Redis state.
- **Link endpoint returns JSON**: The POST /link endpoint returns `{redirect_url: "..."}` to initiate the link flow, NOT a redirect.
- **Encryption key derivation**: Client secrets encrypted with AES-256-GCM using a key derived from JWT_SECRET via HKDF-SHA256 (info: `oauth-encryption-v1`). Override via `OAUTH_ENCRYPTION_KEY` env var.
- **SSRF protection on outbound**: All token exchange and profile fetch HTTP calls MUST use `ssrf.NewClient()`.
- **Callback URL allowlist**: The `callback_url` in Redis state MUST be validated against `OAUTH_FRONTEND_CALLBACK_URL` before redirecting.
- **Provider profile normalization**: Each provider returns a unified `ProviderProfile{ProviderID, Email, EmailVerified, DisplayName, AvatarURL}`.
- **Unverified emails**: Do NOT auto-create accounts with unverified emails. Return `#error=email_not_verified`.
- **RefreshToken on social login**: Social login MUST create a RefreshToken entity in DB (consistent with password login).
- **Microsoft tenant_id**: Configurable per provider (default: `"common"`).

## Requirements

### Functional Requirements

- **FR-001**: System MUST support OAuth 2.0 authorization code flow for Google, GitHub, and Microsoft identity providers, with hardcoded default scopes per provider (Google: `openid email profile`, GitHub: `read:user user:email`, Microsoft: `openid email profile`). Additional scopes may be configured per provider but defaults are always included and cannot be removed.
- **FR-002**: System MUST provide a redirect endpoint that initiates the OAuth flow by redirecting the user to the provider's authorization URL with a cryptographically secure state parameter and PKCE code_verifier. The state parameter is a nonce that maps to a Redis entry containing `{callback_url, provider, intent, user_id, code_verifier, org_id, created_at}`. The PKCE code_challenge (S256) is sent in the authorization URL; the code_verifier is stored in Redis for later verification.
- **FR-003**: System MUST provide a shared callback endpoint (GET) that exchanges the authorization code for an access token (using PKCE code_verifier), retrieves the user's profile from the provider, and based on the `intent` stored in Redis state: for `login` — creates or authenticates a user account and redirects to the frontend with JWT access and refresh tokens in the URL fragment (`#access_token=...&refresh_token=...`); for `link` — links the provider to the authenticated user and redirects to the frontend with a success message in the URL fragment. Errors also redirect to the frontend using fragments (e.g., `#error=email_already_exists`).
- **FR-004**: System MUST automatically create a new user account when a previously unseen social identity signs in, using available profile data (email, name, avatar) from the provider. The auto-created user is set to active status with the default/basic role (identical to password registration), enabling immediate system access.
- **FR-005**: System MUST authenticate an existing user when a previously linked social identity signs in, redirecting to the frontend with JWT access and refresh tokens in the URL fragment (consistent with FR-003).
- **FR-006**: System MUST allow authenticated users to link an additional OAuth provider to their account. The link endpoint (POST) initiates a new OAuth flow by returning a JSON response containing `{redirect_url: "..."}` with the provider's authorization URL. The shared callback handler then completes the link based on the `intent=link` stored in Redis state.
- **FR-007**: System MUST prevent linking a social identity that is already linked to a different user account (return 409 Conflict).
- **FR-008**: System MUST allow authenticated users to unlink an OAuth provider from their account, unless it is their only authentication method and they have no password set (return 400).
- **FR-009**: System MUST allow administrators with `oauth:manage` permission to create, update, enable/disable, and soft-delete OAuth provider configurations.
- **FR-010**: System MUST store OAuth provider client secrets in encrypted form (never plaintext).
- **FR-011**: System MUST validate the OAuth state parameter on callback to prevent CSRF attacks, using a nonce stored as a Redis key `oauth:state:{nonce}` with a JSON value containing `{callback_url, provider, intent, user_id, code_verifier, org_id, created_at}` and a 10-minute TTL. System MUST also implement PKCE (Proof Key for Code Exchange) using S256 code challenge method for all OAuth flows, storing the code_verifier in the same Redis entry.
- **FR-012**: System MUST publish `auth.oauth.linked` and `auth.oauth.unlinked` events to the EventBus when accounts are linked or unlinked.
- **FR-013**: System MUST log all OAuth provider configuration changes and link/unlink actions in the audit log, following the existing `audit_logs` table pattern with before/after JSONB state capture. Specifically:
  - **Provider CRUD**: `oauth_provider.created`, `oauth_provider.updated`, `oauth_provider.deleted` — capturing provider name, enabled status, and config changes (client_secret excluded from after-state).
  - **Account link/unlink**: `oauth_account.linked`, `oauth_account.unlinked` — capturing provider name, user ID, and provider user ID.
  - **Social login**: `auth.oauth.login` — capturing provider name and user ID (no tokens in audit payload).
- **FR-014**: System MUST provide a public endpoint to list enabled OAuth providers (provider name and display name only, no secrets).
- **FR-015**: System MUST enforce organization scoping on OAuth provider configurations (providers can be global or org-specific).
- **FR-016**: System MUST generate JWT access and refresh tokens upon successful social login, following the existing authentication token pattern (15m access, 30d refresh).
- **FR-017**: System MUST record the user's last login timestamp and authentication method (e.g., "google", "github") upon each social login.
- **FR-018**: System MUST enforce Casbin RBAC permissions: `oauth:manage` for admin provider configuration, `oauth:link` for link/unlink operations, `oauth:view` for listing linked accounts.
- **FR-019**: System MUST support soft delete for OAuth provider configurations and OAuth account records (deleted_at field).
- **FR-020**: System MUST propagate context (request ID, user ID, org ID) through all OAuth operations for structured logging.
- **FR-021**: System MUST redirect to the frontend callback URL with an error fragment (e.g., `#error=access_denied&message=...`) when the OAuth flow fails (user denies consent, invalid state, provider error, email already exists). The GET callback endpoint MUST NEVER return JSON — all responses are 302 redirects. Only POST endpoints (link/unlink) return JSON.
- **FR-022**: System MUST encrypt OAuth client secrets at rest using AES-256-GCM with a key derived from the JWT secret via HKDF-SHA256 (info string: `oauth-encryption-v1`). The encrypted format MUST be `v1:` + base64(IV[12 bytes] + ciphertext + GCM_tag[16 bytes]). An optional `OAUTH_ENCRYPTION_KEY` env var overrides the derived key for key rotation scenarios.
- **FR-023**: System MUST use the SSRF-safe HTTP client (`internal/ssrf`) for ALL outbound HTTP requests to OAuth providers (token exchange, profile fetch), preventing SSRF attacks via provider-redirected URLs.
- **FR-024**: System MUST validate the `callback_url` stored in Redis state against an allowlist (the `OAUTH_FRONTEND_CALLBACK_URL` env var) before redirecting, preventing open redirect vulnerabilities.
- **FR-025**: System MUST normalize provider-specific profile data into a unified `ProviderProfile` struct containing: `ProviderID` (string, not UUID), `Email`, `EmailVerified` (bool), `DisplayName`, `AvatarURL`. System MUST NOT auto-create accounts with unverified emails — instead redirect with `#error=email_not_verified`.
- **FR-026**: System MUST create a RefreshToken entity in the database for social login (consistent with the existing password login pattern), including session metadata (DeviceName derived from provider name, UserAgent, IPAddress).
- **FR-027**: System MUST support a configurable `tenant_id` field for Microsoft OAuth providers (defaulting to `"common"`), injected into the Microsoft authorization and token URLs.

### Key Entities

- **OAuthProvider**: Represents a configured identity provider (Google, GitHub, Microsoft). Key attributes: provider name (string: `"google"`, `"github"`, `"microsoft"` — NOT enum/iota), client ID, encrypted client secret (AES-256-GCM with HKDF-derived key, format: `v1:` + base64(IV + ciphertext + GCM_tag)), redirect URL, additional scopes (appended to hardcoded defaults), enabled/disabled status, organization scope, config JSON column (for provider-specific settings like Microsoft `tenant_id`). Supports soft delete (disabled providers block new OAuth flows but existing linked logins continue).
- **OAuthAccount**: Represents the association between a user and their social identity on a specific provider. Key attributes: user ID (FK to users), provider ID (FK to oauth_providers), provider user ID (string — GitHub uses integer IDs, Google uses string IDs), email, email_verified (bool from provider), display name, avatar URL, linked timestamp. Each user can have multiple OAuthAccounts (one per provider). Supports soft delete with unique constraint: `(provider_id, provider_user_id) WHERE deleted_at IS NULL`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can complete a social login flow (click "Sign in with Google" → authenticate → receive tokens) in under 10 seconds for the callback portion (excluding user interaction time on the provider consent screen). Tokens are delivered in the URL fragment (`#access_token=...`), NOT query parameters.
- **SC-002**: System correctly handles all three supported providers (Google, GitHub, Microsoft) without provider-specific bugs or regressions.
- **SC-003**: 100% of OAuth callback attempts with invalid or expired state parameters are rejected with appropriate error responses (no unauthorized access).
- **SC-004**: Users can link and unlink providers without losing their account data or authentication ability (given at least one other auth method remains).
- **SC-005**: Admin configuration changes (create, update, enable/disable, delete) take effect within 1 second, reflected in the public providers list.
- **SC-006**: Audit log entries are created for every OAuth provider configuration change and every link/unlink action, with before/after state captured.

## Assumptions

- Users have stable internet connectivity for OAuth redirect flows.
- The OAuth callback endpoint uses URL fragment encoding (`#access_token=...&refresh_token=...`) for token delivery (RFC 9269 compliant). Fragments are not sent in Referer headers or server logs. The frontend parses `window.location.hash` and removes it after storage. Error flows also use fragments.
- The frontend callback URL is configurable per provider (via `redirect_url` in the OAuthProvider entity) and/or via a configurable default in the application config (environment variable `OAUTH_FRONTEND_CALLBACK_URL`). The stored callback_url is validated against this allowlist before redirecting.
- The shared callback handler distinguishes between `login` and `link` flows via the `intent` field stored in Redis state. The POST /link endpoint returns JSON `{redirect_url: "..."}` to initiate the link flow.
- The existing JWT authentication system (access tokens 15m, refresh tokens 30d) is the source of truth for token generation and will be reused for social login responses. Social login creates a RefreshToken entity in the database (consistent with password login).
- OAuth provider client IDs and secrets are managed via environment variables or admin API, not hardcoded.
- The existing EventBus pattern (Go channels with 256-buffer subscriptions, `SetEventBus()` setter injection) will be followed for `auth.oauth.linked` and `auth.oauth.unlinked` events.
- The existing Casbin RBAC pattern (handler-level permission enforcement, `enforcer.Enforce(userID, orgDomain, resource, action)`) will be followed for `oauth:manage`, `oauth:link`, and `oauth:view`.
- Client secrets are encrypted at rest using AES-256-GCM with a key derived from JWT_SECRET via HKDF-SHA256 (info string: `oauth-encryption-v1`). Format: `v1:` + base64(IV + ciphertext + GCM_tag). An `OAUTH_ENCRYPTION_KEY` env var overrides the derived key for rotation.
- The state parameter for CSRF protection is stored as Redis key `oauth:state:{nonce}` with JSON value containing `{callback_url, provider, intent, user_id, code_verifier, org_id, created_at}` and 10-minute TTL. PKCE (S256) is implemented for all flows.
- Auto-account-creation is enabled by default; auto-link-by-email is disabled by default for security.
- Provider configurations can be global (available to all organizations) or org-specific, following the existing organization scoping pattern (X-Organization-ID header, organization_id UUID).
- Provider names use strings (`"google"`, `"github"`, `"microsoft"`) matching the existing event type pattern, NOT Go enums/iota.
- All new entities follow the existing soft delete pattern (gorm.DeletedAt field with partial unique indexes).
- All new entities use UUID primary keys (gen_random_uuid()), except `provider_user_id` which is a string (GitHub uses integer IDs).
- When a social provider doesn't return an email, the system creates the user with a placeholder email (e.g., `oauth-{uuid}@social.placeholder`) and sets `requires_email_update=true` on the User entity. On subsequent API requests, the response includes a `X-Requires-Email-Update: true` response header and an `email_update_required` field in the user response DTO, prompting the frontend to show an email collection form.
- When a social provider returns an unverified email, the system does NOT auto-create an account. Instead, it redirects to the frontend with `#error=email_not_verified`.
- When a social provider email matches an existing user's email but is not linked, the system does NOT auto-link (security-first). It redirects with `#error=email_already_exists&message=An account with this email already exists. Please log in with your existing account and link this provider in settings.`
- Microsoft OAuth requires a `tenant_id` in the authorization and token URLs. Default: `"common"`. Configurable via the provider's `config` JSON column.
- Soft-deleted/disabled providers block new OAuth flows (login + link) but existing linked accounts continue to work for authentication.
- The StructuredLogging middleware MUST NOT log redirect Location headers that contain tokens in the URL fragment.

## Endpoints

### Public Endpoints (No Authentication Required)

| Method | Path | Description | Response |
|--------|------|-------------|----------|
| GET | `/api/v1/auth/oauth/:provider` | Initiate OAuth login flow | 302 redirect to provider authorization URL |
| GET | `/api/v1/auth/oauth/:provider/callback` | OAuth callback (shared: login + link) | 302 redirect to frontend with tokens in fragment or error in fragment |
| GET | `/api/v1/auth/oauth/providers` | List enabled OAuth providers | 200 JSON `{data: [{name, display_name, scopes}]}` |

### Authenticated Endpoints (JWT Required)

| Method | Path | Permission | Description | Response |
|--------|------|------------|-------------|----------|
| POST | `/api/v1/auth/oauth/:provider/link` | `oauth:link` | Initiate provider link flow | 200 JSON `{redirect_url: "https://provider.auth/..."}` |
| DELETE | `/api/v1/auth/oauth/:provider/unlink` | `oauth:link` | Unlink provider from account | 200 JSON `{data: {message: "Provider unlinked"}}` |
| GET | `/api/v1/auth/oauth/accounts` | `oauth:view` | List current user's linked accounts | 200 JSON `{data: [{provider, email, display_name, avatar_url, linked_at}]}` |

### Admin Endpoints (JWT + `oauth:manage` Permission Required)

| Method | Path | Description | Response |
|--------|------|-------------|----------|
| POST | `/api/v1/oauth-providers` | Create provider config | 201 JSON `{data: ProviderResponse}` |
| GET | `/api/v1/oauth-providers` | List all providers (including disabled) | 200 JSON `{data: [ProviderResponse], meta: {total, page, per_page}}` |
| GET | `/api/v1/oauth-providers/:id` | Get provider by ID | 200 JSON `{data: ProviderResponse}` |
| PUT | `/api/v1/oauth-providers/:id` | Update provider config | 200 JSON `{data: ProviderResponse}` |
| DELETE | `/api/v1/oauth-providers/:id` | Soft-delete provider config | 200 JSON `{data: {message: "Provider deleted"}}` |

### Callback Fragment Format

**Success (login)**:
```
#access_token={jwt}&refresh_token={hex}&token_type=Bearer&expires_in=900
```

**Success (link)**:
```
#status=linked&provider=google&message=Provider+linked+successfully
```

**Error**:
```
#error={error_code}&message={human_readable_message}
```

Error codes: `invalid_state`, `access_denied`, `provider_error`, `email_already_exists`, `email_not_verified`, `provider_already_linked`, `account_already_linked`, `provider_disabled`, `internal_error`

### Redis State Schema

Key: `oauth:state:{nonce}` (random 32-byte hex string)
TTL: 600 seconds (10 minutes)
Value (JSON):
```json
{
  "callback_url": "https://app.example.com/auth/callback",
  "provider": "google",
  "intent": "login",
  "user_id": "00000000-0000-0000-0000-000000000000",
  "code_verifier": "dBjftJeZ4CVP-mB92K27uhbUerw...",
  "org_id": "00000000-0000-0000-0000-000000000000",
  "created_at": "2026-05-18T10:00:00Z"
}
```
- `user_id` is `uuid.Nil` for login flows; actual user UUID for link flows.
- `org_id` is extracted from `X-Organization-ID` header (or `uuid.Nil` for global).
- `code_verifier` is the PKCE S256 verifier used for the authorization request.

## Database Migrations

### Migration: `migrations/000027_oauth_providers.up.sql`

```sql
CREATE TABLE oauth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    client_id VARCHAR(500) NOT NULL,
    client_secret_encrypted TEXT NOT NULL,
    redirect_url VARCHAR(500) NOT NULL,
    additional_scopes TEXT[] DEFAULT '{}',
    config JSONB DEFAULT '{}',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    is_system BOOLEAN NOT NULL DEFAULT false,
    organization_id UUID REFERENCES organizations(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT uq_oauth_providers_name UNIQUE (name) WHERE deleted_at IS NULL,
    CONSTRAINT uq_oauth_providers_name_org UNIQUE (name, organization_id) WHERE deleted_at IS NULL
);

CREATE INDEX idx_oauth_providers_org_id ON oauth_providers(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_oauth_providers_enabled ON oauth_providers(is_enabled) WHERE deleted_at IS NULL;

COMMENT ON TABLE oauth_providers IS 'OAuth 2.0 identity provider configurations';
COMMENT ON COLUMN oauth_providers.client_secret_encrypted IS 'AES-256-GCM encrypted client secret. Format: v1:base64(IV[12]+ciphertext+GCM_tag[16])';
COMMENT ON COLUMN oauth_providers.additional_scopes IS 'Extra OAuth scopes appended to provider defaults. Defaults are hardcoded and cannot be removed.';
COMMENT ON COLUMN oauth_providers.config IS 'Provider-specific configuration JSON. E.g., {"tenant_id": "common"} for Microsoft.';
```

### Migration: `migrations/000028_oauth_accounts.up.sql`

```sql
CREATE TABLE oauth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    provider_id UUID NOT NULL REFERENCES oauth_providers(id),
    provider_user_id VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    email_verified BOOLEAN DEFAULT false,
    display_name VARCHAR(255),
    avatar_url VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT uq_oauth_accounts_provider_user UNIQUE (provider_id, provider_user_id) WHERE deleted_at IS NULL,
    CONSTRAINT uq_oauth_accounts_user_provider UNIQUE (user_id, provider_id) WHERE deleted_at IS NULL
);

CREATE INDEX idx_oauth_accounts_user_id ON oauth_accounts(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_oauth_accounts_provider_id ON oauth_accounts(provider_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_oauth_accounts_email ON oauth_accounts(email) WHERE deleted_at IS NULL;

COMMENT ON TABLE oauth_accounts IS 'Links between users and their OAuth social identities';
COMMENT ON COLUMN oauth_accounts.provider_user_id IS 'Unique user ID from the OAuth provider. String type because GitHub uses integer IDs.';
COMMENT ON COLUMN oauth_accounts.email_verified IS 'Whether the provider confirmed email ownership';
```

### Migration: `migrations/000027_oauth_providers.down.sql`

```sql
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS oauth_providers;
```

### Seed Data (cmd/api/main.go)

```go
// OAuth permissions
{oauthPermissions: []struct{Name, Resource, Action, Scope, Description, IsSystem}{
    {"oauth:view", "oauth", "view", "own", "View OAuth providers and linked accounts", false},
    {"oauth:link", "oauth", "link", "own", "Link and unlink OAuth providers", false},
    {"oauth:manage", "oauth", "manage", "all", "Manage OAuth provider configurations", false},
}}
```

## Request/Response DTOs

### Admin CRUD

```go
// CreateProviderRequest
type CreateOAuthProviderRequest struct {
    Name             string   `json:"name" validate:"required,oneof=google github microsoft"`
    DisplayName      string   `json:"display_name" validate:"required,max=100"`
    ClientID         string   `json:"client_id" validate:"required,max=500"`
    ClientSecret     string   `json:"client_secret" validate:"required,min=16,max=500"`
    RedirectURL      string   `json:"redirect_url" validate:"required,url,max=500"`
    AdditionalScopes []string `json:"additional_scopes" validate:"max=10,dive,max=50"`
    Config           *json.RawMessage `json:"config,omitempty" validate:"omitempty"`
    OrganizationID   *uuid.UUID `json:"organization_id,omitempty"`
}

// UpdateProviderRequest
type UpdateOAuthProviderRequest struct {
    DisplayName      *string   `json:"display_name,omitempty" validate:"omitempty,max=100"`
    ClientID         *string   `json:"client_id,omitempty" validate:"omitempty,max=500"`
    ClientSecret     *string   `json:"client_secret,omitempty" validate:"omitempty,min=16,max=500"`
    RedirectURL      *string   `json:"redirect_url,omitempty" validate:"omitempty,url,max=500"`
    AdditionalScopes *[]string `json:"additional_scopes,omitempty" validate:"omitempty,max=10,dive,max=50"`
    Config           *json.RawMessage `json:"config,omitempty" validate:"omitempty"`
    IsEnabled        *bool     `json:"is_enabled,omitempty"`
}

// ProviderResponse
type OAuthProviderResponse struct {
    ID               uuid.UUID  `json:"id"`
    Name             string     `json:"name"`
    DisplayName      string     `json:"display_name"`
    ClientID         string     `json:"client_id"`
    RedirectURL      string     `json:"redirect_url"`
    Scopes           []string   `json:"scopes"`           // merged defaults + additional
    AdditionalScopes []string   `json:"additional_scopes"`
    Config           *json.RawMessage `json:"config,omitempty"`
    IsEnabled        bool       `json:"is_enabled"`
    IsSystem         bool       `json:"is_system"`
    OrganizationID   *uuid.UUID `json:"organization_id,omitempty"`
    CreatedAt        time.Time  `json:"created_at"`
    UpdatedAt        time.Time  `json:"updated_at"`
}
```

### Authenticated User (Link/Unlink/Accounts)

```go
// LinkResponse (returned by POST /link — NOT a redirect)
type OAuthLinkResponse struct {
    RedirectURL string `json:"redirect_url"` // Provider authorization URL
}

// UnlinkRequest
type OAuthUnlinkProviderRequest struct {
    Provider string `json:"provider" validate:"required,oneof=google github microsoft"`
}

// OAuthAccountResponse
type OAuthAccountResponse struct {
    ID          uuid.UUID  `json:"id"`
    Provider    string     `json:"provider"`     // "google", "github", "microsoft"
    Email       string     `json:"email"`
    DisplayName string     `json:"display_name"`
    AvatarURL   string     `json:"avatar_url"`
    LinkedAt     time.Time  `json:"linked_at"`    // created_at
}

// LinkedAccountsResponse
type LinkedAccountsResponse struct {
    Accounts []OAuthAccountResponse `json:"accounts"`
}
```

### Public (Provider List)

```go
// PublicProviderResponse (no secrets)
type PublicOAuthProviderResponse struct {
    Name        string   `json:"name"`          // "google"
    DisplayName string   `json:"display_name"`  // "Google"
    Scopes      []string `json:"scopes"`        // merged defaults + additional
}
```

### Redis State (Internal)

```go
// OAuthState (stored in Redis, not exposed via API)
type OAuthState struct {
    CallbackURL   string    `json:"callback_url"`
    Provider      string    `json:"provider"`
    Intent        string    `json:"intent"`        // "login" or "link"
    UserID        uuid.UUID `json:"user_id"`       // uuid.Nil for login
    CodeVerifier  string    `json:"code_verifier"` // PKCE S256
    OrgID         uuid.UUID `json:"org_id"`
    CreatedAt     time.Time `json:"created_at"`
}
```

### Provider Profile (Internal)

```go
// ProviderProfile (normalized from all providers)
type ProviderProfile struct {
    ProviderID    string `json:"provider_id"`    // Unique ID from provider
    Email         string `json:"email"`
    EmailVerified bool   `json:"email_verified"`
    DisplayName   string `json:"display_name"`
    AvatarURL     string `json:"avatar_url"`
}
```