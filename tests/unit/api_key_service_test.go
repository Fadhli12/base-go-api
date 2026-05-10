package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

type MockAPIKeyRepository struct {
	mock.Mock
}

func (m *MockAPIKeyRepository) Create(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) FindByIDWithUser(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) FindByKeyHashWithUser(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.APIKey, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]domain.APIKey), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIKeyRepository) Update(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) SoftDeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) UpdateLastUsedAt(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) CountActiveByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

type MockUserRepositoryForAPIKey struct {
	mock.Mock
}

func (m *MockUserRepositoryForAPIKey) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForAPIKey) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForAPIKey) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForAPIKey) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForAPIKey) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepositoryForAPIKey) FindAll(ctx context.Context) ([]domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.User), args.Error(1)
}

func newTestAuditServiceForAPIKey() (*MockAuditLogForAudit, *service.AuditService) {
	repo := new(MockAuditLogForAudit)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	svc := service.NewAuditService(repo, service.AuditServiceConfig{BufferSize: 100})
	return repo, svc
}

func TestAPIKeyService_Create(t *testing.T) {
	t.Run("creates API key successfully", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		user := &domain.User{ID: userID, Email: "test@example.com"}
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)
		apiKeyRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.APIKey")).Return(nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		req := request.CreateAPIKeyRequest{
			Name:   "test-key",
			Scopes: []string{"invoices:view"},
		}

		result, err := svc.Create(context.Background(), userID, req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.APIKey)
		assert.NotEmpty(t, result.Secret)
		assert.NotEmpty(t, result.KeyHash)
		assert.Contains(t, result.Secret, service.KeyPrefixLive)
	})

	t.Run("returns error when user not found", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		userRepo.On("FindByID", mock.Anything, userID).Return(nil, apperrors.ErrNotFound)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		req := request.CreateAPIKeyRequest{
			Name:   "test-key",
			Scopes: []string{"invoices:view"},
		}

		result, err := svc.Create(context.Background(), userID, req)
		assert.Nil(t, result)
		assert.Error(t, err)
		appErr, ok := err.(*apperrors.AppError)
		require.True(t, ok)
		assert.Equal(t, 404, appErr.HTTPStatus)
	})

	t.Run("returns error for past expiration date", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		user := &domain.User{ID: userID, Email: "test@example.com"}
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		pastExpiry := time.Now().Add(-1 * time.Hour)
		req := request.CreateAPIKeyRequest{
			Name:      "test-key",
			Scopes:    []string{"invoices:view"},
			ExpiresAt: &pastExpiry,
		}

		result, err := svc.Create(context.Background(), userID, req)
		assert.Nil(t, result)
		assert.Error(t, err)
		appErr, ok := err.(*apperrors.AppError)
		require.True(t, ok)
		assert.Equal(t, 422, appErr.HTTPStatus)
	})

	t.Run("returns error for invalid scope format", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		user := &domain.User{ID: userID, Email: "test@example.com"}
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		req := request.CreateAPIKeyRequest{
			Name:   "test-key",
			Scopes: []string{"invalidscope"},
		}

		result, err := svc.Create(context.Background(), userID, req)
		assert.Nil(t, result)
		assert.Error(t, err)
		appErr, ok := err.(*apperrors.AppError)
		require.True(t, ok)
		assert.Equal(t, 422, appErr.HTTPStatus)
	})

	t.Run("returns conflict error on duplicate", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		user := &domain.User{ID: userID, Email: "test@example.com"}
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)
		apiKeyRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.APIKey")).Return(apperrors.ErrConflict)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		req := request.CreateAPIKeyRequest{
			Name:   "test-key",
			Scopes: []string{"invoices:view"},
		}

		result, err := svc.Create(context.Background(), userID, req)
		assert.Nil(t, result)
		assert.Error(t, err)
		appErr, ok := err.(*apperrors.AppError)
		require.True(t, ok)
		assert.Equal(t, 409, appErr.HTTPStatus)
	})
}

func TestAPIKeyService_List(t *testing.T) {
	t.Run("lists API keys for user with defaults", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		keys := []domain.APIKey{{ID: uuid.New(), UserID: userID, Name: "key1"}}
		apiKeyRepo.On("FindByUserID", mock.Anything, userID, 20, 0).Return(keys, int64(1), nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		result, total, err := svc.List(context.Background(), userID, 0, 0)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, int64(1), total)
	})

	t.Run("clamps limit to max 100", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		apiKeyRepo.On("FindByUserID", mock.Anything, userID, 100, 0).Return([]domain.APIKey{}, int64(0), nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		result, _, err := svc.List(context.Background(), userID, 200, 0)
		require.NoError(t, err)
		assert.Len(t, result, 0)
	})
}

func TestAPIKeyService_GetByID(t *testing.T) {
	t.Run("returns API key owned by user", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		keyID := uuid.New()
		apiKey := &domain.APIKey{ID: keyID, UserID: userID, Name: "my-key"}
		apiKeyRepo.On("FindByID", mock.Anything, keyID).Return(apiKey, nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		result, err := svc.GetByID(context.Background(), userID, keyID)
		require.NoError(t, err)
		assert.Equal(t, keyID, result.ID)
	})

	t.Run("returns forbidden for wrong owner", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		otherUserID := uuid.New()
		keyID := uuid.New()
		apiKey := &domain.APIKey{ID: keyID, UserID: otherUserID, Name: "my-key"}
		apiKeyRepo.On("FindByID", mock.Anything, keyID).Return(apiKey, nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		result, err := svc.GetByID(context.Background(), userID, keyID)
		assert.Nil(t, result)
		assert.Error(t, err)
		appErr, ok := err.(*apperrors.AppError)
		require.True(t, ok)
		assert.Equal(t, 403, appErr.HTTPStatus)
	})

	t.Run("returns not found for missing key", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		keyID := uuid.New()
		apiKeyRepo.On("FindByID", mock.Anything, keyID).Return(nil, apperrors.ErrNotFound)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		result, err := svc.GetByID(context.Background(), uuid.New(), keyID)
		assert.Nil(t, result)
		assert.Error(t, err)
	})
}

