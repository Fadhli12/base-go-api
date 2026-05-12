-- +goose Down
DROP TRIGGER IF EXISTS update_dashboard_preferences_updated_at ON dashboard_preferences;
DROP TRIGGER IF EXISTS update_dashboard_metrics_updated_at ON dashboard_metrics;

DROP TABLE IF EXISTS dashboard_preferences;
DROP TABLE IF EXISTS dashboard_metrics;
DROP TABLE IF EXISTS metric_events;