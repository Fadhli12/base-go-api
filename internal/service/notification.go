package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/example/go-api-base/internal/cache"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

const (
	notificationRateLimit    = 100
	notificationRateLimitTTL = 3600 // 1 hour in seconds
)

// defaultEmailEnabled maps notification types to their default email-enabled state.
// Types not in this map default to false.
var defaultEmailEnabled = map[domain.NotificationType]bool{
	domain.NotificationTypeSystem:          true,
	domain.NotificationTypeAssignment:      true,
	domain.NotificationTypeMention:         true,
	domain.NotificationTypeInvoiceCreated: false,
	domain.NotificationTypeNewsPublished:  false,
}

// NotificationService handles notification business logic
type NotificationService struct {
	notifRepo    repository.NotificationRepository
	prefRepo     repository.NotificationPreferenceRepository
	emailService *EmailService
	userRepo     repository.UserRepository
	cache        cache.Driver
	log          *slog.Logger
}

// NewNotificationService creates a new NotificationService instance
func NewNotificationService(
	notifRepo repository.NotificationRepository,
	prefRepo repository.NotificationPreferenceRepository,
	emailService *EmailService,
	userRepo repository.UserRepository,
	cacheDriver cache.Driver,
	log *slog.Logger,
) *NotificationService {
	return &NotificationService{
		notifRepo:    notifRepo,
		prefRepo:     prefRepo,
		emailService: emailService,
		userRepo:     userRepo,
		cache:        cacheDriver,
		log:          log,
	}
}

// Send creates a notification with rate limiting and optional email routing.
// It validates the notification type, enforces a 100/hour rate limit per user,
// persists the notification, and queues an email if the user's preferences allow it.
// Email queueing failures are logged but do not block notification creation.
func (s *NotificationService) Send(ctx context.Context, userID uuid.UUID, notifType, title, message, actionURL string) error {
	if !domain.IsValidNotificationType(notifType) {
		return errors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("invalid notification type: %s", notifType), 422)
	}

	// Rate limit check — fail-open on cache errors
	if err := s.checkAndIncrRateLimit(ctx, userID); err != nil {
		return err
	}

	notification := &domain.Notification{
		UserID:    userID,
		Type:      domain.NotificationType(notifType),
		Title:     title,
		Message:   message,
		ActionURL: actionURL,
	}

	if err := s.notifRepo.Create(ctx, notification); err != nil {
		return err
	}

	// Queue email asynchronously — errors are logged, not propagated
	s.maybeQueueEmail(ctx, userID, domain.NotificationType(notifType), title, message)

	return nil
}

// SendBulk creates notifications for multiple users.
// Individual errors are logged; the method continues processing remaining users.
func (s *NotificationService) SendBulk(ctx context.Context, userIDs []uuid.UUID, notifType, title, message, actionURL string) error {
	if !domain.IsValidNotificationType(notifType) {
		return errors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("invalid notification type: %s", notifType), 422)
	}

	for _, userID := range userIDs {
		if err := s.Send(ctx, userID, notifType, title, message, actionURL); err != nil {
			s.log.Error("failed to send notification to user",
				slog.String("user_id", userID.String()),
				slog.String("notif_type", notifType),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// ListByUser returns paginated active (non-archived) notifications for a user
func (s *NotificationService) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error) {
	return s.notifRepo.FindByUserID(ctx, userID, limit, offset)
}

// ListUnreadByUser returns paginated unread notifications for a user
func (s *NotificationService) ListUnreadByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error) {
	return s.notifRepo.FindUnreadByUserID(ctx, userID, limit, offset)
}

// CountUnread returns the count of unread active notifications for a user
func (s *NotificationService) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.notifRepo.CountUnreadByUserID(ctx, userID)
}

// MarkAsRead marks a single notification as read, verifying ownership
func (s *NotificationService) MarkAsRead(ctx context.Context, notifID, userID uuid.UUID) error {
	return s.notifRepo.MarkAsRead(ctx, notifID, userID)
}

// MarkAllAsRead marks all unread notifications as read for a user.
// If notifType is non-nil, only notifications of that type are updated.
// Returns the number of rows affected.
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uuid.UUID, notifType *string) (int64, error) {
	if notifType != nil && !domain.IsValidNotificationType(*notifType) {
		return 0, errors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("invalid notification type: %s", *notifType), 422)
	}
	return s.notifRepo.MarkAllAsRead(ctx, userID, notifType)
}

