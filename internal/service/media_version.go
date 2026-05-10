package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/storage"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// ComputeSHA256Checksum computes the SHA-256 hash of data from an io.Reader.
// It reads all data from the reader and returns the hex-encoded checksum.
// The caller is responsible for resetting the reader if needed.
func ComputeSHA256Checksum(data io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, data); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// VersionService defines the interface for media version business logic
type VersionService interface {
	// UploadVersion uploads a new version of an existing media file
	UploadVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, file io.Reader, filename string, fileSize int64, contentType string, ipAddress, userAgent string) (*domain.MediaVersion, error)
	// ListVersions returns paginated version history for a media file
	ListVersions(ctx context.Context, mediaID uuid.UUID, limit, offset int) (*domain.VersionHistoryResponse, error)
	// GetVersion returns a specific version of a media file
	GetVersion(ctx context.Context, mediaID uuid.UUID, version int) (*domain.MediaVersionResponse, error)
	// DownloadVersion streams a specific version file from storage
	DownloadVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, ipAddress, userAgent string) (io.ReadCloser, *domain.MediaVersion, error)
	// GetVersionSignedURL generates a signed URL for downloading a specific version
	GetVersionSignedURL(ctx context.Context, mediaID uuid.UUID, version int, expiresIn int) (string, time.Time, error)
	// RestoreVersion promotes a previous version as current without creating a new version
	RestoreVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, ipAddress, userAgent string) (*domain.Media, error)
	// DeleteVersion soft-deletes a specific version (admin only, cannot delete current version)
	DeleteVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, ipAddress, userAgent string) error
}

// versionService implements VersionService
type versionService struct {
	mediaRepo     repository.MediaRepository
	versionRepo   repository.MediaVersionRepository
	storageDriver storage.Driver
	signingSecret string
	auditSvc      *AuditService
	enforcer      *permission.Enforcer
}

// NewVersionService creates a new VersionService instance
func NewVersionService(
	mediaRepo repository.MediaRepository,
	versionRepo repository.MediaVersionRepository,
	storageDriver storage.Driver,
	signingSecret string,
	auditSvc *AuditService,
	enforcer *permission.Enforcer,
) VersionService {
	return &versionService{
		mediaRepo:     mediaRepo,
		versionRepo:   versionRepo,
		storageDriver: storageDriver,
		signingSecret: signingSecret,
		auditSvc:      auditSvc,
		enforcer:      enforcer,
	}
}

