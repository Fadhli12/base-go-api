package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestEmailQueueRepository_Create tests the Create method
func TestEmailQueueRepository_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates email queue entry successfully", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		email := &domain.EmailQueue{
			ToAddress: "test@example.com",
			Subject:   "Test Subject",
			Status:    domain.EmailStatusQueued,
		}

		mockRepo.On("Create", ctx, email).Return(nil)

		err := mockRepo.Create(ctx, email)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error on database failure", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		email := &domain.EmailQueue{
			ToAddress: "test@example.com",
			Subject:   "Test Subject",
		}

		mockRepo.On("Create", ctx, email).Return(errors.New("database error"))

		err := mockRepo.Create(ctx, email)
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_FindByID tests the FindByID method
func TestEmailQueueRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("finds email by ID", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		expectedEmail := &domain.EmailQueue{
			ID:        emailID,
			ToAddress: "test@example.com",
			Subject:   "Test Subject",
			Status:    domain.EmailStatusSent,
		}

		mockRepo.On("FindByID", ctx, emailID).Return(expectedEmail, nil)

		result, err := mockRepo.FindByID(ctx, emailID)
		assert.NoError(t, err)
		assert.Equal(t, emailID, result.ID)
		assert.Equal(t, "test@example.com", result.ToAddress)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent ID", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()

		mockRepo.On("FindByID", ctx, nonExistentID).Return(nil, apperrors.ErrNotFound)

		result, err := mockRepo.FindByID(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_FindByStatus tests the FindByStatus method
func TestEmailQueueRepository_FindByStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("finds emails by status with pagination", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		emails := []*domain.EmailQueue{
			{ID: uuid.New(), ToAddress: "test1@example.com", Status: domain.EmailStatusQueued},
			{ID: uuid.New(), ToAddress: "test2@example.com", Status: domain.EmailStatusQueued},
		}

		mockRepo.On("FindByStatus", ctx, domain.EmailStatusQueued, 10, 0).Return(emails, int64(2), nil)

		result, total, err := mockRepo.FindByStatus(ctx, domain.EmailStatusQueued, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, result, 2)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns empty list when no emails found", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("FindByStatus", ctx, domain.EmailStatusFailed, 10, 0).Return([]*domain.EmailQueue{}, int64(0), nil)

		result, total, err := mockRepo.FindByStatus(ctx, domain.EmailStatusFailed, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, result)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_FindByRecipient tests the FindByRecipient method
func TestEmailQueueRepository_FindByRecipient(t *testing.T) {
	ctx := context.Background()

	t.Run("finds emails by recipient", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		emails := []*domain.EmailQueue{
			{ID: uuid.New(), ToAddress: "user@example.com", Subject: "Welcome"},
			{ID: uuid.New(), ToAddress: "user@example.com", Subject: "Confirmation"},
		}

		mockRepo.On("FindByRecipient", ctx, "user@example.com", 10, 0).Return(emails, int64(2), nil)

		result, total, err := mockRepo.FindByRecipient(ctx, "user@example.com", 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, result, 2)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns empty when no emails for recipient", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("FindByRecipient", ctx, "nonexistent@example.com", 10, 0).Return([]*domain.EmailQueue{}, int64(0), nil)

		result, total, err := mockRepo.FindByRecipient(ctx, "nonexistent@example.com", 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, result)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_GetNextBatch tests the GetNextBatch method
func TestEmailQueueRepository_GetNextBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("gets next batch of queued emails", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		emails := []*domain.EmailQueue{
			{ID: uuid.New(), ToAddress: "test1@example.com", Status: domain.EmailStatusQueued, Attempts: 0, MaxAttempts: 3},
			{ID: uuid.New(), ToAddress: "test2@example.com", Status: domain.EmailStatusQueued, Attempts: 1, MaxAttempts: 3},
		}

		mockRepo.On("GetNextBatch", ctx, 10).Return(emails, nil)

		result, err := mockRepo.GetNextBatch(ctx, 10)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns empty when no queued emails", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("GetNextBatch", ctx, 10).Return([]*domain.EmailQueue{}, nil)

		result, err := mockRepo.GetNextBatch(ctx, 10)
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_MarkProcessing tests the MarkProcessing method
func TestEmailQueueRepository_MarkProcessing(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("marks email as processing", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("MarkProcessing", ctx, emailID).Return(nil)

		err := mockRepo.MarkProcessing(ctx, emailID)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()

		mockRepo.On("MarkProcessing", ctx, nonExistentID).Return(apperrors.ErrNotFound)

		err := mockRepo.MarkProcessing(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_MarkSent tests the MarkSent method
func TestEmailQueueRepository_MarkSent(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("marks email as sent with provider details", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("MarkSent", ctx, emailID, "smtp", "msg-123").Return(nil)

		err := mockRepo.MarkSent(ctx, emailID, "smtp", "msg-123")
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()

		mockRepo.On("MarkSent", ctx, nonExistentID, "smtp", "msg-123").Return(apperrors.ErrNotFound)

		err := mockRepo.MarkSent(ctx, nonExistentID, "smtp", "msg-123")
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_MarkDelivered tests the MarkDelivered method
func TestEmailQueueRepository_MarkDelivered(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("marks email as delivered", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("MarkDelivered", ctx, emailID).Return(nil)

		err := mockRepo.MarkDelivered(ctx, emailID)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()

		mockRepo.On("MarkDelivered", ctx, nonExistentID).Return(apperrors.ErrNotFound)

		err := mockRepo.MarkDelivered(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_MarkBounced tests the MarkBounced method
func TestEmailQueueRepository_MarkBounced(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("marks email as bounced with reason", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("MarkBounced", ctx, emailID, "550 User unknown").Return(nil)

		err := mockRepo.MarkBounced(ctx, emailID, "550 User unknown")
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()

		mockRepo.On("MarkBounced", ctx, nonExistentID, "550 User unknown").Return(apperrors.ErrNotFound)

		err := mockRepo.MarkBounced(ctx, nonExistentID, "550 User unknown")
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_MarkFailed tests the MarkFailed method
func TestEmailQueueRepository_MarkFailed(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("marks email as failed with error", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		testErr := errors.New("SMTP connection failed")

		mockRepo.On("MarkFailed", ctx, emailID, testErr).Return(nil)

		err := mockRepo.MarkFailed(ctx, emailID, testErr)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()
		testErr := errors.New("SMTP error")

		mockRepo.On("MarkFailed", ctx, nonExistentID, testErr).Return(apperrors.ErrNotFound)

		err := mockRepo.MarkFailed(ctx, nonExistentID, testErr)
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_IncrementAttempts tests the IncrementAttempts method
func TestEmailQueueRepository_IncrementAttempts(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("increments attempt counter", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("IncrementAttempts", ctx, emailID).Return(nil)

		err := mockRepo.IncrementAttempts(ctx, emailID)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()

		mockRepo.On("IncrementAttempts", ctx, nonExistentID).Return(apperrors.ErrNotFound)

		err := mockRepo.IncrementAttempts(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_CanRetry tests the CanRetry method
func TestEmailQueueRepository_CanRetry(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("returns true when attempts under max", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("CanRetry", ctx, emailID).Return(true, nil)

		canRetry, err := mockRepo.CanRetry(ctx, emailID)
		assert.NoError(t, err)
		assert.True(t, canRetry)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns false when attempts at max", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)

		mockRepo.On("CanRetry", ctx, emailID).Return(false, nil)

		canRetry, err := mockRepo.CanRetry(ctx, emailID)
		assert.NoError(t, err)
		assert.False(t, canRetry)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		nonExistentID := uuid.New()

		mockRepo.On("CanRetry", ctx, nonExistentID).Return(false, apperrors.ErrNotFound)

		canRetry, err := mockRepo.CanRetry(ctx, nonExistentID)
		assert.Error(t, err)
		assert.False(t, canRetry)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmailQueueRepository_Update tests the Update method
func TestEmailQueueRepository_Update(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.New()

	t.Run("updates email successfully", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		email := &domain.EmailQueue{
			ID:        emailID,
			ToAddress: "updated@example.com",
			Subject:   "Updated Subject",
		}

		mockRepo.On("Update", ctx, email).Return(nil)

		err := mockRepo.Update(ctx, email)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns ErrNotFound for non-existent email", func(t *testing.T) {
		mockRepo := new(MockEmailQueueRepository)
		email := &domain.EmailQueue{
			ID: uuid.New(),
		}

		mockRepo.On("Update", ctx, email).Return(apperrors.ErrNotFound)

		err := mockRepo.Update(ctx, email)
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}