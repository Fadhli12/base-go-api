package request

import (
	"github.com/example/go-api-base/internal/domain"

	"github.com/go-playground/validator/v10"
)

// CreateWebhookRequest for POST /webhooks
type CreateWebhookRequest struct {
	Name      string   `json:"name" validate:"required,min=1,max=255"`
	URL       string   `json:"url" validate:"required,url,max=500"`
	Events    []string `json:"events" validate:"required,min=1,dive,webhook_event"`
	Active    *bool    `json:"active,omitempty"`
	RateLimit *int     `json:"rate_limit,omitempty" validate:"omitempty,min=1,max=1000"`
}

// UpdateWebhookRequest for PUT /webhooks/:id — all fields optional
type UpdateWebhookRequest struct {
	Name      *string  `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	URL       *string  `json:"url,omitempty" validate:"omitempty,url,max=500"`
	Events    []string `json:"events,omitempty" validate:"omitempty,min=1,dive,webhook_event"`
	Active    *bool    `json:"active,omitempty"`
	RateLimit *int     `json:"rate_limit,omitempty" validate:"omitempty,min=1,max=1000"`
}

// ListWebhooksQuery for GET /webhooks query params
type ListWebhooksQuery struct {
	Limit  int     `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset int     `query:"offset" validate:"omitempty,min=0"`
	Active *bool   `query:"active"`
	Event  *string `query:"event" validate:"omitempty,webhook_event"`
}

// ListDeliveriesQuery for GET /webhooks/:id/deliveries query params
type ListDeliveriesQuery struct {
	Limit  int     `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset int     `query:"offset" validate:"omitempty,min=0"`
	Status *string `query:"status" validate:"omitempty,delivery_status"`
	Event  *string `query:"event" validate:"omitempty,webhook_event"`
}

// Validate validates the CreateWebhookRequest
func (r *CreateWebhookRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the UpdateWebhookRequest
func (r *UpdateWebhookRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the ListWebhooksQuery and applies defaults
func (q *ListWebhooksQuery) Validate() error {
	if err := validate.Struct(q); err != nil {
		return err
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	return nil
}

// Validate validates the ListDeliveriesQuery and applies defaults
func (q *ListDeliveriesQuery) Validate() error {
	if err := validate.Struct(q); err != nil {
		return err
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	return nil
}

// HasUpdates returns true if any field is non-nil
func (r *UpdateWebhookRequest) HasUpdates() bool {
	return r.Name != nil || r.URL != nil || len(r.Events) > 0 || r.Active != nil || r.RateLimit != nil
}

// GetLimit returns the limit with default fallback
func (q *ListWebhooksQuery) GetLimit() int {
	if q.Limit <= 0 {
		return 20
	}
	if q.Limit > 100 {
		return 100
	}
	return q.Limit
}

// GetOffset returns the offset with default fallback
func (q *ListWebhooksQuery) GetOffset() int {
	if q.Offset < 0 {
		return 0
	}
	return q.Offset
}

// GetLimit returns the limit with default fallback
func (q *ListDeliveriesQuery) GetLimit() int {
	if q.Limit <= 0 {
		return 20
	}
	if q.Limit > 100 {
		return 100
	}
	return q.Limit
}

// GetOffset returns the offset with default fallback
func (q *ListDeliveriesQuery) GetOffset() int {
	if q.Offset < 0 {
		return 0
	}
	return q.Offset
}

// init registers custom validators
func init() {
	validate.RegisterValidation("webhook_event", func(fl validator.FieldLevel) bool {
		return domain.ValidWebhookEvents[fl.Field().String()]
	})

	validate.RegisterValidation("delivery_status", func(fl validator.FieldLevel) bool {
		switch fl.Field().String() {
		case
			string(domain.WebhookDeliveryStatusQueued),
			string(domain.WebhookDeliveryStatusProcessing),
			string(domain.WebhookDeliveryStatusDelivered),
			string(domain.WebhookDeliveryStatusFailed),
			string(domain.WebhookDeliveryStatusRateLimited):
			return true
		default:
			return false
		}
	})
}
