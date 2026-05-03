-- Migration: 000014_webhooks.up.sql
-- Description: Create webhooks and webhook_deliveries tables
-- Created: 2026-05-03

-- Webhook subscriptions
CREATE TABLE IF NOT EXISTS webhooks (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  name            VARCHAR(255) NOT NULL,
  url             VARCHAR(500) NOT NULL,
  secret          VARCHAR(255) NOT NULL,
  events          JSONB NOT NULL DEFAULT '[]',
  active          BOOLEAN NOT NULL DEFAULT TRUE,
  rate_limit      INTEGER NOT NULL DEFAULT 100,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at      TIMESTAMPTZ DEFAULT NULL,

  CONSTRAINT uq_webhooks_url_org UNIQUE (url, organization_id) WHERE deleted_at IS NULL,
  CONSTRAINT chk_webhooks_events_not_empty CHECK (jsonb_array_length(events) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_webhooks_url_org ON webhooks(url, organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_organization_id ON webhooks(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_events ON webhooks USING gin(events);

-- Triggers for updated_at column
CREATE TRIGGER update_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Webhook delivery audit trail (immutable — NO soft delete)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  webhook_id            UUID NOT NULL REFERENCES webhooks(id) ON DELETE RESTRICT,
  event                 VARCHAR(100) NOT NULL,
  payload               JSONB,
  status                VARCHAR(20) NOT NULL DEFAULT 'queued'
                          CHECK (status IN ('queued','processing','delivered','failed','rate_limited')),
  response_code         INTEGER,
  response_body         TEXT,
  duration_ms           INTEGER,
  attempt_number        INTEGER NOT NULL DEFAULT 1,
  max_attempts          INTEGER NOT NULL DEFAULT 3,
  last_error            TEXT,
  next_retry_at         TIMESTAMPTZ,
  processing_started_at TIMESTAMPTZ,
  delivered_at          TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT chk_webhook_deliveries_attempt CHECK (attempt_number >= 1 AND attempt_number <= max_attempts)
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status) WHERE status IN ('queued','rate_limited');
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status IN ('queued','rate_limited') AND next_retry_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_stuck ON webhook_deliveries(processing_started_at) WHERE status = 'processing' AND processing_started_at IS NOT NULL;

-- Documentation comments
COMMENT ON TABLE webhooks IS 'Webhook subscription configuration for outbound HTTP callbacks';
COMMENT ON COLUMN webhooks.id IS 'Unique webhook identifier';
COMMENT ON COLUMN webhooks.organization_id IS 'Organization scope (NULL = global)';
COMMENT ON COLUMN webhooks.name IS 'Human-readable display name';
COMMENT ON COLUMN webhooks.url IS 'Target endpoint URL';
COMMENT ON COLUMN webhooks.secret IS 'HMAC-SHA256 signing key';
COMMENT ON COLUMN webhooks.events IS 'JSONB array of subscribed event types';
COMMENT ON COLUMN webhooks.active IS 'Toggle to pause deliveries without deletion';
COMMENT ON COLUMN webhooks.rate_limit IS 'Max deliveries per minute';
COMMENT ON COLUMN webhooks.deleted_at IS 'Soft delete timestamp (NULL = active)';

COMMENT ON TABLE webhook_deliveries IS 'Immutable audit trail of webhook delivery attempts';
COMMENT ON COLUMN webhook_deliveries.id IS 'Unique delivery identifier';
COMMENT ON COLUMN webhook_deliveries.webhook_id IS 'Reference to webhook configuration';
COMMENT ON COLUMN webhook_deliveries.event IS 'Event type that triggered delivery';
COMMENT ON COLUMN webhook_deliveries.payload IS 'Full request body sent to endpoint';
COMMENT ON COLUMN webhook_deliveries.status IS 'Delivery state';
COMMENT ON COLUMN webhook_deliveries.response_code IS 'HTTP status code received';
COMMENT ON COLUMN webhook_deliveries.response_body IS 'Response body (truncated in code)';
COMMENT ON COLUMN webhook_deliveries.duration_ms IS 'Delivery duration in milliseconds';
COMMENT ON COLUMN webhook_deliveries.attempt_number IS 'Current attempt count';
COMMENT ON COLUMN webhook_deliveries.max_attempts IS 'Maximum allowed retry attempts';
COMMENT ON COLUMN webhook_deliveries.last_error IS 'Last error message';
COMMENT ON COLUMN webhook_deliveries.next_retry_at IS 'When to retry (exponential backoff)';
COMMENT ON COLUMN webhook_deliveries.processing_started_at IS 'Worker pickup timestamp for stuck recovery';
COMMENT ON COLUMN webhook_deliveries.delivered_at IS 'Successful delivery timestamp';