package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes for email queue
	EmailQueueKey       = "email:queue"
	EmailProcessingKey  = "email:processing"
	EmailDeadLetterKey  = "email:dead_letter"
	EmailProcessingSet   = "email:processing:set" // SET to track processing email IDs for efficient enumeration
)

// EmailQueueMessage represents a message in the Redis queue
type EmailQueueMessage struct {
	EmailID  string    `json:"email_id"`
	Priority int       `json:"priority"`
	QueuedAt time.Time `json:"queued_at"`
}

// RedisEmailQueue manages email queue operations via Redis
type RedisEmailQueue struct {
	client *redis.Client
}

// NewRedisEmailQueue creates a new Redis-based email queue
func NewRedisEmailQueue(client *redis.Client) *RedisEmailQueue {
	return &RedisEmailQueue{client: client}
}

// Enqueue adds an email to the processing queue
func (q *RedisEmailQueue) Enqueue(ctx context.Context, emailID string, priority int) error {
	msg := EmailQueueMessage{
		EmailID:  emailID,
		Priority: priority,
		QueuedAt: time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal queue message: %w", err)
	}

	// Use ZADD with priority score (lower = higher priority)
	score := float64(priority) + float64(time.Now().UnixNano())/1e18
	if err := q.client.ZAdd(ctx, EmailQueueKey, redis.Z{
		Score:  score,
		Member: string(data),
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue email: %w", err)
	}

	return nil
}

// Dequeue retrieves the next email from the queue
func (q *RedisEmailQueue) Dequeue(ctx context.Context) (*EmailQueueMessage, error) {
	// Get the lowest score (highest priority)
	result, err := q.client.ZPopMin(ctx, EmailQueueKey, 1).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to dequeue email: %w", err)
	}

	if len(result) == 0 {
		return nil, nil // No messages available
	}

	var msg EmailQueueMessage
	if err := json.Unmarshal([]byte(result[0].Member.(string)), &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue message: %w", err)
	}

	return &msg, nil
}

// MoveToProcessing moves an email to the processing set
func (q *RedisEmailQueue) MoveToProcessing(ctx context.Context, emailID string, workerID string, timeout time.Duration) error {
	key := fmt.Sprintf("%s:%s", EmailProcessingKey, emailID)
	data := map[string]interface{}{
		"worker_id":  workerID,
		"started_at": time.Now().Unix(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal processing data: %w", err)
	}

	// Use pipeline to set processing key AND add to tracking set
	pipe := q.client.Pipeline()
	pipe.Set(ctx, key, jsonData, timeout)
	pipe.SAdd(ctx, EmailProcessingSet, emailID)
	_, err = pipe.Exec(ctx)
	return err
}

// RemoveFromProcessing removes an email from the processing set
func (q *RedisEmailQueue) RemoveFromProcessing(ctx context.Context, emailID string) error {
	key := fmt.Sprintf("%s:%s", EmailProcessingKey, emailID)

	// Use pipeline to remove processing key AND remove from tracking set
	pipe := q.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.SRem(ctx, EmailProcessingSet, emailID)
	_, err := pipe.Exec(ctx)
	return err
}

// MoveToDeadLetter moves a failed email to the dead letter queue
func (q *RedisEmailQueue) MoveToDeadLetter(ctx context.Context, emailID string, reason string) error {
	data := map[string]interface{}{
		"email_id":    emailID,
		"reason":      reason,
		"failed_at":   time.Now().Unix(),
	}

	jsonData, _ := json.Marshal(data)
	return q.client.RPush(ctx, EmailDeadLetterKey, jsonData).Err()
}

// GetQueueLength returns the number of emails in the queue
func (q *RedisEmailQueue) GetQueueLength(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, EmailQueueKey).Result()
}

// GetProcessingLength returns the number of emails being processed
func (q *RedisEmailQueue) GetProcessingLength(ctx context.Context) (int64, error) {
	// Use SCARD on tracking set instead of KEYS command (O(1) instead of O(N))
	return q.client.SCard(ctx, EmailProcessingSet).Result()
}

// GetDeadLetterLength returns the number of emails in dead letter queue
func (q *RedisEmailQueue) GetDeadLetterLength(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, EmailDeadLetterKey).Result()
}

// RequeueTimedOut finds processing emails that have timed out and requeues them
func (q *RedisEmailQueue) RequeueTimedOut(ctx context.Context, timeout time.Duration) (int64, error) {
	// Use SMEMBERS on tracking set instead of KEYS (more efficient)
	emailIDs, err := q.client.SMembers(ctx, EmailProcessingSet).Result()
	if err != nil {
		return 0, err
	}

	var requeued int64
	cutoff := time.Now().Add(-timeout).Unix()

	for _, emailID := range emailIDs {
		key := fmt.Sprintf("%s:%s", EmailProcessingKey, emailID)
		data, err := q.client.Get(ctx, key).Result()
		if err != nil {
			// Key might have expired or been removed, clean up set
			q.client.SRem(ctx, EmailProcessingSet, emailID)
			continue
		}

		var processing struct {
			WorkerID  string `json:"worker_id"`
			StartedAt int64  `json:"started_at"`
		}

		if err := json.Unmarshal([]byte(data), &processing); err != nil {
			continue
		}

		if processing.StartedAt < cutoff {
			// Requeue the email
			if err := q.Enqueue(ctx, emailID, 1); err != nil {
				continue
			}
			// Remove from processing set and individual key
			pipe := q.client.Pipeline()
			pipe.Del(ctx, key)
			pipe.SRem(ctx, EmailProcessingSet, emailID)
			pipe.Exec(ctx)
			requeued++
		}
	}

	return requeued, nil
}

// PurgeQueue removes all emails from the queue (use with caution)
func (q *RedisEmailQueue) PurgeQueue(ctx context.Context) error {
	return q.client.Del(ctx, EmailQueueKey).Err()
}

// PurgeDeadLetter removes all emails from the dead letter queue
func (q *RedisEmailQueue) PurgeDeadLetter(ctx context.Context) error {
	return q.client.Del(ctx, EmailDeadLetterKey).Err()
}