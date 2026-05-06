-- Migration: 000017_saved_searches.up.sql
-- Description: Create saved_searches table for user saved search queries
-- Created: 2026-05-06

-- Create saved_searches table
CREATE TABLE IF NOT EXISTS saved_searches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    query_text TEXT NOT NULL,
    filters JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create indexes for saved_searches table
CREATE INDEX IF NOT EXISTS idx_saved_searches_user_id ON saved_searches(user_id);
CREATE INDEX IF NOT EXISTS idx_saved_searches_deleted_at ON saved_searches(deleted_at)
    WHERE deleted_at IS NOT NULL;

-- Add trigger for saved_searches updated_at
CREATE TRIGGER update_saved_searches_updated_at
    BEFORE UPDATE ON saved_searches
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE saved_searches IS 'User saved search queries';
COMMENT ON COLUMN saved_searches.id IS 'Unique saved search identifier';
COMMENT ON COLUMN saved_searches.user_id IS 'User who owns this saved search';
COMMENT ON COLUMN saved_searches.name IS 'Display name for the saved search';
COMMENT ON COLUMN saved_searches.query_text IS 'The search query text';
COMMENT ON COLUMN saved_searches.filters IS 'Additional search filters as JSONB';
COMMENT ON COLUMN saved_searches.deleted_at IS 'Soft delete timestamp';