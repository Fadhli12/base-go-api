//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/handler"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wsTestJWTSecret = "test-secret-key-for-ws-integration-min32"

func createTestToken(userID, orgID uuid.UUID, secret string) string {
	token, err := auth.GenerateAccessToken(userID.String(), "ws-test@example.com", secret, 15*time.Minute)
	if err != nil {
		panic("failed to generate test token: " + err.Error())
	}
	return token
}

func createExpiredToken(userID, orgID uuid.UUID, secret string) string {
	token, err := auth.GenerateAccessToken(userID.String(), "ws-test@example.com", secret, -1*time.Minute)
	if err != nil {
		panic("failed to generate expired token: " + err.Error())
	}
	return token
}

type wsTestEnv struct {
	echo     *echo.Echo
	hub      *service.Hub
	presence *service.RedisPresence
	enforcer *permission.Enforcer
	cancel   context.CancelFunc
	teardown func()
}

func setupWSTestEnv(t *testing.T) *wsTestEnv {
	t.Helper()

	suite := SetupIntegrationTest(t)

	testLogger, err := logger.NewLogger(logger.Config{Level: "debug", Format: "json", Outputs: "stdout"})
	if err != nil {
		testLogger = logger.NewNopLogger()
	}

	wsConfig := config.DefaultWsConfig()

	presence := service.NewRedisPresence(suite.RedisClient, testLogger, wsConfig)
	redisPubSub := service.NewRedisPubSub(suite.RedisClient, testLogger, wsConfig)
	hub := service.NewHub(presence, redisPubSub, testLogger, wsConfig)
	redisPubSub.SetHub(hub)

	enf, _ := permission.NewEnforcer(suite.DB)

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, hub.Start(ctx), "hub should start without error")

	wsHandler := handler.NewWebSocketHandler(hub, presence, enf, nil, testLogger, wsConfig)

	e := echo.New()
	e.HideBanner = true

	v1 := e.Group("/api/v1")
	wsHandler.RegisterRoutes(v1, wsTestJWTSecret)

	v1.GET("/presence/org/:orgID", wsHandler.GetPresence, middleware.JWT(middleware.JWTConfig{
		Secret:     wsTestJWTSecret,
		ContextKey: "user",
	}), middleware.ExtractOrganizationID())

	return &wsTestEnv{
		echo:     e,
		hub:      hub,
		presence: presence,
		enforcer: enf,
		cancel:   cancel,
		teardown: func() { suite.TeardownTest(t) },
	}
}

func (env *wsTestEnv) close() {
	env.hub.Stop()
	env.cancel()
	env.teardown()
}

func startTestServer(t *testing.T, env *wsTestEnv) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(env.echo)
	t.Cleanup(ts.Close)
	return ts
}

func wsDial(t *testing.T, ts *httptest.Server, token string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws?token=" + token
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err, "WebSocket dial should succeed")
	conn.SetReadLimit(65536)
	return conn
}

func readWSJSON(t *testing.T, ctx context.Context, conn *websocket.Conn) map[string]interface{} {
	t.Helper()
	var msg map[string]interface{}
	err := wsjson.Read(ctx, conn, &msg)
	require.NoError(t, err, "should read WebSocket message")
	return msg
}

