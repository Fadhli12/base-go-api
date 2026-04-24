# Team/Organization Support Implementation Plan

**Feature:** Multi-tenant organization support with organization-scoped resources  
**Complexity:** Medium  
**Effort:** 2-3 days  
**Branch:** `003-organization-support`  
**Status:** Planning  

---

## Executive Summary

This feature adds multi-tenant support to the Go API Base, allowing users to belong to organizations with organization-scoped resources. It extends the existing Casbin RBAC system to support domain-based access control using organization IDs.

**Key Principles:**
- Extend existing RBAC without breaking changes
- All resources scoped to organization via `X-Organization-ID` header
- Soft deletes on all new entities
- UUID primary keys
- SQL migrations only (no AutoMigrate)
- Full audit trail via AuditService

---

## Architecture Overview

### Multi-Tenancy Model

```
User (1) ─── (Many) OrganizationMember ─── (1) Organization
                              │
                         (Role: owner/admin/member)

Organization (1) ───────────── (Many) Resources (News, Invoices, Media)
                              │
                     (organization_id FK)
```

### Permission Hierarchy

```
Resource: organization
├── Action: view        → View organization details
├── Action: manage      → Manage organization settings, update info
├── Action: invite      → Invite new members
└── Action: remove      → Remove members

Scope: global → organization -> resource-specific
```

### Context Flow

```
Request → Extract X-Organization-ID Header
        → Validate user membership in organization
        → Load organization context
        → Enforce Casbin rules with domain=organization_id
        → Execute request
        → Log audit trail
```

---

## Phase 1: Database Schema

### Migration: `000003_organizations.up.sql`

```sql
-- Organizations table
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    INDEX idx_organizations_slug (slug),
    INDEX idx_organizations_owner_id (owner_id),
    INDEX idx_organizations_deleted_at (deleted_at)
);

-- Organization members table
CREATE TABLE organization_members (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (organization_id, user_id),
    INDEX idx_organization_members_user_id (user_id),
    INDEX idx_organization_members_role (role)
);

-- Add organization_id to existing tables
ALTER TABLE news ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE invoices ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE media ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

-- Create indexes
CREATE INDEX idx_news_organization_id ON news(organization_id);
CREATE INDEX idx_invoices_organization_id ON invoices(organization_id);
CREATE INDEX idx_media_organization_id ON media(organization_id);
```

### Migration: `000003_organizations.down.sql`

```sql
-- Remove from existing tables
ALTER TABLE news DROP COLUMN IF EXISTS organization_id;
ALTER TABLE invoices DROP COLUMN IF EXISTS organization_id;
ALTER TABLE media DROP COLUMN IF EXISTS organization_id;

-- Drop tables
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organizations;
```

---

## Phase 2: Domain Models

### File: `internal/domain/organization.go`

```go
package domain

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Organization represents a multi-tenant organization
type Organization struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Slug      string         `gorm:"uniqueIndex;size:100;not null" json:"slug"`
	OwnerID   uuid.UUID      `gorm:"type:uuid;not null" json:"owner_id"`
	Settings  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"settings,omitempty"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Owner   *User                   `gorm:"foreignKey:OwnerID" json:"-"`
	Members []*OrganizationMember   `gorm:"foreignKey:OrganizationID" json:"-"`
}

func (Organization) TableName() string {
	return "organizations"
}

// OrganizationResponse strips sensitive data
type OrganizationResponse struct {
	ID        uuid.UUID              `json:"id"`
	Name      string                 `json:"name"`
	Slug      string                 `json:"slug"`
	OwnerID   uuid.UUID              `json:"owner_id"`
	Settings  json.RawMessage        `json:"settings,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	MemberCount int                  `json:"member_count,omitempty"`
}

func (o *Organization) ToResponse() *OrganizationResponse {
	var settings json.RawMessage
	if len(o.Settings) > 0 {
		settings = json.RawMessage(o.Settings)
	}

	resp := &OrganizationResponse{
		ID:        o.ID,
		Name:      o.Name,
		Slug:      o.Slug,
		OwnerID:   o.OwnerID,
		Settings:  settings,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}

	if o.Members != nil {
		resp.MemberCount = len(o.Members)
	}

	return resp
}

