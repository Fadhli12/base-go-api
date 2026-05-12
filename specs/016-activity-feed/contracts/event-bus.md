# EventBus Integration Contract: Activity Feed

**Version**: 1.0 | **Date**: 2026-05-12 | **Status**: Phase 1 Design

---

## Overview

The Activity Service subscribes to the existing EventBus (`internal/domain/webhook_events.go`) to automatically create Activity records when domain events are published. This integration follows the established `SetEventBus()` setter injection pattern used by `WebhookService`, `UserService`, `InvoiceService`, and `NewsService`.

**No changes to existing services or the EventBus are required.** The Activity Service is a pure subscriber — it only reads events and creates Activity records.

---

## EventBus Interface

The existing EventBus provides:

```go
type WebhookEvent struct {
    Type    string     // e.g., "user.created", "invoice.paid"
    Payload any        // Event payload data (typically map[string]interface{})
    OrgID   *uuid.UUID // Optional: organization context for org-scoped event matching
}

type EventHandler func(event WebhookEvent)
```

---

## Subscription Pattern

Follow the `WebhookService.SubscribeToEventBus()` pattern exactly:

```go
// SubscribeToEventBus registers the ActivityService as an EventBus subscriber.
// Uses a buffered channel to avoid blocking the EventBus's callHandlers goroutine.
func (s *ActivityService) SubscribeToEventBus(bus *domain.EventBus) {
    s.eventBus = bus
    ch := make(chan domain.WebhookEvent, 256) // Buffered channel — same as webhook
    bus.Subscribe(func(event domain.WebhookEvent) {
        ch <- event // Non-blocking send to channel
    })
    go s.processEvents(ch) // Process events in background goroutine
}

// processEvents reads from the buffered channel and handles each event.
func (s *ActivityService) processEvents(ch chan domain.WebhookEvent) {
    for event := range ch {
        if err := s.handleEvent(event); err != nil {
            s.logger.Error(context.Background(), "failed to handle activity event",
                log.String("event_type", event.Type),
                log.String("event_id", event.ID),
                log.Err(err),
            )
        }
    }
}
```

**Key design decisions**:
1. **Buffered channel (256)** — Prevents blocking the EventBus when activity creation is slow. Matches `WebhookService` buffer size.
2. **Background goroutine** — `processEvents` runs in a separate goroutine, decoupling event processing from publishing.
3. **Error handling** — Failed events are logged but DO NOT block subsequent events. No retry at the activity level (individual activities are low-stakes; missing one is acceptable).

---

## Event Type Mapping

The ActivityService maps EventBus event types to Activity ActionType and ResourceType using an explicit mapping registry:

| EventBus Type | Activity ActionType | ResourceType | Description Template |
|--------------|-------------------|-------------|---------------------|
| `user.created` | `created` | `user` | `{actor_name} created a user account` |
| `user.deleted` | `deleted` | `user` | `{actor_name} deleted user {resource_id}` |
| `invoice.created` | `created` | `invoice` | `{actor_name} created invoice {metadata.invoice_number}` |
| `invoice.paid` | `paid` | `invoice` | `{actor_name} marked invoice {metadata.invoice_number} as paid` |
| `news.published` | `published` | `news` | `{actor_name} published news article {metadata.title}` |
| `news.deleted` | `deleted` | `news` | `{actor_name} deleted news article {resource_id}` |
| `comment.created` | `created` | `comment` | `{actor_name} commented on {metadata.commentable_type}` |

**Mapping implementation**:

```go
// eventMapping defines how EventBus events map to Activity fields.
var eventMapping = map[string]ActivityMapping{
    "user.created":     {ActionType: "created", ResourceType: "user"},
    "user.deleted":     {ActionType: "deleted", ResourceType: "user"},
    "invoice.created":  {ActionType: "created", ResourceType: "invoice"},
    "invoice.paid":     {ActionType: "paid",   ResourceType: "invoice"},
    "news.published":   {ActionType: "published", ResourceType: "news"},
    "news.deleted":     {ActionType: "deleted", ResourceType: "news"},
    "comment.created":  {ActionType: "created", ResourceType: "comment"},
}

type ActivityMapping struct {
    ActionType   string
    ResourceType string
}
```

**Fallback for unmapped events**: If an EventBus event type is not in the mapping, derive ActionType from the event type suffix (after the dot) and ResourceType from the prefix:

```go
// Fallback derivation
// e.g., "order.shipped" → ActionType: "shipped", ResourceType: "order"
parts := strings.SplitN(event.Type, ".", 2)
if len(parts) == 2 {
    actionType = parts[1]   // "shipped"
    resourceType = parts[0] // "order"
}
```

---

## Event Payload Extraction

Each EventBus event's `Payload` map provides context for the Activity's `metadata` field:

```go
func (s *ActivityService) handleEvent(event domain.WebhookEvent) error {
    mapping, ok := eventMapping[event.Type]
    if !ok {
        // Fallback derivation
        parts := strings.SplitN(event.Type, ".", 2)
        if len(parts) != 2 {
            return fmt.Errorf("cannot parse event type: %s", event.Type)
        }
        mapping = ActivityMapping{
            ActionType:   parts[1],
            ResourceType: parts[0],
        }
    }

    // Build metadata from event payload
    metadata := buildMetadata(event, mapping)

    // Create Activity
    activity := &domain.Activity{
        ActorID:       extractActorID(event),
        ActionType:     mapping.ActionType,
        ResourceType:   mapping.ResourceType,
        ResourceID:    extractResourceID(event),
        OrganizationID: extractOrgID(event),
        Metadata:      metadata,
    }

    return s.activityRepo.Create(context.Background(), activity)
}
```

