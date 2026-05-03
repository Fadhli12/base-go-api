//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createTestWebhookForEvents creates a webhook directly in the database for event tests.
// This bypasses URL validation so test server addresses can be used.
func createTestWebhookForEvents(t *testing.T, db *gorm.DB, url string, events []string, active bool) *domain.Webhook {
	eventsJSON, err := json.Marshal(events)
	require.NoError(t, err)

	secret := "whsec_" + strings.Repeat("abcdef0123456789", 2)

	wh := &domain.Webhook{
		Name:      "Test Webhook",
		URL:       url,
		Secret:    secret,
		Events:    eventsJSON,
		Active:    active,
		RateLimit: 100,
	}
	require.NoError(t, db.Create(wh).Error)
	return wh
}

// createTestOrgWebhookForEvents creates an org-scoped webhook directly in the database.
func createTestOrgWebhookForEvents(t *testing.T, db *gorm.DB, orgID uuid.UUID, url string, events []string, active bool) *domain.Webhook {
	eventsJSON, err := json.Marshal(events)
	require.NoError(t, err)

	secret := "whsec_" + strings.Repeat("abcdef0123456789", 2)

	wh := &domain.Webhook{
		Name:           "Test Org Webhook",
		URL:            url,
		Secret:         secret,
		Events:         eventsJSON,
		Active:         active,
		RateLimit:      100,
		OrganizationID: &orgID,
	}
	require.NoError(t, db.Create(wh).Error)
	return wh
}

// newWebhookServiceWithDeps creates a WebhookService with real repos, queue and rate limiter.
func newWebhookServiceWithDeps(db *gorm.DB, redisClient *redis.Client) *service.WebhookService {
	webhookRepo := repository.NewWebhookRepository(db)
	deliveryRepo := repository.NewWebhookDeliveryRepository(db)
	cfg := config.DefaultWebhookConfig()
	webhookSvc := service.NewWebhookService(webhookRepo, deliveryRepo, &cfg, logger.NewNopLogger())
	webhookQueue := service.NewWebhookRedisQueue(redisClient)
	webhookRateLimiter := service.NewWebhookRateLimiter(redisClient, cfg.RateLimit)
	webhookSvc.SetQueue(webhookQueue)
	webhookSvc.SetRateLimiter(webhookRateLimiter)
	return webhookSvc
}

// =============================================================================
// EVENT EMISSION TESTS
// =============================================================================

// TestWebhookEventEmission_UserCreated verifies that publishing a user.created
// event creates a queued delivery record for a matching webhook.
func TestWebhookEventEmission_UserCreated(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	// Set up EventBus and subscribe the webhook service
	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)
	defer eventBus.Stop()

	// Create a webhook subscribed to user.created
	wh := createTestWebhookForEvents(t, suite.DB, "https://example.com/webhook", []string{"user.created"}, true)

	// Publish the event
	payload := map[string]interface{}{
		"user_id": uuid.New().String(),
		"email":   "test@example.com",
	}
	err = eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})
	require.NoError(t, err)

	// Allow the EventBus dispatch goroutine to process
	time.Sleep(500 * time.Millisecond)

	// Assert: a delivery record was created
	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)
	deliveries, total, err := deliveryRepo.FindByWebhookID(ctx, wh.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, deliveries, 1)
	assert.Equal(t, "user.created", deliveries[0].Event)
	assert.Equal(t, domain.WebhookDeliveryStatusQueued, deliveries[0].Status)
}

// TestWebhookEventEmission_NoMatchingWebhooks verifies that publishing an event
// for which no webhook is subscribed does not create any delivery records.
func TestWebhookEventEmission_NoMatchingWebhooks(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)
	defer eventBus.Stop()

	// Create a webhook subscribed to invoice.paid (not user.created)
	wh := createTestWebhookForEvents(t, suite.DB, "https://example.com/webhook", []string{"invoice.paid"}, true)

	// Publish user.created event
	payload := map[string]interface{}{"user_id": uuid.New().String()}
	err = eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Assert: no delivery records for this webhook
	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)
	deliveries, total, err := deliveryRepo.FindByWebhookID(ctx, wh.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, deliveries, 0)
}

