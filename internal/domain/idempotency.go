// Package domain provides domain entities and DTOs for the idempotency key feature.
package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Idempotency status constants
const (
	IdempotencyStatusProcessing = "processing"
	IdempotencyStatusCompleted  = "completed"
	IdempotencyStatusConflict   = "conflict"
)

// IdempotencyRecord stores the mapping between an idempotency key and its response.
// Redis is used for fast lookup (TTL-based expiration), while PostgreSQL provides
// durability and audit capability. The reaper soft-deletes expired records.
type IdempotencyRecord struct {
	ID               uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IdempotencyKey   string          `gorm:"type:varchar(128);not null" json:"idempotency_key"`
	UserID           uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	OrganizationID   *uuid.UUID      `gorm:"type:uuid;index" json:"organization_id"`
	HTTPMethod       string          `gorm:"type:varchar(10);not null" json:"http_method"`
	RequestPath      string          `gorm:"type:varchar(500);not null" json:"request_path"`
	RequestHash      string          `gorm:"type:varchar(64);not null" json:"request_hash"` // SHA-256 hex
	Status           string          `gorm:"type:varchar(20);not null;default:'processing'" json:"status"`
	ResponseStatusCode int           `gorm:"integer" json:"response_status_code"`
	ResponseBody     string          `gorm:"type:text" json:"response_body,omitempty"`
	ResponseBodySize int             `gorm:"integer;not null;default:0" json:"response_body_size"`
	ResponseHeaders  MapStringString `gorm:"type:jsonb" json:"response_headers,omitempty"` // Selected headers for replay
	ExpiresAt        time.Time       `gorm:"not null;index" json:"expires_at"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for IdempotencyRecord.
func (IdempotencyRecord) TableName() string {
	return "idempotency_keys"
}

// MapStringString is a helper type for storing map[string]string in JSONB.
type MapStringString map[string]string

// IdempotencyRecordResponse is the DTO for returning idempotency records in API responses.
type IdempotencyRecordResponse struct {
	ID               uuid.UUID `json:"id"`
	IdempotencyKey   string    `json:"idempotency_key"`
	UserID           uuid.UUID `json:"user_id"`
	OrganizationID   *string   `json:"organization_id,omitempty"`
	HTTPMethod       string    `json:"http_method"`
	RequestPath      string    `json:"request_path"`
	Status           string    `json:"status"`
	ResponseStatusCode int     `json:"response_status_code"`
	ResponseBodySize  int      `json:"response_body_size"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// ToResponse converts an IdempotencyRecord to its response DTO.
func (r *IdempotencyRecord) ToResponse() *IdempotencyRecordResponse {
	var orgID *string
	if r.OrganizationID != nil {
		id := r.OrganizationID.String()
		orgID = &id
	}
	return &IdempotencyRecordResponse{
		ID:                 r.ID,
		IdempotencyKey:     r.IdempotencyKey,
		UserID:             r.UserID,
		OrganizationID:     orgID,
		HTTPMethod:         r.HTTPMethod,
		RequestPath:        r.RequestPath,
		Status:             r.Status,
		ResponseStatusCode: r.ResponseStatusCode,
		ResponseBodySize:    r.ResponseBodySize,
		ExpiresAt:          r.ExpiresAt,
		CreatedAt:          r.CreatedAt,
	}
}

// IdempotencyListResponse is the paginated response for listing idempotency records.
type IdempotencyListResponse struct {
	Records []*IdempotencyRecordResponse `json:"data"`
	Meta    *ListMeta                    `json:"meta,omitempty"`
}

// ListMeta holds pagination metadata.
type ListMeta struct {
	Total   int64 `json:"total"`
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
}

// IsExpired returns true if the record has passed its expiration time.
func (r *IdempotencyRecord) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// IsProcessing returns true if the record is in processing state.
func (r *IdempotencyRecord) IsProcessing() bool {
	return r.Status == IdempotencyStatusProcessing
}

// IsCompleted returns true if the record is in completed state.
func (r *IdempotencyRecord) IsCompleted() bool {
	return r.Status == IdempotencyStatusCompleted
}