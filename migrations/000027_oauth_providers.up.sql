-- +goose Up

-- OAuth 2.0 identity provider configurations
CREATE TABLE oauth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    client_id VARCHAR(500) NOT NULL,
    client_secret_encrypted TEXT NOT NULL,
    redirect_url VARCHAR(500) NOT NULL,
    additional_scopes TEXT[] DEFAULT '{}',
    config JSONB DEFAULT '{}',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    is_system BOOLEAN NOT NULL DEFAULT false,
    organization_id UUID REFERENCES organizations(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT uq_oauth_providers_name UNIQUE (name) WHERE deleted_at IS NULL,
    CONSTRAINT uq_oauth_providers_name_org UNIQUE (name, organization_id) WHERE deleted_at IS NULL
);

-- Indexes
CREATE INDEX idx_oauth_providers_org_id ON oauth_providers(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_oauth_providers_enabled ON oauth_providers(is_enabled) WHERE deleted_at IS NULL;

-- Comments
COMMENT ON TABLE oauth_providers IS 'OAuth 2.0 identity provider configurations';
COMMENT ON COLUMN oauth_providers.client_secret_encrypted IS 'AES-256-GCM encrypted client secret. Format: v1:base64(IV[12]+ciphertext+GCM_tag[16])';
COMMENT ON COLUMN oauth_providers.additional_scopes IS 'Extra OAuth scopes appended to provider defaults. Defaults are hardcoded and cannot be removed.';
COMMENT ON COLUMN oauth_providers.config IS 'Provider-specific configuration JSON. E.g., {"tenant_id": "common"} for Microsoft.';

-- Auto-update trigger
CREATE TRIGGER update_oauth_providers_updated_at
    BEFORE UPDATE ON oauth_providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();