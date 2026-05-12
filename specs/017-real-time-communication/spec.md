# Feature Specification: Real-time Communication (WebSocket)

**Feature Branch**: `017-real-time-communication`
**Created**: 2026-05-12
**Status**: Clarified
**Priority**: P3
**Complexity**: High
**Estimated Effort**: 3-4 days
**Dependencies**: Redis (already available), EventBus pattern (already available)
**Input**: Real-time Communication (WebSocket) system — room-based hub with presence indicators, typing events, and Redis pub/sub for multi-instance scaling.

---

## Clarification Decisions

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| 1 | Transport protocol | **WebSocket only (coder/websocket)** | WebSocket is the standard for real-time bidirectional communication. coder/websocket is the actively maintained fork of gorilla/websocket by Coder, production-proven at scale. |
| 2 | Authentication | **JWT query param on upgrade** | Token in `/ws?token=<jwt>`. Simple, standard, validated on upgrade. Token refresh requires reconnect. Matches most production WebSocket implementations and avoids first-message complexity. |
| 3 | Room model | **Organization + entity rooms** | Follows existing org-scoped pattern (comments, tags, webhooks). `org:{orgID}` for org-wide, `org:{orgID}:{entityType}:{entityID}` for entity-specific. |
| 4 | Presence granularity | **Organization-level only (v1)** | Entity-level presence adds complexity and DB load. Org-level is sufficient for most use cases (who's online in my org). |
| 5 | Typing indicators | **Ephemeral — Redis TTL only** | No DB persistence. Use Redis key with 5s TTL. Subscriber broadcasts typing event; no storage, no audit. |
| 6 | Multi-instance scaling | **Redis pub/sub channels** | Follows webhook pattern (Redis sorted sets for queues). Each instance subscribes to relevant channels and broadcasts locally. |
| 7 | Message persistence | **Not in v1** | Real-time events are ephemeral. Offline messages are handled by the notification system (already implemented). WebSocket delivers live events only. |
| 8 | REST fallback endpoints | **HTTP endpoints for room membership** | WebSocket is for real-time delivery. Room join/leave, presence queries, and room listing have REST endpoints (following existing handler patterns). |
| 9 | Soft delete | **Not applicable** | WebSocket connections are ephemeral. Room membership is managed in Redis (no DB table). Presence state is Redis TTL-based. |
| 10| Event namespace | **Reuse EventBus + WebSocket channel** | Existing EventBus events (`user.created`, `invoice.paid`, etc.) are bridged to WebSocket rooms. New WebSocket-specific events use `ws.` prefix. |
| 11| EventBus bridge scope | **All existing events** | Bridge all webhook events (user.created, user.deleted, invoice.created, invoice.paid, news.published, news.deleted) plus any future events automatically. No selective whitelist needed — events are already filtered by room membership. |
| 12| Close codes | **Standard close codes** | Define custom WebSocket close codes: 4001 (auth expired), 4002 (server shutdown), 4003 (policy violation), 4004 (room limit exceeded). Clients implement exponential backoff reconnect. |
| 13| Testing strategy | **Unit + Integration** | Unit tests for Hub, Client, Presence, RedisSubscriber with mocks. Integration tests using httptest.Server + coder/websocket client for end-to-end connection/room/presence flows. |

---

## 1. Overview

Room-based WebSocket communication system for real-time collaboration, notifications, and presence indicators. The system uses a Hub-and-Client architecture where authenticated users join organization and entity-specific rooms, receive live events, and broadcast presence/typing status. Redis pub/sub enables multi-instance scaling.

## 2. Goals

- **WebSocket upgrade endpoint** at `/ws` with JWT authentication via query param (`/ws?token=<jwt>`)
- **Room-based broadcasting**: org rooms (`org:{orgID}`) and entity rooms (`org:{orgID}:{entityType}:{entityID}`)
- **Presence indicators**: online/offline status at org level, heartbeat mechanism
- **Typing indicators**: ephemeral typing events with 5s Redis TTL, no persistence
- **Multi-instance scaling**: Redis pub/sub channels for cross-instance message delivery
- **EventBus bridge**: existing application events (webhook events, notifications) relayed to WebSocket clients
- **REST endpoints**: room membership management and presence queries via standard HTTP
- **RBAC enforcement**: `websocket:connect`, `websocket:manage` permissions
- **Graceful shutdown**: drain connections with 10s timeout, follow existing shutdown order
- **Configurable**: heartbeat interval, max message size, write deadlines via env vars
- **SQL migrations only** — no AutoMigrate (Constitution Principle V)
- **Structured logging** — all connection/disconnect/broadcast events via `internal/logger` (Constitution Principle I)

## 3. User Scenarios & Testing

### User Story 1 — Connect and Receive Real-time Events (Priority: P1)

A user opens the application, establishes a WebSocket connection, joins their organization room, and receives live events (new notifications, invoice updates, news publications).

**Why this priority**: Core value proposition. Without connection and message delivery, nothing else matters. This is the MVP.

**Independent Test**: Client connects to `/ws?token=jwt`, joins `org:{orgID}` room, and receives a broadcast message within 2 seconds of a service publishing an event.

**Acceptance Scenarios**:

1. **Given** a valid JWT token, **When** client upgrades to WebSocket at `/ws?token=<jwt>`, **Then** connection is established and client receives `{"type":"ws.connected","data":{"user_id":"...","rooms":[]}}`
2. **Given** an authenticated WebSocket connection, **When** client sends `{"type":"ws.room.join","data":{"room":"org:123"} }`, **Then** client receives `{"type":"ws.room.joined","data":{"room":"org:123"}}`
3. **Given** client is in room `org:123`, **When** server publishes event to room `org:123`, **Then** client receives `{"type":"ws.event","data":{"event":"notification.created","payload":{...}}}`
4. **Given** an expired JWT, **When** client attempts WebSocket upgrade at `/ws?token=<expired_jwt>`, **Then** connection is rejected with HTTP 401

---

### User Story 2 — Presence and Typing Indicators (Priority: P2)

Users can see who else is online in their organization and broadcast typing indicators during collaborative editing.

**Why this priority**: Presence enables collaborative awareness but requires heartbeat infrastructure first. Typing is ephemeral UX polish.

**Independent Test**: Two clients connect; one sends a typing event; the other receives it. Both can query presence for their org.

**Acceptance Scenarios**:

1. **Given** two authenticated users in room `org:123`, **When** User A sends `{"type":"ws.typing","data":{"room":"org:123","entity_type":"invoice","entity_id":"..."}}`, **Then** User B receives `{"type":"ws.typing","data":{"user_id":"A","room":"org:123","entity_type":"invoice","entity_id":"..."}}`
2. **Given** a user connected, **When** their heartbeat stops for 60s, **Then** they are marked offline and room members receive `{"type":"ws.presence.offline","data":{"user_id":"..."}}`
3. **Given** a user connected, **When** client sends `GET /api/v1/presence/org/:orgID`, **Then** response contains list of online user IDs with timestamps

---

### User Story 3 — Multi-Instance Scaling (Priority: P3)

Multiple API instances can broadcast events to all connected clients across instances via Redis pub/sub.

**Why this priority**: Required for production but can be deferred behind single-instance mode for initial deployment.

**Independent Test**: Instance 1 and Instance 2 share a Redis. A client on Instance 1 sends a message to a room; a client on Instance 2 in the same room receives it.

**Acceptance Scenarios**:

1. **Given** two API instances with shared Redis, **When** a service on Instance 1 publishes to room `org:123`, **Then** clients on Instance 2 in room `org:123` receive the event
2. **Given** Redis is unavailable, **When** publish attempt fails, **Then** local clients still receive messages, error is logged, but service continues (graceful degradation)

---

### Edge Cases

- What happens when a client connects with a valid JWT that expires mid-connection? → Server detects via heartbeat timeout, disconnects with `ws.token.expired` close code
- What happens when Redis pub/sub is temporarily unavailable? → Local hub continues working for single-instance; cross-instance delivery degrades; errors logged; auto-reconnect on recovery
- What happens when a room has 10,000 members and one message is broadcast? → Hub fans out to all local clients; Redis pub/sub handles cross-instance; no per-client queue (fire-and-forget)
- What happens when a user opens multiple tabs (multiple connections)? → Each connection is independent; presence counts unique user IDs per org (dedup via Redis set)
- What happens when the server shuts down gracefully? → Drain connections with 10s timeout; send `ws.server.shutdown` message before close; follow shutdown order (server → eventBus → wsHub → workers)

## 4. Requirements

### Functional Requirements

- **FR-001**: System MUST provide a WebSocket upgrade endpoint at `/ws?token=<jwt>` that authenticates via JWT token in query param
- **FR-002**: System MUST support room-based message broadcasting with rooms identified as `org:{orgID}` and `org:{orgID}:{entityType}:{entityID}`
- **FR-003**: System MUST allow clients to join and leave rooms via WebSocket messages (`ws.room.join`, `ws.room.leave`)
- **FR-004**: System MUST broadcast messages to all clients in a room when a message is published to that room
- **FR-005**: System MUST support multi-instance scaling via Redis pub/sub channels with graceful degradation when Redis is unavailable
- **FR-006**: System MUST provide presence tracking — online/offline status per user per organization with 60s heartbeat timeout
- **FR-007**: System MUST provide typing indicators as ephemeral events (5s Redis TTL, no persistence)
- **FR-008**: System MUST bridge existing EventBus events to WebSocket rooms (notification, webhook, and application events)
- **FR-009**: System MUST enforce RBAC with `websocket:connect` (connect to WebSocket) and `websocket:manage` (manage rooms/presence) permissions
- **FR-010**: System MUST provide REST endpoints: `GET /api/v1/presence/org/:orgID` (online users, with audit logging), `GET /api/v1/ws/rooms` (available rooms, with audit logging)
- **FR-011**: System MUST limit message size to 64KB (configurable via `WS_MAX_MESSAGE_SIZE`)
- **FR-012**: System MUST implement read deadline with ping/pong heartbeat (30s interval, configurable)
- **FR-013**: System MUST follow graceful shutdown order: server → eventBus → wsHub → workers → enforcer → db → redis
- **FR-014**: System MUST use structured logging via `internal/logger` for all connection/disconnect/broadcast events
- **FR-015**: System MUST reject connections for suspended or deleted users (check on connect and periodically)
- **FR-016**: System MUST limit rooms per connection to 50 (configurable) and reject join requests beyond this limit
- **FR-017**: System MUST validate room names format: `org:{uuid}` or `org:{uuid}:{type}:{uuid}` where type is a valid entity type
- **FR-018**: System MUST validate WebSocket origins via configurable `WS_ALLOWED_ORIGINS` (default: `*` for development, must restrict in production)

### Key Entities

- **WebSocketMessage**: Envelope for all WebSocket communication (type, data, timestamp, room)
- **Client**: In-memory representation of a connected WebSocket client (userID, orgID, rooms, connections)
- **Hub**: Central room manager managing client registration and message broadcasting
- **RedisSubscriber**: Bridges Redis pub/sub channels to local hub for multi-instance scaling
- **Presence**: Per-org online user tracking stored in Redis sets with TTL-based heartbeat
- **WsConfig**: Configuration struct for WebSocket settings (heartbeat, max message size, limits)

## 5. Architecture

### 5.1 Component Overview

```
┌─────────────┐     WebSocket      ┌──────────────┐
│   Client    │◄──────────────────►│    Server     │
│  (Browser)  │    /ws?token=JWT   │              │
└─────────────┘                     │  ┌─────────┐│
                                    │  │  Hub    ││
┌─────────────┐     WebSocket      │  │ (rooms) ││
│   Client    │◄──────────────────►│  └────┬────┘│
│  (Mobile)   │                    │       │     │
└─────────────┘                     └───┬───┴─────┘
                                        │
                            ┌───────────┼───────────┐
                            │           │            │
                    ┌───────▼───┐ ┌──────▼────┐ ┌────▼───────┐
                    │ EventBus  │ │  Redis    │ │  Audit      │
                    │ (Go chan) │ │  Pub/Sub  │ │  Service    │
                    └──────┬────┘ └──────┬────┘ └────────────┘
                           │              │
                     ┌─────▼──────┐  ┌───▼────────┐
                     │  Services  │  │  Other      │
                     │  (bridge)  │  │  Instances  │
                     └────────────┘  └─────────────┘
```

### 5.2 Message Protocol

All WebSocket messages use JSON:

```json
// Client → Server
{"type": "ws.room.join",  "data": {"room": "org:123"}}
{"type": "ws.room.leave", "data": {"room": "org:123"}}
{"type": "ws.typing",     "data": {"room": "org:123", "entity_type": "invoice", "entity_id": "abc"}}
{"type": "ws.heartbeat",  "data": {}}

// Server → Client
{"type": "ws.connected",       "data": {"user_id": "...", "rooms": []}}
{"type": "ws.room.joined",     "data": {"room": "org:123"}}
{"type": "ws.room.left",       "data": {"room": "org:123"}}
{"type": "ws.event",           "data": {"event": "notification.created", "payload": {...}, "room": "org:123"}}
{"type": "ws.presence.online", "data": {"user_id": "...", "org_id": "123"}}
{"type": "ws.presence.offline","data": {"user_id": "...", "org_id": "123"}}
{"type": "ws.typing",          "data": {"user_id": "...", "room": "org:123", "entity_type": "invoice", "entity_id": "abc"}}
{"type": "ws.error",           "data": {"code": "ROOM_LIMIT_EXCEEDED", "message": "..."}}
```

### 5.3 WebSocket Message Types

| Type | Direction | Description |
|------|-----------|-------------|
| `ws.connected` | Server→Client | Sent on successful connection |
| `ws.room.join` | Client→Server | Request to join a room |
| `ws.room.joined` | Server→Client | Confirmation of room join |
| `ws.room.leave` | Client→Server | Request to leave a room |
| `ws.room.left` | Server→Client | Confirmation of room leave |
| `ws.event` | Server→Client | Application event broadcast to room |
| `ws.typing` | Bidirectional | Typing indicator (client sends, server broadcasts to room) |
| `ws.heartbeat` | Client→Server | Client heartbeat ping |
| `ws.presence.online` | Server→Client | User came online in org |
| `ws.presence.offline` | Server→Client | User went offline in org |
| `ws.error` | Server→Client | Error message |
| `ws.server.shutdown` | Server→Client | Server is shutting down gracefully |

### 5.4 WebSocket Close Codes

| Code | Name | Description |
|------|------|-------------|
| 1000 | Normal | Normal closure |
| 1001 | Going Away | Server shutting down |
| 4001 | Auth Expired | JWT token expired, client should re-authenticate |
| 4002 | Server Shutdown | Graceful shutdown in progress |
| 4003 | Policy Violation | Room limit exceeded or unauthorized room join |
| 4004 | Room Limit | Too many rooms joined on single connection (max 50) |

Clients **must** implement exponential backoff reconnection: initial 1s, max 30s, jitter ±500ms.

### 5.4 Redis Key Design

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `ws:presence:{orgID}` | SET | 70s (heartbeat+buffer) | Online user IDs for org |
| `ws:presence:{orgID}:{userID}` | STRING | 70s | Last heartbeat timestamp |
| `ws:typing:{orgID}:{entityType}:{entityID}:{userID}` | STRING | 5s | Typing indicator |
| `ws:room:{roomID}` | SET | — | Room members (optional, for presence query) |

Redis Pub/Sub channels:
- `ws:msg:{roomID}` — Room broadcast messages
- `ws:presence:{orgID}` — Presence change events

### 5.5 Integration Points

**EventBus bridge**: All existing application events are automatically relayed to WebSocket rooms:
- `user.created` → broadcast to `org:{orgID}`
- `user.deleted` → broadcast to `org:{orgID}`
- `invoice.created` → broadcast to `org:{orgID}` and `org:{orgID}:invoice:{id}`
- `invoice.paid` → broadcast to `org:{orgID}` and `org:{orgID}:invoice:{id}`
- `news.published` → broadcast to `org:{orgID}` and `org:{orgID}:news:{id}`
- `news.deleted` → broadcast to `org:{orgID}` and `org:{orgID}:news:{id}`
- Future events are bridged automatically — no config changes needed
- Global webhooks (no orgID) broadcast to all org rooms the user belongs to

**Shutdown integration**: Add wsHub.Stop() to existing graceful shutdown sequence in `cmd/api/main.go`:

```
server → eventBus → wsHub → workers → enforcer → db → redis
```

## 6. File Structure

```
internal/
├── domain/
│   ├── websocket.go              # WsMessage, WsMessageType constants, room validation
│   └── websocket_events.go       # EventBus ↔ WebSocket bridge, HubBroadcaster interface
├── service/
│   ├── websocket_hub.go           # Hub: room management, client registry, broadcast
│   ├── websocket_client.go        # Client: connection, read/write pumps
│   ├── websocket_presence.go      # Presence: Redis-based online tracking
│   └── websocket_redis.go         # RedisSubscriber: pub/sub bridge for multi-instance
├── http/
│   ├── handler/
│   │   └── websocket.go           # Handler: upgrade, REST presence/rooms endpoints
│   └── middleware/
│       └── websocket_auth.go      # JWT auth for WebSocket handshake
├── config/
│   └── websocket.go              # WsConfig struct with env vars
migrations/
├── 000024_websocket_presence.up.sql    # No tables needed — Redis-only presence
└── 000024_websocket_presence.down.sql   # Empty (Redis cleanup handled in code)
```

**Note**: No SQL tables are needed for WebSocket connections or rooms — they are entirely in-memory (Hub) and Redis (presence/pub-sub). The migration file exists for future extensibility (e.g., storing room configurations, connection logs) but is empty in v1. If we later want persistent room configs or connection audit trails, the migration number is reserved.

## 7. Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `WS_ENABLED` | true | Enable/disable WebSocket system |
| `WS_ALLOWED_ORIGINS` | * | Comma-separated list of allowed origins for CORS (default: allow all) |
| `WS_HEARTBEAT_INTERVAL` | 30s | Ping/pong heartbeat interval |
| `WS_HEARTBEAT_TIMEOUT` | 60s | Timeout before marking user offline |
| `WS_MAX_MESSAGE_SIZE` | 65536 | Max WebSocket message size in bytes (64KB) |
| `WS_MAX_ROOMS_PER_CLIENT` | 50 | Maximum rooms a single connection can join |
| `WS_WRITE_TIMEOUT` | 10s | Write deadline for WebSocket messages |
| `WS_READ_TIMEOUT` | 60s | Read deadline (pong timeout) |
| `WS_TYPING_TTL` | 5s | Typing indicator TTL in seconds |
| `WS_PRESENCE_TTL` | 70s | Presence heartbeat TTL in Redis |

## 8. Permissions

| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `websocket:connect` | websocket | connect | Connect to WebSocket and join rooms |
| `websocket:manage` | websocket | manage | Manage rooms, presence queries, admin operations |

## 9. REST Endpoints

| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| GET | `/ws` | WebSocket upgrade (JWT token in `?token=` query param) | `websocket:connect` |
| GET | `/api/v1/presence/org/:orgID` | Get online users for organization | `websocket:connect` |
| GET | `/api/v1/ws/rooms` | List available rooms for current user | `websocket:connect` |

**WebSocket Endpoint**: `/ws?token=<jwt_token>`

All WebSocket message handling occurs via the persistent connection after upgrade. REST endpoints are supplementary for status queries.

## 10. Integration with Existing Systems

### EventBus Bridge

```go
// In cmd/api/main.go startup:
wsHub := service.NewWebSocketHub(redisClient, enforcer, logger, wsConfig)
wsHub.Start(ctx)

// Bridge EventBus → WebSocket
// All events automatically bridge to WebSocket rooms
eventBus.Subscribe(func(event domain.WebhookEvent) {
    rooms := wsHub.RoomsForEvent(event)
    for _, room := range rooms {
        wsHub.PublishToRoom(room, event)
    }
})

// Shutdown order update:
// server → eventBus → wsHub → workers → enforcer → db → redis
```

### Presence Service Injection

Services that need to check online status inject `PresenceService`:
```go
// Example: skip WebSocket delivery for offline users
online, _ := presenceSvc.IsOnline(ctx, orgID, userID)
if !online { /* skip real-time notification */ }
```

## 11. Success Criteria

### Measurable Outcomes

- **SC-001**: Client can establish WebSocket connection with JWT authentication and receive `ws.connected` within 500ms
- **SC-002**: Message broadcast to room reaches all connected clients within 200ms (single instance)
- **SC-003**: Presence status updates within 2 heartbeat intervals (60s) of actual connection state change
- **SC-004**: System handles 1,000 concurrent WebSocket connections per instance without degradation
- **SC-005**: Cross-instance message delivery via Redis pub/sub with <50ms additional latency
- **SC-006**: Graceful shutdown completes all connections within 10s timeout
- **SC-007**: 80%+ unit test coverage for Hub, Client, Presence, and RedisSubscriber

## 12. Assumptions

- Clients have stable WebSocket support (modern browsers, mobile SDKs)
- JWT tokens are used for authentication (no separate WebSocket auth mechanism)
- Redis is already deployed and available (used by queue, rate limiting, caching)
- Single-tenant deployment model (org-scoped rooms, not multi-tenant isolation at infra level)
- v1 does NOT include message persistence — offline clients use notification system for catch-up
- v1 does NOT include binary WebSocket frames — JSON text messages only
- Existing EventBus pattern will be extended, not replaced
- `coder/websocket` will be the WebSocket library (actively maintained fork of gorilla/websocket, production-proven at Coder)
- **Integration tests** use `httptest.Server` + coder/websocket client for end-to-end flows

## 13. Out of Scope (v1)

- Binary WebSocket frames
- Message persistence / offline message queue
- File transfer over WebSocket
- Voice/video signaling (WebRTC)
- End-to-end encryption
- Admin dashboard for monitoring connections
- Connection rate limiting per IP (future: tie into existing rate limiter)
- Entity-level presence (only org-level in v1)
- Token rotation mid-connection (reconnect required for auth refresh)

## 14. New Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/coder/websocket` | latest | WebSocket upgrade, read/write pumps, close codes (actively maintained gorilla/websocket fork) |