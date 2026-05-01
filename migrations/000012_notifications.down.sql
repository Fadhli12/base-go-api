-- Migration: 000012_notifications.down.sql
-- Description: Rollback notifications and notification_preferences tables
-- Created: 2026-05-01

-- Drop triggers first (reverse order of creation)
DROP TRIGGER IF EXISTS update_notification_preferences_updated_at ON notification_preferences;
DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS notifications;
