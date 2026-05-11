package request

import (
	"github.com/google/uuid"
	apperrors "github.com/example/go-api-base/pkg/errors"
)

// CreateCommentRequest represents a request to create a new comment.
type CreateCommentRequest struct {
	Content  string `json:"content" validate:"required,min=1,max=5000"`
	ParentID string `json:"parent_id"` // UUID string, optional for replies
}

// UpdateCommentRequest represents a request to update an existing comment.
type UpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=5000"`
}

// Validate validates the CreateCommentRequest fields.
func (r *CreateCommentRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	// ParentID is optional, but if provided must be valid UUID
	if r.ParentID != "" {
		if _, err := uuid.Parse(r.ParentID); err != nil {
			return apperrors.NewAppError("VALIDATION_ERROR", "parent_id must be a valid UUID", 422)
		}
	}
	return nil
}

// Validate validates the UpdateCommentRequest fields.
func (r *UpdateCommentRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return nil
}