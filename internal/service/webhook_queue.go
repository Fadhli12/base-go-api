package service

import (
	"context"
	"time"
)

// WebhookQueue defines the interface for webhook delivery queue operations.
// Implementations must be safe for concurrent use.
type WebhookQueue interface {
	// Enqueue adds a delivery to the queue for immediate processing.
	Enqueue(ctx context.Context, deliveryID string) error

	// Dequeue retrieves the next due delivery from the queue.
	// Returns nil, nil if no items are available.
	Dequeue(ctx context.Context) (*WebhookQueueMessage, error)

	// EnqueueRetry schedules a delivery for retry at the specified time.
	EnqueueRetry(ctx context.Context, deliveryID string, retryAt time.Time) error

	// RemoveFromProcessing removes a delivery from the processing set after completion.
	RemoveFromProcessing(ctx context.Context, deliveryID string) error

	// GetNextBatch returns up to limit due deliveries without removing them.
	GetNextBatch(ctx context.Context, limit int) ([]*WebhookQueueMessage, error)

	// QueueSize returns the number of items waiting in the queue.
	QueueSize(ctx context.Context) (int64, error)

	// ProcessingSize returns the number of items currently being processed.
	ProcessingSize(ctx context.Context) (int64, error)
}

// WebhookQueueMessage represents a message in the webhook delivery queue.
type WebhookQueueMessage struct {
	DeliveryID string    `json:"delivery_id"`
	QueuedAt   time.Time `json:"queued_at"`
}