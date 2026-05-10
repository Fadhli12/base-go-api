package unit

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/internal/storage"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testSigningSecret = "test-signing-secret-min-32-chars-long!"

// MockStorageDriverForVersions is a storage driver mock for version tests
type MockStorageDriverForVersions struct {
	mock.Mock
}

var _ storage.Driver = (*MockStorageDriverForVersions)(nil)

func (m *MockStorageDriverForVersions) Store(ctx context.Context, path string, content io.Reader) error {
	args := m.Called(ctx, path, content)
	return args.Error(0)
}

func (m *MockStorageDriverForVersions) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorageDriverForVersions) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockStorageDriverForVersions) URL(ctx context.Context, path string, expires time.Duration) (string, error) {
	args := m.Called(ctx, path, expires)
	return args.String(0), args.Error(1)
}

func (m *MockStorageDriverForVersions) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorageDriverForVersions) Size(ctx context.Context, path string) (int64, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(int64), args.Error(1)
}

// MockMediaRepository for version service tests
type MockMediaRepository struct {
	mock.Mock
}

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

// MockMediaVersionRepository for version service tests
type MockMediaVersionRepository struct {
	mock.Mock
}

func (m *MockMediaVersionRepository) Create(ctx context.Context, version *domain.MediaVersion) error {
	args := m.Called(ctx, version)
	return args.Error(0)
}

func (m *MockMediaVersionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.MediaVersion, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MediaVersion), args.Error(1)
}

func (m *MockMediaVersionRepository) FindByMediaIDAndVersion(ctx context.Context, mediaID uuid.UUID, version int) (*domain.MediaVersion, error) {
	args := m.Called(ctx, mediaID, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MediaVersion), args.Error(1)
}

func (m *MockMediaVersionRepository) FindByMediaID(ctx context.Context, mediaID uuid.UUID, limit, offset int) ([]*domain.MediaVersion, int64, error) {
	args := m.Called(ctx, mediaID, limit, offset)
	return args.Get(0).([]*domain.MediaVersion), args.Get(1).(int64), args.Error(2)
}

func (m *MockMediaVersionRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMediaVersionRepository) CountByMediaID(ctx context.Context, mediaID uuid.UUID) (int64, error) {
	args := m.Called(ctx, mediaID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMediaVersionRepository) FindCurrentVersion(ctx context.Context, mediaID uuid.UUID, currentVersion int) (*domain.MediaVersion, error) {
	args := m.Called(ctx, mediaID, currentVersion)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MediaVersion), args.Error(1)
}

func (m *MockMediaVersionRepository) FindByChecksum(ctx context.Context, mediaID uuid.UUID, checksum string) (*domain.MediaVersion, error) {
	args := m.Called(ctx, mediaID, checksum)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MediaVersion), args.Error(1)
}

func (m *MockMediaVersionRepository) UpdateCurrentVersion(ctx context.Context, mediaID uuid.UUID, oldVersion, newVersion int) error {
	args := m.Called(ctx, mediaID, oldVersion, newVersion)
	return args.Error(0)
}

func TestComputeSHA256Checksum(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "hello world",
			input:    "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ComputeSHA256Checksum(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestComputeSHA256Checksum_LargeData(t *testing.T) {
	// Test with data larger than buffer size
	data := strings.Repeat("a", 1024*1024) // 1MB of 'a'
	result, err := service.ComputeSHA256Checksum(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 64 {
		t.Errorf("expected 64-char hex string, got %d chars", len(result))
	}
}

func TestListVersions_Success(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
	}
	v1 := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 1,
	}
	versions := []*domain.MediaVersion{v2, v1}
	mockVersionRepo.On("FindByMediaID", ctx, mediaID, 20, 0).Return(versions, int64(2), nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.ListVersions(ctx, mediaID, 20, 0)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, mediaID, result.MediaID)
	assert.Equal(t, 2, result.CurrentVersion)
	assert.Equal(t, int64(2), result.Total)
	assert.Len(t, result.Versions, 2)
	assert.Equal(t, 2, result.Versions[0].Version)
	assert.True(t, result.Versions[0].IsCurrent)
	assert.Equal(t, 1, result.Versions[1].Version)
	assert.False(t, result.Versions[1].IsCurrent)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestListVersions_MediaNotFound(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	mockMediaRepo.On("FindByID", ctx, mediaID).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.ListVersions(ctx, mediaID, 20, 0)

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
}

