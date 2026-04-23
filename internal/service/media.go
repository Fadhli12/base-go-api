package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/conversion"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/storage"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Constants for media validation and processing
const (
	// MaxFileSize is the maximum allowed file size (100MB)
	MaxFileSize = 100 * 1024 * 1024

	// DefaultSignedURLExpiry is the default expiry for signed URLs (1 hour)
	DefaultSignedURLExpiry = time.Hour

	// MaxSignedURLExpiry is the maximum expiry for signed URLs (24 hours)
	MaxSignedURLExpiry = 24 * time.Hour

	// MagicBytesLength is the number of bytes to read for MIME type detection
	MagicBytesLength = 512

	// CollectionDefault is the default collection name
	CollectionDefault = "default"

	// DiskLocal is the local storage disk identifier
	DiskLocal = "local"
)

// Allowed MIME types
var (
	// AllowedImageTypes are allowed image MIME types
	AllowedImageTypes = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
		"image/svg+xml": true,
	}

	// AllowedDocumentTypes are allowed document MIME types
	AllowedDocumentTypes = map[string]bool{
		"application/pdf":                                                         true,
		"application/msword":                                                      true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"text/plain":  true,
		"text/csv":    true,
	}

	// AllowedArchiveTypes are allowed archive MIME types
	AllowedArchiveTypes = map[string]bool{
		"application/zip":               true,
		"application/x-rar-compressed": true,
		"application/x-7z-compressed":  true,
	}

	// BlockedExtensions are file extensions that are not allowed
	BlockedExtensions = map[string]bool{
		".exe":  true,
		".dll":  true,
		".bat":  true,
		".cmd":  true,
		".sh":   true,
		".php":  true,
		".jsp":  true,
		".asp":  true,
		".aspx": true,
		".py":   true,
		".rb":   true,
		".pl":   true,
		".cgi":  true,
	}
)

// MediaService defines the interface for media business logic
type MediaService interface {
	// Upload handles file upload with validation and storage
	Upload(ctx context.Context, userID uuid.UUID, req UploadRequest) (*domain.Media, error)

	// List retrieves media for a specific model with pagination
	List(ctx context.Context, modelType string, modelID uuid.UUID, filter ListFilter) ([]*domain.Media, int64, error)

	// Get retrieves a single media by ID
	Get(ctx context.Context, mediaID uuid.UUID) (*domain.Media, error)

	// Delete performs a soft delete on media
	Delete(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, isAdmin bool) error

	// UpdateMetadata updates custom properties for media
	UpdateMetadata(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, customProperties map[string]interface{}, isAdmin bool) (*domain.Media, error)

	// GetSignedURL generates a signed URL for downloading media
	GetSignedURL(ctx context.Context, mediaID uuid.UUID, conversionName string, expiry time.Duration) (string, time.Time, error)

	// ValidateSignedURL validates a signed URL signature and expiry
	ValidateSignedURL(mediaID uuid.UUID, signature string, expires int64, signingSecret string) bool

	// CheckPermission checks if user has permission for an action on media
	CheckPermission(ctx context.Context, userID uuid.UUID, action string) (bool, error)

	// GetStorageDriver returns the storage driver
	GetStorageDriver() storage.Driver
}

// UploadRequest contains parameters for media upload
type UploadRequest struct {
	File            multipart.File
	FileHeader      *multipart.FileHeader
	ModelType       string
	ModelID         uuid.UUID
	Collection      string
	CustomProperties map[string]interface{}
}

// ListFilter contains parameters for listing media
type ListFilter struct {
	Collection string
	MimeType   string
	Limit      int
	Offset     int
	Sort       string
}

// mediaService implements MediaService
type mediaService struct {
	repo            repository.MediaRepository
	enforcer        *permission.Enforcer
	storageDriver   storage.Driver
	signingSecret   string
	conversionWorker *conversion.Worker
}

// NewMediaService creates a new MediaService instance
func NewMediaService(
	repo repository.MediaRepository,
	enforcer *permission.Enforcer,
	storageDriver storage.Driver,
	signingSecret string,
) MediaService {
	return &mediaService{
		repo:            repo,
		enforcer:        enforcer,
		storageDriver:   storageDriver,
		signingSecret:   signingSecret,
	}
}

// NewMediaServiceWithConversion creates a new MediaService instance with conversion support
func NewMediaServiceWithConversion(
	repo repository.MediaRepository,
	enforcer *permission.Enforcer,
	storageDriver storage.Driver,
	signingSecret string,
	conversionWorker *conversion.Worker,
) MediaService {
	return &mediaService{
		repo:             repo,
		enforcer:         enforcer,
		storageDriver:    storageDriver,
		signingSecret:    signingSecret,
		conversionWorker: conversionWorker,
	}
}

