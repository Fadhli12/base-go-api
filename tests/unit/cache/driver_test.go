package cache

import (
	"testing"

	cache "github.com/example/go-api-base/internal/cache"
)

func TestInvalidCacheDriver(t *testing.T) {
	t.Run("NewDriver with invalid driver", func(t *testing.T) {
		cfg := cache.Config{Driver: "invalid"}
		_, err := cache.NewDriver(cfg, nil)
		if err == nil {
			t.Error("expected error for invalid driver")
		}
	})

	t.Run("NewDriver with redis driver and nil client", func(t *testing.T) {
		cfg := cache.Config{Driver: "redis"}
		_, err := cache.NewDriver(cfg, nil)
		if err == nil {
			t.Error("expected error for redis driver without client")
		}
	})

	t.Run("NewDriver with memory driver", func(t *testing.T) {
		cfg := cache.Config{Driver: "memory"}
		driver, err := cache.NewDriver(cfg, nil)
		if err != nil {
			t.Errorf("expected no error for memory driver: %v", err)
		}
		if driver == nil {
			t.Error("expected driver to be created")
		}
	})

	t.Run("NewDriver with none driver", func(t *testing.T) {
		cfg := cache.Config{Driver: "none"}
		driver, err := cache.NewDriver(cfg, nil)
		if err != nil {
			t.Errorf("expected no error for none driver: %v", err)
		}
		if driver == nil {
			t.Error("expected driver to be created")
		}
	})
}