package request

import (
	"time"
)

// CreateInvoiceRequest represents a request to create an invoice
type CreateInvoiceRequest struct {
	Customer    string     `json:"customer" validate:"required,max=255"`
	Amount      float64    `json:"amount" validate:"required,gt=0"`
	Description string     `json:"description" validate:"omitempty,max=1000"`
	DueDate     *time.Time `json:"due_date"`
}

// UpdateInvoiceRequest represents a request to update an invoice
type UpdateInvoiceRequest struct {
	Customer    string     `json:"customer" validate:"omitempty,max=255"`
	Amount      float64    `json:"amount" validate:"omitempty,gt=0"`
	Description string     `json:"description" validate:"omitempty,max=1000"`
	DueDate     *time.Time `json:"due_date"`
	Status      string     `json:"status" validate:"omitempty,oneof=draft pending paid cancelled"`
}

// InvoiceListQuery represents query parameters for listing invoices
type InvoiceListQuery struct {
	Limit  int `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset int `query:"offset" validate:"omitempty,min=0"`
}

// Validate validates the CreateInvoiceRequest struct
func (r *CreateInvoiceRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the UpdateInvoiceRequest struct
func (r *UpdateInvoiceRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the InvoiceListQuery struct
func (q *InvoiceListQuery) Validate() error {
	return validate.Struct(q)
}

// GetLimit returns the limit with default fallback
func (q *InvoiceListQuery) GetLimit() int {
	if q.Limit <= 0 {
		return 20 // default limit
	}
	if q.Limit > 100 {
		return 100 // max limit
	}
	return q.Limit
}

// GetOffset returns the offset with default fallback
func (q *InvoiceListQuery) GetOffset() int {
	if q.Offset < 0 {
		return 0
	}
	return q.Offset
}