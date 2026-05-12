package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	appErrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

type broadcastMessage struct {
	room string
	data []byte
	msg  domain.WsMessage
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[uuid.UUID]*Client
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast   chan *broadcastMessage
	presence    PresenceService
	redis       RedisSubscriber
	logger      logger.Logger
	config      config.WsConfig
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewHub(presence PresenceService, redis RedisSubscriber, log logger.Logger, cfg config.WsConfig) *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*Client),
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan *broadcastMessage, 256),
		presence:   presence,
		redis:      redis,
		logger:     log,
		config:     cfg,
	}
}

func (h *Hub) Start(ctx context.Context) error {
	h.ctx, h.cancel = context.WithCancel(ctx)

	if err := h.redis.Start(h.ctx); err != nil {
		h.cancel()
		return fmt.Errorf("redis subscriber start: %w", err)
	}

	h.wg.Add(1)
	go h.Run(h.ctx)

	return nil
}

func (h *Hub) Stop() error {
	msg := domain.WsMessage{
		Type:      domain.WsTypeServerShutdown,
		Data: domain.WsErrorData{
			Code:    "SERVER_SHUTDOWN",
			Message: "server is shutting down",
		},
		Timestamp: time.Now().UTC(),
	}

	h.mu.RLock()
	for _, c := range h.clients {
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		select {
		case c.Send <- data:
		default:
			h.logger.Warn(context.Background(), "shutdown message dropped, client send channel full",
				logger.String("client_id", c.ID.String()),
			)
		}
	}
	h.mu.RUnlock()

	h.cancel()

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		h.logger.Warn(context.Background(), "hub stop timed out waiting for run loop")
	}

	if err := h.redis.Stop(); err != nil {
		h.logger.Error(context.Background(), "redis subscriber stop error", logger.Err(err))
	}

	return nil
}

func (h *Hub) Run(ctx context.Context) {
	defer h.wg.Done()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

			client.sendMessage(domain.WsTypeConnected, domain.WsConnectedData{
				UserID: client.UserID,
				Rooms:  client.RoomsList(),
			}, "")

			if err := h.presence.MarkOnline(ctx, client.OrgID, client.UserID); err != nil {
				h.logger.Error(ctx, "failed to mark client online",
					logger.Err(err),
					logger.String("user_id", client.UserID.String()),
				)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			var roomsToUnsubscribe []string
			if existing, ok := h.clients[client.ID]; ok {
				for _, room := range existing.RoomsList() {
					if clients, ok := h.rooms[room]; ok {
						delete(clients, existing)
						if len(clients) == 0 {
							delete(h.rooms, room)
							roomsToUnsubscribe = append(roomsToUnsubscribe, room)
						}
					}
				}
				delete(h.clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()

			for _, room := range roomsToUnsubscribe {
				if err := h.redis.Unsubscribe(ctx, room); err != nil {
					h.logger.Error(ctx, "failed to unsubscribe from room",
						logger.Err(err),
						logger.String("room", room),
					)
				}
			}

			if err := h.presence.MarkOffline(ctx, client.OrgID, client.UserID); err != nil {
				h.logger.Error(ctx, "failed to mark client offline",
					logger.Err(err),
					logger.String("user_id", client.UserID.String()),
				)
			}

		case bm := <-h.broadcast:
			h.BroadcastToRoom(bm.room, bm.data)

		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) Register(client *Client) {
	select {
	case h.register <- client:
	default:
		h.logger.Warn(h.ctx, "hub register channel full, dropping client",
			logger.String("client_id", client.ID.String()),
		)
	}
}

func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	default:
		h.logger.Warn(h.ctx, "hub unregister channel full, dropping client",
			logger.String("client_id", client.ID.String()),
		)
	}
}

func (h *Hub) JoinRoom(client *Client, room string) error {
	if !domain.IsValidRoomName(room) {
		return appErrors.NewAppError("BAD_REQUEST", fmt.Sprintf("invalid room name: %s", room), 400)
	}

	h.mu.Lock()
	if client.RoomCount() >= h.config.MaxRoomsPerClient {
		h.mu.Unlock()
		return appErrors.NewAppError("ROOM_LIMIT", fmt.Sprintf("maximum rooms per client (%d) exceeded", h.config.MaxRoomsPerClient), 400)
	}

	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true
	client.AddRoom(room)
	h.mu.Unlock()

	if err := h.redis.Subscribe(h.ctx, room); err != nil {
		h.logger.Error(h.ctx, "failed to subscribe to redis room",
			logger.Err(err),
			logger.String("room", room),
		)
	}

	client.sendMessage(domain.WsTypeRoomJoined, domain.WsRoomData{Room: room}, room)

	return nil
}

func (h *Hub) LeaveRoom(client *Client, room string) {
	var shouldUnsubscribe bool

	h.mu.Lock()
	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, room)
			shouldUnsubscribe = true
		}
	}
	client.RemoveRoom(room)
	h.mu.Unlock()

	if shouldUnsubscribe {
		if err := h.redis.Unsubscribe(h.ctx, room); err != nil {
			h.logger.Error(h.ctx, "failed to unsubscribe from redis room",
				logger.Err(err),
				logger.String("room", room),
			)
		}
	}

	client.sendMessage(domain.WsTypeRoomLeft, domain.WsRoomData{Room: room}, room)
}

func (h *Hub) PublishToRoom(room string, msg domain.WsMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error(h.ctx, "failed to marshal broadcast message",
			logger.Err(err),
			logger.String("room", room),
		)
		return
	}

	select {
	case h.broadcast <- &broadcastMessage{room: room, data: data, msg: msg}:
	default:
		h.logger.Warn(h.ctx, "broadcast channel full, dropping message",
			logger.String("room", room),
		)
	}

	if err := h.redis.Publish(h.ctx, "ws:msg:"+room, data); err != nil {
		h.logger.Error(h.ctx, "failed to publish message to redis",
			logger.Err(err),
			logger.String("room", room),
		)
	}
}

func (h *Hub) BroadcastToRoom(room string, data []byte) {
	h.mu.RLock()
	clients, ok := h.rooms[room]
	if !ok {
		h.mu.RUnlock()
		return
	}

	for client := range clients {
		select {
		case client.Send <- data:
		default:
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) GetRoomClients(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room])
}

func (h *Hub) GetTotalConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}