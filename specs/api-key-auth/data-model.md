# Data Model: API Key Authentication

**Feature**: API Key Authentication  
**Date**: 2026-04-23  
**Status**: Phase 1 Design

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ Existing Core RBAC                                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│   ┌──────────┐                                               │
│   │  users   │ ◄─────────────────────────────────────┐      │
│   └──────────┘                                        │      │
│          │                                            │      │
│          │ 1:N                                        │      │
│          ▼                                            │      │
│   ┌──────────────┐                                   │      │
│   │ refresh_     │                                   │      │
│   │ tokens       │                                   │      │
│   └──────────────┘                                   │      │
│                                                       │      │
└───────────────────────────────────────┬───────────────┘      │
                                        │
                                        │ 1:N
                                        │
┌───────────────────────────────────────│──────────────────────┐
│ API Key Auth                          │                      │
├───────────────────────────────────────│──────────────────────┤
│                                       │                      │
│   ┌──────────────────┐                │                      │
│   │  api_keys        │                │                      │
│   ├──────────────────┤◄───────────────┘                      │
│   │ id (PK)          │ UUID                                  │
│   │ user_id (FK)     │ UUID → users.id                        │
│   │ name             │ VARCHAR(255)                           │
│   │ prefix           │ VARCHAR(12)                            │
│   │ key_hash         │ VARCHAR(255) UNIQUE                    │
│   │ scopes           │ JSONB                                  │
│   │ expires_at       │ TIMESTAMPTZ (nullable)                 │
│   │ last_used_at     │ TIMESTAMPTZ (nullable)                 │
│   │ revoked_at       │ TIMESTAMPTZ (nullable)                 │
│   │ created_at       │ TIMESTAMPTZ                            │
│   │ updated_at       │ TIMESTAMPTZ                            │
│   │ deleted_at       │ TIMESTAMPTZ (nullable)                 │
│   └──────────────────┘                                       │
│                                                               │
└───────────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────┐
│ Audit Trail                                                   │
├───────────────────────────────────────────────────────────────┤
│                                                                │
│   ┌──────────────────────┐                                    │
│   │  audit_logs           │  (existing, now logs API key ops) │
│   ├──────────────────────┤                                    │
│   │ action: api_key:create, api_key:revoke, api_key:auth    │
│   │ resource: api_key                                        │
│   │ actor_id: user_id (from JWT or API key)                  │
│   └──────────────────────┘                                    │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

---

## Entity: api_keys

**Purpose**: Store API key credentials with scoped permissions for external integrations.

### Schema

```sql
CREATE TABLE api_keys (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  prefix VARCHAR(12) NOT NULL,
  key_hash VARCHAR(255) UNIQUE NOT NULL,
  scopes JSONB DEFAULT '[]',
  expires_at TIMESTAMPTZ DEFAULT NULL,
  last_used_at TIMESTAMPTZ DEFAULT NULL,
  revoked_at TIMESTAMPTZ DEFAULT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ DEFAULT NULL,
  
  CONSTRAINT chk_key_hash_length CHECK (LENGTH(key_hash) >= 60),
  CONSTRAINT chk_prefix_format CHECK (prefix ~ '^ak_[a-z]+_[a-z0-9]+$')
);

-- Indexes for performance
CREATE UNIQUE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_api_keys_revoked_at ON api_keys(revoked_at) WHERE revoked_at IS NOT NULL;
CREATE INDEX idx_api_keys_deleted_at ON api_keys(deleted_at) WHERE deleted_at IS NOT NULL;

-- Composite index for active key lookup
CREATE INDEX idx_api_keys_active ON api_keys(user_id, deleted_at, revoked_at) 
  WHERE deleted_at IS NULL AND revoked_at IS NULL;
```

### Field Descriptions

