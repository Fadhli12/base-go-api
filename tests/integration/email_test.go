//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmailProvider for testing
type mockEmailProvider struct {
	sentMessages []*service.EmailMessage
	sendErr      error
}

func (m *mockEmailProvider) Send(ctx context.Context, msg *service.EmailMessage) (string, error) {
	if m.sendErr != nil {
		return "", m.sendErr
	}
	m.sentMessages = append(m.sentMessages, msg)
	return uuid.New().String(), nil
}

func (m *mockEmailProvider) IsConfigured() bool {
	return true
}

func (m *mockEmailProvider) Name() string {
	return "mock"
}

func TestEmailService_QueueEmail(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	ctx := context.Background()

	t.Run("queues email with direct content", func(t *testing.T) {
		suite.SetupTest(t)

		// Create repositories
		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		cfg := &config.EmailConfig{
			RetryMax: 5,
		}

		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		req := &service.EmailRequest{
			To:          "test@example.com",
			Subject:     "Test Subject",
			HTMLContent: "<p>Hello World</p>",
			TextContent: "Hello World",
		}

		err := emailService.QueueEmail(ctx, req)
		require.NoError(t, err)

		// Verify email was queued
		queued, count, err := queueRepo.FindByStatus(ctx, domain.EmailStatusQueued, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
		assert.Len(t, queued, 1)
		assert.Equal(t, "test@example.com", queued[0].ToAddress)
		assert.Equal(t, "Test Subject", queued[0].Subject)
	})

	t.Run("queues email with template", func(t *testing.T) {
		suite.SetupTest(t)

		// Set up template first
		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		// Create a template
		template := &domain.EmailTemplate{
			Name:        "welcome",
			Subject:     "Welcome to Our Platform",
			HTMLContent: "<h1>Hello {{.Name}}</h1>",
			TextContent: "Hello {{.Name}}",
			IsActive:    true,
		}
		err := templateRepo.Create(ctx, template)
		require.NoError(t, err)

		cfg := &config.EmailConfig{
			RetryMax: 5,
		}

		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		req := &service.EmailRequest{
			To:       "newuser@example.com",
			Template: "welcome",
			Data: map[string]any{
				"Name": "John",
			},
		}

		err = emailService.QueueEmail(ctx, req)
		require.NoError(t, err)

		// Verify email was queued with template
		queued, count, err := queueRepo.FindByStatus(ctx, domain.EmailStatusQueued, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
		assert.Equal(t, "welcome", queued[0].Template)
	})

	t.Run("fails with missing recipient", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		req := &service.EmailRequest{
			Subject:     "Test",
			HTMLContent: "<p>Content</p>",
		}

		err := emailService.QueueEmail(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipient")
	})

	t.Run("fails without content or template", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		req := &service.EmailRequest{
			To: "test@example.com",
		}

		err := emailService.QueueEmail(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template")
	})
}

func TestEmailService_SendTransactional(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	ctx := context.Background()

	t.Run("sends transactional email", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		// Create a template
		template := &domain.EmailTemplate{
			Name:        "password-reset",
			Subject:     "Reset Your Password",
			HTMLContent: "<p>Click <a href=\"{{.ResetURL}}\">here</a> to reset</p>",
			TextContent: "Reset at: {{.ResetURL}}",
			IsActive:    true,
		}
		err := templateRepo.Create(ctx, template)
		require.NoError(t, err)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		email, err := emailService.SendTransactional(ctx, "user@example.com", "password-reset", map[string]any{
			"ResetURL": "https://example.com/reset?token=abc123",
		})

		require.NoError(t, err)
		assert.NotNil(t, email)
		assert.Equal(t, "user@example.com", email.ToAddress)
		assert.Equal(t, domain.EmailStatusSent, email.Status)
		assert.NotEmpty(t, email.Provider)
		assert.NotEmpty(t, email.MessageID)

		// Verify mock provider received the message
		require.Len(t, mockProvider.sentMessages, 1)
		assert.Equal(t, "user@example.com", mockProvider.sentMessages[0].To)
		assert.Contains(t, mockProvider.sentMessages[0].HTMLContent, "https://example.com/reset?token=abc123")
	})
}

func TestEmailService_RecordBounce(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	ctx := context.Background()

	t.Run("records bounce event", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		bounceInfo := &service.EmailBounceInfo{
			Email:        "bounced@example.com",
			MessageID:    uuid.New().String(),
			BounceType:   domain.BounceTypeHard,
			BounceReason: "550 User unknown",
		}

		err := emailService.RecordBounce(ctx, bounceInfo)
		require.NoError(t, err)

		// Verify bounce was recorded
		bounces, count, err := bounceRepo.FindByEmail(ctx, "bounced@example.com", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
		assert.Len(t, bounces, 1)
		assert.Equal(t, domain.BounceTypeHard, bounces[0].BounceType)
		assert.Equal(t, "550 User unknown", bounces[0].BounceReason)
	})

	t.Run("soft bounce does not block future emails", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		bounceInfo := &service.EmailBounceInfo{
			Email:        "softbounce@example.com",
			MessageID:    uuid.New().String(),
			BounceType:   domain.BounceTypeSoft,
			BounceReason: "mailbox full",
		}

		err := emailService.RecordBounce(ctx, bounceInfo)
		require.NoError(t, err)

		// Verify soft bounce recorded
		bounces, _, err := bounceRepo.FindByEmail(ctx, "softbounce@example.com", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, domain.BounceTypeSoft, bounces[0].BounceType)
	})
}

