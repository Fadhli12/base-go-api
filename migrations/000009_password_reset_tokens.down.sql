-- Migration: 000009_password_reset_tokens.down.sql
-- Description: Rollback password_reset_tokens table
-- Created: 2026-04-28

-- Drop indexes
DROP INDEX IF EXISTS idx_password_reset_tokens_unused;
DROP INDEX IF EXISTS idx_password_reset_tokens_used_at;
DROP INDEX IF EXISTS idx_password_reset_tokens_expires_at;
DROP INDEX IF EXISTS idx_password_reset_tokens_token_hash;
DROP INDEX IF EXISTS idx_password_reset_tokens_user_id;

-- Drop table
DROP TABLE IF EXISTS password_reset_tokens;