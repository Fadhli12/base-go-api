# Notification System Implementation Plan

**Feature:** In-app Notification System with optional email delivery
**Complexity:** Medium | **Estimated Effort:** 2 days
**Date:** 2026-05-01
**Dependencies:** Email Service (exists at `internal/service/email.go`)

---

## Context

The Go API Base project needs an in-app notification system for user activities, mentions, alerts, and system events. The system must store notifications with read/unread tracking, support per-user notification preferences by type, and optionally route notifications to the existing email service based on user preferences.

**Product Decisions:**
- ✅ **Deletion:** Use `archived_at` column (option B) to hide notifications without losing audit trail
- ✅ **Default Preferences:** Email enabled for system/assignment/mention; disabled for invoice.created/news.published
- ✅ **Rate Limiting:** Implement (option B) max 100 notifications/hour per user to prevent spam
- ✅ **Type Extensibility:** Use VARCHAR without CHECK constraint (option B) for runtime extensibility
- ✅ **MarkAllAsRead Filter:** Support optional `type` query param (option B) to mark specific notification types as read

### Codebase Patterns (Evidence)

| Pattern | Evidence | File Reference |
|---------|----------|----------------|
| UUID primary keys with `gen_random_uuid()` | All domain entities | `internal/domain/user.go:12`, `internal/domain/news.go:35` |
| Soft delete via `gorm.DeletedAt` | User, News, Roles, etc. | `internal/domain/user.go:17`, `internal/domain/news.go:48` |
| No soft delete for audit-trail tables | EmailQueue, AuditLog | `internal/domain/email_queue.go:27` (comment), `internal/domain/audit_log.go:13` |
| Repository interface + struct pattern | All repos define interface then `type xxxRepository struct` | `internal/repository/news.go:12-48`, `internal/repository/email_queue.go:15-43` |
| Service struct with repo + enforcer deps | NewsService, EmailService | `internal/service/news.go:17-20`, `internal/service/email.go:17-24` |
| Handler struct with service + enforcer | NewsHandler, EmailHandler | `internal/http/handler/news.go:19-22`, `internal/http/handler/email.go:17-20` |
| Request validation via `go-playground/validator` | All request structs | `internal/http/request/email.go:33-35` |
| Response envelope pattern | `response.SuccessWithContext()`, `response.ErrorWithContext()` | `internal/http/response/envelope.go:56-64`, `internal/http/response/envelope.go:94-105` |
| Route registration in `server.go` | `RegisterXxxRoutes()` methods on Server | `internal/http/server.go:273-530` |
| JWT + permission middleware chain | `middleware.JWT()` then `middleware.RequirePermission()` | `internal/http/server.go:536-558` |
| Audit middleware on route groups | `middleware.Audit()` applied to mutating routes | `internal/http/server.go:496-499` |
| Migration naming: `000NNN_name.{up,down}.sql` | Sequential numbering | `migrations/000008_email_service.up.sql` |
| Integration tests with testcontainers | PostgreSQL + Redis containers | `tests/integration/testsuite.go:43-123` |
| Test helpers: `NewTestServer`, `MakeRequest`, `CreateUserWithPermissions` | Standardized test patterns | `tests/integration/helpers/handler_test_helpers.go:45,133,198` |
| Const-based type enums with validation func | `NewsStatus`, `EmailStatus` | `internal/domain/news.go:12-21`, `internal/domain/email_queue.go:14-21` |
| `ToResponse()` method on domain entities | Converts domain to API response | `internal/domain/news.go:74-105`, `internal/domain/user.go:34-41` |
| Seed permissions in `cmd/api/main.go` | Raw SQL inserts + Casbin sync | `cmd/api/main.go:649-700` |
| `TableName()` method on domain entities | Explicit table name | `internal/domain/news.go:53`, `internal/domain/email_queue.go:48` |

---

## Work Objectives

1. Create database schema for `notifications` and `notification_preferences` tables
2. Implement domain entities with proper type enums, response conversion, and validation
3. Build repository layer following the interface + struct pattern
4. Build service layer with notification routing logic (store -> check preferences -> queue email)
5. Build HTTP handler with CRUD endpoints and permission checks
6. Wire into the application (routes, DI, seeds, migrations)
7. Write integration tests following the testcontainers pattern

