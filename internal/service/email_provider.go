package service

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
)

// EmailProvider defines the interface for sending emails via different providers
type EmailProvider interface {
	// Send sends an email and returns the provider message ID
	Send(ctx context.Context, email *EmailMessage) (messageID string, err error)

	// Name returns the provider name for identification
	Name() string
}

// EmailMessage represents an email message to send
type EmailMessage struct {
	To          string                 // Recipient email address
	Subject     string                 // Email subject
	HTMLContent string                 // HTML body (optional)
	TextContent string                 // Plain text body (optional)
	Template    string                 // Template name (optional)
	Data        map[string]interface{} // Template variables
}

// EmailBounceInfo represents bounce information from a provider webhook
type EmailBounceInfo struct {
	Email        string            // Bounced email address
	MessageID    string            // Provider message ID
	BounceType   domain.BounceType // hard, soft, spam, technical
	BounceReason string            // Detailed reason from provider
	Timestamp    int64             // Unix timestamp of bounce event
}