-- +goose Up
CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID,
    author_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    commentable_type VARCHAR(50) NOT NULL,
    commentable_id UUID NOT NULL,
    content TEXT NOT NULL,
    mentioned_user_ids JSONB,
    edited_at TIMESTAMPTZ,
    is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Indexes
CREATE INDEX idx_comments_parent_id ON comments(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_comments_author_id ON comments(author_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_comments_organization_id ON comments(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_comments_commentable ON comments(commentable_type, commentable_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_comments_deleted_at ON comments(deleted_at);

-- Foreign keys
ALTER TABLE comments ADD CONSTRAINT fk_comments_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE comments ADD CONSTRAINT fk_comments_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE comments ADD CONSTRAINT fk_comments_parent FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE;

-- Auto-update trigger
CREATE TRIGGER update_comments_updated_at
    BEFORE UPDATE ON comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();