| Field | Type | Constraints | Purpose |
|-------|------|-------------|---------|
| `id` | UUID | PK, auto-generated | Unique key identifier |
| `user_id` | UUID | FK → users.id, NOT NULL, indexed | Owner of the key |
| `name` | VARCHAR(255) | NOT NULL | Human-readable key name (e.g., "Production Integration") |
| `prefix` | VARCHAR(12) | NOT NULL, format check | First 12 chars of key for identification (e.g., "ak_live_a1b2") |
| `key_hash` | VARCHAR(255) | UNIQUE, NOT NULL, min 60 chars | bcrypt hash of full key value |
| `scopes` | JSONB | DEFAULT '[]' | Array of scopes: `["invoices:read", "news:write"]` |
| `expires_at` | TIMESTAMPTZ | NULL | Optional expiration timestamp |
| `last_used_at` | TIMESTAMPTZ | NULL | Last successful authentication |
| `revoked_at` | TIMESTAMPTZ | NULL | Soft revocation timestamp (NULL = active) |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last modification |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete (cascade from user) |

### Key Format

```
Full key: ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
└─┬─┘ └─┬─┘ └───────────────┬───────────────┘
  │     │                    │
  │     │                    └─ 32 random alphanumeric chars
  │     └─ environment: "live" or "test"
  └─ prefix: "ak" (API key)

Stored in DB:
  prefix: "ak_live_a1b2" (12 chars)
  key_hash: bcrypt("ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6")
```

### Scope Model

```typescript
// Scopes are stored as JSONB array
{
  "scopes": ["invoices:read", "invoices:write", "news:read"]
}

// Scope validation rules:
// - Format: resource:action
// - Actions: "read", "write", "manage", "*"
// - "write" = create + update + delete
// - "*" = full access (admin only)

// Scope → Permission mapping:
// invoices:read → invoices:read
// invoices:write → invoices:create, invoices:update, invoices:delete
// news:manage → news:create, news:read, news:update, news:delete
// * → all permissions
```

### State Transitions

```
         ┌──────────────────┐
         │   Created        │
         │  (active)        │
         └────────┬─────────┘
                  │
        ┌─────────┴─────────┐
        ▼                   ▼
┌───────────────┐   ┌───────────────┐
│   Used        │   │   Expired     │
│ (last_used_at │   │ (expires_at   │
│   updated)    │   │  < NOW)       │
└───────┬───────┘   └───────┬───────┘
        │                   │
        └─────────┬─────────┘
                  ▼
         ┌──────────────────┐
         │   Revoked        │
         │  (revoked_at     │
         │   set)           │
         └────────┬─────────┘
                  │
                  ▼
         ┌──────────────────┐
         │   Deleted        │
         │  (deleted_at    │
         │   set)           │
         └──────────────────┘
```

### Validation Rules

| Field | Rule | Type |
|-------|------|------|
| `name` | Required, max 255 chars | required |
| `name` | Unique per user | unique scope |
| `prefix` | Format: `ak_[a-z]+_[a-z0-9]+` | format |
| `key_hash` | Unique globally | unique |
| `key_hash` | bcrypt hash format | format |
| `scopes` | Valid scope format | format |
| `scopes` | Subset of user permissions | permission |
| `expires_at` | Future timestamp if set | constraint |
| `revoked_at` | >= created_at if set | constraint |

---

## Relationships

```
users (1) ────────────< (M) api_keys

Relationships:
- user_id FK → users.id ON DELETE CASCADE
- When user is soft-deleted, api_keys are soft-deleted
- When user is hard-deleted (cleanup), api_keys are cascade deleted
```

---

## GORM Model

```go
package domain

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/datatypes"
    "gorm.io/gorm"
)

type APIKey struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
    User        User           `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
    Name        string         `gorm:"size:255;not null" json:"name"`
    Prefix      string         `gorm:"size:12;not null" json:"prefix"`
    KeyHash     string         `gorm:"size:255;not null;uniqueIndex" json:"-"`
    Scopes      datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"scopes"`
    ExpiresAt   *time.Time     `gorm:"index" json:"expires_at,omitempty"`
    LastUsedAt  *time.Time     `gorm:"index" json:"last_used_at,omitempty"`
    RevokedAt   *time.Time     `gorm:"index" json:"revoked_at,omitempty"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (APIKey) TableName() string {
    return "api_keys"
}

