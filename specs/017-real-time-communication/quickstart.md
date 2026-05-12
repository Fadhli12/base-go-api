# Quickstart: Real-time Communication (WebSocket)

**Branch**: `017-real-time-communication` | **Date**: 2026-05-12

## 1. Connect to WebSocket

```bash
# Connect with JWT token in query parameter
wscat -c "ws://localhost:8080/ws?token=eyJhbGciOiJIUzI1NiIs..."

# On successful connection, you receive:
# {"type":"ws.connected","data":{"user_id":"550e8400-e29b-41d4-a716-446655440000","rooms":[]}}
```

## 2. Join a Room

```json
// Client sends:
{"type":"ws.room.join","data":{"room":"org:550e8400-e29b-41d4-a716-446655440000"}}

// Server confirms:
{"type":"ws.room.joined","data":{"room":"org:550e8400-e29b-41d4-a716-446655440000"},"timestamp":"2026-05-12T10:30:00Z"}

// Join an entity-specific room:
{"type":"ws.room.join","data":{"room":"org:550e8400-e29b-41d4-a716-446655440000:invoice:660e8400-e29b-41d4-a716-446655440001"}}

// Server confirms:
{"type":"ws.room.joined","data":{"room":"org:550e8400...:invoice:660e8400..."},"timestamp":"2026-05-12T10:30:01Z"}
```

## 3. Receive Events

```json
// When an invoice is paid, all clients in the org room receive:
{
    "type": "ws.event",
    "data": {
        "event": "invoice.paid",
        "payload": {"id":"660e8400-...","status":"paid","amount":1500},
        "room": "org:550e8400-..."
    },
    "timestamp": "2026-05-12T10:31:00Z"
}

// Entity-specific rooms also receive:
{
    "type": "ws.event",
    "data": {
        "event": "invoice.paid",
        "payload": {"id":"660e8400-...","status":"paid","amount":1500},
        "room": "org:550e8400-...:invoice:660e8400-..."
    },
    "timestamp": "2026-05-12T10:31:00Z"
}
```

## 4. Send Typing Indicator

```json
// Client sends:
{"type":"ws.typing","data":{"room":"org:550e8400-...:invoice:660e8400-...","entity_type":"invoice","entity_id":"660e8400-..."}}

// Other clients in the room receive (typing sender excluded):
{"type":"ws.typing","data":{"user_id":"550e8400-...","room":"org:550e8400-...:invoice:660e8400-...","entity_type":"invoice","entity_id":"660e8400-..."},"timestamp":"2026-05-12T10:32:00Z"}
```

## 5. Presence Notifications

```json
// When a user comes online in your org:
{"type":"ws.presence.online","data":{"user_id":"770e8400-...","org_id":"550e8400-..."},"timestamp":"2026-05-12T10:33:00Z"}

// When a user goes offline:
{"type":"ws.presence.offline","data":{"user_id":"770e8400-...","org_id":"550e8400-..."},"timestamp":"2026-05-12T10:35:00Z"}
```

## 6. Leave a Room

```json
// Client sends:
{"type":"ws.room.leave","data":{"room":"org:550e8400-..."}}

// Server confirms:
{"type":"ws.room.left","data":{"room":"org:550e8400-..."},"timestamp":"2026-05-12T10:36:00Z"}
```

## 7. Heartbeat

```json
// Client sends periodically (every 30s):
{"type":"ws.heartbeat","data":{}}

// No response is sent. Heartbeat refreshes presence TTL on the server.
```

## 8. Error Messages

```json
// Invalid room format:
{"type":"ws.error","data":{"code":"INVALID_ROOM","message":"room name must be org:{uuid} or org:{uuid}:{type}:{uuid}"},"timestamp":"2026-05-12T10:37:00Z"}

// Room limit exceeded:
{"type":"ws.error","data":{"code":"ROOM_LIMIT","message":"maximum rooms per connection exceeded"},"timestamp":"2026-05-12T10:38:00Z"}

// Unknown message type:
{"type":"ws.error","data":{"code":"UNKNOWN_MESSAGE_TYPE","message":"unknown message type: ws.invalid"},"timestamp":"2026-05-12T10:39:00Z"}
```

## 9. Server Shutdown

```json
// Server sends before close:
{"type":"ws.server.shutdown","data":{"message":"server shutting down"},"timestamp":"2026-05-12T10:40:00Z"}

// Connection closed with code 4002 (ServerShutdown)
// Client should reconnect with exponential backoff (1s → 30s, ±500ms jitter)
```

## 10. REST Endpoints

```bash
# Get online users in an organization (requires websocket:connect permission)
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/presence/org/550e8400-e29b-41d4-a716-446655440000

# Response:
{
  "data": {
    "org_id": "550e8400-e29b-41d4-a716-446655440000",
    "online_users": ["550e8400-...", "770e8400-..."],
    "count": 2
  }
}

# Get available rooms for the current user
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/ws/rooms

# Response:
{
  "data": {
    "rooms": [
      {"room": "org:550e8400-...", "name": "Organization Main"},
      {"room": "org:550e8400-...:invoice:660e8400-...", "name": "Invoice #660e8400"}
    ]
  }
}
```

## 11. Custom Close Codes

| Code | Name | Meaning |
|------|------|---------|
| 1000 | Normal | Normal closure |
| 1001 | Going Away | Server shutting down |
| 4001 | Auth Expired | JWT expired, re-authenticate |
| 4002 | Server Shutdown | Graceful shutdown in progress |
| 4003 | Policy Violation | Unauthorized room or limit exceeded |
| 4004 | Room Limit | Too many rooms per connection |

## 12. Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `WS_ENABLED` | true | Enable/disable WebSocket |
| `WS_HEARTBEAT_INTERVAL` | 30s | Ping interval |
| `WS_HEARTBEAT_TIMEOUT` | 60s | Pong timeout |
| `WS_MAX_MESSAGE_SIZE` | 65536 | Max message size (64KB) |
| `WS_MAX_ROOMS_PER_CLIENT` | 50 | Max rooms per connection |
| `WS_WRITE_TIMEOUT` | 10s | Write deadline |
| `WS_READ_TIMEOUT` | 60s | Read deadline |
| `WS_TYPING_TTL` | 5s | Typing indicator TTL |
| `WS_PRESENCE_TTL` | 70s | Presence heartbeat TTL |

## 13. Go Client Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    conn, _, err := websocket.Dial(ctx, "ws://localhost:8080/ws?token=<jwt>", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close(websocket.StatusNormalClosure, "")

    // Read loop
    go func() {
        for {
            var msg map[string]interface{}
            err := wsjson.Read(ctx, conn, &msg)
            if err != nil {
                log.Println("read error:", err)
                return
            }
            fmt.Printf("received: %+v\n", msg)
        }
    }()

    // Join org room
    wsjson.Write(ctx, conn, map[string]interface{}{
        "type": "ws.room.join",
        "data": map[string]string{"room": "org:<org-id>"},
    })

    // Heartbeat loop
    ticker := time.NewTicker(25 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        wsjson.Write(ctx, conn, map[string]interface{}{
            "type": "ws.heartbeat",
            "data": map[string]interface{}{},
        })
    }
}
```