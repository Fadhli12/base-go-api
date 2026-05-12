package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var taggableTypes = map[string]bool{
	"news":    true,
	"invoice": true,
	"media":   true,
}

func IsValidTaggableType(t string) bool {
	return taggableTypes[t]
}

type Tag struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	Name           string         `gorm:"size:100;not null" json:"name"`
	Slug           string         `gorm:"size:120;not null" json:"slug"`
	Color          string         `gorm:"size:7" json:"color,omitempty"`
	UsageCount     int            `gorm:"default:0;not null" json:"usage_count"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Tag) TableName() string { return "tags" }

func (t *Tag) ToResponse() TagResponse {
	return TagResponse{
		ID:             t.ID,
		OrganizationID: t.OrganizationID,
		Name:           t.Name,
		Slug:           t.Slug,
		Color:          t.Color,
		UsageCount:     t.UsageCount,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

type EntityTag struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	EntityType     string    `gorm:"size:50;not null" json:"entity_type"`
	EntityID       uuid.UUID `gorm:"type:uuid;not null" json:"entity_id"`
	TagID          uuid.UUID `gorm:"type:uuid;not null" json:"tag_id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null" json:"organization_id"`
	CreatedBy      uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (EntityTag) TableName() string { return "entity_tags" }

var slugNonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

func GenerateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = slugNonAlphaNumeric.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if slug == "" {
		return "tag-" + uuid.New().String()[:8]
	}
	return slug
}

type TagResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Color          string    `json:"color,omitempty"`
	UsageCount     int       `json:"usage_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TagListResponse struct {
	Tags  []TagResponse `json:"tags"`
	Total int64         `json:"total"`
}

type EntityTagsResponse struct {
	EntityID   uuid.UUID     `json:"entity_id"`
	EntityType string        `json:"entity_type"`
	Tags       []TagResponse `json:"tags"`
}

type AutocompleteResponse struct {
	Tags []TagResponse `json:"tags"`
}

type BulkAttachResult struct {
	Attached []TagResponse `json:"attached"`
	Skipped  []TagResponse `json:"skipped"`
	Errors   []BulkError   `json:"errors,omitempty"`
}

type BulkDetachResult struct {
	Detached []TagResponse `json:"detached"`
	Skipped  []TagResponse `json:"skipped"`
	Errors   []BulkError   `json:"errors,omitempty"`
}

type BulkError struct {
	TagID   uuid.UUID `json:"tag_id"`
	Message string    `json:"message"`
}