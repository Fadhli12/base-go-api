package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	wsPresenceKeyPrefix = "ws:presence:"
	wsConnKeyPrefix     = "ws:conn:"
)

// PresenceService tracks WebSocket user presence across organizations using Redis.
type PresenceService interface {
	// MarkOnline increments the connection count for a user in an organization.
	// On first connection, adds the user to the org's presence set and publishes
	// an online event. Errors are logged but not returned (graceful degradation).
	MarkOnline(ctx context.Context, orgID, userID uuid.UUID) error

	// MarkOffline decrements the connection count for a user in an organization.
	// On last connection, removes the user from the org's presence set and publishes
	// an offline event. Errors are logged but not returned (graceful degradation).
	MarkOffline(ctx context.Context, orgID, userID uuid.UUID) error

	// RefreshHeartbeat resets the TTL on a user's connection key.
	RefreshHeartbeat(ctx context.Context, orgID, userID uuid.UUID) error

	// IsOnline checks whether a user is present in the org's presence set.
	// Returns false on Redis errors (graceful degradation).
	IsOnline(ctx context.Context, orgID, userID uuid.UUID) (bool, error)

	// GetOnlineUsers returns all user IDs currently online in an organization.
	// Returns an empty slice on Redis errors (graceful degradation).
	GetOnlineUsers(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)

	// GetOnlineCount returns the number of online users in an organization.
	// Returns 0 on Redis errors (graceful degradation).
	GetOnlineCount(ctx context.Context, orgID uuid.UUID) (int64, error)
}

// RedisPresence implements PresenceService using Redis data structures.
type RedisPresence struct {
	redis *redis.Client
	config config.WsConfig
	logger logger.Logger
}

// NewRedisPresence creates a new RedisPresence instance.
func NewRedisPresence(redisClient *redis.Client, logger logger.Logger, config config.WsConfig) *RedisPresence {
	return &RedisPresence{
		redis:  redisClient,
		config: config,
		logger: logger,
	}
}

func (p *RedisPresence) connKey(orgID, userID uuid.UUID) string {
	return fmt.Sprintf("%s%s:%s", wsConnKeyPrefix, orgID, userID)
}

func (p *RedisPresence) presenceKey(orgID uuid.UUID) string {
	return fmt.Sprintf("%s%s", wsPresenceKeyPrefix, orgID)
}

func (p *RedisPresence) channelKey(orgID uuid.UUID) string {
	return fmt.Sprintf("%s%s", wsPresenceKeyPrefix, orgID)
}

// MarkOnline increments the connection count for a user in an organization.
// On first connection (result == 1), it adds the user to the presence set
// and publishes an online event to the organization's presence channel.
func (p *RedisPresence) MarkOnline(ctx context.Context, orgID, userID uuid.UUID) error {
	connKey := p.connKey(orgID, userID)
	presenceKey := p.presenceKey(orgID)

	result, err := p.redis.Incr(ctx, connKey).Result()
	if err != nil {
		p.logger.Error(ctx, "failed to increment connection count",
			logger.String("org_id", orgID.String()),
			logger.String("user_id", userID.String()),
			logger.Err(err),
		)
		return nil
	}

	if err := p.redis.Expire(ctx, connKey, p.config.PresenceTTL).Err(); err != nil {
		p.logger.Warn(ctx, "failed to set connection TTL",
			logger.String("org_id", orgID.String()),
			logger.String("user_id", userID.String()),
			logger.Err(err),
		)
	}

	if result == 1 {
		if err := p.redis.SAdd(ctx, presenceKey, userID.String()).Err(); err != nil {
			p.logger.Error(ctx, "failed to add user to presence set",
				logger.String("org_id", orgID.String()),
				logger.String("user_id", userID.String()),
				logger.Err(err),
			)
		}

		p.publishPresence(ctx, orgID, userID, domain.WsTypePresenceOnline)
	}

	return nil
}

