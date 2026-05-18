# OAuth 2.0 Social Login — Quickstart Guide

**Date**: 2026-05-18 | **Branch**: `021-oauth-social-login`

## Prerequisites

- Go 1.22+
- Docker & Docker Compose (for integration tests)
- OAuth provider credentials (Google, GitHub, and/or Microsoft)

## Setup

### 1. Run Migrations

```bash
make migrate
# Creates: oauth_providers, oauth_accounts tables
```

### 2. Sync Permissions

```bash
go run ./cmd/api permission:sync
# Adds: oauth:view, oauth:link, oauth:manage
```

### 3. Configure Environment

Add to `.env`:

```env
# OAuth Configuration
OAUTH_ENCRYPTION_KEY=              # Optional: Override JWT_SECRET-derived key
OAUTH_ALLOW_HTTP=false             # Set true for local dev only
OAUTH_FRONTEND_CALLBACK_URL=http://localhost:3000/auth/callback
OAUTH_STATE_TTL=600                # State parameter TTL in seconds
```

### 4. Create Provider Configuration (Admin API)

```bash
# Google
curl -X POST http://localhost:8080/api/v1/oauth-providers \
  -H "Authorization: Bearer {admin_token}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "google",
    "display_name": "Google",
    "client_id": "YOUR_GOOGLE_CLIENT_ID.apps.googleusercontent.com",
    "client_secret": "YOUR_GOOGLE_CLIENT_SECRET",
    "redirect_url": "http://localhost:3000/auth/callback"
  }'

# GitHub
curl -X POST http://localhost:8080/api/v1/oauth-providers \
  -H "Authorization: Bearer {admin_token}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "display_name": "GitHub",
    "client_id": "YOUR_GITHUB_CLIENT_ID",
    "client_secret": "YOUR_GITHUB_CLIENT_SECRET",
    "redirect_url": "http://localhost:3000/auth/callback"
  }'

# Microsoft
curl -X POST http://localhost:8080/api/v1/oauth-providers \
  -H "Authorization: Bearer {admin_token}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "microsoft",
    "display_name": "Microsoft",
    "client_id": "YOUR_MS_CLIENT_ID",
    "client_secret": "YOUR_MS_CLIENT_SECRET",
    "redirect_url": "http://localhost:3000/auth/callback",
    "config": {"tenant_id": "common"}
  }'
```

## Usage

### Login with OAuth (Frontend Flow)

```text
1. Frontend: GET /api/v1/auth/oauth/google
   → 302 redirect to Google authorization URL

2. User authorizes on Google

3. Google redirects to: GET /api/v1/auth/oauth/google/callback?code=xxx&state=yyy

4. Backend validates state, exchanges code, creates/finds user

5. Backend redirects to: http://localhost:3000/auth/callback#access_token=eyJ...&refresh_token=abc123&token_type=Bearer&expires_in=900

6. Frontend: Parse window.location.hash, store tokens, remove hash
```

### Link OAuth Provider (Authenticated User)

```bash
# Step 1: Get redirect URL
curl -X POST http://localhost:8080/api/v1/auth/oauth/google/link \
  -H "Authorization: Bearer {user_token}"
# Response: {"data": {"redirect_url": "https://accounts.google.com/o/oauth2/v2/auth?..."}}

# Step 2: Redirect user to redirect_url

# Step 3: Provider redirects to callback (same as login, but with intent=link)

# Step 4: Frontend receives: http://localhost:3000/auth/callback#status=linked&provider=google&message=Provider+linked+successfully
```

### Unlink OAuth Provider

```bash
curl -X DELETE http://localhost:8080/api/v1/auth/oauth/google/unlink \
  -H "Authorization: Bearer {user_token}"
# Response: {"data": {"message": "Provider unlinked successfully"}}
```

### List Linked Accounts

```bash
curl http://localhost:8080/api/v1/auth/oauth/accounts \
  -H "Authorization: Bearer {user_token}"
# Response: {"data": [{"id": "...", "provider": "google", "email": "...", ...}]}
```

### List Available Providers (Public)

```bash
curl http://localhost:8080/api/v1/auth/oauth/providers
# Response: {"data": [{"name": "google", "display_name": "Google", "scopes": [...]}]}
```

## Error Handling

### Frontend Error Parsing

```javascript
// Parse fragment on callback page
const hash = window.location.hash.substring(1);
const params = new URLSearchParams(hash);

if (params.has('error')) {
  const error = params.get('error');
  const message = params.get('message');
  // Handle error
} else if (params.has('access_token')) {
  const accessToken = params.get('access_token');
  const refreshToken = params.get('refresh_token');
  // Store tokens
}

// Remove hash from URL
window.history.replaceState(null, '', window.location.pathname);
```

### Common Error Scenarios

| Error Code | User Action |
|------------|-------------|
| `invalid_state` | Restart login flow (state expired) |
| `access_denied` | Try again or use different provider |
| `email_already_exists` | Log in with existing account, then link provider |
| `email_not_verified` | Verify email with provider first |
| `provider_already_linked` | Provider already connected (check accounts) |
| `account_already_linked` | Provider account used by another user |

## Testing

```bash
# Unit tests
go test -v -race -coverprofile=coverage.txt ./tests/unit/... -run OAuth

# Integration tests (requires Docker)
go test -v -tags=integration ./tests/integration/... -run OAuth -timeout 5m
```

## Architecture Notes

### Encryption Key Derivation

- Default: `JWT_SECRET` → HKDF-SHA256 (info: `oauth-encryption-v1`) → 32-byte AES-GCM key
- Override: `OAUTH_ENCRYPTION_KEY` env var (bypasses HKDF, used for key rotation)
- Format: `v1:` + base64(IV[12] + ciphertext + GCM_tag[16])
- Version prefix enables future algorithm migration

### PKCE Flow

1. Initiate: Generate `code_verifier` (43-char random), compute `code_challenge` = base64url(sha256(code_verifier))
2. Redirect: Include `code_challenge` and `code_challenge_method=S256` in authorization URL
3. Callback: Send `code_verifier` in token exchange request (stored in Redis state)
4. Provider: Verifies `code_challenge` matches `code_verifier`

### Redis State

- Key: `oauth:state:{nonce}` where nonce = 32-byte hex random
- TTL: 600 seconds (10 minutes)
- Value: JSON `{callback_url, provider, intent, user_id, code_verifier, org_id, created_at}`
- Atomic GET + DEL on validation (prevents replay attacks)