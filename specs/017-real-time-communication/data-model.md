# Data Model: Real-time Communication (WebSocket)

**Branch**: `017-real-time-communication` | **Date**: 2026-05-12

## Overview

This feature uses **no SQL tables** — all state is held in Go memory (Hub) and Redis (presence, pub/sub). The "data model" here describes the in-memory Go structs and Redis key patterns.

## 1. In-Memory Go Structs

### 1.1 WsMessage (Domain DTO)

```go
type WsMessageType string

const (
    WsTypeConnected       WsMessageType = "ws.connected"
    WsTypeRoomJoined      WsMessageType = "ws.room.joined"
    WsTypeRoomLeft        WsMessageType = "ws.room.left"
    WsTypeEvent           WsMessageType = "ws.event"
    WsTypeTyping          WsMessageType = "ws.typing"
    WsTypeHeartbeat       WsMessageType = "ws.heartbeat"
    WsTypePresenceOnline  WsMessageType = "ws.presence.online"
    WsTypePresenceOffline WsMessageType = "ws.presence.offline"
    WsTypeError           WsMessageType = "ws.error"
    WsTypeServerShutdown  WsMessageType = "ws.server.shutdown"
)

type WsMessage struct {
    Type      WsMessageType `json:"type"`
    Data      any            `json:"data"`
    Timestamp time.Time      `json:"timestamp"`
    Room      string         `json:"room,omitempty"`
}
```

### 1.2 WsRoomJoin (Incoming)

```go
type WsRoomJoin struct {
    Room string `json:"room" validate:"required"`
}

type WsRoomLeave struct {
    Room string `json:"room" validate:"required"`
}

type WsTyping struct {
    Room       string `json:"room" validate:"required"`
    EntityType string `json:"entity_type,omitempty"`
    EntityID   string `json:"entity_id,omitempty"`
}
```

### 1.3 WsConnectedData (Outgoing)

```go
type WsConnectedData struct {
    UserID uuid.UUID `json:"user_id"`
    Rooms  []string  `json:"rooms"`
}

type WsRoomData struct {
    Room string `json:"room"`
}

type WsEventData struct {
    Event   string `json:"event"`
    Payload any    `json:"payload"`
    Room    string `json:"room"`
}

type WsTypingData struct {
    UserID     uuid.UUID `json:"user_id"`
    Room       string    `json:"room"`
    EntityType string    `json:"entity_type,omitempty"`
    EntityID   string    `json:"entity_id,omitempty"`
}

type WsPresenceData struct {
    UserID uuid.UUID `json:"user_id"`
    OrgID  uuid.UUID `json:"org_id"`
}

type WsErrorData struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### 1.4 WsCloseCode (Custom Close Codes)

```go
const (
    WsCloseAuthExpired       = 4001
    WsCloseServerShutdown    = 4002
    WsClosePolicyViolation   = 4003
    WsCloseRoomLimitExceeded = 4004
)
```

## 2. Hub (In-Memory)

### 2.1 Hub Struct

```go
type Hub struct {
    mu         sync.RWMutex
    clients    map[uuid.UUID]*Client     // userID → Client (latest connection; dedup via presence)
    rooms      map[string]map[*Client]bool // roomID → set of clients
    register   chan *Client
    unregister chan *Client
    broadcast  chan *BroadcastMessage
    presence   PresenceService
    redis      RedisSubscriber
    logger     logger.Logger
    config     WsConfig
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
}

type BroadcastMessage struct {
    Room string
    Data []byte
    From *Client // nil for system messages
}
```

### 2.2 Client Struct

```go
type Client struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    OrgID      uuid.UUID
    Conn       *websocket.Conn
    Hub        *Hub
    Send       chan []byte
    Rooms      map[string]bool
    logger     logger.Logger
    writeDeadline time.Duration
}
```

## 3. Redis Data Model

### 3.1 Presence Keys

| Key Pattern | Redis Type | TTL | Value | Purpose |
|-------------|-----------|-----|-------|---------|
| `ws:presence:{orgID}` | SET | — | member: userID strings | Online users in org (persistent, members managed by connect/disconnect) |
| `ws:conn:{orgID}:{userID}` | STRING | 70s | connection count (integer string) | Reference counting for multi-tab support |
| `ws:typing:{orgID}:{entityType}:{entityID}:{userID}` | STRING | 5s | timestamp (epoch ms) | Typing indicator with auto-expiry |

### 3.2 Pub/Sub Channels

| Channel Pattern | Direction | Purpose |
|----------------|-----------|---------|
| `ws:msg:{roomID}` | All instances ↔ Redis | Room broadcast messages |
| `ws:presence:{orgID}` | All instances ↔ Redis | Presence change events (online/offline) |

### 3.3 Presence Operations

```
# Connect (user joins)
SADD ws:presence:{orgID} {userID}
INCR ws:conn:{orgID}:{userID}
EXPIRE ws:conn:{orgID}:{userID} 70

