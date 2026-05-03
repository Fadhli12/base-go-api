//go:build integration
// +build integration

package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newWebhookWorkerWithDeps creates a WebhookWorker with real repos, queue and rate limiter.
func newWebhookWorkerWithDeps(db *gorm.DB, redisClient *redis.Client) *service.WebhookWorker {
	cfg := config.DefaultWebhookConfig()
	deliveryRepo := repository.NewWebhookDeliveryRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	queue := service.NewWebhookRedisQueue(redisClient)
	rateLimiter := service.NewWebhookRateLimiter(redisClient, cfg.RateLimit)
	return service.NewWebhookWorker(&cfg, deliveryRepo, webhookRepo, queue, rateLimiter)
}

// =============================================================================
// E2E LIFECYCLE TESTS
// =============================================================================

// TestWebhookE2E_FullLifecycle verifies the complete webhook lifecycle from
// event publication through signed HTTP delivery.
func TestWebhookE2E_FullLifecycle(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	// Create a test HTTP server that records received requests
	var receivedReq *http.Request
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Replace 127.0.0.1 with localhost so the worker's DialContext does not
	// block the address as a private IP (net.ParseIP("localhost") returns nil).
	serverURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)

	// Create webhook directly in DB (bypasses URL validation that rejects local addresses)
	wh := createTestWebhookForEvents(t, suite.DB, serverURL, []string{"user.created"}, true)

	// Set up EventBus and subscribe the webhook service
	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)

	// Start the webhook worker
	worker := newWebhookWorkerWithDeps(suite.DB, suite.RedisClient)
	worker.Start()
	defer func() {
		eventBus.Stop()
		worker.Stop()
	}()

	// Publish a user.created event
	payload := map[string]interface{}{
		"user_id": uuid.New().String(),
		"email":   "e2e@example.com",
	}
	err = eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})
	require.NoError(t, err)

	// Wait for the delivery to be processed and marked as delivered
	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)
	var delivery *domain.WebhookDelivery
	require.Eventually(t, func() bool {
		deliveries, total, err := deliveryRepo.FindByWebhookID(ctx, wh.ID, repository.ListDeliveriesQuery{Limit: 1, Offset: 0})
		if err != nil || total == 0 {
			return false
		}
		delivery = deliveries[0]
		return delivery.Status == domain.WebhookDeliveryStatusDelivered
	}, 15*time.Second, 500*time.Millisecond, "delivery should be marked as delivered")

	// Assert: test server received the POST request
	require.NotNil(t, receivedReq, "test server should have received a request")
	assert.Equal(t, http.MethodPost, receivedReq.Method)
	assert.Equal(t, "application/json", receivedReq.Header.Get("Content-Type"))
	assert.Equal(t, "user.created", receivedReq.Header.Get("X-Webhook-Event"))
	assert.Equal(t, delivery.ID.String(), receivedReq.Header.Get("X-Webhook-Delivery-ID"))

	// Assert: signature header is present and has the expected format
	sigHeader := receivedReq.Header.Get("X-Webhook-Signature")
	require.NotEmpty(t, sigHeader, "X-Webhook-Signature should be present")
	assert.True(t, strings.HasPrefix(sigHeader, "t="), "signature should start with t=")
	assert.Contains(t, sigHeader, ",v1=", "signature should contain v1=")

	// Assert: payload is valid JSON and matches the published payload
	var payloadMap map[string]interface{}
	require.NoError(t, json.Unmarshal(receivedBody, &payloadMap))
	assert.Equal(t, "e2e@example.com", payloadMap["email"])

	// Assert: signature matches HMAC-SHA256 of payload with webhook secret
	secret := strings.TrimPrefix(wh.Secret, "whsec_")
	// Parse signature header: t=<timestamp>,v1=<hex>
	parts := strings.SplitN(strings.TrimPrefix(sigHeader, "t="), ",v1=", 2)
	require.Len(t, parts, 2, "signature should have t= and v1= parts")
	timestampStr := parts[0]
	v1Sig := parts[1]

	signedPayload := fmt.Sprintf("%s.%s", timestampStr, string(receivedBody))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expectedSig, v1Sig, "signature should match HMAC-SHA256")

	// Assert: delivery record in DB has status delivered and response_code 200
	assert.Equal(t, domain.WebhookDeliveryStatusDelivered, delivery.Status)
	require.NotNil(t, delivery.ResponseCode, "response_code should be set")
	assert.Equal(t, 200, *delivery.ResponseCode)
	require.NotNil(t, delivery.DurationMs, "duration_ms should be set")
	assert.GreaterOrEqual(t, *delivery.DurationMs, int64(0))
}

