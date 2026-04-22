-- Migration: 000001_init.down.sql
-- Description: Rollback base schema
-- Created: 2026-04-22

-- Drop triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
DROP TRIGGER IF EXISTS update_permissions_updated_at ON permissions;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order (respecting foreign key dependencies)
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

-- Drop UUID extension (only if we created it and no other tables depend on it)
-- Note: Comment out if other schemas in the same database use pgcrypto
-- DROP EXTENSION IF EXISTS "pgcrypto";
