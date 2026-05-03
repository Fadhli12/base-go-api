package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/google/uuid"
)

// WebhookWorker processes webhook deliveries asynchronously.
type WebhookWorker struct {
	config       *config.WebhookConfig
	deliveryRepo repository.WebhookDeliveryRepository
	webhookRepo  repository.WebhookRepository
	redisQueue   WebhookQueue
	rateLimiter  WebhookRateLimiterInterface

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics (thread-safe using atomic)
	deliveredCount   atomic.Int64
	failedCount      atomic.Int64
	rateLimitedCount atomic.Int64
}

// NewWebhookWorker creates a new WebhookWorker instance.
func NewWebhookWorker(
	config *config.WebhookConfig,
	deliveryRepo repository.WebhookDeliveryRepository,
	webhookRepo repository.WebhookRepository,
	redisQueue WebhookQueue,
	rateLimiter WebhookRateLimiterInterface,
) *WebhookWorker {
	return &WebhookWorker{
		config:       config,
		deliveryRepo: deliveryRepo,
		webhookRepo:  webhookRepo,
		redisQueue:   redisQueue,
		rateLimiter:  rateLimiter,
	}
}

// Start begins the worker goroutine pool and background maintenance tasks.
func (w *WebhookWorker) Start() {
	w.ctx, w.cancel = context.WithCancel(context.Background())

	// Start worker goroutines
	for i := 0; i < w.config.WorkerConcurrency; i++ {
		w.wg.Add(1)
		go w.processLoop()
	}

	// Start background maintenance goroutines
	w.wg.Add(2)
	go w.stuckDeliveryReaper()
	go w.retentionCleanup()

	slog.Info("Webhook worker started",
		"workers", w.config.WorkerConcurrency,
		"retry_max", w.config.RetryMax,
	)
}

// Stop gracefully shuts down the worker.
func (w *WebhookWorker) Stop() {
	slog.Info("Stopping webhook worker...")
	w.cancel()

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Webhook worker stopped gracefully",
			"delivered", w.deliveredCount.Load(),
			"failed", w.failedCount.Load(),
			"rate_limited", w.rateLimitedCount.Load(),
		)
	case <-time.After(30 * time.Second):
		slog.Error("Webhook worker shutdown timeout")
	}
}

// Metrics returns current worker metrics.
func (w *WebhookWorker) Metrics() map[string]int64 {
	return map[string]int64{
		"delivered":    w.deliveredCount.Load(),
		"failed":       w.failedCount.Load(),
		"rate_limited": w.rateLimitedCount.Load(),
	}
}

// processLoop is the main worker loop that dequeues and processes deliveries.
func (w *WebhookWorker) processLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("Webhook worker process loop stopping")
			return
		case <-ticker.C:
			msg, err := w.redisQueue.Dequeue(w.ctx)
			if err != nil {
				slog.Error("Failed to dequeue webhook delivery", "error", err)
				continue
			}
			if msg == nil {
				continue
			}

			deliveryID, parseErr := uuid.Parse(msg.DeliveryID)
			if parseErr != nil {
				slog.Error("Invalid delivery ID in queue message",
					"delivery_id", msg.DeliveryID,
					"error", parseErr,
				)
				_ = w.redisQueue.RemoveFromProcessing(w.ctx, msg.DeliveryID)
				continue
			}

			if err := w.processDelivery(deliveryID); err != nil {
				slog.Error("Failed to process webhook delivery",
					"delivery_id", deliveryID,
					"error", err,
				)
			}
		}
	}
}

