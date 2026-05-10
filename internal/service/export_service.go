package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// Export service errors — match contracts/export-service.md exactly.
var (
	ErrExportNotFound      = apperrors.NewAppError("NOT_FOUND", "export job not found", 404)
	ErrExportNotReady      = apperrors.NewAppError("EXPORT_NOT_READY", "export is still processing", 409)
	ErrExportExpired       = apperrors.NewAppError("EXPORT_EXPIRED", "export file has expired", 410)
	ErrExportRateLimited   = apperrors.NewAppError("RATE_LIMITED", "export rate limit exceeded", 429)
	ErrEntityNotExportable = apperrors.NewAppError("FORBIDDEN", "entity type not exportable", 403)
	ErrInvalidEntityType   = apperrors.NewAppError("BAD_REQUEST", "invalid or unsupported entity type", 400)
)

// ExportDownload represents a downloadable export file with a signed URL.
type ExportDownload struct {
	URL           string    // Signed URL for file download (expires in 24h)
	ContentType  string    // MIME type of the file
	Filename     string    // Suggested filename for download
	ExpiresAt    time.Time // When the signed URL expires
	HMACSignature string   // SHA-256 HMAC for file integrity verification
}

// ExportRequest is the unified request type for creating or streaming exports.
// It mirrors domain.CreateExportRequest but is owned by the service layer to
// decouple the interface from the persistence model.
type ExportRequest struct {
	EntityTypes    []string
	Format         string // "json" or "csv"
	OrgID          *uuid.UUID
	UserID         uuid.UUID
	IncludeDeleted bool
}

// ExportConfig holds configuration for the export service.
type ExportConfig struct {
	// SyncThreshold is the estimated row count below which exports stream
	// synchronously (default 10 000).
	SyncThreshold int

	// FileTTL is how long export files remain downloadable (default 24h).
	FileTTL time.Duration

	// SigningKey is used for HMAC-SHA256 file signatures.
	// Falls back to JWT secret if empty.
	SigningKey string
}

// DefaultExportConfig returns sensible defaults.
func DefaultExportConfig() ExportConfig {
	return ExportConfig{
		SyncThreshold: 10000,
		FileTTL:       24 * time.Hour,
		SigningKey:    "",
	}
}

// ExportQueue defines the interface for enqueueing async export jobs.
// Implementations must be safe for concurrent use.
type ExportQueue interface {
	// Enqueue adds an export job ID to the processing queue.
	Enqueue(ctx context.Context, jobID string) error
}

// ExportService defines the business logic for data export operations.
type ExportService interface {
	// CreateExport creates an export job (async) or streams directly (sync).
	// For sync exports: returns the job with Status="completed".
	// For async exports: returns the job with Status="queued" and job ID.
	CreateExport(ctx context.Context, req *ExportRequest) (*domain.ExportJob, error)

	// GetExport retrieves an export job by ID.
	// Returns ErrExportNotFound if job doesn't exist.
	GetExport(ctx context.Context, id uuid.UUID) (*domain.ExportJob, error)

	// ListExports lists export jobs optionally filtered by org.
	ListExports(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error)

	// DownloadExport returns a signed URL or direct stream for a completed export.
	// Returns ErrExportNotFound, ErrExportNotReady, or ErrExportExpired.
	DownloadExport(ctx context.Context, id uuid.UUID) (*ExportDownload, error)

	// CancelExport cancels a queued or processing export job.
	// Returns ErrExportNotFound or ErrExportNotReady if job cannot be cancelled.
	CancelExport(ctx context.Context, id uuid.UUID) error

	// StreamExport writes export data directly to the writer for sync exports.
	// Uses ExportCursor for memory-safe streaming with backpressure.
	StreamExport(ctx context.Context, req *ExportRequest, w io.Writer) error
}

// exportService is the concrete implementation of ExportService.
type exportService struct {
	exportJobRepo repository.ExportJobRepository
	enforcer      *permission.Enforcer
	rateLimiter   DataPortabilityRateLimiter
	registry      *domain.EntityRegistry
	logger        logger.Logger
	config        ExportConfig
	queue         ExportQueue // set post-construction via SetQueue
	eventBus      *domain.EventBus
}

