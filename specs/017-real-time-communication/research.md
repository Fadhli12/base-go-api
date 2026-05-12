# Research: Real-time Communication (WebSocket)

**Branch**: `017-real-time-communication` | **Date**: 2026-05-12

## 1. Existing Codebase Patterns

### 1.1 EventBus Pattern (`internal/domain/webhook_events.go`)

**Pattern**: Go channels with Subscribe/Publish/Start/Stop

```go
type EventBus struct {
    mu       sync.RWMutex
    buffer   int
    handlers []EventHandler
    ch       chan WebhookEvent
    ctx      context.Context
    cancel   context.CancelFunc
    wg       sync.WaitGroup
}

func NewEventBus(bufferSize int) *EventBus
func (eb *EventBus) Start(ctx context.Context) error
func (eb *EventBus) Stop() error
func (eb *EventBus) Subscribe(handler EventHandler)
func (eb *EventBus) Publish(event WebhookEvent) error
```

**Key decisions for WebSocket**:
- Hub follows same Start/Stop pattern with WaitGroup
- EventBus bridge is a handler registered via `Subscribe()`
- Context cancellation propagates shutdown signal
- Buffer size defaults (256 for EventBus) → configurable for Hub

### 1.2 Redis Queue Pattern (`internal/service/webhook_queue_redis.go`)

**Pattern**: Interface + Redis implementation with post-construction wiring

```go
type WebhookQueue interface {
    Enqueue(ctx context.Context, delivery *WebhookDelivery, score float64) error
    Dequeue(ctx context.Context) (*WebhookDelivery, error)
    // ...
}

type RedisWebhookQueue struct { ... }

// Post-construction wiring
func (s *WebhookService) SetQueue(queue WebhookQueue)
```

**Key decisions for WebSocket**:
- RedisSubscriber is an interface for testability
- Miniredis for unit tests (same pattern as webhook tests)
- `SetRedisSubscriber()` setter for post-construction wiring

### 1.3 Worker Pattern (`internal/service/webhook_worker.go`)

**Pattern**: Goroutine pool with Start/Stop, configurable concurrency

```go
type WebhookWorker struct {
    service  *WebhookService
    quit     chan struct{}
    wg       sync.WaitGroup
    // ...
}

func (w *WebhookWorker) Start() { /* spawn goroutines */ }
func (w *WebhookWorker) Stop()  { /* signal quit, wait with timeout */ }
```

**Key decisions for WebSocket**:
- Hub.Run() follows same pattern: goroutine + WaitGroup + quit channel
- Client has separate read/write pump goroutines (WebSocket-specific)
- Graceful shutdown: send `ws.server.shutdown` message, drain with 10s timeout

### 1.4 Domain Entity Pattern (`internal/domain/webhook.go`)

**Pattern**: UUID PK, soft delete, OrganizationID, TableName(), ToResponse()

```go
type Webhook struct {
    ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    OrganizationID *uuid.UUID     `gorm:"type:uuid;index"`
    // ...
    DeletedAt      gorm.DeletedAt `gorm:"index"`
}
func (Webhook) TableName() string { return "webhooks" }
func (w *Webhook) ToResponse() WebhookResponse { ... }
```

**Key decisions for WebSocket**:
- WsMessage is a DTO, not a GORM entity (no DB table)
- No soft delete needed (ephemeral connections)
- Domain types define constants and validation only

### 1.5 Config Pattern (`internal/config/webhook.go`)

**Pattern**: Config struct with env tags + DefaultXxxConfig() function

```go
type WebhookConfig struct {
    WorkerConcurrency     int           `mapstructure:"worker_concurrency"`
    RetryMax              int           `mapstructure:"retry_max"`
    // ...
}
func DefaultWebhookConfig() WebhookConfig { ... }
```

**Key decisions for WebSocket**: Same pattern for WsConfig

### 1.6 Handler & Permission Pattern

**Pattern**: Handler struct with deps, RegisterRoutes, per-endpoint permission middleware

```go
// In server.go
webhookHandler := handler.NewWebhookHandler(webhookService, s.enforcer, s.logger)
webhookHandler.RegisterRoutes(v1, jwtSecret)

// Permission seeding in cmd/api/main.go
{Name: "websocket:connect", Description: "...", Resource: "websocket", Action: "connect"}
```

