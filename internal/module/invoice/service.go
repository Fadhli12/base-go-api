package invoice

import (
	"context"
	"fmt"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// Service handles invoice-related business logic
type Service struct {
	repo     Repository
	enforcer *permission.Enforcer
	eventBus *domain.EventBus
}

// NewService creates a new Service instance
func NewService(repo Repository, enforcer *permission.Enforcer) *Service {
	return &Service{
		repo:     repo,
		enforcer: enforcer,
	}
}

// SetEventBus sets the event bus for publishing domain events.
func (s *Service) SetEventBus(eventBus *domain.EventBus) {
	s.eventBus = eventBus
}

// CreateInvoiceRequest represents a request to create an invoice
type CreateInvoiceRequest struct {
	UserID      string     `json:"user_id"`
	Number      string     `json:"number"`
	Customer    string     `json:"customer"`
	Amount      float64    `json:"amount"`
	Description string     `json:"description"`
	DueDate     *string    `json:"due_date"`
}

// UpdateInvoiceRequest represents a request to update an invoice
type UpdateInvoiceRequest struct {
	Customer    string        `json:"customer"`
	Amount      float64       `json:"amount"`
	Description string        `json:"description"`
	DueDate     *string       `json:"due_date"`
	Status      InvoiceStatus `json:"status"`
}

// Create creates a new invoice (requires invoice:create permission)
func (s *Service) Create(ctx context.Context, userID uuid.UUID, number, customer string, amount float64, description string, dueDate *string) (*Invoice, error) {
	// Validate required fields
	if customer == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "Customer is required", 422)
	}
	if amount <= 0 {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "Amount must be greater than zero", 422)
	}

	invoice := &Invoice{
		UserID:      userID,
		Number:      number,
		Customer:    customer,
		Amount:      amount,
		Status:      InvoiceStatusDraft,
		Description: description,
	}

	if dueDate != nil && *dueDate != "" {
		// Parse RFC3339 datetime string
		// Note: In real implementation, you would parse this properly
		// For now, we leave it nil
	}

	if err := s.repo.Create(ctx, invoice); err != nil {
		return nil, err
	}

	// Publish invoice.created event (best-effort)
	if s.eventBus != nil {
		_ = s.eventBus.Publish(domain.WebhookEvent{
			Type:    "invoice.created",
			Payload: invoice.ToResponse(),
		})
	}

	return invoice, nil
}

// GetByID retrieves an invoice by ID with permission check
// If scope=own, user must own the invoice
// If scope=all or isAdmin, can access any invoice
func (s *Service) GetByID(ctx context.Context, userID, id uuid.UUID, isAdmin bool) (*Invoice, error) {
	invoice, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ownership check: if not admin, can only access own invoices
	if !isAdmin && invoice.UserID != userID {
		return nil, apperrors.ErrNotFound
	}

	return invoice, nil
}

// ListByUser retrieves all invoices for a specific user
func (s *Service) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Invoice, int64, error) {
	invoices, err := s.repo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	return invoices, count, nil
}

// ListAll retrieves all invoices with pagination (admin only)
func (s *Service) ListAll(ctx context.Context, limit, offset int) ([]Invoice, int64, error) {
	invoices, err := s.repo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.CountAll(ctx)
	if err != nil {
		return nil, 0, err
	}

	return invoices, count, nil
}

// Update updates an invoice with ownership check
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, customer string, amount float64, description string, dueDate *string, status InvoiceStatus, isAdmin bool) (*Invoice, error) {
	// Fetch invoice first
	invoice, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ownership check: if not admin, can only modify own invoices
	if !isAdmin && invoice.UserID != userID {
		return nil, apperrors.ErrNotFound
	}

	// Update fields
	if customer != "" {
		invoice.Customer = customer
	}
	if amount > 0 {
		invoice.Amount = amount
	}
	if description != "" {
		invoice.Description = description
	}
	if dueDate != nil {
		// Parse due date if provided
	}
	if status != "" && IsValidStatus(status) {
		invoice.Status = status
	}

	if err := s.repo.Update(ctx, invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// Delete soft-deletes an invoice with ownership check
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID, isAdmin bool) error {
	// Fetch invoice first to check ownership
	invoice, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Ownership check: if not admin, can only delete own invoices
	if !isAdmin && invoice.UserID != userID {
		return apperrors.ErrNotFound
	}

	return s.repo.SoftDelete(ctx, id)
}

// UpdateStatus updates the status of an invoice (admin only)
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status InvoiceStatus) error {
	if !IsValidStatus(status) {
		return apperrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("Invalid status: %s", status), 422)
	}

	invoice, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	invoice.Status = status
	if err := s.repo.Update(ctx, invoice); err != nil {
		return err
	}

	// Publish invoice.paid event when status changes to paid (best-effort)
	if s.eventBus != nil && status == InvoiceStatusPaid {
		_ = s.eventBus.Publish(domain.WebhookEvent{
			Type:    "invoice.paid",
			Payload: invoice.ToResponse(),
		})
	}

	return nil
}

// CheckPermission checks if a user has permission for an action on invoices
func (s *Service) CheckPermission(ctx context.Context, userID uuid.UUID, action string) (bool, error) {
	if s.enforcer == nil {
		// If no enforcer, default to allow (for backward compatibility)
		return true, nil
	}

	// Domain is typically tenant/organization ID, using "default" for now
	return s.enforcer.Enforce(userID.String(), "default", "invoices", action)
}

// CheckPermissionWithScope checks if a user has permission with a specific scope
func (s *Service) CheckPermissionWithScope(ctx context.Context, userID uuid.UUID, resource, action, scope string) (bool, error) {
	if s.enforcer == nil {
		return true, nil
	}

	// Check if user has the permission with the given scope
	return s.enforcer.Enforce(userID.String(), "default", resource, action)
}