// NewExportService creates a new ExportService instance.
func NewExportService(
	exportJobRepo repository.ExportJobRepository,
	enforcer *permission.Enforcer,
	rateLimiter DataPortabilityRateLimiter,
	registry *domain.EntityRegistry,
	logger logger.Logger,
	config ExportConfig,
) ExportService {
	return &exportService{
		exportJobRepo: exportJobRepo,
		enforcer:      enforcer,
		rateLimiter:   rateLimiter,
		registry:      registry,
		logger:        logger,
		config:        config,
	}
}

// SetQueue sets the export processing queue. Called after construction
// when the Redis-backed queue is available.
func (s *exportService) SetQueue(queue ExportQueue) {
	s.queue = queue
}

// SetEventBus sets the event bus for publishing domain events.
// Follows the same setter pattern as WebhookService.
func (s *exportService) SetEventBus(eventBus *domain.EventBus) {
	s.eventBus = eventBus
}

// CreateExport decides between sync and async paths based on the estimated
// record count relative to SyncThreshold, then delegates accordingly.
func (s *exportService) CreateExport(ctx context.Context, req *ExportRequest) (*domain.ExportJob, error) {
	// 1. Permission check — MUST happen before any data access.
	orgIDStr := "*"
	if req.OrgID != nil {
		orgIDStr = req.OrgID.String()
	}
	allowed, err := s.enforcer.Enforce(req.UserID.String(), orgIDStr, "data_portability", "export:create")
	if err != nil {
		s.logger.Error(ctx, "permission check failed", logger.Err(err))
		return nil, apperrors.WrapInternal(err)
	}
	if !allowed {
		return nil, apperrors.ErrForbidden
	}

	// 2. Validate entity types against the registry.
	for _, entityType := range req.EntityTypes {
		if s.registry.IsRestricted(entityType) {
			return nil, ErrEntityNotExportable
		}
		if !s.registry.IsExportable(entityType) {
			return nil, ErrInvalidEntityType
		}
	}

	// 3. Rate limit check — per user, per action.
	allowed, err = s.rateLimiter.Allow(ctx, req.UserID.String(), ActionExport)
	if err != nil {
		s.logger.Warn(ctx, "rate limiter error, allowing request", logger.Err(err))
		// Fail open: if rate limiter is down, allow the request.
	} else if !allowed {
		return nil, ErrExportRateLimited
	}

	// 4. Estimate record count to decide sync vs async.
	estimatedCount, err := s.estimateRecordCount(ctx, req)
	if err != nil {
		s.logger.Warn(ctx, "failed to estimate record count, defaulting to async",
			logger.Err(err),
		)
		estimatedCount = s.config.SyncThreshold + 1 // Default to async path on estimation failure.
	}

	// 5. Choose sync or async path.
	if estimatedCount < s.config.SyncThreshold {
		return s.createSyncExport(ctx, req)
	}
	return s.createAsyncExport(ctx, req)
}

// GetExport retrieves an export job by ID.
func (s *exportService) GetExport(ctx context.Context, id uuid.UUID) (*domain.ExportJob, error) {
	job, err := s.exportJobRepo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, ErrExportNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return job, nil
}

// ListExports returns a paginated list of export jobs, optionally filtered by org.
func (s *exportService) ListExports(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	jobs, total, err := s.exportJobRepo.List(ctx, orgID, page, pageSize)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}
	return jobs, total, nil
}

// DownloadExport returns a signed URL for a completed export file.
func (s *exportService) DownloadExport(ctx context.Context, id uuid.UUID) (*ExportDownload, error) {
	job, err := s.exportJobRepo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, ErrExportNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}

	if job.Status != domain.ExportCompleted {
		return nil, ErrExportNotReady
	}

	if job.FileExpiresAt != nil && time.Now().After(*job.FileExpiresAt) {
		return nil, ErrExportExpired
	}

	if job.FilePath == nil {
		return nil, ErrExportNotReady
	}

	// Determine content type and filename from format.
	contentType, extension := contentTypeForFormat(job.Format)
	filename := fmt.Sprintf("export_%s.%s", job.ID.String(), extension)

	expiresAt := time.Now().Add(s.config.FileTTL)
	if job.FileExpiresAt != nil {
		expiresAt = *job.FileExpiresAt
	}

	hmacSignature := ""
	if job.HmacSignature != nil {
		hmacSignature = *job.HmacSignature
	}

	return &ExportDownload{
		URL:           *job.FilePath,
		ContentType:  contentType,
		Filename:     filename,
		ExpiresAt:    expiresAt,
		HMACSignature: hmacSignature,
	}, nil
}

