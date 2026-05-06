package unit

import (
	"context"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"log/slog"
)

// MockJobRepository implements repository.JobRepository for testing
type MockJobRepository struct {
	mock.Mock
}

func (m *MockJobRepository) Submit(ctx context.Context, job *domain.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Job), args.Error(1)
}

func (m *MockJobRepository) Update(ctx context.Context, job *domain.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockJobRepository) ListByUser(ctx context.Context, userID uuid.UUID, status domain.JobStatus, limit, offset int) ([]*domain.Job, int, error) {
	args := m.Called(ctx, userID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Job), args.Int(1), args.Error(2)
}

func (m *MockJobRepository) Enqueue(ctx context.Context, jobID uuid.UUID, nextRetryAt time.Time) error {
	args := m.Called(ctx, jobID, nextRetryAt)
	return args.Error(0)
}

func (m *MockJobRepository) Dequeue(ctx context.Context, timeout time.Duration) (*domain.Job, error) {
	args := m.Called(ctx, timeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Job), args.Error(1)
}

func (m *MockJobRepository) SetProcessing(ctx context.Context, jobID uuid.UUID) (*domain.Job, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Job), args.Error(1)
}

func (m *MockJobRepository) SetCompleted(ctx context.Context, jobID uuid.UUID, result []byte) error {
	args := m.Called(ctx, jobID, result)
	return args.Error(0)
}

func (m *MockJobRepository) SetFailed(ctx context.Context, jobID uuid.UUID, errMsg string, nextRetryAt *time.Time) error {
	args := m.Called(ctx, jobID, errMsg, nextRetryAt)
	return args.Error(0)
}

func (m *MockJobRepository) GetStuckJobs(ctx context.Context, threshold time.Duration) ([]*domain.Job, error) {
	args := m.Called(ctx, threshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Job), args.Error(1)
}

func (m *MockJobRepository) Delete(ctx context.Context, jobID uuid.UUID) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

// Helper to create JobService for testing
func newTestJobService(mockRepo *MockJobRepository) *service.JobService {
	cfg := config.DefaultJobConfig()
	log := slog.Default()
	return service.NewJobService(mockRepo, &cfg, log)
}

// TestSubmit tests job submission
func TestSubmit(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	userID := uuid.New()
	req := &service.SubmitRequest{
		Type:       "test.job",
		Payload:    []byte(`{"key":"value"}`),
		MaxRetries: 3,
	}

	mockRepo.On("Submit", ctx, mock.AnythingOfType("*domain.Job")).Return(nil)

	job, err := svc.Submit(ctx, userID, req)

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, domain.JobStatusPending, job.Status)
	assert.Equal(t, "test.job", job.Type)
	assert.Equal(t, 3, job.MaxRetries)
	assert.Equal(t, userID, job.UserID)
	mockRepo.AssertExpectations(t)
}

// TestSubmit_WithWebhookURL tests job submission with webhook callback
func TestSubmit_WithWebhookURL(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	userID := uuid.New()
	webhookURL := "https://example.com/callback"
	req := &service.SubmitRequest{
		Type:       "test.job",
		Payload:    []byte(`{"key":"value"}`),
		WebhookURL: webhookURL,
	}

	mockRepo.On("Submit", ctx, mock.AnythingOfType("*domain.Job")).Return(nil)

	job, err := svc.Submit(ctx, userID, req)

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, webhookURL, job.CallbackURL)
	mockRepo.AssertExpectations(t)
}

// TestSubmit_UsesDefaultRetries tests that default retries are used when not specified
func TestSubmit_UsesDefaultRetries(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	userID := uuid.New()
	req := &service.SubmitRequest{
		Type:    "test.job",
		Payload: []byte(`{"key":"value"}`),
		// MaxRetries not set, should use default (3)
	}

	mockRepo.On("Submit", ctx, mock.AnythingOfType("*domain.Job")).Return(nil)

	job, err := svc.Submit(ctx, userID, req)

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, 3, job.MaxRetries) // Default from config
	mockRepo.AssertExpectations(t)
}

// TestGetByID tests retrieving a job by ID
func TestGetByID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	expectedJob := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusPending,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(expectedJob, nil)

	job, err := svc.GetByID(ctx, jobID)

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, jobID, job.ID)
	assert.Equal(t, "test.job", job.Type)
	mockRepo.AssertExpectations(t)
}

// TestGetByID_NotFound tests retrieving a non-existent job
func TestGetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()

	mockRepo.On("GetByID", ctx, jobID).Return(nil, apperrors.ErrNotFound)

	job, err := svc.GetByID(ctx, jobID)

	require.Error(t, err)
	require.Nil(t, job)
	assert.True(t, apperrors.IsNotFound(err))
	mockRepo.AssertExpectations(t)
}

// TestListByUser tests listing jobs for a user
func TestListByUser(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	userID := uuid.New()
	expectedJobs := []*domain.Job{
		{ID: uuid.New(), Type: "job.1", Status: domain.JobStatusPending},
		{ID: uuid.New(), Type: "job.2", Status: domain.JobStatusCompleted},
	}

	mockRepo.On("ListByUser", ctx, userID, domain.JobStatus(""), 50, 0).Return(expectedJobs, 2, nil)

	jobs, total, err := svc.ListByUser(ctx, userID, "", 50, 0)

	require.NoError(t, err)
	assert.Len(t, jobs, 2)
	assert.Equal(t, 2, total)
	mockRepo.AssertExpectations(t)
}

