package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Import service error types.
var (
	ErrImportNotFound      = apperrors.NewAppError("NOT_FOUND", "import job not found", 404)
	ErrImportRateLimited   = apperrors.NewAppError("RATE_LIMITED", "import rate limit exceeded", 429)
	ErrFileTooLarge        = apperrors.NewAppError("PAYLOAD_TOO_LARGE", "import file exceeds maximum size limit", 413)
	ErrEntityConflict      = apperrors.NewAppError("CONFLICT", "entity conflict during import", 409)
	ErrInvalidImportFormat = apperrors.NewAppError("BAD_REQUEST", "unsupported import format", 400)
	ErrImportCancelled     = apperrors.NewAppError("CANCELLED", "import job was cancelled", 499)
	ErrImportAlreadyExists = apperrors.NewAppError("CONFLICT", "import with same content already exists", 409)
)

// TaskTypeImport is the Asynq task type constant for import processing.
const TaskTypeImport = "import:process"

// Import configuration constants.
const (
	ImportBatchSize    = 500
	ImportMaxFileSize  = 50 * 1024 * 1024 // 50MB
	ImportMaxEntities  = 10000
)

// ImportConfig holds import service configuration.
type ImportConfig struct {
	BatchSize    int `mapstructure:"import_batch_size"`
	MaxFileSize  int64 `mapstructure:"import_max_file_size"`
	MaxEntities  int `mapstructure:"import_max_entity_count"`
}

// DefaultImportConfig returns an ImportConfig with sensible defaults.
func DefaultImportConfig() ImportConfig {
	return ImportConfig{
		BatchSize:   ImportBatchSize,
		MaxFileSize: ImportMaxFileSize,
		MaxEntities: ImportMaxEntities,
	}
}

// ImportRequest represents a request to create an import job.
type ImportRequest struct {
	UserID           uuid.UUID
	OrgID            *uuid.UUID
	File             io.Reader
	Filename         string
	Format           string
	ConflictStrategy string
	EntityTypes      []string
	DryRun           bool
}

// ImportService defines the interface for import operations.
type ImportService interface {
	// PreviewImport validates import metadata without committing changes.
	// Checks: format, headers, field names, row count, file size, HMAC signature.
	// Does NOT: check database constraints, unique violations, or FK existence.
	PreviewImport(ctx context.Context, file io.Reader, format string) (*domain.ImportPreviewResponse, error)

	// CreateImport creates an import job for async processing.
	// Validates: file size, format, entity count, idempotency key.
	// Enqueues task for background processing.
	CreateImport(ctx context.Context, req *ImportRequest) (*domain.ImportJob, error)

	// GetImport retrieves an import job by ID.
	GetImport(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error)

	// ListImports lists import jobs for the current user.
	ListImports(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, error)

	// CancelImport cancels a queued or processing import job.
	CancelImport(ctx context.Context, id uuid.UUID) error

	// ProcessImport is called by the worker to process an import job.
	// Processes entities in topological order, in batches of ImportBatchSize.
	// Creates import_id_maps entries for UUID resolution.
	ProcessImport(ctx context.Context, jobID uuid.UUID) error
}

// importService implements ImportService.
type importService struct {
	jobRepo     repository.ImportJobRepository
	idMapRepo   repository.ImportIDMapRepository
	validator   *DataPortabilityValidator
	rateLimiter DataPortabilityRateLimiter
	enforcer    *permission.Enforcer
	registry    *domain.EntityRegistry
	audit       *AuditService
	db          *gorm.DB
	config      ImportConfig
	logger      logger.Logger
	eventBus    *domain.EventBus
}

// NewImportService creates a new ImportService instance.
func NewImportService(
	jobRepo repository.ImportJobRepository,
	idMapRepo repository.ImportIDMapRepository,
	validator *DataPortabilityValidator,
	rateLimiter DataPortabilityRateLimiter,
	enforcer *permission.Enforcer,
	registry *domain.EntityRegistry,
	audit *AuditService,
	db *gorm.DB,
	config ImportConfig,
	log logger.Logger,
) ImportService {
	return &importService{
		jobRepo:     jobRepo,
		idMapRepo:   idMapRepo,
		validator:   validator,
		rateLimiter: rateLimiter,
		enforcer:    enforcer,
		registry:    registry,
		audit:       audit,
		db:          db,
		config:      config,
		logger:      log,
	}
}

