-- Migration: 000008_email_service.down.sql
-- Description: Rollback email service tables
-- Created: 2026-04-24

-- Drop triggers first (reverse order of creation)
DROP TRIGGER IF EXISTS update_email_queue_updated_at ON email_queue;
DROP TRIGGER IF EXISTS update_email_templates_updated_at ON email_templates;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS email_bounces;
DROP TABLE IF EXISTS email_queue;
DROP TABLE IF EXISTS email_templates;