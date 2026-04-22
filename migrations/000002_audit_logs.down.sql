-- Migration: 000002_audit_logs.down.sql
-- Description: Rollback audit_logs table
-- Created: 2026-04-22

-- Drop trigger
DROP TRIGGER IF EXISTS audit_logs_immutable ON audit_logs;

-- Drop trigger function
DROP FUNCTION IF EXISTS prevent_audit_log_modification();

-- Drop audit_logs table
DROP TABLE IF EXISTS audit_logs;