// TestListByUser_WithStatus tests listing jobs with status filter
func TestListByUser_WithStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	userID := uuid.New()
	expectedJobs := []*domain.Job{
		{ID: uuid.New(), Type: "job.1", Status: domain.JobStatusPending},
	}

	mockRepo.On("ListByUser", ctx, userID, domain.JobStatusPending, 50, 0).Return(expectedJobs, 1, nil)

	jobs, total, err := svc.ListByUser(ctx, userID, domain.JobStatusPending, 50, 0)

	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, 1, total)
	assert.Equal(t, domain.JobStatusPending, jobs[0].Status)
	mockRepo.AssertExpectations(t)
}

// TestCancel tests cancelling a pending job
func TestCancel(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	userID := uuid.New()
	job := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusPending,
		UserID: userID,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Job")).Return(nil)

	err := svc.Cancel(ctx, jobID, userID)

	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusDead, job.Status)
	mockRepo.AssertExpectations(t)
}

// TestCancel_FailedJob tests cancelling a failed job
func TestCancel_FailedJob(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	userID := uuid.New()
	job := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusFailed,
		UserID: userID,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Job")).Return(nil)

	err := svc.Cancel(ctx, jobID, userID)

	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusDead, job.Status)
	mockRepo.AssertExpectations(t)
}

// TestCancel_OwnershipFailure tests that non-owners cannot cancel
func TestCancel_OwnershipFailure(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	ownerID := uuid.New()
	attackerID := uuid.New()
	job := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusPending,
		UserID: ownerID,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)

	err := svc.Cancel(ctx, jobID, attackerID)

	require.Error(t, err)
	assert.True(t, apperrors.IsForbidden(err))
	mockRepo.AssertExpectations(t)
}

// TestCancel_InvalidStateTransition tests cancelling a processing job
func TestCancel_InvalidStateTransition(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	userID := uuid.New()
	job := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusProcessing, // Cannot cancel processing job
		UserID: userID,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)

	err := svc.Cancel(ctx, jobID, userID)

	require.Error(t, err)
	assert.True(t, apperrors.IsConflict(err))
	mockRepo.AssertExpectations(t)
}

// TestCancel_CompletedJob tests that completed jobs cannot be cancelled
func TestCancel_CompletedJob(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	userID := uuid.New()
	job := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusCompleted, // Terminal state
		UserID: userID,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)

	err := svc.Cancel(ctx, jobID, userID)

	require.Error(t, err)
	assert.True(t, apperrors.IsConflict(err))
	mockRepo.AssertExpectations(t)
}

// TestResubmit tests resubmitting a dead job
func TestResubmit(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	userID := uuid.New()
	job := &domain.Job{
		ID:           jobID,
		Type:         "test.job",
		Status:       domain.JobStatusDead,
		AttemptCount: 3,
		UserID:       userID,
		LastError:    "Previous failure",
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Job")).Return(nil)
	mockRepo.On("Enqueue", ctx, jobID, mock.AnythingOfType("time.Time")).Return(nil)

	resubmittedJob, err := svc.Resubmit(ctx, jobID, userID)

	require.NoError(t, err)
	require.NotNil(t, resubmittedJob)
	assert.Equal(t, domain.JobStatusPending, resubmittedJob.Status)
	assert.Equal(t, 0, resubmittedJob.AttemptCount)
	assert.Empty(t, resubmittedJob.LastError)
	assert.Nil(t, resubmittedJob.Result)
	mockRepo.AssertExpectations(t)
}

// TestResubmit_OnlyDeadJobs tests that only dead jobs can be resubmitted
func TestResubmit_OnlyDeadJobs(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	userID := uuid.New()
	job := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusPending, // Not dead
		UserID: userID,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)

	resubmittedJob, err := svc.Resubmit(ctx, jobID, userID)

	require.Error(t, err)
	require.Nil(t, resubmittedJob)
	assert.True(t, apperrors.IsConflict(err))
	mockRepo.AssertExpectations(t)
}

// TestResubmit_OwnershipFailure tests that non-owners cannot resubmit
func TestResubmit_OwnershipFailure(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	ownerID := uuid.New()
	attackerID := uuid.New()
	job := &domain.Job{
		ID:     jobID,
		Type:   "test.job",
		Status: domain.JobStatusDead,
		UserID: ownerID,
	}

	mockRepo.On("GetByID", ctx, jobID).Return(job, nil)

	resubmittedJob, err := svc.Resubmit(ctx, jobID, attackerID)

	require.Error(t, err)
	require.Nil(t, resubmittedJob)
	assert.True(t, apperrors.IsForbidden(err))
	mockRepo.AssertExpectations(t)
}

// TestResubmit_NotFound tests resubmitting a non-existent job
func TestResubmit_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJobRepository)
	svc := newTestJobService(mockRepo)

	jobID := uuid.New()
	userID := uuid.New()

	mockRepo.On("GetByID", ctx, jobID).Return(nil, apperrors.ErrNotFound)

	resubmittedJob, err := svc.Resubmit(ctx, jobID, userID)

	require.Error(t, err)
	require.Nil(t, resubmittedJob)
	assert.True(t, apperrors.IsNotFound(err))
	mockRepo.AssertExpectations(t)
}
