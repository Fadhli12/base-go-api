# Research: OAuth 2.0 Social Login

**Date**: 2026-05-18 | **Branch**: `021-oauth-social-login` | **Spec**: [spec.md](./spec.md)

## Research Tasks

All NEEDS CLARIFICATION items from Technical Context have been resolved during the specification phase (Metis consultation + Momus review). This document consolidates the key decisions and their rationale.

---

## R1: Token Delivery Mechanism (RFC 9269 Compliance)

**Decision**: Use URL fragment encoding (`#access_token=...&refresh_token=...`) for token delivery on OAuth callback redirects.

**Rationale**: RFC 9269 explicitly recommends fragment encoding over query parameters because:
- Fragments are NOT sent in Referer headers (prevents token leakage to third parties)
- Fragments are NOT logged by server access logs (prevents token exposure in logs)
- Fragments are NOT cached by browsers or proxies (prevents token persistence)
- JavaScript can parse `window.location.hash` and remove it after storage

**Alternatives Considered**:
- **Query parameters** (`?access_token=...`): Simpler but tokens visible in logs, Referer headers, and browser history. Explicitly rejected for security.
- **POST form body**: Requires server-side rendering or JavaScript form submission. More complex for SPA frontends.
- **Session-based exchange**: Server stores token in session, frontend fetches via API. Adds session management complexity.

**Implementation**: Callback handler redirects to `callback_url#access_token={jwt}&refresh_token={hex}&token_type=Bearer&expires_in=900`. Error flows also use fragments: `callback_url#error={code}&message={msg}`.

---

## R2: CSRF Protection (State Parameter + PKCE)

**Decision**: Use Redis-stored state parameter with PKCE (S256) for all OAuth flows.

**Rationale**: PKCE (Proof Key for Code Exchange, RFC 7636) was originally designed for native/mobile apps but is now recommended for all OAuth clients (including web apps) as of OAuth 2.1. Combined with the state parameter:
- **State parameter** prevents CSRF attacks by verifying the callback matches an initiated request
- **PKCE code_verifier** prevents authorization code interception attacks even if the code is leaked
- **Redis storage** enables stateless server instances (multi-instance compatible) and automatic TTL-based expiry (10 minutes)

**Alternatives Considered**:
- **State parameter only**: Vulnerable to authorization code interception attacks. Rejected.
- **PKCE without state**: Vulnerable to CSRF attacks. Rejected.
- **In-memory state**: Not compatible with multi-instance deployment. Rejected.
- **Database-stored state**: Slower than Redis, creates write load on primary DB. Rejected in favor of Redis.

**Implementation**: Redis key `oauth:state:{nonce}` with JSON value `{callback_url, provider, intent, user_id, code_verifier, org_id, created_at}` and 600s TTL.

---

## R3: Client Secret Encryption

**Decision**: AES-256-GCM encryption with HKDF-SHA256 key derivation. Format: `v1:` + base64(IV[12] + ciphertext + GCM_tag[16]).

**Rationale**: Client secrets must be encrypted at rest to prevent credential exposure if the database is compromised. HKDF-SHA256 provides:
- **Key separation**: Using `oauth-encryption-v1` as info string ensures the derived key is specific to OAuth encryption, not reusable across other contexts
- **Key rotation**: `OAUTH_ENCRYPTION_KEY` env var allows key rotation without changing `JWT_SECRET`
- **Version prefix**: `v1:` prefix enables future algorithm migration without breaking existing encrypted values
- **GCM mode**: Provides both encryption and authentication (AEAD), preventing tampering with ciphertext

**Alternatives Considered**:
- **Plaintext storage**: Client secrets stored unencrypted in DB. Rejected — major security risk.
- **AES-CBC**: Requires separate HMAC for authentication. More complex, error-prone. Rejected.
- **NaCl SecretBox**: Excellent choice but adds dependency. AES-GCM is in stdlib. Rejected for simplicity.
- **KMS integration**: AWS/GCP KMS for key management. Overkill for current scale. Can be added later via `OAUTH_ENCRYPTION_KEY` rotation.

