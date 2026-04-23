// Package conversion provides image conversion and thumbnail generation
package conversion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/storage"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// Worker handles asynchronous media conversion jobs
type Worker struct {
	redisClient     *redis.Client
	repo            repository.MediaRepository
	storageDriver   storage.Driver
	registry        *Registry
	stopCh          chan struct{}
	jobCh           chan ConversionJob
	enqueueCh       chan ConversionJob
	maxRetries      int
	pollInterval    time.Duration
	processingJobs  map[string]bool
}

// WorkerConfig holds configuration for the conversion worker
type WorkerConfig struct {
	RedisClient    *redis.Client
	Repo           repository.MediaRepository
	StorageDriver  storage.Driver
	MaxRetries     int
	PollInterval   time.Duration
	WorkerCount    int
}

// NewWorker creates a new conversion worker
func NewWorker(config WorkerConfig) *Worker {
	if config.MaxRetries == 0 {
		config.MaxRetries = MaxRetries
	}
	if config.PollInterval == 0 {
		config.PollInterval = 1 * time.Second
	}

	registry := NewRegistry()
	// Register image handler
	registry.Register(NewImageConversionHandler(config.StorageDriver))

	return &Worker{
		redisClient:    config.RedisClient,
		repo:           config.Repo,
		storageDriver:  config.StorageDriver,
		registry:       registry,
		stopCh:         make(chan struct{}),
		jobCh:          make(chan ConversionJob, 100),
		enqueueCh:      make(chan ConversionJob, 100),
		maxRetries:     config.MaxRetries,
		pollInterval:   config.PollInterval,
		processingJobs: make(map[string]bool),
	}
}

// Start starts the worker goroutines
func (w *Worker) Start(ctx context.Context, workerCount int) {
	if workerCount <= 0 {
		workerCount = 3
	}

	slog.Info("Starting conversion worker", "worker_count", workerCount, "poll_interval", w.pollInterval)

	// Start job processors
	for i := 0; i < workerCount; i++ {
		go w.processLoop(ctx, i)
	}

	// Start queue poller
	go w.pollLoop(ctx)

	// Start enqueue processor
	go w.enqueueLoop(ctx)
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	close(w.stopCh)
}

// processLoop processes jobs from the job channel
func (w *Worker) processLoop(ctx context.Context, workerID int) {
	slog.Info("Conversion worker started", "worker_id", workerID)
	defer slog.Info("Conversion worker stopped", "worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case job := <-w.jobCh:
			w.handleJob(ctx, job)
		}
	}
}

// pollLoop periodically polls the Redis queue for jobs
func (w *Worker) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.pollQueue(ctx)
		}
	}
}

// enqueueLoop processes jobs that need to be enqueued to Redis
func (w *Worker) enqueueLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case job := <-w.enqueueCh:
			w.enqueueToRedis(ctx, job)
		}
	}
}

// pollQueue polls the Redis queue for pending jobs
func (w *Worker) pollQueue(ctx context.Context) {
	// Use RPOPLPUSH for reliability (keeps job in processing queue)
	// For simplicity, using BLPop with timeout
	result, err := w.redisClient.BLPop(ctx, w.pollInterval, ConversionQueueKey).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Error("Failed to poll conversion queue", "error", err)
		}
		return
	}

	if len(result) < 2 {
		return
	}

	// Parse job
	var job ConversionJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		slog.Error("Failed to unmarshal conversion job", "error", err)
		return
	}

	// Try to acquire lock
	if !w.acquireLock(ctx, job) {
		// Job is already being processed, skip
		slog.Debug("Conversion job already being processed, skipping", "media_id", job.MediaID, "name", job.ConversionDef.Name)
		return
	}

	// Send to job channel
	select {
	case w.jobCh <- job:
		slog.Debug("Conversion job queued for processing", "media_id", job.MediaID, "name", job.ConversionDef.Name)
	case <-ctx.Done():
		w.releaseLock(ctx, job)
	}
}

// handleJob processes a single conversion job
func (w *Worker) handleJob(ctx context.Context, job ConversionJob) {
	lockKey := w.getLockKey(job)
	w.processingJobs[lockKey] = true
	defer delete(w.processingJobs, lockKey)
	defer w.releaseLock(ctx, job)

	slog.Info("Processing conversion job",
		"media_id", job.MediaID,
		"name", job.ConversionDef.Name,
		"attempt", job.Attempt,
	)

	// Get media
	media, err := w.repo.FindByID(ctx, job.MediaID)
	if err != nil {
		slog.Error("Failed to find media for conversion", "error", err, "media_id", job.MediaID)
		w.handleError(ctx, job, err)
		return
	}

	// Get file from storage
	reader, err := w.storageDriver.Get(ctx, media.Path)
	if err != nil {
		slog.Error("Failed to get media from storage", "error", err, "media_id", job.MediaID, "path", media.Path)
		w.handleError(ctx, job, err)
		return
	}
	defer reader.Close()

	// Get handler
	handler, err := w.registry.FindHandler(media.MimeType)
	if err != nil {
		slog.Error("No handler found for media", "error", err, "media_id", job.MediaID, "mime_type", media.MimeType)
		// Don't retry - this is a permanent error
		return
	}

	// Process conversion
	conversion, err := handler.Handle(ctx, media, job.ConversionDef, reader)
	if err != nil {
		slog.Error("Conversion failed", "error", err, "media_id", job.MediaID, "name", job.ConversionDef.Name)
		w.handleError(ctx, job, err)
		return
	}

	// Save conversion record
	if err := w.repo.CreateConversion(ctx, conversion); err != nil {
		slog.Error("Failed to save conversion record", "error", err, "media_id", job.MediaID)
		w.handleError(ctx, job, err)
		return
	}

	slog.Info("Conversion job completed successfully",
		"media_id", job.MediaID,
		"name", job.ConversionDef.Name,
		"path", conversion.Path,
		"size", conversion.Size,
	)
}

