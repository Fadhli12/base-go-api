package unit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestMetricEvent_TableName(t *testing.T) {
	t.Run("returns metric_events", func(t *testing.T) {
		m := domain.MetricEvent{}
		assert.Equal(t, "metric_events", m.TableName())
	})
}

func TestMetricEvent_IsArchived(t *testing.T) {
	t.Run("returns false when ArchivedAt is nil", func(t *testing.T) {
		m := &domain.MetricEvent{
			ArchivedAt: nil,
		}
		assert.False(t, m.IsArchived())
	})

	t.Run("returns true when ArchivedAt is not nil", func(t *testing.T) {
		now := time.Now()
		m := &domain.MetricEvent{
			ArchivedAt: &now,
		}
		assert.True(t, m.IsArchived())
	})
}

func TestMetricEvent_ToResponse(t *testing.T) {
	t.Run("with OrganizationID nil and ArchivedAt nil", func(t *testing.T) {
		eventTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
		date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		createdAt := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
		actorID := uuid.New()

		m := &domain.MetricEvent{
			ID:             uuid.New(),
			EventType:      domain.MetricEventTypeUserCreated,
			ActorID:        actorID,
			ResourceType:   domain.MetricResourceUser,
			ResourceID:     "user-123",
			OrganizationID: nil,
			Metadata:       datatypes.JSON(`{"key":"value"}`),
			EventTimestamp: eventTime,
			Date:           date,
			Hour:           10,
			ArchivedAt:     nil,
			CreatedAt:      createdAt,
		}

		resp := m.ToResponse()

		assert.Equal(t, m.ID.String(), resp.ID)
		assert.Equal(t, domain.MetricEventTypeUserCreated, resp.EventType)
		assert.Equal(t, actorID.String(), resp.ActorID)
		assert.Equal(t, domain.MetricResourceUser, resp.ResourceType)
		assert.Equal(t, "user-123", resp.ResourceID)
		assert.Nil(t, resp.OrganizationID)
		assert.Equal(t, json.RawMessage(`{"key":"value"}`), resp.Metadata)
		assert.Equal(t, "2024-03-15T10:30:00Z", resp.EventTimestamp)
		assert.Equal(t, "2024-03-15", resp.Date)
		assert.Equal(t, 10, resp.Hour)
		assert.Nil(t, resp.ArchivedAt)
		assert.Equal(t, "2024-03-15T10:30:00Z", resp.CreatedAt)
	})

	t.Run("with OrganizationID set and ArchivedAt set", func(t *testing.T) {
		eventTime := time.Date(2024, 6, 20, 14, 45, 0, 0, time.UTC)
		date := time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC)
		createdAt := time.Date(2024, 6, 20, 14, 45, 0, 0, time.UTC)
		archivedAt := time.Date(2024, 6, 25, 0, 0, 0, 0, time.UTC)
		orgID := uuid.New()
		actorID := uuid.New()

		m := &domain.MetricEvent{
			ID:             uuid.New(),
			EventType:      domain.MetricEventTypeInvoicePaid,
			ActorID:        actorID,
			ResourceType:   domain.MetricResourceInvoice,
			ResourceID:     "invoice-456",
			OrganizationID: &orgID,
			Metadata:       datatypes.JSON(`{}`),
			EventTimestamp: eventTime,
			Date:           date,
			Hour:           14,
			ArchivedAt:     &archivedAt,
			CreatedAt:      createdAt,
		}

		resp := m.ToResponse()

		require.NotNil(t, resp.OrganizationID)
		assert.Equal(t, orgID.String(), *resp.OrganizationID)
		require.NotNil(t, resp.ArchivedAt)
		assert.Equal(t, "2024-06-25T00:00:00Z", *resp.ArchivedAt)
	})

	t.Run("date and hour formatting", func(t *testing.T) {
		eventTime := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
		date := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

		m := &domain.MetricEvent{
			ID:             uuid.New(),
			EventType:      domain.MetricEventTypeNewsPublished,
			ActorID:        uuid.New(),
			ResourceType:   domain.MetricResourceNews,
			ResourceID:     "news-789",
			OrganizationID: nil,
			Metadata:       nil,
			EventTimestamp: eventTime,
			Date:           date,
			Hour:           23,
			ArchivedAt:     nil,
			CreatedAt:      eventTime,
		}

		resp := m.ToResponse()

		assert.Equal(t, "2024-12-31", resp.Date)
		assert.Equal(t, 23, resp.Hour)
		assert.Equal(t, "2024-12-31T23:59:59Z", resp.EventTimestamp)
	})
}