# Heartbeat (every 30s)
EXPIRE ws:conn:{orgID}:{userID} 70

# Disconnect (user leaves)
DECR ws:conn:{orgID}:{userID}
# If result == 0:
SREM ws:presence:{orgID} {userID}
DEL ws:conn:{orgID}:{userID}

# Query presence (REST endpoint)
SMEMBERS ws:presence:{orgID}
```

### 3.4 Pub/Sub Message Format

Messages published to `ws:msg:{roomID}` are JSON-serialized `WsMessage`:

```json
{
    "type": "ws.event",
    "data": {"event": "invoice.paid", "payload": {...}},
    "timestamp": "2026-05-12T10:30:00Z",
    "room": "org:123"
}
```

Messages published to `ws:presence:{orgID}` are JSON-serialized:

```json
{
    "type": "ws.presence.online",
    "data": {"user_id": "uuid", "org_id": "uuid"}
}
```

## 4. Configuration

### 4.1 WsConfig

```go
type WsConfig struct {
    Enabled            bool          `mapstructure:"ws_enabled"`
    AllowedOrigins     []string      `mapstructure:"ws_allowed_origins"`
    HeartbeatInterval  time.Duration `mapstructure:"ws_heartbeat_interval"`
    HeartbeatTimeout   time.Duration `mapstructure:"ws_heartbeat_timeout"`
    MaxMessageSize     int64         `mapstructure:"ws_max_message_size"`
    MaxRoomsPerClient  int           `mapstructure:"ws_max_rooms_per_client"`
    WriteTimeout       time.Duration `mapstructure:"ws_write_timeout"`
    ReadTimeout        time.Duration `mapstructure:"ws_read_timeout"`
    TypingTTL         time.Duration `mapstructure:"ws_typing_ttl"`
    PresenceTTL       time.Duration `mapstructure:"ws_presence_ttl"`
    SendChannelBuffer  int           `mapstructure:"ws_send_channel_buffer"`
}
```

### 4.2 Defaults

```go
func DefaultWsConfig() WsConfig {
    return WsConfig{
        Enabled:           true,
        AllowedOrigins:     []string{"*"}, // Allow all origins in dev; restrict in production
        HeartbeatInterval:  30 * time.Second,
        HeartbeatTimeout:    60 * time.Second,
        MaxMessageSize:      65536, // 64KB
        MaxRoomsPerClient:   50,
        WriteTimeout:        10 * time.Second,
        ReadTimeout:         60 * time.Second,
        TypingTTL:           5 * time.Second,
        PresenceTTL:        70 * time.Second,
        SendChannelBuffer:   256,
    }
}
```

## 5. SQL Migration (000024)

The migration file exists but is **empty** — no database tables are needed for WebSocket in v1. All state is managed in Go memory and Redis.

**000024_websocket.up.sql**:
```sql
-- No tables needed for v1 WebSocket implementation.
-- All state is managed in Go memory (Hub) and Redis (presence, pub/sub).
-- Migration reserved for future extensibility (connection logs, room configs).
```

**000024_websocket.down.sql**:
```sql
-- No tables to drop for v1 WebSocket implementation.
```

## 6. Message Flow Diagrams

### 6.1 Connection Flow

```
Client → GET /ws?token=JWT
        → WebSocketHandler.HandleUpgrade()
        → Validate JWT
        → websocket.Accept()
        → Create Client
        → Hub.Register(client)
        → Start ReadPump goroutine
        → Start WritePump goroutine
        → Send ws.connected message
        → Start heartbeat ticker
```

### 6.2 Room Join Flow

```
Client → {"type":"ws.room.join","data":{"room":"org:123"}}
        → Client.ReadPump()
        → Validate room format
        → Hub.JoinRoom(client, "org:123")
        → RedisSubscriber.Subscribe("ws:msg:org:123")
        → Add client to Hub.rooms["org:123"]
        → Send ws.room.joined to client
        → Broadcast ws.presence.online to room
```

### 6.3 Message Broadcast Flow (Local)

```
Service → Hub.PublishToRoom("org:123", event)
        → Hub.broadcast ← BroadcastMessage
        → For each client in Hub.rooms["org:123"]
        → client.Send ← serialized WsMessage
```

### 6.4 Message Broadcast Flow (Cross-Instance)

```
Instance A → Hub.PublishToRoom("org:123", event)
           → RedisSubscriber.Publish("ws:msg:org:123", data)
           → Redis publishes to channel

Instance B → RedisSubscriber receives message
           → Hub.BroadcastToRoom("org:123", data)
           → For each client in Hub.rooms["org:123"]
           → client.Send ← serialized WsMessage
```

### 6.5 Disconnect Flow

```
Client closes → ReadPump returns error
             → Client.Close()
             → Hub.Unregister(client)
             → For each room client was in:
               → Remove client from Hub.rooms[room]
               → If room empty: RedisSubscriber.Unsubscribe("ws:msg:"+room)
             → PresenceService.MarkOffline(orgID, userID)
             → Broadcast ws.presence.offline to org room
```