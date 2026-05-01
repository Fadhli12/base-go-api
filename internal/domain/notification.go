package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the category of a notification
type NotificationType string

const (
	// NotificationTypeMention is sent when a user is mentioned
	NotificationTypeMention NotificationType = "mention"
	// NotificationTypeAssignment is sent when a task is assigned to a user
	NotificationTypeAssignment NotificationType = "assignment"
	// NotificationTypeSystem is a system-generated notification
	NotificationTypeSystem NotificationType = "system"
	// NotificationTypeInvoiceCreated is sent when an invoice is created
	NotificationTypeInvoiceCreated NotificationType = "invoice.created"
	// NotificationTypeNewsPublished is sent when a news article is published
	NotificationTypeNewsPublished NotificationType = "news.published"
)

// IsValidNotificationType returns true if the given string is a valid NotificationType
func IsValidNotificationType(t string) bool {
	switch NotificationType(t) {
	case NotificationTypeMention,
		NotificationTypeAssignment,
		NotificationTypeSystem,
		NotificationTypeInvoiceCreated,
		NotificationTypeNewsPublished:
		return true
	default:
		return false
	}
}

// Notification represents a user notification.
// IMPORTANT: This table has NO soft delete - records are retained permanently
// for audit trail. Use archived_at to hide notifications from the active view.
type Notification struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Type       NotificationType `gorm:"size:30;not null;index" json:"type"`
	Title      string     `gorm:"size:255;not null" json:"title"`
	Message    string     `gorm:"type:text;not null" json:"message"`
	ActionURL  string     `gorm:"size:500" json:"action_url,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName returns the table name for the Notification model
func (Notification) TableName() string {
	return "notifications"
}

// NotificationResponse represents a notification in API responses
type NotificationResponse struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	ActionURL  string     `json:"action_url,omitempty"`
	ReadAt     *string    `json:"read_at,omitempty"`
	ArchivedAt *string    `json:"archived_at,omitempty"`
	CreatedAt  string     `json:"created_at"`
	UpdatedAt  string     `json:"updated_at"`
}

// ToResponse converts a Notification to NotificationResponse
func (n *Notification) ToResponse() NotificationResponse {
	var readAtStr *string
	if n.ReadAt != nil {
		formatted := n.ReadAt.Format(time.RFC3339)
		readAtStr = &formatted
	}

	var archivedAtStr *string
	if n.ArchivedAt != nil {
		formatted := n.ArchivedAt.Format(time.RFC3339)
		archivedAtStr = &formatted
	}

	return NotificationResponse{
		ID:         n.ID.String(),
		UserID:     n.UserID.String(),
		Type:       string(n.Type),
		Title:      n.Title,
		Message:    n.Message,
		ActionURL:  n.ActionURL,
		ReadAt:     readAtStr,
		ArchivedAt: archivedAtStr,
		CreatedAt:  n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  n.UpdatedAt.Format(time.RFC3339),
	}
}

// IsRead returns true if the notification has been read
func (n *Notification) IsRead() bool {
	return n.ReadAt != nil
}

// IsArchived returns true if the notification has been archived
func (n *Notification) IsArchived() bool {
	return n.ArchivedAt != nil
}

// MarkRead sets ReadAt to the current time if not already read
func (n *Notification) MarkRead() {
	if n.ReadAt == nil {
		now := time.Now()
		n.ReadAt = &now
	}
}

// Archive sets ArchivedAt to the current time if not already archived
func (n *Notification) Archive() {
	if n.ArchivedAt == nil {
		now := time.Now()
		n.ArchivedAt = &now
	}
}
