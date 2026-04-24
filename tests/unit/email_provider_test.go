package unit

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEmailProvider implements EmailProvider for testing
type MockEmailProvider struct {
	SendFunc    func(ctx context.Context, email *service.EmailMessage) (string, error)
	NameFunc    func() string
	sendCalled  bool
	nameCalled  bool
	messageID   string
	sendError   error
}

func (m *MockEmailProvider) Send(ctx context.Context, email *service.EmailMessage) (string, error) {
	m.sendCalled = true
	if m.SendFunc != nil {
		return m.SendFunc(ctx, email)
	}
	return m.messageID, m.sendError
}

func (m *MockEmailProvider) Name() string {
	m.nameCalled = true
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock"
}

// TestEmailMessage_Validation tests EmailMessage structure
func TestEmailMessage_Validation(t *testing.T) {
	tests := []struct {
		name        string
		email       *service.EmailMessage
		expectError bool
	}{
		{
			name: "valid email with HTML content",
			email: &service.EmailMessage{
				To:          "test@example.com",
				Subject:     "Test Subject",
				HTMLContent: "<p>Hello</p>",
			},
			expectError: false,
		},
		{
			name: "valid email with text content",
			email: &service.EmailMessage{
				To:          "test@example.com",
				Subject:     "Test Subject",
				TextContent: "Hello",
			},
			expectError: false,
		},
		{
			name: "valid email with both formats",
			email: &service.EmailMessage{
				To:          "test@example.com",
				Subject:     "Test Subject",
				HTMLContent: "<p>Hello</p>",
				TextContent: "Hello",
			},
			expectError: false,
		},
		{
			name: "email with template",
			email: &service.EmailMessage{
				To:       "test@example.com",
				Subject:  "Test Subject",
				Template: "welcome_email",
				Data:     map[string]interface{}{"name": "John"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// EmailMessage is a simple struct, validation happens at the provider level
			// Just verify the structure can be created
			require.NotNil(t, tt.email)
			assert.NotEmpty(t, tt.email.To)
		})
	}
}

// TestEmailBounceInfo_Structure tests EmailBounceInfo structure
func TestEmailBounceInfo_Structure(t *testing.T) {
	bounceInfo := &service.EmailBounceInfo{
		Email:        "bounced@example.com",
		MessageID:    "msg-123",
		BounceType:   domain.BounceTypeHard,
		BounceReason: "550 User unknown",
		Timestamp:    1234567890,
	}

	assert.Equal(t, "bounced@example.com", bounceInfo.Email)
	assert.Equal(t, "msg-123", bounceInfo.MessageID)
	assert.Equal(t, domain.BounceTypeHard, bounceInfo.BounceType)
	assert.Equal(t, "550 User unknown", bounceInfo.BounceReason)
	assert.Equal(t, int64(1234567890), bounceInfo.Timestamp)
}

// TestMockEmailProvider tests the mock provider implementation
func TestMockEmailProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("successful send", func(t *testing.T) {
		mock := &MockEmailProvider{
			messageID: "msg-123",
		}

		email := &service.EmailMessage{
			To:      "test@example.com",
			Subject: "Test",
		}

		id, err := mock.Send(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, "msg-123", id)
		assert.True(t, mock.sendCalled)
	})

	t.Run("failed send", func(t *testing.T) {
		mock := &MockEmailProvider{
			sendError: assert.AnError,
		}

		email := &service.EmailMessage{
			To:      "test@example.com",
			Subject: "Test",
		}

		id, err := mock.Send(ctx, email)
		require.Error(t, err)
		assert.Equal(t, "", id)
		assert.True(t, mock.sendCalled)
	})

	t.Run("name returns default", func(t *testing.T) {
		mock := &MockEmailProvider{}
		name := mock.Name()
		assert.Equal(t, "mock", name)
		assert.True(t, mock.nameCalled)
	})

	t.Run("custom name function", func(t *testing.T) {
		mock := &MockEmailProvider{
			NameFunc: func() string {
				return "custom-provider"
			},
		}
		name := mock.Name()
		assert.Equal(t, "custom-provider", name)
	})
}

// TestEmailProvider_Interface tests that MockEmailProvider implements EmailProvider
func TestEmailProvider_Interface(t *testing.T) {
	var _ service.EmailProvider = (*MockEmailProvider)(nil)
}