func (s *versionService) UploadVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, file io.Reader, filename string, fileSize int64, contentType string, ipAddress, userAgent string) (*domain.MediaVersion, error) {
	media, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, errors.NewAppError("NOT_FOUND", "Media not found", http.StatusNotFound)
		}
		return nil, errors.WrapInternal(err)
	}

	versionCount, err := s.versionRepo.CountByMediaID(ctx, mediaID)
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	if versionCount == 0 {
		if err := s.createRetroactiveV1(ctx, media); err != nil {
			return nil, err
		}
	}

	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		return nil, errors.NewAppError("INTERNAL_ERROR", "File reader must be seekable", http.StatusInternalServerError)
	}

	detectedMime, err := detectMimeTypeFromReader(file)
	if err != nil {
		return nil, errors.NewAppError("VALIDATION_ERROR", "Failed to validate file type", http.StatusBadRequest)
	}

	if !isMimeTypeCompatible(media.MimeType, detectedMime) {
		return nil, errors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("MIME type mismatch: expected compatible with %s, got %s", media.MimeType, detectedMime),
			http.StatusBadRequest)
	}

	if _, err := seeker.Seek(0, 0); err != nil {
		return nil, errors.WrapInternal(err)
	}

	checksum, err := ComputeSHA256Checksum(file)
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	if err := s.checkDuplicateChecksum(ctx, mediaID, media.CurrentVersion, checksum); err != nil {
		return nil, err
	}

	currentVersion := media.CurrentVersion
	nextVersion := currentVersion + 1
	sanitizedFilename := sanitizeFilename(filename)
	ext := strings.ToLower(filepath.Ext(sanitizedFilename))
	storageFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	versionedPath := fmt.Sprintf("%s/v%d/%s", mediaID.String(), nextVersion, storageFilename)

	if _, err := seeker.Seek(0, 0); err != nil {
		return nil, errors.WrapInternal(err)
	}

	if err := s.storageDriver.Store(ctx, versionedPath, file); err != nil {
		slog.Error("failed to store version file",
			"media_id", mediaID.String(),
			"path", versionedPath,
			"error", err)
		return nil, errors.NewAppError("STORAGE_ERROR", "Failed to store file", http.StatusInternalServerError)
	}

	version := &domain.MediaVersion{
		MediaID:          mediaID,
		Version:          nextVersion,
		Filename:         storageFilename,
		OriginalFilename: sanitizedFilename,
		MimeType:         detectedMime,
		Size:             fileSize,
		FilePath:         versionedPath,
		Checksum:         checksum,
		UploadedByID:     userID,
	}

	if err := s.versionRepo.Create(ctx, version); err != nil {
		if delErr := s.storageDriver.Delete(ctx, versionedPath); delErr != nil {
			slog.Error("failed to clean up version file after DB error",
				"path", versionedPath,
				"error", delErr)
		}
		return nil, errors.WrapInternal(err)
	}

	if err := s.versionRepo.UpdateCurrentVersion(ctx, mediaID, currentVersion, nextVersion); err != nil {
		slog.Error("failed to update media current_version (optimistic lock failed)",
			"media_id", mediaID.String(),
			"version", nextVersion,
			"current_version", currentVersion,
			"error", err)
		if delErr := s.versionRepo.SoftDelete(ctx, version.ID); delErr != nil {
			slog.Error("failed to clean up orphaned version record after optimistic lock failure",
				"version_id", version.ID.String(),
				"error", delErr)
		}
		if delErr := s.storageDriver.Delete(ctx, versionedPath); delErr != nil {
			slog.Error("failed to clean up orphaned version file after optimistic lock failure",
				"path", versionedPath,
				"error", delErr)
		}
		return nil, err
	}

	media.CurrentVersion = nextVersion

	if s.auditSvc != nil {
		s.logVersionUploadAudit(ctx, userID, version.ID, currentVersion, nextVersion, version, ipAddress, userAgent)
	}

	slog.Info("media version uploaded successfully",
		"media_id", mediaID.String(),
		"version", nextVersion,
		"version_id", version.ID.String(),
		"user_id", userID.String(),
		"size", fileSize)

	return version, nil
}

func (s *versionService) createRetroactiveV1(ctx context.Context, media *domain.Media) error {
	v1Checksum := s.computeExistingFileChecksum(ctx, media)
	v1 := &domain.MediaVersion{
		MediaID:          media.ID,
		Version:          1,
		Filename:         media.Filename,
		OriginalFilename: media.OriginalFilename,
		MimeType:         media.MimeType,
		Size:             media.Size,
		FilePath:         media.Path,
		Checksum:         v1Checksum,
		UploadedByID:     media.UploadedByID,
	}

	if err := s.versionRepo.Create(ctx, v1); err != nil {
		// Handle race condition: another concurrent upload may have already created v1
		if isDuplicateKeyError(err) {
			slog.Info("retroactive v1 already exists, continuing",
				"media_id", media.ID.String())
			return nil
		}
		return errors.WrapInternal(err)
	}

	slog.Info("retroactively created v1 for media",
		"media_id", media.ID.String(),
		"version_id", v1.ID.String(),
		"checksum", v1Checksum)
	return nil
}

// isDuplicateKeyError checks if an error is a unique constraint violation.
// This prevents race conditions where concurrent UploadVersion calls both
// attempt to create retroactive v1 simultaneously.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "23505")
}

func (s *versionService) computeExistingFileChecksum(ctx context.Context, media *domain.Media) string {
	existingFile, err := s.storageDriver.Get(ctx, media.Path)
	if err != nil {
		slog.Warn("failed to read existing media file for v1 checksum, continuing with empty checksum",
			"media_id", media.ID.String(),
			"path", media.Path,
			"error", err)
		return ""
	}
	defer existingFile.Close()

	checksum, err := ComputeSHA256Checksum(existingFile)
	if err != nil {
		slog.Warn("failed to compute v1 checksum, continuing with empty checksum",
			"media_id", media.ID.String(),
			"error", err)
		return ""
	}
	return checksum
}

