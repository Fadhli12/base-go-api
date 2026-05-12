# Tasks: Real-time Communication (WebSocket)

**Input**: Design documents from `/specs/017-real-time-communication/`
**Prerequisites**: plan.md ✅, spec.md ✅, data-model.md ✅, contracts/ ✅, research.md ✅

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup (Blocking Prerequisites)

**Purpose**: New dependency, config, domain types, and foundational interfaces. No user story work can begin until this phase is complete.

- [ ] T001 Add `coder/websocket` dependency to `go.mod` (`go get github.com/coder/websocket`) and run `go mod tidy`

- [ ] T002 [P] Create `internal/config/websocket.go`:
  - `WsConfig` struct with env tags (`mapstructure`) matching spec: Enabled, AllowedOrigins, HeartbeatInterval, HeartbeatTimeout, MaxMessageSize, MaxRoomsPerClient, WriteTimeout, ReadTimeout, TypingTTL, PresenceTTL, SendChannelBuffer
  - `DefaultWsConfig()` function with defaults from spec
  - `AllowedOrigins` field: comma-separated list of allowed origins (default `*` for dev, configurable for production)
  - NOTE: Follows patterns in `internal/config/webhook.go` (config struct + defaults function). Service-level defaults are in this file, NOT in a separate `service/websocket_config.go`.

- [ ] T003 [P] Create `internal/domain/websocket.go`:
  - `WsMessageType` type + constants (`ws.connected`, `ws.room.join`, `ws.room.joined`, `ws.room.leave`, `ws.room.left`, `ws.event`, `ws.typing`, `ws.heartbeat`, `ws.presence.online`, `ws.presence.offline`, `ws.error`, `ws.server.shutdown`)
  - `WsMessage` struct (Type, Data, Timestamp, Room)
  - `WsCloseCode` constants: 4001 (AuthExpired), 4002 (ServerShutdown), 4003 (PolicyViolation), 4004 (RoomLimitExceeded)
  - `IsValidRoomName(room string) bool` — validates `org:{uuid}` and `org:{uuid}:{type}:{uuid}` formats
  - `ValidEntityTypes` map — `invoice`, `news`, `media`, `comment`
  - Incoming DTOs: `WsRoomJoin`, `WsRoomLeave`, `WsTyping`
  - Outgoing DTOs: `WsConnectedData`, `WsRoomData`, `WsEventData`, `WsTypingData`, `WsPresenceData`, `WsErrorData`
  - Room naming constants: `RoomOrgPrefix = "org"`

- [ ] T004 [P] Create `internal/domain/websocket_events.go`:
  - `WsEventBridge` struct (EventBus → WebSocket bridge)
  - `WsEventBridge.RoomsForEvent(event WebhookEvent) []string` — returns org room + entity room if applicable
  - `NewWsEventBridge(hub HubBroadcaster) *WsEventBridge` — HubBroadcaster is a minimal interface (see note below)
  - `WsEventBridge.SubscribeToEventBus(eventBus *domain.EventBus)` — registers bridge as EventBus handler
  - NOTE: To avoid circular dependency (Hub ↔ EventBus), define a `HubBroadcaster` interface in domain/websocket_events.go with just `PublishToRoom(room string, event WsMessage)`. Hub implements this interface. EventBridge depends only on the interface, not the concrete Hub.

- [ ] T005 [P] Create `internal/service/websocket_presence.go`:
  - `PresenceService` interface (MarkOnline, MarkOffline, RefreshHeartbeat, IsOnline, GetOnlineUsers, GetOnlineCount)
  - `RedisPresence` struct implementing `PresenceService`
  - Redis key patterns: `ws:presence:{orgID}` (SET), `ws:conn:{orgID}:{userID}` (STRING with TTL)
  - Reference counting: INCR/DECR connection count, SADD/SREM for online set
  - `NewRedisPresence(redisClient, logger, config)` constructor

- [ ] T006 [P] Create `internal/service/websocket_redis.go`:
  - `RedisSubscriber` interface (Subscribe, Unsubscribe, Publish, Start, Stop)
  - `RedisPubSub` struct implementing `RedisSubscriber`
  - Channel patterns: `ws:msg:{roomID}`, `ws:presence:{orgID}`
  - Reference counting for subscriptions (subscribe count, unsub when 0)
  - Background goroutine that forwards Redis messages to `Hub.BroadcastToRoom()`
  - Auto-reconnect with exponential backoff (1s → 30s, ±500ms jitter)
  - Graceful degradation: log error when Redis unavailable, continue with local-only delivery

- [ ] T007 [P] Create empty migration `migrations/000024_websocket.up.sql` and `000024_websocket.down.sql`:
  - Both files contain only comment: "No tables needed for v1 WebSocket implementation"

