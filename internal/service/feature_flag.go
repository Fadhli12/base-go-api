package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// keyRegex validates feature flag keys: must start with lowercase letter,
// followed by lowercase letters, digits, or underscores.
var keyRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FeatureFlagService handles feature flag business logic including
// CRUD operations, RBAC enforcement, evaluation, and audit logging.
type FeatureFlagService struct {
	repo     repository.FeatureFlagRepository
	enforcer *permission.Enforcer
	audit    *AuditService
	log      *slog.Logger
}

// NewFeatureFlagService creates a new FeatureFlagService instance.
func NewFeatureFlagService(
	repo repository.FeatureFlagRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	log *slog.Logger,
) *FeatureFlagService {
	return &FeatureFlagService{
		repo:     repo,
		enforcer: enforcer,
		audit:    audit,
		log:      log,
	}
}

// resolveOrgDomain returns the Casbin domain string for RBAC enforcement.
// If an organization context is provided, it uses the org ID; otherwise "default".
func resolveOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	if hasOrgID && orgID != uuid.Nil {
		return orgID.String()
	}
	return "default"
}

// Create creates a new feature flag with RBAC manage permission.
func (s *FeatureFlagService) Create(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	req request.CreateFeatureFlagRequest,
	ipAddress string,
	userAgent string,
) (*domain.FeatureFlagResponse, error) {
	orgDomain := resolveOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage feature flags", 403)
	}

	// Validate key format
	if !keyRegex.MatchString(req.Key) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "feature flag key must match pattern ^[a-z][a-z0-9_]*$", 422)
	}

	// Validate rollout percentage
	if req.Rollout < 0 || req.Rollout > 100 {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "rollout must be between 0 and 100", 422)
	}

	var conditions datatypes.JSON
	if req.Conditions != nil {
		conditionsBytes, err := json.Marshal(req.Conditions)
		if err != nil {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid conditions format", 422)
		}
		conditions = conditionsBytes
	}

	// Check key uniqueness
	existing, err := s.repo.FindByKey(ctx, req.Key)
	if err != nil && !apperrors.IsNotFound(err) {
		s.log.Error("failed to check feature flag key uniqueness",
			slog.String("error", err.Error()),
			slog.String("key", req.Key),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if existing != nil {
		return nil, apperrors.NewAppError("CONFLICT", "feature flag with key already exists", 409)
	}

	// Default enabled to false if not specified
	enabled := false
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	flag := &domain.FeatureFlag{
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		Enabled:     enabled,
		Rollout:     req.Rollout,
		Conditions:  conditions,
		IsSystem:    false,
	}

	if err := s.repo.Create(ctx, flag); err != nil {
		if isDuplicateKeyError(err) {
			return nil, apperrors.NewAppError("CONFLICT", "feature flag with key already exists", 409)
		}
		s.log.Error("failed to create feature flag",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log
	afterJSON, _ := json.Marshal(flag.ToResponse())
	s.audit.LogAction(ctx, userID, domain.AuditActionCreate, "feature_flag", flag.ID.String(), nil, afterJSON, ipAddress, userAgent)

	resp := flag.ToResponse()
	return &resp, nil
}

// GetByID retrieves a feature flag by ID with RBAC view permission.
func (s *FeatureFlagService) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
) (*domain.FeatureFlagResponse, error) {
	orgDomain := resolveOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view feature flags", 403)
	}

	flag, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		s.log.Error("failed to get feature flag by ID",
			slog.String("error", err.Error()),
			slog.String("id", id.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	resp := flag.ToResponse()
	return &resp, nil
}

// GetByKey retrieves a feature flag by key with RBAC view permission.
func (s *FeatureFlagService) GetByKey(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	key string,
) (*domain.FeatureFlagResponse, error) {
	orgDomain := resolveOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view feature flags", 403)
	}

	flag, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		s.log.Error("failed to get feature flag by key",
			slog.String("error", err.Error()),
			slog.String("key", key),
		)
		return nil, apperrors.WrapInternal(err)
	}

	resp := flag.ToResponse()
	return &resp, nil
}

// List retrieves feature flags with pagination and RBAC view permission.
func (s *FeatureFlagService) List(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	limit int,
	offset int,
) ([]*domain.FeatureFlagResponse, int64, error) {
	orgDomain := resolveOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "view")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, 0, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, 0, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to view feature flags", 403)
	}

	flags, total, err := s.repo.FindAll(ctx, limit, offset)
	if err != nil {
		s.log.Error("failed to list feature flags",
			slog.String("error", err.Error()),
		)
		return nil, 0, apperrors.WrapInternal(err)
	}

	responses := make([]*domain.FeatureFlagResponse, len(flags))
	for i, flag := range flags {
		resp := flag.ToResponse()
		responses[i] = &resp
	}

	return responses, total, nil
}