func TestValidateEventType(t *testing.T) {
	t.Run("all 11 valid event types return true", func(t *testing.T) {
		validTypes := []string{
			domain.MetricEventTypeUserCreated,
			domain.MetricEventTypeUserDeleted,
			domain.MetricEventTypeInvoiceCreated,
			domain.MetricEventTypeInvoicePaid,
			domain.MetricEventTypeNewsPublished,
			domain.MetricEventTypeNewsDeleted,
			domain.MetricEventTypeCommentCreated,
			domain.MetricEventTypeMediaUploaded,
			domain.MetricEventTypeFileVersioned,
			domain.MetricEventTypeLoginSuccess,
			domain.MetricEventTypeLoginFailed,
		}

		for _, eventType := range validTypes {
			t.Run(eventType, func(t *testing.T) {
				assert.True(t, domain.ValidateEventType(eventType))
			})
		}
	})

	t.Run("invalid event types return false", func(t *testing.T) {
		invalidTypes := []string{
			"invalid.type",
			"",
			"user.updated",
			"USER.CREATED",
			"invoice.sent",
			"unknown",
		}

		for _, eventType := range invalidTypes {
			t.Run(eventType, func(t *testing.T) {
				assert.False(t, domain.ValidateEventType(eventType))
			})
		}
	})
}

func TestValidatePeriodType(t *testing.T) {
	t.Run("valid period types return true", func(t *testing.T) {
		assert.True(t, domain.ValidatePeriodType("daily"))
		assert.True(t, domain.ValidatePeriodType("weekly"))
		assert.True(t, domain.ValidatePeriodType("monthly"))
	})

	t.Run("invalid period type returns false", func(t *testing.T) {
		assert.False(t, domain.ValidatePeriodType("yearly"))
		assert.False(t, domain.ValidatePeriodType(""))
		assert.False(t, domain.ValidatePeriodType("invalid"))
		assert.False(t, domain.ValidatePeriodType("DAILY"))
	})
}

func TestDashboardMetric_TableName(t *testing.T) {
	t.Run("returns dashboard_metrics", func(t *testing.T) {
		d := domain.DashboardMetric{}
		assert.Equal(t, "dashboard_metrics", d.TableName())
	})
}

func TestDashboardMetric_ToResponse(t *testing.T) {
	t.Run("with OrganizationID nil", func(t *testing.T) {
		periodStart := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		periodEnd := time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC)
		calculatedAt := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
		createdAt := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
		updatedAt := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)

		d := &domain.DashboardMetric{
			ID:             uuid.New(),
			MetricType:     "user_activity",
			PeriodType:     domain.MetricPeriodMonthly,
			PeriodStart:    periodStart,
			PeriodEnd:      periodEnd,
			Value:          1234.56,
			Metadata:       datatypes.JSON(`{"source":"api"}`),
			OrganizationID: nil,
			CalculatedAt:   calculatedAt,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		}

		resp := d.ToResponse()

		assert.Equal(t, d.ID.String(), resp.ID)
		assert.Equal(t, "user_activity", resp.MetricType)
		assert.Equal(t, domain.MetricPeriodMonthly, resp.PeriodType)
		assert.Equal(t, "2024-03-01T00:00:00Z", resp.PeriodStart)
		assert.Equal(t, "2024-03-31T23:59:59Z", resp.PeriodEnd)
		assert.Equal(t, 1234.56, resp.Value)
		assert.Equal(t, json.RawMessage(`{"source":"api"}`), resp.Metadata)
		assert.Nil(t, resp.OrganizationID)
		assert.Equal(t, "2024-04-01T00:00:00Z", resp.CalculatedAt)
		assert.Equal(t, "2024-04-01T00:00:00Z", resp.CreatedAt)
		assert.Equal(t, "2024-04-01T00:00:00Z", resp.UpdatedAt)
	})

	t.Run("with OrganizationID set", func(t *testing.T) {
		orgID := uuid.New()

		d := &domain.DashboardMetric{
			ID:             uuid.New(),
			MetricType:     "content_metrics",
			PeriodType:     domain.MetricPeriodWeekly,
			PeriodStart:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			PeriodEnd:      time.Date(2024, 1, 7, 23, 59, 59, 0, time.UTC),
			Value:          999.99,
			Metadata:       nil,
			OrganizationID: &orgID,
			CalculatedAt:   time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
			CreatedAt:      time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
			UpdatedAt:      time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
		}

		resp := d.ToResponse()

		require.NotNil(t, resp.OrganizationID)
		assert.Equal(t, orgID.String(), *resp.OrganizationID)
	})

	t.Run("time formatting is RFC3339", func(t *testing.T) {
		ts := time.Date(2024, 7, 15, 12, 30, 45, 0, time.UTC)
		periodEnd := time.Date(2024, 7, 16, 12, 30, 44, 0, time.UTC)

		d := &domain.DashboardMetric{
			ID:             uuid.New(),
			MetricType:     "system_metrics",
			PeriodType:     domain.MetricPeriodDaily,
			PeriodStart:    ts,
			PeriodEnd:      periodEnd,
			Value:          0,
			Metadata:       nil,
			OrganizationID: nil,
			CalculatedAt:   ts,
			CreatedAt:      ts,
			UpdatedAt:      ts,
		}

		resp := d.ToResponse()

		assert.Equal(t, "2024-07-15T12:30:45Z", resp.PeriodStart)
		assert.Equal(t, "2024-07-16T12:30:44Z", resp.PeriodEnd)
		assert.Equal(t, "2024-07-15T12:30:45Z", resp.CalculatedAt)
		assert.Equal(t, "2024-07-15T12:30:45Z", resp.CreatedAt)
		assert.Equal(t, "2024-07-15T12:30:45Z", resp.UpdatedAt)
	})
}