// OrganizationMember represents membership in an organization
type OrganizationMember struct {
	OrganizationID uuid.UUID `gorm:"type:uuid;primaryKey" json:"organization_id"`
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	Role           string    `gorm:"size:50;not null;default:'member'" json:"role"`
	JoinedAt       time.Time `gorm:"autoCreateTime" json:"joined_at"`

	// Relations
	User         *User         `gorm:"foreignKey:UserID" json:"-"`
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
}

func (OrganizationMember) TableName() string {
	return "organization_members"
}

// OrganizationMemberResponse for API responses
type OrganizationMemberResponse struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	UserName       string    `json:"user_name,omitempty"`
	UserEmail      string    `json:"user_email,omitempty"`
	Role           string    `json:"role"`
	JoinedAt       time.Time `json:"joined_at"`
}

func (om *OrganizationMember) ToResponse() *OrganizationMemberResponse {
	resp := &OrganizationMemberResponse{
		OrganizationID: om.OrganizationID,
		UserID:         om.UserID,
		Role:           om.Role,
		JoinedAt:       om.JoinedAt,
	}

	if om.User != nil {
		resp.UserName = om.User.Name
		resp.UserEmail = om.User.Email
	}

	return resp
}

// IsOwner checks if member is organization owner
func (om *OrganizationMember) IsOwner(orgOwnerID uuid.UUID) bool {
	return om.UserID == orgOwnerID
}

// IsAdmin checks if member has admin or owner role
func (om *OrganizationMember) IsAdmin(orgOwnerID uuid.UUID) bool {
	return om.IsOwner(orgOwnerID) || om.Role == "admin"
}
```

---

## Phase 3: Repository Layer

### File: `internal/repository/organization.go`

```go
package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go-api/internal/domain"
	"go-api/pkg/errors"
)

// OrganizationRepository defines org operations
type OrganizationRepository interface {
	Create(ctx context.Context, org *domain.Organization) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Organization, error)
	FindAll(ctx context.Context, limit, offset int) ([]*domain.Organization, int64, error)
	Update(ctx context.Context, org *domain.Organization) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// Member operations
	AddMember(ctx context.Context, member *domain.OrganizationMember) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	FindMember(ctx context.Context, orgID, userID uuid.UUID) (*domain.OrganizationMember, error)
	FindMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.OrganizationMember, int64, error)
	GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error)
}

// organizationRepository implements OrganizationRepository
type organizationRepository struct {
	db *gorm.DB
}

func NewOrganizationRepository(db *gorm.DB) OrganizationRepository {
	return &organizationRepository{db: db}
}

func (r *organizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	return r.db.WithContext(ctx).Create(org).Error
}

func (r *organizationRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Preload("Members").
		Where("id = ?", id).
		First(&org).Error

	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &org, nil
}

func (r *organizationRepository) FindBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Preload("Members").
		Where("slug = ?", slug).
		First(&org).Error

	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &org, nil
}

func (r *organizationRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.Organization, int64, error) {
	var orgs []*domain.Organization
	var total int64

	tx := r.db.WithContext(ctx)

	if err := tx.Model(&domain.Organization{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := tx.Preload("Owner").
		Limit(limit).
		Offset(offset).
		Find(&orgs).Error; err != nil {
		return nil, 0, err
	}

	return orgs, total, nil
}

func (r *organizationRepository) Update(ctx context.Context, org *domain.Organization) error {
	return r.db.WithContext(ctx).
		Model(org).
		Updates(org).Error
}

func (r *organizationRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&domain.Organization{}).
		Where("id = ?", id).
		Delete(nil).Error
}

// Member operations

func (r *organizationRepository) AddMember(ctx context.Context, member *domain.OrganizationMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *organizationRepository) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Delete(&domain.OrganizationMember{}).Error
}

func (r *organizationRepository) FindMember(ctx context.Context, orgID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	var member domain.OrganizationMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		First(&member).Error

	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &member, nil
}