---

## Phase 2: Core Infrastructure (US1 — Connect and Receive Events)

**Purpose**: Client, Hub, and Handler — the heart of WebSocket functionality. US2 and US3 depend on this phase.

- [ ] T008 Create `internal/service/websocket_client.go`:
  - `Client` struct: ID, UserID, OrgID, Conn (*websocket.Conn), Hub, Send (chan []byte), Rooms (map[string]bool), logger, writeDeadline
  - `NewClient(conn, userID, orgID, hub, logger, config)` constructor
  - `Run(ctx context.Context)` — starts read pump + write pump goroutines, blocks until both complete
  - `ReadPump(ctx)` — loop reading JSON messages, routes to handlers, calls `Hub.Unregister()` on error
  - `WritePump(ctx)` — loop reading from Send channel, writes to conn, handles ping/heartbeat, write deadline
  - `Close(closeCode int, reason string)` — sends close frame with custom close code
  - `handleMessage(data []byte)` — unmarshals WsMessage, routes by type
  - `handleJoin(data)` → validates room format → calls `Hub.JoinRoom()`
  - `handleLeave(data)` → calls `Hub.LeaveRoom()`
  - `handleTyping(data)` → calls `Hub.PublishToRoom()` for ephemeral broadcast
  - `handleHeartbeat()` → calls `PresenceService.RefreshHeartbeat()`
  - `sendMessage(msgType, data, room)` — marshals WsMessage, writes to Send channel (non-blocking)
  - Close code handling: 4001 (auth expired), 4002 (server shutdown), 4003 (policy violation), 4004 (room limit)

- [ ] T009 Create `internal/service/websocket_hub.go`:
  - `Hub` struct: mu (sync.RWMutex), clients (map[uuid.UUID]*Client), rooms (map[string]map[*Client]bool), register/unregister/broadcast channels, presence (PresenceService), redis (RedisSubscriber), logger, config, ctx/cancel, wg
  - `NewHub(presence, redis, logger, config)` constructor
  - `Start(ctx) error` — starts Run() goroutine, starts PresenceService, starts RedisSubscriber
  - `Stop() error` — sends ws.server.shutdown to all clients, closes connections, stops goroutines with 10s timeout
  - `Run(ctx)` — event loop processing register/unregister/broadcast channels, blocks until ctx cancelled
  - `Register(client)` — add to clients map, send ws.connected, call PresenceService.MarkOnline()
  - `Unregister(client)` — remove from clients + all rooms, call PresenceService.MarkOffline(), close Send channel
  - `JoinRoom(client, room) error` — validate room format, check room limit (max 50 per client), add to Hub.rooms, subscribe RedisSubscriber, send ws.room.joined + broadcast ws.presence.online
  - `LeaveRoom(client, room)` — remove from Hub.rooms, unsubscribe RedisSubscriber if last client, send ws.room.left
  - `PublishToRoom(room, event)` — marshal WsMessage, write to Hub.broadcast channel + RedisSubscriber.Publish()
  - `BroadcastToRoom(room, data)` — used by RedisSubscriber for cross-instance delivery: send to all local clients
  - `GetRoomClients(room) int` — return count for monitoring
  - `GetTotalConnections() int` — return count for monitoring

- [ ] T010 [P] Create `internal/http/middleware/websocket_auth.go`:
  - `WebSocketAuthMiddleware` — Echo middleware that extracts JWT from query param (`?token=`)
  - Uses existing `authService.ParseAndValidateToken(tokenString)` to validate the token
  - Sets user claims (userID, orgID) in Echo context for downstream handler
  - Returns 401 if token missing/invalid/expired
  - Returns 403 if Casbin check `enforcer.Enforce(userID, orgDomain, "websocket", "connect")` fails
  - NOTE: This reuses the existing JWT validation logic from `internal/service/auth.go` — NOT creating a separate JWT parser. The middleware extracts the token from the query param and delegates to the auth service for validation.

