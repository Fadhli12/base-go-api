package unit

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockExportJobRepository struct {
	mock.Mock
}

var _ repository.ExportJobRepository = (*MockExportJobRepository)(nil)

func (m *MockExportJobRepository) Create(ctx context.Context, job *domain.ExportJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockExportJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ExportJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ExportJob), args.Error(1)
}

func (m *MockExportJobRepository) FindByStatus(ctx context.Context, status string, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	args := m.Called(ctx, status, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.ExportJob), args.Get(1).(int64), args.Error(2)
}

func (m *MockExportJobRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	args := m.Called(ctx, orgID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.ExportJob), args.Get(1).(int64), args.Error(2)
}

func (m *MockExportJobRepository) FindByCreatedBy(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.ExportJob), args.Get(1).(int64), args.Error(2)
}

func (m *MockExportJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *MockExportJobRepository) ClaimJob(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error) {
	args := m.Called(ctx, id, fromStatus, toStatus)
	return args.Bool(0), args.Error(1)
}

func (m *MockExportJobRepository) UpdateFilePath(ctx context.Context, id uuid.UUID, filePath string, recordCount int, hmacSignature string) error {
	args := m.Called(ctx, id, filePath, recordCount, hmacSignature)
	return args.Error(0)
}

func (m *MockExportJobRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockExportJobRepository) List(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	args := m.Called(ctx, orgID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.ExportJob), args.Get(1).(int64), args.Error(2)
}

func (m *MockExportJobRepository) FindExpired(ctx context.Context) ([]*domain.ExportJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ExportJob), args.Error(1)
}

func (m *MockExportJobRepository) ClearFileRefs(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestMockExportJobRepository_Create(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	job := &domain.ExportJob{
		EntityTypes: []string{"organizations", "users"},
		Format:      "json",
		CreatedBy:   uuid.New(),
	}

	mockRepo.On("Create", ctx, job).Return(nil)

	err := mockRepo.Create(ctx, job)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()
	expectedJob := &domain.ExportJob{
		ID:          id,
		Status:      domain.ExportCompleted,
		EntityTypes: []string{"organizations"},
		Format:      "json",
		CreatedBy:   uuid.New(),
	}

	mockRepo.On("FindByID", ctx, id).Return(expectedJob, nil)

	job, err := mockRepo.FindByID(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, expectedJob.ID, job.ID)
	assert.Equal(t, expectedJob.Status, job.Status)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()

	mockRepo.On("FindByID", ctx, id).Return(nil, errors.ErrNotFound)

	job, err := mockRepo.FindByID(ctx, id)
	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_FindByStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	jobs := []*domain.ExportJob{
		{ID: uuid.New(), Status: domain.ExportQueued},
		{ID: uuid.New(), Status: domain.ExportQueued},
	}

	mockRepo.On("FindByStatus", ctx, domain.ExportQueued, 1, 10).Return(jobs, int64(2), nil)

	result, count, err := mockRepo.FindByStatus(ctx, domain.ExportQueued, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(2), count)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_FindByOrgID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	orgID := uuid.New()
	jobs := []*domain.ExportJob{
		{ID: uuid.New(), OrgID: &orgID},
	}

	mockRepo.On("FindByOrgID", ctx, orgID, 1, 10).Return(jobs, int64(1), nil)

	result, total, err := mockRepo.FindByOrgID(ctx, orgID, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), total)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_FindByCreatedBy(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	userID := uuid.New()
	jobs := []*domain.ExportJob{
		{ID: uuid.New(), CreatedBy: userID},
	}

	mockRepo.On("FindByCreatedBy", ctx, userID, 1, 10).Return(jobs, int64(1), nil)

	result, _, err := mockRepo.FindByCreatedBy(ctx, userID, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()
	errMsg := "something failed"

	mockRepo.On("UpdateStatus", ctx, id, domain.ExportFailed, &errMsg).Return(nil)

	err := mockRepo.UpdateStatus(ctx, id, domain.ExportFailed, &errMsg)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_UpdateStatus_NullErrorMessage(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()

	mockRepo.On("UpdateStatus", ctx, id, domain.ExportProcessing, (*string)(nil)).Return(nil)

	err := mockRepo.UpdateStatus(ctx, id, domain.ExportProcessing, nil)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_UpdateFilePath(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()

	mockRepo.On("UpdateFilePath", ctx, id, "/tmp/export.json", 100, "sha256=abc123").Return(nil)

	err := mockRepo.UpdateFilePath(ctx, id, "/tmp/export.json", 100, "sha256=abc123")
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_SoftDelete(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()

	mockRepo.On("SoftDelete", ctx, id).Return(nil)

	err := mockRepo.SoftDelete(ctx, id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_SoftDelete_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()

	mockRepo.On("SoftDelete", ctx, id).Return(errors.ErrNotFound)

	err := mockRepo.SoftDelete(ctx, id)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_List(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	jobs := []*domain.ExportJob{
		{ID: uuid.New()},
		{ID: uuid.New()},
	}

	mockRepo.On("List", ctx, (*uuid.UUID)(nil), 1, 10).Return(jobs, int64(2), nil)

	result, count, err := mockRepo.List(ctx, nil, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(2), count)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_List_WithOrgID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	orgID := uuid.New()
	jobs := []*domain.ExportJob{
		{ID: uuid.New(), OrgID: &orgID},
	}

	mockRepo.On("List", ctx, &orgID, 1, 10).Return(jobs, int64(1), nil)

	result, _, err := mockRepo.List(ctx, &orgID, 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_FindExpired(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	jobs := []*domain.ExportJob{
		{ID: uuid.New(), Status: domain.ExportCompleted},
	}

	mockRepo.On("FindExpired", ctx).Return(jobs, nil)

	result, err := mockRepo.FindExpired(ctx)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_ClearFileRefs(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()

	mockRepo.On("ClearFileRefs", ctx, id).Return(nil)

	err := mockRepo.ClearFileRefs(ctx, id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockExportJobRepository_ClearFileRefs_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockExportJobRepository)

	id := uuid.New()

	mockRepo.On("ClearFileRefs", ctx, id).Return(errors.ErrNotFound)

	err := mockRepo.ClearFileRefs(ctx, id)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}