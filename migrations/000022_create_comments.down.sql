-- +goose Down
DROP TRIGGER IF EXISTS update_comments_updated_at ON comments;
DROP INDEX IF EXISTS idx_comments_deleted_at;
DROP INDEX IF EXISTS idx_comments_commentable;
DROP INDEX IF EXISTS idx_comments_organization_id;
DROP INDEX IF EXISTS idx_comments_author_id;
DROP INDEX IF EXISTS idx_comments_parent_id;
DROP TABLE IF EXISTS comments;