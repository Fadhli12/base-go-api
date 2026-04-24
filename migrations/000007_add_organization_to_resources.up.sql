-- Migration: 000007_add_organization_to_resources.up.sql
-- Description: Add organization_id foreign key to resource tables for multi-tenancy
-- Created: 2026-04-24

-- Add organization_id to news table
ALTER TABLE news 
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_news_organization_id ON news(organization_id);

-- Add organization_id to media table
ALTER TABLE media 
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_media_organization_id ON media(organization_id);

-- Add organization_id to api_keys table
ALTER TABLE api_keys 
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_organization_id ON api_keys(organization_id);

COMMENT ON COLUMN news.organization_id IS 'Organization scope (NULL = global resource)';
COMMENT ON COLUMN media.organization_id IS 'Organization scope (NULL = global resource)';
COMMENT ON COLUMN api_keys.organization_id IS 'Organization scope (NULL = personal API key)';