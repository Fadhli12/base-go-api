package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/repository"
	"github.com/google/uuid"
)

const (
	importWorkerDefaultConcurrency = 5
	importWorkerPollInterval       = 2 * time.Second
	importWorkerStuckTimeout       = 5 * time.Minute
	importWorkerStuckReaperInterval = 60 * time.Second
	importWorkerShutdownTimeout    = 30 * time.Second
)

type ImportWorker struct {
	importService ImportService
	jobRepo       repository.ImportJobRepository
	logger        logger.Logger
	concurrency   int
	eventBus      *domain.EventBus

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	processedCount atomic.Int64
	failedCount    atomic.Int64
}

func NewImportWorker(
	importService ImportService,
	jobRepo repository.ImportJobRepository,
	log logger.Logger,
	concurrency int,
) *ImportWorker {
	if concurrency <= 0 {
		concurrency = importWorkerDefaultConcurrency
	}
	return &ImportWorker{
		importService: importService,
		jobRepo:       jobRepo,
		logger:        log,
		concurrency:   concurrency,
	}
}

func (w *ImportWorker) SetEventBus(eventBus *domain.EventBus) {
	w.eventBus = eventBus
}

func (w *ImportWorker) Start(ctx context.Context) {
	w.ctx, w.cancel = context.WithCancel(ctx)

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.processLoop(i)
	}

	w.wg.Add(1)
	go w.stuckJobReaper()

	slog.Info("Import worker started",
		"workers", w.concurrency,
	)
}

func (w *ImportWorker) Stop() {
	slog.Info("Stopping import worker...")
	w.cancel()

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Import worker stopped gracefully",
			"processed", w.processedCount.Load(),
			"failed", w.failedCount.Load(),
		)
	case <-time.After(importWorkerShutdownTimeout):
		slog.Error("Import worker shutdown timeout")
	}
}

func (w *ImportWorker) Metrics() map[string]int64 {
	return map[string]int64{
		"processed": w.processedCount.Load(),
		"failed":    w.failedCount.Load(),
	}
}

func (w *ImportWorker) processLoop(workerID int) {
	defer w.wg.Done()

	ticker := time.NewTicker(importWorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("Import worker process loop stopping", "worker_id", workerID)
			return
		case <-ticker.C:
			jobs, err := w.jobRepo.FindQueued(w.ctx, 1)
			if err != nil {
				slog.Error("Failed to find queued import jobs", "error", err)
				continue
			}
			if len(jobs) == 0 {
				continue
			}

			job := jobs[0]
			if err := w.processJob(job.ID); err != nil {
				slog.Error("Failed to process import job",
					"job_id", job.ID,
					"error", err,
				)
			}
		}
	}
}

func (w *ImportWorker) processJob(jobID uuid.UUID) error {
	ctx := w.ctx

	claimed, err := w.jobRepo.ClaimJob(ctx, jobID, domain.ImportQueued, domain.ImportProcessing)
	if err != nil {
		slog.Error("Failed to claim import job",
			"job_id", jobID,
			"error", err,
		)
		return fmt.Errorf("claim import job: %w", err)
	}
	if !claimed {
		slog.Info("Import job already claimed by another worker",
			"job_id", jobID,
		)
		return nil
	}

	now := time.Now()
	if err := w.jobRepo.UpdateProcessingStartedAt(ctx, jobID, now); err != nil {
		slog.Error("Failed to mark import job processing started_at",
			"job_id", jobID,
			"error", err,
		)
	}

	slog.Info("Processing import job", "job_id", jobID)

	if err := w.importService.ProcessImport(ctx, jobID); err != nil {
		w.failedCount.Add(1)
		slog.Error("Import job failed",
			"job_id", jobID,
			"error", err,
		)
		return err
	}

	w.processedCount.Add(1)
	return nil
}

func (w *ImportWorker) HandleImportTask(ctx context.Context, payload []byte) error {
	var p ImportTaskPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		slog.Error("Failed to unmarshal import task payload", "error", err)
		return err
	}

	jobID, err := uuid.Parse(p.JobID)
	if err != nil {
		slog.Error("Invalid job ID in import task payload",
			"job_id", p.JobID,
			"error", err,
		)
		return err
	}

	return w.processJob(jobID)
}

func (w *ImportWorker) stuckJobReaper() {
	defer w.wg.Done()

	ticker := time.NewTicker(importWorkerStuckReaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("Import stuck job reaper stopping")
			return
		case <-ticker.C:
			stuckJobs, err := w.jobRepo.FindStuckProcessing(w.ctx, importWorkerStuckTimeout)
			if err != nil {
				slog.Error("Failed to find stuck import jobs", "error", err)
				continue
			}

			for _, job := range stuckJobs {
				slog.Warn("Recovering stuck import job",
					"job_id", job.ID,
					"status", job.Status,
					"processing_started_at", job.ProcessingStartedAt,
				)

				errMsg := "processing timeout: worker recovery"
				if err := w.jobRepo.UpdateStatus(w.ctx, job.ID, domain.ImportQueued, &errMsg); err != nil {
					slog.Error("Failed to reset stuck import job",
						"job_id", job.ID,
						"error", err,
					)
					continue
				}

				slog.Info("Stuck import job recovered and requeued",
					"job_id", job.ID,
				)
			}
		}
	}
}