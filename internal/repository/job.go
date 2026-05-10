package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	defaultQueueKey  = "jobs:queue"
	jobDataKeyFmt    = "jobs:data:%s"
	jobUserKeyFmt    = "jobs:user:%s"
	jobStatusKeyFmt = "jobs:status:%s"

	// Job data TTL (7 days)
	jobDataTTL = 7 * 24 * time.Hour
)

// JobRepository defines job persistence operations.
type JobRepository interface {
	// Submit creates a new job in pending state and enqueues it.
	Submit(ctx context.Context, job *domain.Job) error

	// GetByID retrieves a job by its ID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error)

	// Update updates an existing job.
	Update(ctx context.Context, job *domain.Job) error

	// ListByUser retrieves jobs for a user with optional status filter.
	ListByUser(ctx context.Context, userID uuid.UUID, status domain.JobStatus, limit, offset int) ([]*domain.Job, int, error)

	// Enqueue adds a job to the queue (for initial submit and retry).
	Enqueue(ctx context.Context, jobID uuid.UUID, nextRetryAt time.Time) error

	// Dequeue claims the next available job from the queue.
	// Returns nil if queue is empty or times out.
	Dequeue(ctx context.Context, timeout time.Duration) (*domain.Job, error)

	// SetProcessing atomically claims a job and sets status to processing.
	SetProcessing(ctx context.Context, jobID uuid.UUID) (*domain.Job, error)

	// SetCompleted marks a job as completed.
	SetCompleted(ctx context.Context, jobID uuid.UUID, result []byte) error

	// SetFailed marks a job as failed with error. If retries remain, schedules retry.
	SetFailed(ctx context.Context, jobID uuid.UUID, errMsg string, nextRetryAt *time.Time) error

	// GetStuckJobs finds jobs in processing state longer than threshold.
	GetStuckJobs(ctx context.Context, threshold time.Duration) ([]*domain.Job, error)

	// ResetStuckJob resets a stuck job from processing to pending and updates Redis indexes.
	ResetStuckJob(ctx context.Context, job *domain.Job, nextRetryAt time.Time) error

	// Delete removes a job (for dead letter cleanup).
	Delete(ctx context.Context, jobID uuid.UUID) error
}

// jobRedisRepository implements JobRepository using Redis.
type jobRedisRepository struct {
	client   *redis.Client
	queueKey string
}

// NewJobRepository creates a new JobRepository with Redis backend.
func NewJobRepository(client *redis.Client) JobRepository {
	return &jobRedisRepository{
		client:   client,
		queueKey: defaultQueueKey,
	}
}

// NewJobRepositoryWithQueueKey creates a JobRepository with a custom queue key.
func NewJobRepositoryWithQueueKey(client *redis.Client, queueKey string) JobRepository {
	return &jobRedisRepository{
		client:   client,
		queueKey: queueKey,
	}
}

// Submit creates a new job in pending state and enqueues it.
func (r *jobRedisRepository) Submit(ctx context.Context, job *domain.Job) error {
	job.Status = domain.JobStatusPending
	job.AttemptCount = 1
	job.CreatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return errors.WrapInternal(err)
	}

	pipe := r.client.Pipeline()

	// Store job data
	dataKey := fmt.Sprintf(jobDataKeyFmt, job.ID.String())
	pipe.Set(ctx, dataKey, data, jobDataTTL)

	// Add to user index
	userKey := fmt.Sprintf(jobUserKeyFmt, job.UserID.String())
	pipe.ZAdd(ctx, userKey, redis.Z{Score: float64(job.CreatedAt.Unix()), Member: job.ID.String()})
	pipe.Expire(ctx, userKey, jobDataTTL)

	// Add to status index
	statusKey := fmt.Sprintf(jobStatusKeyFmt, job.Status)
	pipe.SAdd(ctx, statusKey, job.ID.String())
	pipe.Expire(ctx, statusKey, jobDataTTL)

	// Enqueue in priority order (next_retry_at = now for new jobs)
	nextRetry := job.CreatedAt
	pipe.ZAdd(ctx, r.queueKey, redis.Z{Score: float64(nextRetry.UnixMilli()), Member: job.ID.String()})

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// GetByID retrieves a job by its ID.
func (r *jobRedisRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	dataKey := fmt.Sprintf(jobDataKeyFmt, id.String())
	data, err := r.client.Get(ctx, dataKey).Bytes()
	if err == redis.Nil {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	var job domain.Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, errors.WrapInternal(err)
	}

	return &job, nil
}

