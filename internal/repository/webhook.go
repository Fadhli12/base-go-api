package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookRepository defines the interface for webhook data access operations.
// Webhook supports soft delete via gorm.DeletedAt.
type WebhookRepository interface {
	Create(ctx context.Context, webhook *domain.Webhook) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	FindByOrgID(ctx context.Context, orgID *uuid.UUID, limit, offset int) ([]*domain.Webhook, int64, error)
	FindForEventDispatch(ctx context.Context, orgID *uuid.UUID) ([]*domain.Webhook, error)
	Update(ctx context.Context, webhook *domain.Webhook) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	FindActiveByEvent(ctx context.Context, event string) ([]*domain.Webhook, error)
}

// webhookRepository implements WebhookRepository interface.
type webhookRepository struct {
	db *gorm.DB
}

// NewWebhookRepository creates a new WebhookRepository instance.
func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &webhookRepository{db: db}
}

// Create inserts a new webhook into the database.
func (r *webhookRepository) Create(ctx context.Context, webhook *domain.Webhook) error {
	if err := r.db.WithContext(ctx).Create(webhook).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a webhook by its UUID.
// Automatically filters out soft-deleted records.
func (r *webhookRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	var webhook domain.Webhook
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&webhook).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &webhook, nil
}

// FindByOrgID retrieves webhooks by organization ID with pagination.
// If orgID is nil, returns global webhooks (organization_id IS NULL).
func (r *webhookRepository) FindByOrgID(ctx context.Context, orgID *uuid.UUID, limit, offset int) ([]*domain.Webhook, int64, error) {
	var webhooks []*domain.Webhook
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Webhook{})
	if orgID == nil {
		db = db.Where("organization_id IS NULL")
	} else {
		db = db.Where("organization_id = ?", orgID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&webhooks).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return webhooks, total, nil
}

// FindForEventDispatch retrieves webhooks eligible for event dispatch.
// If orgID is nil, returns only global webhooks (organization_id IS NULL).
// If orgID is provided, returns both global webhooks AND org-scoped webhooks
// matching the given organization ID. This ensures global webhooks always
// receive all events regardless of org context.
func (r *webhookRepository) FindForEventDispatch(ctx context.Context, orgID *uuid.UUID) ([]*domain.Webhook, error) {
	var webhooks []*domain.Webhook

	db := r.db.WithContext(ctx).Model(&domain.Webhook{})
	if orgID == nil {
		db = db.Where("organization_id IS NULL")
	} else {
		db = db.Where("organization_id IS NULL OR organization_id = ?", orgID)
	}

	if err := db.Find(&webhooks).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}

	return webhooks, nil
}

// Update updates an existing webhook in the database.
func (r *webhookRepository) Update(ctx context.Context, webhook *domain.Webhook) error {
	result := r.db.WithContext(ctx).Save(webhook)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// SoftDelete performs a soft delete on a webhook by its ID.
func (r *webhookRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.Webhook{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// FindActiveByEvent finds active webhooks subscribed to the given event.
// Uses PostgreSQL JSONB containment operator.
func (r *webhookRepository) FindActiveByEvent(ctx context.Context, event string) ([]*domain.Webhook, error) {
	var webhooks []*domain.Webhook
	err := r.db.WithContext(ctx).
		Where("active = ? AND deleted_at IS NULL AND events::jsonb @> ?::jsonb", true, fmt.Sprintf(`["%s"]`, event)).
		Find(&webhooks).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return webhooks, nil
}

// WebhookDeliveryRepository defines the interface for webhook delivery data access operations.
// WebhookDelivery does NOT support soft delete - records are retained permanently for audit trail.
type WebhookDeliveryRepository interface {
	Create(ctx context.Context, delivery *domain.WebhookDelivery) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.WebhookDelivery, error)
	FindByWebhookID(ctx context.Context, webhookID uuid.UUID, query ListDeliveriesQuery) ([]*domain.WebhookDelivery, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WebhookDeliveryStatus, updates map[string]interface{}) error
	FindStuckProcessing(ctx context.Context, timeout time.Duration) ([]*domain.WebhookDelivery, error)
	DeleteOlderThan(ctx context.Context, retentionDays int) error
}

// ListDeliveriesQuery defines the query parameters for listing webhook deliveries.
type ListDeliveriesQuery struct {
	Limit  int
	Offset int
	Status *domain.WebhookDeliveryStatus // nil = no filter
	Event  string                          // empty = no filter
}

// webhookDeliveryRepository implements WebhookDeliveryRepository interface.
type webhookDeliveryRepository struct {
	db *gorm.DB
}

// NewWebhookDeliveryRepository creates a new WebhookDeliveryRepository instance.
func NewWebhookDeliveryRepository(db *gorm.DB) WebhookDeliveryRepository {
	return &webhookDeliveryRepository{db: db}
}

// Create inserts a new webhook delivery into the database.
func (r *webhookDeliveryRepository) Create(ctx context.Context, delivery *domain.WebhookDelivery) error {
	if err := r.db.WithContext(ctx).Create(delivery).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a webhook delivery by its UUID.
func (r *webhookDeliveryRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.WebhookDelivery, error) {
	var delivery domain.WebhookDelivery
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&delivery).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &delivery, nil
}

// FindByWebhookID retrieves webhook deliveries for a given webhook ID with pagination and optional filters.
func (r *webhookDeliveryRepository) FindByWebhookID(ctx context.Context, webhookID uuid.UUID, query ListDeliveriesQuery) ([]*domain.WebhookDelivery, int64, error) {
	var deliveries []*domain.WebhookDelivery
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.WebhookDelivery{}).Where("webhook_id = ?", webhookID)
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.Event != "" {
		db = db.Where("event = ?", query.Event)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := db.Order("created_at DESC").Limit(query.Limit).Offset(query.Offset).Find(&deliveries).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return deliveries, total, nil
}

// UpdateStatus updates a webhook delivery's status and additional fields in a single call.
func (r *webhookDeliveryRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WebhookDeliveryStatus, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["status"] = status

	result := r.db.WithContext(ctx).
		Model(&domain.WebhookDelivery{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// FindStuckProcessing finds deliveries stuck in processing state for longer than the given timeout.
func (r *webhookDeliveryRepository) FindStuckProcessing(ctx context.Context, timeout time.Duration) ([]*domain.WebhookDelivery, error) {
	var deliveries []*domain.WebhookDelivery
	cutoff := time.Now().Add(-timeout)
	err := r.db.WithContext(ctx).
		Where("status = ? AND processing_started_at < ?", domain.WebhookDeliveryStatusProcessing, cutoff).
		Find(&deliveries).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return deliveries, nil
}

// DeleteOlderThan hard deletes webhook deliveries older than the specified retention period in days.
func (r *webhookDeliveryRepository) DeleteOlderThan(ctx context.Context, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	if err := r.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&domain.WebhookDelivery{}).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}
