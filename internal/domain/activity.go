package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ActionType constants for Activity ActionType field.
const (
	ActivityActionCreated  = "created"
	ActivityActionUpdated  = "updated"
	ActivityActionDeleted  = "deleted"
	ActivityActionPublished = "published"
	ActivityActionPaid     = "paid"
)

// ResourceType constants for Activity ResourceType field.
const (
	ActivityResourceUser    = "user"
	ActivityResourceInvoice = "invoice"
	ActivityResourceNews    = "news"
	ActivityResourceComment = "comment"
)

// validActionTypes is a lookup map for valid Activity ActionType values.
var validActionTypes = map[string]bool{
	ActivityActionCreated:  true,
	ActivityActionUpdated:  true,
	ActivityActionDeleted:  true,
	ActivityActionPublished: true,
	ActivityActionPaid:     true,
}

// validResourceTypes is a lookup map for valid Activity ResourceType values.
var validResourceTypes = map[string]bool{
	ActivityResourceUser:    true,
	ActivityResourceInvoice: true,
	ActivityResourceNews:    true,
	ActivityResourceComment: true,
}

// IsValidActionType returns true if the given string is a valid ActionType.
func IsValidActionType(t string) bool {
	return validActionTypes[t]
}

// IsValidResourceType returns true if the given string is a valid ResourceType.
func IsValidResourceType(t string) bool {
	return validResourceTypes[t]
}

// Activity represents an action performed by a user on a resource.
type Activity struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ActorID        uuid.UUID      `gorm:"type:uuid;not null;index"`
	ActionType     string         `gorm:"size:50;not null;index"`
	ResourceType   string         `gorm:"size:50;not null;index"`
	ResourceID     string         `gorm:"size:100;not null;index"`
	OrganizationID *uuid.UUID     `gorm:"type:uuid;index"`
	Metadata       datatypes.JSON `gorm:"type:jsonb"`
	ArchivedAt     *time.Time     `gorm:""`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
	CreatedAt      time.Time      `gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime"`
}

// TableName returns the table name for the Activity model.
func (Activity) TableName() string {
	return "activities"
}

// IsArchived returns true if the activity has been archived by the 90-day reaper.
func (a *Activity) IsArchived() bool {
	return a.ArchivedAt != nil
}

// Archive sets ArchivedAt to the current time if not already archived.
func (a *Activity) Archive() {
	if a.ArchivedAt == nil {
		now := time.Now()
		a.ArchivedAt = &now
	}
}

// ToResponse converts Activity to ActivityResponse.
// ActorName, IsRead, and IsFollowing are computed at query time and must be provided separately.
func (a *Activity) ToResponse(actorName string, isRead bool, isFollowing bool) ActivityResponse {
	var metadata json.RawMessage
	if len(a.Metadata) > 0 {
		metadata = json.RawMessage(a.Metadata)
	}

	var archivedAtStr *string
	if a.ArchivedAt != nil {
		formatted := a.ArchivedAt.Format(time.RFC3339)
		archivedAtStr = &formatted
	}

	var orgIDStr *string
	if a.OrganizationID != nil {
		s := a.OrganizationID.String()
		orgIDStr = &s
	}

	return ActivityResponse{
		ID:             a.ID.String(),
		ActorID:        a.ActorID.String(),
		ActorName:      actorName,
		ActionType:     a.ActionType,
		ResourceType:   a.ResourceType,
		ResourceID:     a.ResourceID,
		OrganizationID: orgIDStr,
		Metadata:       metadata,
		IsRead:         isRead,
		IsFollowing:    isFollowing,
		ArchivedAt:     archivedAtStr,
		CreatedAt:      a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      a.UpdatedAt.Format(time.RFC3339),
	}
}

// ActivityResponse represents an activity in API responses.
type ActivityResponse struct {
	ID             string          `json:"id"`
	ActorID        string          `json:"actor_id"`
	ActorName      string          `json:"actor_name"`
	ActionType     string          `json:"action_type"`
	ResourceType   string          `json:"resource_type"`
	ResourceID     string          `json:"resource_id"`
	OrganizationID *string         `json:"organization_id,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	IsRead         bool            `json:"is_read"`
	IsFollowing    bool            `json:"is_following"`
	ArchivedAt     *string         `json:"archived_at,omitempty"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

// ActivityListResponse represents a paginated list of activities.
type ActivityListResponse struct {
	Activities []ActivityResponse `json:"activities"`
	Total      int64              `json:"total"`
	UnreadCount int64             `json:"unread_count"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
}

// ActivityRead tracks which users have read which activities.
type ActivityRead struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	ActivityID uuid.UUID `gorm:"type:uuid;not null;index"`
	ReadAt     time.Time `gorm:"not null;default:now()"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// TableName returns the table name for the ActivityRead model.
func (ActivityRead) TableName() string {
	return "activity_reads"
}

// ActivityFollow represents a user following a specific resource.
type ActivityFollow struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	ResourceType string    `gorm:"size:50;not null;index"`
	ResourceID   string    `gorm:"size:100;not null;index"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName returns the table name for the ActivityFollow model.
func (ActivityFollow) TableName() string {
	return "activity_follows"
}

// ToResponse converts ActivityFollow to ActivityFollowResponse.
func (f *ActivityFollow) ToResponse() ActivityFollowResponse {
	return ActivityFollowResponse{
		ID:           f.ID.String(),
		ResourceType: f.ResourceType,
		ResourceID:   f.ResourceID,
		CreatedAt:    f.CreatedAt.Format(time.RFC3339),
	}
}

// ActivityFollowResponse represents a follow relationship in API responses.
type ActivityFollowResponse struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	CreatedAt    string `json:"created_at"`
}