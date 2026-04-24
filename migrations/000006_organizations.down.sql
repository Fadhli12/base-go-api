-- Migration: 000006_organizations.down.sql
-- Description: Rollback organizations and organization_members tables
-- Created: 2026-04-24

-- Drop trigger on organizations
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organizations;