// GetPreferences returns all notification preferences for a user
func (s *NotificationService) GetPreferences(ctx context.Context, userID uuid.UUID) ([]*domain.NotificationPreference, error) {
	return s.prefRepo.FindByUserID(ctx, userID)
}

// UpdatePreference creates or updates a notification preference for a user
func (s *NotificationService) UpdatePreference(ctx context.Context, userID uuid.UUID, notifType string, emailEnabled, pushEnabled bool) error {
	if !domain.IsValidNotificationType(notifType) {
		return errors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("invalid notification type: %s", notifType), 422)
	}

	pref := &domain.NotificationPreference{
		UserID:           userID,
		NotificationType: domain.NotificationType(notifType),
		EmailEnabled:     emailEnabled,
		PushEnabled:      pushEnabled,
	}

	return s.prefRepo.Upsert(ctx, pref)
}

// ArchiveNotification archives a notification, verifying ownership
func (s *NotificationService) ArchiveNotification(ctx context.Context, notifID, userID uuid.UUID) error {
	return s.notifRepo.ArchiveByID(ctx, notifID, userID)
}

// checkAndIncrRateLimit checks the per-user hourly rate limit and increments the counter.
// On cache errors the check is skipped (fail-open) to avoid blocking notification creation.
func (s *NotificationService) checkAndIncrRateLimit(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("notifications:user:%s:hourly", userID.String())

	data, err := s.cache.Get(ctx, key)
	if err != nil {
		s.log.Warn("rate limit cache get failed, skipping check",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}

	if data != nil {
		count, parseErr := strconv.ParseInt(string(data), 10, 64)
		if parseErr != nil {
			s.log.Warn("rate limit counter parse failed, skipping check",
				slog.String("user_id", userID.String()),
				slog.String("error", parseErr.Error()),
			)
			return nil
		}
		if count >= notificationRateLimit {
			return errors.ErrTooManyRequests
		}
		// Increment existing counter. Driver has no Expire/TTL op so the TTL
		// resets to 1 hour on each write — known approximation for this interface.
		newCount := count + 1
		if setErr := s.cache.Set(ctx, key, []byte(strconv.FormatInt(newCount, 10)), notificationRateLimitTTL); setErr != nil {
			s.log.Warn("rate limit counter increment failed, continuing",
				slog.String("user_id", userID.String()),
				slog.String("error", setErr.Error()),
			)
		}
		return nil
	}

	// Key does not exist — first notification this window
	if setErr := s.cache.Set(ctx, key, []byte("1"), notificationRateLimitTTL); setErr != nil {
		s.log.Warn("rate limit counter init failed, continuing",
			slog.String("user_id", userID.String()),
			slog.String("error", setErr.Error()),
		)
	}

	return nil
}

// maybeQueueEmail checks the user's email preference for notifType and queues
// an email if enabled. Errors are logged and never propagated to the caller.
func (s *NotificationService) maybeQueueEmail(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType, title, message string) {
	if s.emailService == nil {
		return
	}

	emailEnabled := s.resolveEmailEnabled(ctx, userID, notifType)
	if !emailEnabled {
		return
	}

	// Fetch user email address
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		s.log.Error("failed to fetch user for email notification",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	req := &EmailRequest{
		To:          user.Email,
		Subject:     title,
		TextContent: message,
	}

	if err := s.emailService.QueueEmail(ctx, req); err != nil {
		s.log.Error("failed to queue notification email",
			slog.String("user_id", userID.String()),
			slog.String("notif_type", string(notifType)),
			slog.String("error", err.Error()),
		)
	}
}

// resolveEmailEnabled returns whether email should be sent for this user+type combination.
// It looks up the stored preference; if none exists, falls back to defaultEmailEnabled.
func (s *NotificationService) resolveEmailEnabled(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) bool {
	pref, err := s.prefRepo.FindByUserIDAndType(ctx, userID, notifType)
	if err == nil {
		return pref.EmailEnabled
	}

	// Preference not found — use per-type defaults
	if defaultVal, ok := defaultEmailEnabled[notifType]; ok {
		return defaultVal
	}
	return false
}
