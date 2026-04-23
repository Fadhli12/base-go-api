package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

// memoryCache implements Driver using in-memory map with expiration
type memoryCache struct {
	data map[string]*memEntry
	mu   sync.RWMutex
}

type memEntry struct {
	value     []byte
	expiresAt time.Time
}

// newMemoryCache creates a new in-memory cache driver
func newMemoryCache() *memoryCache {
	return &memoryCache{
		data: make(map[string]*memEntry),
	}
}

// Get retrieves a value from the in-memory cache
func (c *memoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.data[key]
	if !found {
		return nil, nil // not found
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		// Expired - return nil (will be cleaned up on next Set)
		return nil, nil
	}

	return entry.value, nil
}

// Set stores a value in the in-memory cache with expiration
func (c *memoryCache) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	c.data[key] = &memEntry{value: value, expiresAt: expiresAt}

	return nil
}

// Delete removes a key from the in-memory cache
func (c *memoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	return nil
}

// Exists checks if a key exists in the in-memory cache
func (c *memoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.data[key]
	if !found {
		return false, nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return false, nil
	}

	return true, nil
}

// Close clears the in-memory cache
func (c *memoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]*memEntry)
	return nil
}

// Clear removes all keys matching the given pattern
func (c *memoryCache) Clear(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// For in-memory cache, check each key against the pattern
	// Pattern is treated as a prefix/suffix wildcard (simple glob matching)
	// e.g., "perm:*" matches keys starting with "perm:"
	for key := range c.data {
		if matchesPattern(key, pattern) {
			delete(c.data, key)
		}
	}
	return nil
}

// matchesPattern performs simple wildcard matching
// Supports patterns like "prefix*", "*suffix", "*middle*", "exact"
func matchesPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// *middle*
		middle := pattern[1 : len(pattern)-1]
		return strings.Contains(key, middle)
	}
	if strings.HasSuffix(pattern, "*") {
		// prefix*
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(key, prefix)
	}
	if strings.HasPrefix(pattern, "*") {
		// *suffix
		suffix := pattern[1:]
		return strings.HasSuffix(key, suffix)
	}
	// exact match
	return key == pattern
}
