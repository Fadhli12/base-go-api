package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCacheDriver implements cache.Driver for permission cache testing
type mockCacheDriver struct {
	data      map[string][]byte
	nilResult map[string]bool
	getErr    error
	setErr    error
	delErr    error
	clearErr  error
}

func newMockCacheDriver() *mockCacheDriver {
	return &mockCacheDriver{data: make(map[string][]byte), nilResult: make(map[string]bool)}
}

func (m *mockCacheDriver) Get(ctx context.Context, key string) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.nilResult[key] {
		return nil, nil
	}
	val, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return val, nil
}

func (m *mockCacheDriver) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

func (m *mockCacheDriver) Delete(ctx context.Context, key string) error {
	if m.delErr != nil {
		return m.delErr
	}
	delete(m.data, key)
	return nil
}

func (m *mockCacheDriver) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockCacheDriver) Clear(ctx context.Context, pattern string) error {
	if m.clearErr != nil {
		return m.clearErr
	}
	m.data = make(map[string][]byte)
	return nil
}

func (m *mockCacheDriver) Close() error {
	return nil
}

// --- Permission Cache Tests ---

func TestPermissionCache_NewCache_DefaultTTL(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 0) // 0 should use default
	assert.Equal(t, 5*time.Minute, c.TTL())
}

func TestPermissionCache_NewCache_CustomTTL(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 10*time.Minute)
	assert.Equal(t, 10*time.Minute, c.TTL())
}

func TestPermissionCache_SetAndGet_Allowed(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.Set(ctx, "perm:user1:default:invoices:view", true)
	require.NoError(t, err)

	allowed, err := c.Get(ctx, "perm:user1:default:invoices:view")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestPermissionCache_SetAndGet_Denied(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.Set(ctx, "perm:user1:default:invoices:delete", false)
	require.NoError(t, err)

	allowed, err := c.Get(ctx, "perm:user1:default:invoices:delete")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestPermissionCache_Get_CacheMiss(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	_, err := c.Get(ctx, "perm:nonexistent:key")
	assert.Error(t, err)
	// Cache.Get wraps driver errors; ErrCacheMiss is only returned on nil result
	assert.Contains(t, err.Error(), "failed to get cached permission")
}

func TestPermissionCache_Get_NilResult(t *testing.T) {
	driver := newMockCacheDriver()
	driver.nilResult["perm:nil:key"] = true
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	_, err := c.Get(ctx, "perm:nil:key")
	assert.Equal(t, permission.ErrCacheMiss, err)
}

func TestPermissionCache_Invalidate(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	key := "perm:user1:default:invoices:view"
	err := c.Set(ctx, key, true)
	require.NoError(t, err)

	err = c.Invalidate(ctx, key)
	require.NoError(t, err)

	_, err = c.Get(ctx, key)
	assert.Error(t, err)
}

func TestPermissionCache_InvalidateAll(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.Set(ctx, "perm:user1:default:invoices:view", true)
	require.NoError(t, err)
	err = c.Set(ctx, "perm:user2:default:users:manage", true)
	require.NoError(t, err)

	err = c.InvalidateAll(ctx)
	require.NoError(t, err)

	_, err = c.Get(ctx, "perm:user1:default:invoices:view")
	assert.Error(t, err)
}

func TestPermissionCache_InvalidateForUser(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.Set(ctx, "perm:user1:default:invoices:view", true)
	require.NoError(t, err)
	err = c.Set(ctx, "perm:user2:default:invoices:view", true)
	require.NoError(t, err)

	err = c.InvalidateForUser(ctx, "user1")
	require.NoError(t, err)

	_, err = c.Get(ctx, "perm:user1:default:invoices:view")
	assert.Error(t, err)
}

func TestPermissionCache_InvalidateForDomain(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.Set(ctx, "perm:user1:org1:invoices:view", true)
	require.NoError(t, err)
	err = c.Set(ctx, "perm:user1:org2:invoices:view", true)
	require.NoError(t, err)

	err = c.InvalidateForDomain(ctx, "org1")
	require.NoError(t, err)

	_, err = c.Get(ctx, "perm:user1:org1:invoices:view")
	assert.Error(t, err)
}

func TestPermissionCache_SetTTL(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)

	assert.Equal(t, 5*time.Minute, c.TTL())

	c.SetTTL(10 * time.Minute)
	assert.Equal(t, 10*time.Minute, c.TTL())
}

func TestPermissionCache_SetTTL_IgnoresZero(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)

	c.SetTTL(0)
	assert.Equal(t, 5*time.Minute, c.TTL())
}

