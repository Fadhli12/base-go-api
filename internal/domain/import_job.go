package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ImportJob struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Status              string         `gorm:"type:varchar(20);not null;default:'queued'" json:"status"`
	EntityTypes         pq.StringArray `gorm:"type:text[];not null" json:"entity_types"`
	Format              string         `gorm:"type:varchar(10);not null;default:'json'" json:"format"`
	OrgID               *uuid.UUID     `gorm:"type:uuid" json:"org_id"`
	CreatedBy           uuid.UUID      `gorm:"type:uuid;not null" json:"created_by"`
	ConflictStrategy    string         `gorm:"type:varchar(10);not null;default:'skip'" json:"conflict_strategy"`
	DryRun              bool           `gorm:"not null;default:false" json:"dry_run"`
	SourceFilePath      *string        `gorm:"type:varchar(500)" json:"source_file_path"`
	IdempotencyKey      *string        `gorm:"type:varchar(64);unique" json:"idempotency_key"`
	Result              datatypes.JSON `gorm:"type:jsonb" json:"result"`
	ErrorMessage        *string        `json:"error_message"`
	ProcessingStartedAt *time.Time     `gorm:"index" json:"processing_started_at,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ImportJob) TableName() string { return "import_jobs" }

func (j *ImportJob) ToResponse() ImportJobResponse {
	resp := ImportJobResponse{
		ID:               j.ID.String(),
		Status:           j.Status,
		EntityTypes:      j.EntityTypes,
		Format:           j.Format,
		CreatedBy:        j.CreatedBy.String(),
		ConflictStrategy: j.ConflictStrategy,
		DryRun:           j.DryRun,
		Result:           j.Result,
		CreatedAt:        j.CreatedAt,
		UpdatedAt:        j.UpdatedAt,
	}
	if j.OrgID != nil {
		orgID := j.OrgID.String()
		resp.OrgID = &orgID
	}
	if j.SourceFilePath != nil {
		resp.SourceFilePath = j.SourceFilePath
	}
	if j.IdempotencyKey != nil {
		resp.IdempotencyKey = j.IdempotencyKey
	}
	if j.ErrorMessage != nil {
		resp.ErrorMessage = j.ErrorMessage
	}
	return resp
}

type CreateImportRequest struct {
	EntityTypes      pq.StringArray `json:"entity_types" validate:"required,min=1"`
	Format           string         `json:"format" validate:"required,oneof=json csv"`
	OrgID            *string        `json:"org_id,omitempty"`
	ConflictStrategy string         `json:"conflict_strategy" validate:"required,oneof=skip overwrite fail"`
	DryRun           bool           `json:"dry_run"`
}

type ImportPreviewRequest struct {
	Format           string         `json:"format" validate:"required,oneof=json csv"`
	ConflictStrategy string         `json:"conflict_strategy" validate:"required,oneof=skip overwrite fail"`
}

type ImportJobResponse struct {
	ID               string         `json:"id"`
	Status           string         `json:"status"`
	EntityTypes      pq.StringArray `json:"entity_types"`
	Format           string         `json:"format"`
	OrgID            *string        `json:"org_id,omitempty"`
	CreatedBy        string         `json:"created_by"`
	ConflictStrategy string         `json:"conflict_strategy"`
	DryRun           bool           `json:"dry_run"`
	SourceFilePath   *string        `json:"source_file_path,omitempty"`
	IdempotencyKey   *string        `json:"idempotency_key,omitempty"`
	Result           datatypes.JSON `json:"result,omitempty"`
	ErrorMessage     *string        `json:"error_message,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type ImportPreviewResponse struct {
	EntityTypes []ImportPreviewEntityType `json:"entity_types"`
	TotalRows  int                        `json:"total_rows"`
	Warnings   []string                   `json:"warnings,omitempty"`
}

type ImportPreviewEntityType struct {
	EntityType string `json:"entity_type"`
	RowCount   int    `json:"row_count"`
}

type ImportResult struct {
	EntityTypes     map[string]EntityTypeResult `json:"entity_types"`
	TotalCreated    int                         `json:"total_created"`
	TotalSkipped    int                         `json:"total_skipped"`
	TotalFailed     int                         `json:"total_failed"`
	TotalOverwritten int                        `json:"total_overwritten"`
}

type EntityTypeResult struct {
	Created     int `json:"created"`
	Skipped     int `json:"skipped"`
	Failed      int `json:"failed"`
	Overwritten int `json:"overwritten"`
}