-- Rollback: remove is_2fa_pending column
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS is_2fa_pending;