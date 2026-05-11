# Feature Specification: Feature Flags System

**Feature ID:** 013-feature-flags
**Priority:** P3
**Complexity:** Low-Medium
**Estimated Effort:** 1-2 days
**Dependencies:** None
**Branch:** 013-feature-flags

---

## 1. Overview

Runtime feature toggles for gradual rollout, A/B testing, and environment-based feature control. Enables operators to enable/disable features and control rollout percentage without code deployment.

## 2. Goals

- CRUD management of feature flags via API
- Runtime evaluation: `IsEnabled(ctx, key, userID)` checks flag state, rollout percentage, and conditions
- Audit logging for all flag changes
- RBAC enforcement: `feature_flag:manage` for CRUD, `feature_flag:view` for read
- Soft delete for compliance (Constitution Principle III)
- SQL migrations only — no AutoMigrate (Constitution Principle V)

**Deferred to future iteration**: Organization-scoped flags (per-org overrides of global defaults). Current implementation provides global flags only. When org-scoping is needed, add `organization_id` column with null=global, and update evaluation logic to merge global + org-specific flags.

## 3. Entities

### 3.1 FeatureFlag

```go
type FeatureFlag struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Key          string         `gorm:"size:100;uniqueIndex;not null"`
    Name         string         `gorm:"size:255;not null"`
    Description  string         `gorm:"type:text"`
    Enabled      bool           `gorm:"default:false;not null"`
    Rollout      int            `gorm:"default:100;not null"`            // Percentage 0-100
    Conditions   datatypes.JSON `gorm:"type:jsonb"`                     // {"user_ids":[...],"org_ids":[...],"envs":["staging"]}
    IsSystem     bool           `gorm:"default:false;not null"`          // System flags cannot be deleted
    CreatedAt    time.Time
    UpdatedAt    time.Time
    DeletedAt    gorm.DeletedAt `gorm:"index"`
}
```

### 3.2 FeatureFlagEvaluation (ephemeral, no DB table)

```go
type FeatureFlagEvaluation struct {
    Key       string `json:"key"`
    Enabled   bool   `json:"enabled"`
    Reason    string `json:"reason"`    // "global_enabled", "rollout", "condition_match", "disabled"
    Rollout   int    `json:"rollout"`
}
```

### 3.3 DTOs

```go
type CreateFeatureFlagRequest struct {
    Key         string                 `json:"key" validate:"required,min=1,max=100"`
    Name        string                 `json:"name" validate:"required,min=1,max=255"`
    Description string                 `json:"description"`
    Enabled     *bool                  `json:"enabled"`            // defaults to false
    Rollout     int                    `json:"rollout"`             // 0-100, defaults 100
    Conditions  map[string]interface{} `json:"conditions"`         // optional
}

type UpdateFeatureFlagRequest struct {
    Name        *string                `json:"name"`
    Description *string                `json:"description"`
    Enabled     *bool                  `json:"enabled"`
    Rollout     *int                   `json:"rollout"`
    Conditions  map[string]interface{} `json:"conditions"`
}

type FeatureFlagResponse struct {
    ID          uuid.UUID              `json:"id"`
    Key         string                 `json:"key"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Enabled     bool                   `json:"enabled"`
    Rollout     int                    `json:"rollout"`
    Conditions  json.RawMessage        `json:"conditions"`
    IsSystem    bool                   `json:"is_system"`
    CreatedAt   time.Time             `json:"created_at"`
    UpdatedAt   time.Time             `json:"updated_at"`
}

type EvaluateFeatureFlagRequest struct {
    UserID string `json:"user_id" validate:"required"`  // for rollout/condition evaluation
}