// Update updates an existing job.
func (r *jobRedisRepository) Update(ctx context.Context, job *domain.Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return errors.WrapInternal(err)
	}

	dataKey := fmt.Sprintf(jobDataKeyFmt, job.ID.String())
	err = r.client.Set(ctx, dataKey, data, jobDataTTL).Err()
	if err == redis.Nil {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// ListByUser retrieves jobs for a user with optional status filter.
func (r *jobRedisRepository) ListByUser(ctx context.Context, userID uuid.UUID, status domain.JobStatus, limit, offset int) ([]*domain.Job, int, error) {
	userKey := fmt.Sprintf(jobUserKeyFmt, userID.String())

	// Get job IDs from user index (sorted by created_at desc)
	jobIDs, err := r.client.ZRevRange(ctx, userKey, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Get total count
	total, err := r.client.ZCard(ctx, userKey).Result()
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	if len(jobIDs) == 0 {
		return []*domain.Job{}, int(total), nil
	}

	// Fetch job data
	jobs := make([]*domain.Job, 0, len(jobIDs))
	for _, idStr := range jobIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		job, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}
		// Filter by status if specified
		if status != "" && job.Status != status {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, int(total), nil
}

// Enqueue adds a job to the queue (for initial submit and retry).
func (r *jobRedisRepository) Enqueue(ctx context.Context, jobID uuid.UUID, nextRetryAt time.Time) error {
	err := r.client.ZAdd(ctx, r.queueKey, redis.Z{
		Score:  float64(nextRetryAt.UnixMilli()),
		Member: jobID.String(),
	}).Err()
	if err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// Dequeue claims the next available job from the queue.
// Returns nil if queue is empty or times out.
func (r *jobRedisRepository) Dequeue(ctx context.Context, timeout time.Duration) (*domain.Job, error) {
	// Use ZPOPMIN to get the next job due for processing
	result, err := r.client.ZPopMin(ctx, r.queueKey, 1).Result()
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	if len(result) == 0 {
		return nil, nil
	}

	jobIDStr, ok := result[0].Member.(string)
	if !ok {
		return nil, errors.ErrInternal
	}

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		return nil, errors.ErrInternal
	}

	return r.GetByID(ctx, jobID)
}

// SetProcessing atomically claims a job and sets status to processing.
func (r *jobRedisRepository) SetProcessing(ctx context.Context, jobID uuid.UUID) (*domain.Job, error) {
	job, err := r.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}

	previousStatus := job.Status

	if previousStatus != domain.JobStatusPending && !(previousStatus == domain.JobStatusFailed && job.AttemptCount < job.MaxRetries) {
		return nil, errors.ErrConflict
	}

	now := time.Now()
	job.Status = domain.JobStatusProcessing
	job.StartedAt = &now

	if err := r.Update(ctx, job); err != nil {
		return nil, err
	}

	pipe := r.client.Pipeline()
	oldStatusKey := fmt.Sprintf(jobStatusKeyFmt, previousStatus)
	pipe.SRem(ctx, oldStatusKey, jobID.String())
	newStatusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusProcessing)
	pipe.SAdd(ctx, newStatusKey, jobID.String())
	pipe.Expire(ctx, newStatusKey, jobDataTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	return job, nil
}

// SetCompleted marks a job as completed.
func (r *jobRedisRepository) SetCompleted(ctx context.Context, jobID uuid.UUID, result []byte) error {
	job, err := r.GetByID(ctx, jobID)
	if err != nil {
		return err
	}

	if !job.CanTransitionTo(domain.JobStatusCompleted) {
		return errors.ErrConflict
	}

	now := time.Now()
	job.Status = domain.JobStatusCompleted
	job.Result = result
	job.CompletedAt = &now

	if err := r.Update(ctx, job); err != nil {
		return err
	}

	// Remove from status index and add to completed
	pipe := r.client.Pipeline()
	oldStatusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusProcessing)
	pipe.SRem(ctx, oldStatusKey, jobID.String())
	newStatusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusCompleted)
	pipe.SAdd(ctx, newStatusKey, jobID.String())
	pipe.Expire(ctx, newStatusKey, jobDataTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// SetFailed marks a job as failed with error. If retries remain, schedules retry.
func (r *jobRedisRepository) SetFailed(ctx context.Context, jobID uuid.UUID, errMsg string, nextRetryAt *time.Time) error {
	job, err := r.GetByID(ctx, jobID)
	if err != nil {
		return err
	}

	job.LastError = errMsg

	if job.AttemptCount < job.MaxRetries && nextRetryAt != nil {
		// Schedule retry
		job.AttemptCount++
		job.Status = domain.JobStatusFailed
		job.NextRetryAt = nextRetryAt

		if err := r.Update(ctx, job); err != nil {
			return err
		}

		pipe := r.client.Pipeline()
		oldStatusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusProcessing)
		pipe.SRem(ctx, oldStatusKey, jobID.String())
		newStatusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusFailed)
		pipe.SAdd(ctx, newStatusKey, jobID.String())
		pipe.Expire(ctx, newStatusKey, jobDataTTL)
		_, err = pipe.Exec(ctx)
		if err != nil {
			return errors.WrapInternal(err)
		}

		// Re-enqueue with delay
		return r.Enqueue(ctx, jobID, *nextRetryAt)
	}

	// No retries left - mark as dead
	job.AttemptCount++
	job.Status = domain.JobStatusDead
	job.CompletedAt = func() *time.Time { now := time.Now(); return &now }()
	job.NextRetryAt = nil

	if err := r.Update(ctx, job); err != nil {
		return err
	}

	// Update status index
	pipe := r.client.Pipeline()
	oldStatusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusProcessing)
	pipe.SRem(ctx, oldStatusKey, jobID.String())
	newStatusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusDead)
	pipe.SAdd(ctx, newStatusKey, jobID.String())
	pipe.Expire(ctx, newStatusKey, jobDataTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// GetStuckJobs finds jobs in processing state longer than threshold.
func (r *jobRedisRepository) GetStuckJobs(ctx context.Context, threshold time.Duration) ([]*domain.Job, error) {
	// Get all processing jobs from status index
	statusKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusProcessing)
	jobIDStrs, err := r.client.SMembers(ctx, statusKey).Result()
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	var stuckJobs []*domain.Job
	cutoff := time.Now().Add(-threshold)

	for _, idStr := range jobIDStrs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}

		job, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}

		if job.StartedAt != nil && job.StartedAt.Before(cutoff) {
			stuckJobs = append(stuckJobs, job)
		}
	}

	return stuckJobs, nil
}