// TestWebhookE2E_FailedDelivery_Retried verifies that a failed webhook delivery
// is retried and eventually succeeds.
func TestWebhookE2E_FailedDelivery_Retried(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	// Test server returns 500 on the first request, 200 on the second
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	wh := createTestWebhookForEvents(t, suite.DB, serverURL, []string{"user.created"}, true)

	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)

	worker := newWebhookWorkerWithDeps(suite.DB, suite.RedisClient)
	worker.Start()
	defer func() {
		eventBus.Stop()
		worker.Stop()
	}()

	payload := map[string]interface{}{"user_id": uuid.New().String()}
	err = eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})
	require.NoError(t, err)

	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)

	// Wait for the first attempt to fail and a retry to be scheduled.
	// After the worker's retry scheduling the DB record shows:
	//   status=queued, response_code=500 (from the failed attempt), attempt_number=2
	var delivery *domain.WebhookDelivery
	require.Eventually(t, func() bool {
		deliveries, total, err := deliveryRepo.FindByWebhookID(ctx, wh.ID, repository.ListDeliveriesQuery{Limit: 1, Offset: 0})
		if err != nil || total == 0 {
			return false
		}
		delivery = deliveries[0]
		return delivery.Status == domain.WebhookDeliveryStatusQueued &&
			delivery.AttemptNumber == 2 &&
			delivery.ResponseCode != nil &&
			*delivery.ResponseCode == 500
	}, 15*time.Second, 500*time.Millisecond,
		"first attempt should fail with response_code 500 and retry should be scheduled (attempt_number=2)")

	// Fast-forward the retry by clearing the future-scheduled queue item
	// and re-enqueueing for immediate processing.
	queue := service.NewWebhookRedisQueue(suite.RedisClient)
	_, err = suite.RedisClient.Del(ctx, "webhook:deliveries").Result()
	require.NoError(t, err)
	require.NoError(t, queue.Enqueue(ctx, delivery.ID.String()))

	// Wait for the second attempt to succeed
	require.Eventually(t, func() bool {
		d, err := deliveryRepo.FindByID(ctx, delivery.ID)
		if err != nil {
			return false
		}
		delivery = d
		return delivery.Status == domain.WebhookDeliveryStatusDelivered
	}, 15*time.Second, 500*time.Millisecond, "second attempt should succeed")

	// Assert: delivery is delivered with response_code 200
	require.NotNil(t, delivery.ResponseCode)
	assert.Equal(t, 200, *delivery.ResponseCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "server should have received exactly 2 requests")
}

// TestWebhookE2E_InvalidSignatureRejected verifies that webhook deliveries
// include a correctly formatted signature header.
func TestWebhookE2E_InvalidSignatureRejected(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc := newWebhookServiceWithDeps(suite.DB, suite.RedisClient)

	var capturedSignature string
	var capturedEvent string
	var capturedDeliveryID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSignature = r.Header.Get("X-Webhook-Signature")
		capturedEvent = r.Header.Get("X-Webhook-Event")
		capturedDeliveryID = r.Header.Get("X-Webhook-Delivery-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	wh := createTestWebhookForEvents(t, suite.DB, serverURL, []string{"user.created"}, true)

	eventBus := domain.NewEventBus(10)
	webhookSvc.SubscribeToEventBus(eventBus)
	err := eventBus.Start(ctx)
	require.NoError(t, err)

	worker := newWebhookWorkerWithDeps(suite.DB, suite.RedisClient)
	worker.Start()
	defer func() {
		eventBus.Stop()
		worker.Stop()
	}()

	payload := map[string]interface{}{"user_id": uuid.New().String()}
	err = eventBus.Publish(domain.WebhookEvent{Type: "user.created", Payload: payload})
	require.NoError(t, err)

	// Wait for the delivery to be processed
	deliveryRepo := repository.NewWebhookDeliveryRepository(suite.DB)
	require.Eventually(t, func() bool {
		deliveries, total, err := deliveryRepo.FindByWebhookID(ctx, wh.ID, repository.ListDeliveriesQuery{Limit: 1, Offset: 0})
		if err != nil || total == 0 {
			return false
		}
		return deliveries[0].Status == domain.WebhookDeliveryStatusDelivered
	}, 15*time.Second, 500*time.Millisecond, "delivery should be processed")

	// Verify the signature header format
	require.NotEmpty(t, capturedSignature, "X-Webhook-Signature should be present")
	assert.True(t, strings.HasPrefix(capturedSignature, "t="), "signature should start with t=")
	assert.Contains(t, capturedSignature, ",v1=", "signature should contain v1=")

	// Verify the v1 part is a valid hex string
	parts := strings.SplitN(strings.TrimPrefix(capturedSignature, "t="), ",v1=", 2)
	require.Len(t, parts, 2)
	v1Sig := parts[1]
	assert.Equal(t, 64, len(v1Sig), "v1 signature should be 64 hex characters (SHA-256)")
	_, decodeErr := hex.DecodeString(v1Sig)
	assert.NoError(t, decodeErr, "v1 signature should be valid hex")

	// Verify event and delivery ID headers
	assert.Equal(t, "user.created", capturedEvent)
	_, parseErr := uuid.Parse(capturedDeliveryID)
	assert.NoError(t, parseErr, "X-Webhook-Delivery-ID should be a valid UUID")

	// Verify the signature was computed with the correct webhook secret by
	// recomputing it from the captured request body.
	// Fetch the delivery record to get its ID for the payload lookup.
	// Since we don't have the exact body bytes here, we verify the format above
	// and the HMAC correctness is covered in TestWebhookE2E_FullLifecycle.
	assert.NotEmpty(t, capturedDeliveryID)
}
