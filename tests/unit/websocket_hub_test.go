package unit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────────
// Mock Implementations
// ──────────────────────────────────────────────────────────────────

type mockPresenceService struct {
	mu            sync.Mutex
	onlineUsers   map[string]bool
	markOnlineCalls  []markOnlineCall
	markOfflineCalls []markOfflineCall
}

type markOnlineCall struct {
	OrgID  uuid.UUID
	UserID uuid.UUID
}

type markOfflineCall struct {
	OrgID  uuid.UUID
	UserID uuid.UUID
}

func newMockPresenceService() *mockPresenceService {
	return &mockPresenceService{
		onlineUsers: make(map[string]bool),
	}
}

func (m *mockPresenceService) MarkOnline(_ context.Context, orgID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markOnlineCalls = append(m.markOnlineCalls, markOnlineCall{OrgID: orgID, UserID: userID})
	m.onlineUsers[orgID.String()+":"+userID.String()] = true
	return nil
}

func (m *mockPresenceService) MarkOffline(_ context.Context, orgID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markOfflineCalls = append(m.markOfflineCalls, markOfflineCall{OrgID: orgID, UserID: userID})
	delete(m.onlineUsers, orgID.String()+":"+userID.String())
	return nil
}

func (m *mockPresenceService) RefreshHeartbeat(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (m *mockPresenceService) IsOnline(_ context.Context, orgID, userID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.onlineUsers[orgID.String()+":"+userID.String()], nil
}

func (m *mockPresenceService) GetOnlineUsers(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *mockPresenceService) GetOnlineCount(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

type mockRedisSubscriber struct {
	mu               sync.Mutex
	subscriptions    map[string]int
	subscribeCalls   []string
	unsubscribeCalls []string
	publishCalls     []publishCall
}

type publishCall struct {
	Channel string
	Data    []byte
}

func newMockRedisSubscriber() *mockRedisSubscriber {
	return &mockRedisSubscriber{
		subscriptions: make(map[string]int),
	}
}

func (m *mockRedisSubscriber) Subscribe(_ context.Context, room string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions[room]++
	m.subscribeCalls = append(m.subscribeCalls, room)
	return nil
}

func (m *mockRedisSubscriber) Unsubscribe(_ context.Context, room string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions[room]--
	if m.subscriptions[room] <= 0 {
		delete(m.subscriptions, room)
	}
	m.unsubscribeCalls = append(m.unsubscribeCalls, room)
	return nil
}

func (m *mockRedisSubscriber) Publish(_ context.Context, channel string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishCalls = append(m.publishCalls, publishCall{Channel: channel, Data: data})
	return nil
}

func (m *mockRedisSubscriber) Start(_ context.Context) error {
	return nil
}

func (m *mockRedisSubscriber) Stop() error {
	return nil
}

// ──────────────────────────────────────────────────────────────────
// Test Helpers
// ──────────────────────────────────────────────────────────────────

func newTestHub(t *testing.T) (*service.Hub, *mockPresenceService, *mockRedisSubscriber, context.Context, context.CancelFunc) {
	t.Helper()

	presence := newMockPresenceService()
	redis := newMockRedisSubscriber()
	log := logger.NewNopLogger()
	cfg := config.WsConfig{
		MaxRoomsPerClient: 50,
		SendChannelBuffer: 256,
		WriteTimeout:       10 * time.Second,
		ReadTimeout:        60 * time.Second,
	}

	hub := service.NewHub(presence, redis, log, cfg)
	ctx, cancel := context.WithCancel(context.Background())

	err := hub.Start(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		hub.Stop()
		cancel()
	})

	return hub, presence, redis, ctx, cancel
}

func createTestClient(t *testing.T, hub *service.Hub, userID, orgID uuid.UUID) (*service.Client, *websocket.Conn) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		})
		if err != nil {
			return
		}
		client := service.NewClient(conn, userID, orgID, hub, logger.NewNopLogger(), config.WsConfig{
			SendChannelBuffer: 256,
			WriteTimeout:      10 * time.Second,
		})
		go client.Run(r.Context())
	}))
	t.Cleanup(server.Close)

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)

	// Wait briefly for the server handler to register the client
	time.Sleep(50 * time.Millisecond)

	// Find the client from hub - we need the server-side Client object
	// Since hub.Register is async, we give it time
	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() > 0
	}, time.Second, 10*time.Millisecond, "client should be registered in hub")

	return nil, conn
}

// createConnectedClient sets up a WebSocket server and connects a client to the hub.
// Returns the server-side Client object and a cleanup function.
func createConnectedClient(t *testing.T, hub *service.Hub, userID, orgID uuid.UUID) *service.Client {
	t.Helper()

	var client *service.Client
	clientCh := make(chan *service.Client, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		})
		if err != nil {
			return
		}
		c := service.NewClient(conn, userID, orgID, hub, logger.NewNopLogger(), config.WsConfig{
			SendChannelBuffer: 256,
			WriteTimeout:      10 * time.Second,
		})
		clientCh <- c
		// Don't start Run() - we just need the Client object for hub operations
		conn.CloseNow()
	}))
	t.Cleanup(server.Close)

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	conn.CloseNow()

	select {
	case client = <-clientCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for client creation")
	}

	return client
}

