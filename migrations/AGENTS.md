# Migrations - golang-migrate SQL Files

**Location:** migrations/ | **Purpose:** Versioned schema migrations

---

## OVERVIEW

SQL-based migrations using golang-migrate. Never use GORM AutoMigrate in production.

## FILES

```
migrations/
├── 000001_init.up.sql
├── 000001_init.down.sql
├── 000002_audit_logs.up.sql
├── 000002_audit_logs.down.sql
└── .gitkeep
```

## NAMING CONVENTION

```
{NNNNNN}_{name}.{direction}.sql

NNNNNN   = 6-digit zero-padded sequence (000001, 000002)
name     = Descriptive name (init, audit_logs)
direction = up (apply) or down (rollback)
```

## RUNNING

```bash
# Apply all migrations
make migrate
# OR
migrate -path ./migrations -database "${DATABASE_URL}" up

# Rollback one migration
make migrate-down
# OR  
migrate -path ./migrations -database "${DATABASE_URL}" down

# Rollback all
migrate -path ./migrations -database "${DATABASE_URL}" down -all
```

## SCHEMA PATTERNS

### UUID Primary Keys

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

id UUID PRIMARY KEY DEFAULT gen_random_uuid()
```

Applied to: ALL entity tables (users, roles, permissions, etc.)

### Soft Deletes

```sql
deleted_at TIMESTAMPTZ DEFAULT NULL

-- Partial index for soft-delete queries
CREATE INDEX idx_users_deleted_at ON users(deleted_at) 
    WHERE deleted_at IS NOT NULL;
```

Applied to: users, roles, permissions (NOT junction tables, NOT audit_logs)

### Timestamps

```sql
created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL

-- Auto-update trigger
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Foreign Keys

```sql
-- CASCADE: Delete children when parent deleted
user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE

-- RESTRICT: Prevent deletion of referenced parent
actor_id UUID REFERENCES users(id) ON DELETE RESTRICT
```

| Strategy | Usage |
|----------|-------|
| CASCADE | Dependent records (tokens, junction tables) |
| RESTRICT | Audit integrity (actor_id) |

### Indexes

```sql
-- Single column
CREATE INDEX idx_users_email ON users(email);

-- Partial (soft delete)
CREATE INDEX idx_roles_deleted_at ON roles(deleted_at) 
    WHERE deleted_at IS NOT NULL;

-- Composite (common queries)
CREATE INDEX idx_audit_logs_resource_created_at 
    ON audit_logs(resource, created_at);
```

### CHECK Constraints

```sql
effect VARCHAR(10) NOT NULL CHECK (effect IN ('allow', 'deny'))
status VARCHAR(20) NOT NULL DEFAULT 'draft' 
    CHECK (status IN ('draft', 'pending', 'paid', 'cancelled'))
```

### Triggers

```sql
-- Auto-update timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Enforce immutability
CREATE OR REPLACE FUNCTION prevent_audit_log_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs table is immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();
```

## DOWN ROLLBACK PATTERN

```sql
-- 000001_init.down.sql

-- Drop in reverse dependency order
DROP TRIGGER IF EXISTS update_permissions_updated_at ON permissions;
DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

-- Optional (usually kept)
-- DROP EXTENSION IF EXISTS "pgcrypto";
```

## TABLE INVENTORY

| Table | Type | Soft Delete | Primary Key |
|-------|------|-------------|-------------|
| users | Entity | Yes | UUID |
| roles | Entity | Yes | UUID |
| permissions | Entity | Yes | UUID |
| user_roles | Junction | No | Composite |
| role_permissions | Junction | No | Composite |
| user_permissions | Junction+Effect | No | UUID |
| refresh_tokens | Security | No | UUID |
| audit_logs | Immutable | No | UUID |

## CHECKLIST FOR NEW MIGRATIONS

1. [ ] Use 6-digit sequence number
2. [ ] Create both .up.sql and .down.sql
3. [ ] Include CASCADE constraints where needed
4. [ ] Add indexes for FKs and query columns
5. [ ] Add soft delete field for entities
6. [ ] Add updated_at trigger for entities
7. [ ] Test down migration locally
8. [ ] Verify rollback doesn't lose data

## ANTI-PATTERNS

| Pattern | Reason |
|---------|--------|
| GORM AutoMigrate | Silent schema drift |
| Numbering gaps | Breaks migration chain |
| Missing down file | Can't rollback |
| Hard deletes | Audit trail loss |