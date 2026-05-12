-- +goose Up
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(120) NOT NULL,
    color VARCHAR(7),
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Partial unique indexes: allow name/slug reuse after soft delete
CREATE UNIQUE INDEX idx_tags_name_org_active ON tags (organization_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_tags_slug_org_active ON tags (organization_id, slug) WHERE deleted_at IS NULL;

-- Standard indexes
CREATE INDEX idx_tags_organization_id ON tags (organization_id);
CREATE INDEX idx_tags_deleted_at ON tags (deleted_at);
CREATE INDEX idx_tags_usage_count ON tags (usage_count DESC);

-- Foreign keys
ALTER TABLE tags ADD CONSTRAINT fk_tags_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Auto-update trigger
CREATE TRIGGER update_tags_updated_at
    BEFORE UPDATE ON tags
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Entity tags join table
CREATE TABLE entity_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    tag_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Composite unique constraint: one tag per entity per org
CREATE UNIQUE INDEX idx_entity_tags_unique ON entity_tags (organization_id, entity_type, entity_id, tag_id);

-- Indexes for common queries
CREATE INDEX idx_entity_tags_entity ON entity_tags (entity_type, entity_id);
CREATE INDEX idx_entity_tags_tag_id ON entity_tags (tag_id);
CREATE INDEX idx_entity_tags_organization_id ON entity_tags (organization_id);
CREATE INDEX idx_entity_tags_created_by ON entity_tags (created_by);

-- Foreign keys
ALTER TABLE entity_tags ADD CONSTRAINT fk_entity_tags_tag FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE;
ALTER TABLE entity_tags ADD CONSTRAINT fk_entity_tags_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE entity_tags ADD CONSTRAINT fk_entity_tags_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;