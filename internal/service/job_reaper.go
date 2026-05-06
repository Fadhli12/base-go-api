package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
)

// JobReaper recovers jobs stuck in processing state.
type JobReaper struct {
	jobRepo repository.JobRepository
	config  *config.JobConfig
	logger  *slog.Logger
}

// NewJobReaper creates a new JobReaper.
func NewJobReaper(repo repository.JobRepository, cfg *config.JobConfig, log *slog.Logger) *JobReaper {
	return &JobReaper{
		jobRepo: repo,
		config:  cfg,
		logger:  log,
	}
}

// Reap finds stuck jobs and requeues them.
// Returns the number of jobs recovered.
func (r *JobReaper) Reap(ctx context.Context) (int, error) {
	threshold := time.Duration(r.config.StuckJobThresholdSeconds) * time.Second
	stuckJobs, err := r.jobRepo.GetStuckJobs(ctx, threshold)
	if err != nil {
		r.logger.Error("failed to get stuck jobs", slog.String("error", err.Error()))
		return 0, err
	}

	recovered := 0
	for _, job := range stuckJobs {
		// Reset to pending and re-enqueue
		job.Status = domain.JobStatusPending
		job.AttemptCount++ // Increment to avoid infinite loop

		nextRetry := time.Now().Add(time.Duration(r.config.JobTimeoutSeconds) * time.Second)
		job.NextRetryAt = &nextRetry

		if err := r.jobRepo.Update(ctx, job); err != nil {
			r.logger.Error("failed to reset stuck job",
				slog.String("job_id", job.ID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		if err := r.jobRepo.Enqueue(ctx, job.ID, *job.NextRetryAt); err != nil {
			r.logger.Error("failed to enqueue stuck job",
				slog.String("job_id", job.ID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		recovered++
		r.logger.Info("recovered stuck job",
			slog.String("job_id", job.ID.String()),
			slog.Int("attempt_count", job.AttemptCount),
		)
	}

	return recovered, nil
}

// StartReaper starts a background goroutine that runs the reaper periodically.
// Returns a stop function.
func (r *JobReaper) StartReaper(ctx context.Context) func() {
	interval := time.Duration(r.config.ReaperIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	stopCh := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-stopCh:
				ticker.Stop()
				return
			case <-ticker.C:
				recovered, err := r.Reap(ctx)
				if err != nil {
					r.logger.Error("reaper run failed", slog.String("error", err.Error()))
				} else if recovered > 0 {
					r.logger.Info("reaper completed",
						slog.Int("recovered", recovered),
					)
				}
			}
		}
	}()

	return func() {
		close(stopCh)
	}
}