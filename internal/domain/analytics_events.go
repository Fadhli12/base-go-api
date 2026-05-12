package domain

import (
	"github.com/google/uuid"
)

// MetricCategory constants for dashboard metric grouping.
const (
	MetricCategoryUserActivity     = "user_activity"
	MetricCategoryContentMetrics   = "content_metrics"
	MetricCategoryEngagementMetrics = "engagement_metrics"
	MetricCategorySystemMetrics     = "system_metrics"
)

// AnalyticsMapping defines how an EventBus event type maps to MetricEvent fields.
type AnalyticsMapping struct {
	MetricCategory    string
	ResourceType      string
	ExtractResourceID func(payload map[string]interface{}) string
	ExtractActorID   func(payload map[string]interface{}) (uuid.UUID, bool)
}

// extractPayloadID extracts "id" from an event payload.
func extractPayloadID(payload map[string]interface{}) string {
	if id, ok := payload["id"]; ok {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// extractPayloadActorID extracts "actor_id" from an event payload as uuid.UUID.
func extractPayloadActorID(payload map[string]interface{}) (uuid.UUID, bool) {
	if actorID, ok := payload["actor_id"]; ok {
		if s, ok := actorID.(string); ok {
			id, err := uuid.Parse(s)
			if err == nil {
				return id, true
			}
		}
	}
	return uuid.Nil, false
}

// extractPayloadUserID extracts "user_id" from an event payload as uuid.UUID.
func extractPayloadUserID(payload map[string]interface{}) (uuid.UUID, bool) {
	if userID, ok := payload["user_id"]; ok {
		if s, ok := userID.(string); ok {
			id, err := uuid.Parse(s)
			if err == nil {
				return id, true
			}
		}
	}
	return uuid.Nil, false
}

// analyticsEventMapping maps EventBus event types to AnalyticsMapping.
var analyticsEventMapping = map[string]AnalyticsMapping{
	"user.created": {
		MetricCategory:    MetricCategoryUserActivity,
		ResourceType:      MetricResourceUser,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"user.deleted": {
		MetricCategory:    MetricCategoryUserActivity,
		ResourceType:      MetricResourceUser,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"invoice.created": {
		MetricCategory:    MetricCategoryContentMetrics,
		ResourceType:      MetricResourceInvoice,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"invoice.paid": {
		MetricCategory:    MetricCategoryContentMetrics,
		ResourceType:      MetricResourceInvoice,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"news.published": {
		MetricCategory:    MetricCategoryContentMetrics,
		ResourceType:      MetricResourceNews,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"news.deleted": {
		MetricCategory:    MetricCategoryContentMetrics,
		ResourceType:      MetricResourceNews,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"comment.created": {
		MetricCategory:    MetricCategoryEngagementMetrics,
		ResourceType:      MetricResourceComment,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"media.uploaded": {
		MetricCategory:    MetricCategoryContentMetrics,
		ResourceType:      MetricResourceMedia,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"media.versioned": {
		MetricCategory:    MetricCategoryContentMetrics,
		ResourceType:      MetricResourceMedia,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadActorID,
	},
	"auth.login.success": {
		MetricCategory:    MetricCategorySystemMetrics,
		ResourceType:      MetricResourceAuth,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadUserID,
	},
	"auth.login.failed": {
		MetricCategory:    MetricCategorySystemMetrics,
		ResourceType:      MetricResourceAuth,
		ExtractResourceID: extractPayloadID,
		ExtractActorID:   extractPayloadUserID,
	},
}

// GetAnalyticsMapping returns the AnalyticsMapping for a given event type.
// Returns (AnalyticsMapping{}, false) for unknown event types.
func GetAnalyticsMapping(eventType string) (AnalyticsMapping, bool) {
	if mapping, ok := analyticsEventMapping[eventType]; ok {
		return mapping, true
	}
	return AnalyticsMapping{}, false
}