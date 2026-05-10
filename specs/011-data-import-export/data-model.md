# Data Model: Data Import/Export System

## New Tables

### export_jobs

```sql
CREATE TABLE export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    -- 'queued', 'processing', 'completed', 'failed'
    entity_types TEXT[] NOT NULL,
    format VARCHAR(10) NOT NULL DEFAULT 'json',
    -- 'json', 'csv'
    org_id UUID REFERENCES organizations(id),
    -- NULLABLE: NULL means global (non-org-scoped) export. FK allows NULL per PostgreSQL semantics.
    created_by UUID NOT NULL, VARCHAR(500),
    file_expires_at TIMESTAMP,
    record_count INTEGER,
    error_message TEXT,
    hmac_signature VARCHAR(128),
    sync BOOLEAN NOT NULL DEFAULT FALSE,
    -- TRUE if streamed synchronously, FALSE if async
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_export_jobs_status ON export_jobs(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_export_jobs_created_by ON export_jobs(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_export_jobs_org_id ON export_jobs(org_id) WHERE deleted_at IS NULL;
```

### import_jobs

```sql
CREATE TABLE import_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    -- 'queued', 'validating', 'processing', 'completed', 'failed', 'cancelled'
    entity_types TEXT[] NOT NULL,
    format VARCHAR(10) NOT NULL DEFAULT 'json',
    org_id UUID REFERENCES organizations(id),
    -- NULLABLE: NULL means global (non-org-scoped) import. FK allows NULL per PostgreSQL semantics.
    created_by UUID NOT NULL, VARCHAR(10) NOT NULL DEFAULT 'skip',
    -- 'skip', 'overwrite', 'fail'
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    source_file_path VARCHAR(500),
    idempotency_key VARCHAR(64) UNIQUE,
    -- SHA-256 hash of file content + timestamp for dedup
    result JSONB,
    -- { "entity_type": { "created": N, "skipped": N, "failed": N, "overwritten": N } }
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    
    FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX idx_import_jobs_status ON import_jobs(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_import_jobs_created_by ON import_jobs(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_import_jobs_idempotency ON import_jobs(idempotency_key);
```

### import_id_maps

```sql
CREATE TABLE import_id_maps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES import_jobs(id),
    entity_type VARCHAR(50) NOT NULL,
    external_id UUID NOT NULL,
    -- UUID from the import file
    internal_id UUID NOT NULL,
    -- UUID generated or matched in our DB
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    UNIQUE(job_id, entity_type, external_id)
);

CREATE INDEX idx_import_id_maps_job ON import_id_maps(job_id);
CREATE INDEX idx_import_id_maps_external ON import_id_maps(entity_type, external_id);
```

## Domain Entities

### ExportJob

```go
type ExportJob struct {
    ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    Status       string          `gorm:"type:varchar(20);not null;default:'queued'" json:"status"`
    EntityTypes  pq.StringArray  `gorm:"type:text[];not null" json:"entity_types"`
    Format       string          `gorm:"type:varchar(10);not null;default:'json'" json:"format"`
    OrgID        *uuid.UUID      `gorm:"type:uuid" json:"org_id"`
    CreatedBy    uuid.UUID       `gorm:"type:uuid;not null" json:"created_by"`
    FilePath     *string         `gorm:"type:varchar(500)" json:"file_path"`
    FileExpiresAt *time.Time     `json:"file_expires_at"`
    RecordCount  *int            `json:"record_count"`
    ErrorMessage *string         `json:"error_message"`
    HmacSignature *string        `gorm:"type:varchar(128)" json:"hmac_signature"`
    Sync         bool            `gorm:"not null;default:false" json:"sync"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
    DeletedAt    gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (ExportJob) TableName() string { return "export_jobs" }
```

### ImportJob

```go
type ImportJob struct {
    ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    Status           string         `gorm:"type:varchar(20);not null;default:'queued'" json:"status"`
    EntityTypes      pq.StringArray `gorm:"type:text[];not null" json:"entity_types"`
    Format           string         `gorm:"type:varchar(10);not null;default:'json'" json:"format"`
    OrgID            *uuid.UUID     `gorm:"type:uuid" json:"org_id"`
    CreatedBy        uuid.UUID      `gorm:"type:uuid;not null" json:"created_by"`
    ConflictStrategy string         `gorm:"type:varchar(10);not null;default:'skip'" json:"conflict_strategy"`
    DryRun           bool           `gorm:"not null;default:false" json:"dry_run"`
    SourceFilePath   *string        `gorm:"type:varchar(500)" json:"source_file_path"`
    IdempotencyKey   *string        `gorm:"type:varchar(64);unique" json:"idempotency_key"`
    Result           datatypes.JSON `gorm:"type:jsonb" json:"result"`
    ErrorMessage     *string        `json:"error_message"`
    CreatedAt        time.Time      `json:"created_at"`
    UpdatedAt        time.Time      `json:"updated_at"`
    DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ImportJob) TableName() string { return "import_jobs" }
