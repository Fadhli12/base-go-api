package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/google/uuid"
)

// EmailWorker processes emails from the queue
type EmailWorker struct {
	config      *config.EmailConfig
	queueRepo   repository.EmailQueueRepository
	bounceRepo  repository.EmailBounceRepository
	redisQueue  *RedisEmailQueue
	provider    EmailProvider
	templateEng *TemplateEngine

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	processedCount int64
	errorCount     int64
}

// NewEmailWorker creates a new email worker
func NewEmailWorker(
	cfg *config.EmailConfig,
	queueRepo repository.EmailQueueRepository,
	bounceRepo repository.EmailBounceRepository,
	redisQueue *RedisEmailQueue,
	provider EmailProvider,
	templateEng *TemplateEngine,
) *EmailWorker {
	return &EmailWorker{
		config:      cfg,
		queueRepo:   queueRepo,
		bounceRepo:  bounceRepo,
		redisQueue:  redisQueue,
		provider:    provider,
		templateEng: templateEng,
	}
}

// Start begins processing emails from the queue
func (w *EmailWorker) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)

	// Start worker goroutines
	for i := 0; i < w.config.WorkerConcurrency; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}

	slog.Info("Email worker started",
		"workers", w.config.WorkerConcurrency,
		"retry_max", w.config.RetryMax,
	)

	return nil
}

// Stop gracefully shuts down the worker
func (w *EmailWorker) Stop() error {
	slog.Info("Stopping email worker...")
	w.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Email worker stopped gracefully",
			"processed", w.processedCount,
			"errors", w.errorCount,
		)
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("email worker shutdown timeout")
	}
}

// worker processes emails from the queue
func (w *EmailWorker) worker(id int) {
	defer w.wg.Done()

	slog.Info("Email worker started", "worker_id", id)

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("Email worker stopping", "worker_id", id)
			return
		default:
			// Process next email
			if err := w.processNextEmail(id); err != nil {
				slog.Error("Failed to process email",
					"worker_id", id,
					"error", err,
				)
				// Brief pause on error to avoid tight loop
				time.Sleep(1 * time.Second)
			}
		}
	}
}

