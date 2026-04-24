package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmailTemplate represents a reusable email template with HTML and text versions.
// Templates support variable substitution using Go template syntax.
type EmailTemplate struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"size:100;not null;uniqueIndex" json:"name"`                  // Template identifier (welcome, password-reset, invitation)
	Subject     string         `gorm:"size:255;not null" json:"subject"`                          // Email subject with template variables
	HTMLContent string         `gorm:"type:text;not null" json:"html_content"`                     // HTML email body
	TextContent string         `gorm:"type:text" json:"text_content"`                              // Plain text email body (optional)
	Category    string         `gorm:"size:50;not null;index" json:"category"`                     // Template category (auth, notification, marketing)
	IsActive    bool          `gorm:"default:true;index" json:"is_active"`                        // Whether template is active for use
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"` // Soft delete for template management
}

// TableName returns the table name for EmailTemplate
func (EmailTemplate) TableName() string {
	return "email_templates"
}

// EmailTemplateResponse represents an email template response without sensitive internal fields
type EmailTemplateResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Subject     string    `json:"subject"`
	HTMLContent string    `json:"html_content"`
	TextContent string    `json:"text_content,omitempty"`
	Category    string    `json:"category"`
	IsActive    bool     `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToResponse converts EmailTemplate to EmailTemplateResponse
func (t *EmailTemplate) ToResponse() EmailTemplateResponse {
	return EmailTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Subject:     t.Subject,
		HTMLContent: t.HTMLContent,
		TextContent: t.TextContent,
		Category:    t.Category,
		IsActive:    t.IsActive,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// Template category constants
const (
	TemplateCategoryAuth         = "auth"         // Authentication emails (welcome, password-reset)
	TemplateCategoryNotification = "notification" // System notifications (alerts, updates)
	TemplateCategoryMarketing   = "marketing"   // Marketing emails (newsletters, promotions)
)