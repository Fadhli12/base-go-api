package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// mentionRegex matches @{email} patterns in comment content.
var mentionRegex = regexp.MustCompile(`@\{([^}]+)\}`)

// CommentService handles comment business logic including CRUD operations,
// @mention parsing, RBAC enforcement, and audit logging.
type CommentService struct {
	repo     repository.CommentRepository
	userRepo repository.UserRepository
	enforcer *permission.Enforcer
	audit    *AuditService
	log      *slog.Logger
}

// NewCommentService creates a new CommentService instance.
func NewCommentService(
	repo repository.CommentRepository,
	userRepo repository.UserRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	log *slog.Logger,
) *CommentService {
	return &CommentService{
		repo:     repo,
		userRepo: userRepo,
		enforcer: enforcer,
		audit:    audit,
		log:      log,
	}
}

// resolveCommentOrgDomain returns the Casbin domain string for RBAC enforcement.
func resolveCommentOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	if hasOrgID && orgID != uuid.Nil {
		return orgID.String()
	}
	return "default"
}

// Create creates a new comment with RBAC permission check, mention parsing, and audit logging.
func (s *CommentService) Create(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	commentableType string,
	commentableID uuid.UUID,
	req request.CreateCommentRequest,
	ipAddress string,
	userAgent string,
) (*domain.CommentResponse, error) {
	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "create")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to create comments", 403)
	}

	// Validate commentable type
	if !domain.IsValidCommentableType(commentableType) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid commentable type: "+commentableType, 422)
	}

	// Parse parent ID for replies
	var parentID *uuid.UUID
	if req.ParentID != "" {
		parsedParentID, err := uuid.Parse(req.ParentID)
		if err != nil {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid parent_id format", 422)
		}

		// Validate parent exists and belongs to same commentable entity
		parent, err := s.repo.FindByID(ctx, parsedParentID)
		if err != nil {
			if apperrors.IsNotFound(err) {
				return nil, apperrors.NewAppError("NOT_FOUND", "parent comment not found", 404)
			}
			return nil, apperrors.WrapInternal(err)
		}
		if parent.CommentableType != commentableType || parent.CommentableID != commentableID {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "parent comment must belong to the same commentable entity", 422)
		}
		// Flatten: if replying to a reply, attach to the top-level parent instead
		if parent.ParentID != nil {
			parentID = parent.ParentID
		} else {
			parentID = &parsedParentID
		}
	}

	// Parse @mentions from content
	mentionedIDs, err := s.parseMentions(ctx, req.Content)
	if err != nil {
		s.log.Error("failed to parse mentions",
			slog.String("error", err.Error()),
		)
		// Don't fail comment creation on mention parse errors
		mentionedIDs = nil
	}
	mentionedUserIDsJSON := s.buildMentionedUserIDsJSON(mentionedIDs)

	comment := &domain.Comment{
		ParentID:          parentID,
		AuthorID:          userID,
		OrganizationID:   orgID,
		CommentableType:  commentableType,
		CommentableID:   commentableID,
		Content:          req.Content,
		MentionedUserIDs: mentionedUserIDsJSON,
	}

	if err := s.repo.Create(ctx, comment); err != nil {
		s.log.Error("failed to create comment",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log
	afterJSON, _ := json.Marshal(comment.ToResponse(userID.String(), 0))
	s.audit.LogAction(ctx, userID, domain.AuditActionCreate, "comment", comment.ID.String(), nil, afterJSON, ipAddress, userAgent)

	resp, err := s.buildCommentResponse(ctx, comment)
	if err != nil {
		// Comment was created but response building failed; return basic response
		basicResp := comment.ToResponse(userID.String(), 0)
		return &basicResp, nil
	}
	return resp, nil
}

// GetByID retrieves a comment by ID with RBAC view permission.
func (s *CommentService) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
) (*domain.CommentResponse, error) {
	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view comments", 403)
	}

	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.buildCommentResponse(ctx, comment)
}

