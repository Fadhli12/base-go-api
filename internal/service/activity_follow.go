package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// ActivityFollowService handles follow/unfollow business logic for activity resources.
type ActivityFollowService struct {
	followRepo repository.ActivityFollowRepository
	enforcer   *permission.Enforcer
	audit      *AuditService
	log        *slog.Logger
}

// NewActivityFollowService creates a new ActivityFollowService instance.
func NewActivityFollowService(
	followRepo repository.ActivityFollowRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	log *slog.Logger,
) *ActivityFollowService {
	return &ActivityFollowService{
		followRepo: followRepo,
		enforcer:   enforcer,
		audit:      audit,
		log:        log,
	}
}

// resolveFollowOrgDomain returns the Casbin domain string for RBAC enforcement.
func resolveFollowOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	if hasOrgID && orgID != uuid.Nil {
		return orgID.String()
	}
	return "default"
}

// Follow creates a follow relationship between a user and a resource.
func (s *ActivityFollowService) Follow(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	resourceType string,
	resourceID string,
	ipAddress string,
	userAgent string,
) (*domain.ActivityFollow, error) {
	orgDomain := resolveFollowOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "activity", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to follow activities", 403)
	}

	// Validate resource type
	if !domain.IsValidResourceType(resourceType) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid resource type: "+resourceType, 422)
	}

	// Check if already following
	exists, err := s.followRepo.ExistsByUserAndResource(ctx, userID, resourceType, resourceID)
	if err != nil {
		s.log.Error("failed to check follow existence",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if exists {
		return nil, apperrors.NewAppError("CONFLICT", "already following this resource", 409)
	}

	follow := &domain.ActivityFollow{
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}

	if err := s.followRepo.Create(ctx, follow); err != nil {
		s.log.Error("failed to create follow",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log
	afterJSON, _ := json.Marshal(follow.ToResponse())
	s.audit.LogAction(ctx, userID, domain.AuditActionCreate, "activity_follow", follow.ID.String(), nil, afterJSON, ipAddress, userAgent)

	s.log.Info("followed resource",
		slog.String("user_id", userID.String()),
		slog.String("resource_type", resourceType),
		slog.String("resource_id", resourceID),
	)

	return follow, nil
}

// Unfollow removes a follow relationship.
func (s *ActivityFollowService) Unfollow(
	ctx context.Context,
	userID uuid.UUID,
	followID uuid.UUID,
	ipAddress string,
	userAgent string,
) error {
	// Verify the follow belongs to the user by listing their follows
	follows, _, err := s.followRepo.FindByUser(ctx, userID, 1000, 0)
	if err != nil {
		return apperrors.WrapInternal(err)
	}

	var targetFollow *domain.ActivityFollow
	for _, f := range follows {
		if f.ID == followID {
			targetFollow = f
			break
		}
	}

	if targetFollow == nil {
		return apperrors.NewAppError("NOT_FOUND", "follow not found or does not belong to user", 404)
	}

	// Audit log before deletion
	beforeJSON, _ := json.Marshal(targetFollow.ToResponse())

	if err := s.followRepo.Delete(ctx, followID); err != nil {
		s.log.Error("failed to delete follow",
			slog.String("error", err.Error()),
			slog.String("follow_id", followID.String()),
		)
		return apperrors.WrapInternal(err)
	}

	s.audit.LogAction(ctx, userID, domain.AuditActionDelete, "activity_follow", followID.String(), beforeJSON, nil, ipAddress, userAgent)

	s.log.Info("unfollowed resource",
		slog.String("user_id", userID.String()),
		slog.String("follow_id", followID.String()),
	)

	return nil
}

// ListFollows lists all follows for a user with pagination.
func (s *ActivityFollowService) ListFollows(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
) ([]*domain.ActivityFollow, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return s.followRepo.FindByUser(ctx, userID, limit, offset)
}

// IsFollowing checks if a user is following a specific resource.
func (s *ActivityFollowService) IsFollowing(
	ctx context.Context,
	userID uuid.UUID,
	resourceType string,
	resourceID string,
) (bool, error) {
	return s.followRepo.ExistsByUserAndResource(ctx, userID, resourceType, resourceID)
}