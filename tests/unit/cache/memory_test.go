package cache

import (
	"context"
	"testing"
	"time"

	cache "github.com/example/go-api-base/internal/cache"
)

func TestMemoryCache(t *testing.T) {
	ctx := context.Background()
	mc, err := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
	if err != nil {
		t.Fatalf("failed to create memory cache: %v", err)
	}
	defer mc.Close()

	t.Run("Set and Get", func(t *testing.T) {
		err := mc.Set(ctx, "key1", []byte("value1"), 300)
		if err != nil {
			t.Fatal(err)
		}

		result, err := mc.Get(ctx, "key1")
		if err != nil {
			t.Fatal(err)
		}
		if string(result) != "value1" {
			t.Errorf("expected 'value1', got '%s'", string(result))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		mc.Set(ctx, "key2", []byte("value2"), 300)
		mc.Delete(ctx, "key2")

		result, err := mc.Get(ctx, "key2")
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Error("expected nil after delete")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		mc.Set(ctx, "key3", []byte("value3"), 300)

		exists, err := mc.Exists(ctx, "key3")
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("expected key3 to exist")
		}
	})

	t.Run("TTL expiration", func(t *testing.T) {
		mc.Set(ctx, "key4", []byte("value4"), 1) // 1 second TTL

		exists, _ := mc.Exists(ctx, "key4")
		if !exists {
			t.Error("key4 should exist immediately after set")
		}

		time.Sleep(1100 * time.Millisecond)

		exists, _ = mc.Exists(ctx, "key4")
		if exists {
			t.Error("key4 should have expired")
		}
	})

	t.Run("Close", func(t *testing.T) {
		mc2, _ := cache.NewDriver(cache.Config{Driver: "memory"}, nil)
		mc2.Set(ctx, "key5", []byte("value5"), 300)
		err := mc2.Close()
		if err != nil {
			t.Error("Close should not return error")
		}
	})
}