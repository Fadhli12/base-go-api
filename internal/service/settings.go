package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// SettingsService handles settings-related business logic
type SettingsService struct {
	userSettingsRepo   repository.UserSettingsRepository
	systemSettingsRepo repository.SystemSettingsRepository
	log                *slog.Logger
}

// NewSettingsService creates a new SettingsService instance
func NewSettingsService(
	userSettingsRepo repository.UserSettingsRepository,
	systemSettingsRepo repository.SystemSettingsRepository,
	log *slog.Logger,
) *SettingsService {
	return &SettingsService{
		userSettingsRepo:   userSettingsRepo,
		systemSettingsRepo: systemSettingsRepo,
		log:                log,
	}
}

// GetUserSettings retrieves user settings for a specific user in an organization
// Returns empty settings if not found (user should use defaults)
func (s *SettingsService) GetUserSettings(
	ctx context.Context,
	userID, orgID uuid.UUID,
) (*domain.UserSettingsResponse, error) {
	settings, err := s.userSettingsRepo.FindByUserIDAndOrgID(ctx, userID, orgID)
	if err != nil && !errors.IsNotFound(err) {
		s.log.Error("failed to get user settings",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	// Return empty response if not found
	if settings == nil {
		return &domain.UserSettingsResponse{
			ID:             uuid.New(),
			OrganizationID: orgID,
			UserID:         userID,
			Settings:       json.RawMessage(`{}`),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}, nil
	}

	resp := settings.ToResponse()
	return &resp, nil
}

// GetEffectiveSettings merges system and user settings, with user settings overriding system defaults
// Returns merged settings as a flat JSON object
func (s *SettingsService) GetEffectiveSettings(
	ctx context.Context,
	userID, orgID uuid.UUID,
) (json.RawMessage, error) {
	// Get system settings (with defaults as fallback)
	systemSettings, err := s.systemSettingsRepo.FindByOrgID(ctx, orgID)
	if err != nil && !errors.IsNotFound(err) {
		s.log.Error("failed to get system settings",
			slog.String("error", err.Error()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	var systemJSON map[string]interface{}
	if systemSettings == nil {
		// Use hardcoded defaults if no system settings exist
		defaults := domain.DefaultSystemSettings(orgID)
		err := json.Unmarshal(defaults.Settings, &systemJSON)
		if err != nil {
			s.log.Error("failed to parse default system settings",
				slog.String("error", err.Error()),
			)
			return nil, errors.WrapInternal(err)
		}
	} else {
		err := json.Unmarshal(systemSettings.Settings, &systemJSON)
		if err != nil {
			s.log.Error("failed to parse system settings",
				slog.String("error", err.Error()),
				slog.String("org_id", orgID.String()),
			)
			return nil, errors.WrapInternal(err)
		}
	}

	// Get user settings
	userSettings, err := s.userSettingsRepo.FindByUserIDAndOrgID(ctx, userID, orgID)
	if err != nil && !errors.IsNotFound(err) {
		s.log.Error("failed to get user settings",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	// Merge: user settings override system settings
	var userJSON map[string]interface{}
	if userSettings != nil && len(userSettings.Settings) > 0 {
		err := json.Unmarshal(userSettings.Settings, &userJSON)
		if err != nil {
			s.log.Error("failed to parse user settings",
				slog.String("error", err.Error()),
				slog.String("user_id", userID.String()),
				slog.String("org_id", orgID.String()),
			)
			return nil, errors.WrapInternal(err)
		}

		// Merge user into system (user takes precedence)
		for key, val := range userJSON {
			systemJSON[key] = val
		}
	}

	// Convert merged settings back to JSON
	merged, err := json.Marshal(systemJSON)
	if err != nil {
		s.log.Error("failed to marshal merged settings",
			slog.String("error", err.Error()),
		)
		return nil, errors.WrapInternal(err)
	}

	return json.RawMessage(merged), nil
}

// UpdateUserSettings updates user settings with validation
// Validates timezone if present in updates
func (s *SettingsService) UpdateUserSettings(
	ctx context.Context,
	userID, orgID uuid.UUID,
	updates map[string]interface{},
) (*domain.UserSettingsResponse, error) {
	// Validate timezone if present
	if timezone, ok := updates["timezone"]; ok {
		if tzStr, isString := timezone.(string); isString && tzStr != "" {
			if _, err := time.LoadLocation(tzStr); err != nil {
				s.log.Warn("invalid timezone provided",
					slog.String("timezone", tzStr),
					slog.String("user_id", userID.String()),
				)
				return nil, errors.NewAppError("VALIDATION_ERROR", "invalid timezone value", 422)
			}
		}
	}

	// Convert updates to JSONB
	settingsJSON, err := json.Marshal(updates)
	if err != nil {
		s.log.Error("failed to marshal user settings",
			slog.String("error", err.Error()),
		)
		return nil, errors.WrapInternal(err)
	}

	// Create or update user settings
	userSettings := &domain.UserSettings{
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(settingsJSON),
	}

	if err := s.userSettingsRepo.Upsert(ctx, userSettings); err != nil {
		s.log.Error("failed to update user settings",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	// Fetch updated settings to return
	updated, err := s.userSettingsRepo.FindByUserIDAndOrgID(ctx, userID, orgID)
	if err != nil {
		s.log.Error("failed to fetch updated user settings",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	resp := updated.ToResponse()
	return &resp, nil
}

// GetSystemSettings retrieves system settings for an organization
// Returns defaults if not found
func (s *SettingsService) GetSystemSettings(
	ctx context.Context,
	orgID uuid.UUID,
) (*domain.SystemSettingsResponse, error) {
	settings, err := s.systemSettingsRepo.FindByOrgID(ctx, orgID)
	if err != nil && !errors.IsNotFound(err) {
		s.log.Error("failed to get system settings",
			slog.String("error", err.Error()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	// Return defaults if not found
	if settings == nil {
		defaults := domain.DefaultSystemSettings(orgID)
		resp := defaults.ToResponse()
		return &resp, nil
	}

	resp := settings.ToResponse()
	return &resp, nil
}

// UpdateSystemSettings updates system settings with allowlist validation
// Only allows updates to specific configurable fields
func (s *SettingsService) UpdateSystemSettings(
	ctx context.Context,
	orgID uuid.UUID,
	updates map[string]interface{},
) (*domain.SystemSettingsResponse, error) {
	// Allowlist of configurable fields
	allowedFields := map[string]bool{
		"maintenance_mode":          true,
		"rate_limits":               true,
		"email_config":              true,
		"notifications_per_hour":    true,
		"requests_per_minute":       true,
		"from_address":              true,
		"from_name":                 true,
	}

	// Filter updates to only allowed fields
	filtered := make(map[string]interface{})
	for key, val := range updates {
		if allowedFields[key] {
			filtered[key] = val
		}
	}

	if len(filtered) == 0 {
		return nil, errors.NewAppError("VALIDATION_ERROR", "no valid fields to update", 422)
	}

	// Convert updates to JSONB
	settingsJSON, err := json.Marshal(filtered)
	if err != nil {
		s.log.Error("failed to marshal system settings",
			slog.String("error", err.Error()),
		)
		return nil, errors.WrapInternal(err)
	}

	// Create or update system settings
	systemSettings := &domain.SystemSettings{
		OrganizationID: orgID,
		Settings:       datatypes.JSON(settingsJSON),
	}

	if err := s.systemSettingsRepo.Upsert(ctx, systemSettings); err != nil {
		s.log.Error("failed to update system settings",
			slog.String("error", err.Error()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	// Fetch updated settings to return
	updated, err := s.systemSettingsRepo.FindByOrgID(ctx, orgID)
	if err != nil {
		s.log.Error("failed to fetch updated system settings",
			slog.String("error", err.Error()),
			slog.String("org_id", orgID.String()),
		)
		return nil, err
	}

	resp := updated.ToResponse()
	return &resp, nil
}
