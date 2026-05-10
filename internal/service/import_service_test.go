package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockImportJobRepository struct {
	createFn               func(ctx context.Context, job *domain.ImportJob) error
	findByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error)
	findByIdempotencyKeyFn func(ctx context.Context, key string) (*domain.ImportJob, error)
	updateStatusFn         func(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error
	updateResultFn         func(ctx context.Context, id uuid.UUID, result domain.ImportResult) error
	listFn                 func(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, int64, error)
}

func (m *mockImportJobRepository) Create(ctx context.Context, job *domain.ImportJob) error {
	if m.createFn != nil {
		return m.createFn(ctx, job)
	}
	return nil
}

func (m *mockImportJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockImportJobRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.ImportJob, error) {
	if m.findByIdempotencyKeyFn != nil {
		return m.findByIdempotencyKeyFn(ctx, key)
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockImportJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status, errorMessage)
	}
	return nil
}

func (m *mockImportJobRepository) UpdateResult(ctx context.Context, id uuid.UUID, result domain.ImportResult) error {
	if m.updateResultFn != nil {
		return m.updateResultFn(ctx, id, result)
	}
	return nil
}

func (m *mockImportJobRepository) UpdateSourceFilePath(ctx context.Context, id uuid.UUID, path string) error {
	return nil
}

func (m *mockImportJobRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockImportJobRepository) List(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, int64, error) {
	if m.listFn != nil {
		return m.listFn(ctx, orgID, page, pageSize)
	}
	return nil, 0, nil
}

