package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// WebhookService handles webhook-related business logic.
type WebhookService struct {
	webhookRepo   repository.WebhookRepository
	deliveryRepo  repository.WebhookDeliveryRepository
	config        *config.WebhookConfig
	logger        logger.Logger
	queue         WebhookQueue
	rateLimiter   WebhookRateLimiterInterface
}

// NewWebhookService creates a new WebhookService instance.
func NewWebhookService(
	webhookRepo repository.WebhookRepository,
	deliveryRepo repository.WebhookDeliveryRepository,
	config *config.WebhookConfig,
	logger logger.Logger,
) *WebhookService {
	return &WebhookService{
		webhookRepo:  webhookRepo,
		deliveryRepo: deliveryRepo,
		config:       config,
		logger:       logger,
	}
}

// SetQueue sets the webhook delivery queue. This is called after construction
// when Redis is available.
func (s *WebhookService) SetQueue(queue WebhookQueue) {
	s.queue = queue
}

// SetRateLimiter sets the webhook rate limiter. This is called after construction
// when Redis is available.
func (s *WebhookService) SetRateLimiter(limiter WebhookRateLimiterInterface) {
	s.rateLimiter = limiter
}

// Create creates a new webhook.
func (s *WebhookService) Create(
	ctx context.Context,
	orgID *uuid.UUID,
	req *request.CreateWebhookRequest,
) (*domain.Webhook, error) {
	// Validate name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "webhook name is required", 422)
	}
	if len(name) > 255 {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "webhook name must be at most 255 characters", 422)
	}

	// Validate URL
	if err := validateURL(req.URL, s.config.AllowHTTP); err != nil {
		return nil, err
	}

	// Validate events
	if err := validateEvents(req.Events); err != nil {
		return nil, err
	}

	// Generate secret
	secret, err := generateSecret()
	if err != nil {
		s.logger.Error(ctx, "failed to generate webhook secret")
		return nil, apperrors.WrapInternal(err)
	}

	// Determine rate limit
	rateLimit := s.config.RateLimit
	if req.RateLimit != nil && *req.RateLimit > 0 {
		rateLimit = *req.RateLimit
	}

	// Marshal events to JSON
	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Build entity
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	webhook := &domain.Webhook{
		OrganizationID: orgID,
		Name:           name,
		URL:            req.URL,
		Secret:         secret,
		Events:         eventsJSON,
		Active:         active,
		RateLimit:      rateLimit,
	}

	if err := s.webhookRepo.Create(ctx, webhook); err != nil {
		s.logger.Error(ctx, "failed to create webhook",
			s.logger.String("url", req.URL),
			s.logger.String("name", name),
		)
		return nil, apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "webhook created",
		s.logger.String("webhook_id", webhook.ID.String()),
		s.logger.String("name", webhook.Name),
	)

	return webhook, nil
}

