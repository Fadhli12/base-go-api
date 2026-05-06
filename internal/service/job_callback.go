package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// JobCallbackPayload is the JSON payload sent to the webhook URL.
type JobCallbackPayload struct {
	JobID        string    `json:"job_id"`
	Status       string    `json:"status"`         // "completed" or "failed"
	Result       []byte    `json:"result,omitempty"`
	Error        string    `json:"error,omitempty"`
	AttemptCount int       `json:"attempt_count"`
	CompletedAt  time.Time `json:"completed_at"`
}

// JobCallbackService handles HTTP callbacks to job WebhookURLs.
type JobCallbackService struct {
	client  *http.Client
	timeout time.Duration
}

// NewJobCallbackService creates a callback service with the given timeout.
func NewJobCallbackService(timeout time.Duration) *JobCallbackService {
	return &JobCallbackService{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// Deliver sends the callback payload to the webhook URL.
// Returns nil on success, error on failure.
// Failures are non-retryable - callback delivery is best-effort.
func (s *JobCallbackService) Deliver(ctx context.Context, webhookURL string, payload *JobCallbackPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("callback: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("callback: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoAPI-JobCallback/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("callback: deliver to %s: %w", webhookURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("callback: webhook returned status %d", resp.StatusCode)
	}

	return nil
}