// Update modifies an existing feature flag with RBAC manage permission.
// Only non-nil fields in the request are applied.
func (s *FeatureFlagService) Update(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
	req request.UpdateFeatureFlagRequest,
	ipAddress string,
	userAgent string,
) (*domain.FeatureFlagResponse, error) {
	orgDomain := resolveOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage feature flags", 403)
	}

	// Validate rollout if provided
	if req.Rollout != nil && (*req.Rollout < 0 || *req.Rollout > 100) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "rollout must be between 0 and 100", 422)
	}

	flag, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		s.log.Error("failed to find feature flag for update",
			slog.String("error", err.Error()),
			slog.String("id", id.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Capture before state for audit
	beforeJSON, _ := json.Marshal(flag.ToResponse())

	// Merge updates (only non-nil fields)
	if req.Name != nil {
		flag.Name = *req.Name
	}
	if req.Description != nil {
		flag.Description = *req.Description
	}
	if req.Enabled != nil {
		flag.Enabled = *req.Enabled
	}
	if req.Rollout != nil {
		flag.Rollout = *req.Rollout
	}
	if req.Conditions != nil {
		conditionsBytes, err := json.Marshal(req.Conditions)
		if err != nil {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid conditions format", 422)
		}
		flag.Conditions = conditionsBytes
	}

	if err := s.repo.Update(ctx, flag); err != nil {
		s.log.Error("failed to update feature flag",
			slog.String("error", err.Error()),
			slog.String("id", id.String()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log with before and after state
	afterJSON, _ := json.Marshal(flag.ToResponse())
	s.audit.LogAction(ctx, userID, domain.AuditActionUpdate, "feature_flag", flag.ID.String(), beforeJSON, afterJSON, ipAddress, userAgent)

	resp := flag.ToResponse()
	return &resp, nil
}

// Delete soft-deletes a feature flag with RBAC manage permission.
// System feature flags cannot be deleted.
func (s *FeatureFlagService) Delete(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	id uuid.UUID,
	ipAddress string,
	userAgent string,
) error {
	orgDomain := resolveOrgDomain(hasOrgID, orgID)
	allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "manage")
	if err != nil {
		s.log.Error("failed to enforce permission",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return apperrors.WrapInternal(err)
	}
	if !allowed {
		return apperrors.NewAppError("FORBIDDEN", "insufficient permissions to manage feature flags", 403)
	}

	flag, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return apperrors.ErrNotFound
		}
		s.log.Error("failed to find feature flag for delete",
			slog.String("error", err.Error()),
			slog.String("id", id.String()),
		)
		return apperrors.WrapInternal(err)
	}

	// Prevent deletion of system feature flags
	if flag.IsSystem {
		return apperrors.NewAppError("FORBIDDEN", "system feature flags cannot be deleted", 403)
	}

	// Capture before state for audit
	beforeJSON, _ := json.Marshal(flag.ToResponse())

	if err := s.repo.SoftDelete(ctx, id); err != nil {
		s.log.Error("failed to delete feature flag",
			slog.String("error", err.Error()),
			slog.String("id", id.String()),
			slog.String("user_id", userID.String()),
		)
		return apperrors.WrapInternal(err)
	}

	// Audit log — before state only, after is null
	s.audit.LogAction(ctx, userID, domain.AuditActionDelete, "feature_flag", id.String(), beforeJSON, nil, ipAddress, userAgent)

	return nil
}

// IsEnabled evaluates a feature flag for a given user, returning a boolean.
// This method requires authentication but no RBAC permission check.
// It fails open to false (disabled) on any error or if the flag is not found.
func (s *FeatureFlagService) IsEnabled(
	ctx context.Context,
	key string,
	userID uuid.UUID,
) bool {
	flag, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		// Flag not found or error — fail open to false
		return false
	}

	if !flag.Enabled {
		return false
	}

	// Full rollout
	if flag.Rollout >= 100 {
		return evaluateConditions(flag, userID, uuid.Nil)
	}

	// Zero rollout
	if flag.Rollout <= 0 {
		return false
	}

	// Percentage-based rollout using FNV-1a hash
	if fnvHash(userID.String()+key)%100 < uint32(flag.Rollout) {
		return evaluateConditions(flag, userID, uuid.Nil)
	}

	// Rollout did not match
	return false
}

// IsEnabledWithReason evaluates a feature flag and returns the evaluation result
// with a human-readable reason. Requires authentication but no RBAC permission.
func (s *FeatureFlagService) IsEnabledWithReason(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	key string,
) (*domain.FeatureFlagEvaluation, error) {
	flag, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return &domain.FeatureFlagEvaluation{
				Key:     key,
				Enabled: false,
				Reason:  "flag_not_found",
				Rollout: 0,
			}, nil
		}
		s.log.Error("failed to evaluate feature flag",
			slog.String("error", err.Error()),
			slog.String("key", key),
		)
		return nil, apperrors.WrapInternal(err)
	}

	evaluation := evaluateFlag(flag, userID, hasOrgID, orgID)
	return &evaluation, nil
}

