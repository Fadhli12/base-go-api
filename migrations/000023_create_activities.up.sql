-- +goose Up
CREATE TABLE activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID NOT NULL,
    action_type VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(100) NOT NULL,
    organization_id UUID,
    metadata JSONB,
    archived_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_activities_organization_id ON activities(organization_id) WHERE archived_at IS NULL;
CREATE INDEX idx_activities_actor_id ON activities(actor_id) WHERE archived_at IS NULL;
CREATE INDEX idx_activities_resource ON activities(resource_type, resource_id) WHERE archived_at IS NULL;
CREATE INDEX idx_activities_action_type ON activities(action_type) WHERE archived_at IS NULL;
CREATE INDEX idx_activities_created_at ON activities(created_at DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_activities_feed ON activities(organization_id, created_at DESC) WHERE archived_at IS NULL AND deleted_at IS NULL;
CREATE INDEX idx_activities_archived_at ON activities(archived_at) WHERE archived_at IS NOT NULL;
CREATE INDEX idx_activities_deleted_at ON activities(deleted_at);

-- Foreign keys
ALTER TABLE activities ADD CONSTRAINT fk_activities_actor FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE activities ADD CONSTRAINT fk_activities_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Auto-update trigger
CREATE TRIGGER update_activities_updated_at
    BEFORE UPDATE ON activities
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE activity_reads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    activity_id UUID NOT NULL,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE UNIQUE INDEX idx_activity_reads_user_activity ON activity_reads(user_id, activity_id);
CREATE INDEX idx_activity_reads_user_id ON activity_reads(user_id);
CREATE INDEX idx_activity_reads_activity_id ON activity_reads(activity_id);

-- Foreign keys
ALTER TABLE activity_reads ADD CONSTRAINT fk_activity_reads_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE activity_reads ADD CONSTRAINT fk_activity_reads_activity FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE;

CREATE TABLE activity_follows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE UNIQUE INDEX idx_activity_follows_user_resource ON activity_follows(user_id, resource_type, resource_id);
CREATE INDEX idx_activity_follows_user_id ON activity_follows(user_id);
CREATE INDEX idx_activity_follows_resource ON activity_follows(resource_type, resource_id);

-- Foreign keys
ALTER TABLE activity_follows ADD CONSTRAINT fk_activity_follows_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;