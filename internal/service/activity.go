package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ActivityService handles activity feed business logic including listing,
// read tracking, EventBus subscription, and actor name resolution.
type ActivityService struct {
	activityRepo repository.ActivityRepository
	readRepo     repository.ActivityReadRepository
	followRepo   repository.ActivityFollowRepository
	userRepo     repository.UserRepository
	enforcer     *permission.Enforcer
	audit        *AuditService
	log          *slog.Logger
	eventBus     *domain.EventBus
}

// NewActivityService creates a new ActivityService instance.
func NewActivityService(
	activityRepo repository.ActivityRepository,
	readRepo repository.ActivityReadRepository,
	followRepo repository.ActivityFollowRepository,
	userRepo repository.UserRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	log *slog.Logger,
) *ActivityService {
	return &ActivityService{
		activityRepo: activityRepo,
		readRepo:     readRepo,
		followRepo:   followRepo,
		userRepo:     userRepo,
		enforcer:     enforcer,
		audit:        audit,
		log:          log,
	}
}

// resolveActivityOrgDomain returns the Casbin domain string for RBAC enforcement.
func resolveActivityOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	if hasOrgID && orgID != uuid.Nil {
		return orgID.String()
	}
	return "default"
}

// ListByOrganization lists activities for the given organization context,
// enriching each with is_read, is_following, and actor_name information.
func (s *ActivityService) ListByOrganization(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	filters repository.ActivityFilters,
	limit, offset int,
) (*domain.ActivityListResponse, error) {
	orgDomain := resolveActivityOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "activity", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view activities", 403)
	}

	// Fetch activities
	activities, total, err := s.activityRepo.FindByOrganization(ctx, hasOrgID, orgID, filters, limit, offset)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	if len(activities) == 0 {
		return &domain.ActivityListResponse{
			Activities: []domain.ActivityResponse{},
			Total:      total,
			UnreadCount: 0,
			Limit:      limit,
			Offset:     offset,
		}, nil
	}

	// Batch resolve: is_read status
	activityIDs := make([]uuid.UUID, len(activities))
	for i, a := range activities {
		activityIDs[i] = a.ID
	}
	readMap, err := s.readRepo.FindByUserAndActivityIDs(ctx, userID, activityIDs)
	if err != nil {
		s.log.Warn("failed to batch fetch read status, defaulting to unread",
			slog.String("error", err.Error()),
		)
		readMap = make(map[uuid.UUID]bool)
	}

	// Batch resolve: is_following status
	resources := make([]repository.FollowResource, len(activities))
	for i, a := range activities {
		resources[i] = repository.FollowResource{
			ResourceType: a.ResourceType,
			ResourceID:  a.ResourceID,
		}
	}
	followMap, err := s.followRepo.FindByUserAndResourceIDs(ctx, userID, resources)
	if err != nil {
		s.log.Warn("failed to batch fetch follow status, defaulting to not following",
			slog.String("error", err.Error()),
		)
		followMap = make(map[string]bool)
	}

	// Batch resolve: actor names
	actorIDs := make([]uuid.UUID, 0, len(activities))
	actorSeen := make(map[uuid.UUID]bool)
	for _, a := range activities {
		if !actorSeen[a.ActorID] {
			actorSeen[a.ActorID] = true
			actorIDs = append(actorIDs, a.ActorID)
		}
	}
	actorNames := make(map[uuid.UUID]string)
	for _, aid := range actorIDs {
		user, err := s.userRepo.FindByID(ctx, aid)
		if err != nil {
			s.log.Warn("failed to resolve actor name",
				slog.String("actor_id", aid.String()),
				slog.String("error", err.Error()),
			)
			continue
		}
		actorNames[aid] = user.Email
	}

	// Build response
	responses := make([]domain.ActivityResponse, len(activities))
	for i, a := range activities {
		actorName := actorNames[a.ActorID]
		isRead := readMap[a.ID]
		// Use composite key "resourceType:resourceID" for follow lookup
		followKey := fmt.Sprintf("%s:%s", a.ResourceType, a.ResourceID)
		isFollowing := followMap[followKey]
		responses[i] = a.ToResponse(actorName, isRead, isFollowing)
	}

	// Count unread
	var unreadCount int64
	if filters.UnreadOnly {
		// If filtering unread only, total IS the unread count
		unreadCount = total
	} else {
		countFilters := filters
		countFilters.UnreadOnly = false
		unreadCount, _ = s.readRepo.CountUnreadByUser(ctx, userID, hasOrgID, orgID, countFilters)
	}

	return &domain.ActivityListResponse{
		Activities: responses,
		Total:      total,
		UnreadCount: unreadCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// FindByID retrieves a single activity by ID.
func (s *ActivityService) FindByID(ctx context.Context, id uuid.UUID) (*domain.Activity, error) {
	activity, err := s.activityRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return activity, nil
}

// CountUnread returns the number of unread activities for a user.
func (s *ActivityService) CountUnread(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	filters repository.ActivityFilters,
) (int64, error) {
	return s.readRepo.CountUnreadByUser(ctx, userID, hasOrgID, orgID, filters)
}

// MarkAllRead marks all visible activities as read for a user.
func (s *ActivityService) MarkAllRead(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
) (int64, error) {
	return s.readRepo.MarkAllReadByUser(ctx, userID, hasOrgID, orgID)
}

// MarkAsRead marks a single activity as read for a user.
func (s *ActivityService) MarkAsRead(
	ctx context.Context,
	userID uuid.UUID,
	activityID uuid.UUID,
) error {
	// Verify activity exists
	_, err := s.activityRepo.FindByID(ctx, activityID)
	if err != nil {
		return err
	}

	read := &domain.ActivityRead{
		UserID:     userID,
		ActivityID: activityID,
	}
	return s.readRepo.Upsert(ctx, read)
}

// SoftDelete soft-deletes an activity. Requires activity:manage permission.
func (s *ActivityService) SoftDelete(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
) error {
	orgDomain := resolveActivityOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "activity", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return apperrors.WrapInternal(err)
	}
	if !allowed {
		return apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage activities", 403)
	}

	return s.activityRepo.SoftDelete(ctx, id)
}

