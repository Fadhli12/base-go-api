package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ActivityFilters holds optional filtering criteria for activity queries.
type ActivityFilters struct {
	ActionType    string
	ResourceType  string
	ResourceID    string
	UnreadOnly    bool
	FollowingOnly bool
	UserID        uuid.UUID // for unread/following filters
	Since         *time.Time // for archived activities
}

// FollowResource represents a resource identifier for batch follow lookups.
type FollowResource struct {
	ResourceType string
	ResourceID   string
}

// ActivityRepository defines the data access interface for activities.
type ActivityRepository interface {
	Create(ctx context.Context, activity *domain.Activity) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Activity, error)
	FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters ActivityFilters, limit, offset int) ([]*domain.Activity, int64, error)
	FindByResource(ctx context.Context, resourceType, resourceID string, hasOrgID bool, orgID uuid.UUID, limit, offset int) ([]*domain.Activity, int64, error)
	ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// ActivityReadRepository defines the data access interface for activity read tracking.
type ActivityReadRepository interface {
	Upsert(ctx context.Context, read *domain.ActivityRead) error
	FindByUserAndActivityIDs(ctx context.Context, userID uuid.UUID, activityIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	DeleteByUserAndActivity(ctx context.Context, userID, activityID uuid.UUID) error
	CountUnreadByUser(ctx context.Context, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID, filters ActivityFilters) (int64, error)
	MarkAllReadByUser(ctx context.Context, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID) (int64, error)
}

// ActivityFollowRepository defines the data access interface for activity follow tracking.
type ActivityFollowRepository interface {
	Create(ctx context.Context, follow *domain.ActivityFollow) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ActivityFollow, int64, error)
	FindByUserAndResource(ctx context.Context, userID uuid.UUID, resourceType, resourceID string) (*domain.ActivityFollow, error)
	FindByUserAndResourceIDs(ctx context.Context, userID uuid.UUID, resources []FollowResource) (map[string]bool, error)
	ExistsByUserAndResource(ctx context.Context, userID uuid.UUID, resourceType, resourceID string) (bool, error)
}

// --- ActivityRepository implementation ---

type activityRepository struct {
	db *gorm.DB
}

// NewActivityRepository creates a new GORM-backed ActivityRepository.
func NewActivityRepository(db *gorm.DB) ActivityRepository {
	return &activityRepository{db: db}
}

func (r *activityRepository) Create(ctx context.Context, activity *domain.Activity) error {
	if err := r.db.WithContext(ctx).Create(activity).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *activityRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Activity, error) {
	var activity domain.Activity
	if err := r.db.WithContext(ctx).First(&activity, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &activity, nil
}

func (r *activityRepository) FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters ActivityFilters, limit, offset int) ([]*domain.Activity, int64, error) {
	var activities []*domain.Activity
	var total int64

	// When Since is provided, use Unscoped to bypass GORM soft-delete filter
	// so we can access archived records, then manually filter deleted_at IS NULL.
	db := r.db.WithContext(ctx).Model(&domain.Activity{})
	if filters.Since != nil {
		db = db.Unscoped()
	}

	// Organization scoping
	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	}

	// Core visibility filters
	if filters.Since != nil {
		// Archived activities since a given time: must be archived and not soft-deleted
		db = db.Where("archived_at IS NOT NULL AND created_at >= ? AND deleted_at IS NULL", *filters.Since)
	} else {
		// Active activities: not archived (GORM handles deleted_at IS NULL for non-Unscoped)
		if filters.Since == nil {
			db = db.Where("archived_at IS NULL")
		}
	}

	// Optional filters
	if filters.ActionType != "" {
		db = db.Where("action_type = ?", filters.ActionType)
	}
	if filters.ResourceType != "" {
		db = db.Where("resource_type = ?", filters.ResourceType)
	}
	if filters.ResourceID != "" {
		db = db.Where("resource_id = ?", filters.ResourceID)
	}

	// Count total matching records
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	// Paginate
	query := db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Offset(offset).Find(&activities).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return activities, total, nil
}

func (r *activityRepository) FindByResource(ctx context.Context, resourceType, resourceID string, hasOrgID bool, orgID uuid.UUID, limit, offset int) ([]*domain.Activity, int64, error) {
	var activities []*domain.Activity
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Activity{}).
		Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).
		Where("archived_at IS NULL")

	// Organization scoping
	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	query := db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Offset(offset).Find(&activities).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return activities, total, nil
}

func (r *activityRepository) ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result := r.db.WithContext(ctx).Model(&domain.Activity{}).
		Unscoped().
		Where("created_at < ? AND archived_at IS NULL AND deleted_at IS NULL", cutoff).
		Update("archived_at", gorm.Expr("NOW()"))

	if result.Error != nil {
		return 0, apperrors.WrapInternal(result.Error)
	}
	return result.RowsAffected, nil
}

func (r *activityRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.Activity{}).
		Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// --- ActivityReadRepository implementation ---

type activityReadRepository struct {
	db *gorm.DB
}

// NewActivityReadRepository creates a new GORM-backed ActivityReadRepository.
func NewActivityReadRepository(db *gorm.DB) ActivityReadRepository {
	return &activityReadRepository{db: db}
}