// TestWebhookEventEmission_MultipleWebhooksMatch verifies that when multiple
// active webhooks subscribe to the same event, a delivery record is created
// for each webhook.
func TestWebhookEventEmission_MultipleWebhooksMatch(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)
	defer eventBus.Stop()

	// Create two webhooks both subscribing to user.created with unique URLs
	wh1 := createTestWebhookForEvents(t, suite.DB, "https://hook1.example.com/webhook", []string{"user.created"}, true)
	wh2 := createTestWebhookForEvents(t, suite.DB, "https://hook2.example.com/webhook", []string{"user.created"}, true)

	payload := map[string]interface{}{"user_id": uuid.New().String()}
	err = eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)

	deliveries1, total1, err := deliveryRepo.FindByWebhookID(ctx, wh1.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total1)
	assert.Len(t, deliveries1, 1)

	deliveries2, total2, err := deliveryRepo.FindByWebhookID(ctx, wh2.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	assert.Len(t, deliveries2, 1)
}

// TestWebhookEventEmission_InactiveWebhookSkipped verifies that inactive
// webhooks do not receive delivery records when an event is published.
func TestWebhookEventEmission_InactiveWebhookSkipped(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)
	defer eventBus.Stop()

	// Create an inactive webhook subscribed to user.created
	wh := createTestWebhookForEvents(t, suite.DB, "https://example.com/webhook", []string{"user.created"}, false)

	payload := map[string]interface{}{"user_id": uuid.New().String()}
	err = eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)
	deliveries, total, err := deliveryRepo.FindByWebhookID(ctx, wh.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, deliveries, 0)
}

// TestEventBus_StartStop verifies that the EventBus can be started, accept
// publications, stopped cleanly, and rejects publications after stop.
func TestEventBus_StartStop(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	eventBus := domain.NewEventBus(2)

	err := eventBus.Start(ctx)
	require.NoError(t, err)

	// Publish an event while running
	err = eventBus.Publish(domain.WebhookEvent{Type: "test.event", Payload: "test-payload"})
	require.NoError(t, err)

	// Allow dispatch to drain
	time.Sleep(200 * time.Millisecond)

	// Stop should return nil for a clean shutdown
	err = eventBus.Stop()
	require.NoError(t, err)

	// Publishing after stop should return an error
	err = eventBus.Publish(domain.WebhookEvent{Type: "test.event", Payload: "after-stop"})
	require.Error(t, err)
}

// TestWebhookEventEmission_OrgScopedEvent verifies that when an org-scoped
// event is published, both global webhooks AND matching org-scoped webhooks
// receive delivery records. A webhook from a different org should NOT receive it.
func TestWebhookEventEmission_OrgScopedEvent(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)
	defer eventBus.Stop()

	orgID := uuid.New()
	otherOrgID := uuid.New()

	// Create a GLOBAL webhook subscribed to user.created
	globalWH := createTestWebhookForEvents(t, suite.DB, "https://global.example.com/webhook", []string{"user.created"}, true)

	// Create an ORG-SCOPED webhook for orgID subscribed to user.created
	orgWH := createTestOrgWebhookForEvents(t, suite.DB, orgID, "https://org.example.com/webhook", []string{"user.created"}, true)

	// Create an ORG-SCOPED webhook for a DIFFERENT org subscribed to user.created
	otherOrgWH := createTestOrgWebhookForEvents(t, suite.DB, otherOrgID, "https://other.example.com/webhook", []string{"user.created"}, true)

	// Publish an event WITH org context
	payload := map[string]interface{}{"user_id": uuid.New().String()}
	err = eventBus.Publish(domain.WebhookEvent{
		Type:    "user.created",
		Payload: payload,
		OrgID:   &orgID,
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)

	// Global webhook should receive the event
	globalDeliveries, globalTotal, err := deliveryRepo.FindByWebhookID(ctx, globalWH.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), globalTotal, "global webhook should receive org-scoped event")

	// Matching org webhook should receive the event
	orgDeliveries, orgTotal, err := deliveryRepo.FindByWebhookID(ctx, orgWH.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), orgTotal, "org-scoped webhook should receive event for its org")

	// Other org webhook should NOT receive the event
	otherDeliveries, otherTotal, err := deliveryRepo.FindByWebhookID(ctx, otherOrgWH.ID, repository.ListDeliveriesQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), otherTotal, "webhook from different org should NOT receive event")
	_ = globalDeliveries
	_ = orgDeliveries
	_ = otherDeliveries
}