func TestAPIKeyService_Revoke(t *testing.T) {
	t.Run("revokes active API key", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		keyID := uuid.New()
		apiKey := &domain.APIKey{ID: keyID, UserID: userID, Name: "my-key"}
		apiKeyRepo.On("FindByID", mock.Anything, keyID).Return(apiKey, nil)
		apiKeyRepo.On("Revoke", mock.Anything, keyID).Return(nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		err := svc.Revoke(context.Background(), userID, keyID)
		assert.NoError(t, err)
	})

	t.Run("returns conflict when already revoked", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		keyID := uuid.New()
		now := time.Now()
		apiKey := &domain.APIKey{ID: keyID, UserID: userID, Name: "my-key", RevokedAt: &now}
		apiKeyRepo.On("FindByID", mock.Anything, keyID).Return(apiKey, nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		err := svc.Revoke(context.Background(), userID, keyID)
		assert.Error(t, err)
		appErr, ok := err.(*apperrors.AppError)
		require.True(t, ok)
		assert.Equal(t, 409, appErr.HTTPStatus)
	})
}

func TestAPIKeyService_Validate(t *testing.T) {
	t.Run("returns unauthorized for non-existent key hash", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		apiKeyRepo.On("FindByKeyHashWithUser", mock.Anything, mock.AnythingOfType("string")).Return(nil, apperrors.ErrNotFound)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		key := service.KeyPrefixLive + strings.Repeat("0", 64)
		_, _, err := svc.Validate(context.Background(), key)
		assert.Error(t, err)
		appErr, ok := err.(*apperrors.AppError)
		require.True(t, ok)
		assert.Equal(t, 401, appErr.HTTPStatus)
	})

	t.Run("returns unauthorized for revoked key", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		now := time.Now()
		apiKey := &domain.APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			RevokedAt: &now,
			KeyHash:   "somehash",
			Scopes:    datatypes.JSON(`["invoices:view"]`),
			User:      domain.User{ID: userID},
		}
		apiKeyRepo.On("FindByKeyHashWithUser", mock.Anything, mock.AnythingOfType("string")).Return(apiKey, nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		key := service.KeyPrefixLive + strings.Repeat("0", 64)
		_, _, err := svc.Validate(context.Background(), key)
		assert.Error(t, err)
	})
}

func TestAPIKeyService_ValidateScopes(t *testing.T) {
	t.Run("returns error for empty scopes", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		err := svc.ValidateScopes(context.Background(), uuid.New(), []string{})
		assert.Error(t, err)
	})

	t.Run("returns error for invalid scope format", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		err := svc.ValidateScopes(context.Background(), uuid.New(), []string{"noseparator"})
		assert.Error(t, err)
	})

	t.Run("accepts valid scopes", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		err := svc.ValidateScopes(context.Background(), uuid.New(), []string{"invoices:view", "users:manage"})
		assert.NoError(t, err)
	})

	t.Run("accepts wildcard scope", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		err := svc.ValidateScopes(context.Background(), uuid.New(), []string{"*"})
		assert.NoError(t, err)
	})
}

func TestAPIKeyService_CountActiveByUserID(t *testing.T) {
	t.Run("delegates to repository", func(t *testing.T) {
		apiKeyRepo := new(MockAPIKeyRepository)
		userRepo := new(MockUserRepositoryForAPIKey)
		_, auditSvc := newTestAuditServiceForAPIKey()
		defer auditSvc.Shutdown()

		userID := uuid.New()
		apiKeyRepo.On("CountActiveByUserID", mock.Anything, userID).Return(int64(3), nil)

		svc := service.NewAPIKeyService(apiKeyRepo, userRepo, auditSvc)
		count, err := svc.CountActiveByUserID(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})
}

func TestIsValidScope(t *testing.T) {
	t.Run("accepts wildcard", func(t *testing.T) {
		assert.True(t, request.IsValidScope("*"))
	})
	t.Run("accepts resource:action format", func(t *testing.T) {
		assert.True(t, request.IsValidScope("invoices:view"))
	})
	t.Run("accepts resource:wildcard", func(t *testing.T) {
		assert.True(t, request.IsValidScope("invoices:*"))
	})
	t.Run("rejects bare resource without action", func(t *testing.T) {
		assert.False(t, request.IsValidScope("invoices"))
	})
	t.Run("rejects empty string", func(t *testing.T) {
		assert.False(t, request.IsValidScope(""))
	})
	t.Run("rejects colon at start", func(t *testing.T) {
		assert.False(t, request.IsValidScope(":view"))
	})
	t.Run("rejects colon at end", func(t *testing.T) {
		assert.False(t, request.IsValidScope("invoices:"))
	})
}

func TestAPIKey_KeyConstants(t *testing.T) {
	t.Run("key prefix constants are correct", func(t *testing.T) {
		assert.Equal(t, "ak_live_", service.KeyPrefixLive)
		assert.Equal(t, "ak_test_", service.KeyPrefixTest)
		assert.Equal(t, 32, service.KeyRandomLen)
		assert.Equal(t, 12, service.KeyPrefixLen)
	})
}