func (r *organizationRepository) FindMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.OrganizationMember, int64, error) {
	var members []*domain.OrganizationMember
	var total int64

	tx := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID)

	if err := tx.Model(&domain.OrganizationMember{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := tx.Preload("User").
		Limit(limit).
		Offset(offset).
		Find(&members).Error; err != nil {
		return nil, 0, err
	}

	return members, total, nil
}

func (r *organizationRepository) GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	var role string
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Select("role").
		Scan(&role).Error

	if err == gorm.ErrRecordNotFound {
		return "", errors.ErrNotFound
	}

	return role, err
}
```

---

## Phase 4: Service Layer

### File: `internal/service/organization.go`

```go
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"go-api/internal/domain"
	"go-api/internal/permission"
	"go-api/internal/repository"
	"go-api/pkg/errors"
)

// OrganizationService handles organization operations
type OrganizationService struct {
	repo     repository.OrganizationRepository
	enforcer *permission.Enforcer
	audit    *AuditService
	log      *zap.Logger
}

func NewOrganizationService(
	repo repository.OrganizationRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	log *zap.Logger,
) *OrganizationService {
	return &OrganizationService{
		repo:     repo,
		enforcer: enforcer,
		audit:    audit,
		log:      log,
	}
}

// CreateOrganization creates a new organization
func (s *OrganizationService) CreateOrganization(
	ctx context.Context,
	userID uuid.UUID,
	name, slug string,
	settings map[string]interface{},
) (*domain.Organization, error) {
	// Validate input
	if name == "" || slug == "" {
		return nil, errors.NewAppError("INVALID_INPUT", "name and slug required", 400)
	}

	// Create organization
	settingsJSON, _ := domain.NewJSONB(settings)
	org := &domain.Organization{
		Name:      name,
		Slug:      slug,
		OwnerID:   userID,
		Settings:  settingsJSON,
	}

	if err := s.repo.Create(ctx, org); err != nil {
		s.log.Error("failed to create organization", zap.Error(err))
		return nil, err
	}

	// Add owner as admin member
	member := &domain.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         userID,
		Role:           "owner",
	}

	if err := s.repo.AddMember(ctx, member); err != nil {
		s.log.Error("failed to add owner as member", zap.Error(err))
		return nil, err
	}

	// Audit log
	s.audit.LogMutation(ctx, userID, domain.AuditActionCreate, "organization", org.ID.String(), nil, org)

	// Set Casbin rule: user can do org:* actions in this org
	s.enforcer.AddRoleForUser(userID.String(), "owner", org.ID.String())

	s.log.Info("organization created", zap.String("org_id", org.ID.String()), zap.String("org_slug", org.Slug))

	return org, nil
}

// GetOrganization retrieves an organization
func (s *OrganizationService) GetOrganization(ctx context.Context, userID, orgID uuid.UUID) (*domain.Organization, error) {
	// Check permission
	if !s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "view") {
		return nil, errors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	return org, nil
}

// ListOrganizations lists user's organizations
func (s *OrganizationService) ListOrganizations(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Organization, int64, error) {
	// TODO: Query organizations where user is a member
	// For now, return all (implement scoping)
	return s.repo.FindAll(ctx, limit, offset)
}

// UpdateOrganization updates organization details
func (s *OrganizationService) UpdateOrganization(
	ctx context.Context,
	userID, orgID uuid.UUID,
	name, slug string,
	settings map[string]interface{},
) (*domain.Organization, error) {
	// Check permission
	if !s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "manage") {
		return nil, errors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Store original for audit
	original := *org

	// Update
	if name != "" {
		org.Name = name
	}
	if slug != "" {
		org.Slug = slug
	}
	if settings != nil {
		settingsJSON, _ := domain.NewJSONB(settings)
		org.Settings = settingsJSON
	}

	if err := s.repo.Update(ctx, org); err != nil {
		return nil, err
	}

	// Audit log
	s.audit.LogMutation(ctx, userID, domain.AuditActionUpdate, "organization", org.ID.String(), &original, org)

	return org, nil
}

// DeleteOrganization soft deletes organization
func (s *OrganizationService) DeleteOrganization(ctx context.Context, userID, orgID uuid.UUID) error {
	// Check permission
	if !s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "manage") {
		return errors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return err
	}

	if err := s.repo.SoftDelete(ctx, orgID); err != nil {
		return err
	}

	// Audit log
	s.audit.LogMutation(ctx, userID, domain.AuditActionDelete, "organization", org.ID.String(), org, nil)

	// Remove all Casbin rules for this org
	s.enforcer.RemoveFilteredPolicy(0, orgID.String())

	return nil
}

