package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// OrganizationService handles organization-related business logic
type OrganizationService struct {
	repo        repository.OrganizationRepository
	enforcer    *permission.Enforcer
	audit       *AuditService
	emailService *EmailService // Email service for invitation emails
	log         *slog.Logger
}

// NewOrganizationService creates a new OrganizationService instance
func NewOrganizationService(
	repo repository.OrganizationRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	emailService *EmailService,
	log *slog.Logger,
) *OrganizationService {
	return &OrganizationService{
		repo:        repo,
		enforcer:    enforcer,
		audit:       audit,
		emailService: emailService,
		log:         log,
	}
}

// CreateOrganization creates a new organization
func (s *OrganizationService) CreateOrganization(
	ctx context.Context,
	userID uuid.UUID,
	name, slug string,
	settings map[string]interface{},
	ipAddress, userAgent string,
) (*domain.Organization, error) {
	// 1. Validate input
	if name == "" || slug == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "name and slug are required", 422)
	}

	// 2. Check for duplicate slug
	existing, err := s.repo.FindBySlug(ctx, slug)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		s.log.Error("failed to check slug uniqueness",
			slog.String("error", err.Error()),
			slog.String("slug", slug),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if existing != nil {
		return nil, apperrors.NewAppError("CONFLICT", "organization slug already exists", 409)
	}

	// 3. Convert settings to JSONB
	var settingsJSON []byte
	if settings != nil {
		settingsJSON, err = domain.NewJSONB(settings)
		if err != nil {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid settings format", 422)
		}
	}

	// 4. Create organization
	org := &domain.Organization{
		Name:      name,
		Slug:      slug,
		OwnerID:   userID,
		Settings:  settingsJSON,
	}

	if err := s.repo.Create(ctx, org); err != nil {
		s.log.Error("failed to create organization",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// 5. Add owner as member with owner role
	member := &domain.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         userID,
		Role:           domain.RoleOwner,
	}

	if err := s.repo.AddMember(ctx, member); err != nil {
		s.log.Error("failed to add owner as member",
			slog.String("error", err.Error()),
			slog.String("org_id", org.ID.String()),
			slog.String("user_id", userID.String()),
		)
		// Note: Could rollback org creation here if needed
		return nil, apperrors.WrapInternal(err)
	}

	// 6. Set Casbin policy: user is owner in this organization
	if err := s.enforcer.AddRoleForUser(userID.String(), domain.RoleOwner, org.ID.String()); err != nil {
		s.log.Error("failed to add Casbin role",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", org.ID.String()),
		)
		// Continue - role can be added later
	}

	// 7. Add Casbin policies for owner role
	policies := [][]string{
		{domain.RoleOwner, org.ID.String(), "organization", "view"},
		{domain.RoleOwner, org.ID.String(), "organization", "manage"},
		{domain.RoleOwner, org.ID.String(), "organization", "invite"},
		{domain.RoleOwner, org.ID.String(), "organization", "remove"},
	}
	for _, policy := range policies {
		if err := s.enforcer.AddPolicy(policy[0], policy[1], policy[2], policy[3]); err != nil {
			s.log.Error("failed to add Casbin policy",
				slog.String("error", err.Error()),
				slog.Any("policy", policy),
			)
		}
	}

	// 8. Audit log
	s.audit.LogAction(ctx, userID, domain.AuditActionCreate, "organization", org.ID.String(), nil, org, ipAddress, userAgent)

	s.log.Info("organization created",
		slog.String("org_id", org.ID.String()),
		slog.String("org_slug", slug),
		slog.String("owner_id", userID.String()),
	)

	return org, nil
}

// GetOrganization retrieves an organization
func (s *OrganizationService) GetOrganization(
	ctx context.Context,
	userID, orgID uuid.UUID,
) (*domain.Organization, error) {
	// 1. Check permission
	allowed, err := s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "view")
	if err != nil {
		s.log.Error("permission check failed",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// 2. Retrieve organization
	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	return org, nil
}

// ListOrganizations lists organizations where user is a member
func (s *OrganizationService) ListOrganizations(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
) ([]*domain.Organization, int64, error) {
	// TODO: Implement user-specific organization list
	// For now, return all organizations (global admin scenario)
	// Future: Query based on membership
	return s.repo.FindAll(ctx, limit, offset)
}

// UpdateOrganization updates organization details
func (s *OrganizationService) UpdateOrganization(
	ctx context.Context,
	userID, orgID uuid.UUID,
	name, slug string,
	settings map[string]interface{},
	ipAddress, userAgent string,
) (*domain.Organization, error) {
	// 1. Check permission
	allowed, err := s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "manage")
	if err != nil {
		s.log.Error("permission check failed",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// 2. Retrieve organization
	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// 3. Store original for audit
	original := *org

	// 4. Update fields
	if name != "" {
		org.Name = name
	}
	if slug != "" && slug != org.Slug {
		// Check for duplicate slug
		existing, err := s.repo.FindBySlug(ctx, slug)
		if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.WrapInternal(err)
		}
		if existing != nil {
			return nil, apperrors.NewAppError("CONFLICT", "organization slug already exists", 409)
		}
		org.Slug = slug
	}
	if settings != nil {
		settingsJSON, err := domain.NewJSONB(settings)
		if err != nil {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid settings format", 422)
		}
		org.Settings = settingsJSON
	}

	// 5. Persist changes
	if err := s.repo.Update(ctx, org); err != nil {
		return nil, err
	}

	// 6. Audit log
	s.audit.LogAction(ctx, userID, domain.AuditActionUpdate, "organization", org.ID.String(), &original, org, ipAddress, userAgent)

	s.log.Info("organization updated",
		slog.String("org_id", org.ID.String()),
		slog.String("user_id", userID.String()),
	)

	return org, nil
}

// DeleteOrganization soft deletes an organization
func (s *OrganizationService) DeleteOrganization(
	ctx context.Context,
	userID, orgID uuid.UUID,
	ipAddress, userAgent string,
) error {
	// 1. Check permission
	allowed, err := s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "manage")
	if err != nil {
		s.log.Error("permission check failed",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return apperrors.WrapInternal(err)
	}
	if !allowed {
		return apperrors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// 2. Retrieve organization (for audit)
	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return err
	}

	// 3. Soft delete
	if err := s.repo.SoftDelete(ctx, orgID); err != nil {
		return err
	}

	// 4. Remove all Casbin policies for this organization
	// Note: This is a simplified approach - production may need more sophisticated cleanup
	s.log.Info("organization deleted",
		slog.String("org_id", orgID.String()),
		slog.String("user_id", userID.String()),
	)

	// 5. Audit log
	s.audit.LogAction(ctx, userID, domain.AuditActionDelete, "organization", orgID.String(), org, nil, ipAddress, userAgent)

	return nil
}

// AddMember adds a user to an organization
func (s *OrganizationService) AddMember(
	ctx context.Context,
	userID, orgID, newMemberID uuid.UUID,
	role string,
	ipAddress, userAgent string,
) (*domain.OrganizationMember, error) {
	// 1. Check permission
	allowed, err := s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "invite")
	if err != nil {
		s.log.Error("permission check failed",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// 2. Validate role
	if !domain.IsValidRole(role) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("invalid role: %s", role), 422)
	}

	// 3. Check if organization exists
	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// 4. Check if user is already a member
	isMember, err := s.repo.IsMember(ctx, orgID, newMemberID)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	if isMember {
		return nil, apperrors.NewAppError("CONFLICT", "user is already a member of this organization", 409)
	}

	// 5. Add member
	member := &domain.OrganizationMember{
		OrganizationID: orgID,
		UserID:         newMemberID,
		Role:           role,
	}

	if err := s.repo.AddMember(ctx, member); err != nil {
		s.log.Error("failed to add member",
			slog.String("error", err.Error()),
			slog.String("org_id", orgID.String()),
			slog.String("user_id", newMemberID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// 6. Add Casbin role for user
	if err := s.enforcer.AddRoleForUser(newMemberID.String(), role, orgID.String()); err != nil {
		s.log.Error("failed to add Casbin role for member",
			slog.String("error", err.Error()),
			slog.String("user_id", newMemberID.String()),
			slog.String("role", role),
			slog.String("org_id", orgID.String()),
		)
		// Continue - role can be added later
	}

	// 7. Audit log
	s.audit.LogAction(ctx, userID, domain.AuditActionCreate, "organization_member", newMemberID.String(), nil, member, ipAddress, userAgent)

	// 8. Send invitation email (non-blocking, fire-and-forget)
	if s.emailService != nil {
		go func() {
			bgCtx := context.Background()
			err := s.emailService.QueueEmail(bgCtx, &EmailRequest{
				To:       "", // Will be filled by worker from newMemberID lookup
				Template: "org-invitation",
				Data: map[string]any{
					"OrgID":      orgID.String(),
					"OrgName":    org.Name,
					"OrgSlug":    org.Slug,
					"Role":       role,
					"InviterID":  userID.String(),
					"MemberID":   newMemberID.String(),
				},
			})
			if err != nil {
				s.log.Warn("failed to queue invitation email",
					slog.String("error", err.Error()),
					slog.String("org_id", orgID.String()),
					slog.String("user_id", newMemberID.String()),
				)
			}
		}()
	}

	s.log.Info("member added",
		slog.String("org_id", orgID.String()),
		slog.String("user_id", newMemberID.String()),
		slog.String("role", role),
		slog.String("added_by", userID.String()),
	)

	return member, nil
}

// RemoveMember removes a user from an organization
func (s *OrganizationService) RemoveMember(
	ctx context.Context,
	userID, orgID, memberID uuid.UUID,
	ipAddress, userAgent string,
) error {
	// 1. Check permission
	allowed, err := s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "remove")
	if err != nil {
		s.log.Error("permission check failed",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return apperrors.WrapInternal(err)
	}
	if !allowed {
		return apperrors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// 2. Find member to verify role
	member, err := s.repo.FindMember(ctx, orgID, memberID)
	if err != nil {
		return err
	}

	// 3. Get organization to check owner
	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return err
	}

	// 4. Prevent removing owner
	if member.UserID == org.OwnerID {
		return apperrors.NewAppError("INVALID_OPERATION", "cannot remove organization owner", 422)
	}

	// 5. Remove member
	if err := s.repo.RemoveMember(ctx, orgID, memberID); err != nil {
		return err
	}

	// 6. Remove Casbin role for user
	if err := s.enforcer.RemoveRoleForUser(memberID.String(), member.Role, orgID.String()); err != nil {
		s.log.Error("failed to remove Casbin role for member",
			slog.String("error", err.Error()),
			slog.String("user_id", memberID.String()),
			slog.String("role", member.Role),
			slog.String("org_id", orgID.String()),
		)
		// Continue - role can be removed later
	}

	// 7. Audit log
	s.audit.LogAction(ctx, userID, domain.AuditActionDelete, "organization_member", memberID.String(), member, nil, ipAddress, userAgent)

	s.log.Info("member removed",
		slog.String("org_id", orgID.String()),
		slog.String("user_id", memberID.String()),
		slog.String("removed_by", userID.String()),
	)

	return nil
}

// GetMembers lists organization members
func (s *OrganizationService) GetMembers(
	ctx context.Context,
	userID, orgID uuid.UUID,
	limit, offset int,
) ([]*domain.OrganizationMember, int64, error) {
	// 1. Check permission
	allowed, err := s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "view")
	if err != nil {
		s.log.Error("permission check failed",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, 0, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, 0, apperrors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// 2. Retrieve members
	members, total, err := s.repo.FindMembers(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return members, total, nil
}