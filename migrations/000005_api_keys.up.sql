-- Migration: 000005_api_keys.up.sql
-- Description: Create api_keys table for API key authentication
-- Created: 2026-04-23

-- Create api_keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    prefix VARCHAR(12) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    scopes JSONB DEFAULT '[]',
    expires_at TIMESTAMPTZ DEFAULT NULL,
    last_used_at TIMESTAMPTZ DEFAULT NULL,
    revoked_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    
    -- Constraints
    CONSTRAINT chk_key_hash_length CHECK (LENGTH(key_hash) >= 60),
    CONSTRAINT chk_prefix_format CHECK (prefix ~ '^ak_[a-z]+_[a-z0-9]+$')
);

-- Create unique index on key_hash for fast lookup
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);

-- Create index on user_id for listing user's keys
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);

-- Create partial index on expires_at for expiration checks
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) 
    WHERE expires_at IS NOT NULL;

-- Create partial index on revoked_at for active key queries
CREATE INDEX IF NOT EXISTS idx_api_keys_revoked_at ON api_keys(revoked_at) 
    WHERE revoked_at IS NOT NULL;

-- Create partial index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_api_keys_deleted_at ON api_keys(deleted_at) 
    WHERE deleted_at IS NOT NULL;

-- Create composite index for active key lookup (common query pattern)
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(user_id, deleted_at, revoked_at) 
    WHERE deleted_at IS NULL AND revoked_at IS NULL;

-- Add trigger for updated_at
CREATE TRIGGER update_api_keys_updated_at 
    BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add trigger for audit on key creation
CREATE OR REPLACE FUNCTION audit_api_key_creation()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (
        actor_id,
        action,
        resource,
        resource_id,
        before,
        after,
        created_at
    ) VALUES (
        NEW.user_id,
        'create',
        'api_key',
        NEW.id::TEXT,
        NULL,
        jsonb_build_object(
            'name', NEW.name,
            'prefix', NEW.prefix,
            'scopes', NEW.scopes,
            'expires_at', NEW.expires_at
        ),
        CURRENT_TIMESTAMP
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_api_key_creation
    AFTER INSERT ON api_keys
    FOR EACH ROW EXECUTE FUNCTION audit_api_key_creation();

COMMENT ON TABLE api_keys IS 'API keys for service-to-service authentication';
COMMENT ON COLUMN api_keys.id IS 'Unique API key identifier';
COMMENT ON COLUMN api_keys.user_id IS 'Owner of the API key';
COMMENT ON COLUMN api_keys.name IS 'Human-readable key name';
COMMENT ON COLUMN api_keys.prefix IS 'First 12 chars of key for identification';
COMMENT ON COLUMN api_keys.key_hash IS 'bcrypt hash of full key value';
COMMENT ON COLUMN api_keys.scopes IS 'Array of scopes: ["invoices:read", "news:write"]';
COMMENT ON COLUMN api_keys.expires_at IS 'Optional expiration timestamp';
COMMENT ON COLUMN api_keys.last_used_at IS 'Last successful authentication timestamp';
COMMENT ON COLUMN api_keys.revoked_at IS 'Soft revocation timestamp (NULL = active)';
COMMENT ON COLUMN api_keys.deleted_at IS 'Soft delete timestamp';