type APIKeyResponse struct {
    ID          string     `json:"id"`
    UserID      string     `json:"user_id"`
    Name        string     `json:"name"`
    Prefix      string     `json:"prefix"`
    Scopes      []string   `json:"scopes"`
    ExpiresAt   *string    `json:"expires_at,omitempty"`
    LastUsedAt  *string    `json:"last_used_at,omitempty"`
    IsRevoked   bool       `json:"is_revoked"`
    CreatedAt   string     `json:"created_at"`
    UpdatedAt   string     `json:"updated_at"`
}

func (k *APIKey) ToResponse() APIKeyResponse {
    var scopes []string
    json.Unmarshal(k.Scopes, &scopes)
    
    var expiresAtStr, lastUsedAtStr *string
    if k.ExpiresAt != nil {
        formatted := k.ExpiresAt.Format(time.RFC3339)
        expiresAtStr = &formatted
    }
    if k.LastUsedAt != nil {
        formatted := k.LastUsedAt.Format(time.RFC3339)
        lastUsedAtStr = &formatted
    }
    
    return APIKeyResponse{
        ID:          k.ID.String(),
        UserID:      k.UserID.String(),
        Name:        k.Name,
        Prefix:      k.Prefix,
        Scopes:      scopes,
        ExpiresAt:   expiresAtStr,
        LastUsedAt:  lastUsedAtStr,
        IsRevoked:   k.RevokedAt != nil,
        CreatedAt:   k.CreatedAt.Format(time.RFC3339),
        UpdatedAt:   k.UpdatedAt.Format(time.RFC3339),
    }
}

func (k *APIKey) IsActive() bool {
    if k.DeletedAt.Valid {
        return false
    }
    if k.RevokedAt != nil {
        return false
    }
    if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
        return false
    }
    return true
}

func (k *APIKey) HasScope(scope string) bool {
    var scopes []string
    if err := json.Unmarshal(k.Scopes, &scopes); err != nil {
        return false
    }
    
    for _, s := range scopes {
        if s == "*" || s == scope {
            return true
        }
    }
    return false
}
```

---

## Repository Interface

```go
package repository

type APIKeyRepository interface {
    // Create a new API key
    Create(ctx context.Context, apiKey *domain.APIKey) error
    
    // Find by ID (user must own the key)
    FindByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)
    
    // Find by key hash (for authentication)
    FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
    
    // Find all keys for a user (with pagination)
    FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.APIKey, int64, error)
    
    // Update an API key
    Update(ctx context.Context, apiKey *domain.APIKey) error
    
    // Revoke an API key (soft)
    Revoke(ctx context.Context, id uuid.UUID) error
    
    // Soft delete (cascade from user deletion)
    SoftDelete(ctx context.Context, id uuid.UUID) error
    
    // Soft delete by user (cascade)
    SoftDeleteByUserID(ctx context.Context, userID uuid.UUID) error
    
    // Update LastUsedAt
    UpdateLastUsedAt(ctx context.Context, id uuid.UUID) error
    
    // Count active keys for a user
    CountActiveByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
}
```

---

## Service Interface

```go
package service

type APIKeyService struct {
    repo     repository.APIKeyRepository
    userRepo repository.UserRepository
    enforcer *permission.Enforcer
    audit    *AuditService
}

