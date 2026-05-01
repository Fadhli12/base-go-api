package request

// UpdateNotificationPreferenceRequest represents a request to create or update a notification preference
type UpdateNotificationPreferenceRequest struct {
	NotificationType string `json:"notification_type" validate:"required,oneof=mention assignment system invoice.created news.published"`
	EmailEnabled     *bool  `json:"email_enabled" validate:"required"`
	PushEnabled      *bool  `json:"push_enabled" validate:"required"`
}

// Validate validates the UpdateNotificationPreferenceRequest struct
func (r *UpdateNotificationPreferenceRequest) Validate() error {
	return validate.Struct(r)
}
