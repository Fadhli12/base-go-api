package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/repository"
)

// AnalyticsReaper is a background goroutine that archives old metric events
// based on the configured retention period.
// Follows the exact same pattern as ActivityReaper.
type AnalyticsReaper struct {
	repo   repository.MetricEventRepository
	config config.AnalyticsConfig
	log    *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAnalyticsReaper creates a new AnalyticsReaper instance.
func NewAnalyticsReaper(
	repo repository.MetricEventRepository,
	config config.AnalyticsConfig,
	log *slog.Logger,
) *AnalyticsReaper {
	return &AnalyticsReaper{
		repo:   repo,
		config: config,
		log:    log,
	}
}

// Start begins the reaper goroutine, which runs on a ticker interval.
func (r *AnalyticsReaper) Start(ctx context.Context) {
	r.ctx, r.cancel = context.WithCancel(ctx)

	interval := r.config.ReaperInterval
	if interval == 0 {
		interval = 60 * time.Second
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		r.log.Info("analytics reaper started",
			slog.Duration("interval", interval),
			slog.Int("retention_days", r.config.RetentionDays),
		)

		for {
			select {
			case <-r.ctx.Done():
				r.log.Info("analytics reaper stopped")
				return
			case <-ticker.C:
				r.run()
			}
		}
	}()
}

// Stop cancels the reaper context and waits for the goroutine to finish.
func (r *AnalyticsReaper) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	r.log.Info("analytics reaper shutdown complete")
}

// run executes a single archive pass.
func (r *AnalyticsReaper) run() {
	retentionDays := r.config.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 90
	}

	archived, err := r.repo.ArchiveOlderThan(r.ctx, retentionDays)
	if err != nil {
		r.log.Error("analytics reaper failed",
			slog.String("error", err.Error()),
		)
		return
	}

	if archived > 0 {
		r.log.Info("analytics reaper archived metric events",
			slog.Int64("count", archived),
			slog.Int("retention_days", retentionDays),
		)
	}
}