// ListByCommentable lists comments for a given commentable entity with RBAC view permission.
func (s *CommentService) ListByCommentable(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	commentableType string,
	commentableID uuid.UUID,
	limit, offset int,
) ([]*domain.CommentResponse, int64, error) {
	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, 0, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, 0, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view comments", 403)
	}

	// Validate commentable type
	if !domain.IsValidCommentableType(commentableType) {
		return nil, 0, apperrors.NewAppError("VALIDATION_ERROR", "invalid commentable type: "+commentableType, 422)
	}

	comments, total, err := s.repo.FindByCommentable(ctx, commentableType, commentableID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	responses := make([]*domain.CommentResponse, 0, len(comments))
	for _, comment := range comments {
		resp, err := s.buildCommentResponse(ctx, comment)
		if err != nil {
			// Skip comments whose author can't be resolved
			basicResp := comment.ToResponse("", 0)
			responses = append(responses, &basicResp)
			continue
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

// ListReplies lists replies to a parent comment with RBAC view permission.
func (s *CommentService) ListReplies(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	parentID uuid.UUID,
	limit, offset int,
) ([]*domain.CommentResponse, int64, error) {
	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, 0, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, 0, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view comments", 403)
	}

	replies, total, err := s.repo.FindReplies(ctx, parentID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	responses := make([]*domain.CommentResponse, 0, len(replies))
	for _, reply := range replies {
		resp, err := s.buildCommentResponse(ctx, reply)
		if err != nil {
			basicResp := reply.ToResponse("", 0)
			responses = append(responses, &basicResp)
			continue
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}

// Update updates a comment's content. Only the author can update their own comments.
func (s *CommentService) Update(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
	req request.UpdateCommentRequest,
	ipAddress string,
	userAgent string,
) (*domain.CommentResponse, error) {
	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "create")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to update comments", 403)
	}

	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ownership check: only the author can update
	if comment.AuthorID != userID {
		return nil, apperrors.NewAppError("FORBIDDEN", "you can only update your own comments", 403)
	}

	// Store original for audit
	beforeJSON, _ := json.Marshal(comment.ToResponse("", 0))

	// Update content
	comment.Content = req.Content
	now := time.Now()
	comment.EditedAt = &now

	// Re-parse mentions
	mentionedIDs, err := s.parseMentions(ctx, req.Content)
	if err != nil {
		s.log.Error("failed to parse mentions on update",
			slog.String("error", err.Error()),
		)
		mentionedIDs = nil
	}
	comment.MentionedUserIDs = s.buildMentionedUserIDsJSON(mentionedIDs)

	if err := s.repo.Update(ctx, comment); err != nil {
		s.log.Error("failed to update comment",
			slog.String("error", err.Error()),
			slog.String("comment_id", id.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log
	afterJSON, _ := json.Marshal(comment.ToResponse("", 0))
	s.audit.LogAction(ctx, userID, domain.AuditActionUpdate, "comment", comment.ID.String(), beforeJSON, afterJSON, ipAddress, userAgent)

	return s.buildCommentResponse(ctx, comment)
}

// Delete soft-deletes a comment. The author or an admin with delete_any permission can delete.
func (s *CommentService) Delete(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
	ipAddress string,
	userAgent string,
) error {
	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)

	// Ownership check: author can delete own comments, admins can delete any
	if comment.AuthorID == userID {
		allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "delete")
		if err != nil {
			s.log.Error("failed to enforce permission",
				slog.String("error", err.Error()),
				slog.String("user_id", userID.String()),
			)
			return apperrors.WrapInternal(err)
		}
		if !allowed {
			return apperrors.NewAppError("FORBIDDEN", "insufficient permissions to delete comments", 403)
		}
	} else {
		allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "delete_any")
		if err != nil {
			s.log.Error("failed to enforce permission",
				slog.String("error", err.Error()),
				slog.String("user_id", userID.String()),
			)
			return apperrors.WrapInternal(err)
		}
		if !allowed {
			return apperrors.NewAppError("FORBIDDEN", "insufficient permissions to delete any comment", 403)
		}
	}

	// Store for audit before deletion
	beforeJSON, _ := json.Marshal(comment.ToResponse("", 0))

	if err := s.repo.SoftDelete(ctx, id); err != nil {
		s.log.Error("failed to delete comment",
			slog.String("error", err.Error()),
			slog.String("comment_id", id.String()),
		)
		return apperrors.WrapInternal(err)
	}

	// Audit log
	s.audit.LogAction(ctx, userID, domain.AuditActionDelete, "comment", id.String(), beforeJSON, nil, ipAddress, userAgent)

	return nil
}

// Pin pins a comment. Requires comment:manage permission.
func (s *CommentService) Pin(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
	ipAddress string,
	userAgent string,
) (*domain.CommentResponse, error) {
	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage comments", 403)
	}

	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if comment.IsPinned {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "comment is already pinned", 422)
	}

	// Store before state for audit
	beforeJSON, _ := json.Marshal(comment.ToResponse("", 0))

	comment.IsPinned = true
	if err := s.repo.Update(ctx, comment); err != nil {
		s.log.Error("failed to pin comment",
			slog.String("error", err.Error()),
			slog.String("comment_id", id.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log
	afterJSON, _ := json.Marshal(comment.ToResponse("", 0))
	s.audit.LogAction(ctx, userID, domain.AuditActionPin, "comment", comment.ID.String(), beforeJSON, afterJSON, ipAddress, userAgent)

	return s.buildCommentResponse(ctx, comment)
}

// Unpin unpins a comment. Requires comment:manage permission.
func (s *CommentService) Unpin(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
	ipAddress string,
	userAgent string,
) (*domain.CommentResponse, error) {
	orgDomain := resolveCommentOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "comment", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage comments", 403)
	}

	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !comment.IsPinned {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "comment is already unpinned", 422)
	}

	// Store before state for audit
	beforeJSON, _ := json.Marshal(comment.ToResponse("", 0))

	comment.IsPinned = false
	if err := s.repo.Update(ctx, comment); err != nil {
		s.log.Error("failed to unpin comment",
			slog.String("error", err.Error()),
			slog.String("comment_id", id.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log
	afterJSON, _ := json.Marshal(comment.ToResponse("", 0))
	s.audit.LogAction(ctx, userID, domain.AuditActionUnpin, "comment", comment.ID.String(), beforeJSON, afterJSON, ipAddress, userAgent)

	return s.buildCommentResponse(ctx, comment)
}

// parseMentions extracts @{email} patterns from content and resolves them to user IDs.
func (s *CommentService) parseMentions(ctx context.Context, content string) ([]uuid.UUID, error) {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	var mentionedIDs []uuid.UUID
	seen := make(map[string]bool)

	for _, match := range matches {
		email := match[1]
		if seen[email] {
			continue
		}
		seen[email] = true

		user, err := s.userRepo.FindByEmail(ctx, email)
		if err != nil {
			if apperrors.IsNotFound(err) {
				continue // skip unknown users
			}
			return nil, apperrors.WrapInternal(err)
		}
		mentionedIDs = append(mentionedIDs, user.ID)
	}

	return mentionedIDs, nil
}

// buildMentionedUserIDsJSON converts a slice of UUIDs to JSONB format.
func (s *CommentService) buildMentionedUserIDsJSON(mentionedIDs []uuid.UUID) datatypes.JSON {
	if len(mentionedIDs) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	ids := make([]string, len(mentionedIDs))
	for i, id := range mentionedIDs {
		ids[i] = id.String()
	}
	data, _ := json.Marshal(ids)
	return datatypes.JSON(data)
}

// buildCommentResponse builds a CommentResponse from a Comment entity, resolving author name and reply count.
func (s *CommentService) buildCommentResponse(ctx context.Context, comment *domain.Comment) (*domain.CommentResponse, error) {
	// Resolve author name (use email since User model has no name field)
	authorName := ""
	author, err := s.userRepo.FindByID(ctx, comment.AuthorID)
	if err != nil {
		// Don't fail if author not found (may be soft-deleted)
		s.log.Warn("failed to resolve comment author",
			slog.String("author_id", comment.AuthorID.String()),
			slog.String("error", err.Error()),
		)
	} else {
		authorName = author.Email
	}

	// Count replies
	var replyCount int64
	replyCount, err = s.repo.CountReplies(ctx, comment.ID)
	if err != nil {
		s.log.Warn("failed to count replies",
			slog.String("comment_id", comment.ID.String()),
			slog.String("error", err.Error()),
		)
		replyCount = 0
	}

	return &domain.CommentResponse{
		ID:                comment.ID,
		ParentID:          comment.ParentID,
		AuthorID:          comment.AuthorID,
		AuthorName:        authorName,
		OrganizationID:   comment.OrganizationID,
		CommentableType:  comment.CommentableType,
		CommentableID:   comment.CommentableID,
		Content:          comment.Content,
		MentionedUserIDs: json.RawMessage(comment.MentionedUserIDs),
		IsPinned:         comment.IsPinned,
		EditedAt:         comment.EditedAt,
		ReplyCount:       replyCount,
		CreatedAt:        comment.CreatedAt,
		UpdatedAt:        comment.UpdatedAt,
	}, nil
}