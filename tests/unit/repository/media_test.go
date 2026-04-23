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

// MockMediaRepository is a mock implementation of repository.MediaRepository
type MockMediaRepository struct {
	mock.Mock
}

// Ensure MockMediaRepository implements the interface
var _ repository.MediaRepository = (*MockMediaRepository)(nil)

func (m *MockMediaRepository) Create(ctx context.Context, media *domain.Media) error {
	args := m.Called(ctx, media)
	return args.Error(0)
}

func (m *MockMediaRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Media, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Media), args.Error(1)
}

func (m *MockMediaRepository) FindByModelTypeAndID(ctx context.Context, modelType string, modelID uuid.UUID, collection string) ([]*domain.Media, error) {
	args := m.Called(ctx, modelType, modelID, collection)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Media), args.Error(1)
}

func (m *MockMediaRepository) FindByFilename(ctx context.Context, filename string) (*domain.Media, error) {
	args := m.Called(ctx, filename)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Media), args.Error(1)
}

func (m *MockMediaRepository) Update(ctx context.Context, media *domain.Media) error {
	args := m.Called(ctx, media)
	return args.Error(0)
}

func (m *MockMediaRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMediaRepository) MarkOrphaned(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMediaRepository) FindOrphaned(ctx context.Context, cutoff time.Time) ([]*domain.Media, error) {
	args := m.Called(ctx, cutoff)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Media), args.Error(1)
}

func (m *MockMediaRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMediaRepository) CreateConversion(ctx context.Context, conversion *domain.MediaConversion) error {
	args := m.Called(ctx, conversion)
	return args.Error(0)
}

func (m *MockMediaRepository) FindConversionsByMediaID(ctx context.Context, mediaID uuid.UUID) ([]*domain.MediaConversion, error) {
	args := m.Called(ctx, mediaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MediaConversion), args.Error(1)
}

func (m *MockMediaRepository) DeleteConversion(ctx context.Context, mediaID uuid.UUID, name string) error {
	args := m.Called(ctx, mediaID, name)
	return args.Error(0)
}

func (m *MockMediaRepository) CreateDownload(ctx context.Context, download *domain.MediaDownload) error {
	args := m.Called(ctx, download)
	return args.Error(0)
}

func (m *MockMediaRepository) CountDownloads(ctx context.Context, mediaID uuid.UUID) (int64, error) {
	args := m.Called(ctx, mediaID)
	return args.Get(0).(int64), args.Error(1)
}

