-- +goose Up
CREATE TABLE feature_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rollout INTEGER NOT NULL DEFAULT 100 CHECK (rollout >= 0 AND rollout <= 100),
    conditions JSONB,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE UNIQUE INDEX idx_feature_flags_key ON feature_flags(key) WHERE deleted_at IS NULL;
CREATE INDEX idx_feature_flags_deleted_at ON feature_flags(deleted_at);