**Implementation**: `DeriveEncryptionKey(masterKey []byte)` uses HKDF with SHA256, info string `oauth-encryption-v1`, and 32-byte output. `Encrypt(plaintext, masterKey)` generates random IV, encrypts with AES-GCM, returns `v1:` + base64(IV + ciphertext + tag). `Decrypt(ciphertext, masterKey)` reverses the process.

---

## R4: Provider Profile Normalization

**Decision**: Normalize all provider responses (Google, GitHub, Microsoft) into a unified `ProviderProfile` struct.

**Rationale**: Each OAuth provider returns user info in different formats:
- **Google**: `sub`, `email`, `email_verified`, `name`, `picture`
- **GitHub**: `id` (integer), `email` (may be null), `login`, `avatar_url`
- **Microsoft**: `id`, `mail`/`userPrincipalEmail`, `displayName`, `photo`

Without normalization:
- Callback handler would have 3+ code paths with duplicated logic
- Adding a new provider requires modifying the callback handler
- Testing requires mocking multiple provider formats

With normalization:
- Single callback handler processes `ProviderProfile` regardless of provider
- Adding new providers requires only a `Normalize*Profile()` function
- Testing uses a single normalized struct

**Implementation**: Each provider has a `Normalize*Profile(raw map[string]interface{}) ProviderProfile` function in the login service. The `ProviderProfile` struct contains: `ProviderID string`, `Email string`, `EmailVerified bool`, `DisplayName string`, `AvatarURL string`.

---

## R5: Shared Callback Handler (Login + Link)

**Decision**: Single GET callback handler distinguished by `intent` field in Redis state.

**Rationale**: Both login and link flows follow identical OAuth flows (redirect to provider, receive callback with authorization code). The only difference is:
- **Login**: No user authenticated → create account or find existing → return JWT tokens
- **Link**: User authenticated → attach provider to existing user → return success

Using a shared handler:
- Eliminates code duplication (token exchange, state validation, PKCE verification are identical)
- Single CSRF state management point
- Single set of provider configurations and redirect URLs
- Reduced attack surface (one well-tested callback instead of two)

**Alternatives Considered**:
- **Separate callback handlers**: Would duplicate token exchange, state validation, and error handling. Rejected for DRY.
- **Query parameter routing** (`?intent=login`): Exposes internal state to URL parameters. Rejected for security.
- **POST-based link flow**: While the *initiation* of linking uses POST `/link`, the *callback* is always a GET redirect from the OAuth provider. This is standard OAuth.

**Implementation**: Login: `GET /api/v1/auth/oauth/:provider` → store state with `intent=login`. Link: `POST /api/v1/auth/oauth/:provider/link` → store state with `intent=link` and `user_id` → return JSON `{redirect_url: "..."}`. Callback: `GET /api/v1/auth/oauth/:provider/callback` → read state → route by `intent`.

---

## R6: Microsoft Tenant ID Support

**Decision**: Store `tenant_id` in provider `config` JSONB column. Default: `"common"`.

**Rationale**: Microsoft Graph API requires `tenant_id` in authorization and token URLs:
- Authorization URL: `https://login.microsoftonline.com/{tenant_id}/oauth2/v2.0/authorize`
- Token URL: `https://login.microsoftonline.com/{tenant_id}/oauth2/v2.0/token`

Different tenant configurations:
- `common` — Multi-tenant (any Azure AD org + personal Microsoft accounts)
- `organizations` — Any Azure AD org only
- `consumers` — Personal Microsoft accounts only
- Specific tenant GUID — Single Azure AD tenant (most secure)

Storing in JSONB allows per-provider configuration without schema changes.

**Implementation**: `config` JSONB column on `oauth_providers` table. Microsoft providers must include `{"tenant_id": "common"}`. The login service reads `config.tenant_id` to construct the correct Microsoft OAuth URLs.

---

## R7: Email Verification Edge Cases

**Decision**: Three-tier email handling:
1. Provider returns verified email → Create/link account normally
2. Provider returns unverified email → Reject with `#error=email_not_verified`
3. Provider returns no email → Create with placeholder `oauth-{uuid}@social.placeholder` and set `requires_email_update=true`

