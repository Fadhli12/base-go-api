package service

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/redis/go-redis/v9"
)

const (
	wsRedisChannelPrefix = "ws:msg:"
	wsReconnectBaseDelay = 1 * time.Second
	wsReconnectMaxDelay  = 30 * time.Second
	wsReconnectJitter    = 500 * time.Millisecond
)

// RedisSubscriber defines the interface for WebSocket cross-instance pub/sub via Redis.
type RedisSubscriber interface {
	Subscribe(ctx context.Context, room string) error
	Unsubscribe(ctx context.Context, room string) error
	Publish(ctx context.Context, channel string, data []byte) error
	Start(ctx context.Context) error
	Stop() error
}

// RedisPubSub implements RedisSubscriber using Redis pub/sub for cross-instance
// WebSocket message delivery. Subscriptions are reference-counted so that
// multiple clients in the same room on the same instance share a single
// Redis subscription.
type RedisPubSub struct {
	redis         *redis.Client
	hub           domain.HubBroadcaster
	logger        logger.Logger
	config        config.WsConfig
	mu            sync.Mutex
	subscriptions map[string]int
	sub           *redis.PubSub
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewRedisPubSub creates a new RedisPubSub instance for WebSocket pub/sub.
// The hub broadcaster can be set later via SetHub to break circular dependencies.
func NewRedisPubSub(redisClient *redis.Client, log logger.Logger, cfg config.WsConfig) *RedisPubSub {
	return &RedisPubSub{
		redis:         redisClient,
		logger:        log,
		config:        cfg,
		subscriptions: make(map[string]int),
	}
}

// SetHub sets the hub broadcaster for forwarding Redis messages to local clients.
func (s *RedisPubSub) SetHub(hub domain.HubBroadcaster) {
	s.mu.Lock()
	s.hub = hub
	s.mu.Unlock()
}

// Subscribe registers interest in a room's Redis channel. If the room is already
// subscribed, the reference count is incremented and no Redis call is made.
func (s *RedisPubSub) Subscribe(ctx context.Context, room string) error {
	channel := wsRedisChannelPrefix + room

	s.mu.Lock()
	s.subscriptions[room]++
	count := s.subscriptions[room]
	s.mu.Unlock()

	if count > 1 || s.sub == nil {
		return nil
	}

	if err := s.sub.Subscribe(ctx, channel); err != nil {
		s.logger.Error(ctx, "failed to subscribe to redis channel",
			logger.String("channel", channel),
			logger.Err(err),
		)
	}

	return nil
}

// Unsubscribe decrements the reference count for a room. The Redis subscription
// is only removed when the count reaches zero.
func (s *RedisPubSub) Unsubscribe(ctx context.Context, room string) error {
	channel := wsRedisChannelPrefix + room

	s.mu.Lock()
	count, exists := s.subscriptions[room]
	if !exists {
		s.mu.Unlock()
		return nil
	}

	count--
	if count <= 0 {
		delete(s.subscriptions, room)
	} else {
		s.subscriptions[room] = count
	}
	s.mu.Unlock()

	if count > 0 || s.sub == nil {
		return nil
	}

	if err := s.sub.Unsubscribe(ctx, channel); err != nil {
		s.logger.Error(ctx, "failed to unsubscribe from redis channel",
			logger.String("channel", channel),
			logger.Err(err),
		)
	}

	return nil
}

// Publish sends a message to a Redis channel for cross-instance delivery.
func (s *RedisPubSub) Publish(ctx context.Context, channel string, data []byte) error {
	return s.redis.Publish(ctx, channel, data).Err()
}

// Start launches the background goroutine that listens for Redis pub/sub
// messages and forwards them to the Hub. It reconnects with exponential
// backoff on Redis failures.
func (s *RedisPubSub) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.ctx = ctx
	s.cancel = cancel

	s.wg.Add(1)
	go s.receiveLoop(ctx)

	return nil
}

// Stop gracefully shuts down the subscriber, cancels the context, and waits
// for the receive goroutine to finish.
func (s *RedisPubSub) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

// syncSubscriptions applies the current subscription map to the redis.PubSub
// object by subscribing to any channels not yet tracked.
func (s *RedisPubSub) syncSubscriptions(ctx context.Context) {
	s.mu.Lock()
	channels := make([]string, 0, len(s.subscriptions))
	for room := range s.subscriptions {
		channels = append(channels, wsRedisChannelPrefix+room)
	}
	s.mu.Unlock()

	if len(channels) == 0 {
		return
	}

	if err := s.sub.Subscribe(ctx, channels...); err != nil {
		s.logger.Error(ctx, "failed to sync redis subscriptions",
			logger.Err(err),
		)
	}
}

// receiveLoop manages the Redis subscription lifecycle with reconnection.
func (s *RedisPubSub) receiveLoop(ctx context.Context) {
	defer s.wg.Done()

	delay := wsReconnectBaseDelay

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.sub = s.redis.Subscribe(ctx)
		s.syncSubscriptions(ctx)

		msgCh := s.sub.Channel()
		delay = wsReconnectBaseDelay

		for {
			select {
			case <-ctx.Done():
				s.sub.Close()
				return
			case msg, ok := <-msgCh:
				if !ok {
					s.logger.Warn(ctx, "redis subscription channel closed, reconnecting")
					s.sub.Close()
					delay = nextBackoff(delay)
					s.backoffSleep(ctx, delay)
					goto reconnect
				}
				var wsMsg domain.WsMessage
				if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
					s.logger.Error(ctx, "failed to unmarshal websocket message from redis",
						logger.String("channel", msg.Channel),
						logger.Err(err),
					)
					continue
				}
				room := strings.TrimPrefix(msg.Channel, wsRedisChannelPrefix)
				s.hub.PublishToRoom(room, wsMsg)
			}
		}

	reconnect:
		s.logger.Warn(ctx, "redis pub/sub reconnecting",
			logger.Int("delay_ms", int(delay.Milliseconds())),
		)
	}
}

// backoffSleep pauses for the given duration or until context is cancelled.
func (s *RedisPubSub) backoffSleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// nextBackoff returns the next backoff duration with exponential growth
// capped at maxDelay, plus jitter.
func nextBackoff(d time.Duration) time.Duration {
	next := time.Duration(math.Min(float64(d*2), float64(wsReconnectMaxDelay)))
	jitter := time.Duration(rand.Int63n(int64(2*wsReconnectJitter))) - wsReconnectJitter
	result := next + jitter
	if result < 0 {
		result = next
	}
	return result
}