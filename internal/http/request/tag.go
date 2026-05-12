package request

import (
	"regexp"

	"github.com/google/uuid"
	apperrors "github.com/example/go-api-base/pkg/errors"
)

var hexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type CreateTagRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=100"`
	Color string `json:"color" validate:"omitempty,len=7"`
}

type UpdateTagRequest struct {
	Name  string `json:"name" validate:"omitempty,min=1,max=100"`
	Color string `json:"color" validate:"omitempty,len=7"`
}

type AttachTagRequest struct {
	TagIDs     []uuid.UUID `json:"tag_ids" validate:"required,min=1,max=50,dive,required"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
}

type DetachTagRequest struct {
	TagIDs     []uuid.UUID `json:"tag_ids" validate:"required,min=1,max=50,dive,required"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
}

func (r *CreateTagRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if r.Color != "" && !hexColorRegex.MatchString(r.Color) {
		return apperrors.NewAppError("VALIDATION_ERROR", "color must be a valid hex color code (e.g. #FF5733)", 422)
	}
	return nil
}

func (r *UpdateTagRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if r.Color != "" && !hexColorRegex.MatchString(r.Color) {
		return apperrors.NewAppError("VALIDATION_ERROR", "color must be a valid hex color code (e.g. #FF5733)", 422)
	}
	return nil
}

func (r *AttachTagRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if _, err := uuid.Parse(r.EntityID); err != nil {
		return apperrors.NewAppError("VALIDATION_ERROR", "entity_id must be a valid UUID", 422)
	}
	return nil
}

func (r *DetachTagRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if _, err := uuid.Parse(r.EntityID); err != nil {
		return apperrors.NewAppError("VALIDATION_ERROR", "entity_id must be a valid UUID", 422)
	}
	return nil
}