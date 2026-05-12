package service

import (
	"context"
	"log/slog"

	"strconv"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

type TagService struct {
	tagRepo       repository.TagRepository
	entityTagRepo repository.EntityTagRepository
	enforcer      *permission.Enforcer
	audit         *AuditService
	log           *slog.Logger
}

func NewTagService(
	tagRepo repository.TagRepository,
	entityTagRepo repository.EntityTagRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	log *slog.Logger,
) *TagService {
	return &TagService{
		tagRepo:       tagRepo,
		entityTagRepo: entityTagRepo,
		enforcer:      enforcer,
		audit:         audit,
		log:           log,
	}
}

func resolveTagOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	if hasOrgID && orgID != uuid.Nil {
		return orgID.String()
	}
	return "default"
}

func (s *TagService) Create(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, req request.CreateTagRequest, ipAddress, userAgent string) (*domain.TagResponse, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "create")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to create tags", 403)
	}

	slug := domain.GenerateSlug(req.Name)

	existing, err := s.tagRepo.FindByName(ctx, orgID, req.Name)
	if err == nil && existing != nil {
		return nil, apperrors.NewAppError("CONFLICT", "tag with this name already exists in this organization", 409)
	}

	existingSlug, err := s.tagRepo.FindBySlug(ctx, orgID, slug)
	if err == nil && existingSlug != nil {
		for i := 2; i <= 10; i++ {
			candidateSlug := slug + "-" + strconv.Itoa(i)
			existingSlug, err = s.tagRepo.FindBySlug(ctx, orgID, candidateSlug)
			if err != nil || existingSlug == nil {
				slug = candidateSlug
				break
			}
		}
		if existingSlug != nil {
			return nil, apperrors.NewAppError("CONFLICT", "could not generate unique slug", 409)
		}
	}

	tag := &domain.Tag{
		OrganizationID: orgID,
		Name:           req.Name,
		Slug:           slug,
		Color:          req.Color,
	}

	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, err
	}

	s.audit.LogAction(ctx, userID, "tag.created", "tag", tag.ID.String(), nil, tag.ToResponse(), ipAddress, userAgent)

	resp := tag.ToResponse()
	return &resp, nil
}

func (s *TagService) GetByID(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, id uuid.UUID) (*domain.TagResponse, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view tags", 403)
	}

	tag, err := s.tagRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tag.OrganizationID != orgID {
		return nil, apperrors.ErrNotFound
	}
	resp := tag.ToResponse()
	return &resp, nil
}

func (s *TagService) GetBySlug(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, slug string) (*domain.TagResponse, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view tags", 403)
	}

	tag, err := s.tagRepo.FindBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	resp := tag.ToResponse()
	return &resp, nil
}

func (s *TagService) List(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, limit, offset int, sort, order string) (*domain.TagListResponse, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to list tags", 403)
	}

	tags, total, err := s.tagRepo.FindByOrg(ctx, orgID, limit, offset, sort, order)
	if err != nil {
		return nil, err
	}

	tagResponses := make([]domain.TagResponse, len(tags))
	for i, t := range tags {
		tagResponses[i] = t.ToResponse()
	}

	return &domain.TagListResponse{Tags: tagResponses, Total: total}, nil
}

func (s *TagService) Update(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, id uuid.UUID, req request.UpdateTagRequest, ipAddress, userAgent string) (*domain.TagResponse, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "update")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to update tags", 403)
	}

	tag, err := s.tagRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tag.OrganizationID != orgID {
		return nil, apperrors.ErrNotFound
	}

	before := tag.ToResponse()

	if req.Name != "" && req.Name != tag.Name {
		existing, findErr := s.tagRepo.FindByName(ctx, orgID, req.Name)
		if findErr == nil && existing != nil && existing.ID != tag.ID {
			return nil, apperrors.NewAppError("CONFLICT", "tag with this name already exists in this organization", 409)
		}
		tag.Name = req.Name
		tag.Slug = domain.GenerateSlug(req.Name)

		existingSlug, slugErr := s.tagRepo.FindBySlug(ctx, orgID, tag.Slug)
		if slugErr == nil && existingSlug != nil && existingSlug.ID != tag.ID {
			for i := 2; i <= 10; i++ {
				candidateSlug := tag.Slug + "-" + strconv.Itoa(i)
				existingSlug, slugErr = s.tagRepo.FindBySlug(ctx, orgID, candidateSlug)
				if slugErr != nil || existingSlug == nil || existingSlug.ID == tag.ID {
					tag.Slug = candidateSlug
					break
				}
			}
		}
	}

	if req.Color != "" {
		tag.Color = req.Color
	}

	if err := s.tagRepo.Update(ctx, tag); err != nil {
		return nil, err
	}

	after := tag.ToResponse()
	s.audit.LogAction(ctx, userID, "tag.updated", "tag", tag.ID.String(), before, after, ipAddress, userAgent)

	return &after, nil
}