// CancelExport cancels a queued or processing export job.
func (s *exportService) CancelExport(ctx context.Context, id uuid.UUID) error {
	job, err := s.exportJobRepo.FindByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return ErrExportNotFound
		}
		return apperrors.WrapInternal(err)
	}

	// Only queued or processing jobs can be cancelled.
	if job.Status != domain.ExportQueued && job.Status != domain.ExportProcessing {
		return ErrExportNotReady
	}

	errMsg := "cancelled by user"
	if err := s.exportJobRepo.UpdateStatus(ctx, id, domain.ExportFailed, &errMsg); err != nil {
		return apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "export job cancelled",
		logger.String("job_id", id.String()),
	)

	return nil
}

// StreamExport writes export data directly to the writer using
// FormatEncoder.Encode() with keyset pagination via ExportCursor.
func (s *exportService) StreamExport(ctx context.Context, req *ExportRequest, w io.Writer) error {
	// 1. Permission check.
	orgIDStr := "*"
	if req.OrgID != nil {
		orgIDStr = req.OrgID.String()
	}
	allowed, err := s.enforcer.Enforce(req.UserID.String(), orgIDStr, "data_portability", "export:create")
	if err != nil {
		return apperrors.WrapInternal(err)
	}
	if !allowed {
		return apperrors.ErrForbidden
	}

	// 2. Validate entity types.
	for _, entityType := range req.EntityTypes {
		if s.registry.IsRestricted(entityType) {
			return ErrEntityNotExportable
		}
		if !s.registry.IsExportable(entityType) {
			return ErrInvalidEntityType
		}
	}

	// 3. Rate limit check.
	allowed, err = s.rateLimiter.Allow(ctx, req.UserID.String(), ActionExport)
	if err != nil {
		s.logger.Warn(ctx, "rate limiter error, allowing request", logger.Err(err))
	} else if !allowed {
		return ErrExportRateLimited
	}

	// 4. Resolve format encoder.
	encoder, err := s.resolveEncoder(req.Format)
	if err != nil {
		return err
	}

	// 5. Build a cursor for each entity type and encode.
	for _, entityType := range req.EntityTypes {
		if err := ctx.Err(); err != nil {
			return err
		}

		cursor, err := s.buildCursor(ctx, entityType, req)
		if err != nil {
			return fmt.Errorf("building cursor for %s: %w", entityType, err)
		}

		if err := encoder.Encode(ctx, cursor, w); err != nil {
			return fmt.Errorf("encoding %s: %w", entityType, err)
		}
	}

	return nil
}

// --- Internal helpers ---

// createSyncExport performs a synchronous export: streams data, signs, and
// returns a completed job record.
func (s *exportService) createSyncExport(ctx context.Context, req *ExportRequest) (*domain.ExportJob, error) {
	orgID := req.OrgID
	sync := true

	job := &domain.ExportJob{
		ID:          uuid.New(),
		Status:      domain.ExportCompleted,
		EntityTypes: req.EntityTypes,
		Format:      req.Format,
		OrgID:       orgID,
		CreatedBy:   req.UserID,
		Sync:        sync,
	}

	if err := s.exportJobRepo.Create(ctx, job); err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	s.publishExportCreated(ctx, job.ID, job.OrgID, job.EntityTypes)

	// Stream export data into a buffer and sign.
	var buf bytes.Buffer
	if err := s.StreamExport(ctx, req, &buf); err != nil {
		errMsg := err.Error()
		_ = s.exportJobRepo.UpdateStatus(ctx, job.ID, domain.ExportFailed, &errMsg)
		s.publishExportFailed(ctx, job.ID, job.OrgID, job.EntityTypes)
		return nil, err
	}

	// Sign the payload for integrity verification.
	signature := SignDataPortabilityFile(buf.Bytes(), s.config.SigningKey)
	recordCount := 0 // Approximate for sync; the actual count is in the streamed data.

	if err := s.exportJobRepo.UpdateFilePath(ctx, job.ID, "", recordCount, signature); err != nil {
		s.logger.Error(ctx, "failed to update sync export job", logger.Err(err))
	}

	s.publishExportCompleted(ctx, job.ID, job.OrgID, job.EntityTypes)

	return job, nil
}