// Upload handles file upload with validation and storage
func (s *mediaService) Upload(ctx context.Context, userID uuid.UUID, req UploadRequest) (*domain.Media, error) {
	// Validate file size
	if req.FileHeader.Size > MaxFileSize {
		return nil, errors.NewAppError("VALIDATION_ERROR", 
			fmt.Sprintf("File size exceeds maximum allowed size of %d bytes", MaxFileSize), 
			http.StatusBadRequest)
	}

	// Validate file extension
	originalFilename := req.FileHeader.Filename
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if BlockedExtensions[ext] {
		return nil, errors.NewAppError("VALIDATION_ERROR", "File type not allowed", http.StatusBadRequest)
	}

	// Sanitize filename
	sanitizedFilename := sanitizeFilename(originalFilename)

	// Detect MIME type from file content
	mimeType, err := detectMimeType(req.File)
	if err != nil {
		slog.Error("Failed to detect MIME type", "error", err)
		return nil, errors.NewAppError("VALIDATION_ERROR", "Failed to validate file type", http.StatusBadRequest)
	}

	// Validate MIME type
	if !isAllowedMimeType(mimeType) {
		return nil, errors.NewAppError("VALIDATION_ERROR", "File type not allowed", http.StatusBadRequest)
	}

	// Set default collection if not provided
	collection := req.Collection
	if collection == "" {
		collection = CollectionDefault
	}

	// Generate UUID filename for storage
	storageFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	
	// Build storage path: {model_type}/{model_id}/{filename}
	storagePath := fmt.Sprintf("%s/%s/%s", req.ModelType, req.ModelID.String(), storageFilename)

	// Reset file reader to beginning
	if _, err := req.File.Seek(0, 0); err != nil {
		return nil, errors.WrapInternal(err)
	}

	// Store file
	if err := s.storageDriver.Store(ctx, storagePath, req.File); err != nil {
		slog.Error("Failed to store file", "error", err, "path", storagePath)
		return nil, errors.NewAppError("STORAGE_ERROR", "Failed to store file", http.StatusInternalServerError)
	}

	// Get file size
	fileSize := req.FileHeader.Size

	// Extract image metadata if it's an image
	var metadata datatypes.JSONMap
	if isImageMimeType(mimeType) {
		// Set basic metadata for images
		metadata = datatypes.JSONMap{
			"mime_type": mimeType,
		}
	}

	// Create media record
	media := &domain.Media{
		ModelType:        req.ModelType,
		ModelID:         req.ModelID,
		CollectionName:  collection,
		Disk:            DiskLocal,
		Filename:        storageFilename,
		OriginalFilename: sanitizedFilename,
		MimeType:        mimeType,
		Size:            fileSize,
		Path:            storagePath,
		Metadata:        metadata,
		CustomProperties: req.CustomProperties,
		UploadedByID:    userID,
	}

	// Save to database
	if err := s.repo.Create(ctx, media); err != nil {
		// Attempt to clean up stored file on DB error
		if delErr := s.storageDriver.Delete(ctx, storagePath); delErr != nil {
			slog.Error("Failed to clean up file after DB error", "error", delErr, "path", storagePath)
		}
		return nil, errors.WrapInternal(err)
	}

	// Trigger conversions for images (fire-and-forget, don't block upload)
	if s.conversionWorker != nil && conversion.IsSupportedImageFormat(media.MimeType) {
		go func() {
			s.conversionWorker.ProcessConversions(context.Background(), media)
		}()
	}

	slog.Info("Media uploaded successfully",
		"media_id", media.ID,
		"user_id", userID,
		"model_type", req.ModelType,
		"size", fileSize)

	return media, nil
}

// List retrieves media for a specific model with pagination
func (s *mediaService) List(ctx context.Context, modelType string, modelID uuid.UUID, filter ListFilter) ([]*domain.Media, int64, error) {
	// Apply defaults
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	// Query media
	mediaList, err := s.repo.FindByModelTypeAndID(ctx, modelType, modelID, filter.Collection)
	if err != nil {
		return nil, 0, err
	}

	// Filter by MIME type if specified
	if filter.MimeType != "" {
		filtered := make([]*domain.Media, 0)
		for _, m := range mediaList {
			if m.MimeType == filter.MimeType {
				filtered = append(filtered, m)
			}
		}
		mediaList = filtered
	}

	total := int64(len(mediaList))

	// Apply pagination
	start := filter.Offset
	if start > len(mediaList) {
		start = len(mediaList)
	}
	end := start + limit
	if end > len(mediaList) {
		end = len(mediaList)
	}

	return mediaList[start:end], total, nil
}

// Get retrieves a single media by ID
func (s *mediaService) Get(ctx context.Context, mediaID uuid.UUID) (*domain.Media, error) {
	return s.repo.FindByID(ctx, mediaID)
}

