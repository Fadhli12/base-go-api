-- Migration: 000004_create_media_tables.down.sql
-- Description: Rollback media library tables
-- Created: 2026-04-22

-- Drop triggers first
DROP TRIGGER IF EXISTS update_media_updated_at ON media;

-- Drop indexes for media_downloads
DROP INDEX IF EXISTS idx_downloads_user;
DROP INDEX IF EXISTS idx_downloads_media;

-- Drop indexes for media_conversions
DROP INDEX IF EXISTS idx_media_conversions_created;
DROP INDEX IF EXISTS idx_media_conversions_media;

-- Drop indexes for media
DROP INDEX IF EXISTS idx_media_model_collection;
DROP INDEX IF EXISTS idx_media_filename_disk;
DROP INDEX IF EXISTS idx_media_orphaned;
DROP INDEX IF EXISTS idx_media_mime_type;
DROP INDEX IF EXISTS idx_media_uploaded_by;
DROP INDEX IF EXISTS idx_media_created_at;
DROP INDEX IF EXISTS idx_media_collection;
DROP INDEX IF EXISTS idx_media_model;

-- Drop tables in dependency order (child tables first)
DROP TABLE IF EXISTS media_downloads;
DROP TABLE IF EXISTS media_conversions;
DROP TABLE IF EXISTS media;
