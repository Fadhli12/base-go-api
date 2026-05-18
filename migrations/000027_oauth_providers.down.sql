-- +goose Down

-- Drop trigger on oauth_providers
DROP TRIGGER IF EXISTS update_oauth_providers_updated_at ON oauth_providers;

-- Drop table
DROP TABLE IF EXISTS oauth_providers;