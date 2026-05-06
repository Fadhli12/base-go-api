-- Add is_2fa_pending column for 2FA authentication flow tracking
ALTER TABLE refresh_tokens ADD COLUMN is_2fa_pending BOOLEAN NOT NULL DEFAULT false;