// GetByID retrieves a webhook by its ID.
func (s *WebhookService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	webhook, err := s.webhookRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

// ListByOrg lists webhooks scoped to an organization (or global if orgID is nil).
func (s *WebhookService) ListByOrg(
	ctx context.Context,
	orgID *uuid.UUID,
	limit, offset int,
) ([]*domain.Webhook, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	webhooks, total, err := s.webhookRepo.FindByOrgID(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return webhooks, total, nil
}

// Update updates a webhook's fields.
func (s *WebhookService) Update(
	ctx context.Context,
	id uuid.UUID,
	req *request.UpdateWebhookRequest,
) (*domain.Webhook, error) {
	webhook, err := s.webhookRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	urlChanged := false

	// Name
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "webhook name cannot be empty", 422)
		}
		if len(name) > 255 {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "webhook name must be at most 255 characters", 422)
		}
		webhook.Name = name
	}

	// URL
	if req.URL != nil {
		if err := validateURL(*req.URL, s.config.AllowHTTP); err != nil {
			return nil, err
		}
		if *req.URL != webhook.URL {
			urlChanged = true
		}
		webhook.URL = *req.URL
	}

	// Events
	if len(req.Events) > 0 {
		if err := validateEvents(req.Events); err != nil {
			return nil, err
		}
		eventsJSON, err := json.Marshal(req.Events)
		if err != nil {
			return nil, apperrors.WrapInternal(err)
		}
		webhook.Events = eventsJSON
	}

	// Active
	if req.Active != nil {
		webhook.Active = *req.Active
	}

	// RateLimit
	if req.RateLimit != nil {
		webhook.RateLimit = *req.RateLimit
	}

	// Regenerate secret if URL changed
	if urlChanged {
		secret, err := generateSecret()
		if err != nil {
			s.logger.Error(ctx, "failed to regenerate webhook secret")
			return nil, apperrors.WrapInternal(err)
		}
		webhook.Secret = secret
	}

	if err := s.webhookRepo.Update(ctx, webhook); err != nil {
		s.logger.Error(ctx, "failed to update webhook",
			s.logger.String("webhook_id", id.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "webhook updated",
		s.logger.String("webhook_id", webhook.ID.String()),
	)

	return webhook, nil
}

// SoftDelete soft-deletes a webhook.
func (s *WebhookService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if err := s.webhookRepo.SoftDelete(ctx, id); err != nil {
		s.logger.Error(ctx, "failed to soft delete webhook",
			s.logger.String("webhook_id", id.String()),
		)
		return err
	}

	s.logger.Info(ctx, "webhook soft deleted",
		s.logger.String("webhook_id", id.String()),
	)

	return nil
}

// ListDeliveries retrieves deliveries for a webhook with optional filters.
func (s *WebhookService) ListDeliveries(
	ctx context.Context,
	webhookID uuid.UUID,
	query repository.ListDeliveriesQuery,
) ([]*domain.WebhookDelivery, int64, error) {
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	deliveries, total, err := s.deliveryRepo.FindByWebhookID(ctx, webhookID, query)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return deliveries, total, nil
}

// GetDelivery retrieves a single delivery by ID.
func (s *WebhookService) GetDelivery(
	ctx context.Context,
	deliveryID uuid.UUID,
) (*domain.WebhookDelivery, error) {
	delivery, err := s.deliveryRepo.FindByID(ctx, deliveryID)
	if err != nil {
		return nil, err
	}
	return delivery, nil
}

// ReplayDelivery replays a previously completed delivery.
// Returns an error if the delivery is in queued or processing status.
func (s *WebhookService) ReplayDelivery(
	ctx context.Context,
	deliveryID uuid.UUID,
) (*domain.WebhookDelivery, error) {
	delivery, err := s.deliveryRepo.FindByID(ctx, deliveryID)
	if err != nil {
		return nil, err
	}

	if !delivery.CanReplay() {
		return nil, apperrors.NewAppError("CONFLICT",
			"delivery cannot be replayed while in queued or processing status", 409)
	}

	// Reset delivery to queued state for replay
	delivery.Replay()

	updates := map[string]interface{}{
		"status":               domain.WebhookDeliveryStatusQueued,
		"attempt_number":       1,
		"response_code":        nil,
		"response_body":       "",
		"duration_ms":         nil,
		"last_error":          "",
		"next_retry_at":       nil,
		"processing_started_at": nil,
		"delivered_at":        nil,
	}

	if err := s.deliveryRepo.UpdateStatus(ctx, deliveryID, domain.WebhookDeliveryStatusQueued, updates); err != nil {
		s.logger.Error(ctx, "failed to replay delivery",
			s.logger.String("delivery_id", deliveryID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Re-fetch to get the updated delivery
	updated, err := s.deliveryRepo.FindByID(ctx, deliveryID)
	if err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "delivery replayed",
		s.logger.String("delivery_id", deliveryID.String()),
		s.logger.String("webhook_id", delivery.WebhookID.String()),
	)

	return updated, nil
}

// DispatchEvent finds active webhooks subscribed to the given event, creates
// delivery records, and enqueues them for processing. The orgID parameter
// scopes the event to a specific organization; pass nil for global events.
// When orgID is provided, both global and org-scoped webhooks are matched.
func (s *WebhookService) DispatchEvent(ctx context.Context, eventType string, payload interface{}, orgID *uuid.UUID) error {
	// Find webhooks eligible for this event (global + org-scoped if orgID provided)
	webhooks, err := s.webhookRepo.FindForEventDispatch(ctx, orgID)
	if err != nil {
		s.logger.Error(ctx, "failed to find webhooks for event dispatch",
			s.logger.String("event", eventType),
			s.logger.Any("org_id", orgID),
			logger.Err(err),
		)
		return apperrors.WrapInternal(err)
	}

	var lastErr error
	dispatched := 0

	for _, wh := range webhooks {
		// Skip inactive webhooks
		if !wh.IsActive() {
			continue
		}

		// Skip webhooks not subscribed to this event
		if !wh.IsSubscribedTo(eventType) {
			continue
		}

		// Check org scope: if webhook is org-scoped, only match if orgIDs match
		if wh.OrganizationID != nil {
			if orgID == nil || *wh.OrganizationID != *orgID {
				continue
			}
		}

		// Check rate limit if limiter is available
		if s.rateLimiter != nil {
			allowed, _, retryAfter := s.rateLimiter.Allow(ctx, wh.ID.String())
			if !allowed {
				s.logger.Warn(ctx, "webhook rate limited, skipping dispatch",
					s.logger.String("webhook_id", wh.ID.String()),
					s.logger.String("event", eventType),
					s.logger.String("retry_after", retryAfter.String()),
				)
				// Create a rate_limited delivery record
				now := time.Now()
				payloadJSON, _ := json.Marshal(payload)
				delivery := &domain.WebhookDelivery{
					WebhookID:     wh.ID,
					Event:         eventType,
					Payload:       payloadJSON,
					Status:        domain.WebhookDeliveryStatusRateLimited,
					AttemptNumber: 1,
					MaxAttempts:   3,
					LastError:     fmt.Sprintf("rate limited, retry after %s", retryAfter),
					CreatedAt:     now,
				}
				if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
					s.logger.Error(ctx, "failed to create rate-limited delivery",
						s.logger.String("webhook_id", wh.ID.String()),
						logger.Err(err),
					)
				}
				continue
			}
		}

		// Marshal payload
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			s.logger.Error(ctx, "failed to marshal webhook event payload",
				s.logger.String("webhook_id", wh.ID.String()),
				s.logger.String("event", eventType),
				logger.Err(err),
			)
			lastErr = err
			continue
		}

		// Truncate payload if it exceeds max size
		maxPayloadSize := 1048576 // 1MB default
		if s.config.MaxPayloadSize > 0 {
			maxPayloadSize = s.config.MaxPayloadSize
		}
		if len(payloadJSON) > maxPayloadSize {
			s.logger.Warn(ctx, "webhook payload exceeds max size, truncating",
				s.logger.String("webhook_id", wh.ID.String()),
				s.logger.Int("payload_size", len(payloadJSON)),
				s.logger.Int("max_size", maxPayloadSize),
			)
			payloadJSON = payloadJSON[:maxPayloadSize]
		}

		// Create delivery record
		maxAttempts := 3
		if s.config.RetryMax > 0 {
			maxAttempts = s.config.RetryMax
		}

		delivery := &domain.WebhookDelivery{
			WebhookID:     wh.ID,
			Event:         eventType,
			Payload:       payloadJSON,
			Status:        domain.WebhookDeliveryStatusQueued,
			AttemptNumber: 1,
			MaxAttempts:   maxAttempts,
			CreatedAt:     time.Now(),
		}

		if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
			s.logger.Error(ctx, "failed to create webhook delivery",
				s.logger.String("webhook_id", wh.ID.String()),
				s.logger.String("event", eventType),
				logger.Err(err),
			)
			lastErr = err
			continue
		}

		// Enqueue for processing if queue is available
		if s.queue != nil {
			if err := s.queue.Enqueue(ctx, delivery.ID.String()); err != nil {
				s.logger.Error(ctx, "failed to enqueue webhook delivery",
					s.logger.String("delivery_id", delivery.ID.String()),
					s.logger.String("webhook_id", wh.ID.String()),
					logger.Err(err),
				)
				lastErr = err
				continue
			}
		}

		dispatched++
	}

	s.logger.Info(ctx, "webhook event dispatched",
		s.logger.String("event", eventType),
		s.logger.Int("webhooks_matched", dispatched),
	)

	return lastErr
}

