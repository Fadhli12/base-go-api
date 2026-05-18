-- 000027: Create idempotency_keys table
-- Stores idempotency key records for duplicate request protection.
-- Redis provides fast lookup; PostgreSQL provides durability and audit.

CREATE TABLE idempotency_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(128) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    organization_id UUID REFERENCES organizations(id),
    http_method VARCHAR(10) NOT NULL,
    request_path VARCHAR(500) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'processing',
    response_status_code INTEGER,
    response_body TEXT,
    response_body_size INTEGER NOT NULL DEFAULT 0,
    response_headers JSONB,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Unique constraint: one active key per user+method+path combo (partial index for soft delete)
CREATE UNIQUE INDEX idx_idempotency_keys_active
    ON idempotency_keys (idempotency_key, user_id, http_method, request_path)
    WHERE deleted_at IS NULL;

-- Index for reaper cleanup: find expired records efficiently
CREATE INDEX idx_idempotency_keys_expires_at
    ON idempotency_keys (expires_at)
    WHERE deleted_at IS NULL;

-- Index for per-user lookups: list a user's idempotency records
CREATE INDEX idx_idempotency_keys_user_id
    ON idempotency_keys (user_id)
    WHERE deleted_at IS NULL;

-- Index for organization-scoped lookups
CREATE INDEX idx_idempotency_keys_organization_id
    ON idempotency_keys (organization_id)
    WHERE deleted_at IS NULL AND organization_id IS NOT NULL;

-- Index for status-based queries (find in-flight records)
CREATE INDEX idx_idempotency_keys_status
    ON idempotency_keys (status)
    WHERE deleted_at IS NULL;

-- Auto-update updated_at trigger
CREATE TRIGGER update_idempotency_keys_updated_at
    BEFORE UPDATE ON idempotency_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();