**Key decisions for WebSocket**: Same pattern for WebSocketHandler

### 1.7 DI Wiring Pattern (`internal/http/server.go`)

```go
// Order: Repo → Service → Handler → RegisterRoutes
webhookRepo := repository.NewWebhookRepository(db)
webhookService := service.NewWebhookService(webhookRepo, ...)
webhookHandler := handler.NewWebhookHandler(webhookService, s.enforcer, s.logger)
webhookHandler.RegisterRoutes(v1, jwtSecret)
```

**Key decisions for WebSocket**: Same wiring order. Hub is a singleton service, not a handler.

### 1.8 Shutdown Order (`cmd/api/main.go`)

```
server → eventBus → workers → enforcer → db → redis
```

**With WebSocket Hub**:
```
server → eventBus → wsHub → workers → enforcer → db → redis
```

Hub is stopped after EventBus (to drain events) but before workers.

## 2. WebSocket Library Research

### 2.1 coder/websocket (CHOSEN)

**Package**: `github.com/coder/websocket`
**Fork of**: gorilla/webschema
**Status**: Actively maintained by Coder (formerly coder/coder)
**Why chosen over gorilla/websocket**:
- gorilla/websocket is archived (2023) and no longer maintained
- coder/websocket is a maintained fork with identical API, added context support, and production use at Coder
- Echo v4 compatible via `http.Handler` wrapping
- Supports all close codes including custom 4xxx range
- Net/http compatible (no framework lock-in)

**Key API**:
```go
// Upgrade
conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
    OriginPatterns: []string{"*"},
})

// Read
msgType, data, err := conn.Read(ctx)

// Write
err := conn.Write(ctx, websocket.MessageText, data)

// Close
conn.Close(websocket.StatusNormalClosure, "shutdown")
```

### 2.2 nhooyr.io/websocket (NOT CHOSEN)

- Modern API with context support
- Smaller community, less Echo integration examples
- Different API style from gorilla (learning curve)

### 2.3 gorilla/websocket (NOT CHOSEN)

- Archived since 2023
- No longer receives security patches
- Community fork (coder/websocket) is the recommended migration path

## 3. Redis Pub/Sub for Multi-Instance Scaling

### 3.1 Architecture

```
Instance A                          Instance B
┌──────────┐     Redis PubSub      ┌──────────┐
│  Hub     │◄────────────────────►│  Hub     │
│  Clients │   ws:msg:{roomID}    │  Clients │
│  A1,A2   │   ws:presence:{orgID}│  B1,B2   │
└──────────┘                       └──────────┘
```

### 3.2 Channel Design

- `ws:msg:{roomID}` — Room broadcast messages (JSON-serialized WsMessage)
- `ws:presence:{orgID}` — Presence change events (online/offline)

### 3.3 Subscriber Pattern

```go
type RedisSubscriber struct {
    redis  *redis.Client
    hub    *Hub
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func (rs *RedisSubscriber) Subscribe(room string) error
func (rs *RedisSubscriber) Unsubscribe(room string) error
func (rs *RedisSubscriber) Publish(ctx context.Context, room string, msg []byte) error
```

### 3.4 Graceful Degradation

- If Redis is unavailable, local Hub continues working for single-instance
- Publish errors are logged but not fatal
- Subscriber auto-reconnects with exponential backoff
- Presence degrades to local-only (no cross-instance presence sync)

## 4. coder/websocket Integration with Echo v4

### 4.1 Upgrade Handler Pattern

```go
func (h *WebSocketHandler) HandleUpgrade(c echo.Context) error {
    // 1. Extract JWT from query param
    token := c.QueryParam("token")
    if token == "" {
        return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
    }

    // 2. Validate JWT
    claims, err := h.authService.ValidateToken(token)
    if err != nil {
        return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
    }

    // 3. Upgrade connection
    conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
        OriginPatterns: []string{"*"}, // configure per env
    })
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "upgrade failed")
    }

    // 4. Create client and register
    client := NewClient(conn, claims.UserID, claims.OrgID, h.hub, h.logger)
    h.hub.Register(client)

    // 5. Start pumps (blocks until disconnect)
    client.Run(c.Request().Context())

    return nil
}
```

### 4.2 Read/Write Pump Pattern