// ──────────────────────────────────────────────────────────────────
// Hub Registration Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_Register_AddsClient(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	initialCount := hub.GetTotalConnections()
	assert.Equal(t, 0, initialCount)

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond, "hub should have 1 client after registration")
}

func TestHub_Unregister_RemovesClient(t *testing.T) {
	hub, presence, _, _, _ := newTestHub(t)

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	hub.Unregister(client)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 0
	}, time.Second, 10*time.Millisecond, "hub should have 0 clients after unregistration")

	// Verify presence was called for online and offline
	presence.mu.Lock()
	assert.Len(t, presence.markOnlineCalls, 1, "MarkOnline should be called once")
	assert.Len(t, presence.markOfflineCalls, 1, "MarkOffline should be called once")
	presence.mu.Unlock()
}

// ──────────────────────────────────────────────────────────────────
// Room Join Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_JoinRoom_AddsClientToRoom(t *testing.T) {
	hub, _, redis, _, _ := newTestHub(t)

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	room := "org:" + orgID.String()
	err := hub.JoinRoom(client, room)
	require.NoError(t, err)

	assert.Equal(t, 1, hub.GetRoomClients(room), "room should have 1 client")

	// Verify Redis subscription was made
	redis.mu.Lock()
	assert.Contains(t, redis.subscribeCalls, room)
	redis.mu.Unlock()
}

func TestHub_JoinRoom_ValidRoomName(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	validRooms := []string{
		"org:" + orgID.String(),
		"org:" + orgID.String() + ":invoice:" + uuid.New().String(),
		"org:" + orgID.String() + ":news:" + uuid.New().String(),
	}

	for _, room := range validRooms {
		err := hub.JoinRoom(client, room)
		assert.NoError(t, err, "valid room name should be accepted: %s", room)
	}
}

// ──────────────────────────────────────────────────────────────────
// Room Validation Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_JoinRoom_InvalidRoomName(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)

	invalidRooms := []string{
		"invalid",
		"room",
		"org",          // missing UUID
		"org:notauuid", // invalid UUID
		"prefix:test",  // wrong prefix
		"org:" + orgID.String() + ":invalidtype:" + uuid.New().String(), // invalid entity type
	}

	for _, room := range invalidRooms {
		err := hub.JoinRoom(client, room)
		assert.Error(t, err, "invalid room name should be rejected: %s", room)
	}
}

// ──────────────────────────────────────────────────────────────────
// Room Leave Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_LeaveRoom_RemovesClientFromRoom(t *testing.T) {
	hub, _, redis, _, _ := newTestHub(t)

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	room := "org:" + orgID.String()

	err := hub.JoinRoom(client, room)
	require.NoError(t, err)
	assert.Equal(t, 1, hub.GetRoomClients(room))

	hub.LeaveRoom(client, room)
	assert.Equal(t, 0, hub.GetRoomClients(room), "room should have 0 clients after leave")

	// Verify Redis unsubscribed (room is empty)
	redis.mu.Lock()
	assert.Contains(t, redis.unsubscribeCalls, room)
	redis.mu.Unlock()
}

// ──────────────────────────────────────────────────────────────────
// Broadcast Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_BroadcastToRoom_DeliversToAllClients(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	orgID := uuid.New()
	room := "org:" + orgID.String()

	// Create and register 3 clients
	clients := make([]*service.Client, 3)
	for i := 0; i < 3; i++ {
		userID := uuid.New()
		clients[i] = createConnectedClient(t, hub, userID, orgID)
		hub.Register(clients[i])
	}

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 3
	}, time.Second, 10*time.Millisecond)

	// Join all clients to the same room
	for _, c := range clients {
		err := hub.JoinRoom(c, room)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, hub.GetRoomClients(room))

	// Broadcast a message to the room
	msg := domain.WsMessage{
		Type:      domain.WsTypeEvent,
		Data:      domain.WsEventData{Event: "test.event", Payload: "hello"},
		Timestamp: time.Now().UTC(),
		Room:      room,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	hub.BroadcastToRoom(room, data)

	// All clients' Send channels should receive the message
	receivedCount := 0
	for _, c := range clients {
		select {
		case <-c.Send:
			receivedCount++
		case <-time.After(500 * time.Millisecond):
			// Client did not receive message
		}
	}
	assert.Equal(t, 3, receivedCount, "all 3 clients should receive the broadcast")
}