---

## Guardrails

### Must Have
- Follow existing patterns exactly (UUID PKs, repository interface, service struct, handler struct, envelope response)
- Notifications table must NOT use soft delete (audit trail, like `email_queue` and `audit_logs`)
- NotificationPreferences table DOES use soft delete (user-editable config)
- Per-user notification preferences stored per notification type
- Read/unread tracking via `read_at` timestamp (NULL = unread)
- Bulk mark-as-read endpoint
- Integration with existing `EmailService.QueueEmail()` for email delivery
- Integration with existing `AuditService.LogAction()` for audit trail
- Casbin permission entries for `notifications` resource
- All endpoints require JWT authentication
- Users can only see/manage their own notifications and preferences

### Must NOT Have
- WebSocket/real-time push (explicitly marked as "future" in spec)
- Push notification delivery (PushEnabled field stored but not implemented)
- Notification creation HTTP endpoint (notifications are created programmatically by services, not by users)
- Breaking changes to existing services

---

## Task Flow

```
Step 1: Schema & Domain
  |
  v
Step 2: Repository Layer
  |
  v
Step 3: Service Layer (+ email integration)
  |
  v
Step 4: HTTP Layer (requests, handlers, routes)
  |
  v
Step 5: Wiring & Permissions (main.go, server.go, seeds)
  |
  v
Step 6: Integration Tests & Swagger Docs
```

---

## Detailed TODOs

### Step 1: Database Schema & Domain Entities

**Files to create:**
- `migrations/000012_notifications.up.sql`
- `migrations/000012_notifications.down.sql`
- `internal/domain/notification.go`
- `internal/domain/notification_preference.go`

#### 1A. Migration `000012_notifications.up.sql`

