# Database Migrations Guide

This document provides a comprehensive guide for managing database migrations in the Go API Base project.

## Overview

The project uses **golang-migrate** for database schema version control. All migrations are SQL-based files stored in the `migrations/` directory. This approach ensures:

- **Version control** - Every schema change is tracked
- **Rollback capability** - Changes can be reversed safely
- **Team collaboration** - Consistent schema across all environments
- **Audit trail** - Clear history of database evolution

> **⚠️ Important:** Never use GORM's `AutoMigrate` in production. It can cause silent schema drift and data loss.

## Quick Start

### Running Migrations

```bash
# Apply all pending migrations
make migrate

# Or using migrate CLI directly
migrate -path ./migrations -database "${DATABASE_URL}" up

# Apply one migration at a time
migrate -path ./migrations -database "${DATABASE_URL}" up 1
```

### Rolling Back

```bash
# Rollback the last migration
make migrate-down

# Or using migrate CLI
migrate -path ./migrations -database "${DATABASE_URL}" down 1

# Rollback all migrations (destructive!)
migrate -path ./migrations -database "${DATABASE_URL}" down -all
```

### Verifying Migration Status

```bash
# Check current version
migrate -path ./migrations -database "${DATABASE_URL}" version

# Force a specific version (use with caution)
migrate -path ./migrations -database "${DATABASE_URL}" force 000005
```

## Migration Files

### Naming Convention

```
migrations/
├── 000001_init.up.sql           # Initial schema
├── 000001_init.down.sql         # Initial rollback
├── 000002_audit_logs.up.sql     # Audit logging
├── 000002_audit_logs.down.sql
├── 000003_create_news_table.up.sql
├── 000003_create_news_table.down.sql
├── 000004_create_media_tables.up.sql
├── 000004_create_media_tables.down.sql
├── 000005_api_keys.up.sql       # API key authentication
└── 000005_api_keys.down.sql
```

**Format:** `{NNNNNN}_{description}.{direction}.sql`

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `NNNNNN` | 6-digit zero-padded sequence | `000001`, `000005` |
| `description` | Snake_case description | `create_news_table` |
| `direction` | `up` (apply) or `down` (rollback) | `up.sql`, `down.sql` |

### File Structure

Every migration must have **BOTH** up and down files:

```bash
# ✅ Correct - both files present
000005_api_keys.up.sql
000005_api_keys.down.sql

# ❌ Wrong - missing down file
000006_add_column.up.sql
# Missing: 000006_add_column.down.sql
```

## Current Schema

### Migration History

| Version | Name | Tables Created | Date |
|---------|------|----------------|------|
| 000001 | init | users, roles, permissions, user_roles, role_permissions, user_permissions, refresh_tokens | 2026-04-22 |
| 000002 | audit_logs | audit_logs | 2026-04-22 |
| 000003 | create_news_table | news | 2026-04-22 |
| 000004 | create_media_tables | media, media_conversions, media_downloads | 2026-04-22 |
| 000005 | api_keys | api_keys | 2026-04-23 |

### Table Inventory

| Table | Type | Soft Delete | Primary Key | Description |
|-------|------|-------------|-------------|-------------|
| `users` | Entity | ✅ Yes | UUID | User accounts |
| `roles` | Entity | ✅ Yes | UUID | Role definitions |
| `permissions` | Entity | ✅ Yes | UUID | Permission definitions |
| `user_roles` | Junction | ❌ No | Composite (user_id, role_id) | User-role assignments |
| `role_permissions` | Junction | ❌ No | Composite (role_id, permission_id) | Role-permission grants |
| `user_permissions` | Junction+Effect | ❌ No | UUID | Direct user permissions |
| `refresh_tokens` | Security | ❌ No | UUID | JWT refresh tokens |
| `audit_logs` | Immutable | ❌ No | UUID | Audit trail (immutable) |
| `news` | Entity | ✅ Yes | UUID | News articles |
| `media` | Entity | ✅ Yes | UUID | Uploaded files |
| `media_conversions` | Dependent | ❌ No | UUID | Media variants (thumbnails, etc.) |
| `media_downloads` | Junction | ❌ No | UUID | Download tracking |
| `api_keys` | Security | ✅ Yes | UUID | API key authentication |

