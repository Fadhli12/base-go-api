package domain

import (
	"time"

	"github.com/google/uuid"
)

// Job represents a unit of background work to be processed asynchronously.
type Job struct {
	ID           uuid.UUID  // Primary key
	Type         string     // Job type identifier (e.g., "email", "export")
	Payload      []byte     // JSON payload (max 1MB)
	Status       JobStatus  // pending/processing/completed/failed/dead
	AttemptCount int        // Current attempt number (1-based: 1 = first try)
	MaxRetries   int        // Maximum retry attempts (default 3)
	LastError    string     // Last error message if failed
	Result       []byte     // JSON result data if completed
	CreatedAt    time.Time // Job creation timestamp
	StartedAt    *time.Time // Processing start timestamp
	CompletedAt  *time.Time // Processing completion timestamp
	NextRetryAt  *time.Time // Scheduled retry timestamp (set during failed state for retry wait)
	CallbackURL  string     // Optional webhook URL
	UserID       uuid.UUID // Owner (from JWT)
}

// JobStatus represents the current state of a job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"     // Job is queued and waiting to be processed
	JobStatusProcessing JobStatus = "processing" // Job is currently being processed by a worker
	JobStatusCompleted  JobStatus = "completed"  // Job completed successfully
	JobStatusFailed     JobStatus = "failed"    // Job failed but may retry (next_retry_at set)
	JobStatusDead       JobStatus = "dead"       // Job exhausted all retries, requires manual resubmit
)

// JobResponse is the API response format for job data.
type JobResponse struct {
	JobID         uuid.UUID  `json:"job_id"`
	Type         string     `json:"type"`
	Status       JobStatus  `json:"status"`
	Result       []byte    `json:"result,omitempty"`
	AttemptCount int       `json:"attempt_count"`
	MaxRetries   int       `json:"max_retries"`
	LastError    string    `json:"last_error,omitempty"`
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CallbackURL  string    `json:"callback_url,omitempty"`
}

// ToResponse converts a Job to its API response format.
func (j *Job) ToResponse() *JobResponse {
	return &JobResponse{
		JobID:         j.ID,
		Type:         j.Type,
		Status:       j.Status,
		Result:       j.Result,
		AttemptCount: j.AttemptCount,
		MaxRetries:   j.MaxRetries,
		LastError:    j.LastError,
		NextRetryAt:  j.NextRetryAt,
		CreatedAt:    j.CreatedAt,
		StartedAt:    j.StartedAt,
		CompletedAt:  j.CompletedAt,
		CallbackURL:  j.CallbackURL,
	}
}

// NewJob creates a new job with default values.
func NewJob(jobType string, payload []byte, userID uuid.UUID) *Job {
	now := time.Now()
	return &Job{
		ID:           uuid.New(),
		Type:         jobType,
		Payload:      payload,
		Status:       JobStatusPending,
		AttemptCount: 1, // 1-based: first attempt is 1
		MaxRetries:   3,
		CreatedAt:   now,
		UserID:       userID,
	}
}

// IsRetryable returns true if the job can be retried.
func (j *Job) IsRetryable() bool {
	return j.Status == JobStatusFailed && j.AttemptCount < j.MaxRetries
}

// CanTransitionTo checks if the job can transition to the given status.
func (j *Job) CanTransitionTo(newStatus JobStatus) bool {
	switch j.Status {
	case JobStatusPending:
		return newStatus == JobStatusProcessing
	case JobStatusProcessing:
		return newStatus == JobStatusCompleted || newStatus == JobStatusFailed
	case JobStatusFailed:
		if newStatus == JobStatusProcessing {
			return j.AttemptCount < j.MaxRetries
		}
		return newStatus == JobStatusDead
	case JobStatusCompleted, JobStatusDead:
		return false // Terminal states
	}
	return false
}
