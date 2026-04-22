// Package permission provides cache invalidation functionality.
package permission

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// InvalidationChannel is the Redis channel used for permission cache invalidation.
// All API instances subscribe to this channel for real-time cache synchronization.
// When permissions change, the invalidator broadcasts to all instances < 100ms.
const InvalidationChannel = "permission:invalidate"

// Invalidator handles permission cache invalidation via Redis pub/sub.
type Invalidator struct {
	client *redis.Client
}

// NewInvalidator creates a new invalidator with the given Redis client.
func NewInvalidator(client *redis.Client) *Invalidator {
	return &Invalidator{
		client: client,
	}
}

// PublishInvalidate publishes an invalidation message to all listeners.
// This triggers all connected instances to clear their permission caches.
func (i *Invalidator) PublishInvalidate(ctx context.Context) error {
	if err := i.client.Publish(ctx, InvalidationChannel, "invalidate").Err(); err != nil {
		return fmt.Errorf("failed to publish invalidation message: %w", err)
	}
	slog.Debug("Permission cache invalidation published")
	return nil
}

// SubscribeInvalidation subscribes to the invalidation channel.
// The onInvalidate callback is called whenever an invalidation message is received.
func (i *Invalidator) SubscribeInvalidation(ctx context.Context, onInvalidate func()) error {
	pubsub := i.client.Subscribe(ctx, InvalidationChannel)
	defer func() {
		if err := pubsub.Close(); err != nil {
			slog.Error("Failed to close pubsub", "error", err.Error())
		}
	}()

	// Get the channel for messages
	ch := pubsub.Channel()

	slog.Info("Subscribed to permission invalidation channel", "channel", InvalidationChannel)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Permission invalidation subscription cancelled")
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				slog.Info("Permission invalidation channel closed")
				return nil
			}
			slog.Debug("Permission invalidation message received",
				"channel", msg.Channel,
				"payload", msg.Payload)
			if onInvalidate != nil {
				onInvalidate()
			}
		}
	}
}

// StartInvalidationListener starts a goroutine that listens for invalidation messages.
// On receiving an invalidation message, it calls cache.InvalidateAll and enforcer.LoadPolicy.
// Returns an error channel that will receive errors if the listener fails.
func (i *Invalidator) StartInvalidationListener(ctx context.Context, cache *Cache, enforcer *Enforcer) <-chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		onInvalidate := func() {
			slog.Info("Processing permission cache invalidation")

			// Invalidate Redis cache
			if cache != nil {
				if err := cache.InvalidateAll(ctx); err != nil {
					slog.Error("Failed to invalidate permission cache", "error", err.Error())
				}
			}

			// Reload policy from database
			if enforcer != nil {
				if err := enforcer.LoadPolicy(); err != nil {
					slog.Error("Failed to reload policy", "error", err.Error())
				}
			}

			slog.Info("Permission cache invalidation complete")
		}

		if err := i.SubscribeInvalidation(ctx, onInvalidate); err != nil {
			if err != context.Canceled {
				errChan <- err
			}
		}
	}()

	return errChan
}

// InvalidateUser publishes an invalidation for a specific user's cache.
// This is a convenience method that invalidates the user's cache via Redis.
func (i *Invalidator) InvalidateUser(ctx context.Context, userID string) error {
	// Publish general invalidation for simplicity
	// A more optimized approach would publish user-specific invalidation
	return i.PublishInvalidate(ctx)
}

// InvalidateDomain publishes an invalidation for a specific domain's cache.
// This is a convenience method that invalidates the domain's cache via Redis.
func (i *Invalidator) InvalidateDomain(ctx context.Context, domain string) error {
	// Publish general invalidation for simplicity
	return i.PublishInvalidate(ctx)
}

// InvalidateAndReload invalidates cache and reloads policy on this instance.
// This is useful for immediate local invalidation without pub/sub.
func (i *Invalidator) InvalidateAndReload(ctx context.Context, cache *Cache, enforcer *Enforcer) error {
	// Invalidate Redis cache
	if cache != nil {
		if err := cache.InvalidateAll(ctx); err != nil {
			return fmt.Errorf("failed to invalidate cache: %w", err)
		}
	}

	// Reload policy from database
	if enforcer != nil {
		if err := enforcer.LoadPolicy(); err != nil {
			return fmt.Errorf("failed to reload policy: %w", err)
		}
	}

	slog.Info("Permission cache invalidated and policy reloaded")
	return nil
}

// BroadcastInvalidation invalidates cache locally and broadcasts to other instances.
// Call this after permission changes to ensure all instances have fresh data.
func (i *Invalidator) BroadcastInvalidation(ctx context.Context, cache *Cache, enforcer *Enforcer) error {
	// Invalidate local cache first
	if cache != nil {
		if err := cache.InvalidateAll(ctx); err != nil {
			slog.Error("Failed to invalidate local cache", "error", err.Error())
		}
	}

	// Reload local policy
	if enforcer != nil {
		if err := enforcer.LoadPolicy(); err != nil {
			slog.Error("Failed to reload local policy", "error", err.Error())
		}
	}

	// Broadcast to other instances
	if err := i.PublishInvalidate(ctx); err != nil {
		return fmt.Errorf("failed to broadcast invalidation: %w", err)
	}

	slog.Info("Permission cache invalidation broadcast complete")
	return nil
}