func (r *jobRedisRepository) ResetStuckJob(ctx context.Context, job *domain.Job, nextRetryAt time.Time) error {
	job.Status = domain.JobStatusPending
	job.AttemptCount++
	job.NextRetryAt = &nextRetryAt

	if err := r.Update(ctx, job); err != nil {
		return err
	}

	processingKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusProcessing)
	pendingKey := fmt.Sprintf(jobStatusKeyFmt, domain.JobStatusPending)

	pipe := r.client.Pipeline()
	pipe.SRem(ctx, processingKey, job.ID.String())
	pipe.SAdd(ctx, pendingKey, job.ID.String())

	_, err := pipe.Exec(ctx)
	if err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}

// Delete removes a job (for dead letter cleanup).
func (r *jobRedisRepository) Delete(ctx context.Context, jobID uuid.UUID) error {
	job, err := r.GetByID(ctx, jobID)
	if err != nil {
		return err
	}

	pipe := r.client.Pipeline()

	// Remove job data
	dataKey := fmt.Sprintf(jobDataKeyFmt, jobID.String())
	pipe.Del(ctx, dataKey)

	// Remove from queue if present
	pipe.ZRem(ctx, r.queueKey, jobID.String())

	// Remove from user index
	userKey := fmt.Sprintf(jobUserKeyFmt, job.UserID.String())
	pipe.ZRem(ctx, userKey, jobID.String())

	// Remove from status index
	statusKey := fmt.Sprintf(jobStatusKeyFmt, job.Status)
	pipe.SRem(ctx, statusKey, jobID.String())

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errors.WrapInternal(err)
	}

	return nil
}