func (s *versionService) checkDuplicateChecksum(ctx context.Context, mediaID uuid.UUID, currentVersion int, checksum string) error {
	if checksum == "" {
		return nil
	}

	currentVer, err := s.versionRepo.FindByMediaIDAndVersion(ctx, mediaID, currentVersion)
	if err == nil && currentVer != nil && currentVer.Checksum == checksum {
		return errors.NewAppError("CONFLICT", "File content matches current version", http.StatusConflict)
	}

	existingVer, err := s.versionRepo.FindByChecksum(ctx, mediaID, checksum)
	if err == nil && existingVer != nil {
		return errors.NewAppError("CONFLICT", "File content matches existing version", http.StatusConflict)
	}

	return nil
}

func (s *versionService) ListVersions(ctx context.Context, mediaID uuid.UUID, limit, offset int) (*domain.VersionHistoryResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	media, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, errors.NewAppError("NOT_FOUND", "Media not found", http.StatusNotFound)
		}
		return nil, errors.WrapInternal(err)
	}

	versions, total, err := s.versionRepo.FindByMediaID(ctx, mediaID, limit, offset)
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	versionResponses := make([]*domain.MediaVersionResponse, len(versions))
	for i, v := range versions {
		versionResponses[i] = v.ToResponse(media.CurrentVersion)
	}

	return &domain.VersionHistoryResponse{
		MediaID:        mediaID,
		CurrentVersion: media.CurrentVersion,
		Versions:       versionResponses,
		Total:          total,
	}, nil
}

func (s *versionService) GetVersion(ctx context.Context, mediaID uuid.UUID, version int) (*domain.MediaVersionResponse, error) {
	if version < 1 {
		return nil, errors.NewAppError("VALIDATION_ERROR", "Version number must be >= 1", http.StatusBadRequest)
	}

	media, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, errors.NewAppError("NOT_FOUND", "Media not found", http.StatusNotFound)
		}
		return nil, errors.WrapInternal(err)
	}

	mediaVersion, err := s.versionRepo.FindByMediaIDAndVersion(ctx, mediaID, version)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, errors.NewAppError("NOT_FOUND", "Version not found", http.StatusNotFound)
		}
		return nil, errors.WrapInternal(err)
	}

	if mediaVersion.IsDeleted() {
		return nil, errors.NewAppError("GONE", "Version has been deleted", http.StatusGone)
	}

	return mediaVersion.ToResponse(media.CurrentVersion), nil
}

func (s *versionService) DownloadVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, ipAddress, userAgent string) (io.ReadCloser, *domain.MediaVersion, error) {
	if version < 1 {
		return nil, nil, errors.NewAppError("VALIDATION_ERROR", "Version number must be >= 1", http.StatusBadRequest)
	}

	if _, err := s.mediaRepo.FindByID(ctx, mediaID); err != nil {
		if err == errors.ErrNotFound {
			return nil, nil, errors.NewAppError("NOT_FOUND", "Media not found", http.StatusNotFound)
		}
		return nil, nil, errors.WrapInternal(err)
	}

	mediaVersion, err := s.versionRepo.FindByMediaIDAndVersion(ctx, mediaID, version)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, nil, errors.NewAppError("NOT_FOUND", "Version not found", http.StatusNotFound)
		}
		return nil, nil, errors.WrapInternal(err)
	}

	if mediaVersion.IsDeleted() {
		return nil, nil, errors.NewAppError("GONE", "Version has been deleted", http.StatusGone)
	}

	file, err := s.storageDriver.Get(ctx, mediaVersion.FilePath)
	if err != nil {
		if storage.IsNotFound(err) {
			return nil, nil, errors.NewAppError("NOT_FOUND", "File not found in storage", http.StatusNotFound)
		}
		return nil, nil, errors.NewAppError("STORAGE_ERROR", "Failed to retrieve file", http.StatusInternalServerError)
	}

	if s.auditSvc != nil {
		afterJSON, _ := json.Marshal(map[string]interface{}{
			"version":   mediaVersion.Version,
			"file_path": mediaVersion.FilePath,
			"ip":        ipAddress,
		})
		_ = s.auditSvc.LogAction(ctx, userID, "version_download", "media_version",
			mediaVersion.ID.String(), nil, afterJSON, ipAddress, userAgent)
	}

	return file, mediaVersion, nil
}

