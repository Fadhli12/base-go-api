-- Rollback: 000015_two_factor_auth.down.sql
-- Description: Remove two-factor authentication columns and recovery codes table

DROP INDEX IF EXISTS idx_two_factor_recovery_codes_used_at;
DROP INDEX IF EXISTS idx_two_factor_recovery_codes_user_id;

DROP TABLE IF EXISTS two_factor_recovery_codes;

ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS is_2fa_pending;

ALTER TABLE users DROP COLUMN IF EXISTS two_factor_verified_at;
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_status;
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_secret;
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_enabled;