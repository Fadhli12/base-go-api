# Data Model: Go API Base

**Date**: 2026-04-22 | **Status**: Phase 1 Design

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ Core RBAC + Auth                                             │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│   ┌──────────┐         ┌────────────┐      ┌───────────┐  │
│   │  users   │◄────────│ user_roles │─────►│   roles   │  │
│   └──────────┘         └────────────┘      └───────────┘  │
│         │                                          │         │
│         │◄─── direct override (allow/deny) ────┘             │
│         │                                                      │
│    ┌────┴──────────────────┐                                 │
│    │                        │                                 │
│   ┌▼──────────────┐   ┌────▼──────────────┐                 │
│   │ user_          │   │role_              │                 │
│   │permissions     │   │permissions        │                 │
│   └───────────────┘   └────────────────────┘                 │
│         │                      │                              │
│         └──────┬───────────────┘                             │
│                │                                              │
│         ┌──────▼──────────┐                                  │
│         │  permissions    │  (resource:action, runtime)      │
│         └─────────────────┘                                  │
│                                                               │
│   ┌──────────────────────────┐                              │
│   │  refresh_tokens          │  (revocation store)           │
│   │  - user_id FK            │                              │
│   │  - revoked_at (nullable) │                              │
│   └──────────────────────────┘                              │
│                                                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Audit & Logging                                              │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│   ┌──────────────────────────┐                              │
│   │  audit_logs              │  (immutable)                  │
│   │  - actor_id FK (users)   │                              │
│   │  - action                │                              │
│   │  - resource              │                              │
│   │  - resource_id           │                              │
│   │  - before (JSONB)        │  (snapshot)                  │
│   │  - after (JSONB)         │  (snapshot)                  │
│   │  - ip_address            │                              │
│   │  - user_agent            │                              │
│   │  - created_at            │                              │
│   └──────────────────────────┘                              │
│                                                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Example Domain: Invoices                                     │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│   ┌──────────────────────────┐                              │
│   │  invoices                │                              │
│   │  - id (UUID)             │                              │
│   │  - user_id FK (owner)    │  (scope check)               │
│   │  - amount                │                              │
│   │  - status                │                              │
│   │  - created_at            │                              │
│   │  - updated_at            │                              │
│   │  - deleted_at (soft del) │                              │
│   └──────────────────────────┘                              │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Entities

### 1. users

**Purpose**: Core user identity and authentication.

```sql
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ DEFAULT NULL
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK | Unique user identifier |
| `email` | VARCHAR(255) | UNIQUE, NOT NULL | Login identifier; email format validation |
| `password_hash` | VARCHAR(255) | NOT NULL | bcrypt hash; never store plaintext |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last modification timestamp |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete; NULL = active |

**Validation**:
- Email: valid format + domain check optional
- Password: min 8 chars, complexity rules (uppercase, lowercase, digit, symbol)
- No duplicate email

**Indexes**:
- `UNIQUE INDEX idx_users_email` — fast login lookups
- `INDEX idx_users_deleted_at` — soft delete queries

---

### 2. roles

**Purpose**: Grouping permissions; users assigned to one or more roles.

```sql
CREATE TABLE roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(100) UNIQUE NOT NULL,
  description TEXT,
  is_system BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ DEFAULT NULL
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK | Unique role identifier |
| `name` | VARCHAR(100) | UNIQUE, NOT NULL | Role name (PascalCase: SuperAdmin, Manager) |
| `description` | TEXT | NULL | Human-readable purpose |
| `is_system` | BOOLEAN | DEFAULT FALSE | TRUE = protected; cannot delete |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last modification |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete |

**System Roles** (seeded on init):
- `SuperAdmin` — all permissions
- `Admin` — all permissions except role/permission management (configurable)
- `Viewer` — read-only access

**Validation**:
- Name: alphanumeric + underscore only
- is_system = TRUE roles cannot be deleted (handler prevents it)

**Indexes**:
- `UNIQUE INDEX idx_roles_name`
- `INDEX idx_roles_is_system`

---

### 3. permissions

**Purpose**: Define what actions are allowed on what resources. Customizable at runtime.

