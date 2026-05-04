-- Migration: 000013_settings.up.sql
-- Description: Create user_settings and system_settings tables with JSONB flexibility
-- Created: 2026-05-02

-- User settings table (org-scoped, user preferences)
-- No soft delete - settings are immutable configuration
CREATE TABLE IF NOT EXISTS user_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Settings as JSONB for flexibility
    -- Structure: {theme, language, timezone, notifications_enabled, email_digest_enabled}
    settings JSONB NOT NULL DEFAULT '{}',

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,

    -- Unique constraint: one preference record per user per organization
    CONSTRAINT uq_user_settings_org_user UNIQUE (organization_id, user_id)
);

-- System settings table (org-wide configuration, admin-controlled)
CREATE TABLE IF NOT EXISTS system_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Settings as JSONB: {app_name, logo_url, maintenance_mode, rate_limits, email_config, ...}
    settings JSONB NOT NULL DEFAULT '{}',

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,

    -- Unique constraint: one system settings record per organization
    CONSTRAINT uq_system_settings_org UNIQUE (organization_id)
);

-- Indexes for user_settings
CREATE INDEX IF NOT EXISTS idx_user_settings_org_id ON user_settings(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_settings_user_id ON user_settings(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_settings_org_user ON user_settings(organization_id, user_id) WHERE deleted_at IS NULL;

-- Indexes for system_settings
CREATE INDEX IF NOT EXISTS idx_system_settings_org_id ON system_settings(organization_id) WHERE deleted_at IS NULL;

-- Triggers for updated_at column
CREATE TRIGGER update_user_settings_updated_at
    BEFORE UPDATE ON user_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_system_settings_updated_at
    BEFORE UPDATE ON system_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Documentation comments
COMMENT ON TABLE user_settings IS 'Per-user organization-scoped preferences with JSONB flexibility for themes, localization, and notification settings';
COMMENT ON COLUMN user_settings.id IS 'Unique user settings identifier';
COMMENT ON COLUMN user_settings.organization_id IS 'Organization this user belongs to';
COMMENT ON COLUMN user_settings.user_id IS 'User who owns these settings';
COMMENT ON COLUMN user_settings.settings IS 'JSONB document: {theme, language, timezone, notifications_enabled, email_digest_enabled}';
COMMENT ON COLUMN user_settings.deleted_at IS 'Soft delete timestamp (NULL = active)';

COMMENT ON TABLE system_settings IS 'Organization-wide system configuration (admin-only) with JSONB flexibility for branding, features, and defaults';
COMMENT ON COLUMN system_settings.id IS 'Unique system settings identifier';
COMMENT ON COLUMN system_settings.organization_id IS 'Organization these settings apply to';
COMMENT ON COLUMN system_settings.settings IS 'JSONB document: {app_name, logo_url, maintenance_mode, rate_limits, email_config, ...}';
COMMENT ON COLUMN system_settings.deleted_at IS 'Soft delete timestamp (NULL = active)';
