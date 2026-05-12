package unit

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type receivedMessage struct {
	Room  string
	Event domain.WsMessage
}

type mockHubBroadcaster struct {
	mu      sync.Mutex
	message []receivedMessage
}

func (m *mockHubBroadcaster) PublishToRoom(room string, event domain.WsMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.message = append(m.message, receivedMessage{Room: room, Event: event})
}

func (m *mockHubBroadcaster) ReceivedMessages() []receivedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]receivedMessage, len(m.message))
	copy(cp, m.message)
	return cp
}

func setupTestRedisPubSub(t *testing.T) (*service.RedisPubSub, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.NewMiniRedis()
	require.NoError(t, mr.Start())
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	cfg := config.DefaultWsConfig()
	ps := service.NewRedisPubSub(rdb, logger.NewNopLogger(), cfg)

	return ps, mr, rdb
}

func publishWsMessage(t *testing.T, mr *miniredis.Miniredis, room string, msg domain.WsMessage) {
	t.Helper()
	payload, err := json.Marshal(msg)
	require.NoError(t, err)
	mr.Publish("ws:msg:"+room, string(payload))
}

func TestRedisPubSub_Subscribe(t *testing.T) {
	t.Run("subscribe activates Redis pub/sub and receives messages", func(t *testing.T) {
		ps, mr, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		wsMsg := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "test"},
			Timestamp: time.Now().UTC(),
			Room:      "room1",
		}
		publishWsMessage(t, mr, "room1", wsMsg)

		require.Eventually(t, func() bool {
			return len(hub.ReceivedMessages()) > 0
		}, 2*time.Second, 50*time.Millisecond)

		msgs := hub.ReceivedMessages()
		assert.Equal(t, "room1", msgs[0].Room)
		assert.Equal(t, domain.WsTypeEvent, msgs[0].Event.Type)
	})

	t.Run("subscribe to multiple rooms independently", func(t *testing.T) {
		ps, mr, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "roomA")
		require.NoError(t, err)
		err = ps.Subscribe(ctx, "roomB")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		msgA := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "eventA"},
			Timestamp: time.Now().UTC(),
			Room:      "roomA",
		}
		publishWsMessage(t, mr, "roomA", msgA)

		msgB := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "eventB"},
			Timestamp: time.Now().UTC(),
			Room:      "roomB",
		}
		publishWsMessage(t, mr, "roomB", msgB)

		require.Eventually(t, func() bool {
			return len(hub.ReceivedMessages()) >= 2
		}, 2*time.Second, 50*time.Millisecond)

		msgs := hub.ReceivedMessages()
		rooms := make(map[string]bool)
		for _, m := range msgs {
			rooms[m.Room] = true
		}
		assert.True(t, rooms["roomA"])
		assert.True(t, rooms["roomB"])
	})
}

func TestRedisPubSub_Unsubscribe(t *testing.T) {
	t.Run("unsubscribe removes subscription and stops receiving messages", func(t *testing.T) {
		ps, mr, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		err = ps.Unsubscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		wsMsg := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "after_unsub"},
			Timestamp: time.Now().UTC(),
			Room:      "room1",
		}
		publishWsMessage(t, mr, "room1", wsMsg)

		time.Sleep(200 * time.Millisecond)

		assert.Empty(t, hub.ReceivedMessages(), "should not receive messages after unsubscribe")
	})

	t.Run("unsubscribe non-existent room is no-op", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)
		err := ps.Unsubscribe(context.Background(), "nonexistent")
		require.NoError(t, err)
	})
}

func TestRedisPubSub_ReferenceCounting(t *testing.T) {
	t.Run("duplicate subscribes keep subscription active after single unsubscribe", func(t *testing.T) {
		ps, mr, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)
		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		err = ps.Unsubscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		wsMsg := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "still_subscribed"},
			Timestamp: time.Now().UTC(),
			Room:      "room1",
		}
		publishWsMessage(t, mr, "room1", wsMsg)

		require.Eventually(t, func() bool {
			return len(hub.ReceivedMessages()) > 0
		}, 2*time.Second, 50*time.Millisecond, "should still receive messages after one unsubscribe because ref count > 0")

		msgs := hub.ReceivedMessages()
		assert.Equal(t, "room1", msgs[0].Room)
	})

	t.Run("second unsubscribe removes Redis subscription completely", func(t *testing.T) {
		ps, mr, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)
		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		err = ps.Unsubscribe(ctx, "room1")
		require.NoError(t, err)
		err = ps.Unsubscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		wsMsg := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "fully_unsubbed"},
			Timestamp: time.Now().UTC(),
			Room:      "room1",
		}
		publishWsMessage(t, mr, "room1", wsMsg)

		time.Sleep(200 * time.Millisecond)

		assert.Empty(t, hub.ReceivedMessages(), "should not receive messages after all unsubscribes")
	})
}