func TestDashboardPreference_TableName(t *testing.T) {
	t.Run("returns dashboard_preferences", func(t *testing.T) {
		p := domain.DashboardPreference{}
		assert.Equal(t, "dashboard_preferences", p.TableName())
	})
}

func TestDashboardPreference_ToResponse(t *testing.T) {
	t.Run("with nil MetricCategories", func(t *testing.T) {
		orgID := uuid.New()
		userID := uuid.New()
		createdAt := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
		updatedAt := time.Date(2024, 5, 2, 0, 0, 0, 0, time.UTC)

		p := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			MetricCategories: nil,
			UpdatedByUserID:  userID,
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}

		resp := p.ToResponse()

		assert.Equal(t, orgID.String(), resp.OrganizationID)
		assert.Nil(t, resp.MetricCategories)
		assert.Equal(t, userID.String(), resp.UpdatedByUserID)
		assert.Equal(t, "2024-05-01T00:00:00Z", resp.CreatedAt)
		assert.Equal(t, "2024-05-02T00:00:00Z", resp.UpdatedAt)
	})

	t.Run("with non-nil MetricCategories", func(t *testing.T) {
		orgID := uuid.New()
		userID := uuid.New()

		p := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			MetricCategories: datatypes.JSON(`{"user_activity":true,"content_metrics":false}`),
			UpdatedByUserID:  userID,
			CreatedAt:        time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
		}

		resp := p.ToResponse()

		require.NotNil(t, resp.MetricCategories)
		assert.Equal(t, true, resp.MetricCategories["user_activity"])
		assert.Equal(t, false, resp.MetricCategories["content_metrics"])
	})

	t.Run("OrganizationID string formatting", func(t *testing.T) {
		p := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
			MetricCategories: nil,
			UpdatedByUserID:  uuid.New(),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		resp := p.ToResponse()

		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", resp.OrganizationID)
	})

	t.Run("UpdatedByUserID string formatting", func(t *testing.T) {
		p := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   uuid.New(),
			MetricCategories: nil,
			UpdatedByUserID:  uuid.MustParse("987fcdeb-51a2-34f6-b789-123456789abc"),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		resp := p.ToResponse()

		assert.Equal(t, "987fcdeb-51a2-34f6-b789-123456789abc", resp.UpdatedByUserID)
	})
}

func TestDefaultMetricCategories(t *testing.T) {
	t.Run("returns map with 4 keys", func(t *testing.T) {
		categories := domain.DefaultMetricCategories()
		require.NotNil(t, categories)
		assert.Len(t, categories, 4)
	})

	t.Run("all values are true", func(t *testing.T) {
		categories := domain.DefaultMetricCategories()

		assert.True(t, categories["user_activity"])
		assert.True(t, categories["content_metrics"])
		assert.True(t, categories["engagement_metrics"])
		assert.True(t, categories["system_metrics"])
	})

	t.Run("keys are exact", func(t *testing.T) {
		categories := domain.DefaultMetricCategories()

		_, hasUserActivity := categories["user_activity"]
		_, hasContentMetrics := categories["content_metrics"]
		_, hasEngagementMetrics := categories["engagement_metrics"]
		_, hasSystemMetrics := categories["system_metrics"]

		assert.True(t, hasUserActivity)
		assert.True(t, hasContentMetrics)
		assert.True(t, hasEngagementMetrics)
		assert.True(t, hasSystemMetrics)
	})
}