**Rationale**: Email is the primary account identifier. Unverified emails can be controlled by attackers. Missing emails prevent account recovery. The three-tier approach:
- Balances security (no unverified emails) with usability (account creation still works)
- `requires_email_update` flag signals the frontend to collect an email
- `X-Requires-Email-Update: true` response header allows middleware-level handling

**Alternatives Considered**:
- **Auto-create with unverified email**: Account takeover risk. Rejected.
- **Reject no-email providers**: Loses legitimate GitHub users (7%+ have no public email). Rejected.
- **Separate email collection endpoint**: More complex REST API. Can be added later if needed.

**Implementation**: `ProviderProfile.EmailVerified` field. If false → redirect with `email_not_verified` error. If email empty → set placeholder and `requires_email_update=true`.

---

## R8: Link/Unlink Security Constraints

**Decision**:
- Link: One provider per user (e.g., a user can have at most 1 Google account linked)
- Unlink: Prevented if it would leave the user with no authentication method (no password, no other providers)
- Soft-deleted/disabled providers: Block new flows but existing linked accounts continue working

**Rationale**:
- **One provider per type**: Prevents confusion about which Google account is linked. Simplifies UX.
- **Last auth method protection**: Prevents account lockout. User must set a password before unlinking their last provider.
- **Disabled providers**: Existing OAuth accounts still work for authentication (token exchange succeeds) but new login/link flows are blocked at the redirect stage.

**Implementation**: `UNIQUE (user_id, provider_id) WHERE deleted_at IS NULL` on oauth_accounts. Service-layer check: `CountAuthMethods(userID) >= 2` before allowing unlink. Provider `is_enabled` checked at redirect stage, not callback stage.

---

## R9: Redis State Management

**Decision**: Redis key `oauth:state:{nonce}` with 10-minute TTL. Value: JSON with callback URL, provider, intent, user ID, PKCE code verifier, org ID, timestamp.

**Rationale**: 
- **Redis over database**: Sub-millisecond read/write, automatic TTL expiry, multi-instance compatible
- **JSON value over hash**: Allows atomic read+delete in single operation, extensible schema
- **10-minute TTL**: Balances security (short window for code interception) with UX (allows slow user interaction on provider consent screen)
- **nonce**: cryptographically random 32-byte hex string, preventing prediction attacks

**Implementation**: `StateManager` interface in `internal/service/oauth_state.go` with `CreateState()` and `ValidateState()` methods. Redis implementation uses `SET key value EX 600 NX`. Test implementation uses in-memory map with expiry.

---

## R10: Integration with Existing Auth System

**Decision**: Social login reuses existing `TokenService` for JWT generation. Social login creates `RefreshToken` entity in database (consistent with password login).

**Rationale**:
- **TokenService reuse**: Same access token format, same refresh token rotation, same revocation mechanism
- **RefreshToken entity**: Consistent with password login flow. Enables session management, family-based rotation attack detection, and logout revocation.
- **No separate auth path**: Social login produces the same tokens as password login. Frontend uses the same auth middleware.

**Implementation**: `OAuthLoginService.LoginOrRegister()` calls `TokenService.GenerateTokens()` after finding or creating the user. Returns `LoginResponse` identical to password login.

---

## Summary

| # | Decision | Chosen | Rejected |
|---|----------|--------|----------|
| R1 | Token delivery | URL fragment encoding (RFC 9269) | Query parameters, POST body, session exchange |
| R2 | CSRF protection | Redis state + PKCE S256 | State only, PKCE only, in-memory state |
| R3 | Client secret encryption | AES-256-GCM + HKDF-SHA256 | Plaintext, AES-CBC, NaCl, KMS |
| R4 | Provider normalization | ProviderProfile struct | Per-provider code paths |
| R5 | Callback handler | Shared GET handler (intent in state) | Separate handlers, query parameter routing |
| R6 | Microsoft tenant_id | JSONB config column | Separate table, env vars |
| R7 | Email handling | 3-tier: verified/unverified/missing | Auto-create unverified, reject no-email |
| R8 | Link/unlink security | One provider per type, last auth protection | Unrestricted linking |
| R9 | Redis state | JSON value + 10min TTL + nonce | Database state, in-memory state |
| R10 | Auth integration | Reuse TokenService + RefreshToken entity | Separate auth path |