// MarkOffline decrements the connection count for a user in an organization.
// On last connection (result <= 0), it removes the user from the presence set,
// deletes the connection key, and publishes an offline event.
func (p *RedisPresence) MarkOffline(ctx context.Context, orgID, userID uuid.UUID) error {
	connKey := p.connKey(orgID, userID)
	presenceKey := p.presenceKey(orgID)

	result, err := p.redis.Decr(ctx, connKey).Result()
	if err != nil {
		p.logger.Error(ctx, "failed to decrement connection count",
			logger.String("org_id", orgID.String()),
			logger.String("user_id", userID.String()),
			logger.Err(err),
		)
		return nil
	}

	if result <= 0 {
		p.redis.Del(ctx, connKey)

		if err := p.redis.SRem(ctx, presenceKey, userID.String()).Err(); err != nil {
			p.logger.Error(ctx, "failed to remove user from presence set",
				logger.String("org_id", orgID.String()),
				logger.String("user_id", userID.String()),
				logger.Err(err),
			)
		}

		p.publishPresence(ctx, orgID, userID, domain.WsTypePresenceOffline)
	}

	return nil
}

// RefreshHeartbeat resets the TTL on a user's connection key to PresenceTTL.
func (p *RedisPresence) RefreshHeartbeat(ctx context.Context, orgID, userID uuid.UUID) error {
	connKey := p.connKey(orgID, userID)

	if err := p.redis.Expire(ctx, connKey, p.config.PresenceTTL).Err(); err != nil {
		p.logger.Warn(ctx, "failed to refresh heartbeat TTL",
			logger.String("org_id", orgID.String()),
			logger.String("user_id", userID.String()),
			logger.Err(err),
		)
	}

	return nil
}

// IsOnline checks whether a user is in the organization's presence set.
// Returns false on Redis errors.
func (p *RedisPresence) IsOnline(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	presenceKey := p.presenceKey(orgID)

	result, err := p.redis.SIsMember(ctx, presenceKey, userID.String()).Result()
	if err != nil {
		p.logger.Error(ctx, "failed to check presence",
			logger.String("org_id", orgID.String()),
			logger.String("user_id", userID.String()),
			logger.Err(err),
		)
		return false, nil
	}

	return result, nil
}

// GetOnlineUsers returns all user IDs in the organization's presence set.
// Returns an empty slice on Redis errors.
func (p *RedisPresence) GetOnlineUsers(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	presenceKey := p.presenceKey(orgID)

	members, err := p.redis.SMembers(ctx, presenceKey).Result()
	if err != nil {
		p.logger.Error(ctx, "failed to get online users",
			logger.String("org_id", orgID.String()),
			logger.Err(err),
		)
		return []uuid.UUID{}, nil
	}

	users := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		id, parseErr := uuid.Parse(m)
		if parseErr != nil {
			p.logger.Warn(ctx, "skipping invalid UUID in presence set",
				logger.String("org_id", orgID.String()),
				logger.String("member", m),
				logger.Err(parseErr),
			)
			continue
		}
		users = append(users, id)
	}

	return users, nil
}

// GetOnlineCount returns the number of users in the organization's presence set.
// Returns 0 on Redis errors.
func (p *RedisPresence) GetOnlineCount(ctx context.Context, orgID uuid.UUID) (int64, error) {
	presenceKey := p.presenceKey(orgID)

	count, err := p.redis.SCard(ctx, presenceKey).Result()
	if err != nil {
		p.logger.Error(ctx, "failed to get online count",
			logger.String("org_id", orgID.String()),
			logger.Err(err),
		)
		return 0, nil
	}

	return count, nil
}

func (p *RedisPresence) publishPresence(ctx context.Context, orgID, userID uuid.UUID, eventType domain.WsMessageType) {
	channel := p.channelKey(orgID)
	data := domain.WsPresenceData{
		UserID: userID,
		OrgID:  orgID,
	}
	msg := domain.WsMessage{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		p.logger.Error(ctx, "failed to marshal presence message",
			logger.String("org_id", orgID.String()),
			logger.String("user_id", userID.String()),
			logger.String("event_type", string(eventType)),
			logger.Err(err),
		)
		return
	}

	if err := p.redis.Publish(ctx, channel, payload).Err(); err != nil {
		p.logger.Error(ctx, "failed to publish presence event",
			logger.String("org_id", orgID.String()),
			logger.String("user_id", userID.String()),
			logger.String("event_type", string(eventType)),
			logger.Err(err),
		)
	}
}