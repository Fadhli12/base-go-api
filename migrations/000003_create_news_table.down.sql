-- Migration: 000003_create_news_table.down.sql
-- Description: Rollback news table creation
-- Created: 2026-04-22

-- Drop trigger
DROP TRIGGER IF EXISTS update_news_updated_at ON news;

-- Drop indexes
DROP INDEX IF EXISTS idx_news_author_status;
DROP INDEX IF EXISTS idx_news_status_created_at;
DROP INDEX IF EXISTS idx_news_deleted_at;
DROP INDEX IF EXISTS idx_news_published_at;
DROP INDEX IF EXISTS idx_news_created_at;
DROP INDEX IF EXISTS idx_news_status;
DROP INDEX IF EXISTS idx_news_slug;
DROP INDEX IF EXISTS idx_news_author_id;

-- Drop table
DROP TABLE IF EXISTS news;
