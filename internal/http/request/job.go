package request

import "github.com/go-playground/validator/v10"

var jobValidate = validator.New()

type JobInput struct {
	Type       string
	Payload    map[string]interface{}
	MaxRetries int
	WebhookURL string
}

type SubmitJobRequest struct {
	Type       string                 `json:"type" validate:"required,min=1,max=255"`
	Payload    map[string]interface{} `json:"payload"`
	MaxRetries int                    `json:"max_retries,omitempty" validate:"gte=0,lte=10"`
	WebhookURL string                 `json:"webhook_url,omitempty" validate:"omitempty,url"`
}

func (r *SubmitJobRequest) Validate() error {
	return jobValidate.Struct(r)
}

func (r *SubmitJobRequest) ToJobInput() *JobInput {
	input := &JobInput{
		Type: r.Type,
	}

	if r.Payload != nil {
		input.Payload = r.Payload
	} else {
		input.Payload = make(map[string]interface{})
	}

	input.MaxRetries = r.MaxRetries
	input.WebhookURL = r.WebhookURL

	return input
}
