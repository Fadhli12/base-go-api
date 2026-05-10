//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TestRedisCacheDriver — Redis-backed cache operations
// =============================================================================

func TestRedisCacheDriver(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("Set and Get", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:setget"
		value := []byte("hello world")

		err = drv.Set(ctx, key, value, 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, got)
	})

	t.Run("Get missing key returns nil", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		got, err := drv.Get(ctx, "test:cache:integ:nonexistent")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("Delete removes key", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:delete"
		value := []byte("to be deleted")

		err = drv.Set(ctx, key, value, 60)
		require.NoError(t, err)

		// Verify it exists before deletion
		exists, err := drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists, "key should exist before delete")

		err = drv.Delete(ctx, key)
		require.NoError(t, err)

		// Verify it's gone
		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Nil(t, got, "key should be nil after delete")

		exists, err = drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists, "key should not exist after delete")
	})

	t.Run("Exists checks key presence", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:exists"

		// Should not exist initially
		exists, err := drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists, "key should not exist before set")

		// Set it
		err = drv.Set(ctx, key, []byte("data"), 60)
		require.NoError(t, err)

		// Should exist now
		exists, err = drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists, "key should exist after set")
	})

	t.Run("Clear with pattern removes matching keys", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		// Set multiple keys with the same prefix
		clearKeys := []string{
			"test:cache:clear:a",
			"test:cache:clear:b",
			"test:cache:clear:c",
		}
		for _, k := range clearKeys {
			err = drv.Set(ctx, k, []byte("val"), 60)
			require.NoError(t, err)
		}

		// Set a key that should NOT be cleared
		keepKey := "test:cache:keep:this"
		err = drv.Set(ctx, keepKey, []byte("val"), 60)
		require.NoError(t, err)

		// Verify all keys exist
		for _, k := range clearKeys {
			exists, err := drv.Exists(ctx, k)
			require.NoError(t, err)
			assert.True(t, exists, "key %s should exist before clear", k)
		}

		// Clear only matching pattern
		err = drv.Clear(ctx, "test:cache:clear:*")
		require.NoError(t, err)

		// Matching keys should be gone
		for _, k := range clearKeys {
			exists, err := drv.Exists(ctx, k)
			require.NoError(t, err)
			assert.False(t, exists, "key %s should be cleared", k)
		}

		// Non-matching key should remain
		exists, err := drv.Exists(ctx, keepKey)
		require.NoError(t, err)
		assert.True(t, exists, "non-matching key should survive clear")
	})

	t.Run("Set with TTL, key exists immediately", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:ttl"
		err = drv.Set(ctx, key, []byte("data"), 300)
		require.NoError(t, err)

		// Should exist right after set
		exists, err := drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists, "key should exist immediately after Set with TTL")
	})
}

// =============================================================================
// TestRedisCacheSetGetTypes — different value types
// =============================================================================

func TestRedisCacheSetGetTypes(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("string bytes", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:types:string"
		value := []byte("a simple string value")

		err = drv.Set(ctx, key, value, 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, got)
	})

	t.Run("empty bytes", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:types:empty"
		value := []byte("")

		err = drv.Set(ctx, key, value, 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 0, len(got))
	})

	t.Run("large bytes (10KB)", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:types:large"
		value := make([]byte, 10*1024) // 10KB
		for i := range value {
			value[i] = byte(i % 256)
		}

		err = drv.Set(ctx, key, value, 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, got)
	})

	t.Run("binary/JSON bytes", func(t *testing.T) {
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		ctx := context.Background()

		key := "test:cache:integ:types:json"
		value := []byte(`{"id":"abc-123","name":"test","count":42,"active":true}`)

		err = drv.Set(ctx, key, value, 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, got)
	})
}

// =============================================================================
// TestMemoryCacheDriver — in-memory cache operations
// =============================================================================