// SubscribeToEventBus registers the service as a handler on the given EventBus.
// Uses buffered channel pattern to decouple event publishing from processing.
func (s *ActivityService) SubscribeToEventBus(eventBus *domain.EventBus) {
	s.eventBus = eventBus
	ch := make(chan domain.WebhookEvent, 256)
	eventBus.Subscribe(func(event domain.WebhookEvent) {
		ch <- event
	})
	go s.processEvents(ch)
}

// SetEventBus sets the EventBus for post-construction injection.
func (s *ActivityService) SetEventBus(eventBus *domain.EventBus) {
	s.eventBus = eventBus
}

// processEvents consumes events from the buffered channel in a background goroutine.
func (s *ActivityService) processEvents(ch chan domain.WebhookEvent) {
	for event := range ch {
		if err := s.handleEvent(event); err != nil {
			s.log.Error("failed to handle activity event",
				slog.String("event_type", event.Type),
				slog.String("error", err.Error()),
			)
		}
	}
}

// handleEvent maps an EventBus event to an Activity and persists it.
func (s *ActivityService) handleEvent(event domain.WebhookEvent) error {
	mapping, ok := domain.GetActivityMapping(event.Type)
	if !ok {
		return fmt.Errorf("no activity mapping for event type: %s", event.Type)
	}

	actorID := extractActorID(event)
	orgID := event.OrgID

	// Build metadata with required description field
	metadata := buildActivityMetadata(event, mapping)

	activity := &domain.Activity{
		ActorID:        actorID,
		ActionType:     mapping.ActionType,
		ResourceType:   mapping.ResourceType,
		ResourceID:    extractResourceID(event),
		OrganizationID: orgID,
		Metadata:      metadata,
	}

	return s.activityRepo.Create(context.Background(), activity)
}

// extractActorID extracts the actor user ID from the event payload.
func extractActorID(event domain.WebhookEvent) uuid.UUID {
	if event.Payload == nil {
		return uuid.Nil
	}
	// Payload is a map[string]interface{} from EventBus
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if uid, ok := payload["user_id"].(string); ok {
			if id, err := uuid.Parse(uid); err == nil {
				return id
			}
		}
	}
	return uuid.Nil
}

// extractResourceID extracts the resource ID from the event payload.
func extractResourceID(event domain.WebhookEvent) string {
	if event.Payload == nil {
		return ""
	}
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if rid, ok := payload["id"].(string); ok && rid != "" {
			return rid
		}
		// Fallback for specific resource ID fields
		for _, key := range []string{"invoice_id", "news_id", "comment_id", "media_id"} {
			if rid, ok := payload[key].(string); ok && rid != "" {
				return rid
			}
		}
	}
	return ""
}

// buildActivityMetadata creates JSON metadata from the event payload with a required description field.
func buildActivityMetadata(event domain.WebhookEvent, mapping domain.ActivityMapping) datatypes.JSON {
	description := fmt.Sprintf("%s %s %s", mapping.ActionType, mapping.ResourceType, event.Type)
	meta := map[string]interface{}{
		"description": description,
		"event_type":  event.Type,
	}

	// Merge in original payload fields (skip internal fields)
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		for k, v := range payload {
			switch k {
			case "user_id", "id": // skip internal fields
			default:
				meta[k] = v
			}
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		// Fallback to minimal metadata
		data, _ = json.Marshal(map[string]string{
			"description": description,
			"event_type":  event.Type,
		})
	}
	return datatypes.JSON(data)
}