// AddMember adds a user to organization
func (s *OrganizationService) AddMember(
	ctx context.Context,
	userID, orgID, newMemberID uuid.UUID,
	role string,
) (*domain.OrganizationMember, error) {
	// Check permission
	if !s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "invite") {
		return nil, errors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// Validate role
	validRoles := map[string]bool{"owner": true, "admin": true, "member": true}
	if !validRoles[role] {
		return nil, errors.NewAppError("INVALID_INPUT", fmt.Sprintf("invalid role: %s", role), 400)
	}

	member := &domain.OrganizationMember{
		OrganizationID: orgID,
		UserID:         newMemberID,
		Role:           role,
	}

	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	// Update Casbin rules
	s.enforcer.AddRoleForUser(newMemberID.String(), role, orgID.String())

	// Audit log
	s.audit.LogMutation(ctx, userID, domain.AuditActionCreate, "organization_member", newMemberID.String(), nil, member)

	return member, nil
}

// RemoveMember removes a user from organization
func (s *OrganizationService) RemoveMember(ctx context.Context, userID, orgID, memberID uuid.UUID) error {
	// Check permission
	if !s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "remove") {
		return errors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	// Cannot remove owner
	org, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return err
	}

	if org.OwnerID == memberID {
		return errors.NewAppError("INVALID_OPERATION", "cannot remove organization owner", 400)
	}

	member, err := s.repo.FindMember(ctx, orgID, memberID)
	if err != nil {
		return err
	}

	if err := s.repo.RemoveMember(ctx, orgID, memberID); err != nil {
		return err
	}

	// Remove Casbin rules
	s.enforcer.RemoveRoleForUser(memberID.String(), member.Role, orgID.String())

	// Audit log
	s.audit.LogMutation(ctx, userID, domain.AuditActionDelete, "organization_member", memberID.String(), member, nil)

	return nil
}