### Metadata Construction

The `metadata` JSONB field MUST contain a `description` key with a human-readable summary. Additional context fields are extracted from the event payload:

```go
func buildMetadata(event domain.WebhookEvent, mapping ActivityMapping) map[string]interface{} {
    metadata := make(map[string]interface{})

    // Required: human-readable description
    description := generateDescription(event, mapping)
    metadata["description"] = description

    // Copy useful payload fields (not exhaustive — just common ones)
    if invoiceNum, ok := event.Payload["invoice_number"].(string); ok {
        metadata["invoice_number"] = invoiceNum
    }
    if title, ok := event.Payload["title"].(string); ok {
        metadata["title"] = title
    }
    if commentableType, ok := event.Payload["commentable_type"].(string); ok {
        metadata["commentable_type"] = commentableType
    }
    if oldStatus, ok := event.Payload["old_status"].(string); ok {
        metadata["old_status"] = oldStatus
    }
    if newStatus, ok := event.Payload["new_status"].(string); ok {
        metadata["new_status"] = newStatus
    }

    return metadata
}
```

### Actor ID Extraction

The `actor_id` for the Activity is derived from the event payload's `user_id` field:

```go
func extractActorID(event domain.WebhookEvent) uuid.UUID {
    if userIDStr, ok := event.Payload["user_id"].(string); ok {
        if id, err := uuid.Parse(userIDStr); err == nil {
            return id
        }
    }
    return uuid.Nil // System-generated event with no user actor
}
```

### Organization ID Extraction

```go
func extractOrgID(event domain.WebhookEvent) *uuid.UUID {
    if event.OrgID != nil {
        return event.OrgID
    }
    return nil // Global activity (no org scope)
}
```

---

## Startup Wiring

In `cmd/api/main.go`, after service construction:

```go
// Create ActivityService
activityRepo := repository.NewActivityRepository(db)
activityReadRepo := repository.NewActivityReadRepository(db)
activityFollowRepo := repository.NewActivityFollowRepository(db)
activityService := service.NewActivityService(
    activityRepo, activityReadRepo, activityFollowRepo,
    enforcer, auditService, logger,
)

// Subscribe to EventBus (after eventBus is created)
activityService.SubscribeToEventBus(eventBus)

// Start reaper
activityReaper := service.NewActivityReaper(activityRepo, config.ActivityConfig, logger)
activityReaper.Start(ctx)
```

**Shutdown order** (added to existing graceful shutdown):
```go
// Shutdown order: server → eventBus → activityReaper → workers → enforcer → db → redis
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer shutdownCancel()

if err := server.Shutdown(shutdownCtx); err != nil { ... }
eventBus.Stop()
activityReaper.Stop()  // New: stop reaper
// ... rest of shutdown
```

---

## Event Flow Diagram

```
Existing Service (e.g., UserService)
    │
    ├── After successful Create():
    │   eventBus.Publish(domain.WebhookEvent{
    │       Type: "user.created",
    │       Payload: map[string]interface{}{
    │           "user_id": user.ID.String(),
    │           "email": user.Email,
    │       },
    │       OrgID: orgID,
    │   })
    │
    ▼
EventBus (callHandlers)
    │
    ├── WebhookService.handler(event)  ──→ Dispatch webhook deliveries
    │
    ├── ActivityService.handler(event) ──→ ch <- event (buffered channel, 256)
    │                                           │
    │                                           ▼
    │                                      processEvents(ch)
    │                                           │
    │                                           ├── Map event type → ActionType/ResourceType
    │                                           ├── Build metadata (description + context)
    │                                           ├── Create Activity record in DB
    │                                           └── Log errors, continue on failure
    │
    └── ... other subscribers
```

**Important**: The EventBus calls handlers sequentially in `callHandlers()`. The ActivityService's handler function (`func(event) { ch <- event }`) is extremely fast (just a channel send), so it does NOT block other subscribers even if activity creation is slow.

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Unknown event type | Fallback derivation from dot-separated type (prefix=resource, suffix=action) |
| Missing `user_id` in payload | ActorID set to `uuid.Nil` — activity still recorded |
| Database error on Create | Log error, continue — missing activities are acceptable |
| Channel full (256 events buffered) | Publisher blocks briefly; unlikely in normal operation |
| ActivityService not subscribed | No-op — events pass through without activity creation |

---

## Testing Contract

### Unit Tests
- Mock EventBus, verify handler registration
- Test event mapping for all known event types
- Test fallback derivation for unknown event types
- Test metadata construction with various payloads
- Test actor ID extraction from payload

### Integration Tests
- Publish event via EventBus → verify Activity appears in DB
- Publish event without org ID → verify Activity has null organization_id
- Publish event with unknown type → verify Activity created with derived type
- Test concurrent event processing (multiple events in rapid sequence)

---

**Version**: 1.0 | **Created**: 2026-05-12 | **Status**: Phase 1 Design