// processNextEmail retrieves and processes the next email from the queue
func (w *EmailWorker) processNextEmail(workerID int) error {
	ctx := w.ctx

	// Get next batch of queued emails
	emails, err := w.queueRepo.GetNextBatch(ctx, 1)
	if err != nil {
		return fmt.Errorf("failed to get next batch: %w", err)
	}

	if len(emails) == 0 {
		// No emails to process, brief pause
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	email := emails[0]

	// Mark as processing
	if err := w.queueRepo.MarkProcessing(ctx, email.ID); err != nil {
		return fmt.Errorf("failed to mark as processing: %w", err)
	}

	slog.Info("Processing email",
		"worker_id", workerID,
		"email_id", email.ID,
		"to", email.ToAddress,
	)

	// Process the email
	if err := w.sendEmail(ctx, email); err != nil {
		w.errorCount++
		return w.handleSendError(ctx, email, err)
	}

	w.processedCount++
	return nil
}

// sendEmail sends an email via the provider
func (w *EmailWorker) sendEmail(ctx context.Context, email *domain.EmailQueue) error {
	var htmlContent, textContent string
	var subject string

	// Render template if provided
	if email.Template != "" {
		// Parse template data (if any)
		var data map[string]any
		if email.Data != nil {
			// Note: Data field needs JSON unmarshaling implementation
			// For now, assume it's already structured
		}

		// Get template
		template, err := w.templateEng.templateRepo.FindByName(ctx, email.Template)
		if err != nil {
			return fmt.Errorf("template not found: %w", err)
		}

		subject = template.Subject
		htmlContent = template.HTMLContent
		textContent = template.TextContent

		// Render with data
		if data != nil {
			var err error
			htmlContent, err = w.templateEng.renderString(htmlContent, data)
			if err != nil {
				return fmt.Errorf("failed to render HTML: %w", err)
			}

			if textContent != "" {
				textContent, err = w.templateEng.renderString(textContent, data)
				if err != nil {
					return fmt.Errorf("failed to render text: %w", err)
				}
			}
		}
	} else {
		// Use direct content
		subject = email.Subject
		// HTML and text content would come from email.Data
	}

	// Send via provider
	message := &EmailMessage{
		To:          email.ToAddress,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
	}

	messageID, err := w.provider.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Mark as sent
	if err := w.queueRepo.MarkSent(ctx, email.ID, w.provider.Name(), messageID); err != nil {
		slog.Error("Failed to mark email as sent",
			"email_id", email.ID,
			"error", err,
		)
	}

	slog.Info("Email sent successfully",
		"email_id", email.ID,
		"message_id", messageID,
		"provider", w.provider.Name(),
	)

	return nil
}

// handleSendError handles send errors with retry logic
func (w *EmailWorker) handleSendError(ctx context.Context, email *domain.EmailQueue, err error) error {
	// Increment attempts
	if incErr := w.queueRepo.IncrementAttempts(ctx, email.ID); incErr != nil {
		slog.Error("Failed to increment attempts",
			"email_id", email.ID,
			"error", incErr,
		)
	}

	// Check if can retry
	canRetry, checkErr := w.queueRepo.CanRetry(ctx, email.ID)
	if checkErr != nil {
		slog.Error("Failed to check retry status",
			"email_id", email.ID,
			"error", checkErr,
		)
		canRetry = false
	}

	if canRetry {
		// Reset to queued for retry
		slog.Warn("Email send failed, will retry",
			"email_id", email.ID,
			"attempt", email.Attempts+1,
			"error", err,
		)
		// Note: Need a method to reset status to queued
	} else {
		// Max retries reached, mark as failed
		slog.Error("Email send failed, max retries reached",
			"email_id", email.ID,
			"attempts", email.Attempts+1,
			"error", err,
		)

		if failErr := w.queueRepo.MarkFailed(ctx, email.ID, err); failErr != nil {
			slog.Error("Failed to mark email as failed",
				"email_id", email.ID,
				"error", failErr,
			)
		}
	}

	return err
}

// GetMetrics returns current worker metrics
func (w *EmailWorker) GetMetrics() map[string]int64 {
	return map[string]int64{
		"processed": w.processedCount,
		"errors":    w.errorCount,
	}
}

// RecoverDeadLetters moves dead letter queue emails back to main queue for retry
func (w *EmailWorker) RecoverDeadLetters(ctx context.Context) error {
	// Get length of dead letter queue
	length, err := w.redisQueue.GetDeadLetterLength(ctx)
	if err != nil {
		return fmt.Errorf("failed to get dead letter length: %w", err)
	}

	slog.Info("Recovering dead letter emails", "count", length)

	// Move items back to main queue
	// Note: This would iterate through dead letter queue and requeue
	// Implementation depends on dead letter handling strategy

	return nil
}

// ProcessDeadLetter handles emails that have permanently failed
func (w *EmailWorker) ProcessDeadLetter(ctx context.Context, emailID uuid.UUID, reason string) error {
	// Increment attempts and mark as failed
	email, err := w.queueRepo.FindByID(ctx, emailID)
	if err != nil {
		return fmt.Errorf("email not found: %w", err)
	}

	// Create bounce record
	bounce := &domain.EmailBounce{
		Email:        email.ToAddress,
		BounceType:   domain.BounceTypeTechnical,
		BounceReason: reason,
		MessageID:    email.MessageID,
	}

	if err := w.bounceRepo.Create(ctx, bounce); err != nil {
		slog.Error("Failed to create bounce record",
			"email_id", emailID,
			"error", err,
		)
	}

	// Move to dead letter queue
	if err := w.redisQueue.MoveToDeadLetter(ctx, emailID.String(), reason); err != nil {
		return fmt.Errorf("failed to move to dead letter: %w", err)
	}

	return nil
}