// GetMembers lists organization members
func (s *OrganizationService) GetMembers(ctx context.Context, userID, orgID uuid.UUID, limit, offset int) ([]*domain.OrganizationMember, int64, error) {
	// Check permission
	if !s.enforcer.Enforce(userID.String(), orgID.String(), "organization", "view") {
		return nil, 0, errors.NewAppError("FORBIDDEN", "access denied", 403)
	}

	return s.repo.FindMembers(ctx, orgID, limit, offset)
}
```

---

## Phase 5: HTTP Handlers

### File: `internal/http/handler/organization.go`

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"go-api/internal/http/request"
	"go-api/internal/http/response"
	"go-api/internal/permission"
	"go-api/internal/service"
)

type OrganizationHandler struct {
	svc *service.OrganizationService
}

func NewOrganizationHandler(svc *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{svc: svc}
}

// CreateOrganization POST /organizations
// @Summary Create organization
// @Tags organizations
// @Accept json
// @Produce json
// @Param request body request.CreateOrganizationRequest true "Organization data"
// @Success 201 {object} response.Envelope
// @Router /organizations [post]
func (h *OrganizationHandler) CreateOrganization(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)

	var req request.CreateOrganizationRequest
	if err := c.BindAndValidate(&req); err != nil {
		return response.BadRequest(c, "invalid request")
	}

	org, err := h.svc.CreateOrganization(c.Request().Context(), userID, req.Name, req.Slug, req.Settings)
	if err != nil {
		return response.Error(c, err)
	}

	return response.Created(c, org.ToResponse())
}

// GetOrganization GET /organizations/:id
// @Summary Get organization
// @Tags organizations
// @Produce json
// @Param id path string true "Organization ID"
// @Success 200 {object} response.Envelope
// @Router /organizations/:id [get]
func (h *OrganizationHandler) GetOrganization(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "invalid organization id")
	}

	org, err := h.svc.GetOrganization(c.Request().Context(), userID, orgID)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, org.ToResponse())
}

// ListOrganizations GET /organizations
// @Summary List organizations
// @Tags organizations
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} response.Envelope
// @Router /organizations [get]
func (h *OrganizationHandler) ListOrganizations(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if offset < 0 {
		offset = 0
	}

	orgs, total, err := h.svc.ListOrganizations(c.Request().Context(), userID, limit, offset)
	if err != nil {
		return response.Error(c, err)
	}

	respOrgs := make([]*response.OrganizationResponse, len(orgs))
	for i, org := range orgs {
		respOrgs[i] = org.ToResponse()
	}

	return response.OKWithMeta(c, respOrgs, map[string]interface{}{
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// UpdateOrganization PUT /organizations/:id
// @Summary Update organization
// @Tags organizations
// @Accept json
// @Produce json
// @Param id path string true "Organization ID"
// @Param request body request.UpdateOrganizationRequest true "Organization data"
// @Success 200 {object} response.Envelope
// @Router /organizations/:id [put]
func (h *OrganizationHandler) UpdateOrganization(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "invalid organization id")
	}

	var req request.UpdateOrganizationRequest
	if err := c.BindAndValidate(&req); err != nil {
		return response.BadRequest(c, "invalid request")
	}

	org, err := h.svc.UpdateOrganization(c.Request().Context(), userID, orgID, req.Name, req.Slug, req.Settings)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, org.ToResponse())
}

// DeleteOrganization DELETE /organizations/:id
// @Summary Delete organization
// @Tags organizations
// @Param id path string true "Organization ID"
// @Success 204
// @Router /organizations/:id [delete]
func (h *OrganizationHandler) DeleteOrganization(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "invalid organization id")
	}

	if err := h.svc.DeleteOrganization(c.Request().Context(), userID, orgID); err != nil {
		return response.Error(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// AddMember POST /organizations/:id/members
// @Summary Add member to organization
// @Tags organization-members
// @Accept json
// @Produce json
// @Param id path string true "Organization ID"
// @Param request body request.AddMemberRequest true "Member data"
// @Success 201 {object} response.Envelope
// @Router /organizations/:id/members [post]
func (h *OrganizationHandler) AddMember(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "invalid organization id")
	}

	var req request.AddMemberRequest
	if err := c.BindAndValidate(&req); err != nil {
		return response.BadRequest(c, "invalid request")
	}

	member, err := h.svc.AddMember(c.Request().Context(), userID, orgID, req.UserID, req.Role)
	if err != nil {
		return response.Error(c, err)
	}

	return response.Created(c, member.ToResponse())
}

// RemoveMember DELETE /organizations/:id/members/:user_id
// @Summary Remove member from organization
// @Tags organization-members
// @Param id path string true "Organization ID"
// @Param user_id path string true "User ID"
// @Success 204
// @Router /organizations/:id/members/:user_id [delete]
func (h *OrganizationHandler) RemoveMember(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "invalid organization id")
	}

	memberID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		return response.BadRequest(c, "invalid user id")
	}

	if err := h.svc.RemoveMember(c.Request().Context(), userID, orgID, memberID); err != nil {
		return response.Error(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// ListMembers GET /organizations/:id/members
// @Summary List organization members
// @Tags organization-members
// @Produce json
// @Param id path string true "Organization ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} response.Envelope
// @Router /organizations/:id/members [get]
func (h *OrganizationHandler) ListMembers(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "invalid organization id")
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if offset < 0 {
		offset = 0
	}

	members, total, err := h.svc.GetMembers(c.Request().Context(), userID, orgID, limit, offset)
	if err != nil {
		return response.Error(c, err)
	}

	respMembers := make([]*response.OrganizationMemberResponse, len(members))
	for i, m := range members {
		respMembers[i] = m.ToResponse()
	}

	return response.OKWithMeta(c, respMembers, map[string]interface{}{
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
```

---

## Phase 6: Middleware & Context

### File: `internal/http/middleware/organization.go`