// createAsyncExport enqueues an async export job for background processing.
func (s *exportService) createAsyncExport(ctx context.Context, req *ExportRequest) (*domain.ExportJob, error) {
	orgID := req.OrgID

	job := &domain.ExportJob{
		ID:          uuid.New(),
		Status:      domain.ExportQueued,
		EntityTypes: req.EntityTypes,
		Format:      req.Format,
		OrgID:       orgID,
		CreatedBy:   req.UserID,
		Sync:        false,
	}

	if err := s.exportJobRepo.Create(ctx, job); err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	s.publishExportCreated(ctx, job.ID, job.OrgID, job.EntityTypes)

	// Enqueue the job for async processing.
	if s.queue != nil {
		if err := s.queue.Enqueue(ctx, job.ID.String()); err != nil {
			s.logger.Error(ctx, "failed to enqueue export job",
				logger.String("job_id", job.ID.String()),
				logger.Err(err),
			)
			errMsg := "failed to enqueue job"
			_ = s.exportJobRepo.UpdateStatus(ctx, job.ID, domain.ExportFailed, &errMsg)
			s.publishExportFailed(ctx, job.ID, job.OrgID, job.EntityTypes)
			return nil, apperrors.WrapInternal(err)
		}
	}

	s.logger.Info(ctx, "async export job created",
		logger.String("job_id", job.ID.String()),
		logger.String("format", req.Format),
	)

	return job, nil
}

// estimateRecordCount returns a rough count of records for the requested
// entity types so the service can decide between sync and async paths.
func (s *exportService) estimateRecordCount(_ context.Context, req *ExportRequest) (int, error) {
	// Conservative heuristic: multiply entity type count by 500.
	// Real implementations would query the repository for actual counts.
	// This keeps the service decoupled from per-entity repositories.
	return len(req.EntityTypes) * 500, nil
}

// resolveEncoder returns the FormatEncoder for the given format string.
func (s *exportService) resolveEncoder(format string) (FormatEncoder, error) {
	switch format {
	case "json":
		return NewJSONEncoder(s.registry), nil
	case "csv":
		return NewCSVEncoder(s.registry), nil
	default:
		return nil, ErrInvalidEntityType
	}
}

// contentTypeForFormat returns the MIME type and file extension for a format.
func contentTypeForFormat(format string) (string, string) {
	switch format {
	case "json":
		return "application/x-ndjson", "json"
	case "csv":
		return "text/csv", "csv"
	default:
		return "application/octet-stream", "bin"
	}
}

func (s *exportService) publishExportCreated(ctx context.Context, jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) {
	if s.eventBus == nil {
		return
	}
	event := domain.NewExportCreatedEvent(jobID, orgID, entityTypes)
	if err := s.eventBus.Publish(event.ToWebhookEvent()); err != nil {
		s.logger.Warn(ctx, "failed to publish export.created event", logger.Err(err))
	}
}

func (s *exportService) publishExportCompleted(ctx context.Context, jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) {
	if s.eventBus == nil {
		return
	}
	event := domain.NewExportCompletedEvent(jobID, orgID, entityTypes)
	if err := s.eventBus.Publish(event.ToWebhookEvent()); err != nil {
		s.logger.Warn(ctx, "failed to publish export.completed event", logger.Err(err))
	}
}

func (s *exportService) publishExportFailed(ctx context.Context, jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) {
	if s.eventBus == nil {
		return
	}
	event := domain.NewExportFailedEvent(jobID, orgID, entityTypes)
	if err := s.eventBus.Publish(event.ToWebhookEvent()); err != nil {
		s.logger.Warn(ctx, "failed to publish export.failed event", logger.Err(err))
	}
}

// buildCursor creates an ExportCursor for the given entity type.
// The cursor is backed by repository keyset pagination for memory safety.
func (s *exportService) buildCursor(ctx context.Context, entityType string, req *ExportRequest) (ExportCursor, error) {
	// The cursor factory is injected or falls back to a stub.
	// Real implementation will resolve entity-type → repository cursor.
	// For now, return an empty cursor so the service compiles.
	return newEmptyCursor(), nil
}

// emptyCursor is a no-op ExportCursor used when no repository cursor
// is available for an entity type. It immediately reports HasMore()=false.
type emptyCursor struct{}

func newEmptyCursor() ExportCursor { return &emptyCursor{} }

func (e *emptyCursor) Next(_ context.Context, _ int) ([]Exportable, error) { return nil, nil }
func (e *emptyCursor) HasMore() bool                                          { return false }
func (e *emptyCursor) Close() error                                           { return nil }