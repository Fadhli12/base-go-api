package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// IdempotencyService manages idempotency record lifecycle, lookup, and background
// cleanup via an embedded reaper goroutine.
type IdempotencyService struct {
	repo   repository.IdempotencyRepository
	config config.IdempotencyConfig
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewIdempotencyService creates a new IdempotencyService instance.
func NewIdempotencyService(
	repo repository.IdempotencyRepository,
	config config.IdempotencyConfig,
	logger *slog.Logger,
) *IdempotencyService {
	return &IdempotencyService{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// Config returns the idempotency configuration.
func (s *IdempotencyService) Config() config.IdempotencyConfig {
	return s.config
}

// CreateRecord persists a new idempotency record.
func (s *IdempotencyService) CreateRecord(ctx context.Context, record *domain.IdempotencyRecord) error {
	if err := s.repo.Create(ctx, record); err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindRecord locates an active idempotency record by key, user, method, and path.
func (s *IdempotencyService) FindRecord(ctx context.Context, key string, userID uuid.UUID, method, path string) (*domain.IdempotencyRecord, error) {
	record, err := s.repo.FindByKey(ctx, key, userID, method, path)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// FindRecordByID retrieves an idempotency record by its primary key.
func (s *IdempotencyService) FindRecordByID(ctx context.Context, id uuid.UUID) (*domain.IdempotencyRecord, error) {
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// UpdateRecord updates the status and cached response of an existing record.
func (s *IdempotencyService) UpdateRecord(ctx context.Context, id uuid.UUID, status string, statusCode int, body string, headers map[string]string) error {
	if err := s.repo.UpdateStatus(ctx, id, status, statusCode, body, headers); err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// ListByUser returns paginated idempotency records for a specific user.
func (s *IdempotencyService) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.IdempotencyRecord, int64, error) {
	records, total, err := s.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}
	return records, total, nil
}

// Delete soft-deletes an idempotency record by its primary key.
func (s *IdempotencyService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// CleanupExpired delegates to the repository to soft-delete expired records.
func (s *IdempotencyService) CleanupExpired(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = s.config.RetentionDays
	}
	return s.repo.CleanupExpired(context.Background(), retentionDays)
}

// StartReaper begins the background reaper goroutine that periodically cleans up expired records.
func (s *IdempotencyService) StartReaper(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)

	interval := s.config.ReaperInterval
	if interval == 0 {
		interval = config.DefaultIdempotencyConfig().ReaperInterval
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		s.logger.Info("idempotency reaper started",
			slog.Duration("interval", interval),
			slog.Int("retention_days", s.config.RetentionDays),
		)

		for {
			select {
			case <-s.ctx.Done():
				s.logger.Info("idempotency reaper stopped")
				return
			case <-ticker.C:
				s.runReaper()
			}
		}
	}()
}

// StopReaper cancels the reaper context and waits for the goroutine to finish.
func (s *IdempotencyService) StopReaper() {
	s.logger.Info("idempotency reaper shutdown initiated")
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.logger.Info("idempotency reaper shutdown complete")
}

// runReaper executes a single cleanup pass for expired records.
func (s *IdempotencyService) runReaper() {
	retentionDays := s.config.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}

	cleaned, err := s.repo.CleanupExpired(s.ctx, retentionDays)
	if err != nil {
		s.logger.Error("idempotency reaper failed",
			slog.String("error", err.Error()),
		)
		return
	}

	if cleaned > 0 {
		s.logger.Info("idempotency reaper cleaned up expired records",
			slog.Int64("count", cleaned),
			slog.Int("retention_days", retentionDays),
		)
	}
}