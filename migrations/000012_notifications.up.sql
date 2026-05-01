-- Migration: 000012_notifications.up.sql
-- Description: Create notifications and notification_preferences tables
-- Created: 2026-05-01

-- Create notifications table
-- Permanent audit trail - NO soft delete (like email_queue)
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    action_url VARCHAR(500),
    read_at TIMESTAMPTZ DEFAULT NULL,
    archived_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create notification_preferences table
-- User configuration - HAS soft delete (like other config entities)
CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type VARCHAR(30) NOT NULL,
    email_enabled BOOLEAN DEFAULT true NOT NULL,
    push_enabled BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,

    -- Unique constraint: one preference record per user per notification type
    CONSTRAINT uq_notification_prefs_user_type UNIQUE (user_id, notification_type)
);

-- Indexes for notifications
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, read_at);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_archived ON notifications(user_id, archived_at) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);

-- Indexes for notification_preferences
CREATE INDEX IF NOT EXISTS idx_notification_prefs_user_id ON notification_preferences(user_id) WHERE deleted_at IS NULL;

-- Trigger for notifications updated_at
CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Trigger for notification_preferences updated_at
CREATE TRIGGER update_notification_preferences_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE notifications IS 'User notifications - permanent audit trail (NO soft delete)';
COMMENT ON COLUMN notifications.id IS 'Unique notification identifier';
COMMENT ON COLUMN notifications.user_id IS 'Recipient user ID';
COMMENT ON COLUMN notifications.type IS 'Notification type: mention, assignment, system, invoice.created, news.published';
COMMENT ON COLUMN notifications.title IS 'Short notification title';
COMMENT ON COLUMN notifications.message IS 'Full notification message body';
COMMENT ON COLUMN notifications.action_url IS 'Optional URL for the notification action (deep link)';
COMMENT ON COLUMN notifications.read_at IS 'Timestamp when user read the notification (NULL = unread)';
COMMENT ON COLUMN notifications.archived_at IS 'Timestamp when user archived the notification (NULL = active)';
COMMENT ON COLUMN notifications.created_at IS 'Notification creation timestamp';
COMMENT ON COLUMN notifications.updated_at IS 'Last update timestamp';

COMMENT ON TABLE notification_preferences IS 'Per-user notification preferences with soft delete support';
COMMENT ON COLUMN notification_preferences.id IS 'Unique preference record identifier';
COMMENT ON COLUMN notification_preferences.user_id IS 'User this preference belongs to';
COMMENT ON COLUMN notification_preferences.notification_type IS 'Notification type this preference applies to';
COMMENT ON COLUMN notification_preferences.email_enabled IS 'Whether email delivery is enabled for this type (default: true)';
COMMENT ON COLUMN notification_preferences.push_enabled IS 'Whether push delivery is enabled for this type (default: true)';
COMMENT ON COLUMN notification_preferences.deleted_at IS 'Soft delete timestamp (NULL = active)';
