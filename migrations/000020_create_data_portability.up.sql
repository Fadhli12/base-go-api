-- Data Import/Export System: export_jobs, import_jobs, import_id_maps

CREATE TABLE export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    entity_types TEXT[] NOT NULL,
    format VARCHAR(10) NOT NULL DEFAULT 'json',
    org_id UUID REFERENCES organizations(id),
    created_by UUID NOT NULL,
    file_path VARCHAR(500),
    file_expires_at TIMESTAMP,
    record_count INTEGER,
    error_message TEXT,
    hmac_signature VARCHAR(128),
    sync BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_export_jobs_status ON export_jobs(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_export_jobs_created_by ON export_jobs(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_export_jobs_org_id ON export_jobs(org_id) WHERE deleted_at IS NULL;

CREATE TABLE import_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    entity_types TEXT[] NOT NULL,
    format VARCHAR(10) NOT NULL DEFAULT 'json',
    org_id UUID REFERENCES organizations(id),
    created_by UUID NOT NULL,
    conflict_strategy VARCHAR(10) NOT NULL DEFAULT 'skip',
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    source_file_path VARCHAR(500),
    idempotency_key VARCHAR(64) UNIQUE,
    result JSONB,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_import_jobs_status ON import_jobs(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_import_jobs_created_by ON import_jobs(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_import_jobs_idempotency ON import_jobs(idempotency_key);

CREATE TABLE import_id_maps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES import_jobs(id),
    entity_type VARCHAR(50) NOT NULL,
    external_id UUID NOT NULL,
    internal_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(job_id, entity_type, external_id)
);

CREATE INDEX idx_import_id_maps_job ON import_id_maps(job_id);
CREATE INDEX idx_import_id_maps_external ON import_id_maps(entity_type, external_id);