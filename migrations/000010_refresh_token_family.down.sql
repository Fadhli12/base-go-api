-- Rollback MED-004: Remove token family tracking

-- Drop the family_id index
DROP INDEX IF EXISTS idx_refresh_tokens_family_id;

-- Remove the family_id column
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS family_id;