func (r *activityReadRepository) Upsert(ctx context.Context, read *domain.ActivityRead) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND activity_id = ?", read.UserID, read.ActivityID).
		Assign(map[string]interface{}{"read_at": gorm.Expr("NOW()")}).
		FirstOrCreate(read).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *activityReadRepository) FindByUserAndActivityIDs(ctx context.Context, userID uuid.UUID, activityIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	result := make(map[uuid.UUID]bool)
	if len(activityIDs) == 0 {
		return result, nil
	}

	var reads []domain.ActivityRead
	if err := r.db.WithContext(ctx).
		Select("activity_id").
		Where("user_id = ? AND activity_id IN ?", userID, activityIDs).
		Find(&reads).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	for _, read := range reads {
		result[read.ActivityID] = true
	}
	return result, nil
}

func (r *activityReadRepository) DeleteByUserAndActivity(ctx context.Context, userID, activityID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Delete(&domain.ActivityRead{}, "user_id = ? AND activity_id = ?", userID, activityID)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *activityReadRepository) CountUnreadByUser(ctx context.Context, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID, filters ActivityFilters) (int64, error) {
	var count int64

	// Count activities visible to user that are NOT in activity_reads for that user.
	// Use subquery to avoid JOIN complexity.
	db := r.db.WithContext(ctx).Model(&domain.Activity{}).
		Where("archived_at IS NULL").
		Where("id NOT IN (?)",
			r.db.Table("activity_reads").
				Select("activity_id").
				Where("user_id = ?", userID),
		)

	// Organization scoping
	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	}

	// Optional filters
	if filters.ActionType != "" {
		db = db.Where("action_type = ?", filters.ActionType)
	}
	if filters.ResourceType != "" {
		db = db.Where("resource_type = ?", filters.ResourceType)
	}
	if filters.ResourceID != "" {
		db = db.Where("resource_id = ?", filters.ResourceID)
	}

	if err := db.Count(&count).Error; err != nil {
		return 0, apperrors.WrapInternal(err)
	}
	return count, nil
}

func (r *activityReadRepository) MarkAllReadByUser(ctx context.Context, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID) (int64, error) {
	// Build the WHERE clause for visible activities
	orgCondition := ""
	args := []interface{}{userID}
	if hasOrgID {
		orgCondition = "AND a.organization_id = ?"
		args = append(args, orgID)
	}
	args = append(args, userID) // for the subquery user_id

	query := fmt.Sprintf(`
		INSERT INTO activity_reads (id, user_id, activity_id, read_at)
		SELECT gen_random_uuid(), ?, a.id, NOW()
		FROM activities a
		WHERE a.archived_at IS NULL
		AND a.deleted_at IS NULL
		%s
		AND a.id NOT IN (
			SELECT ar.activity_id FROM activity_reads ar WHERE ar.user_id = ?
		)
	`, orgCondition)

	result := r.db.WithContext(ctx).Exec(query, args...)
	if result.Error != nil {
		return 0, apperrors.WrapInternal(result.Error)
	}
	return result.RowsAffected, nil
}

// --- ActivityFollowRepository implementation ---

type activityFollowRepository struct {
	db *gorm.DB
}

// NewActivityFollowRepository creates a new GORM-backed ActivityFollowRepository.
func NewActivityFollowRepository(db *gorm.DB) ActivityFollowRepository {
	return &activityFollowRepository{db: db}
}

func (r *activityFollowRepository) Create(ctx context.Context, follow *domain.ActivityFollow) error {
	if err := r.db.WithContext(ctx).Create(follow).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *activityFollowRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.ActivityFollow{}, "id = ?", id)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *activityFollowRepository) FindByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ActivityFollow, int64, error) {
	var follows []*domain.ActivityFollow
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.ActivityFollow{}).
		Where("user_id = ?", userID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	query := db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Offset(offset).Find(&follows).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return follows, total, nil
}

func (r *activityFollowRepository) FindByUserAndResource(ctx context.Context, userID uuid.UUID, resourceType, resourceID string) (*domain.ActivityFollow, error) {
	var follow domain.ActivityFollow
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND resource_type = ? AND resource_id = ?", userID, resourceType, resourceID).
		First(&follow).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &follow, nil
}

func (r *activityFollowRepository) FindByUserAndResourceIDs(ctx context.Context, userID uuid.UUID, resources []FollowResource) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(resources) == 0 {
		return result, nil
	}

	// Build (resource_type, resource_id) pairs for the IN clause
	// GORM doesn't support composite IN cleanly, so we build OR conditions.
	db := r.db.WithContext(ctx).Model(&domain.ActivityFollow{}).
		Select("resource_type, resource_id").
		Where("user_id = ?", userID)

	// Build WHERE clause for composite key IN
	for i, res := range resources {
		if i == 0 {
			db = db.Where("(resource_type = ? AND resource_id = ?)", res.ResourceType, res.ResourceID)
		} else {
			db = db.Or("(resource_type = ? AND resource_id = ?)", res.ResourceType, res.ResourceID)
		}
	}

	var follows []domain.ActivityFollow
	if err := db.Find(&follows).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	for _, follow := range follows {
		key := fmt.Sprintf("%s:%s", follow.ResourceType, follow.ResourceID)
		result[key] = true
	}
	return result, nil
}

func (r *activityFollowRepository) ExistsByUserAndResource(ctx context.Context, userID uuid.UUID, resourceType, resourceID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.ActivityFollow{}).
		Where("user_id = ? AND resource_type = ? AND resource_id = ?", userID, resourceType, resourceID).
		Count(&count).Error; err != nil {
		return false, apperrors.WrapInternal(err)
	}
	return count > 0, nil
}