func TestPermissionCache_SetTTL_IgnoresNegative(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)

	c.SetTTL(-1 * time.Minute)
	assert.Equal(t, 5*time.Minute, c.TTL())
}

func TestPermissionCache_Driver(t *testing.T) {
	driver := newMockCacheDriver()
	c := permission.NewCache(driver, 5*time.Minute)

	assert.Equal(t, driver, c.Driver())
}

func TestPermissionCache_CacheKeyPrefix(t *testing.T) {
	assert.Equal(t, "perm:", permission.CacheKeyPrefix)
}

func TestPermissionCache_ErrCacheMiss(t *testing.T) {
	assert.Error(t, permission.ErrCacheMiss)
	assert.Contains(t, permission.ErrCacheMiss.Error(), "cache miss")
}

func TestPermissionCache_Set_DriverError(t *testing.T) {
	driver := newMockCacheDriver()
	driver.setErr = fmt.Errorf("redis error")
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.Set(ctx, "perm:user1:default:invoices:view", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache permission")
}

func TestPermissionCache_Invalidate_DriverError(t *testing.T) {
	driver := newMockCacheDriver()
	driver.delErr = fmt.Errorf("redis error")
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.Invalidate(ctx, "perm:user1:default:invoices:view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalidate")
}

func TestPermissionCache_InvalidateAll_DriverError(t *testing.T) {
	driver := newMockCacheDriver()
	driver.clearErr = fmt.Errorf("redis error")
	c := permission.NewCache(driver, 5*time.Minute)
	ctx := context.Background()

	err := c.InvalidateAll(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clear permission cache")
}

// --- RBAC Model Tests ---

func TestRBACModel(t *testing.T) {
	m := permission.RBACModel()
	require.NotNil(t, m)

	assert.NotNil(t, m["r"])
	assert.NotNil(t, m["p"])
	assert.NotNil(t, m["g"])
	assert.NotNil(t, m["e"])
	assert.NotNil(t, m["m"])
}

func TestPermissionCache_InvalidationChannel(t *testing.T) {
	assert.Equal(t, "permission:invalidate", permission.InvalidationChannel)
}

// --- CacheKey Helper Tests ---

func TestCacheKey(t *testing.T) {
	key := permission.CacheKey("user1", "default", "invoices", "view")
	assert.Equal(t, "perm:user1:default:invoices:view", key)
}

func TestCacheKey_DifferentParts(t *testing.T) {
	key := permission.CacheKey("admin", "org1", "users", "manage")
	assert.Equal(t, "perm:admin:org1:users:manage", key)
}

func TestParseCacheKey_Valid(t *testing.T) {
	sub, dom, obj, act, err := permission.ParseCacheKey("perm:user1:default:invoices:view")
	require.NoError(t, err)
	assert.Equal(t, "user1", sub)
	assert.Equal(t, "default", dom)
	assert.Equal(t, "invoices", obj)
	assert.Equal(t, "view", act)
}

func TestParseCacheKey_InvalidPrefix(t *testing.T) {
	_, _, _, _, err := permission.ParseCacheKey("cache:user1:default:invoices:view")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cache key format")
}

func TestParseCacheKey_TooFewParts(t *testing.T) {
	_, _, _, _, err := permission.ParseCacheKey("perm:user1:default:invoices")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cache key format")
}

func TestParseCacheKey_EmptyKey(t *testing.T) {
	_, _, _, _, err := permission.ParseCacheKey("")
	assert.Error(t, err)
}