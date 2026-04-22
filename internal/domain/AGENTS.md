# Domain Entities - GORM Models

**Location:** internal/domain | **Purpose:** Domain entity definitions

---

## OVERVIEW

GORM entity models with UUID primary keys, soft deletes, and relationship mappings. All entities have explicit `TableName()` methods.

## ENTITIES

| File | Type | Soft Delete |
|------|------|-------------|
| user.go | Primary | Yes |
| role.go | Primary | Yes |
| permission.go | Primary | Yes |
| user_role.go | Junction | No |
| user_permission.go | Junction+Effect | No |
| role_permission.go | Junction | No |
| refresh_token.go | Token | No |
| audit_log.go | Immutable | No |

## ENTITY TEMPLATE

```go
type User struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    Email        string         `gorm:"uniqueIndex;size:255;not null" json:"email"`
    PasswordHash string         `gorm:"size:255;not null" json:"-"`        // Hidden
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`                    // Soft delete
}

func (User) TableName() string { return "users" }
```

## REQUIRED PATTERNS

### UUID Primary Key

```go
ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
```

### Soft Delete (entities only)

```go
DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
```

- Applied to: User, Role, Permission
- NOT for: Junction tables, AuditLog, RefreshToken

### Table Name

```go
func (Entity) TableName() string { return "entities" }  // lowercase plural
```

### Timestamps

```go
CreatedAt time.Time `json:"created_at"`
UpdatedAt time.Time `json:"updated_at"`
```

## JSON PATTERNS

| Tag | Usage |
|-----|-------|
| `json:"-"` | Hide from API (PasswordHash, TokenHash, DeletedAt) |
| `json:",omitempty"` | Optional (RevokedAt, associations) |
| `json:"field_name"` | Standard exposure |

## RESPONSE DTO

```go
type UserResponse struct {
    ID        uuid.UUID `json:"id"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    // No PasswordHash, no DeletedAt
}

func (u *User) ToResponse() UserResponse {
    return UserResponse{
        ID:        u.ID,
        Email:     u.Email,
        CreatedAt: u.CreatedAt,
        UpdatedAt: u.UpdatedAt,
    }
}
```

## RELATIONSHIPS

### Many-to-Many

```go
// role.go
type Role struct {
    // ...
    Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}
```

### Foreign Key with Cascade

```go
// refresh_token.go
type RefreshToken struct {
    UserID uuid.UUID `gorm:"type:uuid;not null;index"`
    User   User      `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}
```

### Composite Primary Key

```go
// user_role.go
type UserRole struct {
    UserID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
    RoleID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"role_id"`
    AssignedAt time.Time `gorm:"autoCreateTime" json:"assigned_at"`
}
```

## GORM TAGS QUICK REFERENCE

| Tag | Purpose |
|-----|---------|
| `type:uuid` | PostgreSQL UUID type |
| `primaryKey` | Primary key field |
| `default:gen_random_uuid()` | Auto-generate UUID |
| `uniqueIndex` | Unique constraint |
| `size:N` | VARCHAR(N) |
| `not null` | Required field |
| `index` | Create index |
| `autoCreateTime` | Set on insert |
| `many2many:table` | Many-to-many relationship |
| `foreignKey:Field` | Foreign key relationship |

## BUSINESS METHODS

```go
// refresh_token.go
func (rt *RefreshToken) IsRevoked() bool {
    return rt.RevokedAt != nil
}

func (rt *RefreshToken) IsValid() bool {
    if rt.IsRevoked() { return false }
    return time.Now().Before(rt.ExpiresAt)
}
```

## CONSTANTS

```go
// audit_log.go - Domain constants
const (
    AuditActionCreate = "create"
    AuditActionUpdate = "update"
    AuditActionDelete = "delete"
    
    AuditResourceUser       = "user"
    AuditResourceRole       = "role"
    AuditResourcePermission = "permission"
)
```

## NO VALIDATION TAGS

Entity files do NOT have `validate:` tags. Validation happens at:
- HTTP request DTOs (`internal/http/request/`)
- Service layer business logic

## CHECKLIST FOR NEW ENTITIES

- [ ] UUID primary key with `gen_random_uuid()`
- [ ] CreatedAt and UpdatedAt fields
- [ ] DeletedAt if entity (not junction/audit)
- [ ] TableName() method returning lowercase plural
- [ ] json:"-" for sensitive fields
- [ ] ToResponse() if any field should be hidden
- [ ] Business methods for state queries (optional)