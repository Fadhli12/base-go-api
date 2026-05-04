# Settings Feature Implementation Plan

**Feature:** User and System Settings with Organization Scoping  
**Complexity:** Low  
**Estimated Effort:** 1 day  
**Dependencies:** Existing RBAC, Organization, and Audit systems  
**Status:** ✅ REVISED (Code Review Issues Fixed)  

---

## Plan Review Fixes Applied

This plan was reviewed by the code-reviewer agent and received a "REVISE" verdict. The following critical and medium-severity issues have been fixed:

### **HIGH Issue #1: Organization ID Extraction ✅ FIXED**
- **Problem:** Original plan said extract orgID from JWT — but JWT only contains UserID/Email
- **Fix Applied:** Updated all handler methods (Steps 4B) to extract orgID from `X-Organization-ID` header via `middleware.GetOrganizationID(c)`
- **Fix Applied:** Added `middleware.ExtractOrganizationID()` to route group (Step 5A)
- **Fix Applied:** Added error handling for missing header (return 400)
- **Location:** Lines 341-368, 391-399, 425-453, 474-481

### **HIGH Issue #2: Permission Model Ambiguity ✅ FIXED**
- **Problem:** Original seed data used same action name `"view"` for both user and system settings, making Casbin unable to differentiate
- **Fix Applied:** Changed to distinct action names: `view`/`manage` for users, `view_system`/`manage_system` for admin system settings
- **Fix Applied:** Updated all `RequirePermission` calls in route registration to use correct action names
- **Location:** Lines 458-481, 425-453

### **MEDIUM Issues ✅ FIXED**
1. **Soft-Delete Upsert:** Added explicit guidance to reset `deleted_at = NULL` on conflict (Line 183-184)
2. **Merge Semantics:** Specified shallow merge strategy and null value handling (Lines 234-240)
3. **Request Structs:** Removed dead `UpdateSettingsRequest`, clarified typed vs. generic approaches (Lines 300-320)
4. **System Settings Validation:** Added allowlist documentation for allowed keys (Lines 300-320, 264)
5. **Timezone Validation:** Added guidance to use `time.LoadLocation()` for validation (Line 263)
6. **File Count Typo:** Updated from "9" to "10" new files (Line 660)

---

---

## Requirements Summary

Implement a flexible settings system with two tiers:

1. **User Settings** - Personal preferences per user per organization
   - Theme (light/dark/system)
   - Language (localization)
   - Timezone
   - Notification toggles
   - Email digest frequency

2. **System Settings** - Global application configuration (admin-only)
   - Application branding (name, logo, colors)
   - Feature toggles (maintenance mode, beta features)
   - Rate limits and timeouts
   - Email configuration defaults
   - Storage bucket defaults
   - Security policies

### Key Constraints

- User settings scoped to user + organization (multi-tenant)
- System settings scoped to organization (org can have different config)
- User settings override system defaults
- Admin-only permission for system settings
- No complex business logic (CRUD-focused)
- Settings stored as JSONB (flexible schema evolution)
- Audit logging for all mutations

---

## Acceptance Criteria

