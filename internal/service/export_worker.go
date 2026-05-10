package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/storage"
	"github.com/google/uuid"
)

const (
	DefaultExportWorkerConcurrency = 5
	DefaultExportPollInterval      = 1 * time.Second
)

type ExportWorker struct {
	exportService ExportService
	exportJobRepo repository.ExportJobRepository
	storageDriver storage.Driver
	queue         ExportQueue
	logger        logger.Logger
	concurrency   int
	pollInterval  time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	completedCount atomic.Int64
	failedCount    atomic.Int64
}

func NewExportWorker(
	exportService ExportService,
	exportJobRepo repository.ExportJobRepository,
	storageDriver storage.Driver,
	logger logger.Logger,
	concurrency int,
) *ExportWorker {
	if concurrency <= 0 {
		concurrency = DefaultExportWorkerConcurrency
	}
	return &ExportWorker{
		exportService: exportService,
		exportJobRepo: exportJobRepo,
		storageDriver: storageDriver,
		logger:        logger,
		concurrency:   concurrency,
		pollInterval:  DefaultExportPollInterval,
	}
}

func (w *ExportWorker) SetQueue(queue ExportQueue) {
	w.queue = queue
}

func (w *ExportWorker) Start(ctx context.Context) {
	w.ctx, w.cancel = context.WithCancel(ctx)

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.processLoop()
	}

	slog.Info("Export worker started",
		"workers", w.concurrency,
	)
}

func (w *ExportWorker) Stop() {
	slog.Info("Stopping export worker...")
	w.cancel()

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Export worker stopped gracefully",
			"completed", w.completedCount.Load(),
			"failed", w.failedCount.Load(),
		)
	case <-time.After(30 * time.Second):
		slog.Error("Export worker shutdown timeout")
	}
}

func (w *ExportWorker) Metrics() map[string]int64 {
	return map[string]int64{
		"completed": w.completedCount.Load(),
		"failed":    w.failedCount.Load(),
	}
}

func (w *ExportWorker) HandleExportTask(ctx context.Context, jobID string) error {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return fmt.Errorf("invalid job ID %q: %w", jobID, err)
	}
	return w.processExport(ctx, id)
}

func (w *ExportWorker) processLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("Export worker process loop stopping")
			return
		case <-ticker.C:
			if w.queue == nil {
				continue
			}
			jobID, err := w.dequeueJob()
			if err != nil {
				slog.Error("Failed to dequeue export job", "error", err)
				continue
			}
			if jobID == "" {
				continue
			}

			id, parseErr := uuid.Parse(jobID)
			if parseErr != nil {
				slog.Error("Invalid job ID in queue",
					"job_id", jobID,
					"error", parseErr,
				)
				continue
			}

			if err := w.processExport(w.ctx, id); err != nil {
				slog.Error("Failed to process export job",
					"job_id", id,
					"error", err,
				)
			}
		}
	}
}

func (w *ExportWorker) dequeueJob() (string, error) {
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	jobs, _, err := w.exportJobRepo.FindByStatus(ctx, domain.ExportQueued, 1, 1)
	if err != nil {
		return "", fmt.Errorf("failed to query queued jobs: %w", err)
	}
	if len(jobs) == 0 {
		return "", nil
	}

	return jobs[0].ID.String(), nil
}

func (w *ExportWorker) processExport(ctx context.Context, jobID uuid.UUID) error {
	job, err := w.exportJobRepo.FindByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to find export job: %w", err)
	}

	if err := w.exportJobRepo.UpdateStatus(ctx, jobID, domain.ExportProcessing, nil); err != nil {
		return fmt.Errorf("failed to mark job as processing: %w", err)
	}

	w.logger.Info(ctx, "processing export job",
		logger.String("job_id", jobID.String()),
		logger.String("format", job.Format),
	)

	req := &ExportRequest{
		EntityTypes:    job.EntityTypes,
		Format:         job.Format,
		OrgID:          job.OrgID,
		UserID:         job.CreatedBy,
		IncludeDeleted: false,
	}

	var buf bytes.Buffer
	if err := w.exportService.StreamExport(ctx, req, &buf); err != nil {
		errMsg := err.Error()
		if updateErr := w.exportJobRepo.UpdateStatus(ctx, jobID, domain.ExportFailed, &errMsg); updateErr != nil {
			w.logger.Error(ctx, "failed to update export job status to failed",
				logger.Err(updateErr),
			)
		}
		w.failedCount.Add(1)

		w.publishFailedEvent(ctx, job)
		return fmt.Errorf("export streaming failed: %w", err)
	}

	signature := SignDataPortabilityFile(buf.Bytes(), "")

	fileName := fmt.Sprintf("exports/%s.%s", jobID.String(), job.Format)
	if err := w.storageDriver.Store(ctx, fileName, &buf); err != nil {
		errMsg := fmt.Sprintf("storage error: %v", err)
		if updateErr := w.exportJobRepo.UpdateStatus(ctx, jobID, domain.ExportFailed, &errMsg); updateErr != nil {
			w.logger.Error(ctx, "failed to update export job status after storage error",
				logger.Err(updateErr),
			)
		}
		w.failedCount.Add(1)

		w.publishFailedEvent(ctx, job)
		return fmt.Errorf("failed to store export file: %w", err)
	}

	if err := w.exportJobRepo.UpdateFilePath(ctx, jobID, fileName, 0, signature); err != nil {
		w.logger.Error(ctx, "failed to update export job file path",
			logger.Err(err),
		)
	}

	w.completedCount.Add(1)

	w.publishCompletedEvent(ctx, job)
	return nil
}

func (w *ExportWorker) publishCompletedEvent(ctx context.Context, job *domain.ExportJob) {
	if eventBus := w.getEventBus(); eventBus != nil {
		event := domain.NewExportCompletedEvent(job.ID, job.OrgID, job.EntityTypes)
		if err := eventBus.Publish(event.ToWebhookEvent()); err != nil {
			w.logger.Warn(ctx, "failed to publish export.completed event",
				logger.Err(err),
			)
		}
	}
}

func (w *ExportWorker) publishFailedEvent(ctx context.Context, job *domain.ExportJob) {
	if eventBus := w.getEventBus(); eventBus != nil {
		event := domain.NewExportFailedEvent(job.ID, job.OrgID, job.EntityTypes)
		if err := eventBus.Publish(event.ToWebhookEvent()); err != nil {
			w.logger.Warn(ctx, "failed to publish export.failed event",
				logger.Err(err),
			)
		}
	}
}

func (w *ExportWorker) getEventBus() *domain.EventBus {
	if es, ok := w.exportService.(*exportService); ok {
		return es.eventBus
	}
	return nil
}