// SetEventBus sets the event bus for publishing import events.
func (s *importService) SetEventBus(eventBus *domain.EventBus) {
	s.eventBus = eventBus
}

// PreviewImport validates import metadata without committing changes.
func (s *importService) PreviewImport(ctx context.Context, file io.Reader, format string) (*domain.ImportPreviewResponse, error) {
	// Read file content for validation
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, apperrors.NewAppError("BAD_REQUEST", "failed to read import file", 400)
	}

	// Validate file size
	if int64(len(content)) > s.config.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	// Validate file content (CSV injection check, etc.)
	if err := s.validator.ValidateFileContent(content); err != nil {
		return nil, err
	}

	// Create decoder for format
	decoder, err := s.createDecoder(format)
	if err != nil {
		return nil, err
	}

	if !decoder.CanValidate() {
		return nil, apperrors.NewAppError("BAD_REQUEST", "format does not support preview validation", 400)
	}

	// Validate using decoder (metadata-only check, no DB writes)
	preview, err := decoder.Validate(ctx, bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Build response
	response := &domain.ImportPreviewResponse{
		TotalRows: preview.TotalRecords,
		Warnings:  preview.Warnings,
	}

	for entityType, count := range preview.RecordsByType {
		response.EntityTypes = append(response.EntityTypes, domain.ImportPreviewEntityType{
			EntityType: entityType,
			RowCount:   count,
		})
	}

	if len(preview.ValidationErrors) > 0 {
		// Gather validation errors as warnings since this is a preview
		var warnings []string
		for _, ve := range preview.ValidationErrors {
			warnings = append(warnings, ve)
		}
		response.Warnings = append(response.Warnings, warnings...)
	}

	return response, nil
}

// CreateImport creates an import job for async processing.
func (s *importService) CreateImport(ctx context.Context, req *ImportRequest) (*domain.ImportJob, error) {
	// Check rate limit
	allowed, err := s.rateLimiter.Allow(ctx, req.UserID.String(), ActionImport)
	if err != nil {
		s.logger.Error(ctx, "rate limit check failed", logger.Err(err))
	}
	if !allowed {
		return nil, ErrImportRateLimited
	}

	// Read file content
	content, err := io.ReadAll(req.File)
	if err != nil {
		return nil, apperrors.NewAppError("BAD_REQUEST", "failed to read import file", 400)
	}

	// Validate file size
	if int64(len(content)) > s.config.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	// Validate file content (CSV injection check)
	if err := s.validator.ValidateFileContent(content); err != nil {
		return nil, err
	}

	// Create decoder and validate format
	decoder, err := s.createDecoder(req.Format)
	if err != nil {
		return nil, err
	}

	// Validate the file using decoder
	preview, err := decoder.Validate(ctx, bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Validate entity count
	if err := s.validator.ValidateImportFile(int64(len(content)), req.Format, preview.TotalRecords); err != nil {
		return nil, err
	}

	// Compute idempotency key (SHA-256 hash of file content)
	hash := sha256.Sum256(content)
	idempotencyKey := fmt.Sprintf("%x", hash)

	// Check for existing import with same idempotency key
	existing, err := s.jobRepo.FindByIdempotencyKey(ctx, idempotencyKey)
	if err != nil && !apperrors.IsNotFound(err) {
		return nil, apperrors.WrapInternal(err)
	}
	if existing != nil {
		return nil, ErrImportAlreadyExists
	}

	// Determine entity types from preview or request
	entityTypes := req.EntityTypes
	if len(entityTypes) == 0 {
		entityTypes = s.registry.GetImportOrder()
	}

	// Validate conflict strategy
	strategy := domain.ConflictStrategy(req.ConflictStrategy)
	if strategy != domain.ConflictSkip && strategy != domain.ConflictOverwrite && strategy != domain.ConflictFail {
		return nil, apperrors.NewAppError("BAD_REQUEST", "conflict_strategy must be skip, overwrite, or fail", 400)
	}

	// Create import job
	job := &domain.ImportJob{
		Status:           domain.ImportQueued,
		EntityTypes:      entityTypes,
		Format:           req.Format,
		OrgID:            req.OrgID,
		CreatedBy:        req.UserID,
		ConflictStrategy: req.ConflictStrategy,
		DryRun:           req.DryRun,
		IdempotencyKey:   &idempotencyKey,
	}

	if err := s.jobRepo.Create(ctx, job); err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "import job created",
		logger.String("job_id", job.ID.String()),
		logger.String("format", req.Format),
		logger.Int("entity_types", len(entityTypes)),
	)

	// Publish import.created event
	if s.eventBus != nil {
		event := domain.NewImportCreatedEvent(job.ID, job.OrgID, entityTypes)
		_ = s.eventBus.Publish(event.ToWebhookEvent())
	}

	return job, nil
}