## Creating New Migrations

### Step 1: Determine Version Number

Check existing migrations to find the next number:

```bash
ls -1 migrations/*.up.sql | tail -1
# Output: migrations/000005_api_keys.up.sql
# Next: 000006
```

### Step 2: Create Migration Files

```bash
# Create both files
touch migrations/000006_add_feature.up.sql
touch migrations/000006_add_feature.down.sql
```

### Step 3: Write the UP Migration

Example: Adding a new column

```sql
-- Migration: 000006_add_feature.up.sql
-- Description: Add feature flag column to users table
-- Created: 2026-04-24

-- Add column with default to avoid breaking existing rows
ALTER TABLE users ADD COLUMN IF NOT EXISTS feature_enabled BOOLEAN DEFAULT FALSE NOT NULL;

-- Add index for queries filtering by feature
CREATE INDEX IF NOT EXISTS idx_users_feature_enabled ON users(feature_enabled) WHERE feature_enabled = TRUE;

-- Add comment for documentation
COMMENT ON COLUMN users.feature_enabled IS 'Feature flag for beta access';
```

### Step 4: Write the DOWN Migration

Example: Rolling back the column

```sql
-- Migration: 000006_add_feature.down.sql
-- Description: Remove feature flag column from users table
-- Created: 2026-04-24

-- Drop index first
DROP INDEX IF EXISTS idx_users_feature_enabled;

-- Drop column
ALTER TABLE users DROP COLUMN IF EXISTS feature_enabled;
```

### Step 5: Test Locally

```bash
# Apply the migration
make migrate

# Verify the change
psql -d go_api_base -c "\d users"

# Test rollback
make migrate-down

# Verify rollback
psql -d go_api_base -c "\d users"

# Re-apply to restore state
make migrate
```

## Schema Patterns

### UUID Primary Keys

**All entity tables use UUID primary keys:**

```sql
-- Enable pgcrypto extension (required once)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Use gen_random_uuid() for automatic UUID generation
id UUID PRIMARY KEY DEFAULT gen_random_uuid()
```

### Soft Deletes

**Applied to entity tables (NOT junction/audit tables):**

```sql
-- Add deleted_at column
deleted_at TIMESTAMPTZ DEFAULT NULL

-- Create partial index for efficient soft-delete queries
CREATE INDEX idx_users_deleted_at ON users(deleted_at) 
    WHERE deleted_at IS NOT NULL;

-- Create composite index for active record queries
CREATE INDEX idx_users_active ON users(id, deleted_at) 
    WHERE deleted_at IS NULL;
```

### Timestamps

**Every entity table has created_at and updated_at:**

```sql
created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL

-- Auto-update trigger (requires update_updated_at_column function)
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Foreign Keys

**Use appropriate cascade strategy:**

```sql
-- CASCADE: Delete child records when parent is deleted
-- Use for: tokens, dependent data, junction tables
user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE

-- RESTRICT: Prevent parent deletion if referenced
-- Use for: audit logs, critical relationships
actor_id UUID REFERENCES users(id) ON DELETE RESTRICT

-- SET NULL: Clear reference but keep child record
-- Use for: optional relationships, nullable foreign keys
downloaded_by_id UUID REFERENCES users(id) ON DELETE SET NULL
```

| Strategy | Use Case | Example |
|----------|----------|---------|
| `CASCADE` | Dependent records | `api_keys.user_id`, `media.media_id` |
| `RESTRICT` | Audit integrity | `audit_logs.actor_id` |
| `SET NULL` | Optional links | `media.downloaded_by_id` |

### Indexes

**Index patterns for common queries:**

```sql
-- Single column index
CREATE INDEX idx_users_email ON users(email);

-- Partial index (soft delete, status flags)
CREATE INDEX idx_users_deleted_at ON users(deleted_at) 
    WHERE deleted_at IS NOT NULL;