```sql
CREATE TABLE permissions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(150) UNIQUE NOT NULL,
  resource VARCHAR(100) NOT NULL,
  action VARCHAR(50) NOT NULL,
  scope VARCHAR(50) DEFAULT 'all',
  description TEXT,
  is_system BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ DEFAULT NULL
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK | Unique permission identifier |
| `name` | VARCHAR(150) | UNIQUE, NOT NULL | `resource:action` (e.g., `invoice:create`) |
| `resource` | VARCHAR(100) | NOT NULL | What resource (e.g., `invoice`, `user`, `role`) |
| `action` | VARCHAR(50) | NOT NULL | What action (e.g., `create`, `read`, `update`, `delete`, `*` for all) |
| `scope` | VARCHAR(50) | DEFAULT 'all' | **Scope hint for service-layer authorization**. See Scope Usage below. |

**Scope Usage**:

The `scope` field provides a hint to service-layer authorization logic, but is NOT enforced by Casbin directly.

| Scope | Meaning | Service Layer Check |
|-------|---------|---------------------|
| `own` | User's own resources only | `WHERE user_id = current_user_id` |
| `team` | Resources owned by user's team | `WHERE team_id IN (SELECT team_id FROM user_teams WHERE user_id = current_user_id)` (requires team model) |
| `all` | All resources | No additional filter |

**Important**: 
- Scope is NOT a separate Casbin policy dimension. It's metadata for service-layer filtering.
- When checking permissions, service layer queries: "Does user have `invoice:read`?" then applies scope filter.
- For full scope-based RBAC, consider ABAC (Attribute-Based Access Control) extensions to Casbin model.

**Scope Implementation Example**:

```go
// Service layer
func (s *InvoiceService) List(userID uuid.UUID) ([]Invoice, error) {
    // 1. Coarse permission check (Casbin)
    hasPermission, _ := s.enforcer.Enforce(userID, "invoice", "read")
    if !hasPermission {
        return nil, ErrPermissionDenied
    }
    
    // 2. Get permission scope (service layer)
    perm := s.repo.GetPermission("invoice:read")
    
    // 3. Apply scope filter
    query := s.db.Model(&Invoice{})
    switch perm.Scope {
    case "own":
        query = query.Where("user_id = ?", userID)
    case "team":
        // Requires team model (not in initial scope)
        query = query.Where("team_id IN (?)", s.getTeamIDs(userID))
    case "all":
        // No filter
    }
    
    return query.Find()
}
```

**Scope in MVP**: For initial implementation, use `own` and `all` scopes. Add `team` scope when team/organization model is added.
| `description` | TEXT | NULL | What this permission grants |
| `is_system` | BOOLEAN | DEFAULT FALSE | TRUE = protected; cannot delete |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last modification |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete |

**System Permissions** (seeded on init):
```
- user:create, user:read, user:update, user:delete
- role:create, role:read, role:update, role:delete
- permission:create, permission:read, permission:update, permission:delete
- invoice:create, invoice:read, invoice:update, invoice:delete
- audit_log:read
- (etc., domain-specific)
```

**Validation**:
- Name: `resource:action` format
- Resource/Action: alphanumeric + underscore
- is_system = TRUE cannot be deleted

**Indexes**:
- `UNIQUE INDEX idx_permissions_name`
- `INDEX idx_permissions_resource_action`
- `INDEX idx_permissions_is_system`

---

### 4. user_roles

**Purpose**: Join table; assigns users to roles (many-to-many).

```sql
CREATE TABLE user_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, role_id)
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `user_id` | UUID | FK users, NOT NULL | User being assigned |
| `role_id` | UUID | FK roles, NOT NULL | Role being granted |
| `created_at` | TIMESTAMPTZ | NOT NULL | When assignment created |

**PK**: `(user_id, role_id)` — each user-role pair unique.

**Cascade**: ON DELETE CASCADE — if user deleted, remove all role assignments.

**Indexes**:
- `PRIMARY KEY (user_id, role_id)`
- `INDEX idx_user_roles_role_id` — find all users with a role

---

### 5. role_permissions

**Purpose**: Join table; assigns permissions to roles (many-to-many).

```sql
CREATE TABLE role_permissions (
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (role_id, permission_id)
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `role_id` | UUID | FK roles, NOT NULL | Role being granted |
| `permission_id` | UUID | FK permissions, NOT NULL | Permission being granted |
| `created_at` | TIMESTAMPTZ | NOT NULL | When assignment created |

**PK**: `(role_id, permission_id)` — each role-permission pair unique.

**Cascade**: ON DELETE CASCADE — if role or permission deleted, remove assignment.

**Indexes**:
- `PRIMARY KEY (role_id, permission_id)`
- `INDEX idx_role_permissions_permission_id` — find all roles with a permission

---

### 6. user_permissions

**Purpose**: Direct permission overrides; user can have explicit ALLOW or DENY grants bypassing roles.

```sql
CREATE TABLE user_permissions (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  effect VARCHAR(10) NOT NULL CHECK (effect IN ('allow', 'deny')),
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, permission_id)
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `user_id` | UUID | FK users, NOT NULL | User receiving override |
| `permission_id` | UUID | FK permissions, NOT NULL | Permission being overridden |
| `effect` | VARCHAR(10) | 'allow' or 'deny' | Explicit grant or revocation |
| `created_at` | TIMESTAMPTZ | NOT NULL | When override created |

