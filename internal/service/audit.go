package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/google/uuid"
)

// AuditService handles audit logging operations with asynchronous processing.
// It uses a buffered channel to decouple audit log creation from request handling,
// ensuring that audit logging does not block the main request flow.
type AuditService struct {
	repo    repository.AuditLogRepository
	logChan chan *domain.AuditLog
	done    chan struct{}
}

// AuditServiceConfig holds configuration for the audit service
type AuditServiceConfig struct {
	// BufferSize is the size of the async write buffer channel
	BufferSize int
}

// DefaultAuditServiceConfig returns the default configuration
func DefaultAuditServiceConfig() AuditServiceConfig {
	return AuditServiceConfig{
		BufferSize: 100,
	}
}

// NewAuditService creates a new AuditService instance.
// It starts a background goroutine that processes audit logs from the channel.
// The buffer size determines how many audit logs can be queued before blocking.
func NewAuditService(repo repository.AuditLogRepository, config AuditServiceConfig) *AuditService {
	if config.BufferSize <= 0 {
		config.BufferSize = DefaultAuditServiceConfig().BufferSize
	}

	as := &AuditService{
		repo:    repo,
		logChan: make(chan *domain.AuditLog, config.BufferSize),
		done:    make(chan struct{}),
	}

	// Start the background worker
	go as.processLogs()

	return as
}

// LogAction creates and queues an audit log entry asynchronously.
// This method does not block - it queues the audit log for background processing.
// Use this for all audit logging to ensure request performance is not impacted.
func (s *AuditService) LogAction(
	ctx context.Context,
	actorID uuid.UUID,
	action string,
	resource string,
	resourceID string,
	before interface{},
	after interface{},
	ipAddress string,
	userAgent string,
) error {
	auditLog := &domain.AuditLog{
		ActorID:    actorID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  ipAddress,
		UserAgent:  sanitizeUserAgent(userAgent),
	}

	// Marshal before state
	if before != nil {
		beforeBytes, err := json.Marshal(before)
		if err != nil {
			slog.Warn("failed to marshal before state for audit log",
				slog.String("error", err.Error()),
				slog.String("action", action),
				slog.String("resource", resource),
			)
		} else {
			auditLog.Before = beforeBytes
		}
	}

	// Marshal after state
	if after != nil {
		afterBytes, err := json.Marshal(after)
		if err != nil {
			slog.Warn("failed to marshal after state for audit log",
				slog.String("error", err.Error()),
				slog.String("action", action),
				slog.String("resource", resource),
			)
		} else {
			auditLog.After = afterBytes
		}
	}

	// Queue for async processing (non-blocking unless channel is full)
	select {
	case s.logChan <- auditLog:
		return nil
	default:
		// Channel is full, log warning but don't block
		slog.Warn("audit log channel full, dropping audit log",
			slog.String("action", action),
			slog.String("resource", resource),
		)
		return nil
	}
}

// LogActionSync creates and persists an audit log entry synchronously.
// Use this for critical audit logs that must be persisted before continuing,
// or for shutdown/cleanup scenarios where async processing won't complete.
func (s *AuditService) LogActionSync(
	ctx context.Context,
	actorID uuid.UUID,
	action string,
	resource string,
	resourceID string,
	before interface{},
	after interface{},
	ipAddress string,
	userAgent string,
) error {
	auditLog := &domain.AuditLog{
		ActorID:    actorID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  ipAddress,
		UserAgent:  sanitizeUserAgent(userAgent),
	}

	// Marshal before state
	if before != nil {
		beforeBytes, err := json.Marshal(before)
		if err != nil {
			slog.Warn("failed to marshal before state for audit log",
				slog.String("error", err.Error()),
				slog.String("action", action),
				slog.String("resource", resource),
			)
		} else {
			auditLog.Before = beforeBytes
		}
	}

	// Marshal after state
	if after != nil {
		afterBytes, err := json.Marshal(after)
		if err != nil {
			slog.Warn("failed to marshal after state for audit log",
				slog.String("error", err.Error()),
				slog.String("action", action),
				slog.String("resource", resource),
			)
		} else {
			auditLog.After = afterBytes
		}
	}

	return s.repo.Create(ctx, auditLog)
}

// processLogs is the background worker that processes audit logs from the channel.
// It runs in a goroutine and persists logs to the database.
func (s *AuditService) processLogs() {
	for {
		select {
		case auditLog := <-s.logChan:
			// Use background context since we're in a background goroutine
			ctx := context.Background()
			if err := s.repo.Create(ctx, auditLog); err != nil {
				slog.Error("failed to persist audit log",
					slog.String("error", err.Error()),
					slog.String("action", auditLog.Action),
					slog.String("resource", auditLog.Resource),
					slog.String("actor_id", auditLog.ActorID.String()),
				)
			}
		case <-s.done:
			// Drain remaining logs before shutting down
			for len(s.logChan) > 0 {
				auditLog := <-s.logChan
				ctx := context.Background()
				if err := s.repo.Create(ctx, auditLog); err != nil {
					slog.Error("failed to persist audit log during shutdown",
						slog.String("error", err.Error()),
						slog.String("action", auditLog.Action),
						slog.String("resource", auditLog.Resource),
					)
				}
			}
			return
		}
	}
}

// Shutdown gracefully stops the audit service, draining any remaining logs.
// Call this during application shutdown to ensure all logs are persisted.
func (s *AuditService) Shutdown() {
	close(s.done)
}

// GetActorAuditLogs retrieves audit logs for a specific actor with pagination
func (s *AuditService) GetActorAuditLogs(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	return s.repo.FindByActorID(ctx, actorID, limit, offset)
}

// GetResourceAuditLogs retrieves audit logs for a specific resource with pagination
func (s *AuditService) GetResourceAuditLogs(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	return s.repo.FindByResource(ctx, resource, resourceID, limit, offset)
}

// GetAllAuditLogs retrieves all audit logs with pagination (for admin purposes)
func (s *AuditService) GetAllAuditLogs(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	return s.repo.FindAll(ctx, limit, offset)
}

// sanitizeUserAgent truncates user agent strings to prevent excessively long entries
func sanitizeUserAgent(userAgent string) string {
	const maxUserAgentLength = 500
	if len(userAgent) > maxUserAgentLength {
		return userAgent[:maxUserAgentLength]
	}
	return userAgent
}