// processDelivery handles a single webhook delivery end-to-end.
func (w *WebhookWorker) processDelivery(deliveryID uuid.UUID) error {
	ctx := w.ctx

	// Ensure delivery is removed from processing set when done
	defer func() {
		if err := w.redisQueue.RemoveFromProcessing(ctx, deliveryID.String()); err != nil {
			slog.Error("Failed to remove delivery from processing set",
				"delivery_id", deliveryID,
				"error", err,
			)
		}
	}()

	// Fetch the delivery record
	delivery, err := w.deliveryRepo.FindByID(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("failed to find delivery: %w", err)
	}

	// Mark as processing
	now := time.Now()
	if err := w.deliveryRepo.UpdateStatus(ctx, deliveryID, domain.WebhookDeliveryStatusProcessing, map[string]interface{}{
		"processing_started_at": now,
	}); err != nil {
		return fmt.Errorf("failed to mark delivery as processing: %w", err)
	}

	slog.Info("Processing webhook delivery",
		"delivery_id", deliveryID,
		"webhook_id", delivery.WebhookID,
		"event", delivery.Event,
		"attempt", delivery.AttemptNumber,
	)

	// Fetch webhook configuration
	webhook, err := w.webhookRepo.FindByID(ctx, delivery.WebhookID)
	if err != nil {
		_ = w.markFailedAndMaybeRetry(ctx, delivery, fmt.Errorf("webhook lookup failed: %w", err), nil)
		return fmt.Errorf("failed to find webhook: %w", err)
	}

	if !webhook.IsActive() {
		_ = w.markFailedAndMaybeRetry(ctx, delivery, fmt.Errorf("webhook is inactive"), nil)
		return nil
	}

	// Check rate limit
	allowed, _, _ := w.rateLimiter.Allow(ctx, webhook.ID.String())
	if !allowed {
		w.rateLimitedCount.Add(1)
		if delivery.CanRetry() {
			nextRetryAt := now.Add(retryBackoff(delivery.AttemptNumber))
			if err := w.deliveryRepo.UpdateStatus(ctx, deliveryID, domain.WebhookDeliveryStatusRateLimited, map[string]interface{}{
				"attempt_number": delivery.AttemptNumber + 1,
				"next_retry_at":  nextRetryAt,
			}); err != nil {
				slog.Error("Failed to mark delivery as rate limited",
					"delivery_id", deliveryID,
					"error", err,
				)
			} else if err := w.redisQueue.EnqueueRetry(ctx, deliveryID.String(), nextRetryAt); err != nil {
				slog.Error("Failed to enqueue rate-limited retry",
					"delivery_id", deliveryID,
					"error", err,
				)
			}
			slog.Warn("Webhook delivery rate limited, retry scheduled",
				"delivery_id", deliveryID,
				"next_retry_at", nextRetryAt,
			)
		} else {
			_ = w.deliveryRepo.UpdateStatus(ctx, deliveryID, domain.WebhookDeliveryStatusRateLimited, map[string]interface{}{
				"last_error": "rate limited and max attempts reached",
			})
			slog.Error("Webhook delivery rate limited, max retries reached",
				"delivery_id", deliveryID,
			)
		}
		return nil
	}

	// Build request body (truncate if needed)
	payload := []byte(delivery.Payload)
	if len(payload) > w.config.MaxPayloadSize {
		payload = payload[:w.config.MaxPayloadSize]
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(payload))
	if err != nil {
		_ = w.markFailedAndMaybeRetry(ctx, delivery, fmt.Errorf("failed to build request: %w", err), nil)
		return fmt.Errorf("failed to build request: %w", err)
	}

	timestamp := time.Now().Unix()
	signature := signWebhookPayload(webhook.Secret, timestamp, payload)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", delivery.Event)
	req.Header.Set("X-Webhook-Delivery-ID", deliveryID.String())
	req.Header.Set("X-Webhook-Signature", signature)

	// Execute HTTP request
	client := w.newHTTPClient()
	start := time.Now()
	resp, err := client.Do(req)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		_ = w.markFailedAndMaybeRetry(ctx, delivery, fmt.Errorf("delivery request failed: %w", err), map[string]interface{}{
			"duration_ms": durationMs,
		})
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	responseCode := resp.StatusCode
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	responseBody := string(body)

	if responseCode >= 200 && responseCode < 300 {
		// Success
		deliveredAt := time.Now()
		if err := w.deliveryRepo.UpdateStatus(ctx, deliveryID, domain.WebhookDeliveryStatusDelivered, map[string]interface{}{
			"response_code": responseCode,
			"response_body": responseBody,
			"duration_ms":   durationMs,
			"delivered_at":  deliveredAt,
		}); err != nil {
			slog.Error("Failed to mark delivery as delivered",
				"delivery_id", deliveryID,
				"error", err,
			)
		}
		w.deliveredCount.Add(1)
		slog.Info("Webhook delivered",
			"delivery_id", deliveryID,
			"webhook_id", webhook.ID,
			"status_code", responseCode,
			"duration_ms", durationMs,
		)
		return nil
	}

	// Non-2xx response
	err = fmt.Errorf("webhook returned non-2xx status: %d", responseCode)
	_ = w.markFailedAndMaybeRetry(ctx, delivery, err, map[string]interface{}{
		"response_code": responseCode,
		"response_body": responseBody,
		"duration_ms":   durationMs,
	})
	return nil
}

