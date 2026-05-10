package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MediaVersion represents a single version of a media file
type MediaVersion struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MediaID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"media_id"`
	Media            *Media         `gorm:"foreignKey:MediaID;constraint:OnDelete:CASCADE" json:"-"`
	Version          int            `gorm:"not null" json:"version"`
	Filename         string         `gorm:"size:255;not null" json:"filename"`
	OriginalFilename string         `gorm:"size:500;not null" json:"original_filename"`
	MimeType         string         `gorm:"size:100;not null" json:"mime_type"`
	Size             int64          `gorm:"not null" json:"size"`
	FilePath         string         `gorm:"size:2000;not null" json:"file_path"`
	Checksum         string         `gorm:"size:64;not null" json:"checksum"`
	UploadedByID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"uploaded_by_id"`
	UploadedBy       *User          `gorm:"foreignKey:UploadedByID;constraint:OnDelete:RESTRICT" json:"uploaded_by,omitempty"`
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the MediaVersion model
func (MediaVersion) TableName() string {
	return "media_versions"
}

// IsCurrent checks if this version is the current/active version
func (mv *MediaVersion) IsCurrent(mediaCurrentVersion int) bool {
	return mv.Version == mediaCurrentVersion
}

// IsDeleted checks if this version has been soft-deleted
func (mv *MediaVersion) IsDeleted() bool {
	return mv.DeletedAt.Valid
}

// MediaVersionResponse represents a media version in API responses
type MediaVersionResponse struct {
	ID               uuid.UUID `json:"id"`
	MediaID          uuid.UUID `json:"media_id"`
	Version          int       `json:"version"`
	Filename         string    `json:"filename"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	Size             int64     `json:"size"`
	Checksum         string    `json:"checksum"`
	UploadedByID     uuid.UUID `json:"uploaded_by_id"`
	IsCurrent        bool      `json:"is_current"`
	CreatedAt        time.Time `json:"created_at"`
}

// ToResponse converts a MediaVersion to MediaVersionResponse
func (mv *MediaVersion) ToResponse(currentVersion int) *MediaVersionResponse {
	return &MediaVersionResponse{
		ID:               mv.ID,
		MediaID:          mv.MediaID,
		Version:          mv.Version,
		Filename:         mv.Filename,
		OriginalFilename: mv.OriginalFilename,
		MimeType:         mv.MimeType,
		Size:             mv.Size,
		Checksum:         mv.Checksum,
		UploadedByID:     mv.UploadedByID,
		IsCurrent:        mv.Version == currentVersion,
		CreatedAt:        mv.CreatedAt,
	}
}

// VersionHistoryResponse represents the version history listing
type VersionHistoryResponse struct {
	MediaID        uuid.UUID              `json:"media_id"`
	CurrentVersion int                    `json:"current_version"`
	Versions       []*MediaVersionResponse `json:"versions"`
	Total          int64                   `json:"total"`
}
