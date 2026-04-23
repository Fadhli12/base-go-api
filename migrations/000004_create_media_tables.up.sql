-- Migration: 000004_create_media_tables.up.sql
-- Description: Create media, media_conversions, and media_downloads tables
-- Created: 2026-04-22

-- Create media table
CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_type VARCHAR(255) NOT NULL,
    model_id UUID NOT NULL,
    collection_name VARCHAR(255) NOT NULL DEFAULT 'default',
    disk VARCHAR(50) NOT NULL DEFAULT 'local',
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(500) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    path VARCHAR(2000) NOT NULL,
    metadata JSONB DEFAULT '{}',
    custom_properties JSONB DEFAULT '{}',
    uploaded_by_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    orphaned_at TIMESTAMPTZ DEFAULT NULL,
    
    -- Constraints
    CONSTRAINT check_media_size CHECK (size > 0),
    CONSTRAINT check_media_mime_type CHECK (mime_type ~ '^[a-z]+/[a-zA-Z0-9+\-\.]+$')
);

-- Create media_conversions table
CREATE TABLE IF NOT EXISTS media_conversions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    disk VARCHAR(50) NOT NULL DEFAULT 'local',
    path VARCHAR(2000) NOT NULL,
    size BIGINT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    
    -- Constraints
    CONSTRAINT check_conversion_size CHECK (size >= 0),
    CONSTRAINT unique_media_conversion_name UNIQUE (media_id, name)
);

-- Create media_downloads table
CREATE TABLE IF NOT EXISTS media_downloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    downloaded_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    downloaded_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ip_address VARCHAR(45),
    user_agent VARCHAR(2000)
);

-- Create indexes for media table
CREATE INDEX IF NOT EXISTS idx_media_model ON media(model_type, model_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_collection ON media(collection_name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_uploaded_by ON media(uploaded_by_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_mime_type ON media(mime_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_orphaned ON media(orphaned_at) WHERE orphaned_at IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_filename_disk ON media(disk, filename) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_model_collection ON media(model_type, model_id, collection_name) WHERE deleted_at IS NULL;

-- Create indexes for media_conversions table
CREATE INDEX IF NOT EXISTS idx_media_conversions_media ON media_conversions(media_id);
CREATE INDEX IF NOT EXISTS idx_media_conversions_created ON media_conversions(created_at DESC);

-- Create indexes for media_downloads table
CREATE INDEX IF NOT EXISTS idx_downloads_media ON media_downloads(media_id, downloaded_at DESC);
CREATE INDEX IF NOT EXISTS idx_downloads_user ON media_downloads(downloaded_by_id, downloaded_at DESC);

-- Add trigger for updated_at on media table
CREATE TRIGGER update_media_updated_at BEFORE UPDATE ON media
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
