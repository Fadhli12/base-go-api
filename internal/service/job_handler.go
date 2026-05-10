package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/example/go-api-base/internal/config"
)

// JobHandler dispatches jobs to their registered handlers.
type JobHandler interface {
	// Handle executes the job with the given payload.
	// Returns result JSON bytes on success, or error on failure.
	// Never retries internally - let the queue handle retries.
	Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error)

	// SupportedTypes returns the list of job types this handler supports.
	SupportedTypes() []string
}

// jobHandlerRegistry maps job types to their handlers.
type jobHandlerRegistry struct {
	handlers map[string]JobHandler
	log      *slog.Logger
}

// newJobHandlerRegistry creates a new handler registry.
func newJobHandlerRegistry(log *slog.Logger) *jobHandlerRegistry {
	return &jobHandlerRegistry{
		handlers: make(map[string]JobHandler),
		log:      log,
	}
}

// Register adds a handler for a specific job type.
func (r *jobHandlerRegistry) Register(jobType string, handler JobHandler) {
	r.handlers[jobType] = handler
	r.log.Info("registered job handler",
		slog.String("job_type", jobType),
		slog.Int("supported_types", len(handler.SupportedTypes())),
	)
}

// Handle dispatches to the appropriate handler.
// Returns an error if no handler is registered for the job type.
func (r *jobHandlerRegistry) Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error) {
	handler, ok := r.handlers[jobType]
	if !ok {
		return nil, ErrJobHandlerNotFound(jobType)
	}

	return handler.Handle(ctx, jobType, payload)
}

// SupportedTypes returns all registered job types.
func (r *jobHandlerRegistry) SupportedTypes() []string {
	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}

// JobHandlerService coordinates job handlers for the queue system.
type JobHandlerService struct {
	registry *jobHandlerRegistry
	config   config.JobConfig
	log      *slog.Logger
}

// NewJobHandlerService creates a new job handler service with the given config.
func NewJobHandlerService(cfg config.JobConfig, log *slog.Logger) *JobHandlerService {
	return &JobHandlerService{
		registry: newJobHandlerRegistry(log),
		config:   cfg,
		log:      log,
	}
}

// Register adds a handler for a specific job type.
func (s *JobHandlerService) Register(jobType string, handler JobHandler) {
	s.registry.Register(jobType, handler)
}

// Handle dispatches to the appropriate handler for the job type.
func (s *JobHandlerService) Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error) {
	return s.registry.Handle(ctx, jobType, payload)
}

// SupportedTypes returns all registered job types.
func (s *JobHandlerService) SupportedTypes() []string {
	return s.registry.SupportedTypes()
}

// JobHandlerServiceInterface abstracts the JobHandlerService for dependency injection.
// This allows mock implementations in unit tests.
type JobHandlerServiceInterface interface {
	Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error)
	SupportedTypes() []string
	Register(jobType string, handler JobHandler)
}

// Compile-time assertion that *JobHandlerService implements JobHandlerServiceInterface.
var _ JobHandlerServiceInterface = (*JobHandlerService)(nil)

// NoopHandler is a test handler that accepts "test.nop" jobs.
// It logs the job and returns an empty result.
type NoopHandler struct {
	log *slog.Logger
}

// NewNoopHandler creates a new NoopHandler.
func NewNoopHandler(log *slog.Logger) *NoopHandler {
	return &NoopHandler{log: log}
}

// Handle logs the job and returns empty result.
func (h *NoopHandler) Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error) {
	h.log.Info("executing noop job",
		slog.String("job_type", jobType),
		slog.Int("payload_size", len(payload)),
	)
	return []byte("{}"), nil
}

// SupportedTypes returns ["test.nop"].
func (h *NoopHandler) SupportedTypes() []string {
	return []string{"test.nop"}
}

// EchoHandler is a test handler that accepts "test.echo" jobs.
// It returns the payload as JSON.
type EchoHandler struct {
	log *slog.Logger
}

// NewEchoHandler creates a new EchoHandler.
func NewEchoHandler(log *slog.Logger) *EchoHandler {
	return &EchoHandler{log: log}
}

// Handle returns the payload as JSON.
func (h *EchoHandler) Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error) {
	h.log.Info("executing echo job",
		slog.String("job_type", jobType),
		slog.Int("payload_size", len(payload)),
	)

	result, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// SupportedTypes returns ["test.echo"].
func (h *EchoHandler) SupportedTypes() []string {
	return []string{"test.echo"}
}

// ErrJobHandlerNotFound is returned when no handler is registered for a job type.
func ErrJobHandlerNotFound(jobType string) error {
	return &jobHandlerError{jobType: jobType, message: "no handler registered for job type: " + jobType}
}

type jobHandlerError struct {
	jobType string
	message string
}

func (e *jobHandlerError) Error() string {
	return e.message
}

// EmailJobHandler handles "email.send" jobs.
type EmailJobHandler struct {
	emailSvc *EmailService
	log      *slog.Logger
}

// NewEmailJobHandler creates a new EmailJobHandler.
func NewEmailJobHandler(emailSvc *EmailService, log *slog.Logger) *EmailJobHandler {
	return &EmailJobHandler{
		emailSvc: emailSvc,
		log:      log,
	}
}

func (h *EmailJobHandler) SupportedTypes() []string {
	return []string{"email.send"}
}

func (h *EmailJobHandler) Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error) {
	if jobType != "email.send" {
		return nil, ErrJobHandlerNotFound(jobType)
	}

	h.log.Info("processing email.send job",
		slog.Int("payload_size", len(payload)),
	)

	to, ok := payload["to"].(string)
	if !ok {
		return nil, fmt.Errorf("email.send: 'to' field required")
	}
	subject, _ := payload["subject"].(string)
	body, _ := payload["body"].(string)
	provider, _ := payload["provider"].(string)

	if provider == "" {
		provider = "sendgrid"
	}

	result, err := h.emailSvc.Send(ctx, to, subject, body, provider)
	if err != nil {
		return nil, fmt.Errorf("email.send failed: %w", err)
	}

	return json.Marshal(result)
}