# Repository Layer - GORM Data Access

**Location:** internal/repository | **Purpose:** Database access abstraction

---

## OVERVIEW

Repository layer provides interface-based data access with GORM. All methods use context propagation and map errors to application errors.

## REPOSITORIES

| File | Entity | Key Methods |
|------|--------|-------------|
| user.go | User | CRUD + FindByEmail + SoftDelete |
| role.go | Role | CRUD + FindByName + system role protection |
| permission.go | Permission | FindByName, FindByResource |
| user_role.go | UserRole | Assign, Remove, FindByUserID |
| user_permission.go | UserPermission | Grant, Deny, GetOverrides |
| role_permission.go | RolePermission | Attach, Detach, Sync |
| refresh_token.go | RefreshToken | ValidateToken, Rotate, Revoke |
| audit_log.go | AuditLog | Create (append-only), List with pagination |

## INTERFACE PATTERN

```go
type UserRepository interface {
    Create(ctx context.Context, user *domain.User) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
    FindByEmail(ctx context.Context, email string) (*domain.User, error)
    FindAll(ctx context.Context) ([]domain.User, error)
    Update(ctx context.Context, user *domain.User) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}

type userRepository struct { db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository {
    return &userRepository{db: db}
}
```

## CONTEXT PROPAGATION

Every method MUST use `r.db.WithContext(ctx)`:

```go
func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    var user domain.User
    err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
    // ...
}
```

## ERROR MAPPING

```go
import (
    gorm "gorm.io/gorm"
    apperrors "github.com/example/go-api-base/pkg/errors"
)

// GORM not found → AppError
if err == gorm.ErrRecordNotFound {
    return nil, apperrors.ErrNotFound
}

// Other errors → wrapped internal error
return nil, apperrors.WrapInternal(err)
```

## QUERY PATTERNS

### Find by ID

```go
func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    var user domain.User
    err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
    if err == gorm.ErrRecordNotFound {
        return nil, apperrors.ErrNotFound
    }
    return &user, apperrors.WrapInternal(err)
}
```

### Find with Where

```go
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
    var user domain.User
    err := r.db.WithContext(ctx).
        Where("email = ?", email).
        First(&user).Error
    if err == gorm.ErrRecordNotFound {
        return nil, apperrors.ErrNotFound
    }
    return &user, apperrors.WrapInternal(err)
}
```

### Create

```go
func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
    err := r.db.WithContext(ctx).Create(user).Error
    return apperrors.WrapInternal(err)
}
```

### Update

```go
func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
    result := r.db.WithContext(ctx).Save(user)
    if result.Error != nil {
        return apperrors.WrapInternal(result.Error)
    }
    if result.RowsAffected == 0 {
        return apperrors.ErrNotFound
    }
    return nil
}
```

### Soft Delete

```go
func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
    result := r.db.WithContext(ctx).Delete(&domain.User{}, "id = ?", id)
    if result.Error != nil {
        return apperrors.WrapInternal(result.Error)
    }
    if result.RowsAffected == 0 {
        return apperrors.ErrNotFound
    }
    return nil
}
```

### Preload Associations

```go
func (r *roleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
    var role domain.Role
    err := r.db.WithContext(ctx).
        Preload("Permissions").  // Prevent N+1
        First(&role, "id = ?", id).Error
    // ...
}
```

### Joins

```go
func (r *userRoleRepository) FindRolesByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
    var roles []domain.Role
    err := r.db.WithContext(ctx).
        Table("roles").
        Joins("JOIN user_roles ON roles.id = user_roles.role_id").
        Where("user_roles.user_id = ?", userID).
        Where("roles.deleted_at IS NULL").  // Manual soft-delete filter
        Find(&roles).Error
    // ...
}
```

### Transaction

```go
func (r *rolePermissionRepository) Sync(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // Delete existing
        if err := tx.Where("role_id = ?", roleID).Delete(&domain.RolePermission{}).Error; err != nil {
            return err
        }
        
        // Insert new
        for _, permID := range permissionIDs {
            rp := &domain.RolePermission{RoleID: roleID, PermissionID: permID}
            if err := tx.Create(rp).Error; err != nil {
                return err
            }
        }
        return nil
    })
}
```

### Upsert

```go
func (r *userPermissionRepository) Grant(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
    up := &domain.UserPermission{
        UserID:       userID,
        PermissionID: permissionID,
        Effect:       "allow",
        AssignedBy:   assignedBy,
    }
    
    return r.db.WithContext(ctx).
        Where("user_id = ? AND permission_id = ?", userID, permissionID).
        Assign(map[string]interface{}{"effect": "allow", "assigned_by": assignedBy}).
        FirstOrCreate(up).Error
}
```

## SOFT DELETE BEHAVIOR

GORM soft delete via `gorm.DeletedAt`:

```go
// Automatic filtering for queries
db.Find(&users)  // Excludes deleted_at IS NOT NULL

// Manual filter when needed
db.Unscoped().Find(&users)  // Include soft-deleted
```

**Note:** Join queries require manual `WHERE deleted_at IS NULL` clause.

## BUSINESS RULES

### System Role Protection

```go
// role.go
var ErrSystemRoleProtected = errors.New("cannot delete system role")

func (r *roleRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
    // Check if system role
    role, err := r.FindByID(ctx, id)
    if err != nil { return err }
    
    if role.IsSystem {
        return ErrSystemRoleProtected
    }
    
    // Proceed with delete
    return r.db.WithContext(ctx).Delete(&domain.Role{}, "id = ?", id).Error
}
```

## CONVENTIONS

| Rule | Description |
|------|-------------|
| Interface first | Define `type XxxRepository interface` |
| Private impl | `type xxxRepository struct { db *gorm.DB }` |
| Constructor | `NewXxxRepository(db *gorm.DB) XxxRepository` |
| Context first | `ctx context.Context` as first param |
| WithContext | ALL GORM operations use `r.db.WithContext(ctx)` |
| Error mapping | `gorm.ErrRecordNotFound` → `apperrors.ErrNotFound` |
| Wrap others | Use `apperrors.WrapInternal(err)` |
| RowsAffected | Check for 0 to return ErrNotFound |
| Preload | Use for associations to prevent N+1 |
| Joins | Manual `WHERE deleted_at IS NULL` |

## AVAILABLE ERRORS

| Import | HTTP | Use When |
|--------|------|----------|
| `apperrors.ErrNotFound` | 404 | Record not found |
| `apperrors.ErrConflict` | 409 | Unique constraint |
| `apperrors.ErrInternal` | 500 | Database errors |
| `ErrSystemRoleProtected` | 403 | Business rule |