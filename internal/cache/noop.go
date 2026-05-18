package cache

import (
	"context"
)

// noopCache implements Driver with no-op operations (disabled cache)
type noopCache struct{}

// newNoopCache creates a no-op cache driver
func newNoopCache() *noopCache {
	return &noopCache{}
}

// Get always returns nil (cache miss) for no-op cache
func (c *noopCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, nil // no-op: always cache miss
}

// Set is a no-op and always succeeds
func (c *noopCache) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	return nil // no-op: silently succeed
}

// SetNX is a no-op and always returns false (key not acquired) with no error.
// This ensures fail-open behavior: when cache is disabled, idempotency middleware
// passes through without locking.
func (c *noopCache) SetNX(ctx context.Context, key string, value []byte, ttlSeconds int) (bool, error) {
	return false, nil // no-op: always return false (not acquired), no error
}

// Delete is a no-op and always succeeds
func (c *noopCache) Delete(ctx context.Context, key string) error {
	return nil // no-op: silently succeed
}

// Exists always returns false for no-op cache
func (c *noopCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil // no-op: always return false
}

// Close is a no-op
func (c *noopCache) Close() error {
	return nil // no-op
}

// Clear is a no-op and always succeeds
func (c *noopCache) Clear(ctx context.Context, pattern string) error {
	return nil // no-op: silently succeed
}