func TestMemoryCacheDriver(t *testing.T) {
	t.Run("Set and Get", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		key := "mem:setget"
		value := []byte("memory value")

		err = drv.Set(ctx, key, value, 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, got)
	})

	t.Run("Get missing key returns nil", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		got, err := drv.Get(ctx, "mem:nonexistent")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("Delete removes key", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		key := "mem:delete"
		err = drv.Set(ctx, key, []byte("val"), 60)
		require.NoError(t, err)

		err = drv.Delete(ctx, key)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("Exists checks key presence", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		key := "mem:exists"

		exists, err := drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists)

		err = drv.Set(ctx, key, []byte("data"), 60)
		require.NoError(t, err)

		exists, err = drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("TTL expiration", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		key := "mem:ttl:expire"
		err = drv.Set(ctx, key, []byte("transient"), 1) // 1 second TTL
		require.NoError(t, err)

		// Should exist immediately
		exists, err := drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists, "should exist immediately after Set")

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, []byte("transient"), got)

		// Wait for expiration (1s TTL + some buffer)
		time.Sleep(1100 * time.Millisecond)

		// Should be expired now
		exists, err = drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists, "should be expired after TTL")

		got, err = drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Nil(t, got, "Get should return nil for expired key")
	})

	t.Run("TTL not expired before time", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		key := "mem:ttl:notexpired"
		err = drv.Set(ctx, key, []byte("long-lived"), 300) // 5 minutes
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		exists, err := drv.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists, "should still exist well within TTL")
	})

	t.Run("Clear with prefix pattern", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		drv.Set(ctx, "prefix:a", []byte("1"), 60)
		drv.Set(ctx, "prefix:b", []byte("2"), 60)
		drv.Set(ctx, "other:key", []byte("3"), 60)

		err = drv.Clear(ctx, "prefix*")
		require.NoError(t, err)

		// Prefix keys should be gone
		exists, _ := drv.Exists(ctx, "prefix:a")
		assert.False(t, exists)
		exists, _ = drv.Exists(ctx, "prefix:b")
		assert.False(t, exists)

		// Other key should remain
		exists, _ = drv.Exists(ctx, "other:key")
		assert.True(t, exists)
	})

	t.Run("Clear with suffix pattern", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		drv.Set(ctx, "a:suffix", []byte("1"), 60)
		drv.Set(ctx, "b:suffix", []byte("2"), 60)
		drv.Set(ctx, "unrelated", []byte("3"), 60)

		err = drv.Clear(ctx, "*suffix")
		require.NoError(t, err)

		exists, _ := drv.Exists(ctx, "a:suffix")
		assert.False(t, exists)
		exists, _ = drv.Exists(ctx, "b:suffix")
		assert.False(t, exists)
		exists, _ = drv.Exists(ctx, "unrelated")
		assert.True(t, exists)
	})

	t.Run("Clear with middle pattern", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		drv.Set(ctx, "a:middle:x", []byte("1"), 60)
		drv.Set(ctx, "b:middle:y", []byte("2"), 60)
		drv.Set(ctx, "no-match", []byte("3"), 60)

		err = drv.Clear(ctx, "*middle*")
		require.NoError(t, err)

		exists, _ := drv.Exists(ctx, "a:middle:x")
		assert.False(t, exists)
		exists, _ = drv.Exists(ctx, "b:middle:y")
		assert.False(t, exists)
		exists, _ = drv.Exists(ctx, "no-match")
		assert.True(t, exists)
	})

	t.Run("Clear with exact match", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		drv.Set(ctx, "exact:key", []byte("1"), 60)
		drv.Set(ctx, "exact:key:other", []byte("2"), 60)

		err = drv.Clear(ctx, "exact:key")
		require.NoError(t, err)

		exists, _ := drv.Exists(ctx, "exact:key")
		assert.False(t, exists, "exact match key should be cleared")
		exists, _ = drv.Exists(ctx, "exact:key:other")
		assert.True(t, exists, "non-exact key should survive")
	})

	t.Run("Clear with wildcard * clears all", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		drv.Set(ctx, "a", []byte("1"), 60)
		drv.Set(ctx, "b", []byte("2"), 60)
		drv.Set(ctx, "c", []byte("3"), 60)

		err = drv.Clear(ctx, "*")
		require.NoError(t, err)

		exists, _ := drv.Exists(ctx, "a")
		assert.False(t, exists)
		exists, _ = drv.Exists(ctx, "b")
		assert.False(t, exists)
		exists, _ = drv.Exists(ctx, "c")
		assert.False(t, exists)
	})

	t.Run("Close clears all data", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		drv.Set(ctx, "a", []byte("1"), 60)
		drv.Set(ctx, "b", []byte("2"), 60)

		err = drv.Close()
		require.NoError(t, err)

		exists, _ := drv.Exists(ctx, "a")
		assert.False(t, exists, "Close should clear all data")
		exists, _ = drv.Exists(ctx, "b")
		assert.False(t, exists, "Close should clear all data")
	})

	t.Run("Overwrite existing key", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		ctx := context.Background()

		key := "mem:overwrite"
		err = drv.Set(ctx, key, []byte("original"), 60)
		require.NoError(t, err)

		err = drv.Set(ctx, key, []byte("updated"), 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, []byte("updated"), got)
	})
}

// =============================================================================
// TestNoopCacheDriver — no-op cache returns zero values
// =============================================================================

