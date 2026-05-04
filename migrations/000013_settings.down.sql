-- Rollback: 000013_settings.down.sql
-- Description: Drop user_settings and system_settings tables

DROP TRIGGER IF EXISTS update_system_settings_updated_at ON system_settings;
DROP TRIGGER IF EXISTS update_user_settings_updated_at ON user_settings;

DROP INDEX IF EXISTS idx_system_settings_org_id;
DROP INDEX IF EXISTS idx_user_settings_org_user;
DROP INDEX IF EXISTS idx_user_settings_user_id;
DROP INDEX IF EXISTS idx_user_settings_org_id;

DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS user_settings;
