package request

import (
	"encoding/json"
	"fmt"
	"regexp"

	apperrors "github.com/example/go-api-base/pkg/errors"
)

var featureFlagKeyRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type CreateFeatureFlagRequest struct {
	Key         string                 `json:"key" validate:"required,min=1,max=100"`
	Name        string                 `json:"name" validate:"required,min=1,max=255"`
	Description string                 `json:"description"`
	Enabled     *bool                  `json:"enabled"`
	Rollout     int                    `json:"rollout"`
	Conditions  map[string]interface{} `json:"conditions"`
}

type UpdateFeatureFlagRequest struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	Enabled     *bool                  `json:"enabled"`
	Rollout     *int                   `json:"rollout"`
	Conditions  map[string]interface{} `json:"conditions"`
}

func (r *CreateFeatureFlagRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if !featureFlagKeyRegex.MatchString(r.Key) {
		return apperrors.NewAppError("VALIDATION_ERROR",
			"key must match pattern ^[a-z][a-z0-9_]*$ (lowercase, starts with letter, underscores allowed)", 422)
	}
	if r.Rollout < 0 || r.Rollout > 100 {
		return apperrors.NewAppError("VALIDATION_ERROR", "rollout must be between 0 and 100", 422)
	}
	if r.Conditions != nil {
		if err := validateConditions(r.Conditions); err != nil {
			return err
		}
	}
	return nil
}

func (r *UpdateFeatureFlagRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	if r.Rollout != nil {
		if *r.Rollout < 0 || *r.Rollout > 100 {
			return apperrors.NewAppError("VALIDATION_ERROR", "rollout must be between 0 and 100", 422)
		}
	}
	if r.Conditions != nil {
		if err := validateConditions(r.Conditions); err != nil {
			return err
		}
	}
	return nil
}

func validateConditions(conditions map[string]interface{}) error {
	for key, value := range conditions {
		switch key {
		case "user_ids", "org_ids", "envs":
			arr, ok := value.([]interface{})
			if !ok {
				return apperrors.NewAppError("VALIDATION_ERROR",
					fmt.Sprintf("conditions.%s must be an array", key), 422)
			}
			for _, item := range arr {
				if _, ok := item.(string); !ok {
					return apperrors.NewAppError("VALIDATION_ERROR",
						fmt.Sprintf("conditions.%s must be an array of strings", key), 422)
				}
			}
		default:
			return apperrors.NewAppError("VALIDATION_ERROR",
				fmt.Sprintf("unknown condition key: %s (allowed: user_ids, org_ids, envs)", key), 422)
		}
	}
	return nil
}

func MarshalConditions(conditions map[string]interface{}) (json.RawMessage, error) {
	if conditions == nil {
		return nil, nil
	}
	data, err := json.Marshal(conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}
	return data, nil
}