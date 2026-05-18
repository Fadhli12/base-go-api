-- +goose Down

-- Drop in reverse dependency order: oauth_accounts references oauth_providers
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS oauth_providers;