package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/config"
)

const (
	sendgridAPIURL = "https://api.sendgrid.com/v3/mail/send"
	maxRateLimitRetries = 3
	baseBackoff = 1 * time.Second
)

type sendgridRequest struct {
	Personalizations []sendgridPersonalization `json:"personalizations"`
	From             sendgridAddress            `json:"from"`
	Subject          string                     `json:"subject,omitempty"`
	Content          []sendgridContent          `json:"content,omitempty"`
	TemplateID       string                     `json:"template_id,omitempty"`
}

type sendgridPersonalization struct {
	To                  []sendgridAddress       `json:"to"`
	Subject             string                  `json:"subject,omitempty"`
	DynamicTemplateData map[string]interface{}  `json:"dynamic_template_data,omitempty"`
	Substitutions       map[string]interface{}  `json:"substitutions,omitempty"`
}

type sendgridAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendgridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type SendGridProvider struct {
	apiKey     string
	fromAddr   string
	fromName   string
	httpClient *http.Client
	baseURL    string
}

// NewSendGridProvider creates a new SendGrid email provider from the shared EmailConfig.
func NewSendGridProvider(cfg *config.EmailConfig) *SendGridProvider {
	return &SendGridProvider{
		apiKey:     cfg.SendGridAPIKey,
		fromAddr:   cfg.SendGridFromAddress,
		fromName:   cfg.SendGridFromName,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		baseURL: sendgridAPIURL,
	}
}

// SetHTTPClient sets a custom HTTP client for testing.
func (p *SendGridProvider) SetHTTPClient(client *http.Client) {
	p.httpClient = client
}

// SetBaseURL sets a custom base URL for testing.
func (p *SendGridProvider) SetBaseURL(url string) {
	p.baseURL = url
}

func (p *SendGridProvider) Send(ctx context.Context, email *EmailMessage) (string, error) {
	if email == nil {
		return "", fmt.Errorf("email message is required")
	}

	if err := p.validateEmail(email); err != nil {
		return "", err
	}

	payload := p.buildPayload(email)

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sendgrid request: %w", err)
	}

	var lastErr error
	var messageID string

	for attempt := 0; attempt <= maxRateLimitRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * baseBackoff
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("failed to create sendgrid request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to send email via SendGrid: %w", err)
		}

		messageID = resp.Header.Get("X-Message-Id")

		switch {
		case resp.StatusCode == http.StatusAccepted:
			resp.Body.Close()
			return messageID, nil

		case resp.StatusCode == http.StatusTooManyRequests:
			resp.Body.Close()
			lastErr = fmt.Errorf("sendgrid rate limited (attempt %d/%d)", attempt+1, maxRateLimitRetries+1)
			continue

		default:
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("sendgrid API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
	}

	return "", fmt.Errorf("sendgrid retries exhausted: %w", lastErr)
}

func (p *SendGridProvider) Name() string {
	return "sendgrid"
}

func (p *SendGridProvider) validateEmail(email *EmailMessage) error {
	if email.To == "" {
		return fmt.Errorf("recipient email address is required")
	}

	if p.fromAddr == "" {
		return fmt.Errorf("sendgrid from address is not configured")
	}

	if p.apiKey == "" {
		return fmt.Errorf("sendgrid api key is not configured")
	}

	if email.Template == "" && email.HTMLContent == "" && email.TextContent == "" {
		return fmt.Errorf("email body content is required (template, html, or text)")
	}

	return nil
}

func (p *SendGridProvider) buildPayload(email *EmailMessage) *sendgridRequest {
	payload := &sendgridRequest{
		From: sendgridAddress{
			Email: p.fromAddr,
			Name:  p.fromName,
		},
	}

	personalization := sendgridPersonalization{
		To: []sendgridAddress{{Email: email.To}},
	}

	// When a template is set, use dynamic_template_data for substitutions.
	if email.Template != "" {
		payload.TemplateID = email.Template
		if len(email.Data) > 0 {
			personalization.DynamicTemplateData = email.Data
		}
		// Template-driven sends omit top-level subject; the template defines it.
		// However, the API allows overriding subject per personalization.
		if email.Subject != "" {
			personalization.Subject = email.Subject
		}
	} else {
		payload.Subject = email.Subject
	}

	// Build content array for non-template sends.
	if email.Template == "" {
		payload.Content = make([]sendgridContent, 0, 2)
		if email.TextContent != "" {
			payload.Content = append(payload.Content, sendgridContent{
				Type:  "text/plain",
				Value: email.TextContent,
			})
		}
		if email.HTMLContent != "" {
			payload.Content = append(payload.Content, sendgridContent{
				Type:  "text/html",
				Value: email.HTMLContent,
			})
		}
	}

	payload.Personalizations = []sendgridPersonalization{personalization}
	return payload
}