-- Composite index (common query patterns)
CREATE INDEX idx_audit_logs_resource_created_at 
    ON audit_logs(resource, created_at);

-- Expression index (case-insensitive search)
CREATE INDEX idx_users_email_lower ON users(LOWER(email));
```

### CHECK Constraints

**Enforce data integrity at database level:**

```sql
-- Enum-like constraints
status VARCHAR(20) NOT NULL DEFAULT 'draft' 
    CHECK (status IN ('draft', 'published', 'archived')),

-- Format constraints
prefix VARCHAR(12) NOT NULL 
    CHECK (prefix ~ '^ak_[a-z]+_[a-z0-9]+$'),

-- Value constraints
size BIGINT NOT NULL CHECK (size > 0),

-- Combined constraints
mime_type VARCHAR(100) NOT NULL 
    CHECK (mime_type ~ '^[a-z]+/[a-zA-Z0-9+\-\.]+$')
```

### Triggers

**Common trigger patterns:**

```sql
-- 1. Auto-update timestamps (required for all entity tables)
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Audit logging on insert
CREATE TRIGGER trigger_api_key_creation
    AFTER INSERT ON api_keys
    FOR EACH ROW EXECUTE FUNCTION audit_api_key_creation();

-- 3. Enforce immutability
CREATE TRIGGER audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();
```

## Troubleshooting

### Common Errors

#### 1. "Dirty Database Version"

**Error:**
```
error: Dirty database version 000005. Fix and force version.
```

**Cause:** Migration failed midway, leaving database in inconsistent state.

**Solution:**
```bash
# 1. Manually fix the failed migration in database
psql -d go_api_base -c "DROP TABLE IF EXISTS incomplete_table;"

# 2. Force version to last successful migration
migrate -path ./migrations -database "${DATABASE_URL}" force 000004

# 3. Re-apply migrations
make migrate
```

#### 2. "Relation Already Exists"

**Error:**
```
error: relation "users" already exists
```

**Cause:** Running migrations on a database that already has the schema.

**Solution:**
```bash
# Check current version
migrate -path ./migrations -database "${DATABASE_URL}" version

# If this returns "000001" or higher, migrations already applied
# For fresh start:
psql -d go_api_base -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
migrate -path ./migrations -database "${DATABASE_URL}" up
```

#### 3. "Foreign Key Violation"

**Error:**
```
error: insert or update on table "api_keys" violates foreign key constraint
```

**Cause:** Referenced row doesn't exist (e.g., user_id not found in users table).

**Solution:**
```bash
# Ensure seed data exists before creating related records
go run ./cmd/api/main.go seed

# Or insert required parent records first
psql -d go_api_base -c "INSERT INTO users (id, email, password_hash) VALUES (...);"
```

#### 4. "Permission Denied" (Windows PostgreSQL)

**Error:**
```
error: FATAL: password authentication failed for user "postgres"
```

**Cause:** PostgreSQL 17 uses scram-sha-256 by default, but `.env` may not match.

**Solution:**
```powershell
# Option 1: Set environment variables matching pg_hba.conf
$env:PGPASSWORD="your_password"
$env:DATABASE_URL="postgres://postgres:your_password@localhost:5432/go_api_base?sslmode=disable"

# Option 2: Modify pg_hba.conf for development (less secure)
# Edit: C:\Program Files\PostgreSQL\17\data\pg_hba.conf
# Change: host all all 127.0.0.1/32 scram-sha-256
# To:     host all all 127.0.0.1/32 trust
# Then reload: pg_ctl reload -D "C:\Program Files\PostgreSQL\17\data"
```

### Manual Migration (Emergency)

If `migrate` tool fails, run SQL directly:

```bash
# Apply migration directly
psql -d go_api_base -f migrations/000005_api_keys.up.sql

# Rollback manually
psql -d go_api_base -f migrations/000005_api_keys.down.sql

# Update version manually (if needed)
psql -d go_api_base -c "UPDATE schema_migrations SET version = 000005;"
```

### Checking Migration State

```sql
-- Check applied migrations
SELECT * FROM schema_migrations ORDER BY version;

