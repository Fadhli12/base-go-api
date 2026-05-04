# Settings Feature Documentation

## Overview

The Settings feature provides a two-tier configuration system for the Go API Base project:

1. **System Settings** - Organization-wide configuration (maintenance mode, rate limits, email config, etc.)
2. **User Settings** - Per-user preferences within an organization (theme, language, timezone, etc.)

Settings use flexible JSONB storage for schema evolution without migrations.

## Architecture

### Domain Models

#### SystemSettings
```go
type SystemSettings struct {
    ID             uuid.UUID          // Primary key
    OrganizationID uuid.UUID          // Org scope (unique constraint)
    Settings       datatypes.JSON     // JSONB flexible schema
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      gorm.DeletedAt     // Soft delete support
}
```

#### UserSettings
```go
type UserSettings struct {
    ID             uuid.UUID          // Primary key
    OrganizationID uuid.UUID          // Org scope (composite unique key)
    UserID         uuid.UUID          // User scope (composite unique key)
    Settings       datatypes.JSON     // JSONB flexible schema
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      gorm.DeletedAt     // Soft delete support
}
```

### Repository Layer

**UserSettingsRepository** (`internal/repository/user_settings.go`):
- `Upsert(ctx, settings)` - Create or update with soft-delete recovery
- `FindByUserIDAndOrgID(ctx, userID, orgID)` - Fetch user settings
- `FindByOrgID(ctx, orgID, limit, offset)` - Paginated list by organization

**SystemSettingsRepository** (`internal/repository/system_settings.go`):
- `Upsert(ctx, settings)` - Create or update with soft-delete recovery
- `FindByOrgID(ctx, orgID)` - Fetch organization settings

### Service Layer

**SettingsService** (`internal/service/settings.go`):

#### User Settings
- `GetUserSettings(ctx, userID, orgID)` - Retrieve user settings (returns empty if not found)
- `UpdateUserSettings(ctx, userID, orgID, updates)` - Update with timezone validation

#### System Settings
- `GetSystemSettings(ctx, orgID)` - Retrieve with hardcoded defaults fallback
- `UpdateSystemSettings(ctx, orgID, updates)` - Update with allowlist validation

#### Merged Settings
- `GetEffectiveSettings(ctx, userID, orgID)` - Merge system + user settings (user takes precedence)

### HTTP Layer

**SettingsHandler** (`internal/http/handler/settings.go`):

Endpoints (all require JWT authentication + X-Organization-ID header):

| Method | Endpoint | Handler | Permission |
|--------|----------|---------|-----------|
| GET | `/api/v1/settings/user` | GetUserSettings | settings:view_user |
| PUT | `/api/v1/settings/user` | UpdateUserSettings | settings:manage_user |
| GET | `/api/v1/settings/effective` | GetEffectiveSettings | settings:view_user |
| GET | `/api/v1/settings/system` | GetSystemSettings | settings:view_system |
| PUT | `/api/v1/settings/system` | UpdateSystemSettings | settings:manage_system |

## Default System Settings

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

## Permissions

Defined in `cmd/api/main.go` DefaultPermissions():

```yaml
permissions:
  - name: settings:view_user
    resource: settings
    action: view_user
  - name: settings:manage_user
    resource: settings
    action: manage_user
  - name: settings:view_system
    resource: settings
    action: view_system
  - name: settings:manage_system
    resource: settings
    action: manage_system
```

Admin role includes all settings permissions.

## Database Schema

### user_settings table
- `id` (UUID, PK)
- `organization_id` (UUID, indexed, part of composite unique key)
- `user_id` (UUID, indexed, part of composite unique key)
- `settings` (JSONB, NOT NULL, default='{}')
- `created_at` (TIMESTAMP)
- `updated_at` (TIMESTAMP, auto-updated trigger)
- `deleted_at` (TIMESTAMP, soft-delete index)

Unique constraint: `(organization_id, user_id)`

### system_settings table
- `id` (UUID, PK)
- `organization_id` (UUID, indexed, unique)
- `settings` (JSONB, NOT NULL, default='{}')
- `created_at` (TIMESTAMP)
- `updated_at` (TIMESTAMP, auto-updated trigger)
- `deleted_at` (TIMESTAMP, soft-delete index)

Unique constraint: `organization_id`

## Usage Examples

### Get User Settings
```bash
curl -H "Authorization: Bearer $TOKEN" \
  -H "X-Organization-ID: $ORG_ID" \
  http://localhost:8080/api/v1/settings/user
```