func TestGetAnalyticsMapping_ValidTypes(t *testing.T) {
	t.Run("user.created maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("user.created")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryUserActivity, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceUser, mapping.ResourceType)
		require.NotNil(t, mapping.ExtractResourceID)
		require.NotNil(t, mapping.ExtractActorID)
	})

	t.Run("user.deleted maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("user.deleted")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryUserActivity, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceUser, mapping.ResourceType)
	})

	t.Run("invoice.created maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("invoice.created")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryContentMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceInvoice, mapping.ResourceType)
	})

	t.Run("invoice.paid maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("invoice.paid")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryContentMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceInvoice, mapping.ResourceType)
	})

	t.Run("news.published maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("news.published")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryContentMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceNews, mapping.ResourceType)
	})

	t.Run("news.deleted maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("news.deleted")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryContentMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceNews, mapping.ResourceType)
	})

	t.Run("comment.created maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("comment.created")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryEngagementMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceComment, mapping.ResourceType)
	})

	t.Run("media.uploaded maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("media.uploaded")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryContentMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceMedia, mapping.ResourceType)
	})

	t.Run("media.versioned maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("media.versioned")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategoryContentMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceMedia, mapping.ResourceType)
	})

	t.Run("auth.login.success maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("auth.login.success")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategorySystemMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceAuth, mapping.ResourceType)
	})

	t.Run("auth.login.failed maps correctly", func(t *testing.T) {
		mapping, ok := domain.GetAnalyticsMapping("auth.login.failed")
		require.True(t, ok)
		assert.Equal(t, domain.MetricCategorySystemMetrics, mapping.MetricCategory)
		assert.Equal(t, domain.MetricResourceAuth, mapping.ResourceType)
	})
}

func TestGetAnalyticsMapping_InvalidType(t *testing.T) {
	t.Run("unknown type returns false", func(t *testing.T) {
		_, ok := domain.GetAnalyticsMapping("unknown.event")
		assert.False(t, ok)
	})

	t.Run("empty string returns false", func(t *testing.T) {
		_, ok := domain.GetAnalyticsMapping("")
		assert.False(t, ok)
	})

	t.Run("partial match returns false", func(t *testing.T) {
		_, ok := domain.GetAnalyticsMapping("user")
		assert.False(t, ok)
		_, ok = domain.GetAnalyticsMapping("user.")
		assert.False(t, ok)
	})
}

func TestExtractPayloadID(t *testing.T) {
	t.Run("with id as string", func(t *testing.T) {
		payload := map[string]interface{}{
			"id": "resource-123",
		}

		mapping, _ := domain.GetAnalyticsMapping("user.created")
		id := mapping.ExtractResourceID(payload)
		assert.Equal(t, "resource-123", id)
	})

	t.Run("with missing id key", func(t *testing.T) {
		payload := map[string]interface{}{
			"other": "value",
		}

		mapping, _ := domain.GetAnalyticsMapping("user.created")
		id := mapping.ExtractResourceID(payload)
		assert.Equal(t, "", id)
	})

	t.Run("with non-string id", func(t *testing.T) {
		payload := map[string]interface{}{
			"id": 12345,
		}

		mapping, _ := domain.GetAnalyticsMapping("user.created")
		id := mapping.ExtractResourceID(payload)
		assert.Equal(t, "", id)
	})
}

func TestExtractPayloadActorID(t *testing.T) {
	t.Run("with valid UUID string", func(t *testing.T) {
		actorUUID := uuid.New()
		payload := map[string]interface{}{
			"actor_id": actorUUID.String(),
		}

		mapping, _ := domain.GetAnalyticsMapping("user.created")
		actorID, ok := mapping.ExtractActorID(payload)
		require.True(t, ok)
		assert.Equal(t, actorUUID, actorID)
	})

	t.Run("with missing actor_id key", func(t *testing.T) {
		payload := map[string]interface{}{
			"id": "resource-123",
		}

		mapping, _ := domain.GetAnalyticsMapping("user.created")
		actorID, ok := mapping.ExtractActorID(payload)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, actorID)
	})

	t.Run("with invalid UUID string", func(t *testing.T) {
		payload := map[string]interface{}{
			"actor_id": "not-a-valid-uuid",
		}

		mapping, _ := domain.GetAnalyticsMapping("user.created")
		actorID, ok := mapping.ExtractActorID(payload)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, actorID)
	})
}

func TestExtractPayloadUserID(t *testing.T) {
	t.Run("with valid UUID string", func(t *testing.T) {
		userUUID := uuid.New()
		payload := map[string]interface{}{
			"user_id": userUUID.String(),
		}

		mapping, _ := domain.GetAnalyticsMapping("auth.login.success")
		userID, ok := mapping.ExtractActorID(payload)
		require.True(t, ok)
		assert.Equal(t, userUUID, userID)
	})

	t.Run("with missing user_id key", func(t *testing.T) {
		payload := map[string]interface{}{
			"actor_id": uuid.New().String(),
		}

		mapping, _ := domain.GetAnalyticsMapping("auth.login.success")
		userID, ok := mapping.ExtractActorID(payload)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, userID)
	})

	t.Run("with invalid UUID string", func(t *testing.T) {
		payload := map[string]interface{}{
			"user_id": "invalid-uuid-format",
		}

		mapping, _ := domain.GetAnalyticsMapping("auth.login.success")
		userID, ok := mapping.ExtractActorID(payload)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, userID)
	})
}