// SubscribeToEventBus registers the service as a handler on the given EventBus.
// When events are published, DispatchEvent is called automatically.
// The event's OrgID is forwarded to DispatchEvent so org-scoped webhooks are matched.
func (s *WebhookService) SubscribeToEventBus(bus *domain.EventBus) {
	bus.Subscribe(func(event domain.WebhookEvent) {
		ctx := context.Background()
		if err := s.DispatchEvent(ctx, event.Type, event.Payload, event.OrgID); err != nil {
			s.logger.Error(ctx, "failed to dispatch webhook event from bus",
				s.logger.String("event", event.Type),
				s.logger.Any("org_id", event.OrgID),
				logger.Err(err),
			)
		}
	})
}

// validateURL validates a webhook URL and blocks private/internal IPs (SSRF protection).
func validateURL(rawURL string, allowHTTP bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return apperrors.NewAppError("VALIDATION_ERROR", "invalid URL", 422)
	}

	switch u.Scheme {
	case "https":
		// always allowed
	case "http":
		if !allowHTTP {
			return apperrors.NewAppError("VALIDATION_ERROR", "URL must use HTTPS", 422)
		}
	default:
		return apperrors.NewAppError("VALIDATION_ERROR", "URL scheme must be http or https", 422)
	}

	host := u.Hostname()
	if host == "" {
		return apperrors.NewAppError("VALIDATION_ERROR", "URL must have a valid host", 422)
	}

	// Resolve host to IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		// If DNS lookup fails, try parsing as a literal IP
		ip := net.ParseIP(host)
		if ip == nil {
			return apperrors.NewAppError("VALIDATION_ERROR", "unable to resolve URL host", 422)
		}
		ips = []net.IP{ip}
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return apperrors.NewAppError("VALIDATION_ERROR", "URL resolves to a private or internal IP address", 422)
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is in a private/link-local/loopback range.
func isPrivateIP(ip net.IP) bool {
	// Loopback: 127.0.0.0/8, ::1
	if ip.IsLoopback() {
		return true
	}

	// Private (RFC1918): 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
	if ip.IsPrivate() {
		return true
	}

	// Link-local: 169.254.0.0/16, fe80::/10
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	return false
}

// validateEvents validates that all provided events are supported.
func validateEvents(events []string) error {
	if len(events) == 0 {
		return apperrors.NewAppError("VALIDATION_ERROR", "at least one event is required", 422)
	}

	var invalid []string
	for _, event := range events {
		if !domain.ValidWebhookEvents[event] {
			invalid = append(invalid, event)
		}
	}

	if len(invalid) > 0 {
		return apperrors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("invalid webhook events: %s", strings.Join(invalid, ", ")),
			422)
	}

	return nil
}

// generateSecret generates a cryptographically secure webhook secret.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(b), nil
}
