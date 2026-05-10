package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/storage"
)

const (
	DefaultExportCleanupInterval = 1 * time.Hour
)

type ExportCleanup struct {
	exportJobRepo repository.ExportJobRepository
	storageDriver storage.Driver
	logger        logger.Logger
	interval      time.Duration
}

func NewExportCleanup(
	exportJobRepo repository.ExportJobRepository,
	storageDriver storage.Driver,
	logger logger.Logger,
) *ExportCleanup {
	return &ExportCleanup{
		exportJobRepo: exportJobRepo,
		storageDriver: storageDriver,
		logger:        logger,
		interval:      DefaultExportCleanupInterval,
	}
}

func (c *ExportCleanup) SetInterval(interval time.Duration) {
	if interval > 0 {
		c.interval = interval
	}
}

func (c *ExportCleanup) Run(ctx context.Context) error {
	expired, err := c.exportJobRepo.FindExpired(ctx)
	if err != nil {
		c.logger.Error(ctx, "failed to query expired export jobs", logger.Err(err))
		return fmt.Errorf("query expired export jobs: %w", err)
	}

	if len(expired) == 0 {
		c.logger.Debug(ctx, "no expired export files to clean up")
		return nil
	}

	cleaned := 0
	failed := 0

	for _, job := range expired {
		if job.FilePath == nil {
			continue
		}

		if err := c.storageDriver.Delete(ctx, *job.FilePath); err != nil {
			c.logger.Warn(ctx, "failed to delete expired export file from storage",
				logger.String("job_id", job.ID.String()),
				logger.String("file_path", *job.FilePath),
				logger.Err(err),
			)
			failed++
			continue
		}

		if err := c.exportJobRepo.ClearFileRefs(ctx, job.ID); err != nil {
			c.logger.Error(ctx, "failed to clear file refs for expired export job",
				logger.String("job_id", job.ID.String()),
				logger.Err(err),
			)
			failed++
			continue
		}

		cleaned++
	}

	c.logger.Info(ctx, "export file cleanup completed",
		logger.Int("cleaned", cleaned),
		logger.Int("failed", failed),
		logger.Int("total", len(expired)),
	)

	return nil
}

func (c *ExportCleanup) Start(ctx context.Context) {
	slog.Info("export cleanup scheduler started",
		"interval", c.interval,
	)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.Run(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("export cleanup scheduler stopped")
			return
		case <-ticker.C:
			if err := c.Run(ctx); err != nil {
				c.logger.Error(ctx, "export cleanup run failed", logger.Err(err))
			}
		}
	}
}