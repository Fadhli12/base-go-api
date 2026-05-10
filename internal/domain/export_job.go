package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type ExportJob struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Status        string         `gorm:"type:varchar(20);not null;default:'queued'" json:"status"`
	EntityTypes   pq.StringArray `gorm:"type:text[];not null" json:"entity_types"`
	Format        string         `gorm:"type:varchar(10);not null;default:'json'" json:"format"`
	OrgID         *uuid.UUID     `gorm:"type:uuid" json:"org_id"`
	CreatedBy     uuid.UUID      `gorm:"type:uuid;not null" json:"created_by"`
	FilePath      *string        `gorm:"type:varchar(500)" json:"file_path"`
	FileExpiresAt *time.Time     `json:"file_expires_at"`
	RecordCount   *int           `json:"record_count"`
	ErrorMessage  *string        `json:"error_message"`
	HmacSignature *string        `gorm:"type:varchar(128)" json:"hmac_signature"`
	Sync          bool           `gorm:"not null;default:false" json:"sync"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ExportJob) TableName() string { return "export_jobs" }

func (e *ExportJob) ToResponse() ExportJobResponse {
	resp := ExportJobResponse{
		ID:          e.ID.String(),
		Status:      e.Status,
		EntityTypes: e.EntityTypes,
		Format:      e.Format,
		CreatedBy:   e.CreatedBy.String(),
		Sync:        e.Sync,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
	if e.OrgID != nil {
		orgID := e.OrgID.String()
		resp.OrgID = &orgID
	}
	if e.FilePath != nil {
		resp.FilePath = e.FilePath
	}
	if e.FileExpiresAt != nil {
		resp.FileExpiresAt = e.FileExpiresAt
	}
	if e.RecordCount != nil {
		resp.RecordCount = e.RecordCount
	}
	if e.ErrorMessage != nil {
		resp.ErrorMessage = e.ErrorMessage
	}
	if e.HmacSignature != nil {
		resp.HmacSignature = e.HmacSignature
	}
	return resp
}

type CreateExportRequest struct {
	EntityTypes    pq.StringArray `json:"entity_types" validate:"required,min=1"`
	Format         string         `json:"format" validate:"required,oneof=json csv"`
	OrgID          *string        `json:"org_id,omitempty"`
	Sync           bool           `json:"sync"`
	IncludeDeleted bool           `json:"include_deleted"`
}

type ExportJobResponse struct {
	ID            string         `json:"id"`
	Status        string         `json:"status"`
	EntityTypes   pq.StringArray `json:"entity_types"`
	Format        string         `json:"format"`
	OrgID         *string        `json:"org_id,omitempty"`
	CreatedBy     string         `json:"created_by"`
	FilePath      *string        `json:"file_path,omitempty"`
	FileExpiresAt *time.Time     `json:"file_expires_at,omitempty"`
	RecordCount   *int           `json:"record_count,omitempty"`
	ErrorMessage  *string        `json:"error_message,omitempty"`
	HmacSignature *string        `json:"hmac_signature,omitempty"`
	Sync          bool           `json:"sync"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}