type CreateAPIKeyRequest struct {
    Name      string   `json:"name" validate:"required,max=255"`
    Scopes    []string `json:"scopes" validate:"required,dive,scope"`
    ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type APIKeyWithSecret struct {
    APIKey *domain.APIKeyResponse
    Secret string // Only returned on creation
}

func (s *APIKeyService) Create(ctx context.Context, userID uuid.UUID, req CreateAPIKeyRequest) (*APIKeyWithSecret, error)
func (s *APIKeyService) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.APIKey, int64, error)
func (s *APIKeyService) GetByID(ctx context.Context, userID, keyID uuid.UUID) (*domain.APIKey, error)
func (s *APIKeyService) Revoke(ctx context.Context, userID, keyID uuid.UUID) error
func (s *APIKeyService) Validate(ctx context.Context, key string) (*domain.APIKey, *domain.User, error)
func (s *APIKeyService) ValidateScopes(ctx context.Context, userID uuid.UUID, scopes []string) error
```

---

## Migration Files

### Migration: 000005_api_keys.up.sql

```sql
-- Create api_keys table
CREATE TABLE api_keys (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  prefix VARCHAR(12) NOT NULL,
  key_hash VARCHAR(255) UNIQUE NOT NULL,
  scopes JSONB DEFAULT '[]',
  expires_at TIMESTAMPTZ DEFAULT NULL,
  last_used_at TIMESTAMPTZ DEFAULT NULL,
  revoked_at TIMESTAMPTZ DEFAULT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ DEFAULT NULL,
  
  CONSTRAINT chk_key_hash_length CHECK (LENGTH(key_hash) >= 60),
  CONSTRAINT chk_prefix_format CHECK (prefix ~ '^ak_[a-z]+_[a-z0-9]+$')
);

-- Create indexes
CREATE UNIQUE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_api_keys_revoked_at ON api_keys(revoked_at) WHERE revoked_at IS NOT NULL;
CREATE INDEX idx_api_keys_deleted_at ON api_keys(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_api_keys_active ON api_keys(user_id, deleted_at, revoked_at) 
  WHERE deleted_at IS NULL AND revoked_at IS NULL;

-- Add trigger for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_api_keys_updated_at 
    BEFORE UPDATE ON api_keys 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add audit log trigger (optional - can be done in application layer)
-- CREATE TRIGGER audit_api_key_changes AFTER INSERT OR UPDATE OR DELETE ON api_keys ...

-- Grant permissions (adjust role name as needed)
GRANT SELECT, INSERT, UPDATE, DELETE ON api_keys TO go_api_base;
```

### Migration: 000005_api_keys.down.sql

```sql
-- Drop indexes
DROP INDEX IF EXISTS idx_api_keys_active;
DROP INDEX IF EXISTS idx_api_keys_deleted_at;
DROP INDEX IF EXISTS idx_api_keys_revoked_at;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_key_hash;

-- Drop trigger
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop table
DROP TABLE IF EXISTS api_keys;

-- Note: This will cascade delete all API keys
-- Ensure backup before running in production
```

---

## Permissions to Add

Add new permissions to `permissions` table via `permission:sync`:

```yaml
# config/permissions.yaml additions
- name: api_key:create
  resource: api_key
  action: create
  scope: all
  is_system: true
  
- name: api_key:read
  resource: api_key
  action: read
  scope: own
  is_system: true
  
- name: api_key:revoke
  resource: api_key
  action: revoke
  scope: own
  is_system: true
  
- name: api_key:manage
  resource: api_key
  action: manage
  scope: all
  is_system: true
```

---

## Audit Log Entries

API Key operations will create audit log entries:

| Action | Resource | ResourceID | Before | After |
|--------|----------|------------|--------|-------|
| `api_key:create` | `api_key` | `{key_id}` | null | `{name, scopes, expires_at}` |
| `api_key:read` | `api_key` | `{key_id}` | null | null |
| `api_key:revoke` | `api_key` | `{key_id}` | `{name, scopes}` | `{name, scopes, revoked_at}` |
| `api_key:delete` | `api_key` | `{key_id}` | `{name, scopes}` | null (soft delete) |
| `api_key:authenticate` | `api_key` | `{key_id}` | null | `{last_used_at}` |

---

**Version**: 1.0 | **Created**: 2026-04-23 | **Status**: Phase 1 Design