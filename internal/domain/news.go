package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NewsStatus represents the status of a news article
type NewsStatus string

const (
	// NewsStatusDraft indicates the news is in draft state
	NewsStatusDraft NewsStatus = "draft"
	// NewsStatusPublished indicates the news is published
	NewsStatusPublished NewsStatus = "published"
	// NewsStatusArchived indicates the news is archived
	NewsStatusArchived NewsStatus = "archived"
)

// IsValidNewsStatus checks if the status is valid
func IsValidNewsStatus(status NewsStatus) bool {
	switch status {
	case NewsStatusDraft, NewsStatusPublished, NewsStatusArchived:
		return true
	default:
		return false
	}
}

// News represents a news article entity in the system
type News struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AuthorID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"author_id"`
	Author      User           `gorm:"foreignKey:AuthorID;references:ID;constraint:OnDelete:CASCADE" json:"author,omitempty"`
	Title       string         `gorm:"size:255;not null" json:"title"`
	Slug        string         `gorm:"size:255;not null;uniqueIndex" json:"slug"`
	Content     string         `gorm:"type:text;not null" json:"content"`
	Excerpt     string         `gorm:"size:500" json:"excerpt"`
	Status      NewsStatus     `gorm:"size:20;not null;default:'draft';index" json:"status"`
	Tags        datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tags"`
	Metadata    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the News model
func (News) TableName() string {
	return "news"
}

// NewsResponse represents a news article in API responses
type NewsResponse struct {
	ID          string     `json:"id"`
	AuthorID    string     `json:"author_id"`
	AuthorEmail string     `json:"author_email,omitempty"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Content     string     `json:"content"`
	Excerpt     string     `json:"excerpt"`
	Status      NewsStatus `json:"status"`
	Tags        any        `json:"tags,omitempty"`
	Metadata    any        `json:"metadata,omitempty"`
	PublishedAt *string    `json:"published_at,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

// ToResponse converts a News to NewsResponse
func (n *News) ToResponse() NewsResponse {
	var publishedAtStr *string
	if n.PublishedAt != nil {
		formatted := n.PublishedAt.Format(time.RFC3339)
		publishedAtStr = &formatted
	}

	resp := NewsResponse{
		ID:        n.ID.String(),
		AuthorID:  n.AuthorID.String(),
		Title:     n.Title,
		Slug:      n.Slug,
		Content:   n.Content,
		Excerpt:   n.Excerpt,
		Status:    n.Status,
		Tags:      n.Tags,
		Metadata:  n.Metadata,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
	}

	if publishedAtStr != nil {
		resp.PublishedAt = publishedAtStr
	}

	// Include author email if loaded
	if n.Author.Email != "" {
		resp.AuthorEmail = n.Author.Email
	}

	return resp
}

// CanTransitionTo checks if the news can transition to the given status
func (n *News) CanTransitionTo(newStatus NewsStatus) bool {
	// Define allowed transitions
	switch n.Status {
	case NewsStatusDraft:
		// Draft can go to published or archived
		return newStatus == NewsStatusPublished || newStatus == NewsStatusArchived
	case NewsStatusPublished:
		// Published can go to archived
		return newStatus == NewsStatusArchived
	case NewsStatusArchived:
		// Archived can go back to draft
		return newStatus == NewsStatusDraft
	default:
		return false
	}
}

// SetPublishedAt sets the published_at timestamp when transitioning to published status
func (n *News) SetPublishedAt() {
	now := time.Now()
	n.PublishedAt = &now
}

// IsPublished checks if the news is published
func (n *News) IsPublished() bool {
	return n.Status == NewsStatusPublished
}

// IsOwnedBy checks if the news is owned by the given user
func (n *News) IsOwnedBy(userID uuid.UUID) bool {
	return n.AuthorID == userID
}
