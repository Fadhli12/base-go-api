package request

import (
	"fmt"
	"strings"

	apperrors "github.com/example/go-api-base/pkg/errors"
)

var userSettingsAllowedKeys = map[string]string{
	"theme":                  "string",
	"language":               "string",
	"timezone":               "string",
	"notifications_enabled":  "bool",
	"email_digest_enabled":   "bool",
	"compact_mode":           "bool",
	"sidebar_collapsed":      "bool",
	"default_project_id":     "string",
	"date_format":            "string",
	"time_format":            "string",
}

var systemSettingsAllowedKeys = map[string]string{
	"maintenance_mode":       "bool",
	"rate_limits":            "object",
	"email_config":           "object",
	"notifications_per_hour": "number",
	"requests_per_minute":    "number",
	"from_address":           "string",
	"from_name":              "string",
}

// UpdateUserSettingsRequest represents a request to update user settings
type UpdateUserSettingsRequest struct {
	Settings map[string]interface{} `json:"settings" validate:"required"`
}

// UpdateSystemSettingsRequest represents a request to update system settings
type UpdateSystemSettingsRequest struct {
	Settings map[string]interface{} `json:"settings" validate:"required"`
}

// Validate validates the UpdateUserSettingsRequest
func (r *UpdateUserSettingsRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return validateSettingsContent(r.Settings, userSettingsAllowedKeys, "user")
}

// Validate validates the UpdateSystemSettingsRequest
func (r *UpdateSystemSettingsRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	return validateSettingsContent(r.Settings, systemSettingsAllowedKeys, "system")
}

func validateSettingsContent(settings map[string]interface{}, allowedKeys map[string]string, settingsType string) error {
	var unknownKeys []string
	var typeErrors []string

	for key, value := range settings {
		expectedType, keyAllowed := allowedKeys[key]
		if !keyAllowed {
			unknownKeys = append(unknownKeys, key)
			continue
		}

		if err := validateSettingValueType(key, value, expectedType); err != nil {
			typeErrors = append(typeErrors, err.Error())
		}
	}

	if len(unknownKeys) > 0 {
		return apperrors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("unknown %s settings keys: %s", settingsType, strings.Join(unknownKeys, ", ")),
			422)
	}

	if len(typeErrors) > 0 {
		return apperrors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("type validation errors: %s", strings.Join(typeErrors, "; ")),
			422)
	}

	return nil
}

func validateSettingValueType(key string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("key %q must be a string", key)
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("key %q must be a boolean", key)
		}
	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32:
			// valid number types from JSON unmarshaling
		default:
			return fmt.Errorf("key %q must be a number", key)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("key %q must be an object", key)
		}
	}
	return nil
}