func TestListVersions_Pagination(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v3 := &domain.MediaVersion{ID: uuid.New(), MediaID: mediaID, Version: 3}
	versions := []*domain.MediaVersion{v3}
	mockVersionRepo.On("FindByMediaID", ctx, mediaID, 1, 1).Return(versions, int64(3), nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.ListVersions(ctx, mediaID, 1, 1)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Versions, 1)
	assert.Equal(t, int64(3), result.Total)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestListVersions_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{ID: mediaID, CurrentVersion: 1}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)
	mockVersionRepo.On("FindByMediaID", ctx, mediaID, 20, 0).Return([]*domain.MediaVersion{}, int64(0), nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.ListVersions(ctx, mediaID, 0, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Empty(t, result.Versions)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestGetVersion_Success(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()
	versionID := uuid.New()
	uploadedByID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:           versionID,
		MediaID:      mediaID,
		Version:      2,
		Filename:     "file.pdf",
		MimeType:     "application/pdf",
		Size:         1024,
		Checksum:     "abc123",
		UploadedByID: uploadedByID,
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(v2, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.GetVersion(ctx, mediaID, 2)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, versionID, result.ID)
	assert.Equal(t, mediaID, result.MediaID)
	assert.Equal(t, 2, result.Version)
	assert.True(t, result.IsCurrent)
	assert.Equal(t, "abc123", result.Checksum)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestGetVersion_NotFound(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 1,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 5).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.GetVersion(ctx, mediaID, 5)

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestGetVersion_SoftDeleted(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	deletedVersion := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
	}
	deletedVersion.DeletedAt.Valid = true
	deletedVersion.DeletedAt.Time = time.Now()
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(deletedVersion, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.GetVersion(ctx, mediaID, 2)

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "GONE", appErr.Code)
	assert.Equal(t, 410, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestGetVersion_MediaNotFound(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	mockMediaRepo.On("FindByID", ctx, mediaID).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.GetVersion(ctx, mediaID, 1)

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
}

func TestGetVersion_InvalidVersionNumber(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.GetVersion(ctx, mediaID, 0)

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestDownloadVersion_Success(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()
	versionID := uuid.New()
	uploadedByID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID: mediaID,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:               versionID,
		MediaID:          mediaID,
		Version:          2,
		Filename:         "file.pdf",
		OriginalFilename: "original-file.pdf",
		MimeType:         "application/pdf",
		Size:             1024,
		FilePath:         "/test/versioned-file.pdf",
		Checksum:         "abc123",
		UploadedByID:     uploadedByID,
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(v2, nil)

	fileContent := io.NopCloser(strings.NewReader("test file content"))
	mockStorage.On("Get", ctx, "/test/versioned-file.pdf").Return(fileContent, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	testUserID := uuid.New()
	reader, result, err := svc.DownloadVersion(ctx, testUserID, mediaID, 2, "127.0.0.1", "test-agent")

	assert.NoError(t, err)
	assert.NotNil(t, reader)
	assert.NotNil(t, result)
	assert.Equal(t, versionID, result.ID)
	assert.Equal(t, 2, result.Version)

	reader.Close()
	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestDownloadVersion_MediaNotFound(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	mockMediaRepo.On("FindByID", ctx, mediaID).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	reader, result, err := svc.DownloadVersion(ctx, uuid.Nil, mediaID, 1, "", "")

	assert.Nil(t, reader)
	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
}

func TestDownloadVersion_VersionNotFound(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{ID: mediaID}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 5).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	reader, result, err := svc.DownloadVersion(ctx, uuid.Nil, mediaID, 5, "", "")

	assert.Nil(t, reader)
	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestDownloadVersion_SoftDeleted(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{ID: mediaID}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	deletedVersion := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
	}
	deletedVersion.DeletedAt.Valid = true
	deletedVersion.DeletedAt.Time = time.Now()
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(deletedVersion, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	reader, result, err := svc.DownloadVersion(ctx, uuid.Nil, mediaID, 2, "", "")

	assert.Nil(t, reader)
	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "GONE", appErr.Code)
	assert.Equal(t, 410, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestDownloadVersion_InvalidVersionNumber(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	reader, result, err := svc.DownloadVersion(ctx, uuid.Nil, mediaID, 0, "", "")

	assert.Nil(t, reader)
	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestGetVersionSignedURL_Success(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{ID: mediaID}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
		FilePath: "/test/versioned-file.pdf",
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(v2, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	url, expiresAt, err := svc.GetVersionSignedURL(ctx, mediaID, 2, 3600)

	assert.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.False(t, expiresAt.IsZero())
	assert.True(t, expiresAt.After(time.Now()))

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestGetVersionSignedURL_DefaultExpiry(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{ID: mediaID}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
		FilePath: "/test/versioned-file.pdf",
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(v2, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	url, expiresAt, err := svc.GetVersionSignedURL(ctx, mediaID, 2, 0)

	assert.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.False(t, expiresAt.IsZero())

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestGetVersionSignedURL_InvalidExpiresIn(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)

	t.Run("too short", func(t *testing.T) {
		url, _, err := svc.GetVersionSignedURL(ctx, mediaID, 1, 30)
		assert.Empty(t, url)
		assert.Error(t, err)
		appErr := apperrors.GetAppError(err)
		assert.NotNil(t, appErr)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	})

	t.Run("too long", func(t *testing.T) {
		url, _, err := svc.GetVersionSignedURL(ctx, mediaID, 1, 99999)
		assert.Empty(t, url)
		assert.Error(t, err)
		appErr := apperrors.GetAppError(err)
		assert.NotNil(t, appErr)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	})
}

func TestGetVersionSignedURL_VersionNotFound(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{ID: mediaID}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 5).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	url, _, err := svc.GetVersionSignedURL(ctx, mediaID, 5, 3600)

	assert.Empty(t, url)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestGetVersionSignedURL_SoftDeleted(t *testing.T) {
	ctx := context.Background()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{ID: mediaID}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	deletedVersion := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
	}
	deletedVersion.DeletedAt.Valid = true
	deletedVersion.DeletedAt.Time = time.Now()
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(deletedVersion, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	url, _, err := svc.GetVersionSignedURL(ctx, mediaID, 2, 3600)

	assert.Empty(t, url)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "GONE", appErr.Code)
	assert.Equal(t, 410, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestRestoreVersion_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()
	versionID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v1 := &domain.MediaVersion{
		ID:      versionID,
		MediaID: mediaID,
		Version: 1,
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 1).Return(v1, nil)
	mockVersionRepo.On("UpdateCurrentVersion", ctx, mediaID, 3, 1).Return(nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.RestoreVersion(ctx, userID, mediaID, 1, "127.0.0.1", "test-agent")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.CurrentVersion)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestRestoreVersion_AlreadyCurrent(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(v2, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.RestoreVersion(ctx, userID, mediaID, 2, "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	assert.Equal(t, 400, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestRestoreVersion_NotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 5).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.RestoreVersion(ctx, userID, mediaID, 5, "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestRestoreVersion_SoftDeleted(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	deletedVersion := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 2,
	}
	deletedVersion.DeletedAt.Valid = true
	deletedVersion.DeletedAt.Time = time.Now()
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(deletedVersion, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.RestoreVersion(ctx, userID, mediaID, 2, "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "GONE", appErr.Code)
	assert.Equal(t, 410, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestRestoreVersion_MediaNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	mockMediaRepo.On("FindByID", ctx, mediaID).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.RestoreVersion(ctx, userID, mediaID, 1, "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
}

func TestRestoreVersion_ConcurrentConflict(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v1 := &domain.MediaVersion{
		ID:      uuid.New(),
		MediaID: mediaID,
		Version: 1,
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 1).Return(v1, nil)
	conflictErr := apperrors.NewAppError("CONFLICT", "Concurrent modification detected", 409)
	mockVersionRepo.On("UpdateCurrentVersion", ctx, mediaID, 3, 1).Return(conflictErr)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, nil, testSigningSecret, nil, nil)
	result, err := svc.RestoreVersion(ctx, userID, mediaID, 1, "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "CONFLICT", appErr.Code)
	assert.Equal(t, 409, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestDeleteVersion_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()
	versionID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:       versionID,
		MediaID:  mediaID,
		Version:  2,
		Filename: "file.pdf",
		MimeType: "application/pdf",
		Size:     1024,
		FilePath: "/test/versioned-file.pdf",
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(v2, nil)
	mockVersionRepo.On("SoftDelete", ctx, versionID).Return(nil)
	mockStorage.On("Delete", ctx, "/test/versioned-file.pdf").Return(nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	err := svc.DeleteVersion(ctx, userID, mediaID, 2, "127.0.0.1", "test-agent")

	assert.NoError(t, err)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestDeleteVersion_CurrentVersion(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	err := svc.DeleteVersion(ctx, userID, mediaID, 2, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	assert.Equal(t, 400, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
}

func TestDeleteVersion_NotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 5).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	err := svc.DeleteVersion(ctx, userID, mediaID, 5, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
	assert.Equal(t, 404, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestDeleteVersion_AlreadyDeleted(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	deletedVersion := &domain.MediaVersion{
		ID:       uuid.New(),
		MediaID:  mediaID,
		Version:  2,
		FilePath: "/test/versioned-file.pdf",
	}
	deletedVersion.DeletedAt.Valid = true
	deletedVersion.DeletedAt.Time = time.Now()
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(deletedVersion, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	err := svc.DeleteVersion(ctx, userID, mediaID, 2, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "CONFLICT", appErr.Code)
	assert.Equal(t, 409, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
}

func TestDeleteVersion_MediaNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	mockMediaRepo.On("FindByID", ctx, mediaID).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	err := svc.DeleteVersion(ctx, userID, mediaID, 1, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
	assert.Equal(t, 404, appErr.HTTPStatus)

	mockMediaRepo.AssertExpectations(t)
}

func TestDeleteVersion_StorageFailure(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()
	versionID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 3,
	}
	mockMediaRepo.On("FindByID", ctx, mediaID).Return(media, nil)

	v2 := &domain.MediaVersion{
		ID:       versionID,
		MediaID:  mediaID,
		Version:  2,
		Filename: "file.pdf",
		MimeType: "application/pdf",
		Size:     1024,
		FilePath: "/test/versioned-file.pdf",
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", ctx, mediaID, 2).Return(v2, nil)
	storageErr := apperrors.NewAppError("STORAGE_ERROR", "File not found in storage", 500)
	mockStorage.On("Delete", ctx, "/test/versioned-file.pdf").Return(storageErr)
	mockVersionRepo.On("SoftDelete", ctx, versionID).Return(nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	err := svc.DeleteVersion(ctx, userID, mediaID, 2, "127.0.0.1", "test-agent")

	assert.NoError(t, err)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestUploadVersion_ConcurrentConflict(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
		MimeType:       "image/png",
		Path:           "/original/path.png",
	}
	mockMediaRepo.On("FindByID", mock.Anything, mediaID).Return(media, nil)
	mockVersionRepo.On("CountByMediaID", mock.Anything, mediaID).Return(int64(1), nil)

	mockVersionRepo.On("FindByMediaIDAndVersion", mock.Anything, mediaID, 2).Return(nil, apperrors.ErrNotFound)
	mockVersionRepo.On("FindByChecksum", mock.Anything, mediaID, mock.AnythingOfType("string")).Return(nil, apperrors.ErrNotFound)

	versionID := uuid.New()
	mockVersionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.MediaVersion")).Run(func(args mock.Arguments) {
		v := args.Get(1).(*domain.MediaVersion)
		v.ID = versionID
	}).Return(nil)

	mockStorage.On("Store", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

	conflictErr := apperrors.NewAppError("CONFLICT", "Concurrent modification detected", 409)
	mockVersionRepo.On("UpdateCurrentVersion", mock.Anything, mediaID, 2, 3).Return(conflictErr)

	mockVersionRepo.On("SoftDelete", mock.Anything, versionID).Return(nil)
	mockStorage.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	reader := strings.NewReader("\x89PNG\r\n\x1a\ntest file content")
	result, err := svc.UploadVersion(ctx, userID, mediaID, reader, "test.png", 19, "image/png", "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "CONFLICT", appErr.Code)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestUploadVersion_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
		MimeType:       "image/png",
		Path:           "/original/path.png",
	}
	mockMediaRepo.On("FindByID", mock.Anything, mediaID).Return(media, nil)
	mockVersionRepo.On("CountByMediaID", mock.Anything, mediaID).Return(int64(1), nil)

	mockVersionRepo.On("FindByMediaIDAndVersion", mock.Anything, mediaID, 2).Return(nil, apperrors.ErrNotFound)
	mockVersionRepo.On("FindByChecksum", mock.Anything, mediaID, mock.AnythingOfType("string")).Return(nil, apperrors.ErrNotFound)

	versionID := uuid.New()
	mockVersionRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.MediaVersion")).Run(func(args mock.Arguments) {
		v := args.Get(1).(*domain.MediaVersion)
		v.ID = versionID
	}).Return(nil)

	mockStorage.On("Store", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)
	mockVersionRepo.On("UpdateCurrentVersion", mock.Anything, mediaID, 2, 3).Return(nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	reader := strings.NewReader("\x89PNG\r\n\x1a\ntest file content")
	result, err := svc.UploadVersion(ctx, userID, mediaID, reader, "test.png", 19, "image/png", "127.0.0.1", "test-agent")

	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, result.Version)

	mockMediaRepo.AssertExpectations(t)
	mockVersionRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestUploadVersion_MediaNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	mockMediaRepo.On("FindByID", mock.Anything, mediaID).Return(nil, apperrors.ErrNotFound)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	reader := strings.NewReader("\x89PNG\r\n\x1a\ntest file content")
	result, err := svc.UploadVersion(ctx, userID, mediaID, reader, "test.png", 19, "image/png", "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
}

func TestUploadVersion_MIMEMismatch(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
		MimeType:       "image/png",
		Path:           "/original/path.png",
	}
	mockMediaRepo.On("FindByID", mock.Anything, mediaID).Return(media, nil)
	mockVersionRepo.On("CountByMediaID", mock.Anything, mediaID).Return(int64(1), nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	reader := strings.NewReader("%PDF-1.4 test content")
	result, err := svc.UploadVersion(ctx, userID, mediaID, reader, "test.pdf", 20, "application/pdf", "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestUploadVersion_DuplicateChecksum(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
		MimeType:       "image/png",
		Path:           "/original/path.png",
	}
	mockMediaRepo.On("FindByID", mock.Anything, mediaID).Return(media, nil)
	mockVersionRepo.On("CountByMediaID", mock.Anything, mediaID).Return(int64(1), nil)

	mockVersionRepo.On("FindByMediaIDAndVersion", mock.Anything, mediaID, 2).Return(nil, apperrors.ErrNotFound)

	existingVersion := &domain.MediaVersion{
		ID:       uuid.New(),
		MediaID:  mediaID,
		Version:  1,
		Checksum: "abc123duplicate",
	}
	mockVersionRepo.On("FindByChecksum", mock.Anything, mediaID, mock.AnythingOfType("string")).Return(existingVersion, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	reader := strings.NewReader("\x89PNG\r\n\x1a\ntest file content")
	result, err := svc.UploadVersion(ctx, userID, mediaID, reader, "test.png", 19, "image/png", "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "CONFLICT", appErr.Code)
}

func TestUploadVersion_CurrentChecksumDuplicate(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	mediaID := uuid.New()

	mockMediaRepo := new(MockMediaRepository)
	mockVersionRepo := new(MockMediaVersionRepository)
	mockStorage := new(MockStorageDriverForVersions)

	media := &domain.Media{
		ID:             mediaID,
		CurrentVersion: 2,
		MimeType:       "image/png",
		Path:           "/original/path.png",
	}
	mockMediaRepo.On("FindByID", mock.Anything, mediaID).Return(media, nil)
	mockVersionRepo.On("CountByMediaID", mock.Anything, mediaID).Return(int64(1), nil)

	testContent := "\x89PNG\r\n\x1a\ntest content"

	checksum, err := service.ComputeSHA256Checksum(strings.NewReader(testContent))
	assert.NoError(t, err)

	currentVersion := &domain.MediaVersion{
		ID:       uuid.New(),
		MediaID:  mediaID,
		Version:  2,
		Checksum: checksum,
	}
	mockVersionRepo.On("FindByMediaIDAndVersion", mock.Anything, mediaID, 2).Return(currentVersion, nil)

	svc := service.NewVersionService(mockMediaRepo, mockVersionRepo, mockStorage, testSigningSecret, nil, nil)
	result, err := svc.UploadVersion(ctx, userID, mediaID, strings.NewReader(testContent), "test.png", int64(len(testContent)), "image/png", "127.0.0.1", "test-agent")

	assert.Nil(t, result)
	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, "CONFLICT", appErr.Code)
}
