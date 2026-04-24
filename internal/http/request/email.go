package request

// CreateTemplateRequest represents an email template creation request
type CreateTemplateRequest struct {
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Subject     string `json:"subject" validate:"required,min=1,max=255"`
	HTMLContent string `json:"html_content" validate:"required"`
	TextContent string `json:"text_content" validate:"omitempty"`
	Category    string `json:"category" validate:"required,oneof=transactional marketing notification system"`
}

// UpdateTemplateRequest represents an email template update request
type UpdateTemplateRequest struct {
	Name        string `json:"name" validate:"omitempty,min=3,max=100"`
	Subject     string `json:"subject" validate:"omitempty,min=1,max=255"`
	HTMLContent string `json:"html_content" validate:"omitempty"`
	TextContent string `json:"text_content" validate:"omitempty"`
	Category    string `json:"category" validate:"omitempty,oneof=transactional marketing notification system"`
	IsActive    *bool  `json:"is_active" validate:"omitempty"`
}

// SendEmailRequest represents an email sending request
type SendEmailRequest struct {
	To          string                 `json:"to" validate:"required,email"`
	Subject     string                 `json:"subject" validate:"required_without=Template,max=500"`
	Template    string                 `json:"template" validate:"required_without=Subject,max=100"`
	Data        map[string]interface{} `json:"data" validate:"omitempty"`
	HTMLContent string                 `json:"html_content" validate:"omitempty"`
	TextContent string                 `json:"text_content" validate:"omitempty"`
}

// Validate validates the request struct using go-playground/validator
func (r *CreateTemplateRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *UpdateTemplateRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *SendEmailRequest) Validate() error {
	return validate.Struct(r)
}