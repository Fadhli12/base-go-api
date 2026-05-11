package unit

import (
	"context"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockImportJobRepository struct {
	mock.Mock
}

var _ repository.ImportJobRepository = (*MockImportJobRepository)(nil)

func (m *MockImportJobRepository) Create(ctx context.Context, job *domain.ImportJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockImportJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ImportJob), args.Error(1)
}

func (m *MockImportJobRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.ImportJob, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ImportJob), args.Error(1)
}

func (m *MockImportJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *MockImportJobRepository) ClaimJob(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error) {
	args := m.Called(ctx, id, fromStatus, toStatus)
	return args.Bool(0), args.Error(1)
}

func (m *MockImportJobRepository) UpdateResult(ctx context.Context, id uuid.UUID, result domain.ImportResult) error {
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

func (m *MockImportJobRepository) UpdateSourceFilePath(ctx context.Context, id uuid.UUID, path string) error {
	args := m.Called(ctx, id, path)
	return args.Error(0)
}

func (m *MockImportJobRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockImportJobRepository) List(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, int64, error) {
	args := m.Called(ctx, orgID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.ImportJob), args.Get(1).(int64), args.Error(2)
}

func (m *MockImportJobRepository) FindQueued(ctx context.Context, limit int) ([]*domain.ImportJob, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ImportJob), args.Error(1)
}

func (m *MockImportJobRepository) FindStuckProcessing(ctx context.Context, timeout time.Duration) ([]*domain.ImportJob, error) {
	args := m.Called(ctx, timeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ImportJob), args.Error(1)
}

func (m *MockImportJobRepository) UpdateProcessingStartedAt(ctx context.Context, id uuid.UUID, startedAt time.Time) error {
	args := m.Called(ctx, id, startedAt)
	return args.Error(0)
}

func TestMockImportJobRepository_Create(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	job := &domain.ImportJob{
		EntityTypes:      []string{"organizations", "users"},
		Format:           "json",
		ConflictStrategy: "skip",
		CreatedBy:        uuid.New(),
	}

	mockRepo.On("Create", ctx, job).Return(nil)

	err := mockRepo.Create(ctx, job)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()
	expectedJob := &domain.ImportJob{
		ID:               id,
		Status:           domain.ImportCompleted,
		EntityTypes:      []string{"organizations"},
		Format:           "json",
		ConflictStrategy: "skip",
		CreatedBy:        uuid.New(),
	}

	mockRepo.On("FindByID", ctx, id).Return(expectedJob, nil)

	job, err := mockRepo.FindByID(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, expectedJob.ID, job.ID)
	assert.Equal(t, expectedJob.Status, job.Status)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()

	mockRepo.On("FindByID", ctx, id).Return(nil, errors.ErrNotFound)

	job, err := mockRepo.FindByID(ctx, id)
	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_FindByIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	key := "unique-key-123"
	expectedJob := &domain.ImportJob{
		ID:              uuid.New(),
		IdempotencyKey: &key,
	}

	mockRepo.On("FindByIdempotencyKey", ctx, key).Return(expectedJob, nil)

	job, err := mockRepo.FindByIdempotencyKey(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, key, *job.IdempotencyKey)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_FindByIdempotencyKey_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	mockRepo.On("FindByIdempotencyKey", ctx, "missing").Return(nil, errors.ErrNotFound)

	job, err := mockRepo.FindByIdempotencyKey(ctx, "missing")
	assert.Error(t, err)
	assert.Nil(t, job)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()
	errMsg := "import failed"

	mockRepo.On("UpdateStatus", ctx, id, domain.ImportFailed, &errMsg).Return(nil)

	err := mockRepo.UpdateStatus(ctx, id, domain.ImportFailed, &errMsg)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_UpdateStatus_NullErrorMessage(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()

	mockRepo.On("UpdateStatus", ctx, id, domain.ImportProcessing, (*string)(nil)).Return(nil)

	err := mockRepo.UpdateStatus(ctx, id, domain.ImportProcessing, nil)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_UpdateResult(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()
	result := domain.ImportResult{
		TotalCreated:     10,
		TotalSkipped:      2,
		TotalFailed:       1,
		TotalOverwritten:  3,
		EntityTypes: map[string]domain.EntityTypeResult{
			"organizations": {Created: 5, Skipped: 1, Failed: 0, Overwritten: 2},
			"users":         {Created: 5, Skipped: 1, Failed: 1, Overwritten: 1},
		},
	}

	mockRepo.On("UpdateResult", ctx, id, result).Return(nil)

	err := mockRepo.UpdateResult(ctx, id, result)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_UpdateSourceFilePath(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()

	mockRepo.On("UpdateSourceFilePath", ctx, id, "/tmp/import.json").Return(nil)

	err := mockRepo.UpdateSourceFilePath(ctx, id, "/tmp/import.json")
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_SoftDelete(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()

	mockRepo.On("SoftDelete", ctx, id).Return(nil)

	err := mockRepo.SoftDelete(ctx, id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_SoftDelete_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()

	mockRepo.On("SoftDelete", ctx, id).Return(errors.ErrNotFound)

	err := mockRepo.SoftDelete(ctx, id)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_List(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	jobs := []*domain.ImportJob{
		{ID: uuid.New(), Status: domain.ImportQueued},
		{ID: uuid.New(), Status: domain.ImportCompleted},
	}

	mockRepo.On("List", ctx, (*uuid.UUID)(nil), 1, 10).Return(jobs, int64(2), nil)

	result, total, err := mockRepo.List(ctx, nil, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(2), total)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_List_WithOrgID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	orgID := uuid.New()
	jobs := []*domain.ImportJob{
		{ID: uuid.New(), OrgID: &orgID},
	}

	mockRepo.On("List", ctx, &orgID, 1, 10).Return(jobs, int64(1), nil)

	result, _, err := mockRepo.List(ctx, &orgID, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_FindQueued(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	jobs := []*domain.ImportJob{
		{ID: uuid.New(), Status: domain.ImportQueued},
	}

	mockRepo.On("FindQueued", ctx, 5).Return(jobs, nil)

	result, err := mockRepo.FindQueued(ctx, 5)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_FindStuckProcessing(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	timeout := 5 * time.Minute
	stuckJobs := []*domain.ImportJob{
		{ID: uuid.New(), Status: domain.ImportProcessing},
	}

	mockRepo.On("FindStuckProcessing", ctx, timeout).Return(stuckJobs, nil)

	result, err := mockRepo.FindStuckProcessing(ctx, timeout)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_UpdateProcessingStartedAt(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()
	now := time.Now()

	mockRepo.On("UpdateProcessingStartedAt", ctx, id, now).Return(nil)

	err := mockRepo.UpdateProcessingStartedAt(ctx, id, now)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportJobRepository_UpdateProcessingStartedAt_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportJobRepository)

	id := uuid.New()
	now := time.Now()

	mockRepo.On("UpdateProcessingStartedAt", ctx, id, now).Return(errors.ErrNotFound)

	err := mockRepo.UpdateProcessingStartedAt(ctx, id, now)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}