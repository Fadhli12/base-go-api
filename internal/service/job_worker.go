package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// JobWorker processes jobs from the Redis queue using a pool of workers.
type JobWorker struct {
	jobRepo     repository.JobRepository
	jobHandler  JobHandler
	callbackSvc *JobCallbackService
	redis       *redis.Client
	config      *config.JobConfig
	logger      *slog.Logger
	stopCh      chan struct{}
	doneCh      chan struct{}
}

// NewJobWorker creates a new JobWorker with the given dependencies.
func NewJobWorker(
	repo repository.JobRepository,
	redisClient *redis.Client,
	cfg *config.JobConfig,
	log *slog.Logger,
) *JobWorker {
	return &JobWorker{
		jobRepo:   repo,
		redis:     redisClient,
		config:    cfg,
		logger:    log,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// SetJobHandler sets the job handler service.
func (w *JobWorker) SetJobHandler(handler JobHandler) {
	w.jobHandler = handler
}

// SetCallbackService sets the callback service for delivering webhook notifications.
func (w *JobWorker) SetCallbackService(callbackSvc *JobCallbackService) {
	w.callbackSvc = callbackSvc
}

// Start begins processing jobs from the queue.
// It spawns poller goroutines to fetch jobs from Redis and worker goroutines to process them.
func (w *JobWorker) Start(ctx context.Context) {
	jobCh := make(chan *domain.Job, w.config.WorkerPoolSize*2)

	// Start pollers
	for i := 0; i < w.config.PollerCount; i++ {
		go w.runPoller(ctx, i, jobCh)
	}

	// Start workers
	for i := 0; i < w.config.WorkerPoolSize; i++ {
		go w.runWorker(ctx, i, jobCh)
	}

	w.logger.Info("job worker started",
		slog.Int("pollers", w.config.PollerCount),
		slog.Int("workers", w.config.WorkerPoolSize),
	)

	// Wait for stop signal
	<-w.stopCh
	close(jobCh)

	// Wait for all workers to finish
	w.logger.Info("job worker stopping, waiting for workers to finish")
	close(w.doneCh)
}

// Stop signals the worker to stop processing new jobs.
func (w *JobWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh
	w.logger.Info("job worker stopped")
}

func (w *JobWorker) runPoller(ctx context.Context, id int, jobCh chan<- *domain.Job) {
	w.logger.Debug("poller started", slog.Int("poller_id", id))

	pollInterval := 5 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("poller stopped: context cancelled", slog.Int("poller_id", id))
			return
		case <-w.stopCh:
			w.logger.Debug("poller stopped: stop signal", slog.Int("poller_id", id))
			return
		case <-ticker.C:
			result, err := w.redis.ZPopMin(ctx, w.config.QueueKey, 1).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				w.logger.Error("ZPOPMIN error",
					slog.Int("poller_id", id),
					slog.String("error", err.Error()),
				)
				continue
			}

			if len(result) == 0 {
				continue
			}

			jobIDStr, ok := result[0].Member.(string)
			if !ok {
				w.logger.Error("invalid job ID type in queue",
					slog.Int("poller_id", id),
					slog.Any("type", result[0].Member),
				)
				continue
			}

			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				w.logger.Error("invalid job ID in queue",
					slog.Int("poller_id", id),
					slog.String("job_id", jobIDStr),
				)
				continue
			}

			job, err := w.jobRepo.GetByID(ctx, jobID)
			if err != nil {
				w.logger.Error("failed to fetch job",
					slog.Int("poller_id", id),
					slog.String("job_id", jobIDStr),
					slog.String("error", err.Error()),
				)
				continue
			}

			select {
			case jobCh <- job:
				w.logger.Debug("job dequeued",
					slog.Int("poller_id", id),
					slog.String("job_id", jobIDStr),
				)
			case <-ctx.Done():
				w.logger.Debug("poller stopped: context cancelled after dequeue",
					slog.Int("poller_id", id),
					slog.String("job_id", jobIDStr),
				)
				return
			case <-w.stopCh:
				w.logger.Debug("poller stopped: stop signal after dequeue",
					slog.Int("poller_id", id),
					slog.String("job_id", jobIDStr),
				)
				return
			}
		}
	}
}

// runWorker processes jobs from the job channel.
func (w *JobWorker) runWorker(ctx context.Context, id int, jobCh <-chan *domain.Job) {
	w.logger.Debug("worker started", slog.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("worker stopped: context cancelled", slog.Int("worker_id", id))
			return
		case <-w.stopCh:
			w.logger.Debug("worker stopped: stop signal", slog.Int("worker_id", id))
			return
		case job, ok := <-jobCh:
			if !ok {
				w.logger.Debug("worker stopped: job channel closed", slog.Int("worker_id", id))
				return
			}
			w.processJob(ctx, id, job)
		}
	}
}

