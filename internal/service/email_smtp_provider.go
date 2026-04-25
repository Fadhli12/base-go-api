package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/smtp"
	"time"

	"github.com/example/go-api-base/internal/config"
)

// SMTPProvider implements EmailProvider using SMTP
type SMTPProvider struct {
	config *config.EmailConfig
}

// NewSMTPProvider creates a new SMTP email provider
func NewSMTPProvider(cfg *config.EmailConfig) *SMTPProvider {
	return &SMTPProvider{config: cfg}
}

// Send sends an email via SMTP and returns a message ID
func (p *SMTPProvider) Send(ctx context.Context, email *EmailMessage) (string, error) {
	// Validate recipient
	if email.To == "" {
		return "", fmt.Errorf("recipient email address is required")
	}

	// Choose body content
	htmlBody := email.HTMLContent
	textBody := email.TextContent
	if htmlBody == "" && textBody == "" {
		return "", fmt.Errorf("email body content is required")
	}

	// Build email message
	from := p.config.SMTPFromAddress
	if p.config.SMTPFromName != "" {
		from = fmt.Sprintf("%s <%s>", p.config.SMTPFromName, p.config.SMTPFromAddress)
	}

	// Generate unique message ID
	messageID := GenerateMessageID()

	// Build MIME message
	message := p.BuildMIMEMessage(from, email.To, email.Subject, htmlBody, textBody, messageID)

	// Send via SMTP
	auth := smtp.PlainAuth(
		"",
		p.config.SMTPUser,
		p.config.SMTPPassword,
		p.config.SMTPHost,
	)

	addr := fmt.Sprintf("%s:%d", p.config.SMTPHost, p.config.SMTPPort)
	err := smtp.SendMail(addr, auth, p.config.SMTPFromAddress, []string{email.To}, []byte(message))
	if err != nil {
		return "", fmt.Errorf("failed to send email via SMTP: %w", err)
	}

	return messageID, nil
}

// Name returns the provider name
func (p *SMTPProvider) Name() string {
	return "smtp"
}

// BuildMIMEMessage constructs a MIME multipart message
func (p *SMTPProvider) BuildMIMEMessage(from, to, subject, htmlBody, textBody, messageID string) string {
	message := fmt.Sprintf("From: %s\r\n", from)
	message += fmt.Sprintf("To: %s\r\n", to)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += fmt.Sprintf("Message-ID: <%s>\r\n", messageID)
	message += "MIME-Version: 1.0\r\n"

	// If both HTML and text are provided, use multipart
	if htmlBody != "" && textBody != "" {
		boundary := "boundary_" + RandomString(16)
		message += fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n", boundary)
		message += "\r\n"
		message += fmt.Sprintf("--%s\r\n", boundary)
		message += "Content-Type: text/plain; charset=UTF-8\r\n\r\n"
		message += textBody + "\r\n\r\n"
		message += fmt.Sprintf("--%s\r\n", boundary)
		message += "Content-Type: text/html; charset=UTF-8\r\n\r\n"
		message += htmlBody + "\r\n\r\n"
		message += fmt.Sprintf("--%s--\r\n", boundary)
	} else if htmlBody != "" {
		message += "Content-Type: text/html; charset=UTF-8\r\n\r\n"
		message += htmlBody
	} else {
		message += "Content-Type: text/plain; charset=UTF-8\r\n\r\n"
		message += textBody
	}

	return message
}

// GenerateMessageID generates a unique message ID (exported for testing)
func GenerateMessageID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%s@%d", hex.EncodeToString(b), time.Now().Unix())
}

// RandomString generates a random string of specified length (exported for testing)
func RandomString(length int) string {
	b := make([]byte, length/2+1)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}