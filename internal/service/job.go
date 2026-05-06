package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

type JobService struct {
	repo   repository.JobRepository
	config *config.JobConfig
	logger *slog.Logger
}

func NewJobService(repo repository.JobRepository, cfg *config.JobConfig, log *slog.Logger) *JobService {
	return &JobService{
		repo:   repo,
		config: cfg,
		logger: log,
	}
}

type SubmitRequest struct {
	Type       string
	Payload    []byte
	MaxRetries int
	WebhookURL string
}

func (s *JobService) Submit(ctx context.Context, userID uuid.UUID, req *SubmitRequest) (*domain.Job, error) {
	job := domain.NewJob(req.Type, req.Payload, userID)

	if req.MaxRetries > 0 {
		job.MaxRetries = req.MaxRetries
	} else {
		job.MaxRetries = s.config.MaxRetries
	}

	if req.WebhookURL != "" {
		job.CallbackURL = req.WebhookURL
	}

	nextRetry := time.Now().Add(time.Duration(s.config.JobTimeoutSeconds) * time.Second)
	job.NextRetryAt = &nextRetry

	s.logger.Info("job submitted",
		slog.String("job_id", job.ID.String()),
		slog.String("type", job.Type),
		slog.String("user_id", userID.String()),
		slog.Int("max_retries", job.MaxRetries),
	)

	if err := s.repo.Submit(ctx, job); err != nil {
		s.logger.Error("failed to submit job",
			slog.String("error", err.Error()),
			slog.String("job_id", job.ID.String()),
		)
		return nil, errors.WrapInternal(err)
	}

	return job, nil
}

func (s *JobService) GetByID(ctx context.Context, jobID uuid.UUID) (*domain.Job, error) {
	s.logger.Debug("getting job", slog.String("job_id", jobID.String()))

	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.WrapNotFound(err, "job")
		}
		s.logger.Error("failed to get job", slog.String("error", err.Error()))
		return nil, errors.WrapInternal(err)
	}

	s.logger.Debug("job retrieved", slog.String("job_id", jobID.String()))
	return job, nil
}

func (s *JobService) ListByUser(ctx context.Context, userID uuid.UUID, status domain.JobStatus, limit, offset int) ([]*domain.Job, int, error) {
	s.logger.Debug("listing jobs",
		slog.String("user_id", userID.String()),
		slog.String("status", string(status)),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
	)

	jobs, total, err := s.repo.ListByUser(ctx, userID, status, limit, offset)
	if err != nil {
		s.logger.Error("failed to list jobs", slog.String("error", err.Error()))
		return nil, 0, errors.WrapInternal(err)
	}

	s.logger.Debug("jobs listed",
		slog.String("user_id", userID.String()),
		slog.Int("count", len(jobs)),
		slog.Int("total", total),
	)
	return jobs, total, nil
}

func (s *JobService) Cancel(ctx context.Context, jobID, userID uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.WrapNotFound(err, "job")
		}
		s.logger.Error("failed to get job for cancellation", slog.String("error", err.Error()))
		return errors.WrapInternal(err)
	}

	if job.UserID != userID {
		s.logger.Warn("unauthorized cancellation attempt",
			slog.String("job_id", jobID.String()),
			slog.String("owner_id", job.UserID.String()),
			slog.String("requester_id", userID.String()),
		)
		return errors.ErrForbidden
	}

	if job.Status != domain.JobStatusPending && job.Status != domain.JobStatusFailed {
		s.logger.Warn("invalid state transition for cancellation",
			slog.String("job_id", jobID.String()),
			slog.String("current_status", string(job.Status)),
		)
		return errors.NewAppError("INVALID_STATE_TRANSITION", "cannot cancel job in current state", 409)
	}

	job.Status = domain.JobStatusDead

	if err := s.repo.Update(ctx, job); err != nil {
		s.logger.Error("failed to cancel job", slog.String("error", err.Error()))
		return errors.WrapInternal(err)
	}

	s.logger.Info("job cancelled",
		slog.String("job_id", jobID.String()),
		slog.String("user_id", userID.String()),
		slog.String("previous_status", string(job.Status)),
	)

	return nil
}

func (s *JobService) Resubmit(ctx context.Context, jobID, userID uuid.UUID) (*domain.Job, error) {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.WrapNotFound(err, "job")
		}
		s.logger.Error("failed to get job for resubmit", slog.String("error", err.Error()))
		return nil, errors.WrapInternal(err)
	}

	if job.UserID != userID {
		s.logger.Warn("unauthorized resubmit attempt",
			slog.String("job_id", jobID.String()),
			slog.String("owner_id", job.UserID.String()),
			slog.String("requester_id", userID.String()),
		)
		return nil, errors.ErrForbidden
	}

	if job.Status != domain.JobStatusDead {
		s.logger.Warn("invalid state transition for resubmit",
			slog.String("job_id", jobID.String()),
			slog.String("current_status", string(job.Status)),
		)
		return nil, errors.ErrConflict
	}

	job.Status = domain.JobStatusPending
	job.AttemptCount = 0
	job.Result = nil
	job.LastError = ""
	nextRetry := time.Now().Add(time.Duration(s.config.JobTimeoutSeconds) * time.Second)
	job.NextRetryAt = &nextRetry

	if err := s.repo.Update(ctx, job); err != nil {
		s.logger.Error("failed to resubmit job", slog.String("error", err.Error()))
		return nil, errors.WrapInternal(err)
	}

	if err := s.repo.Enqueue(ctx, job.ID, *job.NextRetryAt); err != nil {
		s.logger.Error("failed to enqueue resubmitted job", slog.String("error", err.Error()))
		return nil, errors.WrapInternal(err)
	}

	s.logger.Info("job resubmitted",
		slog.String("job_id", jobID.String()),
		slog.String("user_id", userID.String()),
	)

	return job, nil
}