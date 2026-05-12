package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// WsMessageType represents the type of a WebSocket message.
type WsMessageType string

const (
	// Client → Server
	WsTypeRoomJoin  WsMessageType = "ws.room.join"
	WsTypeRoomLeave WsMessageType = "ws.room.leave"
	WsTypeTyping    WsMessageType = "ws.typing"
	WsTypeHeartbeat WsMessageType = "ws.heartbeat"

	// Server → Client
	WsTypeConnected      WsMessageType = "ws.connected"
	WsTypeRoomJoined     WsMessageType = "ws.room.joined"
	WsTypeRoomLeft       WsMessageType = "ws.room.left"
	WsTypeEvent          WsMessageType = "ws.event"
	WsTypePresenceOnline  WsMessageType = "ws.presence.online"
	WsTypePresenceOffline WsMessageType = "ws.presence.offline"
	WsTypeError          WsMessageType = "ws.error"
	WsTypeServerShutdown WsMessageType = "ws.server.shutdown"
)

// WsCloseCode represents custom WebSocket close codes.
type WsCloseCode int

const (
	WsCloseAuthExpired      WsCloseCode = 4001
	WsCloseServerShutdown   WsCloseCode = 4002
	WsClosePolicyViolation  WsCloseCode = 4003
	WsCloseRoomLimitExceeded WsCloseCode = 4004
)

// WsMessage represents a WebSocket message envelope.
type WsMessage struct {
	Type      WsMessageType `json:"type"`
	Data      any           `json:"data"`
	Timestamp time.Time     `json:"timestamp"`
	Room      string        `json:"room,omitempty"`
}

// Incoming DTOs

// WsRoomJoin is the incoming message for joining a room.
type WsRoomJoin struct {
	Room string `json:"room" validate:"required"`
}

// WsRoomLeave is the incoming message for leaving a room.
type WsRoomLeave struct {
	Room string `json:"room" validate:"required"`
}

// WsTyping is the incoming message for typing indicators.
type WsTyping struct {
	Room       string `json:"room" validate:"required"`
	EntityType string `json:"entity_type,omitempty"`
	EntityID   string `json:"entity_id,omitempty"`
}

// Outgoing DTOs

// WsConnectedData is the data payload for ws.connected messages.
type WsConnectedData struct {
	UserID uuid.UUID `json:"user_id"`
	Rooms  []string  `json:"rooms"`
}

// WsRoomData is the data payload for ws.room.joined and ws.room.left messages.
type WsRoomData struct {
	Room string `json:"room"`
}

// WsEventData is the data payload for ws.event messages.
type WsEventData struct {
	Event   string `json:"event"`
	Payload any    `json:"payload"`
	Room    string `json:"room"`
}

// WsTypingData is the data payload for ws.typing messages (server → client).
type WsTypingData struct {
	UserID     uuid.UUID `json:"user_id"`
	Room       string    `json:"room"`
	EntityType string    `json:"entity_type,omitempty"`
	EntityID   string    `json:"entity_id,omitempty"`
}

// WsPresenceData is the data payload for presence online/offline messages.
type WsPresenceData struct {
	UserID uuid.UUID `json:"user_id"`
	OrgID  uuid.UUID `json:"org_id"`
}

// WsErrorData is the data payload for ws.error messages.
type WsErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Room naming constants and validation.

const (
	// RoomOrgPrefix is the prefix for organization rooms.
	RoomOrgPrefix = "org"
	// RoomSeparator is the separator used in room names.
	RoomSeparator = ":"
)

// ValidEntityTypes defines the entity types that can appear in room paths.
// These must match the existing commentable/taggable types in the system.
var ValidEntityTypes = map[string]bool{
	"invoice": true,
	"news":    true,
	"media":   true,
	"comment": true,
}

// IsValidRoomName validates room naming format.
// Valid formats:
//   - org:{uuid}                       (organization room)
//   - org:{uuid}:{entityType}:{uuid}   (entity room)
func IsValidRoomName(room string) bool {
	parts := strings.Split(room, RoomSeparator)

	// org:{uuid}
	if len(parts) == 2 && parts[0] == RoomOrgPrefix {
		_, err := uuid.Parse(parts[1])
		return err == nil
	}

	// org:{uuid}:{entityType}:{uuid}
	if len(parts) == 4 && parts[0] == RoomOrgPrefix {
		if _, err := uuid.Parse(parts[1]); err != nil {
			return false
		}
		if !ValidEntityTypes[parts[2]] {
			return false
		}
		if _, err := uuid.Parse(parts[3]); err != nil {
			return false
		}
		return true
	}

	return false
}

// ExtractOrgID extracts the organization ID from a room name.
// Returns empty UUID if the room format is invalid.
func ExtractOrgID(room string) uuid.UUID {
	parts := strings.Split(room, RoomSeparator)
	if len(parts) >= 2 && parts[0] == RoomOrgPrefix {
		orgID, err := uuid.Parse(parts[1])
		if err == nil {
			return orgID
		}
	}
	return uuid.Nil
}