func TestWebSocket_Connect_ValidToken_ReceivesConnected(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	ts := startTestServer(t, env)

	userID := uuid.New()
	token := createTestToken(userID, uuid.Nil, wsTestJWTSecret)

	conn := wsDial(t, ts, token)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg := readWSJSON(t, ctx, conn)
	assert.Equal(t, string(domain.WsTypeConnected), msg["type"])

	data, ok := msg["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, userID.String(), data["user_id"])
}

func TestWebSocket_Reject_MissingToken(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebSocket_Reject_InvalidToken(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token=invalid-token-value", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebSocket_Reject_ExpiredToken(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	userID := uuid.New()
	token := createExpiredToken(userID, uuid.Nil, wsTestJWTSecret)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token="+token, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebSocket_JoinRoom_ReceivesRoomJoined(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	ts := startTestServer(t, env)

	userID := uuid.New()
	orgID := uuid.New()
	token := createTestToken(userID, orgID, wsTestJWTSecret)

	conn := wsDial(t, ts, token)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readWSJSON(t, ctx, conn) // consume ws.connected

	roomName := "org:" + orgID.String()
	joinMsg := domain.WsMessage{
		Type: domain.WsTypeRoomJoin,
		Data: domain.WsRoomJoin{Room: roomName},
	}
	require.NoError(t, wsjson.Write(ctx, conn, joinMsg))

	joinedMsg := readWSJSON(t, ctx, conn)
	assert.Equal(t, string(domain.WsTypeRoomJoined), joinedMsg["type"])

	data, ok := joinedMsg["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, roomName, data["room"])
}

func TestWebSocket_LeaveRoom_ReceivesRoomLeft(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	ts := startTestServer(t, env)

	userID := uuid.New()
	orgID := uuid.New()
	token := createTestToken(userID, orgID, wsTestJWTSecret)

	conn := wsDial(t, ts, token)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readWSJSON(t, ctx, conn) // consume ws.connected

	roomName := "org:" + orgID.String()

	joinMsg := domain.WsMessage{
		Type: domain.WsTypeRoomJoin,
		Data: domain.WsRoomJoin{Room: roomName},
	}
	require.NoError(t, wsjson.Write(ctx, conn, joinMsg))
	readWSJSON(t, ctx, conn) // consume ws.room.joined

	leaveMsg := domain.WsMessage{
		Type: domain.WsTypeRoomLeave,
		Data: domain.WsRoomLeave{Room: roomName},
	}
	require.NoError(t, wsjson.Write(ctx, conn, leaveMsg))

	leftMsg := readWSJSON(t, ctx, conn)
	assert.Equal(t, string(domain.WsTypeRoomLeft), leftMsg["type"])

	data, ok := leftMsg["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, roomName, data["room"])
}

func TestWebSocket_Broadcast_ServiceEvent_Received(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	ts := startTestServer(t, env)

	userID := uuid.New()
	orgID := uuid.New()
	token := createTestToken(userID, orgID, wsTestJWTSecret)

	conn := wsDial(t, ts, token)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readWSJSON(t, ctx, conn) // consume ws.connected

	roomName := "org:" + orgID.String()

	joinMsg := domain.WsMessage{
		Type: domain.WsTypeRoomJoin,
		Data: domain.WsRoomJoin{Room: roomName},
	}
	require.NoError(t, wsjson.Write(ctx, conn, joinMsg))
	readWSJSON(t, ctx, conn) // consume ws.room.joined

	time.Sleep(100 * time.Millisecond) // allow hub to process join

	eventPayload := map[string]interface{}{"id": uuid.New().String(), "status": "created"}
	env.hub.PublishToRoom(roomName, domain.WsMessage{
		Type:      domain.WsTypeEvent,
		Data:      domain.WsEventData{Event: "invoice.created", Payload: eventPayload, Room: roomName},
		Timestamp: time.Now().UTC(),
		Room:      roomName,
	})

	eventMsg := readWSJSON(t, ctx, conn)
	assert.Equal(t, string(domain.WsTypeEvent), eventMsg["type"])

	data, ok := eventMsg["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "invoice.created", data["event"])
	assert.Equal(t, roomName, data["room"])
}

func TestWebSocket_Typing_EchoedBack(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	ts := startTestServer(t, env)

	userID := uuid.New()
	orgID := uuid.New()
	token := createTestToken(userID, orgID, wsTestJWTSecret)

	conn := wsDial(t, ts, token)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readWSJSON(t, ctx, conn) // consume ws.connected

	roomName := "org:" + orgID.String()

	joinMsg := domain.WsMessage{
		Type: domain.WsTypeRoomJoin,
		Data: domain.WsRoomJoin{Room: roomName},
	}
	require.NoError(t, wsjson.Write(ctx, conn, joinMsg))
	readWSJSON(t, ctx, conn) // consume ws.room.joined

	typingMsg := domain.WsMessage{
		Type: domain.WsTypeTyping,
		Data: domain.WsTyping{Room: roomName},
	}
	require.NoError(t, wsjson.Write(ctx, conn, typingMsg))

	echoMsg := readWSJSON(t, ctx, conn)
	assert.Equal(t, string(domain.WsTypeTyping), echoMsg["type"])

	data, ok := echoMsg["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, userID.String(), data["user_id"])
	assert.Equal(t, roomName, data["room"])
}

func TestWebSocket_Presence_UserOnline(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	ts := startTestServer(t, env)

	userID := uuid.New()
	orgID := uuid.New()
	token := createTestToken(userID, orgID, wsTestJWTSecret)

	conn := wsDial(t, ts, token)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readWSJSON(t, ctx, conn) // consume ws.connected
	time.Sleep(200 * time.Millisecond) // allow presence to propagate

	presenceToken := createTestToken(userID, orgID, wsTestJWTSecret)
	presenceURL := "/api/v1/presence/org/" + orgID.String()
	req := httptest.NewRequest(http.MethodGet, presenceURL, nil)
	req.Header.Set("Authorization", "Bearer "+presenceToken)
	req.Header.Set("X-Organization-ID", orgID.String())

	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)

	onlineUsers, ok := data["online_users"].([]interface{})
	require.True(t, ok)

	found := false
	for _, u := range onlineUsers {
		if u == userID.String() {
			found = true
			break
		}
	}
	assert.True(t, found, "user should appear in online users list")
}

func TestWebSocket_MultiTab_SameUser_SinglePresence(t *testing.T) {
	env := setupWSTestEnv(t)
	defer env.close()

	ts := startTestServer(t, env)

	userID := uuid.New()
	orgID := uuid.New()
	token := createTestToken(userID, orgID, wsTestJWTSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn1 := wsDial(t, ts, token)
	defer conn1.Close(websocket.StatusNormalClosure, "test done")
	readWSJSON(t, ctx, conn1) // consume ws.connected

	conn2 := wsDial(t, ts, token)
	defer conn2.Close(websocket.StatusNormalClosure, "test done")
	readWSJSON(t, ctx, conn2) // consume ws.connected

	time.Sleep(200 * time.Millisecond) // allow presence to propagate

	count, err := env.presence.GetOnlineCount(context.Background(), orgID)
	require.NoError(t, err)

	assert.Equal(t, int64(1), count, "same user with multiple connections should count as 1")
}