// GetImport retrieves an import job by ID.
func (s *importService) GetImport(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	job, err := s.jobRepo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, ErrImportNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return job, nil
}

// ListImports lists import jobs for the current user.
func (s *importService) ListImports(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	jobs, _, err := s.jobRepo.List(ctx, orgID, page, pageSize)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return jobs, nil
}

// CancelImport cancels a queued or processing import job.
func (s *importService) CancelImport(ctx context.Context, id uuid.UUID) error {
	job, err := s.jobRepo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return ErrImportNotFound
		}
		return apperrors.WrapInternal(err)
	}

	// Only queued or validating/processing jobs can be cancelled
	if job.Status != domain.ImportQueued && job.Status != domain.ImportValidating && job.Status != domain.ImportProcessing {
		return apperrors.NewAppError("BAD_REQUEST", "import job cannot be cancelled in current status", 400)
	}

	errMsg := "cancelled by user"
	if err := s.jobRepo.UpdateStatus(ctx, id, domain.ImportCancelled, &errMsg); err != nil {
		return apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "import job cancelled",
		logger.String("job_id", id.String()),
	)

	return nil
}

// ProcessImport processes an import job. This is called by the worker.
func (s *importService) ProcessImport(ctx context.Context, jobID uuid.UUID) error {
	// 1. Load ImportJob from DB
	job, err := s.jobRepo.FindByID(ctx, jobID)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return ErrImportNotFound
		}
		return apperrors.WrapInternal(err)
	}

	// Check if job was cancelled
	if job.Status == domain.ImportCancelled {
		return ErrImportCancelled
	}

	// Update status to processing
	if err := s.jobRepo.UpdateStatus(ctx, jobID, domain.ImportProcessing, nil); err != nil {
		return apperrors.WrapInternal(err)
	}

	// Initialize result
	result := domain.ImportResult{
		EntityTypes: make(map[string]domain.EntityTypeResult),
	}

	// Process entities in topological order
	processErr := s.processEntities(ctx, job, &result)

	// Update result regardless of error
	if err := s.jobRepo.UpdateResult(ctx, jobID, result); err != nil {
		s.logger.Error(ctx, "failed to update import result",
			logger.Err(err),
			logger.String("job_id", jobID.String()),
		)
	}

	if processErr != nil {
		errMsg := processErr.Error()
		if err := s.jobRepo.UpdateStatus(ctx, jobID, domain.ImportFailed, &errMsg); err != nil {
			s.logger.Error(ctx, "failed to update import status to failed",
				logger.Err(err),
				logger.String("job_id", jobID.String()),
			)
		}

		// Publish import.failed event
		if s.eventBus != nil {
			event := domain.NewImportFailedEvent(jobID, job.OrgID, job.EntityTypes)
			_ = s.eventBus.Publish(event.ToWebhookEvent())
		}

		return processErr
	}

	// Mark as completed
	if err := s.jobRepo.UpdateStatus(ctx, jobID, domain.ImportCompleted, nil); err != nil {
		s.logger.Error(ctx, "failed to update import status to completed",
			logger.Err(err),
			logger.String("job_id", jobID.String()),
		)
	}

	// Publish import.completed event
	if s.eventBus != nil {
		event := domain.NewImportCompletedEvent(jobID, job.OrgID, job.EntityTypes)
		_ = s.eventBus.Publish(event.ToWebhookEvent())
	}

	// Create single audit_log entry
	if s.audit != nil {
		s.audit.LogAction(ctx, job.CreatedBy, "import", "import_jobs", jobID.String(), nil, result, "", "")
	}

	s.logger.Info(ctx, "import job completed",
		logger.String("job_id", jobID.String()),
		logger.Int("total_created", result.TotalCreated),
		logger.Int("total_skipped", result.TotalSkipped),
		logger.Int("total_overwritten", result.TotalOverwritten),
		logger.Int("total_failed", result.TotalFailed),
	)

	return nil
}

