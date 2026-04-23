package cache

import (
	"context"
	"testing"

	cache "github.com/example/go-api-base/internal/cache"
)

func TestNoopCache(t *testing.T) {
	ctx := context.Background()
	nc, err := cache.NewDriver(cache.Config{Driver: "none"}, nil)
	if err != nil {
		t.Fatalf("failed to create noop cache: %v", err)
	}
	defer nc.Close()

	t.Run("Get returns nil", func(t *testing.T) {
		result, err := nc.Get(ctx, "any-key")
		if err != nil {
			t.Error("expected no error")
		}
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("Set returns no error", func(t *testing.T) {
		err := nc.Set(ctx, "any-key", []byte("any-value"), 300)
		if err != nil {
			t.Error("expected no error")
		}
	})

	t.Run("Delete returns no error", func(t *testing.T) {
		err := nc.Delete(ctx, "any-key")
		if err != nil {
			t.Error("expected no error")
		}
	})

	t.Run("Exists returns false", func(t *testing.T) {
		exists, err := nc.Exists(ctx, "any-key")
		if err != nil {
			t.Error("expected no error")
		}
		if exists {
			t.Error("expected false")
		}
	})

	t.Run("Close returns no error", func(t *testing.T) {
		err := nc.Close()
		if err != nil {
			t.Error("expected no error")
		}
	})
}