```sql
-- notifications table (with archived_at for soft deletion, allowing extensible types)
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    action_url VARCHAR(500),
    read_at TIMESTAMPTZ DEFAULT NULL,
    archived_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- notification_preferences table (soft delete - user config)
CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type VARCHAR(30) NOT NULL,
    email_enabled BOOLEAN DEFAULT true NOT NULL,
    push_enabled BOOLEAN DEFAULT false NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,

    CONSTRAINT uq_user_notification_type UNIQUE (user_id, notification_type),
    CONSTRAINT chk_pref_notification_type CHECK (notification_type IN (
        'mention', 'assignment', 'system',
        'invoice.created', 'news.published'
    ))
);

-- Indexes for notifications
CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_user_read ON notifications(user_id, read_at);
CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_archived ON notifications(user_id, archived_at) WHERE archived_at IS NULL;
CREATE INDEX idx_notifications_type ON notifications(type);

-- Indexes for notification_preferences
CREATE INDEX idx_notification_prefs_user_id ON notification_preferences(user_id)
    WHERE deleted_at IS NULL;

-- Triggers
CREATE TRIGGER update_notification_preferences_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

#### 1B. Domain entity `internal/domain/notification.go`

Define:
- `NotificationType` string enum with constants: `NotificationTypeMention`, `NotificationTypeAssignment`, `NotificationTypeSystem`, `NotificationTypeInvoiceCreated`, `NotificationTypeNewsPublished`
- `IsValidNotificationType()` validation function -- accepts any non-empty string (runtime extensibility per decision 4)
- `Notification` struct with fields: ID (uuid), UserID (uuid), Type (string), Title (string), Message (string), ActionURL (string), ReadAt (*time.Time), ArchivedAt (*time.Time), CreatedAt (time.Time), UpdatedAt (time.Time)
- `TableName()` returning `"notifications"` (pattern: `internal/domain/email_queue.go:48`)
- `NotificationResponse` struct for API responses
- `ToResponse()` method (pattern: `internal/domain/news.go:74-105`)
- `IsRead()` helper returning `n.ReadAt != nil`
- `IsArchived()` helper returning `n.ArchivedAt != nil`
- `MarkRead()` helper setting `ReadAt` to `time.Now()`
- `Archive()` helper setting `ArchivedAt` to `time.Now()`

#### 1C. Domain entity `internal/domain/notification_preference.go`

Define:
- `NotificationPreference` struct with fields: ID (uuid), UserID (uuid), NotificationType (NotificationType), EmailEnabled (bool), PushEnabled (bool), CreatedAt, UpdatedAt, DeletedAt (gorm.DeletedAt)
- `TableName()` returning `"notification_preferences"`
- `NotificationPreferenceResponse` struct
- `ToResponse()` method

**Acceptance Criteria:**
- [ ] Migration runs without errors on a fresh database
- [ ] Down migration drops both tables cleanly
- [ ] Domain entities compile with correct GORM tags
- [ ] `IsValidNotificationType()` returns true for all 5 types and false for invalid ones
- [ ] `ToResponse()` methods produce correct JSON-serializable structs

---

### Step 2: Repository Layer

**Files to create:**
- `internal/repository/notification.go`
- `internal/repository/notification_preference.go`

#### 2A. NotificationRepository (`internal/repository/notification.go`)

Interface (pattern: `internal/repository/news.go:12-36`):
```go
type NotificationRepository interface {
    Create(ctx context.Context, notification *domain.Notification) error
    CreateBatch(ctx context.Context, notifications []*domain.Notification) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
    FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error)
    FindUnreadByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error)
    CountUnreadByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
    MarkAsRead(ctx context.Context, id, userID uuid.UUID) error
    MarkAllAsRead(ctx context.Context, userID uuid.UUID, notifType *string) (int64, error)
    ArchiveByID(ctx context.Context, id, userID uuid.UUID) error
}
```

Implementation struct with `*gorm.DB` field (pattern: `internal/repository/email_queue.go:37-43`).

Key details:
- All FindByUserID queries filter `WHERE archived_at IS NULL` (soft delete awareness)
- `FindByUserID`: ordered by `created_at DESC`, returns total count for pagination
- `FindUnreadByUserID`: filter `WHERE read_at IS NULL AND archived_at IS NULL`, ordered by `created_at DESC`
- `MarkAsRead`: update `read_at = NOW()` where `id = ? AND user_id = ?` (ownership check at DB level)
- `MarkAllAsRead`: update `read_at = NOW()` where `user_id = ? AND read_at IS NULL AND archived_at IS NULL`; if `notifType` provided, add `AND type = ?` filter (decision 5); return affected count
- `ArchiveByID`: update `archived_at = NOW()` where `id = ? AND user_id = ?` (soft delete instead of hard delete)
- Use `errors.WrapInternal()` and `errors.ErrNotFound` (pattern: `internal/repository/email_queue.go:49,62`)

#### 2B. NotificationPreferenceRepository (`internal/repository/notification_preference.go`)

Interface:
```go
type NotificationPreferenceRepository interface {
    Upsert(ctx context.Context, pref *domain.NotificationPreference) error
    FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.NotificationPreference, error)
    FindByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) (*domain.NotificationPreference, error)
    DeleteByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) error
}
```

Key details:
- `Upsert`: use `db.Clauses(clause.OnConflict{...})` for INSERT ... ON CONFLICT (user_id, notification_type) DO UPDATE
- `FindByUserID`: return all preferences for a user (soft delete aware via GORM default)
- `FindByUserIDAndType`: return single preference or `errors.ErrNotFound`

**Acceptance Criteria:**
- [ ] Both repository interfaces are defined with all required methods
- [ ] Implementation structs use `*gorm.DB` and follow constructor pattern `NewXxxRepository(db *gorm.DB) XxxRepository`
- [ ] All methods use `ctx context.Context` as first parameter
- [ ] Error wrapping uses `errors.WrapInternal()` and `errors.ErrNotFound`
- [ ] `MarkAsRead` and `DeleteByID` enforce user ownership at DB query level

---

### Step 3: Service Layer

**Files to create:**
- `internal/service/notification.go`

#### 3A. NotificationService

```go
type NotificationService struct {
    notifRepo    repository.NotificationRepository
    prefRepo     repository.NotificationPreferenceRepository
    emailService *EmailService
    userRepo     repository.UserRepository  // to look up user email for email delivery
    cache        cache.Cache               // for rate limiting (Redis or in-memory)
}
```

Constructor: `NewNotificationService(notifRepo, prefRepo, emailService, userRepo, cache)` (pattern: `internal/service/news.go:23-28`)

Methods:

1. **`Send(ctx, userID, notifType, title, message, actionURL) error`**
   - Core method called by other services to create a notification
   - Validate `notifType` with `IsValidNotificationType()` (accepts any non-empty string per decision 4)
   - **Rate limit check (decision 3):** Check Redis key `notifications:user:{userID}:hourly` for count. If >= 100, return `apperrors.ErrTooManyNotifications`. Increment and set 1-hour TTL.
   - Create notification record via `notifRepo.Create()`
   - Check user preferences via `prefRepo.FindByUserIDAndType()`
   - If preference not found, use defaults (email enabled for system/assignment/mention, disabled for others -- decision 2)
   - If `EmailEnabled == true`, look up user email via `userRepo`, then call `emailService.QueueEmail()` with a notification template
   - Return nil on success (email queuing failures should be logged but not block notification creation)

2. **`SendBulk(ctx, userIDs []uuid.UUID, notifType, title, message, actionURL) error`**
   - Create notifications for multiple users
   - Loop through userIDs calling `Send()` for each
   - Collect and log errors but don't fail on individual user failures

3. **`ListByUser(ctx, userID, limit, offset) ([]*domain.Notification, int64, error)`**
   - Delegate to `notifRepo.FindByUserID()`

4. **`ListUnreadByUser(ctx, userID, limit, offset) ([]*domain.Notification, int64, error)`**
   - Delegate to `notifRepo.FindUnreadByUserID()`

5. **`CountUnread(ctx, userID) (int64, error)`**
   - Delegate to `notifRepo.CountUnreadByUserID()`

6. **`MarkAsRead(ctx, notifID, userID) error`**
   - Delegate to `notifRepo.MarkAsRead()`

7. **`MarkAllAsRead(ctx, userID, notifType *string) (int64, error)`**
   - Delegate to `notifRepo.MarkAllAsRead(userID, notifType)` (accepts optional type filter per decision 5)

8. **`GetPreferences(ctx, userID) ([]*domain.NotificationPreference, error)`**
   - Delegate to `prefRepo.FindByUserID()`

9. **`UpdatePreference(ctx, userID, notifType, emailEnabled, pushEnabled) error`**
   - Validate `notifType`
   - Delegate to `prefRepo.Upsert()`

10. **`ArchiveNotification(ctx, notifID, userID) error`**
    - Delegate to `notifRepo.ArchiveByID()` (soft delete via archived_at column per decision 1)

**Acceptance Criteria:**
- [ ] `Send()` creates a notification AND queues email when preference allows
- [ ] `Send()` succeeds even if email queuing fails (email failure is logged, not propagated)
- [ ] `Send()` uses default preferences when no preference record exists for the user/type
- [ ] `MarkAsRead()` and `DeleteNotification()` enforce user ownership (userID passed to repo)
- [ ] All methods use `errors.NewAppError()` for validation failures (pattern: `internal/service/news.go:35`)
- [ ] Service does not import `permission.Enforcer` (ownership is enforced via userID params, unlike news which has admin override)

---

### Step 4: HTTP Layer

**Files to create:**
- `internal/http/request/notification.go`
- `internal/http/handler/notification.go`

#### 4A. Request structs (`internal/http/request/notification.go`)

```go
type UpdateNotificationPreferenceRequest struct {
    NotificationType string `json:"notification_type" validate:"required,oneof=mention assignment system invoice.created news.published"`
    EmailEnabled     *bool  `json:"email_enabled" validate:"required"`
    PushEnabled      *bool  `json:"push_enabled" validate:"required"`
}
```

Add `Validate()` method using `validate.Struct(r)` (pattern: `internal/http/request/email.go:33-35`).

#### 4B. NotificationHandler (`internal/http/handler/notification.go`)

```go
type NotificationHandler struct {
    service *service.NotificationService
}
```

Constructor: `NewNotificationHandler(service)` (pattern: `internal/http/handler/email.go:24-28`)

Endpoints:

1. **`List(c echo.Context) error`** -- `GET /api/v1/notifications`
   - Extract userID from JWT via `middleware.GetUserID(c)` (pattern: `internal/http/handler/news.go:57-59`)
   - Parse `limit` and `offset` query params (default limit=20, max=100)
   - Parse optional `unread_only` query param (bool)
   - Call `service.ListByUser()` or `service.ListUnreadByUser()`
   - Return paginated response with total count
   - Swagger annotations (pattern: `internal/http/handler/news.go:43-54`)

2. **`CountUnread(c echo.Context) error`** -- `GET /api/v1/notifications/unread-count`
   - Extract userID from JWT
   - Call `service.CountUnread()`
   - Return `{"unread_count": N}`

3. **`MarkAsRead(c echo.Context) error`** -- `PATCH /api/v1/notifications/:id/read`
   - Extract userID from JWT
   - Parse notification ID from path param
   - Call `service.MarkAsRead(notifID, userID)`
   - Return 200 with success envelope

4. **`MarkAllAsRead(c echo.Context) error`** -- `POST /api/v1/notifications/read-all`
   - Extract userID from JWT
   - Parse optional `type` query param (e.g., `?type=system` to mark only system notifications as read)
   - Call `service.MarkAllAsRead(userID, typeFilter)` where typeFilter is nil or string pointer
   - Return 200 with `{"marked_count": N}`

5. **`Archive(c echo.Context) error`** -- `DELETE /api/v1/notifications/:id`
   - Extract userID from JWT
   - Parse notification ID from path param
   - Call `service.ArchiveNotification(notifID, userID)` (soft delete via archived_at per decision 1)
   - Return 200 with success envelope

6. **`GetPreferences(c echo.Context) error`** -- `GET /api/v1/notifications/preferences`
   - Extract userID from JWT
   - Call `service.GetPreferences(userID)`
   - Return array of preference responses

7. **`UpdatePreference(c echo.Context) error`** -- `PUT /api/v1/notifications/preferences`
   - Extract userID from JWT
   - Bind and validate `UpdateNotificationPreferenceRequest`
   - Call `service.UpdatePreference(userID, ...)`
   - Return 200 with updated preference

Error handling pattern: use `apperrors.GetAppError(err)` to map to HTTP status (pattern: `internal/http/handler/news.go:72-76`).

**Acceptance Criteria:**
- [ ] All 7 handler methods extract userID from JWT (never trust client-supplied userID)
- [ ] List endpoint supports pagination with `limit` and `offset` query params
- [ ] List endpoint supports `unread_only=true` filter
- [ ] List endpoint filters archived notifications (archived_at IS NULL)
- [ ] MarkAllAsRead endpoint supports optional `type` query param to filter by notification type
- [ ] Archive endpoint uses archived_at instead of hard delete
- [ ] All endpoints return `response.Envelope` format
- [ ] Request validation uses `go-playground/validator` tags
- [ ] Swagger annotations present on all handler methods
- [ ] Error responses use `response.ErrorWithContext()` pattern

---

### Step 5: Application Wiring

**Files to modify:**
- `internal/http/server.go` -- add `RegisterNotificationRoutes()` method and call it from `RegisterRoutes()`
- `cmd/api/main.go` -- add notification permissions to seed data and permission manifest
- `tests/integration/testsuite.go` -- add notification tables to migration and cleanup

#### 5A. Route Registration (`internal/http/server.go`)

Add `RegisterNotificationRoutes()` method (pattern: `RegisterEmailRoutes()` at line 460-530):

```go
func (s *Server) RegisterNotificationRoutes(api *echo.Group, notifHandler *handler.NotificationHandler) {
    notifications := api.Group("/notifications")
    notifications.Use(middleware.JWT(middleware.JWTConfig{
        Secret:     s.config.JWT.Secret,
        ContextKey: "user",
    }))

    // No RequirePermission middleware -- all users can manage their own notifications
    // Ownership is enforced in the service/repository layer via userID from JWT

    // Apply audit middleware to mutating routes
    if s.auditSvc != nil {
        notifications.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
            Skipper:      middleware.DefaultAuditSkipper(),
            AuditService: s.auditSvc,
        }))
    }

    notifications.GET("", notifHandler.List)
    notifications.GET("/unread-count", notifHandler.CountUnread)
    notifications.PATCH("/:id/read", notifHandler.MarkAsRead)
    notifications.POST("/read-all", notifHandler.MarkAllAsRead) // supports ?type=system filter
    notifications.DELETE("/:id", notifHandler.Archive)
    notifications.GET("/preferences", notifHandler.GetPreferences)
    notifications.PUT("/preferences", notifHandler.UpdatePreference)
}
```

In `RegisterRoutes()`, add after email route registration (around line 455):
- Initialize `notificationRepo`, `notificationPrefRepo`
- Initialize `notificationService` with repos, emailService, userRepo
- Initialize `notificationHandler`
- Call `s.RegisterNotificationRoutes(v1, notifHandler)`

#### 5B. Seed Permissions (`cmd/api/main.go`)

Add to the `permissions` slice (around line 649):
```go
// Notification permissions
{"notifications:view", "notifications", "view", "own", "View own notifications", false},
{"notifications:manage", "notifications", "manage", "own", "Manage own notifications", false},
```

Add `"notifications:view"` to `viewerPermissions` and both to `adminPermissions`.

#### 5C. Test Suite Migration (`tests/integration/testsuite.go`)

Add notification tables to `RunMigrations()` method and add `"notifications"`, `"notification_preferences"` to the cleanup table list in `SetupTest()`.

**Acceptance Criteria:**
- [ ] `RegisterNotificationRoutes()` follows the same pattern as `RegisterEmailRoutes()`
- [ ] Route group uses JWT middleware but NOT RequirePermission (ownership-based access)
- [ ] Notification repositories and service are initialized in `RegisterRoutes()`
- [ ] Seed data includes notification permissions
- [ ] Test suite cleans up notification tables between tests

---

### Step 6: Integration Tests & Swagger

**Files to create:**
- `tests/integration/notification_handler_test.go`

#### 6A. Integration Tests

Follow pattern from `tests/integration/email_handler_test.go`:

```go
//go:build integration

