package request

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Constants for media request validation
const (
	// MaxUploadSize is the maximum allowed file size (100MB)
	MaxUploadSize = 100 * 1024 * 1024

	// DefaultPageLimit is the default number of items per page
	DefaultPageLimit = 20

	// MaxPageLimit is the maximum number of items per page
	MaxPageLimit = 100

	// DefaultSignedURLExpiry is the default expiry in seconds (1 hour)
	DefaultSignedURLExpiry = 3600

	// MaxSignedURLExpiry is the maximum expiry in seconds (24 hours)
	MaxSignedURLExpiry = 86400
)

// UploadMediaRequest represents a request to upload media
// This is used for binding multipart form data
// Note: The actual file is handled separately by Echo's multipart form handling
type UploadMediaRequest struct {
	Collection       string          `form:"collection" validate:"omitempty,max=255"`
	CustomProperties json.RawMessage `form:"custom_properties,omitempty" validate:"omitempty,json" swaggertype:"object"`
}

// Validate validates the UploadMediaRequest struct
func (r *UploadMediaRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}

	// Validate custom_properties is valid JSON if provided
	if len(r.CustomProperties) > 0 {
		var props map[string]interface{}
		if err := json.Unmarshal(r.CustomProperties, &props); err != nil {
			return fmt.Errorf("custom_properties must be valid JSON: %w", err)
		}
	}

	return nil
}

// GetCustomProperties parses and returns custom properties as a map
func (r *UploadMediaRequest) GetCustomProperties() (map[string]interface{}, error) {
	if len(r.CustomProperties) == 0 {
		return nil, nil
	}

	var props map[string]interface{}
	if err := json.Unmarshal(r.CustomProperties, &props); err != nil {
		return nil, fmt.Errorf("invalid custom_properties JSON: %w", err)
	}

	return props, nil
}

// ListMediaQuery represents query parameters for listing media
type ListMediaQuery struct {
	Collection string `query:"collection" validate:"omitempty,max=255"`
	MimeType   string `query:"mime_type" validate:"omitempty,max=100"`
	Limit      int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset     int    `query:"offset" validate:"omitempty,min=0"`
	Sort       string `query:"sort" validate:"omitempty,oneof=created_at -created_at size -size filename -filename"`
}

// Validate validates the ListMediaQuery struct
func (q *ListMediaQuery) Validate() error {
	return validate.Struct(q)
}

// GetLimit returns the limit with default fallback
func (q *ListMediaQuery) GetLimit() int {
	if q.Limit <= 0 {
		return DefaultPageLimit
	}
	if q.Limit > MaxPageLimit {
		return MaxPageLimit
	}
	return q.Limit
}

// GetOffset returns the offset with default fallback
func (q *ListMediaQuery) GetOffset() int {
	if q.Offset < 0 {
		return 0
	}
	return q.Offset
}

// GetSort returns the sort field with default fallback
func (q *ListMediaQuery) GetSort() string {
	if q.Sort == "" {
		return "-created_at"
	}
	return q.Sort
}

// GetSortFieldAndOrder returns the sort field and order (asc/desc)
func (q *ListMediaQuery) GetSortFieldAndOrder() (field string, desc bool) {
	sort := q.GetSort()
	if strings.HasPrefix(sort, "-") {
		return sort[1:], true
	}
	return sort, false
}

// UpdateMediaMetadataRequest represents a request to update media metadata
type UpdateMediaMetadataRequest struct {
	CustomProperties json.RawMessage `json:"custom_properties" validate:"required" swaggertype:"object"`
}

// Validate validates the UpdateMediaMetadataRequest struct
func (r *UpdateMediaMetadataRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}

	// Validate custom_properties is valid JSON
	if len(r.CustomProperties) == 0 {
		return fmt.Errorf("custom_properties is required")
	}

	var props map[string]interface{}
	if err := json.Unmarshal(r.CustomProperties, &props); err != nil {
		return fmt.Errorf("custom_properties must be valid JSON: %w", err)
	}

	return nil
}

// GetCustomProperties parses and returns custom properties as a map
func (r *UpdateMediaMetadataRequest) GetCustomProperties() (map[string]interface{}, error) {
	var props map[string]interface{}
	if err := json.Unmarshal(r.CustomProperties, &props); err != nil {
		return nil, fmt.Errorf("invalid custom_properties JSON: %w", err)
	}
	return props, nil
}

// GetSignedURLRequest represents query parameters for generating a signed URL
type GetSignedURLRequest struct {
	Conversion string `query:"conversion" validate:"omitempty,max=255"`
	ExpiresIn  int    `query:"expires_in" validate:"omitempty,min=60,max=86400"`
}

// Validate validates the GetSignedURLRequest struct
func (r *GetSignedURLRequest) Validate() error {
	return validate.Struct(r)
}

// GetExpiresIn returns the expiry time in seconds with default fallback
func (r *GetSignedURLRequest) GetExpiresIn() int {
	if r.ExpiresIn <= 0 {
		return DefaultSignedURLExpiry
	}
	if r.ExpiresIn > MaxSignedURLExpiry {
		return MaxSignedURLExpiry
	}
	return r.ExpiresIn
}

// DownloadMediaQuery represents query parameters for downloading media
type DownloadMediaQuery struct {
	Conversion string `query:"conversion" validate:"omitempty,max=255"`
	Signature  string `query:"sig" validate:"omitempty,len=64"`
	Expires    int64  `query:"expires" validate:"omitempty,min=1"`
}

// Validate validates the DownloadMediaQuery struct
func (q *DownloadMediaQuery) Validate() error {
	return validate.Struct(q)
}

// AdminListMediaQuery represents query parameters for admin media listing
type AdminListMediaQuery struct {
	ModelType string `query:"model_type" validate:"omitempty,max=255"`
	ModelID   string `query:"model_id" validate:"omitempty,uuid"`
	Deleted   bool   `query:"deleted"`
	Limit     int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset    int    `query:"offset" validate:"omitempty,min=0"`
}

// Validate validates the AdminListMediaQuery struct
func (q *AdminListMediaQuery) Validate() error {
	return validate.Struct(q)
}

// GetLimit returns the limit with default fallback
func (q *AdminListMediaQuery) GetLimit() int {
	if q.Limit <= 0 {
		return DefaultPageLimit
	}
	if q.Limit > MaxPageLimit {
		return MaxPageLimit
	}
	return q.Limit
}

// GetOffset returns the offset with default fallback
func (q *AdminListMediaQuery) GetOffset() int {
	if q.Offset < 0 {
		return 0
	}
	return q.Offset
}

// CleanupMediaRequest represents a request to cleanup orphaned media
type CleanupMediaRequest struct {
	DryRun       bool `json:"dry_run"`
	OrphanedDays int  `json:"orphaned_days" validate:"omitempty,min=1,max=365"`
	Force        bool `json:"force"`
}

// Validate validates the CleanupMediaRequest struct
func (r *CleanupMediaRequest) Validate() error {
	return validate.Struct(r)
}

// GetOrphanedDays returns the orphaned days threshold with default fallback
func (r *CleanupMediaRequest) GetOrphanedDays() int {
	if r.OrphanedDays <= 0 {
		return 30 // Default: 30 days
	}
	return r.OrphanedDays
}

// MediaStatsQuery represents query parameters for media statistics
type MediaStatsQuery struct {
	ModelType string `query:"model_type" validate:"omitempty,max=255"`
	GroupBy   string `query:"groupby" validate:"omitempty,oneof=model_type disk mime_type"`
}

// Validate validates the MediaStatsQuery struct
func (q *MediaStatsQuery) Validate() error {
	return validate.Struct(q)
}
