package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/repository"
)

// ActivityReaper is a background goroutine that archives old activities
// based on the configured retention period.
type ActivityReaper struct {
	repo   repository.ActivityRepository
	config config.ActivityConfig
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// NewActivityReaper creates a new ActivityReaper instance.
func NewActivityReaper(
	repo repository.ActivityRepository,
	config config.ActivityConfig,
	logger *slog.Logger,
) *ActivityReaper {
	return &ActivityReaper{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// Start begins the reaper goroutine, which runs on a ticker interval.
func (r *ActivityReaper) Start(ctx context.Context) {
	r.ctx, r.cancel = context.WithCancel(ctx)

	interval := r.config.ReaperInterval
	if interval == 0 {
		interval = 60 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		r.logger.Info("activity reaper started",
			slog.Duration("interval", interval),
			slog.Int("retention_days", r.config.RetentionDays),
		)

		for {
			select {
			case <-r.ctx.Done():
				r.logger.Info("activity reaper stopped")
				return
			case <-ticker.C:
				r.run()
			}
		}
	}()
}

// Stop cancels the reaper context, shutting down the goroutine.
func (r *ActivityReaper) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.logger.Info("activity reaper shutdown initiated")
}

// run executes a single archive pass.
func (r *ActivityReaper) run() {
	retentionDays := r.config.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 90
	}

	archived, err := r.repo.ArchiveOlderThan(r.ctx, retentionDays)
	if err != nil {
		r.logger.Error("activity reaper failed",
			slog.String("error", err.Error()),
		)
		return
	}

	if archived > 0 {
		r.logger.Info("activity reaper archived activities",
			slog.Int64("count", archived),
			slog.Int("retention_days", retentionDays),
		)
	}
}