CREATE TABLE IF NOT EXISTS media_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id        UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    filename        VARCHAR(255) NOT NULL,
    original_filename VARCHAR(500) NOT NULL,
    mime_type       VARCHAR(100) NOT NULL,
    size            BIGINT NOT NULL,
    file_path       VARCHAR(2000) NOT NULL,
    checksum        CHAR(64) NOT NULL,
    uploaded_by_id  UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at      TIMESTAMPTZ DEFAULT NULL,

    CONSTRAINT uq_media_versions_version UNIQUE (media_id, version),
    CONSTRAINT chk_media_versions_size CHECK (size > 0),
    CONSTRAINT chk_media_versions_version_positive CHECK (version > 0),
    CONSTRAINT chk_media_versions_mime_type CHECK (mime_type ~ '^[a-z]+/[a-zA-Z0-9+\-\.]+$')
);

CREATE INDEX IF NOT EXISTS idx_media_versions_media_id ON media_versions(media_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_versions_uploaded_by ON media_versions(uploaded_by_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_versions_checksum ON media_versions(media_id, checksum) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_versions_created_at ON media_versions(created_at DESC) WHERE deleted_at IS NULL;

CREATE TRIGGER update_media_versions_updated_at BEFORE UPDATE ON media_versions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

ALTER TABLE media ADD COLUMN current_version INTEGER NOT NULL DEFAULT 1;