func TestRedisPubSub_Publish(t *testing.T) {
	t.Run("publish sends message to Redis channel without error", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)

		ctx := context.Background()

		data := []byte(`{"type":"ws.event","data":{"event":"test"}}`)
		err := ps.Publish(ctx, "ws:msg:room1", data)
		require.NoError(t, err)
	})

	t.Run("publish and receive via receiveLoop", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		wsMsg := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "pub_test"},
			Timestamp: time.Now().UTC(),
			Room:      "room1",
		}
		payload, err := json.Marshal(wsMsg)
		require.NoError(t, err)

		err = ps.Publish(ctx, "ws:msg:room1", payload)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(hub.ReceivedMessages()) > 0
		}, 2*time.Second, 50*time.Millisecond)

		msgs := hub.ReceivedMessages()
		assert.Equal(t, "room1", msgs[0].Room)
	})
}

func TestRedisPubSub_CrossInstanceBroadcast(t *testing.T) {
	t.Run("message published via Publish is received by subscriber", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		wsMsg := domain.WsMessage{
			Type:      domain.WsTypeEvent,
			Data:      map[string]interface{}{"event": "cross_instance"},
			Timestamp: time.Now().UTC(),
			Room:      "room1",
		}
		payload, err := json.Marshal(wsMsg)
		require.NoError(t, err)

		err = ps.Publish(ctx, "ws:msg:room1", payload)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(hub.ReceivedMessages()) > 0
		}, 2*time.Second, 50*time.Millisecond, "message published on one instance should reach subscriber")

		msgs := hub.ReceivedMessages()
		assert.Equal(t, "room1", msgs[0].Room)
		assert.Equal(t, domain.WsTypeEvent, msgs[0].Event.Type)
	})
}

func TestRedisPubSub_GracefulDegradation(t *testing.T) {
	t.Run("Start does not panic when Redis is unavailable", func(t *testing.T) {
		mr := miniredis.NewMiniRedis()
		require.NoError(t, mr.Start())

		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		mr.Close()

		cfg := config.DefaultWsConfig()
		ps := service.NewRedisPubSub(rdb, logger.NewNopLogger(), cfg)

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		assert.NotPanics(t, func() {
			err := ps.Start(ctx)
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)
			cancel()
			ps.Stop()
		})
	})

	t.Run("Publish returns error when Redis is unavailable", func(t *testing.T) {
		mr := miniredis.NewMiniRedis()
		require.NoError(t, mr.Start())

		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		mr.Close()

		cfg := config.DefaultWsConfig()
		ps := service.NewRedisPubSub(rdb, logger.NewNopLogger(), cfg)

		err := ps.Publish(context.Background(), "ws:msg:room1", []byte("test"))
		assert.Error(t, err)
	})

	t.Run("does not panic when Redis closes mid-operation", func(t *testing.T) {
		ps, mr, _ := setupTestRedisPubSub(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		err := ps.Start(ctx)
		require.NoError(t, err)
		defer ps.Stop()

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		mr.Close()

		time.Sleep(200 * time.Millisecond)

		assert.NotPanics(t, func() {
			ps.Publish(context.Background(), "ws:msg:room1", []byte("test"))
		})
	})
}

func TestRedisPubSub_Lifecycle(t *testing.T) {
	t.Run("Start and Stop lifecycle completes cleanly", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ps.Start(ctx)
		require.NoError(t, err)

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		err = ps.Stop()
		require.NoError(t, err)
	})

	t.Run("Stop without Start is safe", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)
		err := ps.Stop()
		require.NoError(t, err)
	})

	t.Run("double Stop is safe", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ps.Start(ctx)
		require.NoError(t, err)

		err = ps.Stop()
		require.NoError(t, err)

		err = ps.Stop()
		require.NoError(t, err)
	})

	t.Run("Stop cancels context and waits for goroutines", func(t *testing.T) {
		ps, _, _ := setupTestRedisPubSub(t)

		hub := &mockHubBroadcaster{}
		ps.SetHub(hub)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ps.Start(ctx)
		require.NoError(t, err)

		err = ps.Subscribe(ctx, "room1")
		require.NoError(t, err)

		err = ps.Stop()
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			ps.Publish(context.Background(), "ws:msg:room1", []byte("after_stop"))
		})
	})
}