func TestNoopCacheDriver(t *testing.T) {
	drv, err := cache.NewDriver(cache.Config{Driver: "none"}, nil)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("Get returns nil, nil", func(t *testing.T) {
		got, err := drv.Get(ctx, "any-key")
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("Set returns nil", func(t *testing.T) {
		err := drv.Set(ctx, "any-key", []byte("value"), 60)
		assert.NoError(t, err)

		// Get still returns nil even after Set (noop doesn't actually store)
		got, err := drv.Get(ctx, "any-key")
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("Delete returns nil", func(t *testing.T) {
		err := drv.Delete(ctx, "any-key")
		assert.NoError(t, err)
	})

	t.Run("Exists returns false, nil", func(t *testing.T) {
		exists, err := drv.Exists(ctx, "any-key")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Close returns nil", func(t *testing.T) {
		err := drv.Close()
		assert.NoError(t, err)
	})

	t.Run("Clear returns nil", func(t *testing.T) {
		err := drv.Clear(ctx, "*")
		assert.NoError(t, err)
	})
}

// =============================================================================
// TestNewDriverFactory — factory function with different configs
// =============================================================================

func TestNewDriverFactory(t *testing.T) {
	t.Run("redis driver with valid client", func(t *testing.T) {
		suite := NewTestSuite(t)
		defer suite.Cleanup()
		suite.RunMigrations(t)
		suite.SetupTest(t)

		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
		require.NoError(t, err)
		require.NotNil(t, drv)

		ctx := context.Background()
		err = drv.Set(ctx, "test:cache:integ:factory:redis", []byte("ok"), 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, "test:cache:integ:factory:redis")
		require.NoError(t, err)
		assert.Equal(t, []byte("ok"), got)
	})

	t.Run("memory driver", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		require.NoError(t, err)
		require.NotNil(t, drv)

		ctx := context.Background()
		err = drv.Set(ctx, "mem:factory", []byte("memory"), 60)
		require.NoError(t, err)

		got, err := drv.Get(ctx, "mem:factory")
		require.NoError(t, err)
		assert.Equal(t, []byte("memory"), got)
	})

	t.Run("none driver", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "none"}, nil)
		require.NoError(t, err)
		require.NotNil(t, drv)

		// Noop - Get always returns nil
		got, err := drv.Get(context.Background(), "anything")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("invalid driver returns error", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "invalid"}, nil)
		assert.Error(t, err)
		assert.Nil(t, drv)
		assert.Contains(t, err.Error(), "unsupported cache driver")
	})

	t.Run("redis driver with nil client returns error", func(t *testing.T) {
		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, nil)
		assert.Error(t, err)
		assert.Nil(t, drv)
		assert.Contains(t, err.Error(), "redis client is required")
	})

	t.Run("redis driver with wrong client type returns non-nil but operations fail", func(t *testing.T) {
		// newRedisCacheFromClient type-asserts to *redis.Client,
		// passing a wrong type results in internal client == nil
		drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, "not-a-redis-client")
		require.NoError(t, err)
		require.NotNil(t, drv)

		// Operations on this driver should return "redis client not initialized"
		_, err = drv.Get(context.Background(), "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client not initialized")

		err = drv.Set(context.Background(), "test", []byte("val"), 60)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client not initialized")

		err = drv.Delete(context.Background(), "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client not initialized")

		_, err = drv.Exists(context.Background(), "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client not initialized")

		err = drv.Clear(context.Background(), "*")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client not initialized")
	})
}

// =============================================================================
// TestRedisCacheCloseDoesNotCloseClient — Close is no-op
// =============================================================================

func TestRedisCacheCloseDoesNotCloseClient(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	redisClient := suite.GetRedisClient()

	drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, redisClient)
	require.NoError(t, err)
	ctx := context.Background()

	// Set a value
	err = drv.Set(ctx, "test:cache:integ:close", []byte("data"), 60)
	require.NoError(t, err)

	// Close the cache driver (should NOT close the Redis client — it's a no-op)
	err = drv.Close()
	require.NoError(t, err)

	// Redis client should still be functional
	pingErr := redisClient.Ping(ctx).Err()
	assert.NoError(t, pingErr, "Redis client should still be alive after cache driver Close")

	// The key should still exist too
	exists, err := drv.Exists(ctx, "test:cache:integ:close")
	require.NoError(t, err)
	assert.True(t, exists, "key should survive cache driver Close")
}

// =============================================================================
// TestRedisClearEmptyPattern — SCAN with no matching keys
// =============================================================================

func TestRedisClearEmptyPattern(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	drv, err := cache.NewDriver(cache.Config{Driver: "redis"}, suite.GetRedisClient())
	require.NoError(t, err)
	ctx := context.Background()

	// Set some unrelated keys
	drv.Set(ctx, "unrelated:a", []byte("1"), 60)
	drv.Set(ctx, "unrelated:b", []byte("2"), 60)

	// Clear with a pattern that matches nothing
	err = drv.Clear(ctx, "test:cache:integ:definitely:nonexistent:*")
	require.NoError(t, err)

	// Unrelated keys should still be there
	exists, _ := drv.Exists(ctx, "unrelated:a")
	assert.True(t, exists)
	exists, _ = drv.Exists(ctx, "unrelated:b")
	assert.True(t, exists)
}