```

### ImportIDMap

```go
type ImportIDMap struct {
    ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    JobID      uuid.UUID `gorm:"type:uuid;not null" json:"job_id"`
    EntityType string    `gorm:"type:varchar(50);not null" json:"entity_type"`
    ExternalID uuid.UUID `gorm:"type:uuid;not null" json:"external_id"`
    InternalID uuid.UUID `gorm:"type:uuid;not null" json:"internal_id"`
    CreatedAt  time.Time `json:"created_at"`
}

func (ImportIDMap) TableName() string { return "import_id_maps" }
```

## Entity Exportability Registry

```go
// Hardcoded registry - not in Casbin, this is MAC (mandatory access control)
var EntityRegistry = map[string]EntityConfig{
    // EXPORTABLE (v1)
    "organizations":         {Exportable: true, Importable: true, Sensitivity: "pii"},
    "roles":                  {Exportable: true, Importable: true, Sensitivity: "public"},
    "permissions":            {Exportable: true, Importable: true, Sensitivity: "public"},
    "users":                  {Exportable: true, Importable: true, Sensitivity: "pii"},
    "organization_members":   {Exportable: true, Importable: true, Sensitivity: "pii"},
    "user_roles":             {Exportable: true, Importable: true, Sensitivity: "public"},
    "user_permissions":       {Exportable: true, Importable: true, Sensitivity: "public"},
    
    // BLOCKED from export/import (security-sensitive)
    "api_keys":                  {Exportable: false, Importable: false, Sensitivity: "restricted"},
    "two_factor":                {Exportable: false, Importable: false, Sensitivity: "restricted"},
    "two_factor_recovery_codes": {Exportable: false, Importable: false, Sensitivity: "restricted"},
    "refresh_tokens":            {Exportable: false, Importable: false, Sensitivity: "restricted"},
    "password_reset_tokens":     {Exportable: false, Importable: false, Sensitivity: "restricted"},
    
    // BLOCKED from import (system-managed)
    "audit_logs":          {Exportable: false, Importable: false, Sensitivity: "system"},
    "system_settings":     {Exportable: false, Importable: false, Sensitivity: "system"},
    "email_queue":         {Exportable: false, Importable: false, Sensitivity: "system"},
    "email_bounces":       {Exportable: false, Importable: false, Sensitivity: "system"},
    "email_templates":     {Exportable: true, Importable: false, Sensitivity: "public"}, // export only
}

// Topological import order (dependencies first)
var ImportOrder = []string{
    "organizations",
    "roles",
    "permissions",
    "users",
    "organization_members",
    "user_roles",
    "user_permissions",
}
```

## New Permissions

```sql
-- Added to permission:sync manifest
('data_portability', 'export', 'create', 'Create data export jobs'),
('data_portability', 'export', 'download', 'Download exported data files'),
('data_portability', 'import', 'create', 'Create data import jobs'),
('data_portability', 'import', 'view', 'View import job status and results'),
('data_portability', 'import', 'cancel', 'Cancel queued or processing import jobs');
```

## DTOs

```go
// Export request
type ExportRequest struct {
    EntityTypes   []string `json:"entity_types" validate:"required,min=1"`
    Format        string   `json:"format" validate:"omitempty,oneof=json csv"`
    OrgID         *string  `json:"org_id,omitempty"`
    IncludeDeleted bool     `json:"include_deleted,omitempty"`
    Sync          bool      `json:"sync,omitempty"`
}

// Export response
type ExportJobResponse struct {
    ID           uuid.UUID   `json:"id"`
    Status       string      `json:"status"`
    EntityTypes  []string    `json:"entity_types"`
    Format       string      `json:"format"`
    OrgID        *uuid.UUID   `json:"org_id,omitempty"`
    RecordCount  *int         `json:"record_count,omitempty"`
    DownloadURL  *string      `json:"download_url,omitempty"`
    ErrorMessage *string      `json:"error_message,omitempty"`
    CreatedAt    time.Time    `json:"created_at"`
    UpdatedAt    time.Time    `json:"updated_at"`
}

// Import request (multipart form)
type ImportRequest struct {
    File             io.Reader `form:"file" validate:"required"`
    Format           string    `form:"format" validate:"omitempty,oneof=json csv"`
    ConflictStrategy string    `form:"conflict_strategy" validate:"omitempty,oneof=skip overwrite fail"`
    DryRun           bool      `form:"dry_run,omitempty"`
}

// Import preview response
type ImportPreviewResponse struct {
    TotalRecords    int                        `json:"total_records"`
    EntityCounts   map[string]int             `json:"entity_counts"`
    Conflicts       map[string]int             `json:"conflicts"`
    ValidationErrors []string                  `json:"validation_errors"`
    HmacValid       bool                       `json:"hmac_valid"`
}

// Import result
type ImportResult struct {
    EntityTypes map[string]EntityTypeResult `json:"entity_types"`
    TotalCreated   int `json:"total_created"`
    TotalSkipped   int `json:"total_skipped"` 
    TotalFailed    int `json:"total_failed"`
    TotalOverwritten int `json:"total_overwritten"`
}

type EntityTypeResult struct {
    Created     int `json:"created"`
    Skipped     int `json:"skipped"`
    Failed      int `json:"failed"`
    Overwritten int `json:"overwritten"`
}
```