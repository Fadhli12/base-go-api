-- Migration: 000016_add_news_search.up.sql
-- Description: Add tsvector generated column with GIN index for full-text search on news
-- Created: 2026-05-06

-- Add generated tsvector column for full-text search
ALTER TABLE news ADD COLUMN IF NOT EXISTS search_vector TSVECTOR
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(content, '')), 'B')
    ) STORED;

-- Create GIN index for fast full-text search queries
CREATE INDEX IF NOT EXISTS idx_news_search_vector ON news USING GIN (search_vector);

-- Create index for common search patterns (title only, content only)
CREATE INDEX IF NOT EXISTS idx_news_title_search ON news USING GIN (to_tsvector('english', coalesce(title, '')));