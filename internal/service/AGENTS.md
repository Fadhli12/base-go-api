# Service Layer - Business Logic

**Location:** internal/service | **Purpose:** Domain business logic

---

## OVERVIEW

Service layer coordinates repositories and enforces business rules. No permission checks (handled by handlers/middleware).

## SERVICES

| File | Purpose | Dependencies |
|------|---------|--------------|
| user.go | User management, roles, effective permissions | UserRepository, UserRoleRepository, UserPermissionRepository |
| role.go | Role CRUD, permission attach/detach | RoleRepository, RolePermissionRepository, PermissionRepository |
| permission.go | Permission creation/listing | PermissionRepository |
| auth.go | Register, login, refresh, logout | UserRepository, RefreshTokenRepository, TokenService, PasswordHasher |
| audit.go | Async audit logging | AuditLogRepository |

## PATTERN

### Service Struct

```go
type UserService struct {
    userRepo repository.UserRepository
    roleRepo repository.UserRoleRepository
    permRepo repository.UserPermissionRepository
}

func NewUserService(
    userRepo repository.UserRepository,
    roleRepo repository.UserRoleRepository,
    permRepo repository.UserPermissionRepository,
) *UserService {
    return &UserService{
        userRepo: userRepo,
        roleRepo: roleRepo,
        permRepo: permRepo,
    }
}
```

### Method Signature

```go
// Context ALWAYS first parameter
func (s *UserService) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
```

## ERROR HANDLING

```go
// Domain-specific errors
return nil, apperrors.NewAppError("VALIDATION_ERROR", "Name is required", 422)

// Wrap repository errors
if err != nil {
    return nil, apperrors.WrapInternal(err)
}

// Return sentinel errors
return nil, apperrors.ErrNotFound
```

## VALIDATION

Inline validation with early returns:

```go
func (s *RoleService) Create(ctx context.Context, name, description string) (*domain.Role, error) {
    if name == "" {
        return nil, apperrors.NewAppError("VALIDATION_ERROR", "Role name is required", 422)
    }
    
    // Check for duplicates
    existing, _ := s.roleRepo.FindByName(ctx, name)
    if existing != nil {
        return nil, apperrors.NewAppError("CONFLICT", "Role already exists", 409)
    }
    
    role := &domain.Role{Name: name, Description: description}
    return role, s.roleRepo.Create(ctx, role)
}
```

## COMMON PATTERNS

### CRUD Create

```go
func (s *RoleService) Create(ctx context.Context, name, description string) (*domain.Role, error) {
    // 1. Validate input
    if name == "" {
        return nil, apperrors.NewAppError("VALIDATION_ERROR", "Name required", 422)
    }
    
    // 2. Check business rules (uniqueness)
    existing, _ := s.roleRepo.FindByName(ctx, name)
    if existing != nil {
        return nil, apperrors.NewAppError("CONFLICT", "Already exists", 409)
    }
    
    // 3. Create entity
    role := &domain.Role{
        Name:        name,
        Description: description,
    }
    
    // 4. Persist
    if err := s.roleRepo.Create(ctx, role); err != nil {
        return nil, apperrors.WrapInternal(err)
    }
    
    return role, nil
}
```

### CRUD Find

```go
func (s *UserService) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    user, err := s.userRepo.FindByID(ctx, id)
    if err != nil {
        return nil, err  // Already wrapped by repo
    }
    return user, nil
}
```

### CRUD Update

```go
func (s *RoleService) Update(ctx context.Context, id uuid.UUID, name, description string) (*domain.Role, error) {
    role, err := s.roleRepo.FindByID(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Business validation
    if name != "" && name != role.Name {
        existing, _ := s.roleRepo.FindByName(ctx, name)
        if existing != nil {
            return nil, apperrors.NewAppError("CONFLICT", "Name taken", 409)
        }
        role.Name = name
    }
    
    role.Description = description
    
    if err := s.roleRepo.Update(ctx, role); err != nil {
        return nil, apperrors.WrapInternal(err)
    }
    
    return role, nil
}
```

### Relationship Management

```go
func (s *UserService) AssignRole(ctx context.Context, userID, roleID, assignedBy uuid.UUID) error {
    // Validate user exists
    _, err := s.userRepo.FindByID(ctx, userID)
    if err != nil {
        return apperrors.ErrNotFound
    }
    
    // Validate role exists
    // ...
    
    // Assign
    return s.roleRepo.Assign(ctx, userID, roleID, assignedBy)
}
```

## AUDIT PATTERN (Async)

```go
// audit.go - Non-blocking audit logging
type AuditService struct {
    repo  repository.AuditLogRepository
    logs  chan *domain.AuditLog  // Buffered channel
}

func (s *AuditService) LogAction(ctx context.Context, log *domain.AuditLog) error {
    select {
    case s.logs <- log:  // Non-blocking send
        return nil
    default:
        return errors.New("audit queue full")
    }
}
```

## AUTH PATTERN

```go
// auth.go - Multiple return values
func (s *AuthService) Login(ctx context.Context, req *request.LoginRequest) (
    *domain.User,    // User entity
    string,          // Access token
    string,          // Refresh token
    error,
) {
    // 1. Find user
    user, err := s.userRepo.FindByEmail(ctx, req.Email)
    if err != nil {
        return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid credentials", 401)
    }
    
    // 2. Verify password
    if !s.passwordHasher.Verify(user.PasswordHash, req.Password) {
        return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid credentials", 401)
    }
    
    // 3. Generate tokens
    accessToken, _ := auth.GenerateAccessToken(user.ID, user.Email, ...)
    refreshToken, _ := s.tokenService.GenerateRefreshToken(ctx, user.ID)
    
    return user, accessToken, refreshToken, nil
}
```

## NO PERMISSION CHECKS

Services do NOT call `enforcer.Enforce()`. Permission enforcement happens at:
- Middleware layer (route-level checks)
- Handler layer (scope checks before service call)

## CONVENTIONS

| Rule | Description |
|------|-------------|
| Context first | All methods have `ctx context.Context` as first param |
| No repositories interfaces | Services are concrete, not interfaces |
| Wrap errors | Use `apperrors.WrapInternal()` for repo errors |
| Return apperrors | Return `apperrors.NewAppError()` for business errors |
| No GORM directly | Access DB only through repository interfaces |
| No permissions | Enforcer used in handlers, not services |