### Functional
- [ ] User can retrieve their settings for current organization
- [ ] User can update their own settings (theme, language, timezone, notifications)
- [ ] User updates trigger audit log with before/after JSONB
- [ ] Organization admin can retrieve system settings
- [ ] Organization admin can update system settings (email config, feature flags, etc.)
- [ ] System settings mutation triggers audit log
- [ ] User settings inherit unset keys from system defaults
- [ ] Settings endpoints return 403 for unauthorized users
- [ ] Settings endpoints return 401 for unauthenticated requests
- [ ] Settings are organization-scoped (user A cannot see user B's settings in different org)

### Non-Functional
- [ ] All handlers have Swagger annotations (matching NewsHandler pattern)
- [ ] All requests validated with go-playground/validator tags
- [ ] Response envelope uses standard success/error format
- [ ] Database queries use indexes on (user_id, org_id) and (org_id)
- [ ] No N+1 queries in settings retrieval
- [ ] Settings changes propagate via audit log (observable for compliance)

### Test Coverage
- [ ] User can get and update own settings
- [ ] Organization admin can get and update system settings
- [ ] User isolation: cannot access other user's settings
- [ ] Permission validation: non-admin cannot access system settings
- [ ] Unauthenticated requests return 401
- [ ] Settings inherit system defaults
- [ ] Invalid settings rejected with validation errors
- [ ] Audit logging captures before/after values

---

## Implementation Steps

### Step 1: Database Schema & Domain Entities

**Files to create:**
- `migrations/000013_settings.up.sql`
- `migrations/000013_settings.down.sql`
- `internal/domain/user_settings.go`
- `internal/domain/system_settings.go`

#### 1A. Migration `000013_settings.up.sql`

```sql
-- User settings table
CREATE TABLE IF NOT EXISTS user_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Settings as JSONB for flexibility
    -- Structure: {theme, language, timezone, notifications_enabled, email_digest_enabled}
    settings JSONB NOT NULL DEFAULT '{}',
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    
    CONSTRAINT uq_user_settings_org_user UNIQUE (organization_id, user_id)
);

-- System settings table (org-wide configuration)
CREATE TABLE IF NOT EXISTS system_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    -- Settings as JSONB: {app_name, logo_url, maintenance_mode, rate_limits, email_config, ...}
    settings JSONB NOT NULL DEFAULT '{}',
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    
    CONSTRAINT uq_system_settings_org UNIQUE (organization_id)
);

-- Indexes
CREATE INDEX idx_user_settings_org_id ON user_settings(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_user_settings_user_id ON user_settings(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_system_settings_org_id ON system_settings(organization_id) WHERE deleted_at IS NULL;

-- Triggers for updated_at
CREATE TRIGGER update_user_settings_updated_at
    BEFORE UPDATE ON user_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_system_settings_updated_at
    BEFORE UPDATE ON system_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE user_settings IS 'Per-user organization-scoped preferences with JSONB flexibility';
COMMENT ON TABLE system_settings IS 'Organization-wide system configuration (admin-only)';
```

#### 1B. Domain Entity `internal/domain/user_settings.go`

Define:
- `UserSettings` struct with ID, OrganizationID, UserID, Settings (datatypes.JSONType), CreatedAt, UpdatedAt, DeletedAt
- `TableName()` returning `"user_settings"`
- `UserSettingsResponse` for API responses with Settings unmarshaled to map[string]interface{}
- `ToResponse()` method

**Pattern reference:** `internal/domain/notification_preference.go:12-37`

#### 1C. Domain Entity `internal/domain/system_settings.go`

Define:
- `SystemSettings` struct with ID, OrganizationID, Settings (datatypes.JSONType), CreatedAt, UpdatedAt, DeletedAt
- `TableName()` returning `"system_settings"`
- `SystemSettingsResponse` for API responses
- `ToResponse()` method

**Acceptance Criteria:**
- [ ] Migration runs cleanly on fresh database
- [ ] Down migration drops both tables
- [ ] Domain entities compile with GORM tags
- [ ] `ToResponse()` methods produce valid JSON-serializable structs
- [ ] Unique constraints prevent duplicate entries per org/user

---

### Step 2: Repository Layer

**Files to create:**
- `internal/repository/user_settings.go`
- `internal/repository/system_settings.go`

#### 2A. UserSettingsRepository

**Interface methods:**
```go
type UserSettingsRepository interface {
    Upsert(ctx context.Context, settings *domain.UserSettings) error
    FindByUserIDAndOrgID(ctx context.Context, userID, orgID uuid.UUID) (*domain.UserSettings, error)
    FindByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserSettings, int64, error)
}
```

**Implementation details:**
- `Upsert`: Use `db.Clauses(clause.OnConflict{UpdateAll: true})` for INSERT ... ON CONFLICT (organization_id, user_id) DO UPDATE **SET deleted_at = NULL, settings = ..., updated_at = ...** to handle soft-delete recovery. Pattern: `internal/repository/notification_preference.go:49-55`
- `FindByUserIDAndOrgID`: Return `errors.ErrNotFound` if not found (will trigger creation of defaults)
- Soft delete aware (filter `WHERE deleted_at IS NULL`)

**Pattern reference:** `internal/repository/notification_preference.go:14-100`

#### 2B. SystemSettingsRepository

**Interface methods:**
```go
type SystemSettingsRepository interface {
    Upsert(ctx context.Context, settings *domain.SystemSettings) error
    FindByOrgID(ctx context.Context, orgID uuid.UUID) (*domain.SystemSettings, error)
}
```

**Implementation details:**
- One system settings record per organization
- Upsert ensures only one record exists per org
- Soft delete aware

**Acceptance Criteria:**
- [ ] Both repository interfaces defined with all required methods
- [ ] Implementation structs use `*gorm.DB` and follow constructor pattern
- [ ] All methods use `ctx context.Context` as first parameter
- [ ] Error wrapping uses `errors.WrapInternal()` and `errors.ErrNotFound`
- [ ] `Upsert` uses ON CONFLICT for atomicity
- [ ] Soft delete filtering applied

---

### Step 3: Service Layer

**Files to create:**
- `internal/service/settings.go`

#### 3A. SettingsService

```go
type SettingsService struct {
    userSettingsRepo   repository.UserSettingsRepository
    systemSettingsRepo repository.SystemSettingsRepository
    permissionService  *PermissionService
    enforcer           *permission.Enforcer
    log                *slog.Logger
}
```

**Methods:**

1. **`GetUserSettings(ctx, userID, orgID) (map[string]interface{}, error)`**
   - Retrieve user settings or return nil if not found (will use defaults)
   - No permission checks (users always see their own settings)

2. **`GetEffectiveSettings(ctx, userID, orgID) (map[string]interface{}, error)`**
   - Retrieve system settings as baseline; if not found, return hardcoded defaults
   - Merge user settings on top (shallow merge: user keys override system keys at top level only)
   - If user settings not found, return system defaults only
   - **Merge strategy:** Top-level key replacement (not deep merge). If user sets `theme: null`, that key is removed from user settings so system default takes over.
   - **Error handling:** If `systemSettingsRepo.FindByOrgID()` returns `ErrNotFound`, use hardcoded defaults instead of failing

3. **`UpdateUserSettings(ctx, userID, orgID, updates map[string]interface{}) (map[string]interface{}, error)`**
   - Validate input (allowed keys: theme, language, timezone, notifications_enabled, email_digest_enabled)
   - Upsert to database
   - Return updated effective settings
   - Log via AuditService (before/after JSONB)

4. **`GetSystemSettings(ctx, orgID) (map[string]interface{}, error)`**
   - Retrieve system settings for organization
   - Called by handlers which enforce admin permission

5. **`UpdateSystemSettings(ctx, orgID, updates map[string]interface{}) (map[string]interface{}, error)`**
   - Validate input (allowed keys: app_name, logo_url, maintenance_mode, rate_limits, email_config, etc.)
   - Upsert to database
   - Log via AuditService (before/after)

**Validation rules:**
- theme: oneof=light dark system
- language: oneof=en es fr de (extendable)
- timezone: valid IANA timezone string — validate using `time.LoadLocation(tzString)` in the `UpdateUserSettings()` method. Return 400 with "Invalid timezone" if LoadLocation fails.
- notifications_enabled: boolean
- email_digest_enabled: boolean
- System settings: flexible but validated against allowlist in code comments (app_name, logo_url, maintenance_mode, rate_limits, email_config)

**Default system settings per new org:**
```json
{
  "app_name": "API Base",
  "maintenance_mode": false,
  "rate_limits": {
    "requests_per_minute": 60,
    "notifications_per_hour": 100
  },
  "email_config": {
    "from_address": "noreply@example.com",
    "from_name": "API Base"
  }
}
```

**Acceptance Criteria:**
- [ ] Service retrieves and merges settings correctly
- [ ] User settings update validated (no injection)
- [ ] System settings update validated (admin-only check done in handler)
- [ ] AuditService logs all mutations with before/after JSONB
- [ ] Defaults properly merged (user overrides don't erase system defaults)

---

### Step 4: HTTP Layer

**Files to create:**
- `internal/http/request/settings.go`
- `internal/http/handler/settings.go`

#### 4A. Request Structs `internal/http/request/settings.go`

```go
// User settings update request (strongly typed for user preferences)
type UserSettingsRequest struct {
    Theme                 *string `json:"theme" validate:"omitempty,oneof=light dark system"`
    Language              *string `json:"language" validate:"omitempty,oneof=en es fr de"`
    Timezone              *string `json:"timezone" validate:"omitempty"` // Validated at service layer with time.LoadLocation()
    NotificationsEnabled  *bool   `json:"notifications_enabled" validate:"omitempty"`
    EmailDigestEnabled    *bool   `json:"email_digest_enabled" validate:"omitempty"`
}

// System settings update request (generic map for flexibility and future extensibility)
type SystemSettingsRequest struct {
    Settings map[string]interface{} `json:"settings" validate:"required"`
}
```

Add `Validate()` method to both structs using `validate.Struct(r)` pattern.

**System Settings Allowlist (document in code comments):**
Allowed top-level keys for system settings:
- `app_name` (string)
- `logo_url` (string)
- `maintenance_mode` (boolean)
- `rate_limits` (object: `{requests_per_minute: int, notifications_per_hour: int}`)
- `email_config` (object: `{from_address: string, from_name: string}`)
- Future extensibility: document that new keys are added to the allowlist in code comments

**Pattern reference:** `internal/http/request/notification.go:4-20`

#### 4B. SettingsHandler `internal/http/handler/settings.go`

```go
type SettingsHandler struct {
    service  *service.SettingsService
    enforcer *permission.Enforcer
    auditSvc *service.AuditService
}

func NewSettingsHandler(service *service.SettingsService, enforcer *permission.Enforcer, auditSvc *service.AuditService) *SettingsHandler {
    return &SettingsHandler{
        service:  service,
        enforcer: enforcer,
        auditSvc: auditSvc,
    }
}
```

**Endpoints:**

1. **`GetUserSettings(c echo.Context) error`** -- `GET /api/v1/settings/user`
   - Extract userID from JWT via `middleware.GetUserID(c)`
   - Extract orgID from `X-Organization-ID` header via `middleware.GetOrganizationID(c)` (returns `uuid.Nil, false` if missing)
   - Return 400 if orgID missing: `"X-Organization-ID header required"`
   - Call `service.GetEffectiveSettings(ctx, userID, orgID)`
   - Return 200 with settings response envelope
   - No permission check (users see their own settings)
   - Swagger annotations: @Summary, @Tags settings, @Security BearerAuth, @Success 200, @Failure 400/401/500

2. **`UpdateUserSettings(c echo.Context) error`** -- `PUT /api/v1/settings/user`
   - Extract userID from JWT via `middleware.GetUserID(c)`
   - Extract orgID from `X-Organization-ID` header via `middleware.GetOrganizationID(c)`
   - Return 400 if orgID missing
   - Bind & validate `UserSettingsRequest`
   - Convert request to map[string]interface{}
   - Call `service.UpdateUserSettings(ctx, userID, orgID, updates)`
   - Return 200 with updated effective settings
   - Swagger annotations: @Summary, @Tags settings, @Param request body, @Security BearerAuth, @Success 200, @Failure 400/401/500

3. **`GetSystemSettings(c echo.Context) error`** -- `GET /api/v1/settings/system`
   - Extract userID from JWT via `middleware.GetUserID(c)`
   - Extract orgID from `X-Organization-ID` header via `middleware.GetOrganizationID(c)`
   - Return 400 if orgID missing
   - Check permission: `enforcer.Enforce(userID, orgID, "settings", "view_system")` (admin-only, see Issue #2 fix)
   - Call `service.GetSystemSettings(ctx, orgID)`
   - Return 200 with settings
   - Swagger annotations: @Summary, @Tags settings, @Security BearerAuth, @Failure 400/403 Forbidden/500

4. **`UpdateSystemSettings(c echo.Context) error`** -- `PUT /api/v1/settings/system`
   - Extract userID from JWT via `middleware.GetUserID(c)`
   - Extract orgID from `X-Organization-ID` header via `middleware.GetOrganizationID(c)`
   - Return 400 if orgID missing
   - Check permission: `enforcer.Enforce(userID, orgID, "settings", "manage_system")` (admin-only, see Issue #2 fix)
   - Bind & validate `SystemSettingsRequest`
   - Call `service.UpdateSystemSettings(ctx, orgID, updates)`
   - Return 200 with updated settings
   - Swagger annotations: @Summary, @Tags settings, @Param request body, @Security BearerAuth, @Failure 400/403 Forbidden/500

**Error handling pattern:** Use `apperrors.GetAppError(err)` to map to HTTP status.

**Acceptance Criteria:**
- [ ] All 4 handler methods extract userID from JWT via `middleware.GetUserID(c)` **(CRITICAL FIX per HIGH Issue #1)**
- [ ] All 4 handler methods extract orgID from `X-Organization-ID` header via `middleware.GetOrganizationID(c)` and return 400 if missing **(CRITICAL FIX per HIGH Issue #1)**
- [ ] User settings endpoints have no permission checks (users manage their own)
- [ ] System settings endpoints enforce admin permission via `enforcer.Enforce(..., "settings", "view_system")` and `enforcer.Enforce(..., "settings", "manage_system")` **(CRITICAL FIX per HIGH Issue #2)**
- [ ] All requests bound/validated with `go-playground/validator` tags
- [ ] All responses use `response.Envelope` format
- [ ] Error responses use `response.ErrorWithContext()` pattern
- [ ] Swagger annotations present on all handler methods

---

### Step 5: Application Wiring

**Files to modify:**
- `internal/http/server.go` -- add `RegisterSettingsRoutes()` method and call from `RegisterRoutes()`
- `cmd/api/main.go` -- add settings permissions to seed data

#### 5A. Route Registration `internal/http/server.go`

Add `RegisterSettingsRoutes()` method after `RegisterNotificationRoutes()`:

```go
func (s *Server) RegisterSettingsRoutes(api *echo.Group, settingsHandler *handler.SettingsHandler) {
    settings := api.Group("/settings")
    
    // Extract organization ID from X-Organization-ID header (REQUIRED)
    settings.Use(middleware.ExtractOrganizationID())
    
    // JWT authentication
    settings.Use(middleware.JWT(middleware.JWTConfig{
        Secret:     s.config.JWT.Secret,
        ContextKey: "user",
    }))

    // Apply audit middleware to mutating routes
    if s.auditSvc != nil {
        settings.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
            Skipper:      middleware.DefaultAuditSkipper(),
            AuditService: s.auditSvc,
        }))
    }

    // User settings (no permission check - users manage their own)
    settings.GET("/user", settingsHandler.GetUserSettings)
    settings.PUT("/user", settingsHandler.UpdateUserSettings)

    // System settings (admin-only with distinct permission actions per Issue #2)
    settings.GET("/system", 
        middleware.RequirePermission(s.enforcer, "settings", "view_system"), 
        settingsHandler.GetSystemSettings)
    settings.PUT("/system", 
        middleware.RequirePermission(s.enforcer, "settings", "manage_system"), 
        settingsHandler.UpdateSystemSettings)
}
```

In `RegisterRoutes()`, add before `RegisterNotificationRoutes()`:

```go
userSettingsRepo := repository.NewUserSettingsRepository(s.db)
systemSettingsRepo := repository.NewSystemSettingsRepository(s.db)
settingsService := service.NewSettingsService(userSettingsRepo, systemSettingsRepo, permissionService, s.enforcer, slog.Default())
settingsHandler := handler.NewSettingsHandler(settingsService, s.enforcer, s.auditSvc)
s.RegisterSettingsRoutes(v1, settingsHandler)
```

#### 5B. Seed Permissions `cmd/api/main.go`

Add to `permissions` slice in `runSeed()` (around line 702). **NOTE: Issue #2 fix — use distinct action names for system settings to avoid Casbin policy ambiguity:**

```go
// Settings permissions (user-level)
{"settings:view", "settings", "view", "own", "View own settings", false},
{"settings:manage", "settings", "manage", "own", "Manage own settings", false},

// Settings permissions (system/admin-level)
{"settings:view-system", "settings", "view_system", "organization", "View organization settings (admin)", false},
{"settings:manage-system", "settings", "manage_system", "organization", "Manage organization settings (admin)", false},
```

Assign to roles:
- **Admin:** all four permissions (settings:view, settings:manage, settings:view-system, settings:manage-system)
- **Viewer:** settings:view + settings:view-system (read-only access)

**Acceptance Criteria:**
- [ ] `RegisterSettingsRoutes()` follows same pattern as `RegisterNotificationRoutes()`
- [ ] Route group includes `middleware.ExtractOrganizationID()` **BEFORE** JWT middleware **(CRITICAL per HIGH Issue #1)**
- [ ] Route group uses JWT middleware and RequirePermission only for system settings
- [ ] User/System settings repos initialized in `RegisterRoutes()`
- [ ] Seed data includes settings permissions with distinct action names: `view`/`manage` (user level) and `view_system`/`manage_system` (admin level) **(CRITICAL per HIGH Issue #2)**
- [ ] RequirePermission calls use correct action names: `view_system` and `manage_system` for system settings
- [ ] User settings endpoints have NO RequirePermission middleware
- [ ] Admin and Viewer roles assigned correct permissions

---

### Step 6: Integration Tests

**Files to create:**
- `tests/integration/settings_handler_test.go`

**Test suites (following `notification_handler_test.go` pattern):**

1. **TestSettingsHandler_GetUserSettings**
   - User gets own settings
   - Returns merged system defaults + user overrides
   - Unauthenticated returns 401

2. **TestSettingsHandler_UpdateUserSettings**
   - User updates own settings
   - Settings are persisted
   - Audit log captures before/after
   - Invalid theme/language rejected with 400
   - Unauthenticated returns 401

3. **TestSettingsHandler_GetSystemSettings**
   - Organization admin can get system settings
   - Non-admin returns 403
   - Unauthenticated returns 401

4. **TestSettingsHandler_UpdateSystemSettings**
   - Organization admin can update system settings
   - Changes persisted
   - Non-admin returns 403
   - Unauthenticated returns 401

5. **TestSettingsHandler_UserIsolation**
   - User A cannot access User B's settings
   - User B's updates don't affect User A's merged settings

6. **TestSettingsHandler_SettingsMerge**
   - System defaults provide initial values
   - User settings override system defaults
   - Unsetting a user setting falls back to system default

**Use helpers:**
- `helpers.NewTestServer(t, suite)` for server setup
- `helpers.MakeRequest(t, server, method, path, body, token)` for HTTP calls
- `helpers.ParseEnvelope(t, rec)` for response parsing
- `helpers.CreateUserWithPermissions(t, suite, enforcer, perms)` for authenticated users

**Acceptance Criteria:**
- [ ] At least 6 test functions covering all endpoints
- [ ] Tests use testcontainers (PostgreSQL + Redis)
- [ ] Tests verify user isolation (A cannot see B's settings)
- [ ] Tests verify permission enforcement (non-admin cannot update system settings)
- [ ] Tests verify settings merging (user overrides system defaults)
- [ ] Tests verify 401 for unauthenticated requests
- [ ] All handler methods have complete Swagger annotations

---

### Step 7: Verification & Documentation

**Pre-commit checklist:**

1. **Build validation:**
   ```bash
   go build ./cmd/api
   go test ./internal/... -count=1
   go test -tags=integration ./tests/integration -run TestSettings
   ```

2. **Migration validation:**
   ```bash
   make migrate  # Should not error
   make migrate-down  # Should drop both tables cleanly
   ```

3. **Swagger regeneration:**
   ```bash
   swag init -g cmd/api/main.go -o docs/swagger
   ```
   Check: `/api/v1/settings/user` and `/api/v1/settings/system` endpoints present

4. **Postman/Manual testing:**
   - Create user account
   - GET /api/v1/settings/user → returns system defaults
   - PUT /api/v1/settings/user → update theme to "dark"
   - GET /api/v1/settings/user → returns merged settings (dark theme)
   - GET /api/v1/settings/system (non-admin) → 403
   - Promote user to admin
   - GET /api/v1/settings/system → returns system settings
   - PUT /api/v1/settings/system → update app_name
   - GET /api/v1/audit-logs → verify settings changes logged

**Acceptance Criteria:**
- [ ] All builds without errors
- [ ] All tests pass (unit + integration)
- [ ] Migrations reversible (up + down)
- [ ] Swagger docs regenerate cleanly
- [ ] Manual testing confirms expected behavior
- [ ] Audit logs capture settings changes
- [ ] No breaking changes to existing APIs

---

## RALPLAN-DR Summary

### Principles

1. **Flexibility over schema rigidity** - JSONB settings allow organic evolution without migrations
2. **Multi-tenant isolation** - Every setting scoped to organization (supports future white-label)
3. **Inheritance with override** - System defaults provide baseline; user settings selectively override
4. **Audit trail for compliance** - All mutations logged with before/after values
5. **Permission-driven access** - Admin-only for system settings; users own their preferences

### Decision Drivers

1. **Reduce schema churn** - JSONB vs fixed columns: JSONB avoids migration per new setting type (reduces risk, increases velocity)
2. **Multi-tenant support** - Organization-scoped settings align with existing org structure (reuses Casbin domain model)
3. **Ease of integration** - Low complexity (1 day), no new patterns, leverages existing middleware/audit/permission stack

### Viable Options

**Option A: JSONB Settings (Recommended)**
- **Approach:** Store all settings as JSONB documents; validate keys server-side
- **Pros:** 
  - New setting types added without migration
  - Supports nested config (rate_limits.requests_per_minute)
  - Aligns with Organization.Settings pattern already in codebase
  - Fast prototyping for features like feature flags
- **Cons:** 
  - No database-level schema enforcement (rely on app validation)
  - Requires app-level documentation of allowed keys
  - Potentially larger rows if many settings stored
- **Evidence:** Organization entity uses `settings datatypes.JSON` at `internal/domain/organization.go:14`

**Option B: Strict Columnar Schema**
- **Approach:** Create columns for each setting (theme, language, timezone, etc.)
- **Pros:**
  - Database enforces schema (NOT NULL constraints, domain types)
  - Clear contracts in schema docs
  - Simpler validation (PostgreSQL handles some types)
- **Cons:**
  - New setting requires migration (blocks releases, risky)
  - Nullable columns for optional settings (schema clutter)
  - System settings table would need 20+ columns (bloat)
  - Breaks future extensibility (white-label features)
- **Not recommended:** Feature recommendations document explicitly calls for JSONB + flexibility

**Option C: Hybrid (Columns + JSONB)**
- **Approach:** Fixed columns for core settings (theme, language, timezone); JSONB for extensible config
- **Pros:**
  - Database validation for common settings
  - Extensibility via JSONB for custom org settings
- **Cons:**
  - Added complexity (two storage paths, inconsistent query patterns)
  - Two merge algorithms (column overrides + JSONB overrides)
  - Not worth the extra code for 1-day feature
- **Not recommended:** Over-engineered for current scope

**Decision:** **Option A (JSONB Settings)** — aligns with existing patterns, minimizes scope, maximizes future flexibility.

---

## Risk Identification & Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|-----------|------------|
| **Settings mutation without audit trail** | Compliance violation (who changed what) | Low | Audit middleware on all mutation routes; test audit logging in integration tests |
| **System settings not org-scoped** | Multi-tenant data leak (org A sees org B's config) | Low | Foreign key constraint on organization_id; test user isolation in tests |
| **Permission check bypassed for system settings** | Non-admin users modify system settings | Low | Handler enforces permission via `enforcer.Enforce()` on GET/PUT; integration tests verify 403 for non-admin |
| **Settings inheritance loop** (e.g., system setting references user setting) | Circular dependency / infinite merge | Very Low | Document that system settings are immutable defaults; user settings are user-only overrides (one-way inheritance) |
| **Large settings JSONB doc** | Slow queries or storage bloat | Low | Document key count limits in code comments; index on (organization_id) for fast lookups; JSONB is indexed (GIN by default in PostgreSQL) |
| **Unbounded JSON keys from user** | Injection / DoS (user sends 1MB settings doc) | Medium | Request body size limit enforced by Echo framework; validate allowed keys in service layer; test with oversized payloads |

---

## Success Criteria

1. ✅ Both user and system settings endpoints respond with correct envelope format
2. ✅ User settings are user-scoped (no cross-user data leakage)
3. ✅ System settings are organization-scoped (no cross-org leakage)
4. ✅ User settings override system defaults in `GetEffectiveSettings()`
5. ✅ All settings mutations trigger audit log with before/after JSONB
6. ✅ System settings endpoints return 403 for non-admin users
7. ✅ Unset user settings fall back to system defaults
8. ✅ Integration tests pass (all 6 test suites)
9. ✅ Swagger docs regenerate cleanly with /settings endpoints
10. ✅ No breaking changes to existing services
11. ✅ Migration is reversible (up + down)
12. ✅ Permissions seeded (settings:view, settings:manage)
13. ✅ Endpoints protected with JWT middleware

---

## File Inventory

### New Files (10)
| File | Purpose |
|------|---------|
| `migrations/000013_settings.up.sql` | Create user_settings + system_settings tables with JSONB columns |
| `migrations/000013_settings.down.sql` | Drop both tables |
| `internal/domain/user_settings.go` | UserSettings entity, response struct, ToResponse() |
| `internal/domain/system_settings.go` | SystemSettings entity, response struct, ToResponse() |
| `internal/repository/user_settings.go` | UserSettingsRepository interface + implementation |
| `internal/repository/system_settings.go` | SystemSettingsRepository interface + implementation |
| `internal/service/settings.go` | SettingsService with GetUserSettings, UpdateUserSettings, GetEffectiveSettings, etc. |
| `internal/http/request/settings.go` | UpdateSettingsRequest, UserSettingsRequest, SystemSettingsRequest with validation |
| `internal/http/handler/settings.go` | SettingsHandler with Get/Update endpoints for user and system settings |
| `tests/integration/settings_handler_test.go` | 6 test suites covering all endpoints and isolation |

### Modified Files (3)
| File | Change |
|------|--------|
| `internal/http/server.go` | Add `RegisterSettingsRoutes()` method; wire repos/service/handler in `RegisterRoutes()` |
| `cmd/api/main.go` | Add settings:view, settings:manage permissions to seed; assign to admin/viewer roles |
| `docs/swagger/docs.go`, `docs/swagger/swagger.json`, `docs/swagger/swagger.yaml` | Auto-regenerated after swag init |

**Total: 9 new files, 3 modified files**

---

## Recommended Execution Path

1. **Start with domain + migration** (Step 1) — Validate schema with database migration
2. **Build repositories** (Step 2) — Ensure GORM queries work correctly
3. **Implement service layer** (Step 3) — Core business logic for merging settings
4. **Create HTTP handlers** (Step 4) — Wire everything together
5. **Register routes & permissions** (Step 5) — Integrate with existing server
6. **Write integration tests** (Step 6) — Comprehensive coverage before commit
7. **Manual verification** (Step 7) — Postman/cURL testing

---

## Notes

- All domain entities follow existing UUID PK + GORM soft delete + JSONB patterns from codebase
- Repository interfaces match UserRepository and NotificationPreferenceRepository style
- Service layer leverages existing PermissionService for admin checks (no new abstractions)
- Handlers follow NewsHandler and NotificationHandler patterns for consistency
- Permission model reuses Casbin Resource/Action/Scope structure
- Audit middleware automatically logs mutations (inherited from existing middleware)
- No external dependencies beyond existing libraries (GORM, validator, Casbin)
- JSONB allows future extensibility (feature flags, org-specific settings, white-label) without code changes

---

## Implementation Time Breakdown

- **Domain + Migration:** 20 min
- **Repositories:** 25 min
- **Service Layer:** 30 min
- **HTTP Layer (Requests + Handlers):** 35 min
- **Routes + Permissions:** 15 min
- **Integration Tests:** 40 min
- **Manual Testing + Docs:** 15 min

**Total: ~3 hours (well within 1-day estimate)**