**Effect Logic**:
- `allow` — user has this permission (override roles)
- `deny` — user lacks this permission (deny overrides allow; safest default)

**PK**: `(user_id, permission_id)` — at most one override per user-permission pair.

**Cascade**: ON DELETE CASCADE — if user or permission deleted, remove override.

**Indexes**:
- `PRIMARY KEY (user_id, permission_id)`
- `INDEX idx_user_permissions_effect` — find all explicit denies

---

### 7. refresh_tokens

**Purpose**: Token revocation store; tracks issued refresh tokens and whether they've been revoked. Supports multi-device sessions.

```sql
CREATE TABLE refresh_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash VARCHAR(64) NOT NULL,
  device_name VARCHAR(100) DEFAULT NULL,
  device_id VARCHAR(100) DEFAULT NULL,
  ip_address INET DEFAULT NULL,
  user_agent TEXT DEFAULT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ DEFAULT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT uq_refresh_tokens_token_hash UNIQUE (token_hash)
);

CREATE UNIQUE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK | Token identifier (not exposed) |
| `user_id` | UUID | FK users, NOT NULL | Token issued to user |
| `token_hash` | VARCHAR(64) | UNIQUE, NOT NULL | SHA256 hash (64 hex chars); UNIQUE prevents duplicate token issuance |
| `device_name` | VARCHAR(100) | NULL | Human-readable device name (e.g., "Chrome on Windows") |
| `device_id` | VARCHAR(100) | NULL | Device fingerprint (hash of UA + IP) |
| `ip_address` | INET | NULL | IP at token issuance |
| `user_agent` | TEXT | NULL | User-Agent at token issuance |
| `expires_at` | TIMESTAMPTZ | NOT NULL | When token expires (30 days from creation) |
| `revoked_at` | TIMESTAMPTZ | NULL | When revoked (NULL = active); logout sets this |
| `created_at` | TIMESTAMPTZ | NOT NULL | When token issued |

**Security Note**: `token_hash` MUST be UNIQUE. This prevents the same refresh token from being issued multiple times, which could bypass revocation checks.

**Validation**:
- token_hash: SHA256(token) for secure comparison (64 hex characters)
- expires_at > created_at
- revoked_at: NULL or >= created_at

**Session Management**:
- Users can view active sessions: `SELECT * FROM refresh_tokens WHERE user_id = ? AND revoked_at IS NULL`
- Users can revoke specific sessions: `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = ? AND user_id = ?`

**Cleanup**:
- Scheduled job (cron or asynq): delete rows where `revoked_at IS NOT NULL AND created_at < NOW() - INTERVAL '7 days'`
- Scheduled job: delete rows where `expires_at < NOW()`

**Indexes**:
- `UNIQUE INDEX idx_refresh_tokens_token_hash` — validate token (MUST be unique for security)
- `INDEX idx_refresh_tokens_user_id` — find user's active sessions
- `INDEX idx_refresh_tokens_expires_at` — cleanup job
- `INDEX idx_refresh_tokens_revoked_at` — find active vs revoked

---

### 8. audit_logs

**Purpose**: Immutable record of all user actions; non-repudiation and compliance.

```sql
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  action VARCHAR(100) NOT NULL,
  resource VARCHAR(100) NOT NULL,
  resource_id VARCHAR(255),
  before JSONB,
  after JSONB,
  ip_address INET,
  user_agent TEXT,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK | Unique audit log ID |
