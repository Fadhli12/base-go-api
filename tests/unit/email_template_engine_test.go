package unit

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEmailTemplateRepository mocks EmailTemplateRepository for testing
type MockEmailTemplateRepository struct {
	mock.Mock
}

func (m *MockEmailTemplateRepository) Create(ctx context.Context, template *domain.EmailTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockEmailTemplateRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.EmailTemplate, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EmailTemplate), args.Error(1)
}

func (m *MockEmailTemplateRepository) FindByName(ctx context.Context, name string) (*domain.EmailTemplate, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EmailTemplate), args.Error(1)
}

func (m *MockEmailTemplateRepository) FindByCategory(ctx context.Context, category string, limit, offset int) ([]*domain.EmailTemplate, int64, error) {
	args := m.Called(ctx, category, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.EmailTemplate), args.Get(1).(int64), args.Error(2)
}

func (m *MockEmailTemplateRepository) FindActive(ctx context.Context, category string) ([]*domain.EmailTemplate, error) {
	args := m.Called(ctx, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.EmailTemplate), args.Error(1)
}

func (m *MockEmailTemplateRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.EmailTemplate, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.EmailTemplate), args.Get(1).(int64), args.Error(2)
}

func (m *MockEmailTemplateRepository) Update(ctx context.Context, template *domain.EmailTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockEmailTemplateRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// TestTemplateEngine_RenderTemplate tests template rendering
func TestTemplateEngine_RenderTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("renders template successfully", func(t *testing.T) {
		mockRepo := new(MockEmailTemplateRepository)
		engine := service.NewTemplateEngine(mockRepo)

		template := &domain.EmailTemplate{
			ID:          uuid.New(),
			Name:        "welcome",
			Subject:     "Welcome!",
			HTMLContent: "<h1>Hello, {{.Name}}!</h1>",
			TextContent: "Hello, {{.Name}}!",
			Category:    "transactional",
			IsActive:    true,
		}

		mockRepo.On("FindByName", ctx, "welcome").Return(template, nil)

		data := map[string]any{"Name": "John"}
		html, text, err := engine.RenderTemplate(ctx, "welcome", data)

		require.NoError(t, err)
		assert.Equal(t, "<h1>Hello, John!</h1>", html)
		assert.Equal(t, "Hello, John!", text)
		mockRepo.AssertExpectations(t)
	})

	t.Run("renders HTML only template", func(t *testing.T) {
		mockRepo := new(MockEmailTemplateRepository)
		engine := service.NewTemplateEngine(mockRepo)

		template := &domain.EmailTemplate{
			ID:          uuid.New(),
			Name:        "notification",
			Subject:     "Notification",
			HTMLContent: "<p>Alert: {{.Message}}</p>",
			TextContent: "",
			Category:    "notification",
			IsActive:    true,
		}

		mockRepo.On("FindByName", ctx, "notification").Return(template, nil)

		data := map[string]any{"Message": "Test alert"}
		html, text, err := engine.RenderTemplate(ctx, "notification", data)

		require.NoError(t, err)
		assert.Equal(t, "<p>Alert: Test alert</p>", html)
		assert.Equal(t, "", text)
	})

	t.Run("returns error for inactive template", func(t *testing.T) {
		mockRepo := new(MockEmailTemplateRepository)
		engine := service.NewTemplateEngine(mockRepo)

		template := &domain.EmailTemplate{
			ID:          uuid.New(),
			Name:        "inactive",
			Subject:     "Inactive",
			HTMLContent: "<p>Test</p>",
			Category:    "transactional",
			IsActive:    false,
		}

		mockRepo.On("FindByName", ctx, "inactive").Return(template, nil)

		data := map[string]any{}
		_, _, err := engine.RenderTemplate(ctx, "inactive", data)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not active")
	})

	t.Run("returns error for non-existent template", func(t *testing.T) {
		mockRepo := new(MockEmailTemplateRepository)
		engine := service.NewTemplateEngine(mockRepo)

		mockRepo.On("FindByName", ctx, "nonexistent").Return(nil, assert.AnError)

		data := map[string]any{}
		_, _, err := engine.RenderTemplate(ctx, "nonexistent", data)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "template not found")
	})
}

// TestTemplateEngine_ValidateTemplate tests template validation
func TestTemplateEngine_ValidateTemplate(t *testing.T) {
	mockRepo := new(MockEmailTemplateRepository)
	engine := service.NewTemplateEngine(mockRepo)

	tests := []struct {
		name        string
		htmlContent string
		textContent string
		expectError bool
	}{
		{
			name:        "valid HTML content",
			htmlContent: "<p>Hello {{.Name}}</p>",
			textContent:  "",
			expectError: false,
		},
		{
			name:        "valid text content",
			htmlContent: "",
			textContent:  "Hello {{.Name}}",
			expectError: false,
		},
		{
			name:        "valid both formats",
			htmlContent: "<p>Hello</p>",
			textContent:  "Hello",
			expectError: false,
		},
		{
			name:        "empty both",
			htmlContent: "",
			textContent:  "",
			expectError: true,
		},
		{
			name:        "invalid HTML template",
			htmlContent: "<p>{{.Name}</p>",
			textContent:  "",
			expectError: true,
		},
		{
			name:        "invalid text template",
			htmlContent: "",
			textContent:  "Hello {{.Name",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidateTemplate(tt.htmlContent, tt.textContent)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}