package request

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
	return validate.Struct(r)
}

// Validate validates the UpdateSystemSettingsRequest
func (r *UpdateSystemSettingsRequest) Validate() error {
	return validate.Struct(r)
}