```go
package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"go-api/pkg/errors"
)

// OrganizationContext middleware extracts and validates organization context
func OrganizationContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get organization ID from header
		orgIDStr := c.Request().Header.Get("X-Organization-ID")
		if orgIDStr == "" {
			// Organization ID is optional for global operations
			return next(c)
		}

		// Validate UUID format
		orgID, err := uuid.Parse(orgIDStr)
		if err != nil {
			return echo.NewHTTPError(400, "invalid organization id format")
		}

		// Set in context
		c.Set("organization_id", orgID)

		return next(c)
	}
}

// RequireOrganization middleware requires organization context
func RequireOrganization(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		orgIDVal := c.Get("organization_id")
		if orgIDVal == nil {
			return echo.NewHTTPError(400, "X-Organization-ID header required")
		}

		return next(c)
	}
}

// ValidateMembership middleware validates user is org member
// Requires: organization_id in context, user_id in context
func ValidateMembership(repo interface{}) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userIDVal := c.Get("user_id")
			orgIDVal := c.Get("organization_id")

			if userIDVal == nil || orgIDVal == nil {
				return next(c)
			}

			userID := userIDVal.(uuid.UUID)
			orgID := orgIDVal.(uuid.UUID)

			// Check membership (simplified - implement actual check)
			// TODO: Query repo to verify user is member of organization
			// For now, just validate format

			c.Set("org_context", map[string]interface{}{
				"organization_id": orgID,
				"user_id":         userID,
			})

			return next(c)
		}
	}
}
```

---

## Phase 7: Request/Response Types

### File: `internal/http/request/organization.go`

```go
package request

import "github.com/google/uuid"

type CreateOrganizationRequest struct {
	Name     string                 `json:"name" validate:"required"`
	Slug     string                 `json:"slug" validate:"required"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

