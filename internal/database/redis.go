// Package database provides Redis connection management.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/redis/go-redis/v9"
)

// RedisClient wraps a Redis client with additional functionality.
type RedisClient struct {
	Client *redis.Client
}

// NewRedisClient creates a new Redis client connection.
// It configures the connection pool and validates the connection.
// Returns the Redis client or an error if connection fails.
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr(),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     100,
		MinIdleConns: 10,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	// Verify connection with a ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	slog.Info("Connected to Redis",
		"host", cfg.Redis.Host,
		"port", cfg.Redis.Port,
		"db", cfg.Redis.DB,
		"pool_size", 100,
	)

	return client, nil
}

// Close closes the Redis client connection.
func CloseRedis(client *redis.Client) error {
	if err := client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis connection: %w", err)
	}

	slog.Info("Redis connection closed")
	return nil
}

// RedisHealthCheck performs a health check on the Redis connection.
// Returns nil if healthy, otherwise returns an error.
func RedisHealthCheck(client *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}

// RedisPubSub provides Redis pub/sub functionality for cache invalidation.
type RedisPubSub struct {
	client *redis.Client
}

// NewRedisPubSub creates a new Redis pub/sub handler.
func NewRedisPubSub(client *redis.Client) *RedisPubSub {
	return &RedisPubSub{client: client}
}

// Publish publishes a message to a channel.
func (ps *RedisPubSub) Publish(ctx context.Context, channel string, message string) error {
	return ps.client.Publish(ctx, channel, message).Err()
}

// Subscribe subscribes to channels and returns a channel for messages.
func (ps *RedisPubSub) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return ps.client.Subscribe(ctx, channels...)
}