// processJob executes a single job and handles success or failure.
func (w *JobWorker) processJob(ctx context.Context, workerID int, job *domain.Job) {
	w.logger.Info("processing job",
		slog.Int("worker_id", workerID),
		slog.String("job_id", job.ID.String()),
		slog.String("type", job.Type),
		slog.Int("attempt", job.AttemptCount),
	)

	// 1. Set processing status
	updatedJob, err := w.jobRepo.SetProcessing(ctx, job.ID)
	if err != nil {
		w.logger.Error("failed to set processing",
			slog.Int("worker_id", workerID),
			slog.String("job_id", job.ID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	// 2. Parse payload
	var payload map[string]interface{}
	if len(updatedJob.Payload) > 0 {
		if err := json.Unmarshal(updatedJob.Payload, &payload); err != nil {
			w.logger.Error("failed to parse job payload",
				slog.Int("worker_id", workerID),
				slog.String("job_id", job.ID.String()),
				slog.String("error", err.Error()),
			)
			w.handleError(ctx, workerID, updatedJob, err)
			return
		}
	}

	// 3. Execute handler
	result, err := w.jobHandler.Handle(ctx, updatedJob.Type, payload)
	if err != nil {
		w.handleError(ctx, workerID, updatedJob, err)
		return
	}

	// 4. Mark completed
	if err := w.jobRepo.SetCompleted(ctx, updatedJob.ID, result); err != nil {
		w.logger.Error("failed to mark completed",
			slog.Int("worker_id", workerID),
			slog.String("job_id", updatedJob.ID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	w.logger.Info("job completed",
		slog.Int("worker_id", workerID),
		slog.String("job_id", updatedJob.ID.String()),
	)

	// 5. Deliver callback if configured (non-blocking)
	if updatedJob.CallbackURL != "" && w.callbackSvc != nil {
		go func() {
			callbackCtx := context.Background()
			if err := w.callbackSvc.Deliver(callbackCtx, updatedJob.CallbackURL, &JobCallbackPayload{
				JobID:        updatedJob.ID.String(),
				Status:       "completed",
				Result:       result,
				AttemptCount: updatedJob.AttemptCount,
				CompletedAt:  time.Now(),
			}); err != nil {
				w.logger.Error("callback delivery failed",
					slog.String("job_id", updatedJob.ID.String()),
					slog.String("url", updatedJob.CallbackURL),
					slog.String("error", err.Error()),
				)
			}
		}()
	}
}

// handleError handles job failure, scheduling retry if applicable.
func (w *JobWorker) handleError(ctx context.Context, workerID int, job *domain.Job, jobErr error) {
	w.logger.Warn("job failed",
		slog.Int("worker_id", workerID),
		slog.String("job_id", job.ID.String()),
		slog.String("error", jobErr.Error()),
		slog.Int("attempt", job.AttemptCount),
		slog.Int("max_retries", job.MaxRetries),
	)

	if job.AttemptCount < job.MaxRetries {
		nextRetry := calculateBackoff(job.AttemptCount, w.config)
		if err := w.jobRepo.SetFailed(ctx, job.ID, jobErr.Error(), &nextRetry); err != nil {
			w.logger.Error("failed to set failed with retry",
				slog.Int("worker_id", workerID),
				slog.String("job_id", job.ID.String()),
				slog.String("error", err.Error()),
			)
		} else {
			w.logger.Info("job scheduled for retry",
				slog.Int("worker_id", workerID),
				slog.String("job_id", job.ID.String()),
				slog.Time("next_retry_at", nextRetry),
			)
		}
	} else {
		if err := w.jobRepo.SetFailed(ctx, job.ID, jobErr.Error(), nil); err != nil {
			w.logger.Error("failed to set failed",
				slog.Int("worker_id", workerID),
				slog.String("job_id", job.ID.String()),
				slog.String("error", err.Error()),
			)
		}

		w.logger.Warn("job dead after exhausting retries",
			slog.Int("worker_id", workerID),
			slog.String("job_id", job.ID.String()),
			slog.Int("attempts", job.AttemptCount),
		)

		// Deliver failure callback if configured (non-blocking)
		if job.CallbackURL != "" && w.callbackSvc != nil {
			go func() {
				callbackCtx := context.Background()
				if err := w.callbackSvc.Deliver(callbackCtx, job.CallbackURL, &JobCallbackPayload{
					JobID:        job.ID.String(),
					Status:       "dead",
					Error:        jobErr.Error(),
					AttemptCount: job.AttemptCount,
					CompletedAt:  time.Now(),
				}); err != nil {
					w.logger.Error("failure callback delivery failed",
						slog.String("job_id", job.ID.String()),
						slog.String("url", job.CallbackURL),
						slog.String("error", err.Error()),
					)
				}
			}()
		}
	}
}

// calculateBackoff determines the next retry time based on attempt count.
func calculateBackoff(attemptCount int, cfg *config.JobConfig) time.Time {
	switch attemptCount {
	case 1:
		return time.Now().Add(1 * time.Minute)
	case 2:
		return time.Now().Add(5 * time.Minute)
	default:
		backoff := time.Now().Add(time.Duration(attemptCount-1) * time.Duration(cfg.JobTimeoutSeconds) * time.Second)
		maxBackoff := time.Now().Add(30 * time.Minute)
		if backoff.After(maxBackoff) {
			return maxBackoff
		}
		return backoff
	}
}