// Delete performs a soft delete on media
func (s *mediaService) Delete(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, isAdmin bool) error {
	// Fetch media first
	media, err := s.repo.FindByID(ctx, mediaID)
	if err != nil {
		return err
	}

	// Ownership check: if not admin, can only delete own uploads
	if !isAdmin && media.UploadedByID != userID {
		return errors.ErrNotFound
	}

	// Mark as orphaned before soft delete (for cleanup tracking)
	if err := s.repo.MarkOrphaned(ctx, mediaID); err != nil {
		slog.Warn("Failed to mark media as orphaned", "error", err, "media_id", mediaID)
	}

	// Perform soft delete
	if err := s.repo.SoftDelete(ctx, mediaID); err != nil {
		return err
	}

	slog.Info("Media deleted", "media_id", mediaID, "user_id", userID)
	return nil
}

// UpdateMetadata updates custom properties for media
func (s *mediaService) UpdateMetadata(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, customProperties map[string]interface{}, isAdmin bool) (*domain.Media, error) {
	// Fetch media first
	media, err := s.repo.FindByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	// Ownership check: if not admin, can only update own uploads
	if !isAdmin && media.UploadedByID != userID {
		return nil, errors.ErrNotFound
	}

	// Update custom properties
	media.CustomProperties = customProperties
	media.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, media); err != nil {
		return nil, err
	}

	return media, nil
}

// GetSignedURL generates a signed URL for downloading media
func (s *mediaService) GetSignedURL(ctx context.Context, mediaID uuid.UUID, conversionName string, expiry time.Duration) (string, time.Time, error) {
	// Fetch media
	media, err := s.repo.FindByID(ctx, mediaID)
	if err != nil {
		return "", time.Time{}, err
	}

	// Use storage signer
	signer := storage.NewSigner(s.signingSecret)
	
	var signedURL *storage.SignedURL
	if conversionName != "" {
		// Find conversion path
		path := media.Path
		for _, conv := range media.Conversions {
			if conv.Name == conversionName {
				path = conv.Path
				break
			}
		}
		signedURL, err = signer.GenerateWithConversion(mediaID.String(), path, conversionName, expiry)
	} else {
		signedURL, err = signer.Generate(mediaID.String(), media.Path, expiry)
	}
	
	if err != nil {
		return "", time.Time{}, err
	}

	return signedURL.URL, signedURL.ExpiresAt, nil
}

// ValidateSignedURL validates a signed URL signature and expiry
func (s *mediaService) ValidateSignedURL(mediaID uuid.UUID, signature string, expires int64, signingSecret string) bool {
	signer := storage.NewSigner(signingSecret)
	return signer.Validate(mediaID.String(), signature, expires, "")
}

// CheckPermission checks if user has permission for an action on media
func (s *mediaService) CheckPermission(ctx context.Context, userID uuid.UUID, action string) (bool, error) {
	if s.enforcer == nil {
		return true, nil
	}

	return s.enforcer.Enforce(userID.String(), "default", "media", action)
}

// GetStorageDriver returns the storage driver
func (s *mediaService) GetStorageDriver() storage.Driver {
	return s.storageDriver
}

// AdminListFilter contains parameters for admin listing media
// This is a placeholder for potential future implementation
type AdminListFilter struct {
	ModelType      string
	ModelID        string
	Collection     string
	MimeType       string
	IncludeDeleted bool
	Limit          int
	Offset         int
	Sort           string
}

// sanitizeFilename removes path components and cleans the filename
func sanitizeFilename(filename string) string {
	// Get base filename (remove path)
	base := filepath.Base(filename)
	
	// Remove any null bytes
	base = strings.ReplaceAll(base, "\x00", "")
	
	// Trim spaces
	base = strings.TrimSpace(base)
	
	// Ensure filename is not empty
	if base == "" || base == "." || base == ".." {
		base = "unnamed"
	}
	
	return base
}

// detectMimeType detects the MIME type from file content using magic bytes
func detectMimeType(file multipart.File) (string, error) {
	// Read first 512 bytes for magic number detection
	buffer := make([]byte, MagicBytesLength)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}
	buffer = buffer[:n]

	// Detect MIME type from magic bytes
	mimeType := http.DetectContentType(buffer)
	
	return mimeType, nil
}

// isAllowedMimeType checks if the MIME type is in the allowed list
func isAllowedMimeType(mimeType string) bool {
	if AllowedImageTypes[mimeType] {
		return true
	}
	if AllowedDocumentTypes[mimeType] {
		return true
	}
	if AllowedArchiveTypes[mimeType] {
		return true
	}
	return false
}

// isImageMimeType checks if the MIME type is an image
func isImageMimeType(mimeType string) bool {
	return len(mimeType) > 6 && mimeType[:6] == "image/"
}


