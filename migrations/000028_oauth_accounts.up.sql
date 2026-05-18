-- +goose Up

-- Links between users and their OAuth social identities
CREATE TABLE oauth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES oauth_providers(id) ON DELETE CASCADE,
    provider_user_id VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    email_verified BOOLEAN DEFAULT false,
    display_name VARCHAR(255),
    avatar_url VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT uq_oauth_accounts_provider_user UNIQUE (provider_id, provider_user_id) WHERE deleted_at IS NULL,
    CONSTRAINT uq_oauth_accounts_user_provider UNIQUE (user_id, provider_id) WHERE deleted_at IS NULL
);

-- Indexes
CREATE INDEX idx_oauth_accounts_user_id ON oauth_accounts(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_oauth_accounts_provider_id ON oauth_accounts(provider_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_oauth_accounts_email ON oauth_accounts(email) WHERE deleted_at IS NULL;

-- Comments
COMMENT ON TABLE oauth_accounts IS 'Links between users and their OAuth social identities';
COMMENT ON COLUMN oauth_accounts.provider_user_id IS 'Unique user ID from the OAuth provider. String type because GitHub uses integer IDs.';
COMMENT ON COLUMN oauth_accounts.email_verified IS 'Whether the provider confirmed email ownership';

-- Auto-update trigger
CREATE TRIGGER update_oauth_accounts_updated_at
    BEFORE UPDATE ON oauth_accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();