package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes for webhook delivery queue
	WebhookQueueKey      = "webhook:deliveries"
	WebhookProcessingKey = "webhook:processing"
)

// WebhookRedisQueue manages webhook delivery queue operations via Redis
type WebhookRedisQueue struct {
	client *redis.Client
}

// NewWebhookRedisQueue creates a new Redis-based webhook delivery queue
func NewWebhookRedisQueue(client *redis.Client) *WebhookRedisQueue {
	return &WebhookRedisQueue{client: client}
}

// Enqueue adds a delivery to the queue for immediate processing
func (q *WebhookRedisQueue) Enqueue(ctx context.Context, deliveryID string) error {
	msg := WebhookQueueMessage{
		DeliveryID: deliveryID,
		QueuedAt:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal queue message: %w", err)
	}

	score := float64(time.Now().Unix())
	if err := q.client.ZAdd(ctx, WebhookQueueKey, redis.Z{
		Score:  score,
		Member: string(data),
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue webhook delivery: %w", err)
	}

	return nil
}

// Dequeue retrieves the next delivery due for processing
func (q *WebhookRedisQueue) Dequeue(ctx context.Context) (*WebhookQueueMessage, error) {
	now := float64(time.Now().Unix())
	maxScore := fmt.Sprintf("%f", now)

	// Find the next due delivery
	results, err := q.client.ZRangeByScore(ctx, WebhookQueueKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   maxScore,
		Count: 1,
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to range webhook queue: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // No messages available
	}

	member := results[0]

	// Atomically remove the member; if count is 0, another worker took it
	removed, err := q.client.ZRem(ctx, WebhookQueueKey, member).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to remove webhook from queue: %w", err)
	}
	if removed == 0 {
		return nil, nil // Another worker processed this item
	}

	// Add delivery ID to processing set
	var msg WebhookQueueMessage
	if err := json.Unmarshal([]byte(member), &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue message: %w", err)
	}

	if err := q.client.SAdd(ctx, WebhookProcessingKey, msg.DeliveryID).Err(); err != nil {
		return nil, fmt.Errorf("failed to add to processing set: %w", err)
	}

	return &msg, nil
}

// EnqueueRetry schedules a retry for future delivery
func (q *WebhookRedisQueue) EnqueueRetry(ctx context.Context, deliveryID string, retryAt time.Time) error {
	msg := WebhookQueueMessage{
		DeliveryID: deliveryID,
		QueuedAt:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal queue message: %w", err)
	}

	score := float64(retryAt.Unix())
	if err := q.client.ZAdd(ctx, WebhookQueueKey, redis.Z{
		Score:  score,
		Member: string(data),
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue webhook retry: %w", err)
	}

	return nil
}

// RemoveFromProcessing removes a delivery from the processing set after completion
func (q *WebhookRedisQueue) RemoveFromProcessing(ctx context.Context, deliveryID string) error {
	if err := q.client.SRem(ctx, WebhookProcessingKey, deliveryID).Err(); err != nil {
		return fmt.Errorf("failed to remove from processing set: %w", err)
	}
	return nil
}

// GetNextBatch returns a paginated scan of due deliveries without removing them
func (q *WebhookRedisQueue) GetNextBatch(ctx context.Context, limit int) ([]*WebhookQueueMessage, error) {
	now := float64(time.Now().Unix())
	maxScore := fmt.Sprintf("%f", now)

	results, err := q.client.ZRangeByScore(ctx, WebhookQueueKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   maxScore,
		Count: int64(limit),
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to range webhook queue: %w", err)
	}

	messages := make([]*WebhookQueueMessage, 0, len(results))
	for _, member := range results {
		var msg WebhookQueueMessage
		if err := json.Unmarshal([]byte(member), &msg); err != nil {
			continue
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// QueueSize returns the number of items in the queue ZSET
func (q *WebhookRedisQueue) QueueSize(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, WebhookQueueKey).Result()
}

// ProcessingSize returns the number of items in the processing set
func (q *WebhookRedisQueue) ProcessingSize(ctx context.Context) (int64, error) {
	return q.client.SCard(ctx, WebhookProcessingKey).Result()
}
