-- +goose Down
DROP TRIGGER IF EXISTS update_activities_updated_at ON activities;
DROP TABLE IF EXISTS activity_follows;
DROP TABLE IF EXISTS activity_reads;
DROP TABLE IF EXISTS activities;