| `actor_id` | UUID | FK users, NOT NULL | WHO performed the action; ON DELETE RESTRICT = prevent user deletion if audited |
| `action` | VARCHAR(100) | NOT NULL | WHAT action (e.g., `create`, `update`, `delete`, `login`, `permission_grant`) |
| `resource` | VARCHAR(100) | NOT NULL | WHAT resource type (e.g., `invoice`, `user`, `role`, `permission`) |
| `resource_id` | VARCHAR(255) | NULL | WHICH resource instance (UUID or natural key) |
| `before` | JSONB | NULL | State before change (nullable for creation) |
| `after` | JSONB | NULL | State after change (nullable for deletion) |
| `ip_address` | INET | NULL | Client IP (extracted from X-Forwarded-For or socket) |
| `user_agent` | TEXT | NULL | Client User-Agent header |
| `created_at` | TIMESTAMPTZ | NOT NULL | When action occurred |

**Immutability Enforcement**:
- **Database Level**: Revoke UPDATE and DELETE privileges from application database user for audit_logs table
- **Application Level**: Repository only exposes Create() and Find() methods, no Update() or Delete()
- **Recommended**: Add database trigger to prevent any UPDATE or DELETE operations:
```sql
CREATE OR REPLACE FUNCTION prevent_audit_log_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs table is immutable: no UPDATE or DELETE allowed';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER prevent_audit_log_modification
BEFORE UPDATE OR DELETE ON audit_logs
FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();
```

**IP Address Capture**:
- Extract from `X-Forwarded-For` header if present (first IP in chain)
- Fallback to `X-Real-IP` header
- Fallback to `echo.Context().RealIP()` (remote socket address)
- Store as INET type for efficient querying and indexing

**Indexes**:
- `INDEX idx_audit_logs_actor_id` — find user's actions
- `INDEX idx_audit_logs_resource` — find actions on resource type
- `INDEX idx_audit_logs_created_at` — time-range queries (compliance reviews)
- `INDEX idx_audit_logs_action` — find specific action types

---

### 9. invoices (Example Domain)

**Purpose**: Example domain entity demonstrating full auth + permission + scope checking pattern.

```sql
CREATE TABLE invoices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
  status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sent', 'paid', 'cancelled')),
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ DEFAULT NULL
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK | Unique invoice ID |
| `user_id` | UUID | FK users, NOT NULL | Invoice owner; ON DELETE RESTRICT = prevent user deletion if invoices exist |
| `amount` | NUMERIC(12, 2) | NOT NULL, >0 | Invoice amount in cents (store as integer in production) |
| `status` | VARCHAR(50) | NOT NULL, DEFAULT 'draft' | Workflow state (draft → sent → paid or cancelled) |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last modification |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete |

**Authorization Pattern**:
1. **Coarse (Casbin)**: User has `invoice:read` permission (can see invoices)?
2. **Fine (Service)**: User owns this invoice or has `invoice:read:all` permission (can see THIS invoice)?

**Indexes**:
- `INDEX idx_invoices_user_id` — find user's invoices
- `INDEX idx_invoices_status` — find by status
- `INDEX idx_invoices_deleted_at` — soft delete queries

---

## Relationships Summary

```
users (1) ──────────< (M) user_roles ──────────> (1) roles
users (1) ──────────< (M) user_permissions ────> (1) permissions
roles (1) ──────────< (M) role_permissions ────> (1) permissions

users (1) ──────────< (M) refresh_tokens (revocation)
users (1) ──────────< (M) audit_logs (as actor_id)
users (1) ──────────< (M) invoices (ownership)
```

---

## Validation Rules

| Entity | Field | Rule | Type |
|--------|-------|------|------|
| users | email | Valid email format | format |
| users | email | Globally unique | unique |
| users | password_hash | Non-empty | required |
| roles | name | Alphanumeric + underscore | format |
| roles | name | Globally unique | unique |
| permissions | name | `resource:action` format | format |
| permissions | name | Globally unique | unique |
| permissions | resource | Alphanumeric + underscore | format |
| permissions | action | Alphanumeric + underscore or `*` | format |
| user_permissions | effect | 'allow' or 'deny' only | enum |
| refresh_tokens | expires_at | > created_at | constraint |
| audit_logs | action | Non-empty | required |
| audit_logs | resource | Non-empty | required |
| invoices | amount | > 0 | check constraint |
| invoices | status | In (draft, sent, paid, cancelled) | enum |

---

## Migration Strategy

**Phase 1.1**: Create base tables (users, roles, permissions, pivots)
**Phase 1.2**: Create auth tables (refresh_tokens)
**Phase 1.3**: Create audit table (audit_logs)
**Phase 2**: Add domain tables (invoices, etc.)

Each migration versioned, reversible, idempotent.

---

**Version**: 1.0 | **Created**: 2026-04-22 | **Status**: Phase 1 Design