package integration

// Test cases:

// TestNotificationHandler_List
//   - Create notifications for user, verify paginated list returns them
//   - Verify unread_only=true filter works
//   - Verify user cannot see other users' notifications

// TestNotificationHandler_CountUnread
//   - Create mix of read/unread, verify count matches unread only

// TestNotificationHandler_MarkAsRead
//   - Create notification, mark as read, verify read_at is set
//   - Verify user cannot mark another user's notification as read (returns 404)

// TestNotificationHandler_MarkAllAsRead
//   - Create multiple unread notifications, mark all as read
//   - Verify returned count matches

// TestNotificationHandler_Archive
//   - Create notification, archive it (DELETE), verify archived_at is set
//   - Verify archived notifications are hidden from list (archived_at filter)
//   - Verify user cannot archive another user's notification (returns 404)

// TestNotificationHandler_MarkAllAsReadWithTypeFilter
//   - Create multiple notifications of different types
//   - POST /read-all?type=system marks only system notifications as read
//   - Verify other types remain unread

// TestNotificationHandler_Preferences
//   - GET preferences returns empty array for new user
//   - PUT preference creates/updates preference
//   - Verify preference values are persisted correctly

// TestNotificationHandler_Unauthorized
//   - All endpoints return 401 without JWT token
```

Use helpers:
- `helpers.NewTestServer(t, suite)` for server setup
- `helpers.MakeRequest(t, server, method, path, body, token)` for HTTP calls
- `helpers.ParseEnvelope(t, rec)` for response parsing
- `helpers.CreateUserWithPermissions(t, suite, enforcer, perms)` for authenticated users

#### 6B. Swagger Documentation

All handler methods must include Swagger annotations (pattern from `internal/http/handler/news.go:43-54`):
- `@Summary`, `@Description`, `@Tags notifications`
- `@Security BearerAuth`
- `@Param` for path/query/body params
- `@Success` and `@Failure` with response types
- `@Router` with method

After implementation, regenerate swagger docs:
```bash
swag init -g cmd/api/main.go -o docs/swagger
```

**Acceptance Criteria:**
- [ ] At least 7 test functions covering all endpoints
- [ ] Tests use testcontainers (PostgreSQL + Redis) via `NewTestSuite(t)`
- [ ] Tests verify user isolation (user A cannot see/modify user B's notifications)
- [ ] Tests verify pagination (limit/offset parameters)
- [ ] Tests verify 401 for unauthenticated requests
- [ ] All handler methods have complete Swagger annotations
- [ ] `swag init` generates updated swagger docs without errors

---

## Risk Identification & Mitigations

| # | Risk | Impact | Likelihood | Mitigation |
|---|------|--------|------------|------------|
| 1 | **Email delivery blocks notification creation** | User doesn't see notification if email queue is down | Medium | Service catches email errors, logs them, but still persists the notification (fire-and-forget for email). Implemented in `Send()` method. |
| 2 | **Rate limiting cache miss/failure** | Misbehaving services bypass rate limit if Redis is down | Low-Medium | Rate limiting uses Redis cache with graceful degradation: log cache errors but allow notification creation to proceed (fail open, not closed). Implement retry logic for cache operations. |
| 3 | **Unbounded notification growth** | Table grows indefinitely | Low | Add `archived_at` partial index for efficient filtering. Document a future cleanup job (e.g., hard-delete archived notifications older than 90 days). Not blocking for v1. |
| 4 | **Notification type extensibility** | Open VARCHAR allows invalid types at runtime | Low | Document that new types can be added at runtime without migrations (decision 4). Add type validation/documentation in service layer. |
| 5 | **Race condition on MarkAllAsRead** | Concurrent requests could conflict | Very Low | PostgreSQL handles concurrent updates correctly. The `UPDATE WHERE read_at IS NULL` is atomic. No application-level locking needed. |
| 6 | **Circular dependency: services calling NotificationService** | Other services (invoice, news) need to send notifications | Medium | NotificationService is a leaf dependency -- it depends on repos and EmailService. Other services receive NotificationService as a dependency injected in `server.go`, keeping the dependency graph acyclic. |

---

## Testing Strategy

### Unit Tests (future, not in scope for this plan)
- `NotificationService.Send()` with mocked repos and email service
- `IsValidNotificationType()` validation
- `ToResponse()` conversion correctness

### Integration Tests (in scope)
- **Handler-level**: HTTP request -> response cycle via testcontainers
- **Coverage targets**: All 7 endpoints, error paths, user isolation, pagination
- **Pattern**: Follow `tests/integration/email_handler_test.go` structure

### Manual Verification
- Run `make migrate` and verify tables exist
- Run `make seed` and verify notification permissions are seeded
- Run `swag init` and verify swagger docs include notification endpoints
- Start server and test endpoints via Postman/curl

---

## Success Criteria

1. All 7 HTTP endpoints respond correctly with proper envelope format
2. Notifications are user-scoped (no cross-user data leakage)
3. Read/unread tracking works via `read_at` timestamp
4. Archiving works via `archived_at` timestamp (soft delete, not hard delete)
5. Archived notifications are hidden from list endpoints (filtered at DB query level)
6. Per-user preferences control email delivery routing with documented defaults
7. Email delivery failures do not block notification creation
8. Rate limiting enforces max 100 notifications/hour per user
9. MarkAllAsRead endpoint supports optional `type` filter for selective marking
10. Integration tests pass with `go test -tags=integration ./tests/integration/...`
11. Swagger docs regenerate cleanly
12. No breaking changes to existing services or routes
13. Migration is reversible (up + down)
14. Seed data includes notification permissions for admin/viewer roles

---

## File Inventory

### New Files (9)
| File | Purpose |
|------|---------|
| `migrations/000012_notifications.up.sql` | Create notifications + notification_preferences tables |
| `migrations/000012_notifications.down.sql` | Drop both tables |
| `internal/domain/notification.go` | Notification entity, type enum, response struct |
| `internal/domain/notification_preference.go` | NotificationPreference entity, response struct |
| `internal/repository/notification.go` | NotificationRepository interface + implementation |
| `internal/repository/notification_preference.go` | NotificationPreferenceRepository interface + implementation |
| `internal/service/notification.go` | NotificationService with routing logic |
| `internal/http/request/notification.go` | Request validation structs |
| `internal/http/handler/notification.go` | HTTP handlers with Swagger annotations |

### Modified Files (3)
| File | Change |
|------|--------|
| `internal/http/server.go` | Add `RegisterNotificationRoutes()`, wire DI in `RegisterRoutes()` |
| `cmd/api/main.go` | Add notification permissions to seed data |
| `tests/integration/testsuite.go` | Add notification migration SQL + cleanup tables |

### New Test Files (1)
| File | Purpose |
|------|---------|
| `tests/integration/notification_handler_test.go` | Integration tests for all notification endpoints |

**Total: 10 new files, 3 modified files**
