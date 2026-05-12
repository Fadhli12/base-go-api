# Interface Contract: Hub

**Branch**: `017-real-time-communication` | **Date**: 2026-05-12

## Hub

```go
package service

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// Hub manages WebSocket client connections and room-based message broadcasting.
// It is a singleton service — one instance per server process.
type Hub struct {
    // fields unexported — access via methods
}

// NewHub creates a new Hub instance with the given dependencies.
func NewHub(presence PresenceService, redis RedisSubscriber, logger logger.Logger, config WsConfig) *Hub

// Start begins the hub's event processing goroutine.
// Must be called before any client registration.
// Follows EventBus pattern: Start(ctx) / Stop() with WaitGroup.
func (h *Hub) Start(ctx context.Context) error

// Stop gracefully shuts down the hub.
// Sends ws.server.shutdown to all connected clients.
// Waits for goroutines with 10s timeout.
// Follows EventBus pattern.
func (h *Hub) Stop() error

// Run processes hub events (register, unregister, broadcast) in a loop.
// Called internally by Start(). Blocks until context cancelled.
func (h *Hub) Run(ctx context.Context)

// Register adds a client to the hub.
// Starts the client's read and write pump goroutines.
// Updates presence to online.
// Sends ws.connected message to client.
func (h *Hub) Register(client *Client)

// Unregister removes a client from the hub.
// Removes client from all rooms.
// Updates presence to offline (decrement connection count).
// Closes the client's send channel.
func (h *Hub) Unregister(client *Client)

// JoinRoom adds a client to a room.
// Validates room format (org:{uuid} or org:{uuid}:{type}:{uuid}).
// Subscribes to Redis pub/sub for the room if first local client.
// Sends ws.room.joined confirmation to client.
// Returns error if room format is invalid or client has exceeded room limit.
func (h *Hub) JoinRoom(client *Client, room string) error

// LeaveRoom removes a client from a room.
// Unsubscribes from Redis pub/sub if last local client in room.
// Sends ws.room.left confirmation to client.
func (h *Hub) LeaveRoom(client *Client, room string)

// PublishToRoom sends a message to all clients in a room.
// Publishes to Redis pub/sub for cross-instance delivery.
// Local clients receive the message directly from the hub.
func (h *Hub) PublishToRoom(room string, event domain.WebhookEvent)

// BroadcastToRoom sends raw bytes to all local clients in a room.
// Used by RedisSubscriber to deliver cross-instance messages.
func (h *Hub) BroadcastToRoom(room string, data []byte)

// RoomsForEvent determines which rooms an EventBus event should be broadcast to.
// Returns org room + entity room if applicable.
func (h *Hub) RoomsForEvent(event domain.WebhookEvent) []string

// GetRoomClients returns the number of local clients in a room.
// Primarily for monitoring and debugging.
func (h *Hub) GetRoomClients(room string) int

// GetTotalConnections returns the total number of local connections.
// Primarily for monitoring and debugging.
func (h *Hub) GetTotalConnections() int
```

## Room Validation

```go
// IsValidRoomName validates room naming format.
// Valid formats:
//   - org:{uuid}                              (organization room)
//   - org:{uuid}:{entityType}:{uuid}          (entity room)
func IsValidRoomName(room string) bool

// ValidEntityTypes returns the set of valid entity types for room paths.
var ValidEntityTypes = map[string]bool{
    "invoice": true,
    "news":    true,
    "media":   true,
    "comment": true,
}
```

## Error Responses

```go
var (
    ErrInvalidRoomFormat  = errors.NewAppError("INVALID_ROOM", "room name must be org:{uuid} or org:{uuid}:{type}:{uuid}", 400)
    ErrRoomLimitExceeded  = errors.NewAppError("ROOM_LIMIT", "maximum rooms per connection exceeded", 429)
    ErrUnauthorizedRoom   = errors.NewAppError("UNAUTHORIZED", "not authorized to join this room", 403)
)
```