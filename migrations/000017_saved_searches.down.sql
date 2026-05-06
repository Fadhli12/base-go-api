-- Migration: 000017_saved_searches.down.sql
-- Description: Rollback saved_searches table
-- Created: 2026-05-06

-- Drop trigger first (reverse order of creation)
DROP TRIGGER IF EXISTS update_saved_searches_updated_at ON saved_searches;

-- Drop indexes
DROP INDEX IF EXISTS idx_saved_searches_user_id;
DROP INDEX IF EXISTS idx_saved_searches_deleted_at;

-- Drop table
DROP TABLE IF EXISTS saved_searches;
