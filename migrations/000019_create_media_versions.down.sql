DROP TRIGGER IF EXISTS update_media_versions_updated_at ON media_versions;
DROP TABLE IF EXISTS media_versions;
ALTER TABLE media DROP COLUMN IF EXISTS current_version;