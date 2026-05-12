package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/google/uuid"
)

// Client represents a single WebSocket connection with read and write pumps.
type Client struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	OrgID         uuid.UUID
	Conn          *websocket.Conn
	Hub           *Hub
	Send          chan []byte
	Rooms         map[string]bool
	mu            sync.RWMutex
	logger        logger.Logger
	writeDeadline time.Duration
}

// RoomCount returns the number of rooms this client has joined.
func (c *Client) RoomCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.Rooms)
}

// AddRoom adds a room to the client's room set. Returns true if the room was newly added.
func (c *Client) AddRoom(room string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Rooms[room] {
		return false
	}
	c.Rooms[room] = true
	return true
}

// RemoveRoom removes a room from the client's room set. Returns true if the room was present.
func (c *Client) RemoveRoom(room string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.Rooms[room] {
		return false
	}
	delete(c.Rooms, room)
	return true
}

// RoomsList returns a copy of the client's current rooms.
func (c *Client) RoomsList() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rooms := make([]string, 0, len(c.Rooms))
	for room := range c.Rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// NewClient creates a new Client with the given connection and user info.
func NewClient(conn *websocket.Conn, userID, orgID uuid.UUID, hub *Hub, log logger.Logger, cfg config.WsConfig) *Client {
	return &Client{
		ID:            uuid.New(),
		UserID:        userID,
		OrgID:         orgID,
		Conn:          conn,
		Hub:           hub,
		Send:          make(chan []byte, cfg.SendChannelBuffer),
		Rooms:         make(map[string]bool),
		logger:        log,
		writeDeadline: cfg.WriteTimeout,
	}
}

// Run starts the client's read and write pumps concurrently and waits for both to complete.
func (c *Client) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c.ReadPump(ctx)
	}()

	go func() {
		defer wg.Done()
		c.WritePump(ctx)
	}()

	wg.Wait()
}

// ReadPump reads messages from the WebSocket connection and routes them to handlers.
// On error or close, it triggers Hub.Unregister.
func (c *Client) ReadPump(ctx context.Context) {
	for {
		_, data, err := c.Conn.Read(ctx)
		if err != nil {
			c.logger.Error(ctx, "websocket read error",
				logger.String("client_id", c.ID.String()),
				logger.Err(err),
			)
			c.Hub.Unregister(c)
			return
		}

		c.handleMessage(data)
	}
}

// WritePump writes messages from the Send channel to the WebSocket connection.
// Exits on channel close or context cancellation. Ping/pong is handled
// automatically by the coder/websocket library.
func (c *Client) WritePump(ctx context.Context) {
	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				c.Close(int(websocket.StatusNormalClosure), "channel closed")
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, c.writeDeadline)
			if err := c.Conn.Write(writeCtx, websocket.MessageText, msg); err != nil {
				cancel()
				c.logger.Error(ctx, "websocket write error",
					logger.String("client_id", c.ID.String()),
					logger.Err(err),
				)
				return
			}
			cancel()

		case <-ctx.Done():
			c.Close(int(domain.WsCloseServerShutdown), "server shutdown")
			return
		}
	}
}

// Close sends a WebSocket close frame with the given close code and reason.
func (c *Client) Close(closeCode int, reason string) {
	c.Conn.Close(websocket.StatusCode(closeCode), reason)
}

func (c *Client) handleMessage(data []byte) {
	var msg domain.WsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Error(context.Background(), "websocket message unmarshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		c.sendMessage(domain.WsTypeError, domain.WsErrorData{
			Code:    "INVALID_JSON",
			Message: "failed to parse message",
		}, "")
		return
	}

	switch msg.Type {
	case domain.WsTypeRoomJoin:
		c.handleJoin(msg.Data)
	case domain.WsTypeRoomLeave:
		c.handleLeave(msg.Data)
	case domain.WsTypeTyping:
		c.handleTyping(msg.Data)
	case domain.WsTypeHeartbeat:
		c.handleHeartbeat(context.Background())
	default:
		c.sendMessage(domain.WsTypeError, domain.WsErrorData{
			Code:    "UNKNOWN_TYPE",
			Message: "unknown message type: " + string(msg.Type),
		}, "")
	}
}

func (c *Client) handleJoin(data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		c.logger.Error(context.Background(), "websocket handle join marshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		return
	}

	var room domain.WsRoomJoin
	if err := json.Unmarshal(raw, &room); err != nil {
		c.logger.Error(context.Background(), "websocket handle join unmarshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		return
	}

	if !domain.IsValidRoomName(room.Room) {
		c.sendMessage(domain.WsTypeError, domain.WsErrorData{
			Code:    "INVALID_ROOM",
			Message: "invalid room name format: " + room.Room,
		}, "")
		return
	}

	if err := c.Hub.JoinRoom(c, room.Room); err != nil {
		c.logger.Error(context.Background(), "websocket join room error",
			logger.String("client_id", c.ID.String()),
			logger.String("room", room.Room),
			logger.Err(err),
		)
	}
}

func (c *Client) handleLeave(data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		c.logger.Error(context.Background(), "websocket handle leave marshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		return
	}

	var room domain.WsRoomLeave
	if err := json.Unmarshal(raw, &room); err != nil {
		c.logger.Error(context.Background(), "websocket handle leave unmarshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		return
	}

	c.Hub.LeaveRoom(c, room.Room)
}

func (c *Client) handleTyping(data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		c.logger.Error(context.Background(), "websocket handle typing marshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		return
	}

	var typing domain.WsTyping
	if err := json.Unmarshal(raw, &typing); err != nil {
		c.logger.Error(context.Background(), "websocket handle typing unmarshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		return
	}

	typingData := domain.WsTypingData{
		UserID:     c.UserID,
		Room:       typing.Room,
		EntityType: typing.EntityType,
		EntityID:   typing.EntityID,
	}

	c.Hub.PublishToRoom(typing.Room, domain.WsMessage{
		Type:      domain.WsTypeTyping,
		Data:      typingData,
		Timestamp: time.Now().UTC(),
		Room:      typing.Room,
	})
}

func (c *Client) handleHeartbeat(ctx context.Context) {
	if err := c.Hub.presence.RefreshHeartbeat(ctx, c.OrgID, c.UserID); err != nil {
		c.logger.Error(ctx, "websocket heartbeat refresh error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
	}
}

func (c *Client) sendMessage(msgType domain.WsMessageType, data any, room string) {
	msg := domain.WsMessage{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now().UTC(),
		Room:      room,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error(context.Background(), "websocket send message marshal error",
			logger.String("client_id", c.ID.String()),
			logger.Err(err),
		)
		return
	}

	select {
	case c.Send <- payload:
	default:
	}
}