# Data Model: OAuth 2.0 Social Login

**Date**: 2026-05-18 | **Branch**: `021-oauth-social-login` | **Spec**: [spec.md](./spec.md)

## Entity Relationship Diagram

```text
┌──────────────────────┐        ┌──────────────────────────┐
│      users           │        │     oauth_providers      │
├──────────────────────┤        ├──────────────────────────┤
│ id (UUID) PK         │        │ id (UUID) PK             │
│ email                │        │ name (VARCHAR 50) UNIQUE │
│ password_hash        │        │ display_name (VARCHAR100) │
│ requires_email_update│        │ client_id (VARCHAR 500)  │
│ ...                  │        │ client_secret_encrypted  │
└──────────┬───────────┘        │ redirect_url (VARCHAR500)│
           │                    │ additional_scopes (TEXT[])│
           │ 1                  │ config (JSONB)            │
           │                    │ is_enabled (BOOL)         │
           │                    │ is_system (BOOL)          │
┌──────────┴───────────┐        │ organization_id (UUID FK) │
│   oauth_accounts     │        │ created_at, updated_at    │
├──────────────────────┤        │ deleted_at (soft delete)  │
│ id (UUID) PK         │        └──────────┬───────────────┘
│ user_id (UUID FK)    │                   │
│ provider_id (UUID FK├───────────────────┘
│ provider_user_id     │        NOTE: Both tables have
│ email (VARCHAR 255)  │        partial unique indexes
│ email_verified (BOOL)│        (WHERE deleted_at IS NULL)
│ display_name         │
│ avatar_url           │
│ created_at, updated_at│
│ deleted_at (soft del)│
└──────────────────────┘

         ┌──────────────────┐
         │   Redis State    │
         │  oauth:state:{n} │
         ├──────────────────┤
         │ callback_url     │
         │ provider         │
         │ intent           │
         │ user_id          │
         │ code_verifier    │
         │ org_id           │
         │ created_at       │
         │ TTL: 600s        │
         └──────────────────┘
```

## Entity Definitions

### OAuthProvider

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, DEFAULT gen_random_uuid() | Primary key |
| name | VARCHAR(50) | NOT NULL, UNIQUE WHERE deleted_at IS NULL | Provider identifier: "google", "github", "microsoft" |
| display_name | VARCHAR(100) | NOT NULL | Human-readable: "Google", "GitHub", "Microsoft" |
| client_id | VARCHAR(500) | NOT NULL | OAuth client ID from provider |
| client_secret_encrypted | TEXT | NOT NULL | AES-256-GCM encrypted. Format: `v1:` + base64(IV + ciphertext + tag) |
| redirect_url | VARCHAR(500) | NOT NULL | Frontend callback URL for OAuth redirects |
| additional_scopes | TEXT[] | DEFAULT '{}' | Extra OAuth scopes appended to provider defaults |
| config | JSONB | DEFAULT '{}' | Provider-specific config (e.g., Microsoft tenant_id) |
| is_enabled | BOOLEAN | NOT NULL DEFAULT true | Disabled providers block new OAuth flows |
| is_system | BOOLEAN | NOT NULL DEFAULT false | System providers cannot be deleted |
| organization_id | UUID | FK → organizations(id), nullable | NULL = global provider, non-null = org-scoped |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() | Last update timestamp |
| deleted_at | TIMESTAMPTZ | nullable | Soft delete timestamp |

**Indexes**:
- `idx_oauth_providers_org_id ON oauth_providers(organization_id) WHERE deleted_at IS NULL`
- `idx_oauth_providers_enabled ON oauth_providers(is_enabled) WHERE deleted_at IS NULL`
- `uq_oauth_providers_name UNIQUE (name) WHERE deleted_at IS NULL`
- `uq_oauth_providers_name_org UNIQUE (name, organization_id) WHERE deleted_at IS NULL`

**Validations**:
- `name` must be one of: "google", "github", "microsoft" (enforced in service layer)
- `client_secret_encrypted` must be valid `v1:` prefixed format after encryption
- `redirect_url` must be a valid HTTPS URL (HTTP allowed if `OAUTH_ALLOW_HTTP=true`)
- `additional_scopes` max 10 entries, each max 50 chars

**Business Methods**:
- `EncryptSecret(plaintext, masterKey) → encryptedString`
- `DecryptSecret(encrypted, masterKey) → plaintext`
- `GetEffectiveScopes() []string` — merges default scopes + additional_scopes
- `GetAuthorizationURL(state, codeChallenge) string` — builds provider-specific auth URL
- `GetTokenURL() string` — returns provider token exchange URL
- `GetUserInfoURL() string` — returns provider user info endpoint
- `ToResponse() OAuthProviderResponse` — strips encrypted secret for API responses

### OAuthAccount

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, DEFAULT gen_random_uuid() | Primary key |
| user_id | UUID | NOT NULL, FK → users(id) | Linked user |
| provider_id | UUID | NOT NULL, FK → oauth_providers(id) | Linked provider |
| provider_user_id | VARCHAR(255) | NOT NULL | User ID from provider (string: GitHub uses integers) |
| email | VARCHAR(255) | nullable | Email from provider (may be null for some GitHub users) |
| email_verified | BOOLEAN | DEFAULT false | Whether provider confirmed email ownership |
| display_name | VARCHAR(255) | nullable | Display name from provider |
| avatar_url | VARCHAR(500) | nullable | Profile picture URL from provider |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() | Link timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() | Last update timestamp |
| deleted_at | TIMESTAMPTZ | nullable | Soft delete timestamp |