// markFailedAndMaybeRetry marks a delivery as failed and schedules a retry if allowed.
func (w *WebhookWorker) markFailedAndMaybeRetry(ctx context.Context, delivery *domain.WebhookDelivery, err error, extraUpdates map[string]interface{}) error {
	deliveryID := delivery.ID
	lastError := err.Error()

	updates := map[string]interface{}{
		"last_error": lastError,
	}
	for k, v := range extraUpdates {
		updates[k] = v
	}

	if updateErr := w.deliveryRepo.UpdateStatus(ctx, deliveryID, domain.WebhookDeliveryStatusFailed, updates); updateErr != nil {
		slog.Error("Failed to mark delivery as failed",
			"delivery_id", deliveryID,
			"error", updateErr,
		)
	}

	w.failedCount.Add(1)

	if delivery.CanRetry() {
		nextRetryAt := time.Now().Add(retryBackoff(delivery.AttemptNumber))
		retryUpdates := map[string]interface{}{
			"attempt_number": delivery.AttemptNumber + 1,
			"next_retry_at":  nextRetryAt,
			"last_error":     lastError,
		}
		if updateErr := w.deliveryRepo.UpdateStatus(ctx, deliveryID, domain.WebhookDeliveryStatusQueued, retryUpdates); updateErr != nil {
			slog.Error("Failed to schedule retry",
				"delivery_id", deliveryID,
				"error", updateErr,
			)
			return updateErr
		}
		if enqueueErr := w.redisQueue.EnqueueRetry(ctx, deliveryID.String(), nextRetryAt); enqueueErr != nil {
			slog.Error("Failed to enqueue retry",
				"delivery_id", deliveryID,
				"error", enqueueErr,
			)
			return enqueueErr
		}
		slog.Warn("Webhook delivery failed, retry scheduled",
			"delivery_id", deliveryID,
			"attempt", delivery.AttemptNumber+1,
			"next_retry_at", nextRetryAt,
		)
	} else {
		slog.Error("Webhook delivery failed, max retries reached",
			"delivery_id", deliveryID,
			"attempts", delivery.AttemptNumber,
			"error", lastError,
		)
	}

	return nil
}

// stuckDeliveryReaper finds and recovers deliveries stuck in processing.
func (w *WebhookWorker) stuckDeliveryReaper() {
	defer w.wg.Done()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("Webhook stuck delivery reaper stopping")
			return
		case <-ticker.C:
			stuckDeliveries, err := w.deliveryRepo.FindStuckProcessing(w.ctx, w.config.DeliveryTimeout*2)
			if err != nil {
				slog.Error("Failed to find stuck deliveries", "error", err)
				continue
			}

			for _, d := range stuckDeliveries {
				if err := w.deliveryRepo.UpdateStatus(w.ctx, d.ID, domain.WebhookDeliveryStatusFailed, map[string]interface{}{
					"last_error": "processing timeout: worker recovery",
				}); err != nil {
					slog.Error("Failed to mark stuck delivery as failed",
						"delivery_id", d.ID,
						"error", err,
					)
					continue
				}

				if d.CanRetry() {
					nextRetryAt := time.Now().Add(retryBackoff(d.AttemptNumber))
					retryUpdates := map[string]interface{}{
						"attempt_number": d.AttemptNumber + 1,
						"next_retry_at":  nextRetryAt,
					}
					if err := w.deliveryRepo.UpdateStatus(w.ctx, d.ID, domain.WebhookDeliveryStatusQueued, retryUpdates); err != nil {
						slog.Error("Failed to schedule retry for stuck delivery",
							"delivery_id", d.ID,
							"error", err,
						)
						continue
					}
					if err := w.redisQueue.EnqueueRetry(w.ctx, d.ID.String(), nextRetryAt); err != nil {
						slog.Error("Failed to enqueue stuck delivery retry",
							"delivery_id", d.ID,
							"error", err,
						)
						continue
					}
					slog.Warn("Stuck webhook delivery recovered and requeued",
						"delivery_id", d.ID,
						"next_retry_at", nextRetryAt,
					)
				} else {
					slog.Error("Stuck webhook delivery marked as failed, max retries reached",
						"delivery_id", d.ID,
					)
				}
			}
		}
	}
}

// retentionCleanup removes old webhook delivery records.
func (w *WebhookWorker) retentionCleanup() {
	defer w.wg.Done()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("Webhook retention cleanup stopping")
			return
		case <-ticker.C:
			if err := w.deliveryRepo.DeleteOlderThan(w.ctx, w.config.DeliveryRetentionDays); err != nil {
				slog.Error("Failed to delete old webhook deliveries", "error", err)
			} else {
				slog.Info("Webhook retention cleanup completed")
			}
		}
	}
}

// newHTTPClient creates an HTTP client with SSRF protection.
func (w *WebhookWorker) newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: w.config.DeliveryTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, _ := net.SplitHostPort(addr)
				ip := net.ParseIP(host)
				if ip != nil && isPrivateIP(ip) {
					return nil, fmt.Errorf("SSRF blocked: private IP %s", host)
				}
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
}

// retryBackoff calculates the retry delay based on attempt number.
func retryBackoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 1 * time.Minute
	case 2:
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