type UpdateOrganizationRequest struct {
	Name     string                 `json:"name,omitempty"`
	Slug     string                 `json:"slug,omitempty"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

type AddMemberRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Role   string    `json:"role" validate:"required,oneof=owner admin member"`
}
```

---

## Phase 8: Registration & Routing

### Update: `internal/http/server.go`

```go
// In setupRoutes()
api := e.Group("/api")
api.Use(authMiddleware, organizationContextMiddleware)

// Organization routes
orgHandler := handler.NewOrganizationHandler(orgService)
api.POST("/organizations", orgHandler.CreateOrganization)
api.GET("/organizations", orgHandler.ListOrganizations)
api.GET("/organizations/:id", orgHandler.GetOrganization)
api.PUT("/organizations/:id", orgHandler.UpdateOrganization)
api.DELETE("/organizations/:id", orgHandler.DeleteOrganization)

// Member routes
api.POST("/organizations/:id/members", orgHandler.AddMember)
api.DELETE("/organizations/:id/members/:user_id", orgHandler.RemoveMember)
api.GET("/organizations/:id/members", orgHandler.ListMembers)
```

---

## Phase 9: Integration Tests

### File: `tests/integration/organization_test.go`

```go
//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-api/internal/domain"
	"go-api/internal/repository"
)

func TestCreateOrganization(t *testing.T) {
	suite := NewTestSuite(t)
	suite.RunMigrations(t)
	suite.SetupTest(t)
	defer suite.Cleanup(t)

	ctx := context.Background()

	// Create test user
	user := &domain.User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	require.NoError(t, suite.DB.Create(user).Error)

	// Create organization
	org := &domain.Organization{
		Name:    "Test Org",
		Slug:    "test-org",
		OwnerID: user.ID,
	}

	require.NoError(t, suite.DB.Create(org).Error)

	// Verify
	var retrieved domain.Organization
	err := suite.DB.WithContext(ctx).Where("id = ?", org.ID).First(&retrieved).Error
	require.NoError(t, err)
	assert.Equal(t, "Test Org", retrieved.Name)
	assert.Equal(t, "test-org", retrieved.Slug)
	assert.Equal(t, user.ID, retrieved.OwnerID)
}

func TestOrganizationMembership(t *testing.T) {
	suite := NewTestSuite(t)
	suite.RunMigrations(t)
	suite.SetupTest(t)
	defer suite.Cleanup(t)

	ctx := context.Background()

	// Create users
	owner := &domain.User{Name: "Owner", Email: "owner@example.com"}
	member := &domain.User{Name: "Member", Email: "member@example.com"}
	require.NoError(t, suite.DB.Create([]*domain.User{owner, member}).Error)

	// Create org
	org := &domain.Organization{Name: "Test", Slug: "test", OwnerID: owner.ID}
	require.NoError(t, suite.DB.Create(org).Error)

	// Add member
	orgMember := &domain.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         member.ID,
		Role:           "member",
	}
	require.NoError(t, suite.DB.Create(orgMember).Error)

	// Verify membership
	var retrieved domain.OrganizationMember
	err := suite.DB.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", org.ID, member.ID).
		First(&retrieved).Error
	require.NoError(t, err)
	assert.Equal(t, "member", retrieved.Role)
}
```

---

## Phase 10: Permission Seeding

### Update: `cmd/api/main.go` (runSeed function)

```go
// Add organization permissions to seed
permissions := []struct {
	Resource string
	Actions  []string
}{
	{"organization", []string{"view", "manage", "invite", "remove"}},
	// ... existing permissions
}

for _, perm := range permissions {
	for _, action := range perm.Actions {
		err := permRepo.Create(ctx, &domain.Permission{
			Resource: perm.Resource,
			Action:   action,
		})
		if err != nil && !strings.Contains(err.Error(), "duplicate") {
			log.Fatal(err)
		}
	}
}
```

---

## Implementation Checklist

### Phase 1: Database
- [ ] Create migration file `migrations/000003_organizations.up.sql`
- [ ] Create rollback file `migrations/000003_organizations.down.sql`
- [ ] Run migrations: `make migrate`
- [ ] Verify schema in PostgreSQL

### Phase 2: Domain Models
- [ ] Create `internal/domain/organization.go`
- [ ] Add `Organization` entity with timestamps and soft delete
- [ ] Add `OrganizationMember` entity
- [ ] Add `ToResponse()` methods

### Phase 3: Repository
- [ ] Create `internal/repository/organization.go`
- [ ] Implement `OrganizationRepository` interface
- [ ] Implement member operations
- [ ] Test repository with integration tests

### Phase 4: Service Layer
- [ ] Create `internal/service/organization.go`
- [ ] Implement `OrganizationService`
- [ ] Add permission checks via enforcer
- [ ] Add audit logging for all mutations

### Phase 5: HTTP Handlers
- [ ] Create `internal/http/handler/organization.go`
- [ ] Implement CRUD handlers
- [ ] Implement member management handlers
- [ ] Add Swagger annotations

### Phase 6: Middleware
- [ ] Create `internal/http/middleware/organization.go`
- [ ] Implement `OrganizationContext` middleware
- [ ] Implement membership validation
- [ ] Register in server setup

### Phase 7: Requests/Responses
- [ ] Create request types
- [ ] Create response types
- [ ] Add validation rules

### Phase 8: Server Registration
- [ ] Register organization handler
- [ ] Add routes
- [ ] Register middleware

### Phase 9: Testing
- [ ] Write integration tests for CRUD
- [ ] Write tests for membership
- [ ] Write tests for permissions
- [ ] Achieve 80%+ coverage

### Phase 10: Documentation
- [ ] Add Swagger annotations
- [ ] Update API docs
- [ ] Document permission model
- [ ] Document header usage

---

## API Endpoints

```
POST   /api/organizations              Create organization
GET    /api/organizations              List user's organizations
GET    /api/organizations/:id          Get organization
PUT    /api/organizations/:id          Update organization
DELETE /api/organizations/:id          Delete organization

POST   /api/organizations/:id/members  Add member
GET    /api/organizations/:id/members  List members
DELETE /api/organizations/:id/members/:user_id  Remove member
```

---

## Permission Rules (Casbin)

```
# Organization owner/admin can manage their org
p, {user_id}, {org_id}, organization, view
p, {user_id}, {org_id}, organization, manage
p, {user_id}, {org_id}, organization, invite
p, {user_id}, {org_id}, organization, remove

# Role inheritance
g, {user_id}, owner, {org_id}
g, {user_id}, admin, {org_id}
g, {user_id}, member, {org_id}
```

---

## Success Criteria

- ✅ All entities created with UUID PK and soft deletes
- ✅ All operations logged to audit trail
- ✅ Permission checks enforced via Casbin
- ✅ Integration tests passing (80%+ coverage)
- ✅ API endpoints fully documented with Swagger
- ✅ No breaking changes to existing code
- ✅ X-Organization-ID header extraction working
- ✅ Organization scoping working for resources

---

## Timeline

**Day 1:**
- Database schema & migrations
- Domain models
- Repository layer
- Integration tests for data layer

**Day 2:**
- Service layer
- HTTP handlers
- Request/response types
- Permission system integration

**Day 3:**
- Middleware setup
- Server registration
- Full integration testing
- Documentation & Swagger
- Bug fixes & polish
