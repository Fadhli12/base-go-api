-- Migration: 000008_email_service.up.sql
-- Description: Create email service tables (templates, queue, bounces)
-- Created: 2026-04-24

-- Create email_templates table
-- Templates are user-editable with soft delete support
CREATE TABLE IF NOT EXISTS email_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    html_content TEXT NOT NULL,
    text_content TEXT,
    category VARCHAR(50) NOT NULL,
    is_active BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    
    -- Constraints
    CONSTRAINT chk_template_name CHECK (LENGTH(name) >= 3),
    CONSTRAINT chk_template_subject CHECK (LENGTH(subject) >= 1),
    CONSTRAINT chk_template_category CHECK (category IN ('transactional', 'marketing', 'notification', 'system'))
);

-- Create email_queue table
-- Permanent audit trail - NO soft delete, NO updates except status transitions
CREATE TABLE IF NOT EXISTS email_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    to_address VARCHAR(255) NOT NULL,
    subject VARCHAR(500) NOT NULL,
    template VARCHAR(100),
    data JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    provider VARCHAR(50),
    message_id VARCHAR(255),
    attempts INTEGER DEFAULT 0 NOT NULL,
    max_attempts INTEGER DEFAULT 5 NOT NULL,
    last_error TEXT,
    sent_at TIMESTAMPTZ DEFAULT NULL,
    delivered_at TIMESTAMPTZ DEFAULT NULL,
    bounced_at TIMESTAMPTZ DEFAULT NULL,
    bounce_reason TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    
    -- Constraints
    CONSTRAINT chk_email_status CHECK (status IN ('queued', 'processing', 'sent', 'delivered', 'bounced', 'failed')),
    CONSTRAINT chk_email_attempts CHECK (attempts >= 0 AND attempts <= max_attempts),
    CONSTRAINT chk_email_max_attempts CHECK (max_attempts >= 1 AND max_attempts <= 10)
);

-- Create email_bounces table
-- Insert-only for compliance (no updates, no deletes)
CREATE TABLE IF NOT EXISTS email_bounces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    bounce_type VARCHAR(50) NOT NULL,
    bounce_reason TEXT,
    message_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    
    -- Constraints
    CONSTRAINT chk_bounce_type CHECK (bounce_type IN ('hard', 'soft', 'spam', 'technical'))
);

-- Create indexes for email_templates
CREATE INDEX IF NOT EXISTS idx_email_templates_name ON email_templates(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_email_templates_category ON email_templates(category) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_email_templates_active ON email_templates(is_active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_email_templates_deleted_at ON email_templates(deleted_at) WHERE deleted_at IS NOT NULL;

-- Create indexes for email_queue
CREATE INDEX IF NOT EXISTS idx_email_queue_status_created ON email_queue(status, created_at);
CREATE INDEX IF NOT EXISTS idx_email_queue_to_created ON email_queue(to_address, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_queue_id ON email_queue(id);

-- Create indexes for email_bounces
CREATE INDEX IF NOT EXISTS idx_email_bounces_email_created ON email_bounces(email, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_bounces_type ON email_bounces(bounce_type);

-- Add triggers for updated_at
CREATE TRIGGER update_email_templates_updated_at 
    BEFORE UPDATE ON email_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_email_queue_updated_at 
    BEFORE UPDATE ON email_queue
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add comments for documentation
COMMENT ON TABLE email_templates IS 'Email templates with soft delete support';
COMMENT ON COLUMN email_templates.id IS 'Unique template identifier';
COMMENT ON COLUMN email_templates.name IS 'Unique template name (e.g., welcome_email, password_reset)';
COMMENT ON COLUMN email_templates.subject IS 'Email subject line template';
COMMENT ON COLUMN email_templates.html_content IS 'HTML email body template';
COMMENT ON COLUMN email_templates.text_content IS 'Plain text email body template (optional)';
COMMENT ON COLUMN email_templates.category IS 'Template category: transactional, marketing, notification, system';
COMMENT ON COLUMN email_templates.is_active IS 'Whether template is active (default: true)';
COMMENT ON COLUMN email_templates.deleted_at IS 'Soft delete timestamp (NULL = active)';

COMMENT ON TABLE email_queue IS 'Email queue for async processing - permanent audit trail (NO soft delete)';
COMMENT ON COLUMN email_queue.id IS 'Unique queue entry identifier';
COMMENT ON COLUMN email_queue.to_address IS 'Recipient email address';
COMMENT ON COLUMN email_queue.subject IS 'Email subject line';
COMMENT ON COLUMN email_queue.template IS 'Template name reference (nullable for direct sends)';
COMMENT ON COLUMN email_queue.data IS 'Template variables as JSONB';
COMMENT ON COLUMN email_queue.status IS 'Email status: queued, processing, sent, delivered, bounced, failed';
COMMENT ON COLUMN email_queue.provider IS 'Email provider: smtp, sendgrid, ses';
COMMENT ON COLUMN email_queue.message_id IS 'Provider message ID for tracking';
COMMENT ON COLUMN email_queue.attempts IS 'Number of send attempts';
COMMENT ON COLUMN email_queue.max_attempts IS 'Maximum retry attempts (default: 5)';
COMMENT ON COLUMN email_queue.last_error IS 'Last error message if failed';
COMMENT ON COLUMN email_queue.sent_at IS 'Timestamp when email was sent';
COMMENT ON COLUMN email_queue.delivered_at IS 'Timestamp when delivery confirmed';
COMMENT ON COLUMN email_queue.bounced_at IS 'Timestamp when bounce detected';
COMMENT ON COLUMN email_queue.bounce_reason IS 'Bounce reason from provider';

COMMENT ON TABLE email_bounces IS 'Email bounce records - insert-only for compliance';
COMMENT ON COLUMN email_bounces.id IS 'Unique bounce record identifier';
COMMENT ON COLUMN email_bounces.email IS 'Bounced email address';
COMMENT ON COLUMN email_bounces.bounce_type IS 'Bounce type: hard, soft, spam, technical';
COMMENT ON COLUMN email_bounces.bounce_reason IS 'Bounce reason from provider';
COMMENT ON COLUMN email_bounces.message_id IS 'Provider message ID for correlation';
COMMENT ON COLUMN email_bounces.created_at IS 'Bounce detection timestamp';