func TestMockMediaRepository_Create(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	userID := uuid.New()
	media := &domain.Media{
		ModelType:        "news",
		ModelID:          uuid.New(),
		CollectionName:   "images",
		Filename:         "test-file.jpg",
		OriginalFilename: "original.jpg",
		MimeType:         "image/jpeg",
		Size:             1024,
		Path:             "news/uuid/test-file.jpg",
		UploadedByID:     userID,
	}

	mockRepo.On("Create", ctx, media).Return(nil)

	err := mockRepo.Create(ctx, media)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	id := uuid.New()
	expectedMedia := &domain.Media{
		ID:               id,
		ModelType:        "news",
		ModelID:          uuid.New(),
		CollectionName:   "images",
		Filename:         "test-file.jpg",
		OriginalFilename: "original.jpg",
		MimeType:         "image/jpeg",
		Size:             1024,
		Path:             "news/uuid/test-file.jpg",
		UploadedByID:     uuid.New(),
	}

	mockRepo.On("FindByID", ctx, id).Return(expectedMedia, nil)

	media, err := mockRepo.FindByID(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, expectedMedia.ID, media.ID)
	assert.Equal(t, expectedMedia.Filename, media.Filename)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	id := uuid.New()

	mockRepo.On("FindByID", ctx, id).Return(nil, errors.ErrNotFound)

	media, err := mockRepo.FindByID(ctx, id)
	assert.Error(t, err)
	assert.Nil(t, media)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_FindByModelTypeAndID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	modelType := "news"
	modelID := uuid.New()
	collection := "images"

	expectedMedia := []*domain.Media{
		{
			ID:               uuid.New(),
			ModelType:        modelType,
			ModelID:          modelID,
			CollectionName:   collection,
			Filename:         "file1.jpg",
			OriginalFilename: "original1.jpg",
			MimeType:         "image/jpeg",
			Size:             1024,
			Path:             "news/model/file1.jpg",
			UploadedByID:     uuid.New(),
		},
		{
			ID:               uuid.New(),
			ModelType:        modelType,
			ModelID:          modelID,
			CollectionName:   collection,
			Filename:         "file2.jpg",
			OriginalFilename: "original2.jpg",
			MimeType:         "image/jpeg",
			Size:             2048,
			Path:             "news/model/file2.jpg",
			UploadedByID:     uuid.New(),
		},
	}

	mockRepo.On("FindByModelTypeAndID", ctx, modelType, modelID, collection).Return(expectedMedia, nil)

	media, err := mockRepo.FindByModelTypeAndID(ctx, modelType, modelID, collection)
	assert.NoError(t, err)
	assert.Len(t, media, 2)
	assert.Equal(t, modelType, media[0].ModelType)
	assert.Equal(t, modelID, media[0].ModelID)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_FindByFilename(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	filename := "test-file-uuid.jpg"
	expectedMedia := &domain.Media{
		ID:               uuid.New(),
		ModelType:        "news",
		ModelID:          uuid.New(),
		Filename:         filename,
		OriginalFilename: "original.jpg",
		MimeType:         "image/jpeg",
		Size:             1024,
		Path:             "news/uuid/test-file.jpg",
		UploadedByID:     uuid.New(),
	}

	mockRepo.On("FindByFilename", ctx, filename).Return(expectedMedia, nil)

	media, err := mockRepo.FindByFilename(ctx, filename)
	assert.NoError(t, err)
	assert.Equal(t, filename, media.Filename)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_Update(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	media := &domain.Media{
		ID:               uuid.New(),
		ModelType:        "news",
		ModelID:          uuid.New(),
		CollectionName:   "images",
		Filename:         "updated-file.jpg",
		OriginalFilename: "original.jpg",
		MimeType:         "image/jpeg",
		Size:             2048,
		Path:             "news/uuid/updated-file.jpg",
		UploadedByID:     uuid.New(),
	}

	mockRepo.On("Update", ctx, media).Return(nil)

	err := mockRepo.Update(ctx, media)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_SoftDelete(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	id := uuid.New()

	mockRepo.On("SoftDelete", ctx, id).Return(nil)

	err := mockRepo.SoftDelete(ctx, id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_MarkOrphaned(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	id := uuid.New()

	mockRepo.On("MarkOrphaned", ctx, id).Return(nil)

	err := mockRepo.MarkOrphaned(ctx, id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_FindOrphaned(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	cutoff := time.Now().AddDate(0, 0, -30)
	orphanedTime := time.Now().AddDate(0, 0, -35)

	expectedMedia := []*domain.Media{
		{
			ID:               uuid.New(),
			ModelType:        "news",
			ModelID:          uuid.New(),
			Filename:         "orphaned1.jpg",
			OrphanedAt:       &orphanedTime,
			UploadedByID:     uuid.New(),
		},
		{
			ID:               uuid.New(),
			ModelType:        "invoice",
			ModelID:          uuid.New(),
			Filename:         "orphaned2.pdf",
			OrphanedAt:       &orphanedTime,
			UploadedByID:     uuid.New(),
		},
	}

	mockRepo.On("FindOrphaned", ctx, cutoff).Return(expectedMedia, nil)

	media, err := mockRepo.FindOrphaned(ctx, cutoff)
	assert.NoError(t, err)
	assert.Len(t, media, 2)
	assert.True(t, media[0].IsOrphaned())
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_HardDelete(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	id := uuid.New()

	mockRepo.On("HardDelete", ctx, id).Return(nil)

	err := mockRepo.HardDelete(ctx, id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_CreateConversion(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	mediaID := uuid.New()
	conversion := &domain.MediaConversion{
		MediaID:  mediaID,
		Name:     "thumbnail",
		Disk:     "local",
		Path:     "news/model/thumbnail.jpg",
		Size:     512,
	}

	mockRepo.On("CreateConversion", ctx, conversion).Return(nil)

	err := mockRepo.CreateConversion(ctx, conversion)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_FindConversionsByMediaID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	mediaID := uuid.New()
	expectedConversions := []*domain.MediaConversion{
		{
			ID:      uuid.New(),
			MediaID: mediaID,
			Name:    "thumbnail",
			Path:    "news/model/thumbnail.jpg",
			Size:    512,
		},
		{
			ID:      uuid.New(),
			MediaID: mediaID,
			Name:    "preview",
			Path:    "news/model/preview.jpg",
			Size:    1024,
		},
	}

	mockRepo.On("FindConversionsByMediaID", ctx, mediaID).Return(expectedConversions, nil)

	conversions, err := mockRepo.FindConversionsByMediaID(ctx, mediaID)
	assert.NoError(t, err)
	assert.Len(t, conversions, 2)
	assert.Equal(t, "thumbnail", conversions[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_DeleteConversion(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	mediaID := uuid.New()
	name := "thumbnail"

	mockRepo.On("DeleteConversion", ctx, mediaID, name).Return(nil)

	err := mockRepo.DeleteConversion(ctx, mediaID, name)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_CreateDownload(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	mediaID := uuid.New()
	userID := uuid.New()
	ipAddress := "192.168.1.1"
	userAgent := "Mozilla/5.0"

	download := &domain.MediaDownload{
		MediaID:        mediaID,
		DownloadedByID: &userID,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
	}

	mockRepo.On("CreateDownload", ctx, download).Return(nil)

	err := mockRepo.CreateDownload(ctx, download)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockMediaRepository_CountDownloads(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockMediaRepository)

	mediaID := uuid.New()
	expectedCount := int64(42)

	mockRepo.On("CountDownloads", ctx, mediaID).Return(expectedCount, nil)

	count, err := mockRepo.CountDownloads(ctx, mediaID)
	assert.NoError(t, err)
	assert.Equal(t, expectedCount, count)
	mockRepo.AssertExpectations(t)
}

// Domain Entity Tests

func TestMedia_IsImage(t *testing.T) {
	media := &domain.Media{MimeType: "image/jpeg"}
	assert.True(t, media.IsImage())

	media.MimeType = "image/png"
	assert.True(t, media.IsImage())

	media.MimeType = "application/pdf"
	assert.False(t, media.IsImage())
}

func TestMedia_IsVideo(t *testing.T) {
	media := &domain.Media{MimeType: "video/mp4"}
	assert.True(t, media.IsVideo())

	media.MimeType = "image/jpeg"
	assert.False(t, media.IsVideo())
}

func TestMedia_IsAudio(t *testing.T) {
	media := &domain.Media{MimeType: "audio/mpeg"}
	assert.True(t, media.IsAudio())

	media.MimeType = "image/jpeg"
	assert.False(t, media.IsAudio())
}

func TestMedia_IsDocument(t *testing.T) {
	media := &domain.Media{MimeType: "application/pdf"}
	assert.True(t, media.IsDocument())

	media.MimeType = "image/jpeg"
	assert.False(t, media.IsDocument())
}

func TestMedia_IsOrphaned(t *testing.T) {
	media := &domain.Media{}
	assert.False(t, media.IsOrphaned())

	now := time.Now()
	media.OrphanedAt = &now
	assert.True(t, media.IsOrphaned())
}

func TestMedia_MarkOrphaned(t *testing.T) {
	media := &domain.Media{}
	assert.False(t, media.IsOrphaned())

	media.MarkOrphaned()
	assert.True(t, media.IsOrphaned())
	assert.NotNil(t, media.OrphanedAt)
}

func TestMedia_GetImageDimensions(t *testing.T) {
	media := &domain.Media{
		Metadata: map[string]interface{}{
			"width":  1920.0,
			"height": 1080.0,
		},
	}

	width, height := media.GetImageDimensions()
	assert.Equal(t, 1920, width)
	assert.Equal(t, 1080, height)
}

func TestMedia_GetImageDimensions_NoMetadata(t *testing.T) {
	media := &domain.Media{}

	width, height := media.GetImageDimensions()
	assert.Equal(t, 0, width)
	assert.Equal(t, 0, height)
}

func TestMedia_ToResponse(t *testing.T) {
	id := uuid.New()
	media := &domain.Media{
		ID:               id,
		ModelType:        "news",
		CollectionName:   "images",
		OriginalFilename: "original.jpg",
		MimeType:         "image/jpeg",
		Size:             1024,
		CustomProperties: map[string]interface{}{
			"alt_text": "Test image",
		},
		Conversions: []*domain.MediaConversion{
			{
				Name: "thumbnail",
				Size: 512,
			},
		},
	}

	response := media.ToResponse()
	assert.Equal(t, id, response.ID)
	assert.Equal(t, "news", response.ModelType)
	assert.Equal(t, "images", response.CollectionName)
	assert.Equal(t, "original.jpg", response.Filename)
	assert.Equal(t, "image/jpeg", response.MimeType)
	assert.Equal(t, int64(1024), response.Size)
	assert.Len(t, response.Conversions, 1)
	assert.Equal(t, "thumbnail", response.Conversions[0].Name)
}

func TestMediaDisk_IsValid(t *testing.T) {
	assert.True(t, domain.IsValidMediaDisk(domain.MediaDiskLocal))
	assert.True(t, domain.IsValidMediaDisk(domain.MediaDiskS3))
	assert.True(t, domain.IsValidMediaDisk(domain.MediaDiskMinIO))
	assert.False(t, domain.IsValidMediaDisk("invalid"))
}
