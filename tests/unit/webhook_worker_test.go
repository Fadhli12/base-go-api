package unit

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────────
// Mock Implementations
// ──────────────────────────────────────────────────────────────────

type mockWebhookQueue struct {
	dequeueCalled           atomic.Int64
	dequeueFunc             func(ctx context.Context) (*service.WebhookQueueMessage, error)
	enqueueFunc             func(ctx context.Context, deliveryID string) error
	enqueueRetryFunc        func(ctx context.Context, deliveryID string, retryAt time.Time) error
	removeFromProcessingFunc func(ctx context.Context, deliveryID string) error
	mu                      sync.Mutex
	enqueueRetryCalls       []enqueueRetryCall
	removeProcessingCalls   []string
}

type enqueueRetryCall struct {
	DeliveryID string
	RetryAt    time.Time
}

func (m *mockWebhookQueue) Enqueue(ctx context.Context, deliveryID string) error {
	if m.enqueueFunc != nil {
		return m.enqueueFunc(ctx, deliveryID)
	}
	return nil
}

func (m *mockWebhookQueue) Dequeue(ctx context.Context) (*service.WebhookQueueMessage, error) {
	m.dequeueCalled.Add(1)
	if m.dequeueFunc != nil {
		return m.dequeueFunc(ctx)
	}
	return nil, nil
}

func (m *mockWebhookQueue) EnqueueRetry(ctx context.Context, deliveryID string, retryAt time.Time) error {
	m.mu.Lock()
	m.enqueueRetryCalls = append(m.enqueueRetryCalls, enqueueRetryCall{DeliveryID: deliveryID, RetryAt: retryAt})
	m.mu.Unlock()
	if m.enqueueRetryFunc != nil {
		return m.enqueueRetryFunc(ctx, deliveryID, retryAt)
	}
	return nil
}

func (m *mockWebhookQueue) RemoveFromProcessing(ctx context.Context, deliveryID string) error {
	m.mu.Lock()
	m.removeProcessingCalls = append(m.removeProcessingCalls, deliveryID)
	m.mu.Unlock()
	if m.removeFromProcessingFunc != nil {
		return m.removeFromProcessingFunc(ctx, deliveryID)
	}
	return nil
}

func (m *mockWebhookQueue) GetNextBatch(ctx context.Context, limit int) ([]*service.WebhookQueueMessage, error) {
	return nil, nil
}

func (m *mockWebhookQueue) QueueSize(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockWebhookQueue) ProcessingSize(ctx context.Context) (int64, error) {
	return 0, nil
}

type mockWebhookDeliveryRepository struct {
	findByIDFunc            func(ctx context.Context, id uuid.UUID) (*domain.WebhookDelivery, error)
	updateStatusFunc        func(ctx context.Context, id uuid.UUID, status domain.WebhookDeliveryStatus, updates map[string]interface{}) error
	findStuckProcessingFunc func(ctx context.Context, timeout time.Duration) ([]*domain.WebhookDelivery, error)
	deleteOlderThanFunc     func(ctx context.Context, retentionDays int) error
	mu                      sync.Mutex
	updateStatusCalls       []updateStatusCall
}

type updateStatusCall struct {
	ID      uuid.UUID
	Status  domain.WebhookDeliveryStatus
	Updates map[string]interface{}
}

func (m *mockWebhookDeliveryRepository) Create(ctx context.Context, delivery *domain.WebhookDelivery) error {
	return nil
}

func (m *mockWebhookDeliveryRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.WebhookDelivery, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockWebhookDeliveryRepository) FindByWebhookID(ctx context.Context, webhookID uuid.UUID, query repository.ListDeliveriesQuery) ([]*domain.WebhookDelivery, int64, error) {
	return nil, 0, nil
}

func (m *mockWebhookDeliveryRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WebhookDeliveryStatus, updates map[string]interface{}) error {
	m.mu.Lock()
	m.updateStatusCalls = append(m.updateStatusCalls, updateStatusCall{ID: id, Status: status, Updates: updates})
	m.mu.Unlock()
	if m.updateStatusFunc != nil {
		return m.updateStatusFunc(ctx, id, status, updates)
	}
	return nil
}

func (m *mockWebhookDeliveryRepository) FindStuckProcessing(ctx context.Context, timeout time.Duration) ([]*domain.WebhookDelivery, error) {
	if m.findStuckProcessingFunc != nil {
		return m.findStuckProcessingFunc(ctx, timeout)
	}
	return nil, nil
}

