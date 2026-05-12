-- +goose Down
DROP TRIGGER IF EXISTS update_tags_updated_at ON tags;
DROP TABLE IF EXISTS entity_tags;
DROP TABLE IF EXISTS tags;