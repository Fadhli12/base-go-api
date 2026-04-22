// Package invoice provides invoice domain logic for the application.
package invoice

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InvoiceStatus represents the status of an invoice
type InvoiceStatus string

const (
	// InvoiceStatusDraft indicates the invoice is in draft state
	InvoiceStatusDraft InvoiceStatus = "draft"
	// InvoiceStatusPending indicates the invoice is pending payment
	InvoiceStatusPending InvoiceStatus = "pending"
	// InvoiceStatusPaid indicates the invoice has been paid
	InvoiceStatusPaid InvoiceStatus = "paid"
	// InvoiceStatusCancelled indicates the invoice has been cancelled
	InvoiceStatusCancelled InvoiceStatus = "cancelled"
)

// Invoice represents an invoice entity in the system
type Invoice struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"` // Owner
	Number      string         `gorm:"size:50;not null;uniqueIndex" json:"number"`
	Customer    string         `gorm:"size:255;not null" json:"customer"`
	Amount      float64        `gorm:"not null" json:"amount"`
	Status      InvoiceStatus  `gorm:"size:20;not null;default:'draft'" json:"status"`
	Description string         `gorm:"size:1000" json:"description"`
	DueDate     *time.Time     `json:"due_date,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the Invoice model
func (Invoice) TableName() string {
	return "invoices"
}

// InvoiceResponse represents an invoice in API responses
type InvoiceResponse struct {
	ID          string        `json:"id"`
	UserID      string        `json:"user_id"`
	Number      string        `json:"number"`
	Customer    string        `json:"customer"`
	Amount      float64       `json:"amount"`
	Status      InvoiceStatus `json:"status"`
	Description string        `json:"description"`
	DueDate     *string       `json:"due_date,omitempty"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
}

// ToResponse converts an Invoice to InvoiceResponse
func (i *Invoice) ToResponse() InvoiceResponse {
	var dueDateStr *string
	if i.DueDate != nil {
		formatted := i.DueDate.Format(time.RFC3339)
		dueDateStr = &formatted
	}
	return InvoiceResponse{
		ID:          i.ID.String(),
		UserID:      i.UserID.String(),
		Number:      i.Number,
		Customer:    i.Customer,
		Amount:      i.Amount,
		Status:      i.Status,
		Description: i.Description,
		DueDate:     dueDateStr,
		CreatedAt:   i.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   i.UpdatedAt.Format(time.RFC3339),
	}
}

// IsValidStatus checks if the status is valid
func IsValidStatus(status InvoiceStatus) bool {
	switch status {
	case InvoiceStatusDraft, InvoiceStatusPending, InvoiceStatusPaid, InvoiceStatusCancelled:
		return true
	default:
		return false
	}
}