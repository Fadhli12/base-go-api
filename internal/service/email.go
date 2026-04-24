package service

import (
	"context"
	"fmt"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// EmailService handles email sending business logic
type EmailService struct {
	config       *config.EmailConfig
	templateRepo repository.EmailTemplateRepository
	queueRepo    repository.EmailQueueRepository
	bounceRepo   repository.EmailBounceRepository
	templateEng  *TemplateEngine
	provider     EmailProvider
}

// NewEmailService creates a new EmailService instance
func NewEmailService(
	cfg *config.EmailConfig,
	templateRepo repository.EmailTemplateRepository,
	queueRepo repository.EmailQueueRepository,
	bounceRepo repository.EmailBounceRepository,
	templateEng *TemplateEngine,
	provider EmailProvider,
) *EmailService {
	return &EmailService{
		config:       cfg,
		templateRepo: templateRepo,
		queueRepo:    queueRepo,
		bounceRepo:   bounceRepo,
		templateEng:  templateEng,
		provider:     provider,
	}
}

// SetProvider sets the email provider (DI for provider switching)
func (s *EmailService) SetProvider(provider EmailProvider) {
	s.provider = provider
}

// EmailRequest represents a request to send an email
type EmailRequest struct {
	To          string                 // Recipient email address
	Subject     string                 // Email subject (required for direct send)
	Template    string                 // Template name (optional)
	Data        map[string]any         // Template variables
	HTMLContent string                 // Direct HTML content (optional)
	TextContent string                 // Direct text content (optional)
}

// QueueEmail queues an email for async delivery
func (s *EmailService) QueueEmail(ctx context.Context, req *EmailRequest) error {
	// Validate request
	if err := s.validateRequest(req); err != nil {
		return err
	}

	// If template is provided, validate it exists and is active
	if req.Template != "" {
		tmpl, err := s.templateRepo.FindByName(ctx, req.Template)
		if err != nil {
			return errors.NewAppError("TEMPLATE_NOT_FOUND", fmt.Sprintf("Template %s not found", req.Template), 404)
		}
		if !tmpl.IsActive {
			return errors.NewAppError("TEMPLATE_INACTIVE", fmt.Sprintf("Template %s is not active", req.Template), 400)
		}
	}

	// Create queue entry
	email := &domain.EmailQueue{
		ToAddress:    req.To,
		Subject:      req.Subject,
		Template:     req.Template,
		Status:       domain.EmailStatusQueued,
		MaxAttempts:  s.config.RetryMax,
	}

	// Note: Data field handling will be implemented when JSON marshaling is added

	// Queue the email
	if err := s.queueRepo.Create(ctx, email); err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// SendTransactional sends an email immediately without queuing
// Use for: password reset, welcome emails (bypass queue)
func (s *EmailService) SendTransactional(ctx context.Context, to, template string, data map[string]any) (*domain.EmailQueue, error) {
	// Render template
	htmlContent, textContent, err := s.templateEng.RenderTemplate(ctx, template, data)
	if err != nil {
		return nil, errors.NewAppError("TEMPLATE_ERROR", err.Error(), 500)
	}

	// Get template for subject
	tmpl, err := s.templateRepo.FindByName(ctx, template)
	if err != nil {
		return nil, errors.NewAppError("TEMPLATE_NOT_FOUND", fmt.Sprintf("Template %s not found", template), 404)
	}

	// Create queue entry for tracking
	email := &domain.EmailQueue{
		ToAddress:    to,
		Subject:      tmpl.Subject,
		Template:     template,
		Status:       domain.EmailStatusProcessing,
		MaxAttempts:  s.config.RetryMax,
	}

	if err := s.queueRepo.Create(ctx, email); err != nil {
		return nil, errors.WrapInternal(err)
	}

	// Send via provider
	message := &EmailMessage{
		To:          to,
		Subject:     tmpl.Subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
	}

	messageID, err := s.provider.Send(ctx, message)
	if err != nil {
		// Mark as failed
		_ = s.queueRepo.MarkFailed(ctx, email.ID, err)
		return nil, errors.NewAppError("SEND_FAILED", err.Error(), 500)
	}

	// Mark as sent
	if err := s.queueRepo.MarkSent(ctx, email.ID, s.provider.Name(), messageID); err != nil {
		return nil, errors.WrapInternal(err)
	}

	// Fetch updated record
	return s.queueRepo.FindByID(ctx, email.ID)
}

// RecordBounce records a bounce event from provider webhook
func (s *EmailService) RecordBounce(ctx context.Context, info *EmailBounceInfo) error {
	// Find the email by message ID
	email, err := s.queueRepo.FindByID(ctx, uuid.MustParse(info.MessageID))
	if err != nil {
		// Email not found - could be from another system, log and continue
		_ = err // Ignore - still record the bounce
	} else {
		// Mark email as bounced
		if err := s.queueRepo.MarkBounced(ctx, email.ID, info.BounceReason); err != nil {
			return errors.WrapInternal(err)
		}
	}

	// Create bounce record
	bounce := &domain.EmailBounce{
		Email:        info.Email,
		BounceType:   info.BounceType,
		BounceReason: info.BounceReason,
		MessageID:    info.MessageID,
	}

	if err := s.bounceRepo.Create(ctx, bounce); err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// RecordDelivery records a delivery confirmation from provider webhook
func (s *EmailService) RecordDelivery(ctx context.Context, messageID string) error {
	// Find the email by message ID
	email, err := s.queueRepo.FindByID(ctx, uuid.MustParse(messageID))
	if err != nil {
		return errors.ErrNotFound
	}

	// Mark as delivered
	if err := s.queueRepo.MarkDelivered(ctx, email.ID); err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// GetQueueStatus retrieves the status of a queued email
func (s *EmailService) GetQueueStatus(ctx context.Context, id uuid.UUID) (*domain.EmailQueue, error) {
	return s.queueRepo.FindByID(ctx, id)
}

// GetBounceHistory retrieves bounce history for an email address
func (s *EmailService) GetBounceHistory(ctx context.Context, email string, limit, offset int) ([]*domain.EmailBounce, int64, error) {
	return s.bounceRepo.FindByEmail(ctx, email, limit, offset)
}

// validateRequest validates an email request
func (s *EmailService) validateRequest(req *EmailRequest) error {
	if req.To == "" {
		return errors.NewAppError("VALIDATION_ERROR", "recipient email address is required", 400)
	}

	// Either template or direct content must be provided
	if req.Template == "" && req.HTMLContent == "" && req.TextContent == "" {
		return errors.NewAppError("VALIDATION_ERROR", "template name or content must be provided", 400)
	}

	return nil
}