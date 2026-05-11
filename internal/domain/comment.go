package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CommentableTypes defines which entity types support comments.
var commentableTypes = map[string]bool{
	"news":    true,
	"invoice": true,
	"media":   true,
}

// IsValidCommentableType checks whether a given type is registered as commentable.
func IsValidCommentableType(t string) bool {
	return commentableTypes[t]
}

// Comment represents a user comment on a commentable entity (news, invoice, media).
type Comment struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ParentID          *uuid.UUID     `gorm:"type:uuid;index"`
	AuthorID          uuid.UUID      `gorm:"type:uuid;not null;index"`
	OrganizationID    uuid.UUID      `gorm:"type:uuid;not null;index"`
	CommentableType    string         `gorm:"size:50;not null;index"`
	CommentableID      uuid.UUID      `gorm:"type:uuid;not null;index"`
	Content           string         `gorm:"type:text;not null"`
	MentionedUserIDs   datatypes.JSON `gorm:"type:jsonb"`
	EditedAt          *time.Time     `gorm:""`
	IsPinned          bool           `gorm:"default:false;not null"`
	CreatedAt         time.Time      `gorm:"autoCreateTime"`
	UpdatedAt         time.Time      `gorm:"autoUpdateTime"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for the Comment model.
func (Comment) TableName() string {
	return "comments"
}

// ToResponse converts Comment to CommentResponse.
func (c *Comment) ToResponse(authorName string, replyCount int64) CommentResponse {
	var mentionedUserIDs json.RawMessage
	if len(c.MentionedUserIDs) > 0 {
		mentionedUserIDs = json.RawMessage(c.MentionedUserIDs)
	}

	return CommentResponse{
		ID:                c.ID,
		ParentID:          c.ParentID,
		AuthorID:          c.AuthorID,
		AuthorName:        authorName,
		OrganizationID:    c.OrganizationID,
		CommentableType:  c.CommentableType,
		CommentableID:    c.CommentableID,
		Content:          c.Content,
		MentionedUserIDs: mentionedUserIDs,
		IsPinned:         c.IsPinned,
		EditedAt:         c.EditedAt,
		ReplyCount:       replyCount,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

// CommentResponse represents a comment in API responses.
type CommentResponse struct {
	ID                uuid.UUID       `json:"id"`
	ParentID          *uuid.UUID      `json:"parent_id,omitempty"`
	AuthorID          uuid.UUID       `json:"author_id"`
	AuthorName        string          `json:"author_name"`
	OrganizationID    uuid.UUID       `json:"organization_id"`
	CommentableType   string          `json:"commentable_type"`
	CommentableID     uuid.UUID       `json:"commentable_id"`
	Content           string          `json:"content"`
	MentionedUserIDs  json.RawMessage `json:"mentioned_user_ids"`
	IsPinned          bool            `json:"is_pinned"`
	EditedAt          *time.Time      `json:"edited_at,omitempty"`
	ReplyCount        int64           `json:"reply_count"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// CreateCommentRequest represents a request to create a new comment.
type CreateCommentRequest struct {
	Content  string `json:"content" validate:"required,min=1,max=5000"`
	ParentID string `json:"parent_id,omitempty"`
}

// UpdateCommentRequest represents a request to update an existing comment.
type UpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=5000"`
}