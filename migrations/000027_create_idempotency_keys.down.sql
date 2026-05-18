-- 000027: Drop idempotency_keys table (rollback)

DROP TRIGGER IF EXISTS update_idempotency_keys_updated_at ON idempotency_keys;
DROP INDEX IF EXISTS idx_idempotency_keys_status;
DROP INDEX IF EXISTS idx_idempotency_keys_organization_id;
DROP INDEX IF EXISTS idx_idempotency_keys_user_id;
DROP INDEX IF EXISTS idx_idempotency_keys_expires_at;
DROP INDEX IF EXISTS idx_idempotency_keys_active;
DROP TABLE IF EXISTS idempotency_keys;