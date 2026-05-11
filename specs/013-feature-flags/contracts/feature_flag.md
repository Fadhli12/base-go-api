# Feature Flag Service Contract

**Feature ID:** 013-feature-flags | **Date:** 2026-05-11

## Service Interface

```go
type FeatureFlagService struct {
    repo     repository.FeatureFlagRepository
    enforcer *permission.Enforcer
    audit    *AuditService
    log      *slog.Logger
}

func NewFeatureFlagService(
    repo repository.FeatureFlagRepository,
    enforcer *permission.Enforcer,
    audit *AuditService,
    log *slog.Logger,
) *FeatureFlagService
```

## Method Contracts

### Create(ctx, userID, orgID, orgDomain, req) (*FeatureFlagResponse, error)

**RBAC**: `enforcer.Enforce(userID, orgDomain, "feature_flag", "manage")`
**Audit**: `LogAction(ctx, userID, "create", "feature_flag", flagID, nil, flag, ip, userAgent)`
**Validation**: Key format `^[a-z][a-z0-9_]*$`, rollout 0-100, conditions against known keys
**Error cases**: 403 Forbidden, 422 Validation Error, 409 Conflict (duplicate key)

### GetByID(ctx, userID, orgID, orgDomain, id) (*FeatureFlagResponse, error)

**RBAC**: `enforcer.Enforce(userID, orgDomain, "feature_flag", "view")`
**Error cases**: 403 Forbidden, 404 Not Found

### GetByKey(ctx, userID, orgID, orgDomain, key) (*FeatureFlagResponse, error)

**RBAC**: `enforcer.Enforce(userID, orgDomain, "feature_flag", "view")`
**Error cases**: 403 Forbidden, 404 Not Found

### List(ctx, userID, orgID, orgDomain, limit, offset) ([]*FeatureFlagResponse, int64, error)

**RBAC**: `enforcer.Enforce(userID, orgDomain, "feature_flag", "view")`
**Returns**: Paginated list with total count

### Update(ctx, userID, orgID, orgDomain, id, updates, ip, userAgent) (*FeatureFlagResponse, error)

**RBAC**: `enforcer.Enforce(userID, orgDomain, "feature_flag", "manage")`
**Audit**: `LogAction(ctx, userID, "update", "feature_flag", id, before, after, ip, userAgent)`
**Validation**: Same as Create for provided fields
**Error cases**: 403 Forbidden, 404 Not Found, 422 Validation Error

### Delete(ctx, userID, orgID, orgDomain, id, ip, userAgent) error

**RBAC**: `enforcer.Enforce(userID, orgDomain, "feature_flag", "manage")`
**Audit**: `LogAction(ctx, userID, "delete", "feature_flag", id, flag, nil, ip, userAgent)`
**System Protection**: Returns 403 if `IsSystem == true`
**Error cases**: 403 Forbidden (RBAC or system flag), 404 Not Found

### IsEnabled(ctx, key, userID) bool

**No RBAC** — authenticated-only
**Logic**: Find flag → check Enabled → check Rollout → check Conditions → default false
**Returns**: bool only, no error (fails open to false)

### IsEnabledWithReason(ctx, key, userID) (*FeatureFlagEvaluation, error)

**No RBAC** — authenticated-only
**Returns**: Evaluation with reason string

### EvaluateAll(ctx, userID) (*BulkEvaluateResponse, error)

**No RBAC** — authenticated-only
**Returns**: All flags evaluated for the given user

## Handler Contract

```go
type FeatureFlagHandler struct {
    service *service.FeatureFlagService
    enforcer *permission.Enforcer
}

func NewFeatureFlagHandler(service *service.FeatureFlagService, enforcer *permission.Enforcer) *FeatureFlagHandler

func (h *FeatureFlagHandler) RegisterRoutes(api *echo.Group, jwtSecret string)
```

## Route Details

| Method | Path | Handler | Permission | Notes |
|--------|------|---------|-----------|-------|
| POST | `/api/v1/feature-flags` | Create | `feature_flag:manage` | |
| GET | `/api/v1/feature-flags` | List | `feature_flag:view` | Paginated |
| GET | `/api/v1/feature-flags/:id` | GetByID | `feature_flag:view` | |
| PUT | `/api/v1/feature-flags/:id` | Update | `feature_flag:manage` | |
| DELETE | `/api/v1/feature-flags/:id` | Delete | `feature_flag:manage` | System flags protected |
| GET | `/api/v1/feature-flags/:key/evaluate` | Evaluate | authenticated | No RBAC, user context required |
| GET | `/api/v1/feature-flags/evaluate` | EvaluateAll | authenticated | No RBAC, user context required |

## Repository Contract

```go
type FeatureFlagRepository interface {
    Create(ctx context.Context, flag *domain.FeatureFlag) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.FeatureFlag, error)
    FindByKey(ctx context.Context, key string) (*domain.FeatureFlag, error)
    FindAll(ctx context.Context, limit, offset int) ([]*domain.FeatureFlag, int64, error)
    Update(ctx context.Context, flag *domain.FeatureFlag) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

Error mapping: `gorm.ErrRecordNotFound` → `errors.ErrNotFound`