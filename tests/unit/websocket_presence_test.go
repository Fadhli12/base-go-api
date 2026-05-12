package unit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestPresence(t *testing.T) (*service.RedisPresence, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { client.Close() })

	cfg := config.WsConfig{
		PresenceTTL: 5 * time.Minute,
	}

	p := service.NewRedisPresence(client, logger.NewNopLogger(), cfg)
	return p, mr
}

func TestRedisPresence_MarkOnline_AddsToSet(t *testing.T) {
	p, mr := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)

	exists, _ := mr.SIsMember("ws:presence:"+orgID.String(), userID.String())
	assert.True(t, exists, "user should be in presence set after MarkOnline")

	val, _ := mr.Get("ws:conn:" + orgID.String() + ":" + userID.String())
	assert.Equal(t, "1", val, "connection count should be 1")
}

func TestRedisPresence_MarkOnline_SecondTab(t *testing.T) {
	p, mr := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)
	err = p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)

	exists, _ := mr.SIsMember("ws:presence:"+orgID.String(), userID.String())
	assert.True(t, exists, "user should still be in presence set")

	val, _ := mr.Get("ws:conn:" + orgID.String() + ":" + userID.String())
	assert.Equal(t, "2", val, "connection count should be 2 after two MarkOnline calls")
}

func TestRedisPresence_MarkOffline_StillOnline(t *testing.T) {
	p, mr := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)
	err = p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)

	err = p.MarkOffline(ctx, orgID, userID)
	require.NoError(t, err)

	exists, _ := mr.SIsMember("ws:presence:"+orgID.String(), userID.String())
	assert.True(t, exists, "user should still be in presence set when count > 0")

	val, _ := mr.Get("ws:conn:" + orgID.String() + ":" + userID.String())
	assert.Equal(t, "1", val, "connection count should be 1 after one MarkOffline")
}

func TestRedisPresence_MarkOffline_RemovesWhenCountZero(t *testing.T) {
	p, mr := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)
	err = p.MarkOffline(ctx, orgID, userID)
	require.NoError(t, err)

	exists, _ := mr.SIsMember("ws:presence:"+orgID.String(), userID.String())
	assert.False(t, exists, "user should be removed from presence set when count reaches 0")

	_, err = mr.Get("ws:conn:" + orgID.String() + ":" + userID.String())
	assert.Error(t, err, "connection key should be deleted when count reaches 0")
}

func TestRedisPresence_MarkOffline_MultiTabThenAllOffline(t *testing.T) {
	p, mr := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)
	err = p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)

	err = p.MarkOffline(ctx, orgID, userID)
	require.NoError(t, err)
	exists, _ := mr.SIsMember("ws:presence:"+orgID.String(), userID.String())
	assert.True(t, exists, "user should still be in set after first offline")

	err = p.MarkOffline(ctx, orgID, userID)
	require.NoError(t, err)
	exists, _ = mr.SIsMember("ws:presence:"+orgID.String(), userID.String())
	assert.False(t, exists, "user should be removed from set after second offline")
}

func TestRedisPresence_RefreshHeartbeat(t *testing.T) {
	p, mr := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)

	connKey := "ws:conn:" + orgID.String() + ":" + userID.String()
	ttlBefore, err := client.TTL(ctx, connKey).Result()
	require.NoError(t, err)

	mr.FastForward(2 * time.Minute)

	err = p.RefreshHeartbeat(ctx, orgID, userID)
	require.NoError(t, err)

	ttlAfter, err := client.TTL(ctx, connKey).Result()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, ttlAfter.Seconds(), ttlBefore.Seconds()-float64(2*time.Minute.Seconds()),
		"TTL should be refreshed back to original duration after advancing time")
}

func TestRedisPresence_IsOnline_True(t *testing.T) {
	p, _ := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)

	online, err := p.IsOnline(ctx, orgID, userID)
	require.NoError(t, err)
	assert.True(t, online, "user should be online after MarkOnline")
}

func TestRedisPresence_IsOnline_False(t *testing.T) {
	p, _ := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	online, err := p.IsOnline(ctx, orgID, userID)
	require.NoError(t, err)
	assert.False(t, online, "user should not be online without MarkOnline")
}

func TestRedisPresence_GetOnlineUsers(t *testing.T) {
	p, _ := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()
	user3 := uuid.New()

	err := p.MarkOnline(ctx, orgID, user1)
	require.NoError(t, err)
	err = p.MarkOnline(ctx, orgID, user2)
	require.NoError(t, err)
	err = p.MarkOnline(ctx, orgID, user3)
	require.NoError(t, err)

	users, err := p.GetOnlineUsers(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, users, 3, "should return 3 online users")

	userMap := make(map[uuid.UUID]bool)
	for _, u := range users {
		userMap[u] = true
	}
	assert.True(t, userMap[user1], "user1 should be in online list")
	assert.True(t, userMap[user2], "user2 should be in online list")
	assert.True(t, userMap[user3], "user3 should be in online list")
}

