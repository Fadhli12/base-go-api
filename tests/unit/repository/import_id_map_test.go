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

type MockImportIDMapRepository struct {
	mock.Mock
}

var _ repository.ImportIDMapRepository = (*MockImportIDMapRepository)(nil)

func (m *MockImportIDMapRepository) Create(ctx context.Context, mapping *domain.ImportIDMap) error {
	args := m.Called(ctx, mapping)
	return args.Error(0)
}

func (m *MockImportIDMapRepository) FindByJobAndEntityType(ctx context.Context, jobID uuid.UUID, entityType string) ([]*domain.ImportIDMap, error) {
	args := m.Called(ctx, jobID, entityType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ImportIDMap), args.Error(1)
}

func (m *MockImportIDMapRepository) FindByExternalID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (*domain.ImportIDMap, error) {
	args := m.Called(ctx, jobID, entityType, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ImportIDMap), args.Error(1)
}

func (m *MockImportIDMapRepository) BatchCreate(ctx context.Context, mappings []*domain.ImportIDMap) error {
	args := m.Called(ctx, mappings)
	return args.Error(0)
}

func (m *MockImportIDMapRepository) ResolveUUID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, jobID, entityType, externalID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func TestMockImportIDMapRepository_Create(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()
	mapping := &domain.ImportIDMap{
		JobID:      jobID,
		EntityType: "organizations",
		ExternalID: uuid.New(),
		InternalID: uuid.New(),
	}

	mockRepo.On("Create", ctx, mapping).Return(nil)

	err := mockRepo.Create(ctx, mapping)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_FindByJobAndEntityType(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()
	mappings := []*domain.ImportIDMap{
		{ID: uuid.New(), JobID: jobID, EntityType: "organizations", ExternalID: uuid.New(), InternalID: uuid.New()},
		{ID: uuid.New(), JobID: jobID, EntityType: "organizations", ExternalID: uuid.New(), InternalID: uuid.New()},
	}

	mockRepo.On("FindByJobAndEntityType", ctx, jobID, "organizations").Return(mappings, nil)

	result, err := mockRepo.FindByJobAndEntityType(ctx, jobID, "organizations")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_FindByJobAndEntityType_Empty(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()

	mockRepo.On("FindByJobAndEntityType", ctx, jobID, "users").Return([]*domain.ImportIDMap{}, nil)

	result, err := mockRepo.FindByJobAndEntityType(ctx, jobID, "users")
	assert.NoError(t, err)
	assert.Len(t, result, 0)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_FindByExternalID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()
	externalID := uuid.New()
	internalID := uuid.New()

	expectedMapping := &domain.ImportIDMap{
		ID:         uuid.New(),
		JobID:      jobID,
		EntityType: "organizations",
		ExternalID: externalID,
		InternalID: internalID,
	}

	mockRepo.On("FindByExternalID", ctx, jobID, "organizations", externalID).Return(expectedMapping, nil)

	mapping, err := mockRepo.FindByExternalID(ctx, jobID, "organizations", externalID)
	assert.NoError(t, err)
	assert.Equal(t, externalID, mapping.ExternalID)
	assert.Equal(t, internalID, mapping.InternalID)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_FindByExternalID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()
	externalID := uuid.New()

	mockRepo.On("FindByExternalID", ctx, jobID, "organizations", externalID).Return(nil, errors.ErrNotFound)

	mapping, err := mockRepo.FindByExternalID(ctx, jobID, "organizations", externalID)
	assert.Error(t, err)
	assert.Nil(t, mapping)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_BatchCreate(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()
	mappings := []*domain.ImportIDMap{
		{ID: uuid.New(), JobID: jobID, EntityType: "organizations", ExternalID: uuid.New(), InternalID: uuid.New()},
		{ID: uuid.New(), JobID: jobID, EntityType: "users", ExternalID: uuid.New(), InternalID: uuid.New()},
	}

	mockRepo.On("BatchCreate", ctx, mappings).Return(nil)

	err := mockRepo.BatchCreate(ctx, mappings)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_BatchCreate_Empty(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	mockRepo.On("BatchCreate", ctx, []*domain.ImportIDMap{}).Return(nil)

	err := mockRepo.BatchCreate(ctx, []*domain.ImportIDMap{})
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_ResolveUUID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()
	externalID := uuid.New()
	internalID := uuid.New()

	mockRepo.On("ResolveUUID", ctx, jobID, "organizations", externalID).Return(internalID, nil)

	result, err := mockRepo.ResolveUUID(ctx, jobID, "organizations", externalID)
	assert.NoError(t, err)
	assert.Equal(t, internalID, result)
	mockRepo.AssertExpectations(t)
}

func TestMockImportIDMapRepository_ResolveUUID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockImportIDMapRepository)

	jobID := uuid.New()
	externalID := uuid.New()

	mockRepo.On("ResolveUUID", ctx, jobID, "organizations", externalID).Return(uuid.Nil, errors.ErrNotFound)

	result, err := mockRepo.ResolveUUID(ctx, jobID, "organizations", externalID)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, result)
	mockRepo.AssertExpectations(t)
}