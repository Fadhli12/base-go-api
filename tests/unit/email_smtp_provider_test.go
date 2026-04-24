package unit

import (
	"testing"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/assert"
)

// TestSMTPProvider_Name tests the Name method
func TestSMTPProvider_Name(t *testing.T) {
	cfg := &config.EmailConfig{
		Provider: "smtp",
	}
	provider := service.NewSMTPProvider(cfg)
	assert.Equal(t, "smtp", provider.Name())
}

// TestSMTPProvider_BuildMIMEMessage tests MIME message building
func TestSMTPProvider_BuildMIMEMessage(t *testing.T) {
	cfg := &config.EmailConfig{
		SMTPHost:        "localhost",
		SMTPPort:         587,
		SMTPFromAddress:  "noreply@example.com",
		SMTPFromName:     "Test App",
	}
	provider := service.NewSMTPProvider(cfg)

	tests := []struct {
		name        string
		from        string
		to          string
		subject     string
		htmlBody    string
		textBody    string
		messageID   string
		expectMIME  bool
	}{
		{
			name:      "HTML only",
			from:      "sender@example.com",
			to:        "recipient@example.com",
			subject:   "Test Subject",
			htmlBody:  "<p>Hello World</p>",
			textBody:  "",
			messageID: "test-123@localhost",
			expectMIME: false,
		},
		{
			name:      "Text only",
			from:      "sender@example.com",
			to:        "recipient@example.com",
			subject:   "Test Subject",
			htmlBody:  "",
			textBody:  "Hello World",
			messageID: "test-456@localhost",
			expectMIME: false,
		},
		{
			name:      "Both HTML and text",
			from:      "sender@example.com",
			to:        "recipient@example.com",
			subject:   "Test Subject",
			htmlBody:  "<p>Hello World</p>",
			textBody:  "Hello World",
			messageID: "test-789@localhost",
			expectMIME: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := provider.BuildMIMEMessage(tt.from, tt.to, tt.subject, tt.htmlBody, tt.textBody, tt.messageID)

			// Verify required headers
			assert.Contains(t, message, "From: "+tt.from)
			assert.Contains(t, message, "To: "+tt.to)
			assert.Contains(t, message, "Subject: "+tt.subject)
			assert.Contains(t, message, "Message-ID: <"+tt.messageID+">")
			assert.Contains(t, message, "MIME-Version: 1.0")

			if tt.expectMIME {
				// Should have multipart/alternative
				assert.Contains(t, message, "Content-Type: multipart/alternative")
				assert.Contains(t, message, "Content-Type: text/plain")
				assert.Contains(t, message, "Content-Type: text/html")
			} else if tt.htmlBody != "" {
				// Should have HTML only
				assert.Contains(t, message, "Content-Type: text/html")
				assert.Contains(t, message, tt.htmlBody)
			} else {
				// Should have text only
				assert.Contains(t, message, "Content-Type: text/plain")
				assert.Contains(t, message, tt.textBody)
			}
		})
	}
}

// TestGenerateMessageID tests message ID generation
func TestGenerateMessageID(t *testing.T) {
	id1 := service.GenerateMessageID()
	id2 := service.GenerateMessageID()

	// IDs should be unique
	assert.NotEqual(t, id1, id2)

	// IDs should contain @
	assert.Contains(t, id1, "@")
	assert.Contains(t, id2, "@")

	// IDs should have correct format: hex@timestamp
	// Basic format check - should have exactly one @
	atCount1 := 0
	for _, c := range id1 {
		if c == '@' {
			atCount1++
		}
	}
	assert.Equal(t, 1, atCount1, "Message ID should have exactly one @")
}