func (m *mockWebhookDeliveryRepository) DeleteOlderThan(ctx context.Context, retentionDays int) error {
	if m.deleteOlderThanFunc != nil {
		return m.deleteOlderThanFunc(ctx, retentionDays)
	}
	return nil
}

type mockWebhookRepository struct {
	findByIDFunc func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
}

func (m *mockWebhookRepository) Create(ctx context.Context, webhook *domain.Webhook) error            { return nil }
func (m *mockWebhookRepository) Update(ctx context.Context, webhook *domain.Webhook) error            { return nil }
func (m *mockWebhookRepository) SoftDelete(ctx context.Context, id uuid.UUID) error                   { return nil }
func (m *mockWebhookRepository) FindByOrgID(ctx context.Context, orgID *uuid.UUID, limit, offset int) ([]*domain.Webhook, int64, error) {
	return nil, 0, nil
}
func (m *mockWebhookRepository) FindForEventDispatch(ctx context.Context, orgID *uuid.UUID) ([]*domain.Webhook, error) {
	return nil, nil
}
func (m *mockWebhookRepository) FindActiveByEvent(ctx context.Context, event string) ([]*domain.Webhook, error) {
	return nil, nil
}

func (m *mockWebhookRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, fmt.Errorf("not found")
}

// ──────────────────────────────────────────────────────────────────
// Test Helpers
// ──────────────────────────────────────────────────────────────────

func defaultTestConfig() *config.WebhookConfig {
	cfg := config.DefaultWebhookConfig()
	cfg.AllowHTTP = true
	cfg.DeliveryTimeout = 5 * time.Second
	cfg.MaxPayloadSize = 1048576
	return &cfg
}

// ──────────────────────────────────────────────────────────────────
// Tests: Constructor
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_NewWebhookWorker(t *testing.T) {
	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, &mockWebhookDeliveryRepository{}, &mockWebhookRepository{}, &mockWebhookQueue{}, newMockWebhookRateLimiter(), nil)

	assert.NotNil(t, worker)
	m := worker.Metrics()
	assert.Zero(t, m["delivered"])
	assert.Zero(t, m["failed"])
	assert.Zero(t, m["rate_limited"])
}

// ──────────────────────────────────────────────────────────────────
// Tests: Metrics
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_Metrics(t *testing.T) {
	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, &mockWebhookDeliveryRepository{}, &mockWebhookRepository{}, &mockWebhookQueue{}, newMockWebhookRateLimiter(), nil)

	m := worker.Metrics()
	assert.Len(t, m, 3)
	assert.Equal(t, int64(0), m["delivered"])
	assert.Equal(t, int64(0), m["failed"])
	assert.Equal(t, int64(0), m["rate_limited"])

	// Mutating the returned map should not affect the worker
	m["delivered"] = 999
	m2 := worker.Metrics()
	assert.Equal(t, int64(0), m2["delivered"])
}

// ──────────────────────────────────────────────────────────────────
// Tests: Graceful Shutdown
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_GracefulShutdown(t *testing.T) {
	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, &mockWebhookDeliveryRepository{}, &mockWebhookRepository{}, &mockWebhookQueue{}, newMockWebhookRateLimiter(), nil)

	worker.Start()
	time.Sleep(100 * time.Millisecond)
	worker.Stop()
}

func TestWebhookWorker_StartStopMultiple(t *testing.T) {
	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, &mockWebhookDeliveryRepository{}, &mockWebhookRepository{}, &mockWebhookQueue{}, newMockWebhookRateLimiter(), nil)

	worker.Start()
	time.Sleep(50 * time.Millisecond)
	worker.Stop()

	worker.Start()
	time.Sleep(50 * time.Millisecond)
	worker.Stop()
}

// ──────────────────────────────────────────────────────────────────
// Tests: Empty Queue
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_EmptyQueue(t *testing.T) {
	queue := &mockWebhookQueue{
		dequeueFunc: func(ctx context.Context) (*service.WebhookQueueMessage, error) {
			return nil, nil
		},
	}

	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, &mockWebhookDeliveryRepository{}, &mockWebhookRepository{}, queue, newMockWebhookRateLimiter(), nil)
	worker.Start()
	time.Sleep(1100 * time.Millisecond)
	worker.Stop()

	assert.Greater(t, queue.dequeueCalled.Load(), int64(0), "dequeue should be called at least once on the 1-second ticker")
}

