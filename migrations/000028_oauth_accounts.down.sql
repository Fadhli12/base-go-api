-- +goose Down

-- Drop trigger on oauth_accounts
DROP TRIGGER IF EXISTS update_oauth_accounts_updated_at ON oauth_accounts;

-- Drop table
DROP TABLE IF EXISTS oauth_accounts;