func TestRedisPresence_GetOnlineUsers_Empty(t *testing.T) {
	p, _ := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()

	users, err := p.GetOnlineUsers(ctx, orgID)
	require.NoError(t, err)
	assert.Empty(t, users, "should return empty list for org with no online users")
}

func TestRedisPresence_GetOnlineCount(t *testing.T) {
	p, _ := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	count, err := p.GetOnlineCount(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "count should be 0 before any MarkOnline")

	err = p.MarkOnline(ctx, orgID, user1)
	require.NoError(t, err)
	err = p.MarkOnline(ctx, orgID, user2)
	require.NoError(t, err)

	count, err = p.GetOnlineCount(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "count should be 2 after two MarkOnline calls")
}

func TestRedisPresence_StaleCleanup_ConnKeyTTxpiry(t *testing.T) {
	p, mr := setupTestPresence(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	require.NoError(t, err)

	connKey := "ws:conn:" + orgID.String() + ":" + userID.String()
	_, err = mr.Get(connKey)
	require.NoError(t, err, "conn key should exist before TTL expiry")

	mr.FastForward(6 * time.Minute)

	_, err = mr.Get(connKey)
	require.Error(t, err, "conn key should be expired after TTL")
}

func TestRedisPresence_GracefulDegradation_MarkOnline(t *testing.T) {
	p, mr := setupTestPresence(t)
	mr.Close()

	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOnline(ctx, orgID, userID)
	assert.NoError(t, err, "MarkOnline should not return error on Redis failure")
}

func TestRedisPresence_GracefulDegradation_MarkOffline(t *testing.T) {
	p, mr := setupTestPresence(t)
	mr.Close()

	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.MarkOffline(ctx, orgID, userID)
	assert.NoError(t, err, "MarkOffline should not return error on Redis failure")
}

func TestRedisPresence_GracefulDegradation_IsOnline(t *testing.T) {
	p, mr := setupTestPresence(t)
	mr.Close()

	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	online, err := p.IsOnline(ctx, orgID, userID)
	assert.NoError(t, err, "IsOnline should not return error on Redis failure")
	assert.False(t, online, "IsOnline should return false on Redis failure")
}

func TestRedisPresence_GracefulDegradation_GetOnlineUsers(t *testing.T) {
	p, mr := setupTestPresence(t)
	mr.Close()

	ctx := context.Background()
	orgID := uuid.New()

	users, err := p.GetOnlineUsers(ctx, orgID)
	assert.NoError(t, err, "GetOnlineUsers should not return error on Redis failure")
	assert.Empty(t, users, "GetOnlineUsers should return empty list on Redis failure")
}

func TestRedisPresence_GracefulDegradation_GetOnlineCount(t *testing.T) {
	p, mr := setupTestPresence(t)
	mr.Close()

	ctx := context.Background()
	orgID := uuid.New()

	count, err := p.GetOnlineCount(ctx, orgID)
	assert.NoError(t, err, "GetOnlineCount should not return error on Redis failure")
	assert.Equal(t, int64(0), count, "GetOnlineCount should return 0 on Redis failure")
}

func TestRedisPresence_GracefulDegradation_RefreshHeartbeat(t *testing.T) {
	p, mr := setupTestPresence(t)
	mr.Close()

	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	err := p.RefreshHeartbeat(ctx, orgID, userID)
	assert.NoError(t, err, "RefreshHeartbeat should not return error on Redis failure")
}

func TestRedisPresence_TwoOrgs_Isolated(t *testing.T) {
	p, _ := setupTestPresence(t)
	ctx := context.Background()
	org1 := uuid.New()
	org2 := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	err := p.MarkOnline(ctx, org1, user1)
	require.NoError(t, err)
	err = p.MarkOnline(ctx, org2, user2)
	require.NoError(t, err)

	online, err := p.IsOnline(ctx, org1, user1)
	require.NoError(t, err)
	assert.True(t, online, "user1 should be online in org1")

	online, err = p.IsOnline(ctx, org1, user2)
	require.NoError(t, err)
	assert.False(t, online, "user2 should not be online in org1")

	online, err = p.IsOnline(ctx, org2, user2)
	require.NoError(t, err)
	assert.True(t, online, "user2 should be online in org2")

	users1, err := p.GetOnlineUsers(ctx, org1)
	require.NoError(t, err)
	assert.Len(t, users1, 1)

	users2, err := p.GetOnlineUsers(ctx, org2)
	require.NoError(t, err)
	assert.Len(t, users2, 1)
}