- [ ] T011 Create `internal/http/handler/websocket.go`:
  - `WebSocketHandler` struct with hub, authService, enforcer, auditSvc, logger, config
  - `NewWebSocketHandler(hub, authService, enforcer, auditSvc, logger, config)` constructor
  - `HandleUpgrade(c echo.Context) error`:
    1. Extract JWT from `c.QueryParam("token")` (validated by middleware)
    2. Get userID and orgID from context
    3. Casbin check: `enforcer.Enforce(userID, orgDomain, "websocket", "connect")`
    4. `websocket.Accept(w, r, &AcceptOptions{OriginPatterns: config.AllowedOrigins})`
    5. Create Client, Hub.Register(client), client.Run(ctx)
    6. Returns nil (goroutine blocks until disconnect)
  - REST endpoints:
    - `GET /api/v1/presence/org/:orgID` → `PresenceService.GetOnlineUsers()` → returns `PresenceResponse`
      - Audit log: `LogAction(ctx, userID, orgDomain, "websocket", "view_presence", "presence", orgIDStr, nil, nil)` (9-param signature matching existing pattern)
    - `GET /api/v1/ws/rooms` → returns list of rooms the user can access (org room + entity rooms from JWT claims)
      - Audit log: `LogAction(ctx, userID, orgDomain, "websocket", "view_rooms", "websocket", "", nil, nil)` (9-param signature)
  - `RegisterRoutes(api, jwtSecret)` — registers `/ws` endpoint and REST routes with JWT middleware

---

## Phase 3: Integration & Wiring (US1)

**Purpose**: Connect all pieces into the existing application bootstrap.

- [ ] T012 Update `internal/http/server.go`:
  - Initialize `wsPresence := service.NewRedisPresence(redis, logger, wsConfig)`
  - Initialize `wsRedis := service.NewRedisPubSub(redis, logger)`
  - Initialize `wsHub := service.NewWebSocketHub(wsPresence, wsRedis, logger, wsConfig)`
  - Initialize `wsHandler := handler.NewWebSocketHandler(wsHub, authService, enforcer, s.auditSvc, logger, wsConfig)`
  - Call `wsHandler.RegisterRoutes(v1, jwtSecret)` — register before other routes to avoid conflicts
  - Store wsHub on Server struct for shutdown access

- [ ] T013 Update `cmd/api/main.go`:
  - After EventBus.Start(): `wsHub.Start(ctx)`
  - Bridge EventBus → WebSocket:
    ```go
    eventBridge := domain.NewWsEventBridge(wsHub)
    eventBridge.SubscribeToEventBus(eventBus, wsHub)
    ```
  - Shutdown order update: `server → eventBus → wsHub → workers → enforcer → db → redis`
  - Add to `WsConfig` default wiring in config loading
  - Permission seeding: add `websocket:connect` and `websocket:manage` to `seedPermissions()`

---

## Phase 4: Presence & Typing (US2 — Presence and Typing Indicators)

**Purpose**: Implement presence tracking and typing indicators. Depends on T005 (PresenceService) and T008-T009 (Client/Hub).

- [ ] T014 [P] Implement presence heartbeat in `websocket_client.go`:
  - ReadPump: on `ws.heartbeat` message, call `PresenceService.RefreshHeartbeat()`
  - WritePump: periodic ping (every 30s, configurable)
  - Client disconnect: trigger `PresenceService.MarkOffline()` via Hub.Unregister()
  - Presence update: broadcast `ws.presence.online`/`ws.presence.offline` to org room on connect/disconnect

- [ ] T015 [P] Implement typing indicator in `websocket_client.go`:
  - handleTyping: validate `entity_type` and `entity_id`, publish `WsTyping` to room
  - Hub.PublishToRoom broadcasts `ws.typing` to all room members except sender
  - Redis: SET `ws:typing:{orgID}:{entityType}:{entityID}:{userID}` with 5s TTL
  - No persistence, no audit, fire-and-forget

- [ ] T016 [P] Implement `GET /api/v1/presence/org/:orgID` endpoint in `websocket.go` handler:
  - Extract orgID from path param
  - Casbin check: `websocket:connect`
  - Call `PresenceService.GetOnlineUsers()`
  - Return JSON: `{org_id, online_users: [uuid], count: int}`

- [ ] T017 [P] Implement `GET /api/v1/ws/rooms` endpoint in `websocket.go` handler:
  - Extract user claims from context
  - Return available rooms based on user's org memberships and accessible entities
  - Response: `{rooms: [{room, name}]}`

---

## Phase 5: Multi-Instance Scaling (US3 — Cross-Instance Delivery)

**Purpose**: Redis pub/sub for cross-instance message delivery. Depends on T006 (RedisSubscriber) and T009 (Hub).

- [ ] T018 Integrate RedisSubscriber with Hub in `websocket_hub.go`:
  - `JoinRoom`: if first client in room → `redis.Subscribe(ctx, room)`
  - `LeaveRoom`: if last client in room → `redis.Unsubscribe(ctx, room)`
  - `PublishToRoom`: always → `redis.Publish(ctx, "ws:msg:"+room, data)`
  - `BroadcastToRoom`: called by RedisSubscriber when cross-instance message received → send to local clients