func TestEmailTemplate_Rendering(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	ctx := context.Background()

	t.Run("renders template with variables", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)

		// Create template with variables
		template := &domain.EmailTemplate{
			Name:        "invite",
			Subject:     "You're invited to {{.OrgName}}",
			HTMLContent: "<h1>Hello {{.UserName}}</h1><p>Join {{.OrgName}}</p>",
			TextContent: "Hello {{.UserName}}\nJoin {{.OrgName}}",
			IsActive:    true,
		}
		err := templateRepo.Create(ctx, template)
		require.NoError(t, err)

		templateEngine := service.NewTemplateEngine(templateRepo)

		html, text, err := templateEngine.RenderTemplate(ctx, "invite", map[string]any{
			"UserName": "John Doe",
			"OrgName":  "Acme Corp",
		})

		require.NoError(t, err)
		assert.Contains(t, html, "John Doe")
		assert.Contains(t, html, "Acme Corp")
		assert.Contains(t, text, "John Doe")
		assert.Contains(t, text, "Acme Corp")
	})

	t.Run("fails with inactive template", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)

		// Create inactive template
		template := &domain.EmailTemplate{
			Name:        "inactive",
			Subject:     "Test",
			HTMLContent: "<p>Test</p>",
			TextContent: "Test",
			IsActive:    false,
		}
		err := templateRepo.Create(ctx, template)
		require.NoError(t, err)

		cfg := &config.EmailConfig{RetryMax: 5}
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		req := &service.EmailRequest{
			To:       "test@example.com",
			Template: "inactive",
		}

		err = emailService.QueueEmail(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inactive")
	})

	t.Run("fails with non-existent template", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		req := &service.EmailRequest{
			To:       "test@example.com",
			Template: "nonexistent",
		}

		err := emailService.QueueEmail(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestEmailQueue_StatusTransitions(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	ctx := context.Background()

	t.Run("email status transitions correctly", func(t *testing.T) {
		suite.SetupTest(t)

		queueRepo := repository.NewEmailQueueRepository(suite.DB)

		// Create queued email
		email := &domain.EmailQueue{
			ToAddress:   "test@example.com",
			Subject:     "Test",
			Status:      domain.EmailStatusQueued,
			MaxAttempts: 5,
		}
		err := queueRepo.Create(ctx, email)
		require.NoError(t, err)
		assert.NotEmpty(t, email.ID)

		// Transition to processing
		err = queueRepo.MarkProcessing(ctx, email.ID)
		require.NoError(t, err)

		email, err = queueRepo.FindByID(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.EmailStatusProcessing, email.Status)

		// Transition to sent
		err = queueRepo.MarkSent(ctx, email.ID, "smtp", "msg-123")
		require.NoError(t, err)

		email, err = queueRepo.FindByID(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.EmailStatusSent, email.Status)
		assert.Equal(t, "smtp", email.Provider)
		assert.Equal(t, "msg-123", email.MessageID)

		// Transition to delivered
		err = queueRepo.MarkDelivered(ctx, email.ID)
		require.NoError(t, err)

		email, err = queueRepo.FindByID(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.EmailStatusDelivered, email.Status)
	})

	t.Run("failed email is marked correctly", func(t *testing.T) {
		suite.SetupTest(t)

		queueRepo := repository.NewEmailQueueRepository(suite.DB)

		email := &domain.EmailQueue{
			ToAddress:   "fail@example.com",
			Subject:     "Test",
			Status:      domain.EmailStatusQueued,
			MaxAttempts: 5,
		}
		err := queueRepo.Create(ctx, email)
		require.NoError(t, err)

		// Mark as processing then failed
		err = queueRepo.MarkProcessing(ctx, email.ID)
		require.NoError(t, err)

		testErr := assert.AnError
		err = queueRepo.MarkFailed(ctx, email.ID, testErr)
		require.NoError(t, err)

		email, err = queueRepo.FindByID(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.EmailStatusFailed, email.Status)
		assert.Equal(t, 1, email.Attempts)
	})

	t.Run("bounced email is recorded correctly", func(t *testing.T) {
		suite.SetupTest(t)

		queueRepo := repository.NewEmailQueueRepository(suite.DB)

		email := &domain.EmailQueue{
			ToAddress:   "bounce@example.com",
			Subject:     "Test",
			Status:      domain.EmailStatusSent,
			MaxAttempts: 5,
		}
		err := queueRepo.Create(ctx, email)
		require.NoError(t, err)

		// Mark as bounced
		err = queueRepo.MarkBounced(ctx, email.ID, "550 User unknown")
		require.NoError(t, err)

		email, err = queueRepo.FindByID(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.EmailStatusBounced, email.Status)
		assert.Equal(t, "550 User unknown", email.BounceReason)
	})
}

func TestEmailQueue_RetryMechanism(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	ctx := context.Background()

	t.Run("CanRetry returns true under max attempts", func(t *testing.T) {
		suite.SetupTest(t)

		queueRepo := repository.NewEmailQueueRepository(suite.DB)

		email := &domain.EmailQueue{
			ToAddress:   "retry@example.com",
			Subject:     "Test",
			Status:      domain.EmailStatusQueued,
			MaxAttempts: 5,
			Attempts:    3,
		}
		err := queueRepo.Create(ctx, email)
		require.NoError(t, err)

		canRetry, err := queueRepo.CanRetry(ctx, email.ID)
		require.NoError(t, err)
		assert.True(t, canRetry)
	})

	t.Run("CanRetry returns false at max attempts", func(t *testing.T) {
		suite.SetupTest(t)

		queueRepo := repository.NewEmailQueueRepository(suite.DB)

		email := &domain.EmailQueue{
			ToAddress:   "maxretry@example.com",
			Subject:     "Test",
			Status:      domain.EmailStatusQueued,
			MaxAttempts: 5,
			Attempts:    5,
		}
		err := queueRepo.Create(ctx, email)
		require.NoError(t, err)

		canRetry, err := queueRepo.CanRetry(ctx, email.ID)
		require.NoError(t, err)
		assert.False(t, canRetry)
	})

	t.Run("IncrementAttempts increments counter", func(t *testing.T) {
		suite.SetupTest(t)

		queueRepo := repository.NewEmailQueueRepository(suite.DB)

		email := &domain.EmailQueue{
			ToAddress:   "increment@example.com",
			Subject:     "Test",
			Status:      domain.EmailStatusQueued,
			MaxAttempts: 5,
			Attempts:    0,
		}
		err := queueRepo.Create(ctx, email)
		require.NoError(t, err)

		err = queueRepo.IncrementAttempts(ctx, email.ID)
		require.NoError(t, err)

		email, err = queueRepo.FindByID(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, email.Attempts)

		// Increment again
		err = queueRepo.IncrementAttempts(ctx, email.ID)
		require.NoError(t, err)

		email, err = queueRepo.FindByID(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, email.Attempts)
	})
}

func TestEmailService_GetQueueStatus(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	ctx := context.Background()

	t.Run("retrieves email status by ID", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		// Create queued email
		email := &domain.EmailQueue{
			ToAddress:   "status@example.com",
			Subject:     "Status Test",
			Status:      domain.EmailStatusQueued,
			MaxAttempts: 5,
		}
		err := queueRepo.Create(ctx, email)
		require.NoError(t, err)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		result, err := emailService.GetQueueStatus(ctx, email.ID)
		require.NoError(t, err)
		assert.Equal(t, email.ID, result.ID)
		assert.Equal(t, "status@example.com", result.ToAddress)
		assert.Equal(t, domain.EmailStatusQueued, result.Status)
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		suite.SetupTest(t)

		templateRepo := repository.NewEmailTemplateRepository(suite.DB)
		queueRepo := repository.NewEmailQueueRepository(suite.DB)
		bounceRepo := repository.NewEmailBounceRepository(suite.DB)

		cfg := &config.EmailConfig{RetryMax: 5}
		templateEngine := service.NewTemplateEngine(templateRepo)
		mockProvider := &mockEmailProvider{}

		emailService := service.NewEmailService(
			cfg,
			templateRepo,
			queueRepo,
			bounceRepo,
			templateEngine,
			mockProvider,
		)

		_, err := emailService.GetQueueStatus(ctx, uuid.New())
		require.Error(t, err)
	})
}