func (s *TagService) Delete(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, id uuid.UUID, ipAddress, userAgent string) error {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "delete")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return apperrors.WrapInternal(err)
	}
	if !allowed {
		return apperrors.NewAppError("FORBIDDEN", "insufficient permissions to delete tags", 403)
	}

	tag, err := s.tagRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if tag.OrganizationID != orgID {
		return apperrors.ErrNotFound
	}

	before := tag.ToResponse()

	if err := s.tagRepo.SoftDelete(ctx, id); err != nil {
		return err
	}

	s.audit.LogAction(ctx, userID, "tag.deleted", "tag", tag.ID.String(), before, nil, ipAddress, userAgent)

	return nil
}

func (s *TagService) Autocomplete(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, query string, limit int) (*domain.AutocompleteResponse, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view tags", 403)
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	tags, err := s.tagRepo.Autocomplete(ctx, orgID, query, limit)
	if err != nil {
		return nil, err
	}

	tagResponses := make([]domain.TagResponse, len(tags))
	for i, t := range tags {
		tagResponses[i] = t.ToResponse()
	}

	return &domain.AutocompleteResponse{Tags: tagResponses}, nil
}

func (s *TagService) AttachTags(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, entityType string, entityID uuid.UUID, tagIDs []uuid.UUID, ipAddress, userAgent string) (*domain.BulkAttachResult, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage tags", 403)
	}

	if !domain.IsValidTaggableType(entityType) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid entity type: "+entityType, 422)
	}

	result := &domain.BulkAttachResult{
		Attached: []domain.TagResponse{},
		Skipped:  []domain.TagResponse{},
		Errors:   []domain.BulkError{},
	}

	for _, tagID := range tagIDs {
		tag, err := s.tagRepo.FindByID(ctx, tagID)
		if err != nil {
			if apperrors.IsNotFound(err) {
				result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "tag not found"})
				continue
			}
			result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "internal error"})
			continue
		}

		if tag.DeletedAt.Valid {
			result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "tag is soft-deleted"})
			continue
		}

		if tag.OrganizationID != orgID {
			result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "tag does not belong to this organization"})
			continue
		}

		existing, err := s.entityTagRepo.FindByEntityAndTag(ctx, orgID, entityType, entityID, tagID)
		if err != nil {
			result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "internal error"})
			continue
		}
		if existing != nil {
			result.Skipped = append(result.Skipped, tag.ToResponse())
			continue
		}

		entityTag := &domain.EntityTag{
			EntityType:     entityType,
			EntityID:       entityID,
			TagID:          tagID,
			OrganizationID: orgID,
			CreatedBy:      userID,
		}

		if err := s.entityTagRepo.Create(ctx, entityTag); err != nil {
			result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "failed to attach tag"})
			continue
		}

		_ = s.tagRepo.IncrementUsageCount(ctx, tagID)
		result.Attached = append(result.Attached, tag.ToResponse())
	}

	s.audit.LogAction(ctx, userID, "tag.attached", entityType, entityID.String(), nil, tagIDs, ipAddress, userAgent)

	return result, nil
}

func (s *TagService) DetachTags(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, entityType string, entityID uuid.UUID, tagIDs []uuid.UUID, ipAddress, userAgent string) (*domain.BulkDetachResult, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage tags", 403)
	}

	if !domain.IsValidTaggableType(entityType) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid entity type: "+entityType, 422)
	}

	result := &domain.BulkDetachResult{
		Detached: []domain.TagResponse{},
		Skipped:  []domain.TagResponse{},
		Errors:   []domain.BulkError{},
	}

	for _, tagID := range tagIDs {
		entityTag, err := s.entityTagRepo.FindByEntityAndTag(ctx, orgID, entityType, entityID, tagID)
		if err != nil {
			result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "internal error"})
			continue
		}
		if entityTag == nil {
			result.Skipped = append(result.Skipped, domain.TagResponse{ID: tagID})
			continue
		}

		if err := s.entityTagRepo.DeleteByEntityAndTag(ctx, entityTag.ID); err != nil {
			result.Errors = append(result.Errors, domain.BulkError{TagID: tagID, Message: "failed to detach tag"})
			continue
		}

		_ = s.tagRepo.DecrementUsageCount(ctx, tagID)
		result.Detached = append(result.Detached, domain.TagResponse{ID: tagID})
	}

	s.audit.LogAction(ctx, userID, "tag.detached", entityType, entityID.String(), tagIDs, nil, ipAddress, userAgent)

	return result, nil
}

func (s *TagService) ListEntityTags(ctx context.Context, hasOrgID bool, orgID uuid.UUID, userID uuid.UUID, entityType string, entityID uuid.UUID) (*domain.EntityTagsResponse, error) {
	orgDomain := resolveTagOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "tag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission", slog.String("error", err.Error()), slog.String("user_id", userID.String()))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view tags", 403)
	}

	if !domain.IsValidTaggableType(entityType) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid entity type: "+entityType, 422)
	}

	tags, err := s.entityTagRepo.ListTagDetailsByEntity(ctx, orgID, entityType, entityID)
	if err != nil {
		return nil, err
	}

	tagResponses := make([]domain.TagResponse, len(tags))
	for i, t := range tags {
		tagResponses[i] = t.ToResponse()
	}

	return &domain.EntityTagsResponse{
		EntityID:   entityID,
		EntityType: entityType,
		Tags:       tagResponses,
	}, nil
}

