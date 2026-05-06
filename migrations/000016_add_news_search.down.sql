-- Migration: 000016_add_news_search.down.sql
-- Description: Remove tsvector column and GIN indexes from news table
-- Created: 2026-05-06

-- Drop GIN indexes first (dependencies on the column)
DROP INDEX IF EXISTS idx_news_title_search;
DROP INDEX IF EXISTS idx_news_search_vector;

-- Drop the generated tsvector column
ALTER TABLE news DROP COLUMN IF EXISTS search_vector;