package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MetricEventType constants for the event_type field.
const (
	MetricEventTypeUserCreated    = "user.created"
	MetricEventTypeUserDeleted    = "user.deleted"
	MetricEventTypeInvoiceCreated = "invoice.created"
	MetricEventTypeInvoicePaid    = "invoice.paid"
	MetricEventTypeNewsPublished  = "news.published"
	MetricEventTypeNewsDeleted    = "news.deleted"
	MetricEventTypeCommentCreated = "comment.created"
	MetricEventTypeMediaUploaded   = "media.uploaded"
	MetricEventTypeFileVersioned  = "media.versioned"
	MetricEventTypeLoginSuccess   = "auth.login.success"
	MetricEventTypeLoginFailed    = "auth.login.failed"
)

// MetricResourceType constants for the resource_type field.
const (
	MetricResourceUser    = "user"
	MetricResourceInvoice = "invoice"
	MetricResourceNews    = "news"
	MetricResourceComment = "comment"
	MetricResourceMedia   = "media"
	MetricResourceAuth    = "auth"
)

// MetricPeriodType constants for dashboard metric period types.
const (
	MetricPeriodHourly  = "hourly"
	MetricPeriodDaily   = "daily"
	MetricPeriodWeekly  = "weekly"
	MetricPeriodMonthly = "monthly"
)

// validMetricEventTypes is a lookup map for valid MetricEventType values.
var validMetricEventTypes = map[string]bool{
	MetricEventTypeUserCreated:    true,
	MetricEventTypeUserDeleted:    true,
	MetricEventTypeInvoiceCreated: true,
	MetricEventTypeInvoicePaid:    true,
	MetricEventTypeNewsPublished:  true,
	MetricEventTypeNewsDeleted:    true,
	MetricEventTypeCommentCreated: true,
	MetricEventTypeMediaUploaded:  true,
	MetricEventTypeFileVersioned:  true,
	MetricEventTypeLoginSuccess:   true,
	MetricEventTypeLoginFailed:    true,
}

// validMetricPeriodTypes is a lookup map for valid MetricPeriodType values.
var validMetricPeriodTypes = map[string]bool{
	MetricPeriodHourly:  true,
	MetricPeriodDaily:   true,
	MetricPeriodWeekly:  true,
	MetricPeriodMonthly: true,
}

// ValidateEventType returns true if the given string is a valid MetricEventType.
func ValidateEventType(t string) bool {
	return validMetricEventTypes[t]
}

// ValidatePeriodType returns true if the given string is a valid MetricPeriodType.
func ValidatePeriodType(t string) bool {
	return validMetricPeriodTypes[t]
}

// MetricEvent represents an immutable analytics event recorded for aggregation.
// MetricEvents are never updated after creation (no UpdatedAt field).
type MetricEvent struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	EventType      string         `gorm:"size:100;not null;index"`
	ActorID        uuid.UUID      `gorm:"type:uuid;not null;index"`
	ResourceType   string         `gorm:"size:50;not null;index"`
	ResourceID     string         `gorm:"size:100;not null;index"`
	OrganizationID *uuid.UUID     `gorm:"type:uuid;index"`
	Metadata       datatypes.JSON `gorm:"type:jsonb"`
	EventTimestamp time.Time      `gorm:"not null;index"`
	Date           time.Time      `gorm:"not null;index"`
	Hour           int            `gorm:"not null;check:hour >= 0 AND hour <= 23"`
	ArchivedAt     *time.Time     `gorm:""`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
	CreatedAt      time.Time      `gorm:"autoCreateTime"`
}

// TableName returns the table name for the MetricEvent model.
func (MetricEvent) TableName() string {
	return "metric_events"
}

// IsArchived returns true if the metric event has been archived by the reaper.
func (m *MetricEvent) IsArchived() bool {
	return m.ArchivedAt != nil
}

// ToResponse converts MetricEvent to MetricEventResponse.
func (m *MetricEvent) ToResponse() MetricEventResponse {
	var metadata json.RawMessage
	if len(m.Metadata) > 0 {
		metadata = json.RawMessage(m.Metadata)
	}

	var orgIDStr *string
	if m.OrganizationID != nil {
		s := m.OrganizationID.String()
		orgIDStr = &s
	}

	var archivedAtStr *string
	if m.ArchivedAt != nil {
		formatted := m.ArchivedAt.Format(time.RFC3339)
		archivedAtStr = &formatted
	}

	return MetricEventResponse{
		ID:             m.ID.String(),
		EventType:      m.EventType,
		ActorID:        m.ActorID.String(),
		ResourceType:   m.ResourceType,
		ResourceID:     m.ResourceID,
		OrganizationID: orgIDStr,
		Metadata:       metadata,
		EventTimestamp:  m.EventTimestamp.Format(time.RFC3339),
		Date:           m.Date.Format("2006-01-02"),
		Hour:           m.Hour,
		ArchivedAt:     archivedAtStr,
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
	}
}

// MetricEventResponse represents a metric event in API responses.
type MetricEventResponse struct {
	ID             string          `json:"id"`
	EventType      string          `json:"event_type"`
	ActorID        string          `json:"actor_id"`
	ResourceType   string          `json:"resource_type"`
	ResourceID     string          `json:"resource_id"`
	OrganizationID *string         `json:"organization_id,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	EventTimestamp  string          `json:"event_timestamp"`
	Date           string          `json:"date"`
	Hour           int             `json:"hour"`
	ArchivedAt     *string         `json:"archived_at,omitempty"`
	CreatedAt      string          `json:"created_at"`
}

// MetricTimeSeriesPoint represents a single data point in a time series.
type MetricTimeSeriesPoint struct {
	Date  string  `json:"date"`
	Hour  *int    `json:"hour,omitempty"`
	Count int64   `json:"count"`
	Value float64 `json:"value,omitempty"`
}