**Indexes**:
- `idx_oauth_accounts_user_id ON oauth_accounts(user_id) WHERE deleted_at IS NULL`
- `idx_oauth_accounts_provider_id ON oauth_accounts(provider_id) WHERE deleted_at IS NULL`
- `idx_oauth_accounts_email ON oauth_accounts(email) WHERE deleted_at IS NULL`
- `uq_oauth_accounts_provider_user UNIQUE (provider_id, provider_user_id) WHERE deleted_at IS NULL`
- `uq_oauth_accounts_user_provider UNIQUE (user_id, provider_id) WHERE deleted_at IS NULL`

**Validations**:
- One account per provider per user (enforced by unique constraint)
- One provider_user_id per provider (enforced by unique constraint)
- Cannot unlink last authentication method (service-layer check)

**Business Methods**:
- `ToResponse() OAuthAccountResponse` — converts to API response DTO

### OAuthState (Redis, not persisted in PostgreSQL)

| Field | Type | Description |
|-------|------|-------------|
| callback_url | string | Frontend URL to redirect to after callback |
| provider | string | Provider name: "google", "github", "microsoft" |
| intent | string | "login" or "link" |
| user_id | UUID | uuid.Nil for login, actual user ID for link |
| code_verifier | string | PKCE S256 code verifier |
| org_id | UUID | Organization context (uuid.Nil for global) |
| created_at | time.Time | State creation timestamp |

**Redis Key**: `oauth:state:{nonce}` where nonce is a 32-byte hex string
**TTL**: 600 seconds (10 minutes)

### ProviderProfile (Internal, not persisted)

| Field | Type | Description |
|-------|------|-------------|
| ProviderID | string | Unique user ID from the provider |
| Email | string | Email from provider (may be empty) |
| EmailVerified | bool | Whether provider confirmed email |
| DisplayName | string | Name from provider |
| AvatarURL | string | Profile picture URL |

## State Transitions

### OAuthProvider Lifecycle

```text
[Created] ──is_enabled=true──→ [Active] ──is_enabled=false──→ [Disabled]
    │                              │                              │
    │                              │                              │
    │                              └──soft_delete──→ [Deleted]     │
    │                                                              │
    └──soft_delete──→ [Deleted]    ┌──restore──→ [Active]        │
                                   └──hard_delete──→ [Removed]     │
                                                                    │
                                   [Disabled] ──is_enabled=true──→ [Active]
```

- **Disabled providers**: Block new login/link redirects. Existing linked accounts continue to work.
- **Soft-deleted providers**: Invisible to API. Existing linked accounts still authenticate via provider_id lookup.

### OAuthAccount Lifecycle

```text
[Not Linked] ──Link──→ [Linked] ──Unlink──→ [Soft Deleted]
                              │
                              └──User Soft Delete──→ [Cascade Soft Delete]
```

### OAuth Flow State Machine

```text
[Initiate] ──GET /auth/oauth/:provider──→ [Redirect to Provider]
[Initiate] ──POST /auth/oauth/:provider/link──→ [Store State + Redirect URL]

[Redirect to Provider] ──User Authorizes──→ [Callback with Code]

[Callback with Code] ──Validate State──→ [Code Exchange]
[Callback with Code] ──Invalid State──→ [Error: invalid_state]

[Code Exchange] ──PKCE Verify──→ [Fetch Provider Profile]
[Code Exchange] ──PKCE Fail──→ [Error: invalid_state]

[Fetch Provider Profile] ──intent=login──→ [Login Flow]
[Fetch Provider Profile] ──intent=link──→ [Link Flow]

[Login Flow] ──Account Found──→ [Generate Tokens + Redirect]
[Login Flow] ──No Account + Auto-Create──→ [Create User + Generate Tokens]
[Login Flow] ──No Account + Email Exists──→ [Error: email_already_exists]
[Login Flow] ──No Account + Email Unverified──→ [Error: email_not_verified]

[Link Flow] ──Not Already Linked──→ [Create OAuthAccount + Redirect]
[Link Flow] ──Already Linked──→ [Error: provider_already_linked]
[Link Flow] ──Different User──→ [Error: account_already_linked]

[Generate Tokens] ──Success──→ [Redirect with Fragment]
[Generate Tokens] ──Failure──→ [Error: internal_error]
```

## Migration Plan

### Migration 000027: oauth_providers

```sql
CREATE TABLE oauth_providers (
    -- See spec.md for full DDL
);
```

### Migration 000028: oauth_accounts

```sql
CREATE TABLE oauth_accounts (
    -- See spec.md for full DDL
);
```

### Seed Data

OAuth permissions added via `permission:sync` CLI command:
- `oauth:view` — resource: `oauth`, action: `view`, scope: `own`
- `oauth:link` — resource: `oauth`, action: `link`, scope: `own`
- `oauth:manage` — resource: `oauth`, action: `manage`, scope: `all`

## Encryption Key Hierarchy

```text
JWT_SECRET (env var)
    │
    ├── HKDF-SHA256(info="oauth-encryption-v1", salt=random)
    │   └── Derived Key (32 bytes) ──→ AES-256-GCM Key
    │                                   │
    │                                   ├── Encrypt(plaintext) → v1:base64(IV+ciphertext+tag)
    │                                   └── Decrypt(v1:base64(...)) → plaintext
    │
    └── OAUTH_ENCRYPTION_KEY (env var, optional override)
        └── Used directly if set (bypasses HKDF derivation)
```