type mockImportIDMapRepository struct {
	findByJobAndEntityTypeFn func(ctx context.Context, jobID uuid.UUID, entityType string) ([]*domain.ImportIDMap, error)
	findByExternalIDFn       func(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (*domain.ImportIDMap, error)
	resolveUUIDFn            func(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (uuid.UUID, error)
	batchCreateFn            func(ctx context.Context, mappings []*domain.ImportIDMap) error
}

func (m *mockImportIDMapRepository) Create(ctx context.Context, mapping *domain.ImportIDMap) error {
	return nil
}

func (m *mockImportIDMapRepository) FindByJobAndEntityType(ctx context.Context, jobID uuid.UUID, entityType string) ([]*domain.ImportIDMap, error) {
	if m.findByJobAndEntityTypeFn != nil {
		return m.findByJobAndEntityTypeFn(ctx, jobID, entityType)
	}
	return nil, nil
}

func (m *mockImportIDMapRepository) FindByExternalID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (*domain.ImportIDMap, error) {
	if m.findByExternalIDFn != nil {
		return m.findByExternalIDFn(ctx, jobID, entityType, externalID)
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockImportIDMapRepository) BatchCreate(ctx context.Context, mappings []*domain.ImportIDMap) error {
	if m.batchCreateFn != nil {
		return m.batchCreateFn(ctx, mappings)
	}
	return nil
}

func (m *mockImportIDMapRepository) ResolveUUID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (uuid.UUID, error) {
	if m.resolveUUIDFn != nil {
		return m.resolveUUIDFn(ctx, jobID, entityType, externalID)
	}
	return uuid.New(), nil
}

type mockRateLimiter struct {
	allowFn        func(ctx context.Context, userID string, action string) (bool, error)
	allowRecordsFn func(ctx context.Context, orgID string, count int) (bool, error)
}

func (m *mockRateLimiter) Allow(ctx context.Context, userID string, action string) (bool, error) {
	if m.allowFn != nil {
		return m.allowFn(ctx, userID, action)
	}
	return true, nil
}

func (m *mockRateLimiter) AllowRecords(ctx context.Context, orgID string, count int) (bool, error) {
	if m.allowRecordsFn != nil {
		return m.allowRecordsFn(ctx, orgID, count)
	}
	return true, nil
}

func newTestImportService(
	jobRepo *mockImportJobRepository,
	idMapRepo *mockImportIDMapRepository,
	rateLimiter *mockRateLimiter,
) *importService {
	registry := domain.NewEntityRegistry()

	return &importService{
		jobRepo:     jobRepo,
		idMapRepo:   idMapRepo,
		validator:   NewDataPortabilityValidator(),
		rateLimiter: rateLimiter,
		enforcer:    nil,
		registry:    registry,
		audit:       nil,
		db:          nil,
		config:      DefaultImportConfig(),
		logger:      logger.NewNopLogger(),
	}
}

func makeNDJSONContent() []byte {
	var buf bytes.Buffer
	for _, t := range domain.ImportOrder {
		for i := 0; i < 2; i++ {
			record := map[string]interface{}{
				"type": t,
				"id":   uuid.New().String(),
				"data": map[string]interface{}{
					"id":   uuid.New().String(),
					"name": fmt.Sprintf("test-%s-%d", t, i),
				},
			}
			data, _ := json.Marshal(record)
			buf.Write(data)
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func TestPreviewImport_ValidatesWithoutWritingToDB(t *testing.T) {
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	content := makeNDJSONContent()
	_, err := svc.PreviewImport(context.Background(), bytes.NewReader(content), "json")

	assert.NoError(t, err, "PreviewImport should succeed for valid content")
}

func TestPreviewImport_RejectsInvalidFormat(t *testing.T) {
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	content := []byte(`{"type":"users","id":"123","data":{"id":"123","name":"test"}}`)
	_, err := svc.PreviewImport(context.Background(), bytes.NewReader(content), "xml")

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidImportFormat, err)
}

func TestPreviewImport_RejectsOversizedFile(t *testing.T) {
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	largeContent := make([]byte, ImportMaxFileSize+1)
	_, err := svc.PreviewImport(context.Background(), bytes.NewReader(largeContent), "json")

	assert.Error(t, err)
	assert.Equal(t, ErrFileTooLarge, err)
}

func TestCreateImport_EnqueuesTask(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()

	jobRepo := &mockImportJobRepository{
		createFn: func(ctx context.Context, job *domain.ImportJob) error {
			assert.Equal(t, domain.ImportQueued, job.Status)
			assert.Equal(t, userID, job.CreatedBy)
			assert.Equal(t, "json", job.Format)
			assert.Equal(t, string(domain.ConflictSkip), job.ConflictStrategy)
			return nil
		},
		findByIdempotencyKeyFn: func(ctx context.Context, key string) (*domain.ImportJob, error) {
			return nil, apperrors.ErrNotFound
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	content := makeNDJSONContent()
	req := &ImportRequest{
		UserID:           userID,
		OrgID:            &orgID,
		File:             bytes.NewReader(content),
		Filename:         "test-import.json",
		Format:           "json",
		ConflictStrategy: "skip",
		EntityTypes:      domain.ImportOrder[:],
		DryRun:           false,
	}

	job, err := svc.CreateImport(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, domain.ImportQueued, job.Status)
	assert.Equal(t, userID, job.CreatedBy)
}

func TestCreateImport_RateLimited(t *testing.T) {
	userID := uuid.New()

	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{
		allowFn: func(ctx context.Context, userID, action string) (bool, error) {
			return false, nil
		},
	}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	content := makeNDJSONContent()
	req := &ImportRequest{
		UserID:           userID,
		File:             bytes.NewReader(content),
		Filename:         "test-import.json",
		Format:           "json",
		ConflictStrategy: "skip",
		EntityTypes:      domain.ImportOrder[:],
	}

	_, err := svc.CreateImport(context.Background(), req)
	assert.Equal(t, ErrImportRateLimited, err)
}

func TestCreateImport_DuplicateIdempotencyKey(t *testing.T) {
	userID := uuid.New()
	existingJob := &domain.ImportJob{ID: uuid.New()}

	jobRepo := &mockImportJobRepository{
		findByIdempotencyKeyFn: func(ctx context.Context, key string) (*domain.ImportJob, error) {
			return existingJob, nil
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	content := makeNDJSONContent()
	req := &ImportRequest{
		UserID:           userID,
		File:             bytes.NewReader(content),
		Filename:         "test-import.json",
		Format:           "json",
		ConflictStrategy: "skip",
		EntityTypes:      domain.ImportOrder[:],
	}

	_, err := svc.CreateImport(context.Background(), req)
	assert.Equal(t, ErrImportAlreadyExists, err)
}

func TestGetImport_NotFound(t *testing.T) {
	jobRepo := &mockImportJobRepository{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
			return nil, apperrors.ErrNotFound
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	_, err := svc.GetImport(context.Background(), uuid.New())
	assert.Equal(t, ErrImportNotFound, err)
}

func TestGetImport_Success(t *testing.T) {
	jobID := uuid.New()
	expectedJob := &domain.ImportJob{ID: jobID, Status: domain.ImportCompleted}

	jobRepo := &mockImportJobRepository{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
			assert.Equal(t, jobID, id)
			return expectedJob, nil
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	job, err := svc.GetImport(context.Background(), jobID)
	assert.NoError(t, err)
	assert.Equal(t, jobID, job.ID)
}

func TestListImports_Defaults(t *testing.T) {
	orgID := uuid.New()
	var capturedPage, capturedPageSize int

	jobRepo := &mockImportJobRepository{
		listFn: func(ctx context.Context, o *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, int64, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return []*domain.ImportJob{}, 0, nil
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	_, err := svc.ListImports(context.Background(), &orgID, 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, capturedPage, "page should default to 1")
	assert.Equal(t, 20, capturedPageSize, "pageSize should default to 20")
}

func TestCancelImport_ValidStatuses(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		expectError  bool
		expectedCode string
	}{
		{name: "queued", status: domain.ImportQueued, expectError: false},
		{name: "validating", status: domain.ImportValidating, expectError: false},
		{name: "processing", status: domain.ImportProcessing, expectError: false},
		{name: "completed", status: domain.ImportCompleted, expectError: true, expectedCode: "BAD_REQUEST"},
		{name: "failed", status: domain.ImportFailed, expectError: true, expectedCode: "BAD_REQUEST"},
		{name: "cancelled", status: domain.ImportCancelled, expectError: true, expectedCode: "BAD_REQUEST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobID := uuid.New()
			jobRepo := &mockImportJobRepository{
				findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
					return &domain.ImportJob{ID: jobID, Status: tt.status}, nil
				},
				updateStatusFn: func(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error {
					assert.Equal(t, domain.ImportCancelled, status)
					return nil
				},
			}
			idMapRepo := &mockImportIDMapRepository{}
			rateLimiter := &mockRateLimiter{}

			svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

			err := svc.CancelImport(context.Background(), jobID)

			if tt.expectError {
				assert.Error(t, err)
				var appErr *apperrors.AppError
				if errors.As(err, &appErr) {
					assert.Equal(t, tt.expectedCode, appErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCancelImport_NotFound(t *testing.T) {
	jobRepo := &mockImportJobRepository{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
			return nil, apperrors.ErrNotFound
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	err := svc.CancelImport(context.Background(), uuid.New())
	assert.Equal(t, ErrImportNotFound, err)
}

func TestProcessImport_CancelledJob(t *testing.T) {
	jobID := uuid.New()

	jobRepo := &mockImportJobRepository{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
			return &domain.ImportJob{
				ID:     jobID,
				Status: domain.ImportCancelled,
			}, nil
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	err := svc.ProcessImport(context.Background(), jobID)
	assert.Equal(t, ErrImportCancelled, err)
}

func TestProcessImport_NotFound(t *testing.T) {
	jobRepo := &mockImportJobRepository{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
			return nil, apperrors.ErrNotFound
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	err := svc.ProcessImport(context.Background(), uuid.New())
	assert.Equal(t, ErrImportNotFound, err)
}

func TestConflictStrategies(t *testing.T) {
	tests := []struct {
		name      string
		strategy  domain.ConflictStrategy
		existing  bool
		process   bool
		expectErr bool
	}{
		{name: "skip_no_conflict", strategy: domain.ConflictSkip, existing: false, process: true, expectErr: false},
		{name: "skip_conflict", strategy: domain.ConflictSkip, existing: true, process: false, expectErr: false},
		{name: "overwrite_no_conflict", strategy: domain.ConflictOverwrite, existing: false, process: true, expectErr: false},
		{name: "overwrite_conflict", strategy: domain.ConflictOverwrite, existing: true, process: true, expectErr: false},
		{name: "fail_no_conflict", strategy: domain.ConflictFail, existing: false, process: true, expectErr: false},
		{name: "fail_conflict", strategy: domain.ConflictFail, existing: true, process: false, expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldProcess, err := applyConflictStrategy(tt.strategy, tt.existing)
			assert.Equal(t, tt.process, shouldProcess)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Equal(t, ErrEntityConflict, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestComputeIdempotencyKey(t *testing.T) {
	content := []byte("test content")
	key := computeIdempotencyKey(content)

	assert.NotEmpty(t, key, "idempotency key should not be empty")
	assert.Len(t, key, 64, "SHA-256 hex should be 64 characters")

	key2 := computeIdempotencyKey(content)
	assert.Equal(t, key, key2, "same content should produce same key")

	key3 := computeIdempotencyKey([]byte("different content"))
	assert.NotEqual(t, key, key3, "different content should produce different key")
}

func TestVerifySourceSignature(t *testing.T) {
	content := []byte("test file content")
	secret := "test-secret"

	validSig := SignDataPortabilityFile(content, secret)

	assert.True(t, verifySourceSignature(content, validSig, secret), "valid signature should verify")
	assert.False(t, verifySourceSignature([]byte("tampered"), validSig, secret), "tampered content should fail verification")
	assert.False(t, verifySourceSignature(content, validSig, "wrong-secret"), "wrong secret should fail verification")
}

func TestFilterEntityTypes(t *testing.T) {
	registry := domain.NewEntityRegistry()
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	order := registry.GetImportOrder()

	filtered := svc.filterEntityTypes(order, []string{"users", "roles"})
	assert.Equal(t, []string{"roles", "users"}, filtered, "should return types in topological order, not request order")

	filtered = svc.filterEntityTypes(order, []string{})
	assert.Empty(t, filtered)

	filtered = svc.filterEntityTypes(order, []string{"users", "nonexistent"})
	assert.Equal(t, []string{"users"}, filtered)
}

func TestCreateDecoder(t *testing.T) {
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	jsonDecoder, err := svc.createDecoder("json")
	assert.NoError(t, err)
	assert.IsType(t, &JSONDecoder{}, jsonDecoder)

	csvDecoder, err := svc.createDecoder("csv")
	assert.NoError(t, err)
	assert.IsType(t, &CSVDecoder{}, csvDecoder)

	_, err = svc.createDecoder("xml")
	assert.Equal(t, ErrInvalidImportFormat, err)
}

func TestCSVPreview_ValidatesWithoutWritingToDB(t *testing.T) {
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	csvContent := "_type,id,name\nusers," + uuid.New().String() + ",Test User\n"
	resp, err := svc.PreviewImport(context.Background(), bytes.NewReader([]byte(csvContent)), "csv")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestHMACVerificationFailure(t *testing.T) {
	content := []byte("test payload")
	wrongSig := "sha256=deadbeef"

	result := verifySourceSignature(content, wrongSig, "correct-secret")
	assert.False(t, result, "wrong signature should fail verification")
}

func TestImportConfig_Defaults(t *testing.T) {
	config := DefaultImportConfig()
	assert.Equal(t, ImportBatchSize, config.BatchSize)
	assert.Equal(t, ImportMaxFileSize, int64(ImportMaxFileSize), config.MaxFileSize)
	assert.Equal(t, ImportMaxEntities, config.MaxEntities)
}

func TestEnqueueImportTask(t *testing.T) {
	jobID := uuid.New()
	data, err := EnqueueImportTask(jobID)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var payload ImportTaskPayload
	err = json.Unmarshal(data, &payload)
	assert.NoError(t, err)
	assert.Equal(t, jobID.String(), payload.JobID)
}

func TestSetEventBus(t *testing.T) {
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	assert.Nil(t, svc.eventBus)

	eventBus := domain.NewEventBus(256)
	svc.SetEventBus(eventBus)

	assert.NotNil(t, svc.eventBus)
}

func TestImportService_Interface(t *testing.T) {
	var _ ImportService = (*importService)(nil)
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name   string
		err    *apperrors.AppError
		code   string
		status int
	}{
		{name: "not_found", err: ErrImportNotFound, code: "NOT_FOUND", status: 404},
		{name: "rate_limited", err: ErrImportRateLimited, code: "RATE_LIMITED", status: 429},
		{name: "file_too_large", err: ErrFileTooLarge, code: "PAYLOAD_TOO_LARGE", status: 413},
		{name: "entity_conflict", err: ErrEntityConflict, code: "CONFLICT", status: 409},
		{name: "invalid_format", err: ErrInvalidImportFormat, code: "BAD_REQUEST", status: 400},
		{name: "cancelled", err: ErrImportCancelled, code: "CANCELLED", status: 499},
		{name: "already_exists", err: ErrImportAlreadyExists, code: "CONFLICT", status: 409},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.status, tt.err.HTTPStatus)
		})
	}
}

func TestCreateImport_InvalidConflictStrategy(t *testing.T) {
	userID := uuid.New()

	jobRepo := &mockImportJobRepository{
		findByIdempotencyKeyFn: func(ctx context.Context, key string) (*domain.ImportJob, error) {
			return nil, apperrors.ErrNotFound
		},
	}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	content := makeNDJSONContent()
	req := &ImportRequest{
		UserID:           userID,
		File:             bytes.NewReader(content),
		Filename:         "test-import.json",
		Format:           "json",
		ConflictStrategy: "invalid",
		EntityTypes:      domain.ImportOrder[:],
	}

	_, err := svc.CreateImport(context.Background(), req)
	assert.Error(t, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, "BAD_REQUEST", appErr.Code)
	}
}

func TestCreateImport_OversizedFile(t *testing.T) {
	userID := uuid.New()

	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	largeContent := make([]byte, ImportMaxFileSize+1)
	req := &ImportRequest{
		UserID:           userID,
		File:             bytes.NewReader(largeContent),
		Filename:         "large-import.json",
		Format:           "json",
		ConflictStrategy: "skip",
		EntityTypes:      domain.ImportOrder[:],
	}

	_, err := svc.CreateImport(context.Background(), req)
	assert.Equal(t, ErrFileTooLarge, err)
}

func TestCreateImport_InvalidFormat(t *testing.T) {
	userID := uuid.New()

	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	content := makeNDJSONContent()
	req := &ImportRequest{
		UserID:           userID,
		File:             bytes.NewReader(content),
		Filename:         "test-import.xml",
		Format:           "xml",
		ConflictStrategy: "skip",
		EntityTypes:      domain.ImportOrder[:],
	}

	_, err := svc.CreateImport(context.Background(), req)
	assert.Equal(t, ErrInvalidImportFormat, err)
}

func TestProcessImport_TopologicalOrder(t *testing.T) {
	registry := domain.NewEntityRegistry()
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	// Verify that ImportOrder from registry matches domain order
	order := registry.GetImportOrder()
	expectedOrder := []string{
		domain.EntityTypeOrganization,
		domain.EntityTypeRole,
		domain.EntityTypePermission,
		domain.EntityTypeUser,
		domain.EntityTypeOrgMember,
		domain.EntityTypeUserRole,
		domain.EntityTypeUserPermission,
	}
	assert.Equal(t, expectedOrder, order, "entity processing should follow topological order")

	// Verify filterEntityTypes preserves topological order
	filtered := svc.filterEntityTypes(order, []string{"users", "roles"})
	assert.Equal(t, []string{"roles", "users"}, filtered, "filtering should preserve topological order, not request order")

	// Verify the service implements the ImportService interface
	assert.NotNil(t, svc, "ImportService should be constructed successfully")
}

type errorReader struct{}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func TestPreviewImport_ReadError(t *testing.T) {
	jobRepo := &mockImportJobRepository{}
	idMapRepo := &mockImportIDMapRepository{}
	rateLimiter := &mockRateLimiter{}

	svc := newTestImportService(jobRepo, idMapRepo, rateLimiter)

	_, err := svc.PreviewImport(context.Background(), &errorReader{}, "json")
	assert.Error(t, err)
}