-- Check if function exists
SELECT proname FROM pg_proc WHERE proname = 'update_updated_at_column';

-- Check if extension exists
SELECT extname FROM pg_extension WHERE extname = 'pgcrypto';

-- List all tables
\dt

-- Describe a table
\d api_keys
```

## Best Practices

### DO ✅

1. **Always create both up and down files**
   - Every migration must be reversible
   - Test rollback locally before committing

2. **Use incremental version numbers**
   - Never skip a number (000001 → 000002 → 000003)
   - Never reuse a number

3. **Add indexes for foreign keys**
   ```sql
   CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
   ```

4. **Use partial indexes for conditional queries**
   ```sql
   CREATE INDEX idx_api_keys_active ON api_keys(user_id, deleted_at, revoked_at)
       WHERE deleted_at IS NULL AND revoked_at IS NULL;
   ```

5. **Include CHECK constraints for data validation**
   ```sql
   CONSTRAINT chk_key_hash_length CHECK (LENGTH(key_hash) >= 60)
   ```

6. **Add comments for documentation**
   ```sql
   COMMENT ON COLUMN api_keys.prefix IS 'First 12 chars of key for identification';
   ```

7. **Test migrations on a copy of production data**
   ```bash
   # Create test database
   createdb go_api_test
   pg_dump go_api_base | psql go_api_test
   migrate -path ./migrations -database "postgres://localhost/go_api_test" up
   ```

### DON'T ❌

1. **Never use GORM AutoMigrate in production**
   ```go
   // ❌ Never do this in production
   db.AutoMigrate(&User{}, &Role{})
   ```

2. **Never edit applied migrations**
   ```bash
   # ❌ Wrong: Editing 000001_init.up.sql
   # ✅ Correct: Create 000006_fix_schema.up.sql
   ```

3. **Never skip migration numbers**
   ```bash
   # ❌ Wrong: 000001, 000003 (missing 000002)
   # ✅ Correct: 000001, 000002, 000003
   ```

4. **Never use hard deletes in entity tables**
   ```sql
   -- ❌ Wrong
   DELETE FROM users WHERE id = '...';

   -- ✅ Correct
   UPDATE users SET deleted_at = NOW() WHERE id = '...';
   ```

5. **Never forget foreign key cascade rules**
   ```sql
   -- ❌ Wrong: No cascade specified
   user_id UUID REFERENCES users(id)

   -- ✅ Correct: Explicit cascade strategy
   user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
   ```

6. **Never create a migration without testing rollback**
   ```bash
   # ❌ Wrong: Apply and forget
   make migrate

   # ✅ Correct: Test both directions
   make migrate
   make migrate-down
   make migrate
   ```

## Migration Checklist

Before creating a new migration:

- [ ] Determine next version number (check existing migrations)
- [ ] Create both `.up.sql` and `.down.sql` files
- [ ] Use `IF NOT EXISTS` / `IF EXISTS` for idempotency
- [ ] Add appropriate indexes for new columns/tables
- [ ] Include foreign key constraints with cascade rules
- [ ] Add soft delete column for entity tables
- [ ] Add `updated_at` trigger for entity tables
- [ ] Test `up` migration locally
- [ ] Test `down` migration locally
- [ ] Verify rollback doesn't lose data
- [ ] Add comments for complex schema decisions
- [ ] Update this documentation if adding new patterns

## Production Checklist

Before applying migrations to production:

- [ ] Backup database
- [ ] Test on staging environment
- [ ] Verify migration is reversible
- [ ] Schedule during low-traffic window
- [ ] Monitor application logs after apply
- [ ] Verify indexes are created (check query plans)
- [ ] Run `ANALYZE` after large data modifications

## Additional Resources

- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Migration Best Practices](https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md)
- [Project AGENTS.md](migrations/AGENTS.md) - Internal conventions

## Support

For issues with migrations:

1. Check this documentation
2. Review `migrations/AGENTS.md` for project-specific conventions
3. Run `psql` commands to inspect database state
4. Check migration version with `migrate version`
5. Review error logs for specific error messages