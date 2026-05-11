-- +goose Down
DROP INDEX IF EXISTS idx_feature_flags_deleted_at;
DROP INDEX IF EXISTS idx_feature_flags_key;
DROP TABLE IF EXISTS feature_flags;