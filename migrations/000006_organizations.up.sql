-- Migration: 000006_organizations.up.sql
-- Description: Create organizations and organization_members tables
-- Created: 2026-04-24

-- Create organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    settings JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create organization_members table
CREATE TABLE IF NOT EXISTS organization_members (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    joined_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (organization_id, user_id)
);

-- Create indexes for organizations table
CREATE INDEX IF NOT EXISTS idx_organizations_slug ON organizations(slug);
CREATE INDEX IF NOT EXISTS idx_organizations_owner_id ON organizations(owner_id);
CREATE INDEX IF NOT EXISTS idx_organizations_deleted_at ON organizations(deleted_at) 
    WHERE deleted_at IS NOT NULL;

-- Create indexes for organization_members table
CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members(user_id);

-- Add trigger for organizations updated_at
CREATE TRIGGER update_organizations_updated_at 
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE organizations IS 'Organizations for multi-tenancy support';
COMMENT ON COLUMN organizations.id IS 'Unique organization identifier';
COMMENT ON COLUMN organizations.name IS 'Organization display name';
COMMENT ON COLUMN organizations.slug IS 'URL-friendly organization identifier';
COMMENT ON COLUMN organizations.owner_id IS 'User who owns the organization';
COMMENT ON COLUMN organizations.settings IS 'Organization configuration as JSONB';
COMMENT ON COLUMN organizations.deleted_at IS 'Soft delete timestamp';

COMMENT ON TABLE organization_members IS 'User membership in organizations';
COMMENT ON COLUMN organization_members.organization_id IS 'Reference to organization';
COMMENT ON COLUMN organization_members.user_id IS 'Reference to user member';
COMMENT ON COLUMN organization_members.role IS 'Member role: owner, admin, member';
COMMENT ON COLUMN organization_members.joined_at IS 'Timestamp when user joined';