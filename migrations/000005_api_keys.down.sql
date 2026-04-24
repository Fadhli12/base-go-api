-- Migration: 000005_api_keys.down.sql
-- Description: Drop api_keys table and related triggers
-- Created: 2026-04-23

-- Drop triggers first
DROP TRIGGER IF EXISTS trigger_api_key_creation ON api_keys;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;

-- Drop functions
DROP FUNCTION IF EXISTS audit_api_key_creation();

-- Drop indexes
DROP INDEX IF EXISTS idx_api_keys_active;
DROP INDEX IF EXISTS idx_api_keys_deleted_at;
DROP INDEX IF EXISTS idx_api_keys_revoked_at;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_key_hash;

-- Drop table
DROP TABLE IF EXISTS api_keys;

-- Note: This will cascade delete all API keys due to foreign key constraint
-- Ensure backup before running in production