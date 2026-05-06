-- Migration: 000015_two_factor_auth.up.sql
-- Description: Add two-factor authentication columns and recovery codes table
-- Created: 2026-05-05

-- Add two-factor authentication columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_secret TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_status VARCHAR(20) NOT NULL DEFAULT 'disabled'
    CHECK (two_factor_status IN ('disabled', 'pending', 'enabled'));
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_verified_at TIMESTAMPTZ;

-- Add is_2fa_pending column to refresh_tokens for 2FA verification state
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS is_2fa_pending BOOLEAN NOT NULL DEFAULT false;

-- Create recovery codes table for two-factor authentication backup codes
CREATE TABLE IF NOT EXISTS two_factor_recovery_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash   TEXT NOT NULL UNIQUE,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMPTZ DEFAULT NULL
);

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_two_factor_recovery_codes_user_id ON two_factor_recovery_codes(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_two_factor_recovery_codes_used_at ON two_factor_recovery_codes(used_at) WHERE used_at IS NULL;

-- Documentation comments
COMMENT ON TABLE two_factor_recovery_codes IS 'Backup recovery codes for two-factor authentication';
COMMENT ON COLUMN users.two_factor_enabled IS 'Whether 2FA is enabled for the user';
COMMENT ON COLUMN users.two_factor_secret IS 'Encrypted TOTP secret (stored encrypted at rest)';
COMMENT ON COLUMN users.two_factor_status IS '2FA status: disabled, pending (awaiting verification), or enabled';
COMMENT ON COLUMN users.two_factor_verified_at IS 'Timestamp when 2FA was verified (NULL when disabled)';
COMMENT ON COLUMN refresh_tokens.is_2fa_pending IS 'Marks token as pending 2FA verification before full authentication completes';