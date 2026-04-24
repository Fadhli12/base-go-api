package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEmailQueueRepository mocks EmailQueueRepository for testing
type MockEmailQueueRepository struct {
	mock.Mock
}

func (m *MockEmailQueueRepository) Create(ctx context.Context, email *domain.EmailQueue) error {
	args := m.Called(ctx, email)
	if args.Error(0) == nil {
		email.ID = uuid.New() // Simulate DB auto-generation
	}
	return args.Error(0)
}

func (m *MockEmailQueueRepository) Update(ctx context.Context, email *domain.EmailQueue) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockEmailQueueRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.EmailQueue, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EmailQueue), args.Error(1)
}

func (m *MockEmailQueueRepository) FindByStatus(ctx context.Context, status domain.EmailStatus, limit, offset int) ([]*domain.EmailQueue, int64, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.EmailQueue), args.Get(1).(int64), args.Error(2)
}

func (m *MockEmailQueueRepository) FindByRecipient(ctx context.Context, toAddress string, limit, offset int) ([]*domain.EmailQueue, int64, error) {
	args := m.Called(ctx, toAddress, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.EmailQueue), args.Get(1).(int64), args.Error(2)
}

func (m *MockEmailQueueRepository) GetNextBatch(ctx context.Context, batchSize int) ([]*domain.EmailQueue, error) {
	args := m.Called(ctx, batchSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.EmailQueue), args.Error(1)
}

func (m *MockEmailQueueRepository) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockEmailQueueRepository) MarkSent(ctx context.Context, id uuid.UUID, provider, messageID string) error {
	args := m.Called(ctx, id, provider, messageID)
	return args.Error(0)
}

func (m *MockEmailQueueRepository) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockEmailQueueRepository) MarkBounced(ctx context.Context, id uuid.UUID, reason string) error {
	args := m.Called(ctx, id, reason)
	return args.Error(0)
}

func (m *MockEmailQueueRepository) MarkFailed(ctx context.Context, id uuid.UUID, err error) error {
	args := m.Called(ctx, id, err)
	return args.Error(0)
}

func (m *MockEmailQueueRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockEmailQueueRepository) CanRetry(ctx context.Context, id uuid.UUID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

// MockEmailBounceRepository mocks EmailBounceRepository for testing
type MockEmailBounceRepository struct {
	mock.Mock
}

func (m *MockEmailBounceRepository) Create(ctx context.Context, bounce *domain.EmailBounce) error {
	args := m.Called(ctx, bounce)
	return args.Error(0)
}

func (m *MockEmailBounceRepository) FindByID(ctx context.Context, id string) (*domain.EmailBounce, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EmailBounce), args.Error(1)
}

func (m *MockEmailBounceRepository) FindByEmail(ctx context.Context, email string, limit, offset int) ([]*domain.EmailBounce, int64, error) {
	args := m.Called(ctx, email, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.EmailBounce), args.Get(1).(int64), args.Error(2)
}

func (m *MockEmailBounceRepository) FindByType(ctx context.Context, bounceType domain.BounceType, limit, offset int) ([]*domain.EmailBounce, int64, error) {
	args := m.Called(ctx, bounceType, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.EmailBounce), args.Get(1).(int64), args.Error(2)
}

func (m *MockEmailBounceRepository) FindByMessageID(ctx context.Context, messageID string) (*domain.EmailBounce, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EmailBounce), args.Error(1)
}

func (m *MockEmailBounceRepository) CountByType(ctx context.Context) (map[domain.BounceType]int64, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.BounceType]int64), args.Error(1)
}

func (m *MockEmailBounceRepository) CountByEmail(ctx context.Context, email string) (int64, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(int64), args.Error(1)
}