Following gorilla/coder websocket chat example:

```go
func (c *Client) ReadPump(ctx context.Context) {
    defer c.Close()
    for {
        msgType, data, err := c.conn.Read(ctx)
        if err != nil { break }
        c.handleMessage(data)
    }
}

func (c *Client) WritePump(ctx context.Context) {
    defer c.Close()
    for {
        select {
        case msg, ok := <-c.send:
            if !ok { return }
            c.conn.Write(ctx, websocket.MessageText, msg)
        case <-ctx.Done():
            c.conn.Close(websocket.StatusNormalClosure, "shutdown")
        }
    }
}
```

## 5. Presence System Research

### 5.1 Redis Key Design

| Key | Type | TTL | Purpose |
|-----|------|-----|---------|
| `ws:presence:{orgID}` | SET | 70s | Member user IDs of online users in org |
| `ws:presence:{orgID}:{userID}` | STRING | 70s | Last heartbeat timestamp (epoch ms) |

### 5.2 Heartbeat Flow

1. Client connects → `SADD ws:presence:{orgID} {userID}` + `SET ws:presence:{orgID}:{userID} {timestamp} EX 70`
2. Client sends heartbeat every 30s → refresh TTL
3. Server pings client every 30s → client must pong within 60s
4. Client disconnects → `SREM ws:presence:{orgID} {userID}` + `DEL ws:presence:{orgID}:{userID}`
5. Stale entries cleaned by TTL expiry (70s = 2x heartbeat interval + buffer)

### 5.3 Deduplication

Multiple connections from same user (tabs/devices):
- `SADD` is idempotent — adding same user ID multiple times doesn't create duplicates
- `DEL` on disconnect only removes the key, not the SET member — need `SREM` on explicit disconnect
- If user has 3 tabs and closes one, `SREM` would incorrectly remove them — need reference counting

**Solution**: Use a Redis HASH per user tracking connection count:
```
ws:presence:{orgID}              → SET of online user IDs
ws:conn:{orgID}:{userID}         → STRING (connection count) EX 70s
```

On connect: `SADD ws:presence:{orgID} {userID}` + `INCR ws:conn:{orgID}:{userID}` + `EXPIRE ws:conn:{orgID}:{userID} 70`
On disconnect: `DECR ws:conn:{orgID}:{userID}`; if 0 → `SREM ws:presence:{orgID} {userID}` + `DEL ws:conn:{orgID}:{userID}`
Heartbeat: `EXPIRE ws:conn:{orgID}:{userID} 70` (refresh TTL)

## 6. Room Naming Convention Research

Based on existing org-scoped patterns in codebase:

```go
const (
    RoomOrgPrefix    = "org"     // org:{orgID}
    RoomEntityPrefix = "entity"  // org:{orgID}:{entityType}:{entityID}
)

var ValidEntityTypes = map[string]bool{
    "invoice": true,
    "news":    true,
    "media":   true,
    "comment": true,
}

func IsValidRoom(name string) bool {
    parts := strings.Split(name, ":")
    if len(parts) == 2 && parts[0] == "org" {
        _, err := uuid.Parse(parts[1])
        return err == nil
    }
    if len(parts) == 4 && parts[0] == "org" && ValidEntityTypes[parts[2]] {
        _, err1 := uuid.Parse(parts[1])
        _, err2 := uuid.Parse(parts[3])
        return err1 == nil && err2 == nil
    }
    return false
}
```

## 7. Testing Strategy

### 7.1 Unit Tests

- **Hub**: Room join/leave, broadcast, unregister, room validation
- **Client**: Message handling, heartbeat, write deadline
- **Presence**: Redis SET operations, heartbeat refresh, stale cleanup, connection counting
- **RedisSubscriber**: Pub/sub publish, subscribe, unsubscribe, reconnect

All unit tests use miniredis for Redis mocking.

### 7.2 Integration Tests

- **WebSocket connection**: Full upgrade + join room + send message + receive message
- **Presence**: Connect → check presence → disconnect → check presence cleared
- **Multi-instance simulation**: Two Hub instances sharing miniredis, verify cross-instance delivery
- **Auth**: Valid token → connect, expired token → 401, invalid token → 401

Uses `net/http/httptest.Server` + `coder/websocket` client for real WebSocket connections.