type BulkEvaluateResponse struct {
    Flags []FeatureFlagEvaluation `json:"flags"`
}
```

## 4. Repository Interface

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

## 5. Service Interface

```go
type FeatureFlagService interface {
    Create(ctx context.Context, flag *domain.FeatureFlag) (*domain.FeatureFlagResponse, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.FeatureFlagResponse, error)
    GetByKey(ctx context.Context, key string) (*domain.FeatureFlagResponse, error)
    List(ctx context.Context, limit, offset int) ([]*domain.FeatureFlagResponse, int64, error)
    Update(ctx context.Context, id uuid.UUID, updates map[string]interface{}) (*domain.FeatureFlagResponse, error)
    Delete(ctx context.Context, id uuid.UUID) error
    IsEnabled(ctx context.Context, key string, userID uuid.UUID) bool
    IsEnabledWithReason(ctx context.Context, key string, userID uuid.UUID) (*domain.FeatureFlagEvaluation, error)
    EvaluateAll(ctx context.Context, userID uuid.UUID) (*domain.BulkEvaluateResponse, error)
}
```

## 6. Evaluation Logic

```
IsEnabled(key, userID):
1. Find flag by key (return false if not found)
2. If flag.Enabled == false → return false, reason="disabled"
3. If flag.Rollout == 100 → return true, reason="global_enabled"
4. If flag.Rollout == 0 → return false, reason="rollout_excluded"
5. Hash(userID + key) % 100 < flag.Rollout → return true, reason="rollout"
6. Check conditions:
   - If "user_ids" in conditions and userID in list → return true, reason="condition_match"
   - If "org_ids" in conditions → check user's org membership
7. Default: return false, reason="no_match"
```

## 7. Endpoints

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/api/v1/feature-flags` | `feature_flag:manage` | Create flag |
| GET | `/api/v1/feature-flags` | `feature_flag:view` | List flags |
| GET | `/api/v1/feature-flags/:id` | `feature_flag:view` | Get flag |
| PUT | `/api/v1/feature-flags/:id` | `feature_flag:manage` | Update flag |
| DELETE | `/api/v1/feature-flags/:id` | `feature_flag:manage` | Delete flag (soft) |
| GET | `/api/v1/feature-flags/:key/evaluate` | authenticated | Evaluate flag for user |
| GET | `/api/v1/feature-flags/evaluate` | authenticated | Evaluate all flags for user |

**Note:** `evaluate` endpoints require only authentication (any logged-in user can check flags). RBAC is for management endpoints only.

## 8. Permissions (seeded via `permission:sync`)

| Permission | Resource | Action |
|-----------|----------|--------|
| `feature_flag:view` | feature_flag | view |
| `feature_flag:manage` | feature_flag | manage |

## 9. Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `FEATURE_FLAG_CACHE_TTL` | 300 | Cache TTL in seconds for flag lookups |

## 10. Migrations

- `000021_create_feature_flags.up.sql` — `feature_flags` table with unique index on `key`
- `000021_create_feature_flags.down.sql` — Drop table

## 11. Constraints

- System flags (`is_system=true`) cannot be deleted (only disabled)
- Key format: lowercase, underscores, no spaces (validated on create)
- Rollout must be 0-100 (validated on create/update)
- Conditions JSONB is optional and validated for known keys (`user_ids`, `org_ids`, `envs`)
- Audit logging on all Create/Update/Delete operations (Constitution Principle VII)
- Soft delete on all operations (Constitution Principle III)
- RBAC enforcement via Casbin (Constitution Principle II)

## 12. Files to Create/Modify

| File | Action |
|------|--------|
| `internal/domain/feature_flag.go` | Create — entities + DTOs |
| `internal/repository/feature_flag.go` | Create — interface + GORM impl |
| `internal/service/feature_flag.go` | Create — business logic, evaluation, RBAC, audit |
| `internal/http/handler/feature_flag.go` | Create — 7 HTTP endpoints |
| `internal/http/request/feature_flag.go` | Create — request DTOs + validation |
| `internal/http/response/feature_flag.go` | Create — response envelope |
| `internal/http/server.go` | Modify — DI wiring + route registration |
| `cmd/api/main.go` | Modify — permission seeding |
| `migrations/000021_create_feature_flags.up.sql` | Create |
| `migrations/000021_create_feature_flags.down.sql` | Create |
| `tests/unit/feature_flag_service_test.go` | Create — unit tests |
| `tests/integration/feature_flag_test.go` | Create — integration tests |