// TestEmailService_QueueEmail tests email queueing
func TestEmailService_QueueEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("queues email successfully", func(t *testing.T) {
		mockTemplateRepo := new(MockEmailTemplateRepository)
		mockQueueRepo := new(MockEmailQueueRepository)
		mockBounceRepo := new(MockEmailBounceRepository)
		mockProvider := new(MockEmailProvider)

		cfg := &config.EmailConfig{
			RetryMax: 5,
		}

		emailService := service.NewEmailService(
			cfg,
			mockTemplateRepo,
			mockQueueRepo,
			mockBounceRepo,
			service.NewTemplateEngine(mockTemplateRepo),
			mockProvider,
		)

		req := &service.EmailRequest{
			To:          "test@example.com",
			Subject:     "Test Subject",
			HTMLContent: "<p>Test content</p>",
		}

		mockQueueRepo.On("Create", ctx, mock.Anything).Return(nil)

		err := emailService.QueueEmail(ctx, req)
		require.NoError(t, err)
		mockQueueRepo.AssertExpectations(t)
	})

	t.Run("validates recipient", func(t *testing.T) {
		mockTemplateRepo := new(MockEmailTemplateRepository)
		mockQueueRepo := new(MockEmailQueueRepository)
		mockBounceRepo := new(MockEmailBounceRepository)
		mockProvider := new(MockEmailProvider)

		cfg := &config.EmailConfig{RetryMax: 5}

		emailService := service.NewEmailService(
			cfg,
			mockTemplateRepo,
			mockQueueRepo,
			mockBounceRepo,
			service.NewTemplateEngine(mockTemplateRepo),
			mockProvider,
		)

		req := &service.EmailRequest{
			To:      "",
			Subject: "Test",
		}

		err := emailService.QueueEmail(ctx, req)
		require.Error(t, err)
		appErr := apperrors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.HTTPStatus)
	})

	t.Run("validates email content", func(t *testing.T) {
		mockTemplateRepo := new(MockEmailTemplateRepository)
		mockQueueRepo := new(MockEmailQueueRepository)
		mockBounceRepo := new(MockEmailBounceRepository)
		mockProvider := new(MockEmailProvider)

		cfg := &config.EmailConfig{RetryMax: 5}

		emailService := service.NewEmailService(
			cfg,
			mockTemplateRepo,
			mockQueueRepo,
			mockBounceRepo,
			service.NewTemplateEngine(mockTemplateRepo),
			mockProvider,
		)

		req := &service.EmailRequest{
			To: "test@example.com",
		}

		err := emailService.QueueEmail(ctx, req)
		require.Error(t, err)
		appErr := apperrors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.HTTPStatus)
	})
}

// TestEmailService_RecordBounce tests bounce recording
func TestEmailService_RecordBounce(t *testing.T) {
	ctx := context.Background()

	t.Run("records bounce successfully", func(t *testing.T) {
		mockTemplateRepo := new(MockEmailTemplateRepository)
		mockQueueRepo := new(MockEmailQueueRepository)
		mockBounceRepo := new(MockEmailBounceRepository)
		mockProvider := new(MockEmailProvider)

		cfg := &config.EmailConfig{RetryMax: 5}

		emailService := service.NewEmailService(
			cfg,
			mockTemplateRepo,
			mockQueueRepo,
			mockBounceRepo,
			service.NewTemplateEngine(mockTemplateRepo),
			mockProvider,
		)

		emailID := uuid.New()
		bounceInfo := &service.EmailBounceInfo{
			Email:        "bounced@example.com",
			MessageID:    emailID.String(),
			BounceType:   domain.BounceTypeHard,
			BounceReason: "550 User unknown",
		}

		// RecordBounce tries to FindByID, and if not found, it ignores the error
		// and still creates the bounce record. The test should mock FindByID to return nil
		// and then expect the bounce to be created.
		mockQueueRepo.On("FindByID", ctx, emailID).Return(nil, errors.New("not found"))
		mockBounceRepo.On("Create", ctx, mock.Anything).Return(nil)

		err := emailService.RecordBounce(ctx, bounceInfo)
		require.NoError(t, err)
		mockQueueRepo.AssertExpectations(t)
		mockBounceRepo.AssertExpectations(t)
	})
}

// TestEmailService_GetQueueStatus tests status retrieval
func TestEmailService_GetQueueStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieves status successfully", func(t *testing.T) {
		mockTemplateRepo := new(MockEmailTemplateRepository)
		mockQueueRepo := new(MockEmailQueueRepository)
		mockBounceRepo := new(MockEmailBounceRepository)
		mockProvider := new(MockEmailProvider)

		cfg := &config.EmailConfig{RetryMax: 5}

		emailService := service.NewEmailService(
			cfg,
			mockTemplateRepo,
			mockQueueRepo,
			mockBounceRepo,
			service.NewTemplateEngine(mockTemplateRepo),
			mockProvider,
		)

		emailID := uuid.New()
		email := &domain.EmailQueue{
			ID:        emailID,
			ToAddress: "test@example.com",
			Subject:   "Test",
			Status:    domain.EmailStatusSent,
		}

		mockQueueRepo.On("FindByID", ctx, emailID).Return(email, nil)

		result, err := emailService.GetQueueStatus(ctx, emailID)
		require.NoError(t, err)
		assert.Equal(t, emailID, result.ID)
		assert.Equal(t, "test@example.com", result.ToAddress)
		mockQueueRepo.AssertExpectations(t)
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		mockTemplateRepo := new(MockEmailTemplateRepository)
		mockQueueRepo := new(MockEmailQueueRepository)
		mockBounceRepo := new(MockEmailBounceRepository)
		mockProvider := new(MockEmailProvider)

		cfg := &config.EmailConfig{RetryMax: 5}

		emailService := service.NewEmailService(
			cfg,
			mockTemplateRepo,
			mockQueueRepo,
			mockBounceRepo,
			service.NewTemplateEngine(mockTemplateRepo),
			mockProvider,
		)

		emailID := uuid.New()

		mockQueueRepo.On("FindByID", ctx, emailID).Return(nil, errors.New("not found"))

		_, err := emailService.GetQueueStatus(ctx, emailID)
		require.Error(t, err)
		mockQueueRepo.AssertExpectations(t)
	})
}