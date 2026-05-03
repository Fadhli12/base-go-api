-- Rollback: 000014_webhooks.down.sql
-- Description: Drop webhooks and webhook_deliveries tables

-- Drop triggers first
DROP TRIGGER IF EXISTS update_webhooks_updated_at ON webhooks;

-- Drop indexes
DROP INDEX IF EXISTS idx_webhook_deliveries_stuck;
DROP INDEX IF EXISTS idx_webhook_deliveries_created_at;
DROP INDEX IF EXISTS idx_webhook_deliveries_next_retry;
DROP INDEX IF EXISTS idx_webhook_deliveries_status;
DROP INDEX IF EXISTS idx_webhook_deliveries_webhook_id;
DROP INDEX IF EXISTS idx_webhooks_events;
DROP INDEX IF EXISTS idx_webhooks_active;
DROP INDEX IF EXISTS idx_webhooks_organization_id;
DROP INDEX IF EXISTS uq_webhooks_url_org;

-- Drop tables in reverse dependency order (deliveries first because of FK)
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;