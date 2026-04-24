-- Migration: 000007_add_organization_to_resources.down.sql
-- Description: Remove organization_id from resource tables
-- Created: 2026-04-24

-- Remove organization_id from api_keys
DROP INDEX IF EXISTS idx_api_keys_organization_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS organization_id;

-- Remove organization_id from media
DROP INDEX IF EXISTS idx_media_organization_id;
ALTER TABLE media DROP COLUMN IF EXISTS organization_id;

-- Remove organization_id from news
DROP INDEX IF EXISTS idx_news_organization_id;
ALTER TABLE news DROP COLUMN IF EXISTS organization_id;