- [ ] T019 [P] Implement RedisSubscriber message forwarding in `websocket_redis.go`:
  - Background goroutine: subscribe to `ws:msg:*` and `ws:presence:*` channels
  - On message: unmarshal JSON → call `Hub.BroadcastToRoom()` for `ws:msg:*` or `Hub.BroadcastPresence()` for `ws:presence:*`
  - On subscribe error: exponential backoff reconnection
  - On Redis unavailable: log error, continue (graceful degradation)

---

## Phase 6: Testing

**Purpose**: Unit tests with miniredis, integration tests with httptest + coder/websocket client.

- [ ] T020 [P] Create `tests/unit/websocket_hub_test.go`:
  - Hub registration: client connects, hub has client
  - Hub unregistration: client disconnects, hub has no client
  - Room join: client joins org room, room has client
  - Room leave: client leaves room, room has no client
  - Broadcast to room: multiple clients, message reaches all in room
  - Room validation: invalid room names rejected, valid accepted
  - Room limit: >50 rooms rejected

- [ ] T021 [P] Create `tests/unit/websocket_presence_test.go`:
  - MarkOnline: Redis SET contains user, connection count = 1
  - MarkOnline (second tab): Redis SET still has user, connection count = 2
  - MarkOffline: Redis SET still has user (count = 1), count = 0 → user removed
  - RefreshHeartbeat: TTL refreshed
  - IsOnline: returns true when online, false when offline
  - GetOnlineUsers: returns correct list
  - Stale cleanup: TTL expiry removes entry

- [ ] T022 [P] Create `tests/unit/websocket_redis_test.go`:
  - Subscribe: Redis pub/sub subscription active
  - Unsubscribe: subscription removed
  - Publish: message reaches subscriber
  - Cross-instance broadcast: message published on one Hub, received on another Hub sharing miniredis
  - Graceful degradation: Redis unavailable → error logged, local-only delivery

- [ ] T023 [P] Create `tests/integration/websocket_test.go`:
  - Connect: valid token → connection established, `ws.connected` received
  - Reject: missing token → HTTP 401
  - Reject: invalid token → HTTP 401
  - Reject: expired token → HTTP 401
  - Join room: `ws.room.join` → `ws.room.joined`
  - Leave room: `ws.room.leave` → `ws.room.left`
  - Broadcast: service publishes event → client receives `ws.event`
  - Typing: client sends typing → other client receives typing
  - Presence: connect → query `/api/v1/presence/org/:id` → user in list
  - Multi-tab: two connections same user → presence count = 1, disconnect one → still present
  - Graceful shutdown: server shutdown → `ws.server.shutdown` message + close code 4002
  - Uses `httptest.Server` + `coder/websocket` client

---

## Dependency Graph

```
Phase 1 (Setup)
├── T001 (go mod) ─────────────────┐
├── T002 (config) ─────────────────┤
├── T003 (domain types) ───────────┤
├── T004 (event bridge) ───────────┤
├── T005 (presence) ───────────────┤
├── T006 (redis subscriber) ─────────┤
└── T007 (migrations) ───────────────┘
                │
                ▼
Phase 2 (Core Infrastructure)
├── T008 (client) ─────────────────┐
├── T009 (hub) ────────────────────┤
├── T010 (middleware) ─────────────┤
└── T011 (handler) ────────────────┘
                │
                ▼
Phase 3 (Integration & Wiring)
├── T012 (server.go DI) ───────────┐
└── T013 (cmd/api/main.go) ────────┘
                │
                ├───→ Phase 4 (US2: Presence & Typing)
                │     ├── T014 (heartbeat)
                │     ├── T015 (typing)
                │     ├── T016 (GET presence endpoint)
                │     └── T017 (GET rooms endpoint)
                │
                └───→ Phase 5 (US3: Multi-Instance)
                      ├── T018 (hub-redis integration)
                      └── T019 (subscriber forwarding)
                              │
                              ▼
                        Phase 6 (Testing)
                        ├── T020 (hub unit tests)
                        ├── T021 (presence unit tests)
                        ├── T022 (redis unit tests)
                        └── T023 (integration tests)
```

## Parallel Execution Opportunities

| Tasks | Parallel? | Notes |
|-------|-----------|-------|
| T002, T003, T004, T005, T006, T007 | ✅ Yes | All independent setup tasks |
| T008, T009, T010, T011 | ✅ Partial | Client + Middleware can be done in parallel. Hub depends on Client. Handler depends on Hub + Middleware. |
| T012, T013 | ⚠️ Sequential | Server wiring → main.go startup |
| T014, T015, T016, T017 | ✅ Yes | All US2 tasks are independent once Phase 2 is complete |
| T018, T019 | ⚠️ Sequential | T019 depends on T018's integration |
| T020, T021, T022, T023 | ✅ Partial | Unit tests (T020, T021, T022) can run in parallel. Integration tests (T023) depend on all unit tests passing. |