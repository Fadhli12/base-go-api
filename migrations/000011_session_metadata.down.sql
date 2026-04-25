-- Rollback MED-005: Remove session management metadata

-- Drop indexes
DROP INDEX IF EXISTS idx_refresh_tokens_user_active;

-- Remove session metadata columns
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS device_name;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS ip_address;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS user_agent;