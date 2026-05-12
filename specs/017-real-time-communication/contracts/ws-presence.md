# Interface Contract: PresenceService & RedisSubscriber

**Branch**: `017-real-time-communication` | **Date**: 2026-05-12

## PresenceService

```go
package service

import (
    "context"

    "github.com/google/uuid"
)

// PresenceService manages online/offline status of users per organization.
// Uses Redis for multi-instance coordination with reference counting
// to handle multiple connections per user (tabs/devices).
type PresenceService interface {
    // MarkOnline registers a user as online in an organization.
    // Increments connection count. If first connection, adds to presence set
    // and publishes ws.presence.online via Redis pub/sub.
    MarkOnline(ctx context.Context, orgID, userID uuid.UUID) error

    // MarkOffline decrements connection count for a user in an organization.
    // If count reaches 0, removes from presence set and publishes
    // ws.presence.offline via Redis pub/sub.
    MarkOffline(ctx context.Context, orgID, userID uuid.UUID) error

    // RefreshHeartbeat extends the TTL for a user's presence entry.
    // Called on every ws.heartbeat message and periodic ping.
    RefreshHeartbeat(ctx context.Context, orgID, userID uuid.UUID) error

    // IsOnline checks if a user is currently online in an organization.
    IsOnline(ctx context.Context, orgID, userID uuid.UUID) (bool, error)

    // GetOnlineUsers returns the list of user IDs currently online in an organization.
    GetOnlineUsers(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)

    // GetOnlineCount returns the number of online users in an organization.
    GetOnlineCount(ctx context.Context, orgID uuid.UUID) (int64, error)
}
```

## PresenceService Implementation

```go
// RedisPresence implements PresenceService using Redis.
type RedisPresence struct {
    redis  *redis.Client
    config WsConfig
    logger logger.Logger
}

func NewRedisPresence(redisClient *redis.Client, logger logger.Logger, config WsConfig) *RedisPresence

// Key patterns:
// ws:presence:{orgID}         → SET of online user IDs
// ws:conn:{orgID}:{userID}    → STRING of connection count (with TTL)
```

## RedisSubscriber

```go
package service

import (
    "context"
)

// RedisSubscriber manages Redis pub/sub subscriptions for cross-instance
// WebSocket message delivery.
type RedisSubscriber interface {
    // Subscribe subscribes to messages for a specific room channel.
    // If already subscribed, this is a no-op (reference counted).
    Subscribe(ctx context.Context, room string) error

    // Unsubscribe unsubscribes from a room channel.
    // Only actually unsubscribes when reference count reaches 0.
    Unsubscribe(ctx context.Context, room string) error

    // Publish sends a message to a room's Redis channel.
    // All instances subscribed to the channel will receive it.
    Publish(ctx context.Context, channel string, data []byte) error

    // Start begins listening for Redis pub/sub messages.
    // Background goroutine that forwards messages to Hub.BroadcastToRoom().
    Start(ctx context.Context) error

    // Stop gracefully shuts down the subscriber.
    Stop() error
}

// RedisPubSub implements RedisSubscriber using Redis pub/sub.
type RedisPubSub struct {
    redis    *redis.Client
    hub      *Hub
    logger   logger.Logger
    ctx      context.Context
    cancel   context.CancelFunc
    wg       sync.WaitGroup
}
```

## Presence Operations (Redis Commands)

### Connect (User Comes Online)
```redis
SADD ws:presence:{orgID} {userID}
INCR ws:conn:{orgID}:{userID}
EXPIRE ws:conn:{orgID}:{userID} 70
# Publish presence change
PUBLISH ws:presence:{orgID} {"type":"ws.presence.online","data":{"user_id":"...","org_id":"..."}}
```

### Heartbeat (Every 30 Seconds)
```redis
EXPIRE ws:conn:{orgID}:{userID} 70
```

### Disconnect (User Goes Offline)
```redis
DECR ws:conn:{orgID}:{userID}
# If result == 0:
SREM ws:presence:{orgID} {userID}
DEL ws:conn:{orgID}:{userID}
# Publish presence change
PUBLISH ws:presence:{orgID} {"type":"ws.presence.offline","data":{"user_id":"...","org_id":"..."}}
```

### Query Presence (REST Endpoint)
```redis
SMEMBERS ws:presence:{orgID}
```

## Error Handling

| Error | Behavior |
|-------|----------|
| Redis unavailable (MarkOnline) | Log error, continue with local-only presence |
| Redis unavailable (Publish) | Log error, continue with local-only delivery |
| Redis available again | Auto-reconnect subscriber, re-subscribe to active rooms |
| Invalid orgID or userID | Return AppError |
| Presence TTL expiry | Auto-cleanup — user removed from online set |

## Graceful Degradation

When Redis is unavailable:
- **Presence**: Falls back to in-memory tracking per instance (users appear offline on other instances)
- **Cross-instance**: Messages only delivered to local clients
- **Error logging**: All Redis errors logged via `internal/logger` with context fields
- **Auto-recovery**: Subscriber attempts reconnection with exponential backoff (1s → 30s, ±500ms jitter)