package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockSESClient implements service.SESAPI for testing.
type mockSESClient struct {
	mock.Mock
}

func (m *mockSESClient) SendEmail(ctx context.Context, params *ses.SendEmailInput, optFns ...func(*ses.Options)) (*ses.SendEmailOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ses.SendEmailOutput), args.Error(1)
}

func newTestSESProvider() *service.SESProvider {
	cfg := &config.SESConfig{
		Region:      "us-east-1",
		FromAddress: "noreply@example.com",
		FromName:    "Test App",
	}
	return service.NewSESProvider(cfg)
}

// TestSESProvider_Name tests the Name method.
func TestSESProvider_Name(t *testing.T) {
	provider := newTestSESProvider()
	assert.Equal(t, "ses", provider.Name())
}

// TestSESProvider_Send_Success tests a successful SES send.
func TestSESProvider_Send_Success(t *testing.T) {
	provider := newTestSESProvider()
	client := new(mockSESClient)
	provider.SetSESClient(client)

	expectedMsgID := "ses-msg-abc123"
	client.On("SendEmail", mock.Anything, mock.MatchedBy(func(input *ses.SendEmailInput) bool {
		return aws.ToString(input.Source) == "Test App <noreply@example.com>" &&
			len(input.Destination.ToAddresses) == 1 &&
			input.Destination.ToAddresses[0] == "user@example.com" &&
			aws.ToString(input.Message.Subject.Data) == "Test Subject" &&
			aws.ToString(input.Message.Body.Html.Data) == "<p>Hello</p>"
	})).Return(&ses.SendEmailOutput{
		MessageId: aws.String(expectedMsgID),
	}, nil)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test Subject",
		HTMLContent: "<p>Hello</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, expectedMsgID, msgID)
	client.AssertExpectations(t)
}

// TestSESProvider_Send_Success_FromNameEmpty tests when FromName is not configured.
func TestSESProvider_Send_Success_FromNameEmpty(t *testing.T) {
	cfg := &config.SESConfig{
		Region:      "us-east-1",
		FromAddress: "noreply@example.com",
		FromName:    "",
	}
	provider := service.NewSESProvider(cfg)
	client := new(mockSESClient)
	provider.SetSESClient(client)

	expectedMsgID := "ses-msg-def456"
	client.On("SendEmail", mock.Anything, mock.MatchedBy(func(input *ses.SendEmailInput) bool {
		return aws.ToString(input.Source) == "noreply@example.com"
	})).Return(&ses.SendEmailOutput{
		MessageId: aws.String(expectedMsgID),
	}, nil)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hello</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, expectedMsgID, msgID)
}

// TestSESProvider_Send_TextOnly tests sending with only plain text content.
func TestSESProvider_Send_TextOnly(t *testing.T) {
	provider := newTestSESProvider()
	client := new(mockSESClient)
	provider.SetSESClient(client)

	expectedMsgID := "ses-msg-text"
	client.On("SendEmail", mock.Anything, mock.MatchedBy(func(input *ses.SendEmailInput) bool {
		return input.Message.Body.Html == nil &&
			aws.ToString(input.Message.Body.Text.Data) == "Hello, World"
	})).Return(&ses.SendEmailOutput{
		MessageId: aws.String(expectedMsgID),
	}, nil)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Plain Text",
		TextContent: "Hello, World",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, expectedMsgID, msgID)
}

// TestSESProvider_Send_BothHTMLAndText tests sending with both HTML and text content.
func TestSESProvider_Send_BothHTMLAndText(t *testing.T) {
	provider := newTestSESProvider()
	client := new(mockSESClient)
	provider.SetSESClient(client)

	expectedMsgID := "ses-msg-multi"
	client.On("SendEmail", mock.Anything, mock.MatchedBy(func(input *ses.SendEmailInput) bool {
		return aws.ToString(input.Message.Body.Html.Data) == "<p>Rich</p>" &&
			aws.ToString(input.Message.Body.Text.Data) == "Plain"
	})).Return(&ses.SendEmailOutput{
		MessageId: aws.String(expectedMsgID),
	}, nil)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Multi-part",
		HTMLContent: "<p>Rich</p>",
		TextContent: "Plain",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, expectedMsgID, msgID)
}

// TestSESProvider_Send_CharsetSetCorrectly tests that UTF-8 charset is set.
func TestSESProvider_Send_CharsetSetCorrectly(t *testing.T) {
	provider := newTestSESProvider()
	client := new(mockSESClient)
	provider.SetSESClient(client)

	expectedMsgID := "ses-msg-charset"
	client.On("SendEmail", mock.Anything, mock.MatchedBy(func(input *ses.SendEmailInput) bool {
		return aws.ToString(input.Message.Subject.Charset) == "UTF-8" &&
			aws.ToString(input.Message.Body.Html.Charset) == "UTF-8"
	})).Return(&ses.SendEmailOutput{
		MessageId: aws.String(expectedMsgID),
	}, nil)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Charset Test",
		HTMLContent: "<p>Test</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, expectedMsgID, msgID)
}

// TestSESProvider_Send_NilEmail tests nil email handling.
func TestSESProvider_Send_NilEmail(t *testing.T) {
	provider := newTestSESProvider()

	ctx := context.Background()
	msgID, err := provider.Send(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "recipient email address is required")
}

// TestSESProvider_Send_MissingRecipient tests missing To field.
func TestSESProvider_Send_MissingRecipient(t *testing.T) {
	provider := newTestSESProvider()

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "recipient email address is required")
}

// TestSESProvider_Send_MissingContent tests that at least one content field is required.
func TestSESProvider_Send_MissingContent(t *testing.T) {
	provider := newTestSESProvider()

	ctx := context.Background()
	email := &service.EmailMessage{
		To:      "user@example.com",
		Subject: "Test",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "email body content is required")
}

// TestSESProvider_Send_AWSError tests that AWS errors are propagated correctly.
func TestSESProvider_Send_AWSError(t *testing.T) {
	provider := newTestSESProvider()
	client := new(mockSESClient)
	provider.SetSESClient(client)

	awsErr := &types.MessageRejected{}
	client.On("SendEmail", mock.Anything, mock.Anything).Return(nil, awsErr)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "failed to send email via SES")
	assert.ErrorIs(t, err, awsErr)
	client.AssertExpectations(t)
}

// TestSESProvider_Send_GenericAWSError tests a generic AWS error.
func TestSESProvider_Send_GenericAWSError(t *testing.T) {
	provider := newTestSESProvider()
	client := new(mockSESClient)
	provider.SetSESClient(client)

	genericErr := errors.New("network timeout")
	client.On("SendEmail", mock.Anything, mock.Anything).Return(nil, genericErr)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "failed to send email via SES")
	assert.Contains(t, err.Error(), "network timeout")
	client.AssertExpectations(t)
}

// TestSESProvider_Send_ConfigurationError tests when the SES client cannot be configured
// and the mock client is not injected (real AWS config loading would fail).
// This validates the lazy client initialization path.
func TestSESProvider_Send_ConfigurationError(t *testing.T) {
	cfg := &config.SESConfig{
		Region:      "invalid-region-name-12345",
		FromAddress: "noreply@example.com",
	}
	provider := service.NewSESProvider(cfg)

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "failed to load AWS config")
}