// handleError handles a failed job - retry or discard
func (w *Worker) handleError(ctx context.Context, job ConversionJob, err error) {
	job.Attempt++

	if job.Attempt >= w.maxRetries {
		slog.Error("Conversion job failed after max retries",
			"media_id", job.MediaID,
			"name", job.ConversionDef.Name,
			"attempts", job.Attempt,
			"error", err,
		)
		return
	}

	// Calculate backoff
	backoff := time.Duration(CalculateBackoff(job.Attempt)) * time.Second
	slog.Info("Retrying conversion job",
		"media_id", job.MediaID,
		"name", job.ConversionDef.Name,
		"attempt", job.Attempt,
		"backoff", backoff,
	)

	// Re-enqueue with delay
	go func() {
		time.Sleep(backoff)
		select {
		case w.enqueueCh <- job:
		case <-ctx.Done():
		case <-w.stopCh:
		}
	}()
}

// acquireLock attempts to acquire a distributed lock for the job
func (w *Worker) acquireLock(ctx context.Context, job ConversionJob) bool {
	lockKey := w.getLockKey(job)
	
	// Use SET NX EX for atomic lock acquisition
	ok, err := w.redisClient.SetNX(ctx, lockKey, "1", time.Duration(JobTTL)*time.Second).Result()
	if err != nil {
		slog.Error("Failed to acquire conversion lock", "error", err, "key", lockKey)
		return false
	}

	return ok
}

// releaseLock releases the distributed lock for the job
func (w *Worker) releaseLock(ctx context.Context, job ConversionJob) {
	lockKey := w.getLockKey(job)
	if err := w.redisClient.Del(ctx, lockKey).Err(); err != nil {
		slog.Error("Failed to release conversion lock", "error", err, "key", lockKey)
	}
}

// getLockKey generates the lock key for a job
func (w *Worker) getLockKey(job ConversionJob) string {
	return fmt.Sprintf(ConversionLockKeyPattern, job.MediaID.String(), job.ConversionDef.Name)
}

// enqueueToRedis enqueues a job to the Redis queue
func (w *Worker) enqueueToRedis(ctx context.Context, job ConversionJob) {
	jobData, err := json.Marshal(job)
	if err != nil {
		slog.Error("Failed to marshal conversion job", "error", err, "media_id", job.MediaID)
		return
	}

	if err := w.redisClient.LPush(ctx, ConversionQueueKey, jobData).Err(); err != nil {
		slog.Error("Failed to enqueue conversion job", "error", err, "media_id", job.MediaID)
		return
	}

	slog.Debug("Conversion job enqueued to Redis", "media_id", job.MediaID, "name", job.ConversionDef.Name)
}

// EnqueueSync adds a job for synchronous processing in a goroutine
func (w *Worker) EnqueueSync(ctx context.Context, job ConversionJob) {
	// Try to acquire lock
	if !w.acquireLock(ctx, job) {
		slog.Debug("Conversion job already being processed, skipping sync", "media_id", job.MediaID, "name", job.ConversionDef.Name)
		return
	}

	// Process in goroutine
	go w.handleJob(context.Background(), job)
}

// EnqueueAsync adds a job to the Redis queue for async processing
func (w *Worker) EnqueueAsync(ctx context.Context, job ConversionJob) error {
	jobData, err := json.Marshal(job)
	if err != nil {
		return errors.WrapInternal(err)
	}

	// Push to queue (left push, right pop)
	if err := w.redisClient.LPush(ctx, ConversionQueueKey, jobData).Err(); err != nil {
		return errors.WrapInternal(err)
	}

	slog.Debug("Conversion job enqueued for async processing", "media_id", job.MediaID, "name", job.ConversionDef.Name)
	return nil
}

// GetQueueLength returns the current length of the conversion queue
func (w *Worker) GetQueueLength(ctx context.Context) (int64, error) {
	return w.redisClient.LLen(ctx, ConversionQueueKey).Result()
}

// ProcessConversions processes conversions based on file size
func (w *Worker) ProcessConversions(ctx context.Context, media *domain.Media) {
	if !IsSupportedImageFormat(media.MimeType) {
		return
	}

	jobs := CreateConversionJobs(media)

	if ShouldProcessSync(media.Size) {
		// Process synchronously in goroutines
		slog.Debug("Processing conversions synchronously", "media_id", media.ID, "size", media.Size)
		for _, job := range jobs {
			w.EnqueueSync(ctx, job)
		}
	} else {
		// Enqueue to Redis for async processing
		slog.Debug("Processing conversions asynchronously", "media_id", media.ID, "size", media.Size)
		for _, job := range jobs {
			if err := w.EnqueueAsync(ctx, job); err != nil {
				slog.Error("Failed to enqueue conversion", "error", err, "media_id", media.ID, "name", job.ConversionDef.Name)
			}
		}
	}
}