func (s *versionService) GetVersionSignedURL(ctx context.Context, mediaID uuid.UUID, version int, expiresIn int) (string, time.Time, error) {
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	if expiresIn < 60 {
		return "", time.Time{}, errors.NewAppError("VALIDATION_ERROR", "Expiry must be at least 60 seconds", http.StatusBadRequest)
	}
	if expiresIn > 86400 {
		return "", time.Time{}, errors.NewAppError("VALIDATION_ERROR", "Expiry must not exceed 86400 seconds (24 hours)", http.StatusBadRequest)
	}

	if _, err := s.mediaRepo.FindByID(ctx, mediaID); err != nil {
		if err == errors.ErrNotFound {
			return "", time.Time{}, errors.NewAppError("NOT_FOUND", "Media not found", http.StatusNotFound)
		}
		return "", time.Time{}, errors.WrapInternal(err)
	}

	mediaVersion, err := s.versionRepo.FindByMediaIDAndVersion(ctx, mediaID, version)
	if err != nil {
		if err == errors.ErrNotFound {
			return "", time.Time{}, errors.NewAppError("NOT_FOUND", "Version not found", http.StatusNotFound)
		}
		return "", time.Time{}, errors.WrapInternal(err)
	}

	if mediaVersion.IsDeleted() {
		return "", time.Time{}, errors.NewAppError("GONE", "Version has been deleted", http.StatusGone)
	}

	signer := storage.NewSigner(s.signingSecret)
	signedURL, err := signer.Generate(mediaID.String(), mediaVersion.FilePath, time.Duration(expiresIn)*time.Second)
	if err != nil {
		return "", time.Time{}, errors.WrapInternal(err)
	}

	return signedURL.URL, signedURL.ExpiresAt, nil
}

func (s *versionService) RestoreVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, ipAddress, userAgent string) (*domain.Media, error) {
	media, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, errors.NewAppError("NOT_FOUND", "Media not found", http.StatusNotFound)
		}
		return nil, errors.WrapInternal(err)
	}

	mediaVersion, err := s.versionRepo.FindByMediaIDAndVersion(ctx, mediaID, version)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, errors.NewAppError("NOT_FOUND", "Version not found", http.StatusNotFound)
		}
		return nil, errors.WrapInternal(err)
	}

	if version == media.CurrentVersion {
		return nil, errors.NewAppError("VALIDATION_ERROR", "Version is already the current version", http.StatusBadRequest)
	}

	if mediaVersion.IsDeleted() {
		return nil, errors.NewAppError("GONE", "Cannot restore a deleted version", http.StatusGone)
	}

	previousVersion := media.CurrentVersion
	if err := s.versionRepo.UpdateCurrentVersion(ctx, mediaID, previousVersion, version); err != nil {
		return nil, err
	}
	media.CurrentVersion = version

	if s.auditSvc != nil {
		beforeSnapshot, _ := json.Marshal(map[string]interface{}{
			"current_version": previousVersion,
		})
		afterSnapshot, _ := json.Marshal(map[string]interface{}{
			"current_version":      version,
			"restored_version_id":  mediaVersion.ID.String(),
		})
		_ = s.auditSvc.LogAction(ctx, userID, "version_restore", "media_version",
			mediaVersion.ID.String(), beforeSnapshot, afterSnapshot, ipAddress, userAgent)
	}

	slog.Info("media version restored",
		"media_id", mediaID.String(),
		"from_version", previousVersion,
		"to_version", version,
		"user_id", userID.String())

	return media, nil
}