func TestHub_PublishToRoom_SendsViaBroadcastChannel(t *testing.T) {
	hub, _, redis, _, _ := newTestHub(t)

	orgID := uuid.New()
	room := "org:" + orgID.String()
	userID := uuid.New()

	client := createConnectedClient(t, hub, userID, orgID)
	hub.Register(client)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	err := hub.JoinRoom(client, room)
	require.NoError(t, err)

	msg := domain.WsMessage{
		Type:      domain.WsTypeEvent,
		Data:      domain.WsEventData{Event: "test.publish", Payload: "data"},
		Timestamp: time.Now().UTC(),
		Room:      room,
	}

	hub.PublishToRoom(room, msg)

	// Wait for the broadcast goroutine to process
	require.Eventually(t, func() bool {
		select {
		case <-client.Send:
			return true
		default:
			return false
		}
	}, time.Second, 50*time.Millisecond, "client should receive published message")

	// Verify Redis publish was called
	redis.mu.Lock()
	assert.NotEmpty(t, redis.publishCalls, "Redis.Publish should be called")
	redis.mu.Unlock()
}

func TestHub_BroadcastToRoom_NonexistentRoom_DoesNothing(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	// Broadcasting to a room that doesn't exist should not panic
	hub.BroadcastToRoom("org:"+uuid.New().String(), []byte(`{"type":"test"}`))
}

// ──────────────────────────────────────────────────────────────────
// Room Limit Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_JoinRoom_ExceedsMaxRooms(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	// Join 50 rooms (MaxRoomsPerClient = 50)
	validTypes := []string{"invoice", "news", "media", "comment"}
	for i := 0; i < 50; i++ {
		room := fmt.Sprintf("org:%s:%s:%s", orgID.String(), validTypes[i%len(validTypes)], uuid.New().String())
		err := hub.JoinRoom(client, room)
		require.NoError(t, err, "joining room %d should succeed", i)
	}

	// The 51st room should fail
	room51 := fmt.Sprintf("org:%s:invoice:%s", orgID.String(), uuid.New().String())
	err := hub.JoinRoom(client, room51)
	assert.Error(t, err, "joining 51st room should fail with room limit error")
	assert.Contains(t, err.Error(), "maximum rooms per client")
}

// ──────────────────────────────────────────────────────────────────
// Server Shutdown Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_Stop_ClosesAllConnections(t *testing.T) {
	presence := newMockPresenceService()
	redisSub := newMockRedisSubscriber()
	log := logger.NewNopLogger()
	cfg := config.WsConfig{
		MaxRoomsPerClient: 50,
		SendChannelBuffer: 256,
		WriteTimeout:      10 * time.Second,
		ReadTimeout:       60 * time.Second,
	}

	hub := service.NewHub(presence, redisSub, log, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := hub.Start(ctx)
	require.NoError(t, err)

	// Register 3 clients
	clients := make([]*service.Client, 3)
	for i := 0; i < 3; i++ {
		userID := uuid.New()
		orgID := uuid.New()
		clients[i] = createConnectedClient(t, hub, userID, orgID)
		hub.Register(clients[i])
	}

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 3
	}, time.Second, 10*time.Millisecond)

	// Stop the hub
	err = hub.Stop()
	require.NoError(t, err)
}

// ──────────────────────────────────────────────────────────────────
// GetTotalConnections and GetRoomClients Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_GetTotalConnections_ReturnsCorrectCount(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	assert.Equal(t, 0, hub.GetTotalConnections())

	userID := uuid.New()
	orgID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	// Register a second client
	userID2 := uuid.New()
	orgID2 := uuid.New()
	client2 := createConnectedClient(t, hub, userID2, orgID2)

	hub.Register(client2)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 2
	}, time.Second, 10*time.Millisecond)
}

func TestHub_GetRoomClients_ReturnsCorrectCount(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	orgID := uuid.New()
	room := "org:" + orgID.String()

	// Non-existent room should return 0
	assert.Equal(t, 0, hub.GetRoomClients(room))

	userID1 := uuid.New()
	userID2 := uuid.New()
	client1 := createConnectedClient(t, hub, userID1, orgID)
	client2 := createConnectedClient(t, hub, userID2, orgID)

	hub.Register(client1)
	hub.Register(client2)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 2
	}, time.Second, 10*time.Millisecond)

	err := hub.JoinRoom(client1, room)
	require.NoError(t, err)
	err = hub.JoinRoom(client2, room)
	require.NoError(t, err)

	assert.Equal(t, 2, hub.GetRoomClients(room))
}

// ──────────────────────────────────────────────────────────────────
// Unregistration Cleans Up Rooms Tests
// ──────────────────────────────────────────────────────────────────

func TestHub_Unregister_CleansUpRoomMemberships(t *testing.T) {
	hub, _, _, _, _ := newTestHub(t)

	orgID := uuid.New()
	room := "org:" + orgID.String()

	userID := uuid.New()
	client := createConnectedClient(t, hub, userID, orgID)

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 1
	}, time.Second, 10*time.Millisecond)

	err := hub.JoinRoom(client, room)
	require.NoError(t, err)
	assert.Equal(t, 1, hub.GetRoomClients(room))

	// Unregister the client - should also remove from all rooms
	hub.Unregister(client)

	require.Eventually(t, func() bool {
		return hub.GetTotalConnections() == 0
	}, time.Second, 10*time.Millisecond)

	// Room should be empty after client leaves
	assert.Equal(t, 0, hub.GetRoomClients(room))
}