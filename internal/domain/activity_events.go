package domain

// ActivityMapping defines how an EventBus event type maps to Activity fields.
type ActivityMapping struct {
	ActionType   string
	ResourceType string
}

// eventMapping maps EventBus event types to Activity ActionType and ResourceType.
var eventMapping = map[string]ActivityMapping{
	"user.created":    {ActionType: ActivityActionCreated, ResourceType: ActivityResourceUser},
	"user.deleted":    {ActionType: ActivityActionDeleted, ResourceType: ActivityResourceUser},
	"invoice.created": {ActionType: ActivityActionCreated, ResourceType: ActivityResourceInvoice},
	"invoice.paid":    {ActionType: ActivityActionPaid, ResourceType: ActivityResourceInvoice},
	"news.published":  {ActionType: ActivityActionPublished, ResourceType: ActivityResourceNews},
	"news.deleted":    {ActionType: ActivityActionDeleted, ResourceType: ActivityResourceNews},
	"comment.created": {ActionType: ActivityActionCreated, ResourceType: ActivityResourceComment},
}

// GetActivityMapping returns the mapping for an event type, with fallback derivation.
// For unknown types like "order.shipped", it returns (ActivityMapping{}, false).
func GetActivityMapping(eventType string) (ActivityMapping, bool) {
	if mapping, ok := eventMapping[eventType]; ok {
		return mapping, true
	}
	// Fallback: derive from dot-separated type (prefix=resource, suffix=action)
	return ActivityMapping{}, false
}