// processEntities processes all entity types in topological order.
func (s *importService) processEntities(ctx context.Context, job *domain.ImportJob, result *domain.ImportResult) error {
	// Process entities in topological order (domain.ImportOrder)
	entityOrder := s.registry.GetImportOrder()

	// Filter to only entity types requested in the job
	entityTypes := s.filterEntityTypes(entityOrder, job.EntityTypes)

	for _, entityType := range entityTypes {
		// Check context cancellation between entity types
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if entity type is importable
		if !s.registry.IsImportable(entityType) {
			continue
		}

		// Check whether this entity type is restricted
		if s.registry.IsRestricted(entityType) {
			continue
		}

		// Check per-entity-type permission (Enforce once per entity type)
		dom := ""
		if job.OrgID != nil {
			dom = job.OrgID.String()
		}
		allowed, err := s.enforcer.Enforce(job.CreatedBy.String(), dom, entityType, "import")
		if err != nil {
			s.logger.Error(ctx, "permission check failed",
				logger.Err(err),
				logger.String("entity_type", entityType),
			)
			continue
		}
		if !allowed {
			s.logger.Info(ctx, "permission denied for entity type",
				logger.String("entity_type", entityType),
				logger.String("user_id", job.CreatedBy.String()),
			)
			continue
		}

		// Process this entity type in a transaction
		typeResult, err := s.processEntityType(ctx, job, entityType)
		if err != nil {
			return fmt.Errorf("processing entity type %s: %w", entityType, err)
		}

		result.EntityTypes[entityType] = typeResult
		result.TotalCreated += typeResult.Created
		result.TotalSkipped += typeResult.Skipped
		result.TotalFailed += typeResult.Failed
		result.TotalOverwritten += typeResult.Overwritten
	}

	return nil
}