// ──────────────────────────────────────────────────────────────────
// Tests: Invalid Delivery ID in Queue
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_InvalidDeliveryID(t *testing.T) {
	var dequeueCount atomic.Int32
	queue := &mockWebhookQueue{
		dequeueFunc: func(ctx context.Context) (*service.WebhookQueueMessage, error) {
			if dequeueCount.Add(1) == 1 {
				return &service.WebhookQueueMessage{
					DeliveryID: "not-a-valid-uuid",
					QueuedAt:   time.Now(),
				}, nil
			}
			return nil, nil
		},
	}

	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, &mockWebhookDeliveryRepository{}, &mockWebhookRepository{}, queue, newMockWebhookRateLimiter(), nil)
	worker.Start()
	time.Sleep(1500 * time.Millisecond)
	worker.Stop()

	assert.Contains(t, queue.removeProcessingCalls, "not-a-valid-uuid",
		"RemoveFromProcessing should be called even for invalid UUID")
}

// ──────────────────────────────────────────────────────────────────
// Tests: Delivery Not Found
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_Delivery_DeliveryNotFound(t *testing.T) {
	deliveryID := uuid.New()
	deliveryRepo := &mockWebhookDeliveryRepository{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.WebhookDelivery, error) {
			return nil, fmt.Errorf("delivery not found in database")
		},
	}

	var dequeueCount atomic.Int32
	queue := &mockWebhookQueue{
		dequeueFunc: func(ctx context.Context) (*service.WebhookQueueMessage, error) {
			if dequeueCount.Add(1) == 1 {
				return &service.WebhookQueueMessage{
					DeliveryID: deliveryID.String(),
					QueuedAt:   time.Now(),
				}, nil
			}
			return nil, nil
		},
	}

	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, deliveryRepo, &mockWebhookRepository{}, queue, newMockWebhookRateLimiter(), nil)
	worker.Start()
	time.Sleep(1500 * time.Millisecond)
	worker.Stop()

	assert.Contains(t, queue.removeProcessingCalls, deliveryID.String(),
		"RemoveFromProcessing should be called even when delivery is not found")
}

// ──────────────────────────────────────────────────────────────────
// Tests: Webhook Not Found
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_Delivery_WebhookNotFound(t *testing.T) {
	webhookID := uuid.New()
	deliveryID := uuid.New()

	deliveryRepo := &mockWebhookDeliveryRepository{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.WebhookDelivery, error) {
			return &domain.WebhookDelivery{
				ID:            deliveryID,
				WebhookID:     webhookID,
				Event:         "user.created",
				Payload:       []byte(`{"test":"not-found"}`),
				Status:        domain.WebhookDeliveryStatusQueued,
				AttemptNumber: 1,
				MaxAttempts:   3,
				CreatedAt:     time.Now(),
			}, nil
		},
	}

	webhookRepo := &mockWebhookRepository{
		findByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return nil, fmt.Errorf("webhook not found")
		},
	}

	var dequeueCount atomic.Int32
	queue := &mockWebhookQueue{
		dequeueFunc: func(ctx context.Context) (*service.WebhookQueueMessage, error) {
			if dequeueCount.Add(1) == 1 {
				return &service.WebhookQueueMessage{
					DeliveryID: deliveryID.String(),
					QueuedAt:   time.Now(),
				}, nil
			}
			return nil, nil
		},
	}

	cfg := defaultTestConfig()
	worker := service.NewWebhookWorker(cfg, deliveryRepo, webhookRepo, queue, newMockWebhookRateLimiter(), nil)
	worker.Start()
	time.Sleep(1500 * time.Millisecond)
	worker.Stop()

	m := worker.Metrics()
	assert.Equal(t, int64(1), m["failed"], "failed count should increment when webhook is not found")
}

// ──────────────────────────────────────────────────────────────────
// Tests: Concurrent Workers
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_ConcurrentWorkers(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.WorkerConcurrency = 3

	var dequeueTotal atomic.Int64
	queue := &mockWebhookQueue{
		dequeueFunc: func(ctx context.Context) (*service.WebhookQueueMessage, error) {
			dequeueTotal.Add(1)
			return nil, nil
		},
	}

	worker := service.NewWebhookWorker(cfg, &mockWebhookDeliveryRepository{}, &mockWebhookRepository{}, queue, newMockWebhookRateLimiter(), nil)
	worker.Start()
	time.Sleep(2500 * time.Millisecond)
	worker.Stop()

	assert.Greater(t, dequeueTotal.Load(), int64(3),
		"with 3 workers and ~2 tick cycles, expect > 3 dequeues")
}

