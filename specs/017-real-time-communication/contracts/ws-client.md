# Interface Contract: Client

**Branch**: `017-real-time-communication` | **Date**: 2026-05-12

## Client

```go
package service

import (
    "context"
    "time"

    "github.com/coder/websocket"
    "github.com/google/uuid"
    "github.com/example/go-api-base/internal/logger"
)

// Client represents a single WebSocket connection.
// Each client has read and write pump goroutines.
type Client struct {
    ID            uuid.UUID          // Unique connection ID (different from UserID for multi-tab)
    UserID        uuid.UUID          // Authenticated user ID from JWT
    OrgID         uuid.UUID         // User's organization ID from JWT
    Conn          *websocket.Conn    // Underlying WebSocket connection
    Hub           *Hub              // Reference to hub for broadcasting
    Send          chan []byte        // Buffered channel for outbound messages
    Rooms         map[string]bool    // Set of rooms this client has joined
    logger        logger.Logger
    writeDeadline time.Duration
}

// NewClient creates a new Client with the given connection and user info.
func NewClient(conn *websocket.Conn, userID, orgID uuid.UUID, hub *Hub, logger logger.Logger, config WsConfig) *Client

// Run starts the client's read and write pump goroutines.
// Blocks until both goroutines complete (connection closed or context cancelled).
func (c *Client) Run(ctx context.Context)

// ReadPump reads messages from the WebSocket connection.
// Handles:
// - ws.room.join messages → Hub.JoinRoom()
// - ws.room.leave messages → Hub.LeaveRoom()
// - ws.typing messages → broadcast to room
// - ws.heartbeat messages → refresh presence TTL
// - Invalid messages → send ws.error
// On error or close, triggers Hub.Unregister()
func (c *Client) ReadPump(ctx context.Context)

// WritePump writes messages from the Send channel to the WebSocket connection.
// Handles:
// - Outbound messages from Hub.BroadcastToRoom()
// - Ping/pong heartbeat (configurable interval)
// - Write deadline enforcement
// On channel close or context cancellation, sends close frame and exits.
func (c *Client) WritePump(ctx context.Context)

// Close cleanly closes the WebSocket connection with a close code.
func (c *Client) Close(closeCode int, reason string)

// handleMessage routes an incoming JSON message to the appropriate handler.
func (c *Client) handleMessage(data []byte)

// handleJoin processes a ws.room.join message.
func (c *Client) handleJoin(data []byte)

// handleLeave processes a ws.room.leave message.
func (c *Client) handleLeave(data []byte)

// handleTyping processes a ws.typing message.
func (c *Client) handleTyping(data []byte)

// handleHeartbeat processes a ws.heartbeat message.
func (c *Client) handleHeartbeat()

// sendMessage serializes and sends a WsMessage to the client.
// Non-blocking: drops message if send buffer is full.
func (c *Client) sendMessage(msgType WsMessageType, data any, room string)
```

## Message Handling Matrix

| Incoming Type | Required Fields | Handler | Outgoing Response |
|---------------|----------------|---------|-------------------|
| `ws.room.join` | `room` (string) | `handleJoin` | `ws.room.joined` to self, `ws.presence.online` to room |
| `ws.room.leave` | `room` (string) | `handleLeave` | `ws.room.left` to self |
| `ws.typing` | `room`, optional `entity_type`, `entity_id` | `handleTyping` | `ws.typing` broadcast to room (not self) |
| `ws.heartbeat` | none | `handleHeartbeat` | (no response, refreshes presence TTL) |
| Unknown type | — | — | `ws.error` with `UNKNOWN_MESSAGE_TYPE` |

## Close Code Handling

| Scenario | Close Code | Reason String |
|----------|-----------|---------------|
| Normal disconnect | 1000 (Normal) | "" |
| Server shutdown | 4002 (ServerShutdown) | "server shutting down" |
| Auth expired | 4001 (AuthExpired) | "token expired" |
| Policy violation | 4003 (PolicyViolation) | "room limit exceeded" or "unauthorized room" |
| Internal error | 1011 (InternalError) | "internal error" |