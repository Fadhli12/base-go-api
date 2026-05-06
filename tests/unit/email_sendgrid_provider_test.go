package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSendGridProvider(baseURL string) *service.SendGridProvider {
	cfg := &config.EmailConfig{
		SendGridAPIKey:      "test-api-key",
		SendGridFromAddress: "noreply@example.com",
		SendGridFromName:    "Test App",
	}
	provider := service.NewSendGridProvider(cfg)
	provider.SetBaseURL(baseURL)
	provider.SetHTTPClient(&http.Client{Timeout: 5 * time.Second})
	return provider
}

// TestSendGridProvider_Name tests the Name method.
func TestSendGridProvider_Name(t *testing.T) {
	provider := newTestSendGridProvider("http://localhost")
	assert.Equal(t, "sendgrid", provider.Name())
}

// TestSendGridProvider_Send_Success tests a successful SendGrid API call.
func TestSendGridProvider_Send_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("X-Message-Id", "msg-abc123")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test Subject",
		HTMLContent: "<p>Hello</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, "msg-abc123", msgID)
}

// TestSendGridProvider_Send_Success_NoMessageID tests success when X-Message-Id is absent.
func TestSendGridProvider_Send_Success_NoMessageID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, "", msgID)
}

// TestSendGridProvider_Send_RateLimitThenSuccess tests that rate-limited requests are retried.
func TestSendGridProvider_Send_RateLimitThenSuccess(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("X-Message-Id", "msg-after-retry")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, "msg-after-retry", msgID)
	assert.Equal(t, 3, attempts, "expected 3 attempts (2 rate limits + 1 success)")
}

// TestSendGridProvider_Send_RateLimitExhausted tests that exhausting all rate-limit retries fails.
func TestSendGridProvider_Send_RateLimitExhausted(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "retries exhausted")
	// maxRateLimitRetries=3 → 1 initial + 3 retries = 4 total
	assert.Equal(t, 4, attempts)
}

// TestSendGridProvider_Send_APIError tests a non-429 error response.
func TestSendGridProvider_Send_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errors":[{"message":"Invalid email address"}]}`))
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "invalid",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "sendgrid API error")
	assert.Contains(t, err.Error(), "400")
}

// TestSendGridProvider_Send_APIError_Unauthorized tests a 401 response.
func TestSendGridProvider_Send_APIError_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errors":[{"message":"Permission denied"}]}`))
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "401")
}

// TestSendGridProvider_Send_APIError_ServerError tests a 500 response.
func TestSendGridProvider_Send_APIError_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
}

// TestSendGridProvider_Send_NetworkFailure tests a network-level failure (connection refused).
func TestSendGridProvider_Send_NetworkFailure(t *testing.T) {
	provider := newTestSendGridProvider("http://127.0.0.1:1")
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "failed to send email via SendGrid")
}

// TestSendGridProvider_Send_ContextCancelledDuringBackoff tests that context cancellation
// during rate-limit backoff is respected.
func TestSendGridProvider_Send_ContextCancelledDuringBackoff(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	// Cancel after a short delay to trigger during backoff
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.True(t, strings.Contains(err.Error(), "context") || strings.Contains(err.Error(), "canceled"),
		"expected context cancellation error, got: %v", err)
}

// TestSendGridProvider_Send_NilEmail tests nil email handling.
func TestSendGridProvider_Send_NilEmail(t *testing.T) {
	provider := newTestSendGridProvider("http://localhost")
	ctx := context.Background()

	msgID, err := provider.Send(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "email message is required")
}

// TestSendGridProvider_Send_MissingRecipient tests missing To field.
func TestSendGridProvider_Send_MissingRecipient(t *testing.T) {
	provider := newTestSendGridProvider("http://localhost")
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

// TestSendGridProvider_Send_MissingContent tests that at least one content field is required.
func TestSendGridProvider_Send_MissingContent(t *testing.T) {
	provider := newTestSendGridProvider("http://localhost")
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

// TestSendGridProvider_Send_WithTemplate tests sending with a template ID.
func TestSendGridProvider_Send_WithTemplate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Message-Id", "msg-template")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:       "user@example.com",
		Template: "d-abc123",
		Data: map[string]interface{}{
			"name": "John",
		},
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, "msg-template", msgID)
}

// TestSendGridProvider_Send_TextOnly tests sending with only plain text content.
func TestSendGridProvider_Send_TextOnly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Message-Id", "msg-text")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	provider := newTestSendGridProvider(ts.URL)
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Plain Text",
		TextContent: "Hello, World",
	}

	msgID, err := provider.Send(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, "msg-text", msgID)
}

// TestSendGridProvider_Send_MissingFromAddress tests that a configured from address is required.
func TestSendGridProvider_Send_MissingFromAddress(t *testing.T) {
	cfg := &config.EmailConfig{
		SendGridAPIKey:      "test-api-key",
		SendGridFromAddress: "",
		SendGridFromName:    "Test",
	}
	provider := service.NewSendGridProvider(cfg)
	provider.SetBaseURL("http://localhost")
	provider.SetHTTPClient(&http.Client{Timeout: 5 * time.Second})

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "from address is not configured")
}

// TestSendGridProvider_Send_MissingAPIKey tests that an API key is required.
func TestSendGridProvider_Send_MissingAPIKey(t *testing.T) {
	cfg := &config.EmailConfig{
		SendGridAPIKey:      "",
		SendGridFromAddress: "noreply@example.com",
	}
	provider := service.NewSendGridProvider(cfg)
	provider.SetBaseURL("http://localhost")
	provider.SetHTTPClient(&http.Client{Timeout: 5 * time.Second})

	ctx := context.Background()
	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	msgID, err := provider.Send(ctx, email)
	require.Error(t, err)
	assert.Equal(t, "", msgID)
	assert.Contains(t, err.Error(), "api key is not configured")
}

// TestSendGridProvider_Send_RequestHeaderValidation verifies the correct headers are sent.
func TestSendGridProvider_Send_RequestHeaderValidation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/v3/mail/send")

		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	// Use a URL that includes the SendGrid path to test URL construction
	provider := newTestSendGridProvider(ts.URL + "/v3/mail/send")
	ctx := context.Background()

	email := &service.EmailMessage{
		To:          "user@example.com",
		Subject:     "Test",
		HTMLContent: "<p>Hi</p>",
	}

	_, err := provider.Send(ctx, email)
	require.NoError(t, err)
}
