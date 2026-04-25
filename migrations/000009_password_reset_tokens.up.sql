-- Migration: 000009_password_reset_tokens.up.sql
-- Description: Create password_reset_tokens table for secure password recovery
-- Created: 2026-04-28
-- Issue: CRIT-002 - Password reset tokens must be persisted for validation

-- Create password_reset_tokens table
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_used_at ON password_reset_tokens(used_at) WHERE used_at IS NOT NULL;

-- Add comment for documentation
COMMENT ON TABLE password_reset_tokens IS 'Stores hashed password reset tokens for secure password recovery. Tokens are stored as SHA256 hashes and expire after a configurable duration.';

-- Optional: Create a partial index for finding unused tokens
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_unused ON password_reset_tokens(user_id, expires_at) 
    WHERE used_at IS NULL;