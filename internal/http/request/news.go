package request

import (
	"encoding/json"
)

// CreateNewsRequest represents a request to create a news article
type CreateNewsRequest struct {
	Title    string          `json:"title" validate:"required,max=255"`
	Content  string          `json:"content" validate:"required"`
	Excerpt  string          `json:"excerpt" validate:"omitempty,max=500"`
	Tags     json.RawMessage `json:"tags,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// UpdateNewsRequest represents a request to update a news article
type UpdateNewsRequest struct {
	Title    string          `json:"title" validate:"omitempty,max=255"`
	Content  string          `json:"content" validate:"omitempty"`
	Excerpt  string          `json:"excerpt" validate:"omitempty,max=500"`
	Status   string          `json:"status" validate:"omitempty,oneof=draft published archived"`
	Tags     json.RawMessage `json:"tags,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// NewsListQuery represents query parameters for listing news articles
type NewsListQuery struct {
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset int    `query:"offset" validate:"omitempty,min=0"`
	Status string `query:"status" validate:"omitempty,oneof=draft published archived"`
}

// UpdateNewsStatusRequest represents a request to update news status
type UpdateNewsStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=draft published archived"`
}

// Validate validates the CreateNewsRequest struct
func (r *CreateNewsRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the UpdateNewsRequest struct
func (r *UpdateNewsRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the NewsListQuery struct
func (q *NewsListQuery) Validate() error {
	return validate.Struct(q)
}

// Validate validates the UpdateNewsStatusRequest struct
func (r *UpdateNewsStatusRequest) Validate() error {
	return validate.Struct(r)
}

// GetLimit returns the limit with default fallback
func (q *NewsListQuery) GetLimit() int {
	if q.Limit <= 0 {
		return 20 // default limit
	}
	if q.Limit > 100 {
		return 100 // max limit
	}
	return q.Limit
}

// GetOffset returns the offset with default fallback
func (q *NewsListQuery) GetOffset() int {
	if q.Offset < 0 {
		return 0
	}
	return q.Offset
}