// EvaluateAll evaluates all feature flags for a user and returns bulk results.
// Requires authentication but no RBAC permission.
func (s *FeatureFlagService) EvaluateAll(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
) (*domain.BulkEvaluateResponse, error) {
	flags, _, err := s.repo.FindAll(ctx, 0, 0)
	if err != nil {
		s.log.Error("failed to fetch feature flags for evaluation",
			slog.String("error", err.Error()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	evaluations := make([]domain.FeatureFlagEvaluation, 0, len(flags))
	for _, flag := range flags {
		evaluation := evaluateFlag(flag, userID, hasOrgID, orgID)
		evaluations = append(evaluations, evaluation)
	}

	return &domain.BulkEvaluateResponse{
		Flags: evaluations,
	}, nil
}

// evaluateFlag contains the core evaluation logic that determines whether
// a feature flag is enabled for a given user, with a reason string.
func evaluateFlag(flag *domain.FeatureFlag, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID) domain.FeatureFlagEvaluation {
	if !flag.Enabled {
		return domain.FeatureFlagEvaluation{
			Key:     flag.Key,
			Enabled: false,
			Reason:  "flag_disabled",
			Rollout: flag.Rollout,
		}
	}

	// Full rollout
	if flag.Rollout >= 100 {
		if evaluateConditions(flag, userID, orgID) {
			return domain.FeatureFlagEvaluation{
				Key:     flag.Key,
				Enabled: true,
				Reason:  "rollout_100",
				Rollout: flag.Rollout,
			}
		}
		return domain.FeatureFlagEvaluation{
			Key:     flag.Key,
			Enabled: false,
			Reason:  "condition_no_match",
			Rollout: flag.Rollout,
		}
	}

	// Zero rollout
	if flag.Rollout <= 0 {
		return domain.FeatureFlagEvaluation{
			Key:     flag.Key,
			Enabled: false,
			Reason:  "rollout_0",
			Rollout: flag.Rollout,
		}
	}

	// Percentage-based rollout
	if fnvHash(userID.String()+flag.Key)%100 < uint32(flag.Rollout) {
		if evaluateConditions(flag, userID, orgID) {
			return domain.FeatureFlagEvaluation{
				Key:     flag.Key,
				Enabled: true,
				Reason:  "rollout_match",
				Rollout: flag.Rollout,
			}
		}
		return domain.FeatureFlagEvaluation{
			Key:     flag.Key,
			Enabled: false,
			Reason:  "condition_no_match",
			Rollout: flag.Rollout,
		}
	}

	// Rollout did not match
	return domain.FeatureFlagEvaluation{
		Key:     flag.Key,
		Enabled: false,
		Reason:  "rollout_no_match",
		Rollout: flag.Rollout,
	}
}

// evaluateConditions checks the conditions JSONB field of a feature flag.
// Conditions can contain user_ids, org_ids, and envs lists.
// If conditions are empty/nil, returns true (no conditions to check).
// If conditions are present and match, returns true.
// If conditions are present but don't match, returns false.
func evaluateConditions(flag *domain.FeatureFlag, userID uuid.UUID, orgID uuid.UUID) bool {
	if len(flag.Conditions) == 0 {
		return true
	}

	var conditions map[string]interface{}
	if err := json.Unmarshal(flag.Conditions, &conditions); err != nil {
		// Invalid conditions JSON — fail open to no match
		return false
	}

	if len(conditions) == 0 {
		return true
	}

	// Check user_ids condition
	if userIDs, ok := conditions["user_ids"].([]interface{}); ok && len(userIDs) > 0 {
		userIDStr := userID.String()
		for _, uid := range userIDs {
			if s, ok := uid.(string); ok && s == userIDStr {
				return true
			}
		}
		// user_ids present but user not in list — condition fails
		return false
	}

	// Check org_ids condition
	if orgIDs, ok := conditions["org_ids"].([]interface{}); ok && len(orgIDs) > 0 && orgID != uuid.Nil {
		orgIDStr := orgID.String()
		for _, oid := range orgIDs {
			if s, ok := oid.(string); ok && s == orgIDStr {
				return true
			}
		}
		// org_ids present but org not in list — condition fails
		return false
	}

	// Check envs condition
	if envs, ok := conditions["envs"].([]interface{}); ok && len(envs) > 0 {
		// For now, check against "production" as default env
		// This can be extended to check against config/env
		currentEnv := "production"
		for _, env := range envs {
			if s, ok := env.(string); ok && s == currentEnv {
				return true
			}
		}
		// envs present but current env not in list — condition fails
		return false
	}

	// Conditions exist but none of the specific condition types matched
	return false
}

// fnvHash implements the FNV-1a hash algorithm for consistent percentage-based rollout.
func fnvHash(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for _, c := range s {
		h ^= uint32(c)
		h *= prime32
	}
	return h
}