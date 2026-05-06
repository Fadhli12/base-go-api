package service

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"

	appconfig "github.com/example/go-api-base/internal/config"
)

// SESAPI defines the subset of the SES client needed for sending emails.
// This interface enables mocking in unit tests.
type SESAPI interface {
	SendEmail(ctx context.Context, params *ses.SendEmailInput, optFns ...func(*ses.Options)) (*ses.SendEmailOutput, error)
}

// SESProvider implements EmailProvider using AWS SES
type SESProvider struct {
	config    *appconfig.SESConfig
	sesClient SESAPI
}

// NewSESProvider creates a new AWS SES email provider.
// The SES client is lazily created on first Send() call unless SetSESClient is called.
func NewSESProvider(cfg *appconfig.SESConfig) *SESProvider {
	return &SESProvider{config: cfg}
}

// SetSESClient injects a custom SESAPI client for testing.
func (p *SESProvider) SetSESClient(client SESAPI) {
	p.sesClient = client
}

// getClient returns the SES client, creating it from config if not already set.
func (p *SESProvider) getClient(ctx context.Context) (SESAPI, error) {
	if p.sesClient != nil {
		return p.sesClient, nil
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(p.config.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if p.config.AccessKeyID != "" && p.config.SecretAccessKey != "" {
		awsCfg.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     p.config.AccessKeyID,
				SecretAccessKey: p.config.SecretAccessKey,
			}, nil
		})
	}

	return ses.NewFromConfig(awsCfg), nil
}

// buildInput constructs the SendEmailInput from the email message.
func (p *SESProvider) buildInput(email *EmailMessage) *ses.SendEmailInput {
	source := p.config.FromAddress
	if p.config.FromName != "" {
		source = fmt.Sprintf("%s <%s>", p.config.FromName, p.config.FromAddress)
	}

	input := &ses.SendEmailInput{
		Source: aws.String(source),
		Destination: &types.Destination{
			ToAddresses: []string{email.To},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(email.Subject),
			},
			Body: &types.Body{},
		},
	}

	if email.HTMLContent != "" {
		input.Message.Body.Html = &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(email.HTMLContent),
		}
	}
	if email.TextContent != "" {
		input.Message.Body.Text = &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(email.TextContent),
		}
	}

	return input
}

// Send sends an email via AWS SES and returns the message ID
func (p *SESProvider) Send(ctx context.Context, email *EmailMessage) (string, error) {
	if email.To == "" {
		return "", fmt.Errorf("recipient email address is required")
	}

	if email.HTMLContent == "" && email.TextContent == "" {
		return "", fmt.Errorf("email body content is required")
	}

	client, err := p.getClient(ctx)
	if err != nil {
		return "", err
	}

	input := p.buildInput(email)

	output, err := client.SendEmail(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to send email via SES: %w", err)
	}

	return *output.MessageId, nil
}

// Name returns the provider name
func (p *SESProvider) Name() string {
	return "ses"
}