### Update User Settings
```bash
curl -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Organization-ID: $ORG_ID" \
  -H "Content-Type: application/json" \
  -d '{"settings": {"theme": "dark", "language": "en", "timezone": "America/New_York"}}' \
  http://localhost:8080/api/v1/settings/user
```

### Get Effective Settings (Merged)
```bash
curl -H "Authorization: Bearer $TOKEN" \
  -H "X-Organization-ID: $ORG_ID" \
  http://localhost:8080/api/v1/settings/effective
```

### Get System Settings
```bash
curl -H "Authorization: Bearer $TOKEN" \
  -H "X-Organization-ID: $ORG_ID" \
  http://localhost:8080/api/v1/settings/system
```

### Update System Settings (Admin Only)
```bash
curl -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Organization-ID: $ORG_ID" \
  -H "Content-Type: application/json" \
  -d '{"settings": {"maintenance_mode": true}}' \
  http://localhost:8080/api/v1/settings/system
```

## Validation

### User Settings
- **Timezone**: Must be a valid IANA timezone (validated with `time.LoadLocation()`)
- Returns 422 VALIDATION_ERROR if invalid

### System Settings
- **Allowlist**: Only specific fields can be updated:
  - maintenance_mode
  - rate_limits
  - email_config
  - notifications_per_hour
  - requests_per_minute
  - from_address
  - from_name
- Returns 422 VALIDATION_ERROR if no valid fields provided

## Settings Inheritance

When retrieving effective settings via `/settings/effective`:

1. Load system settings (or hardcoded defaults if not found)
2. Load user settings (or empty if not found)
3. Merge: user settings override system settings at the root level (shallow merge)
4. Return merged JSON

Example:
```json
// System settings
{
  "app_name": "API Base",
  "maintenance_mode": false,
  "theme": "light"
}

// User settings
{
  "theme": "dark"
}

// Effective (user overrides system)
{
  "app_name": "API Base",          // From system
  "maintenance_mode": false,       // From system
  "theme": "dark"                  // From user (overrides)
}
```

## Soft Delete Support

Both tables support soft delete with recovery:

- Settings are marked with `deleted_at` timestamp
- Upsert operations reset `deleted_at = NULL` to recover from soft delete
- FindBy* queries filter out soft-deleted records
- Provides audit trail while maintaining clean queries

## Testing

Integration tests in `tests/integration/settings_test.go`:

```bash
# Run all settings tests
go test -tags=integration ./tests/integration/... -run Settings -v

# Run specific test
go test -tags=integration ./tests/integration/... -run TestUserSettings_UpdateWithValidData -v
```

Test coverage:
- Empty settings retrieval
- Valid updates
- Timezone validation
- Soft delete recovery
- System settings defaults
- Allowlist validation
- Settings merging and precedence

## Migration

Migration: `migrations/000013_settings.up.sql`

Includes:
- user_settings table creation
- system_settings table creation
- Indexes on organization_id, user_id
- Auto-update triggers for updated_at

Rollback: `migrations/000013_settings.down.sql`

## Files Modified/Created

**New Files:**
- `internal/domain/user_settings.go`
- `internal/domain/system_settings.go`
- `internal/repository/user_settings.go`
- `internal/repository/system_settings.go`
- `internal/service/settings.go`
- `internal/http/handler/settings.go`
- `internal/http/request/settings.go`
- `tests/integration/settings_test.go`
- `migrations/000013_settings.up.sql`
- `migrations/000013_settings.down.sql`

**Modified Files:**
- `cmd/api/main.go` - Added settings permissions to DefaultPermissions()
- `internal/http/server.go` - Added SettingsService initialization and route registration

## Error Handling

All endpoints return standardized error responses:

```json
{
  "error": {
    "code": "VALIDATION_ERROR|NOT_FOUND|UNAUTHORIZED|INTERNAL_ERROR",
    "message": "Human-readable error message",
    "request_id": "unique-request-id"
  }
}
```

HTTP Status Codes:
- 200: Success
- 400: Bad Request (invalid JSON)
- 401: Unauthorized (missing JWT or org context)
- 403: Forbidden (insufficient permissions)
- 422: Unprocessable Entity (validation failed)
- 500: Internal Server Error

## Future Enhancements

1. **Settings Versioning** - Track setting changes over time
2. **Audit Trail** - Log who changed what settings and when
3. **Settings Templates** - Pre-defined setting profiles for organizations
4. **Nested Merge** - Deep merge for nested JSON settings
5. **Settings Validation Schema** - Define allowed fields/types per org
6. **Real-time Updates** - WebSocket/SSE for setting change notifications
