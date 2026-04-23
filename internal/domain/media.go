package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MediaDisk represents the storage backend type
type MediaDisk string

const (
	// MediaDiskLocal indicates local filesystem storage
	MediaDiskLocal MediaDisk = "local"
	// MediaDiskS3 indicates AWS S3 storage
	MediaDiskS3 MediaDisk = "s3"
	// MediaDiskMinIO indicates MinIO storage
	MediaDiskMinIO MediaDisk = "minio"
)

// IsValidMediaDisk checks if the disk type is valid
func IsValidMediaDisk(disk MediaDisk) bool {
	switch disk {
	case MediaDiskLocal, MediaDiskS3, MediaDiskMinIO:
		return true
	default:
		return false
	}
}

// Media represents a media file in the system
type Media struct {
	ID               uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ModelType        string             `gorm:"type:varchar(255);not null;index:idx_media_model" json:"model_type"`
	ModelID          uuid.UUID          `gorm:"type:uuid;not null;index:idx_media_model" json:"model_id"`
	CollectionName   string             `gorm:"type:varchar(255);default:'default';not null;index" json:"collection_name"`
	Disk             string             `gorm:"type:varchar(50);default:'local';not null" json:"disk"`
	Filename         string             `gorm:"type:varchar(255);not null;uniqueIndex:idx_media_filename_disk" json:"filename"`
	OriginalFilename string             `gorm:"type:varchar(500);not null" json:"original_filename"`
	MimeType         string             `gorm:"type:varchar(100);not null;index" json:"mime_type"`
	Size             int64              `gorm:"not null" json:"size"`
	Path             string             `gorm:"type:varchar(2000);not null" json:"path"`
	Metadata         datatypes.JSONMap  `gorm:"type:jsonb" json:"metadata,omitempty"`
	CustomProperties datatypes.JSONMap  `gorm:"type:jsonb" json:"custom_properties,omitempty"`
	UploadedByID     uuid.UUID          `gorm:"type:uuid;not null;index" json:"uploaded_by_id"`
	UploadedBy       *User              `gorm:"foreignKey:UploadedByID;constraint:OnDelete:RESTRICT" json:"uploaded_by,omitempty"`
	Conversions      []*MediaConversion `gorm:"foreignKey:MediaID;constraint:OnDelete:CASCADE" json:"conversions,omitempty"`
	CreatedAt        time.Time          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time          `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt     `gorm:"index" json:"-"`
	OrphanedAt       *time.Time         `gorm:"index" json:"orphaned_at,omitempty"`
}

// TableName returns the table name for the Media model
func (Media) TableName() string {
	return "media"
}

// MediaConversion represents a generated file variant (thumbnail, preview, etc.)
type MediaConversion struct {
	ID        uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MediaID   uuid.UUID         `gorm:"type:uuid;not null" json:"media_id"`
	Name      string            `gorm:"type:varchar(255);not null;uniqueIndex:idx_media_conv_name,composite:media_id" json:"name"`
	Disk      string            `gorm:"type:varchar(50);default:'local'" json:"disk"`
	Path      string            `gorm:"type:varchar(2000);not null" json:"path"`
	Size      int64             `gorm:"not null" json:"size"`
	Metadata  datatypes.JSONMap `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt time.Time         `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName returns the table name for the MediaConversion model
func (MediaConversion) TableName() string {
	return "media_conversions"
}

// MediaResponse represents a media item in API responses
type MediaResponse struct {
	ID               uuid.UUID                  `json:"id"`
	ModelType        string                     `json:"model_type"`
	CollectionName   string                     `json:"collection_name"`
	Filename         string                     `json:"filename"`
	MimeType         string                     `json:"mime_type"`
	Size             int64                      `json:"size"`
	URL              string                     `json:"url"`
	Conversions      []*MediaConversionResponse `json:"conversions,omitempty"`
	CustomProperties map[string]interface{}     `json:"custom_properties,omitempty"`
	CreatedAt        time.Time                  `json:"created_at"`
}

// MediaConversionResponse represents a media conversion in API responses
type MediaConversionResponse struct {
	Name     string                 `json:"name"`
	Size     int64                  `json:"size"`
	URL      string                 `json:"url"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToResponse converts a Media to MediaResponse
func (m *Media) ToResponse() *MediaResponse {
	conversions := make([]*MediaConversionResponse, len(m.Conversions))
	for i, c := range m.Conversions {
		conversions[i] = c.ToResponse()
	}

	return &MediaResponse{
		ID:               m.ID,
		ModelType:        m.ModelType,
		CollectionName:   m.CollectionName,
		Filename:         m.OriginalFilename,
		MimeType:         m.MimeType,
		Size:             m.Size,
		Conversions:      conversions,
		CustomProperties: m.CustomProperties,
		CreatedAt:        m.CreatedAt,
	}
}

// ToResponse converts a MediaConversion to MediaConversionResponse
func (c *MediaConversion) ToResponse() *MediaConversionResponse {
	return &MediaConversionResponse{
		Name:     c.Name,
		Size:     c.Size,
		Metadata: c.Metadata,
	}
}

// IsImage checks if the media is an image based on MIME type
func (m *Media) IsImage() bool {
	return len(m.MimeType) > 6 && m.MimeType[:6] == "image/"
}

// IsVideo checks if the media is a video based on MIME type
func (m *Media) IsVideo() bool {
	return len(m.MimeType) > 6 && m.MimeType[:6] == "video/"
}

// IsAudio checks if the media is an audio file based on MIME type
func (m *Media) IsAudio() bool {
	return len(m.MimeType) > 6 && m.MimeType[:6] == "audio/"
}

// IsDocument checks if the media is a document based on MIME type
func (m *Media) IsDocument() bool {
	return len(m.MimeType) > 11 && m.MimeType[:11] == "application"
}

// IsOrphaned checks if the media has been marked for cleanup
func (m *Media) IsOrphaned() bool {
	return m.OrphanedAt != nil
}

// MarkOrphaned marks the media as orphaned for cleanup
func (m *Media) MarkOrphaned() {
	now := time.Now()
	m.OrphanedAt = &now
}

// GetImageDimensions returns width and height from metadata if available
func (m *Media) GetImageDimensions() (width, height int) {
	if m.Metadata == nil {
		return 0, 0
	}
	if w, ok := m.Metadata["width"].(float64); ok {
		width = int(w)
	}
	if h, ok := m.Metadata["height"].(float64); ok {
		height = int(h)
	}
	return width, height
}

// MediaDownload represents a media download audit record
type MediaDownload struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MediaID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"media_id"`
	DownloadedByID *uuid.UUID `gorm:"type:uuid;index" json:"downloaded_by_id,omitempty"`
	DownloadedAt   time.Time  `gorm:"autoCreateTime;index" json:"downloaded_at"`
	IPAddress      string     `gorm:"type:varchar(45)" json:"ip_address,omitempty"`
	UserAgent      string     `gorm:"type:varchar(2000)" json:"user_agent,omitempty"`
}

// TableName returns the table name for the MediaDownload model
func (MediaDownload) TableName() string {
	return "media_downloads"
}