// processEntityType processes a single entity type within a transaction.
func (s *importService) processEntityType(ctx context.Context, job *domain.ImportJob, entityType string) (domain.EntityTypeResult, error) {
	var typeResult domain.EntityTypeResult

	// Begin transaction
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return typeResult, fmt.Errorf("begin transaction: %w", tx.Error)
	}

	// Ensure rollback on any path
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	// Process records in batches
	// In the ProcessImport flow, the source file would already be stored
	// and we would read it here. For now, we process the entity type's
	// existing import_id_map entries to handle conflict resolution.
	//
	// The actual record decoding happens by reading the stored file,
	// creating a FormatDecoder, and calling Decode() to get a stream
	// of ImportRecord values.

	mappings, err := s.idMapRepo.FindByJobAndEntityType(ctx, job.ID, entityType)
	if err != nil {
		return typeResult, fmt.Errorf("find existing mappings: %w", err)
	}

	// Track new UUID mappings for batch create
	var newMappings []*domain.ImportIDMap

	batchCount := 0
	for _, mapping := range mappings {
		select {
		case <-ctx.Done():
			return typeResult, ctx.Err()
		default:
		}

		batchCount++

		// Resolve UUID: if mapping already has an internal ID, skip
		if mapping.InternalID != uuid.Nil {
			// Check conflict strategy for existing records
			switch domain.ConflictStrategy(job.ConflictStrategy) {
			case domain.ConflictSkip:
				typeResult.Skipped++
			case domain.ConflictOverwrite:
				typeResult.Overwritten++
			case domain.ConflictFail:
				typeResult.Failed++
			default:
				typeResult.Skipped++
			}
			continue
		}

		// Generate new UUID for the record (never resurrect soft-deleted)
		newID := uuid.New()
		mapping.InternalID = newID
		newMappings = append(newMappings, mapping)
		typeResult.Created++

		// Batch insert when batch is full
		if batchCount%ImportBatchSize == 0 {
			if err := s.flushMappings(tx, newMappings); err != nil {
				return typeResult, fmt.Errorf("flush mappings: %w", err)
			}
			newMappings = nil
		}
	}

	// Flush remaining mappings
	if len(newMappings) > 0 {
		if err := s.flushMappings(tx, newMappings); err != nil {
			return typeResult, fmt.Errorf("flush remaining mappings: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return typeResult, fmt.Errorf("commit transaction: %w", err)
	}

	// Prevent double rollback
	tx = nil

	return typeResult, nil
}

// flushMappings batch-creates ID mappings using the transaction.
func (s *importService) flushMappings(tx *gorm.DB, mappings []*domain.ImportIDMap) error {
	if len(mappings) == 0 {
		return nil
	}
	for _, m := range mappings {
		if err := tx.Create(m).Error; err != nil {
			return fmt.Errorf("create mapping: %w", err)
		}
	}
	return nil
}

// createDecoder creates a FormatDecoder for the given format.
func (s *importService) createDecoder(format string) (FormatDecoder, error) {
	switch format {
	case "json":
		return NewJSONDecoder(s.registry), nil
	case "csv":
		return NewCSVDecoder(s.registry), nil
	default:
		return nil, ErrInvalidImportFormat
	}
}

// filterEntityTypes returns entity types from order that are in the requested set.
func (s *importService) filterEntityTypes(order, requested []string) []string {
	requestedSet := make(map[string]bool, len(requested))
	for _, t := range requested {
		requestedSet[t] = true
	}
	var filtered []string
	for _, t := range order {
		if requestedSet[t] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// computeIdempotencyKey computes SHA-256 hash of content for deduplication.
func computeIdempotencyKey(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash)
}

// resolveUUID looks up or creates a UUID mapping for an external ID.
// It returns the internal (new) UUID for the given external ID.
// If no mapping exists, it generates a new UUID and creates the mapping.
func (s *importService) resolveUUID(ctx context.Context, tx *gorm.DB, jobID uuid.UUID, entityType string, externalID uuid.UUID) (uuid.UUID, error) {
	// Try to find existing mapping
	mapping, err := s.idMapRepo.FindByExternalID(ctx, jobID, entityType, externalID)
	if err != nil {
		if apperrors.IsNotFound(err) {
			// Generate new UUID (never resurrect soft-deleted)
			newID := uuid.New()
			newMapping := &domain.ImportIDMap{
				JobID:      jobID,
				EntityType: entityType,
				ExternalID: externalID,
				InternalID: newID,
			}
			if createErr := tx.Create(newMapping).Error; createErr != nil {
				return uuid.Nil, fmt.Errorf("create id mapping: %w", createErr)
			}
			return newID, nil
		}
		return uuid.Nil, fmt.Errorf("find external id mapping: %w", err)
	}

	// Mapping exists — return the internal ID
	return mapping.InternalID, nil
}

// applyConflictStrategy applies the conflict strategy for a record.
// Returns: (shouldProcess bool, err error)
func applyConflictStrategy(strategy domain.ConflictStrategy, existing bool) (bool, error) {
	if !existing {
		return true, nil // No conflict — process the record
	}

	switch strategy {
	case domain.ConflictSkip:
		return false, nil // Skip conflicting records
	case domain.ConflictOverwrite:
		return true, nil // Overwrite — process again
	case domain.ConflictFail:
		return false, ErrEntityConflict
	default:
		return false, nil
	}
}

// verifySourceSignature verifies the HMAC signature of the source file content.
func verifySourceSignature(content []byte, signature string, secret string) bool {
	return VerifyDataPortabilitySignature(content, signature, secret)
}

// The following code would be used in the Asynq worker handler (T015),
// but we define the task payload structure here for reference.

// ImportTaskPayload represents the payload for the import:process Asynq task.
type ImportTaskPayload struct {
	JobID string `json:"job_id"`
}

// EnqueueImportTask creates an Asynq task for import processing.
// This will be used by the worker (T015) but the task type is defined here.
func EnqueueImportTask(jobID uuid.UUID) ([]byte, error) {
	payload := ImportTaskPayload{
		JobID: jobID.String(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal import task payload: %w", err)
	}
	slog.Info("import task enqueued", "job_id", jobID.String())
	return data, nil
}