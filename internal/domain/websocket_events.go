package domain

import (
	"time"

	"github.com/google/uuid"
)

// HubBroadcaster is the interface that the WebSocket Hub implements.
// It allows the event bridge to publish messages to rooms without
// creating a circular dependency (domain -> service -> domain).
type HubBroadcaster interface {
	PublishToRoom(room string, event WsMessage)
}

// WsEventBridge connects the EventBus to the WebSocket Hub.
type WsEventBridge struct {
	hub HubBroadcaster
}

// NewWsEventBridge creates a new event bridge with the given hub broadcaster.
func NewWsEventBridge(hub HubBroadcaster) *WsEventBridge {
	return &WsEventBridge{
		hub: hub,
	}
}

// SubscribeToEventBus registers the bridge as a handler on the EventBus.
func (b *WsEventBridge) SubscribeToEventBus(eventBus *EventBus) {
	eventBus.Subscribe(func(event WebhookEvent) {
		rooms := b.RoomsForEvent(event)
		wsEvent := WsMessage{
			Type: WsTypeEvent,
			Data: WsEventData{
				Event:   string(event.Type),
				Payload: event.Payload,
			},
			Timestamp: time.Now().UTC(),
		}
		for _, room := range rooms {
			wsEvent.Room = room
			b.hub.PublishToRoom(room, wsEvent)
		}
	})
}

// RoomsForEvent determines which WebSocket rooms an EventBus event
// should be broadcast to. Returns org room + entity room if applicable.
func (b *WsEventBridge) RoomsForEvent(event WebhookEvent) []string {
	var rooms []string

	if event.OrgID != nil && *event.OrgID != uuid.Nil {
		orgRoom := RoomOrgPrefix + RoomSeparator + event.OrgID.String()
		rooms = append(rooms, orgRoom)

		entityRoom := entityRoomForEvent(event)
		if entityRoom != "" {
			rooms = append(rooms, entityRoom)
		}
	}

	return rooms
}

// entityRoomForEvent returns the entity-specific room for an event.
// Format: org:{orgID}:{entityType}:{entityID}
func entityRoomForEvent(event WebhookEvent) string {
	if event.OrgID == nil {
		return ""
	}

	switch event.Type {
	case WebhookEventInvoiceCreated, WebhookEventInvoicePaid:
		if payload, ok := event.Payload.(interface{ GetID() uuid.UUID }); ok {
			return RoomOrgPrefix + RoomSeparator + event.OrgID.String() +
				RoomSeparator + "invoice" + RoomSeparator + payload.GetID().String()
		}
	case WebhookEventNewsPublished, WebhookEventNewsDeleted:
		if payload, ok := event.Payload.(interface{ GetID() uuid.UUID }); ok {
			return RoomOrgPrefix + RoomSeparator + event.OrgID.String() +
				RoomSeparator + "news" + RoomSeparator + payload.GetID().String()
		}
	}

	return ""
}