func (s *versionService) DeleteVersion(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, version int, ipAddress, userAgent string) error {
	media, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return errors.NewAppError("NOT_FOUND", "Media not found", http.StatusNotFound)
		}
		return errors.WrapInternal(err)
	}

	if version == media.CurrentVersion {
		return errors.NewAppError("VALIDATION_ERROR", "Cannot delete the current version", http.StatusBadRequest)
	}

	mediaVersion, err := s.versionRepo.FindByMediaIDAndVersion(ctx, mediaID, version)
	if err != nil {
		if err == errors.ErrNotFound {
			return errors.NewAppError("NOT_FOUND", "Version not found", http.StatusNotFound)
		}
		return errors.WrapInternal(err)
	}

	if mediaVersion.IsDeleted() {
		return errors.NewAppError("CONFLICT", "Version is already deleted", http.StatusConflict)
	}

	if err := s.versionRepo.SoftDelete(ctx, mediaVersion.ID); err != nil {
		if err == errors.ErrNotFound {
			return errors.NewAppError("NOT_FOUND", "Version not found", http.StatusNotFound)
		}
		return errors.WrapInternal(err)
	}

	if delErr := s.storageDriver.Delete(ctx, mediaVersion.FilePath); delErr != nil {
		slog.Error("failed to delete version file from storage, continuing",
			"media_id", mediaID.String(),
			"version", version,
			"file_path", mediaVersion.FilePath,
			"error", delErr)
	}

	if s.auditSvc != nil {
		beforeSnapshot, _ := json.Marshal(map[string]interface{}{
			"version":       mediaVersion.Version,
			"filename":      mediaVersion.Filename,
			"mime_type":     mediaVersion.MimeType,
			"size":          mediaVersion.Size,
			"file_path":     mediaVersion.FilePath,
		})
		_ = s.auditSvc.LogAction(ctx, userID, "version_delete", "media_version",
			mediaVersion.ID.String(), beforeSnapshot, nil, ipAddress, userAgent)
	}

	slog.Info("media version deleted",
		"media_id", mediaID.String(),
		"version", version,
		"version_id", mediaVersion.ID.String(),
		"user_id", userID.String())

	return nil
}

func (s *versionService) logVersionUploadAudit(ctx context.Context, userID, versionID uuid.UUID, fromVersion, toVersion int, version *domain.MediaVersion, ipAddress, userAgent string) {
	beforeSnapshot, _ := json.Marshal(map[string]interface{}{
		"current_version": fromVersion,
	})
	afterSnapshot, _ := json.Marshal(map[string]interface{}{
		"current_version":      toVersion,
		"version_id":           versionID.String(),
		"filename":             version.Filename,
		"mime_type":            version.MimeType,
		"size":                 version.Size,
		"checksum":             version.Checksum,
	})
	_ = s.auditSvc.LogAction(ctx, userID, "version_upload", "media_version", versionID.String(),
		beforeSnapshot, afterSnapshot, ipAddress, userAgent)
}

// mimeCompatibilityAliases defines pairs of MIME types that are considered compatible.
// This handles common cases where the same format may be detected differently.
var mimeCompatibilityAliases = map[string][]string{
	"image/jpeg": {"image/jpg"},
	"image/jpg":  {"image/jpeg"},
	"text/plain": {"text/csv"},
	"text/csv":   {"text/plain"},
}

// isMimeTypeCompatible checks if a new file's MIME type is compatible with the parent media's MIME type.
// Exact match is always allowed. A limited set of well-known aliases is also permitted.
// Same-category matching (e.g., all "image/*") is intentionally NOT allowed to prevent
// uploading a PNG as a new version of a JPEG, or an SVG as a new version of a GIF.
func isMimeTypeCompatible(parentType, newType string) bool {
	if parentType == newType {
		return true
	}

	// Check well-known aliases
	if aliases, ok := mimeCompatibilityAliases[parentType]; ok {
		for _, alias := range aliases {
			if newType == alias {
				return true
			}
		}
	}

	return false
}

// detectMimeTypeFromReader detects MIME type from an io.Reader without consuming it
// The caller is responsible for resetting the reader if needed.
func detectMimeTypeFromReader(r io.Reader) (string, error) {
	buffer := make([]byte, MagicBytesLength)
	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}
	buf := buffer[:n]
	return http.DetectContentType(buf), nil
}

// DetectContentTypeFromBytes detects MIME type from a byte slice (used for multipart FileHeader files)
func DetectContentTypeFromBytes(data []byte) string {
	return http.DetectContentType(data)
}

// ComputeSHA256ChecksumFromBytes computes SHA-256 checksum from a byte slice
func ComputeSHA256ChecksumFromBytes(data []byte) string {
	hash := sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// ComputeSHA256ChecksumFromReaderSeeker computes SHA-256 checksum and returns the reader reset to position 0
func ComputeSHA256ChecksumFromReaderSeeker(rs io.ReadSeeker) (string, error) {
	// Read into buffer for dual use (checksum + reset)
	buf := &bytes.Buffer{}
	tee := io.TeeReader(rs, buf)

	hash := sha256.New()
	if _, err := io.Copy(hash, tee); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
