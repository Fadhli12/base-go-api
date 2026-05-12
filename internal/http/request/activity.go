package request

import (
	"time"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// ListActivitiesRequest represents query parameters for listing activities.
type ListActivitiesRequest struct {
	Limit        int     `query:"limit"`
	Offset       int     `query:"offset"`
	ResourceType string  `query:"resource_type"`
	ResourceID   string  `query:"resource_id"`
	ActionType   string  `query:"action_type"`
	Unread       *bool   `query:"unread"`    // pointer to distinguish missing from false
	Following    *bool   `query:"following"` // pointer to distinguish missing from false
	Since        string  `query:"since"`     // ISO 8601 timestamp for archived activities
}

// FollowResourceRequest represents a request to follow a resource.
type FollowResourceRequest struct {
	ResourceType string `json:"resource_type" validate:"required,oneof=user invoice news comment media"`
	ResourceID   string `json:"resource_id" validate:"required"`
}

// Validate validates the ListActivitiesRequest fields.
func (r *ListActivitiesRequest) Validate() error {
	if r.ResourceType != "" && !domain.IsValidResourceType(r.ResourceType) {
		return apperrors.NewAppError("VALIDATION_ERROR", "invalid resource_type filter: "+r.ResourceType, 422)
	}
	if r.ActionType != "" && !domain.IsValidActionType(r.ActionType) {
		return apperrors.NewAppError("VALIDATION_ERROR", "invalid action_type filter: "+r.ActionType, 422)
	}
	if r.Since != "" {
		if _, err := time.Parse(time.RFC3339, r.Since); err != nil {
			return apperrors.NewAppError("VALIDATION_ERROR", "since must be a valid ISO 8601 timestamp", 422)
		}
	}
	// Validate resource_id is a valid UUID if provided with resource_type
	if r.ResourceID != "" {
		if _, err := uuid.Parse(r.ResourceID); err != nil {
			return apperrors.NewAppError("VALIDATION_ERROR", "resource_id must be a valid UUID", 422)
		}
	}
	return nil
}

// Validate validates the FollowResourceRequest fields.
func (r *FollowResourceRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if !domain.IsValidResourceType(r.ResourceType) {
		return apperrors.NewAppError("VALIDATION_ERROR", "invalid resource_type: "+r.ResourceType, 422)
	}
	if _, err := uuid.Parse(r.ResourceID); err != nil {
		return apperrors.NewAppError("VALIDATION_ERROR", "resource_id must be a valid UUID", 422)
	}
	return nil
}