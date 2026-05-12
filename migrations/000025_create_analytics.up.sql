-- +goose Up

-- Metric events table: immutable event records for analytics aggregation
CREATE TABLE metric_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    actor_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(100) NOT NULL,
    organization_id UUID,
    metadata JSONB,
    event_timestamp TIMESTAMPTZ NOT NULL,
    date DATE NOT NULL,
    hour INTEGER NOT NULL CHECK (hour >= 0 AND hour <= 23),
    archived_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Note: No updated_at column because MetricEvents are never updated after creation

-- Indexes for metric_events
CREATE UNIQUE INDEX idx_metric_events_idempotency ON metric_events(event_type, resource_id, date, hour) WHERE archived_at IS NULL;
CREATE INDEX idx_metric_events_organization_id ON metric_events(organization_id) WHERE archived_at IS NULL;
CREATE INDEX idx_metric_events_actor_id ON metric_events(actor_id) WHERE archived_at IS NULL;
CREATE INDEX idx_metric_events_resource ON metric_events(resource_type, resource_id) WHERE archived_at IS NULL;
CREATE INDEX idx_metric_events_event_type ON metric_events(event_type) WHERE archived_at IS NULL;
CREATE INDEX idx_metric_events_date ON metric_events(date DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_metric_events_hour ON metric_events(date, hour) WHERE archived_at IS NULL;
CREATE INDEX idx_metric_events_feed ON metric_events(organization_id, date DESC) WHERE archived_at IS NULL AND deleted_at IS NULL;
CREATE INDEX idx_metric_events_archived_at ON metric_events(archived_at) WHERE archived_at IS NOT NULL;
CREATE INDEX idx_metric_events_deleted_at ON metric_events(deleted_at);
-- Aggregation-optimized index for counting by type and period
CREATE INDEX idx_metric_events_aggregation ON metric_events(event_type, date, hour) WHERE archived_at IS NULL;

-- Foreign keys
ALTER TABLE metric_events ADD CONSTRAINT fk_metric_events_actor FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE metric_events ADD CONSTRAINT fk_metric_events_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Dashboard metrics table: pre-aggregated metric data
CREATE TABLE dashboard_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metric_type VARCHAR(50) NOT NULL,
    period_type VARCHAR(20) NOT NULL CHECK (period_type IN ('hourly', 'daily', 'weekly', 'monthly')),
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    metadata JSONB,
    organization_id UUID,
    calculated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for dashboard_metrics
CREATE UNIQUE INDEX idx_dashboard_metrics_unique ON dashboard_metrics(metric_type, period_type, period_start, organization_id);
CREATE INDEX idx_dashboard_metrics_type_period ON dashboard_metrics(metric_type, period_type, period_start) WHERE organization_id IS NULL;
CREATE INDEX idx_dashboard_metrics_type_period_org ON dashboard_metrics(metric_type, period_type, period_start, organization_id);
CREATE INDEX idx_dashboard_metrics_calculated_at ON dashboard_metrics(calculated_at);

-- Auto-update trigger
CREATE TRIGGER update_dashboard_metrics_updated_at
    BEFORE UPDATE ON dashboard_metrics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Foreign keys
ALTER TABLE dashboard_metrics ADD CONSTRAINT fk_dashboard_metrics_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Dashboard preferences table: per-organization dashboard configuration
CREATE TABLE dashboard_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    metric_categories JSONB,
    updated_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for dashboard_preferences
CREATE UNIQUE INDEX idx_dashboard_preferences_organization ON dashboard_preferences(organization_id);

-- Auto-update trigger
CREATE TRIGGER update_dashboard_preferences_updated_at
    BEFORE UPDATE ON dashboard_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Foreign keys
ALTER TABLE dashboard_preferences ADD CONSTRAINT fk_dashboard_preferences_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE dashboard_preferences ADD CONSTRAINT fk_dashboard_preferences_user FOREIGN KEY (updated_by_user_id) REFERENCES users(id) ON DELETE CASCADE;