// ──────────────────────────────────────────────────────────────────
// Tests: SSRF Protection (private IP detection via net package)
// ──────────────────────────────────────────────────────────────────

func TestWebhookWorker_PrivateIPDetection(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.1.1",
		"::1",
		"fe80::1",
	}

	for _, ipStr := range privateIPs {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip, "failed to parse %s", ipStr)

		isPrivate := ip.IsLoopback() || ip.IsPrivate() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
		assert.True(t, isPrivate, "%s should be classified as private", ipStr)
	}

	publicIPs := []string{"8.8.8.8", "1.1.1.1"}
	for _, ipStr := range publicIPs {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip, "failed to parse %s", ipStr)

		isPrivate := ip.IsLoopback() || ip.IsPrivate() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
		assert.False(t, isPrivate, "%s should NOT be classified as private", ipStr)
	}
}

func TestWebhookWorker_PrivateIPRanges(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 192.168.x", "192.168.1.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"private 172.31.x", "172.31.255.255", true},
		{"link-local 169.254.x", "169.254.1.1", true},
		{"link-local IPv6 fe80", "fe80::1", true},
		{"public 8.8.8.8", "8.8.8.8", false},
		{"public 1.1.1.1", "1.1.1.1", false},
		{"public 93.184.216.34 (example.com)", "93.184.216.34", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			require.NotNil(t, ip)

			isPrivate := ip.IsLoopback() || ip.IsPrivate() ||
				ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
			assert.Equal(t, tc.expected, isPrivate)
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// Interface compliance checks (compile-time assertions)
// ──────────────────────────────────────────────────────────────────

var _ service.WebhookQueue = (*mockWebhookQueue)(nil)
var _ repository.WebhookDeliveryRepository = (*mockWebhookDeliveryRepository)(nil)
var _ repository.WebhookRepository = (*mockWebhookRepository)(nil)

// ──────────────────────────────────────────────────────────────────
// Tests: Domain helper methods
// ──────────────────────────────────────────────────────────────────

func TestWebhookDelivery_HelperMethods(t *testing.T) {
	t.Run("CanRetry when attempt < max and not delivered", func(t *testing.T) {
		d := &domain.WebhookDelivery{
			AttemptNumber: 1,
			MaxAttempts:   3,
			Status:        domain.WebhookDeliveryStatusQueued,
		}
		assert.True(t, d.CanRetry())
	})

	t.Run("CanRetry false when attempt equals max", func(t *testing.T) {
		d := &domain.WebhookDelivery{
			AttemptNumber: 3,
			MaxAttempts:   3,
			Status:        domain.WebhookDeliveryStatusFailed,
		}
		assert.False(t, d.CanRetry())
	})

	t.Run("CanRetry false when already delivered", func(t *testing.T) {
		d := &domain.WebhookDelivery{
			AttemptNumber: 1,
			MaxAttempts:   3,
			Status:        domain.WebhookDeliveryStatusDelivered,
		}
		assert.False(t, d.CanRetry())
	})

	t.Run("CanReplay true for delivered", func(t *testing.T) {
		d := &domain.WebhookDelivery{Status: domain.WebhookDeliveryStatusDelivered}
		assert.True(t, d.CanReplay())
	})

	t.Run("CanReplay false for queued", func(t *testing.T) {
		d := &domain.WebhookDelivery{Status: domain.WebhookDeliveryStatusQueued}
		assert.False(t, d.CanReplay())
	})

	t.Run("CanReplay false for processing", func(t *testing.T) {
		d := &domain.WebhookDelivery{Status: domain.WebhookDeliveryStatusProcessing}
		assert.False(t, d.CanReplay())
	})

	t.Run("IsTerminal for delivered", func(t *testing.T) {
		d := &domain.WebhookDelivery{Status: domain.WebhookDeliveryStatusDelivered}
		assert.True(t, d.IsTerminal())
	})

	t.Run("IsTerminal for failed with max attempts", func(t *testing.T) {
		d := &domain.WebhookDelivery{
			Status:        domain.WebhookDeliveryStatusFailed,
			AttemptNumber: 3,
			MaxAttempts:   3,
		}
		assert.True(t, d.IsTerminal())
	})

	t.Run("IsTerminal false for queued", func(t *testing.T) {
		d := &domain.WebhookDelivery{Status: domain.WebhookDeliveryStatusQueued}
		assert.False(t, d.IsTerminal())
	})
}

func TestWebhook_HelperMethods(t *testing.T) {
	t.Run("IsActive with Active=true and not deleted", func(t *testing.T) {
		w := &domain.Webhook{Active: domain.BoolPtr(true)}
		assert.True(t, w.IsActive())
	})

	t.Run("IsActive with Active=false", func(t *testing.T) {
		w := &domain.Webhook{Active: domain.BoolPtr(false)}
		assert.False(t, w.IsActive())
	})

	t.Run("IsGlobal with nil OrganizationID", func(t *testing.T) {
		w := &domain.Webhook{}
		assert.True(t, w.IsGlobal())
	})

	t.Run("IsGlobal with OrganizationID set", func(t *testing.T) {
		orgID := uuid.New()
		w := &domain.Webhook{OrganizationID: &orgID}
		assert.False(t, w.IsGlobal())
	})
}

func TestWebhookDelivery_StatusConstants(t *testing.T) {
	assert.Equal(t, domain.WebhookDeliveryStatus("queued"), domain.WebhookDeliveryStatusQueued)
	assert.Equal(t, domain.WebhookDeliveryStatus("processing"), domain.WebhookDeliveryStatusProcessing)
	assert.Equal(t, domain.WebhookDeliveryStatus("delivered"), domain.WebhookDeliveryStatusDelivered)
	assert.Equal(t, domain.WebhookDeliveryStatus("failed"), domain.WebhookDeliveryStatusFailed)
	assert.Equal(t, domain.WebhookDeliveryStatus("rate_limited"), domain.WebhookDeliveryStatusRateLimited)
}

func TestWebhookDelivery_Replay(t *testing.T) {
	deliveryID := uuid.New()
	d := &domain.WebhookDelivery{
		ID:                  deliveryID,
		Status:              domain.WebhookDeliveryStatusFailed,
		AttemptNumber:       3,
		ResponseCode:        intPtr(500),
		ResponseBody:        "error",
		LastError:           "timeout",
		NextRetryAt:         timePtr(time.Now().Add(5 * time.Minute)),
		ProcessingStartedAt: timePtr(time.Now()),
		DeliveredAt:         nil,
	}

	d.Replay()

	assert.Equal(t, domain.WebhookDeliveryStatusQueued, d.Status)
	assert.Equal(t, 1, d.AttemptNumber)
	assert.Nil(t, d.ResponseCode)
	assert.Empty(t, d.ResponseBody)
	assert.Empty(t, d.LastError)
	assert.Nil(t, d.NextRetryAt)
	assert.Nil(t, d.ProcessingStartedAt)
	assert.Nil(t, d.DeliveredAt)
}

func TestWebhook_ToResponse(t *testing.T) {
	id := uuid.New()
	w := &domain.Webhook{
		ID:     id,
		Name:   "response-test",
		URL:    "https://example.com/hook",
		Active: domain.BoolPtr(true),
		Events: []byte(`["user.created","invoice.paid"]`),
	}

	resp := w.ToResponse()
	assert.Equal(t, id.String(), resp.ID)
	assert.Equal(t, "response-test", resp.Name)
	assert.Equal(t, "https://example.com/hook", resp.URL)
	assert.True(t, resp.Active)
	assert.Len(t, resp.Events, 2)
}

func TestWebhook_ToCreateResponse(t *testing.T) {
	id := uuid.New()
	w := &domain.Webhook{
		ID:     id,
		Name:   "create-test",
		URL:    "https://example.com/hook",
		Secret: "whsec_abc123",
		Active: domain.BoolPtr(true),
		Events: []byte(`["user.created"]`),
	}

	resp := w.ToCreateResponse()
	assert.Equal(t, id.String(), resp.ID)
	assert.Equal(t, "whsec_abc123", resp.Secret)
	assert.Len(t, resp.Events, 1)
	assert.Equal(t, "user.created", resp.Events[0])
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

func intPtr(v int) *int       { return &v }
func timePtr(v time.Time) *time.Time { return &v }
