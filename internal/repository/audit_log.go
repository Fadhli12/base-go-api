package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogRepository defines the interface for audit log data access operations.
type AuditLogRepository interface {
	// Create inserts a new audit log entry into the database
	Create(ctx context.Context, auditLog *domain.AuditLog) error
	// FindByActorID retrieves audit logs for a specific actor with pagination
	FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error)
	// FindByResource retrieves audit logs for a specific resource with pagination
	FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error)
	// FindAll retrieves all audit logs with pagination (for admin purposes)
	FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error)
}

// auditLogRepository implements AuditLogRepository interface
type auditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository creates a new AuditLogRepository instance
func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepository{
		db: db,
	}
}

// Create inserts a new audit log entry into the database
func (r *auditLogRepository) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(auditLog).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindByActorID retrieves audit logs for a specific actor with pagination
func (r *auditLogRepository) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	var auditLogs []domain.AuditLog
	query := r.db.WithContext(ctx).
		Where("actor_id = ?", actorID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&auditLogs).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return auditLogs, nil
}

// FindByResource retrieves audit logs for a specific resource with pagination
func (r *auditLogRepository) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	var auditLogs []domain.AuditLog
	query := r.db.WithContext(ctx).
		Where("resource = ?", resource)

	if resourceID != "" {
		query = query.Where("resource_id = ?", resourceID)
	}

	query = query.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&auditLogs).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return auditLogs, nil
}

// FindAll retrieves all audit logs with pagination
func (r *auditLogRepository) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	var auditLogs []domain.AuditLog
	query := r.db.WithContext(ctx).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&auditLogs).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return auditLogs, nil
}