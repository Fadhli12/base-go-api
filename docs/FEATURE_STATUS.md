# Feature Implementation Status

**Generated:** 2026-05-08
**Last Updated:** 2026-05-13 — All 17 features complete
**Build Status:** `go build ./...` ✅ PASSES

---

## Overview

This document tracks which features from `FEATURE_RECOMMENDATIONS.md` have been implemented vs. what still needs to be built.

---

## Priority 1: Essential Business Features

### 1.1 Team/Organization Support

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/organization.go`, `internal/domain/organization_member.go` |
| Repository | `internal/repository/organization.go` (interface + GORM impl) |
| Service | `internal/service/organization.go` (579 lines, Casbin RBAC, audit logging) |
| Handler | `internal/http/handler/organization.go` (8 endpoints) |
| Middleware | `internal/http/middleware/organization.go` (X-Organization-ID extraction) |
| Request DTOs | `internal/http/request/organization.go` |
| Migrations | `migrations/000006_organizations.up.sql`, `migrations/000007_add_organization_to_resources.up.sql` |

**Endpoints:**
- `POST /api/v1/organizations` - Create
- `GET /api/v1/organizations` - List
- `GET /api/v1/organizations/:id` - GetByID
- `PUT /api/v1/organizations/:id` - Update
- `DELETE /api/v1/organizations/:id` - Delete
- `POST /api/v1/organizations/:id/members` - AddMember
- `GET /api/v1/organizations/:id/members` - GetMembers
- `DELETE /api/v1/organizations/:id/members/:user_id` - RemoveMember

---

### 1.2 API Key Authentication

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/api_key.go` |
| Repository (interface + impl) | `internal/repository/api_key.go`, `internal/repository/api_key_repo.go` |
| Service | `internal/service/api_key.go` |
| Middleware | `internal/http/middleware/api_key.go` |
| Handler | `internal/http/handler/api_key.go` |
| Request DTOs | `internal/http/request/api_key.go` |
| Migration | `migrations/000005_api_keys.up.sql` |
| Tests | `tests/integration/apikey_handler_test.go` |

**Key Features:**
- Key format: `ak_live_<32-hex>` or `ak_test_<32-hex>`
- bcrypt hashing (cost 12)
- Scopes: JSONB with wildcard support (`*`)
- Key methods: IsActive, IsRevoked, IsExpired, HasScope, Validate

**Endpoints:**
- `POST /api/v1/api-keys` - Create
- `GET /api/v1/api-keys` - List
- `GET /api/v1/api-keys/:id` - GetByID
- `DELETE /api/v1/api-keys/:id` - Revoke

---

### 1.3 Email Service

**Status:** ✅ FULLY IMPLEMENTED (SMTP + SendGrid + SES)

**What Exists:**
| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/email_template.go`, `internal/domain/email_queue.go`, `internal/domain/email_bounce.go` |
| Repository | `internal/repository/email_template.go`, `internal/repository/email_queue.go`, `internal/repository/email_bounce.go` |
| Service | `internal/service/email.go`, `internal/service/email_provider.go` |
| SMTP Provider | `internal/service/email_smtp_provider.go` ✅ |
| SendGrid Provider | `internal/service/email_sendgrid_provider.go` ✅ |
| SES Provider | `internal/service/email_ses_provider.go` ✅ |
| SendGrid Config | `internal/config/email_sendgrid.go` ✅ |
| SES Config | `internal/config/email_ses.go` ✅ |
| Queue (Redis) | `internal/service/email_queue_redis.go` |
| Worker | `internal/service/email_worker.go` |
| Template Engine | `internal/service/email_template_engine.go` |
| HTTP Handlers | `internal/http/handler/email.go`, `internal/http/handler/email_template.go` |
| Routes wired | `internal/http/server.go:575-645` ✅ |
| Migration | `migrations/000008_email_service.up.sql` ✅ |
| Config | `internal/config/email.go` (includes SendGrid + SES fields) ✅ |

**What's Implemented:**
- SMTP provider (baseline) - fully working
- SendGrid provider - HTTP-based with rate limit backoff ✅
- SES provider - AWS SDK v2 with credential chain ✅
- `EmailProvider` interface supports multiple providers
- Provider selection via `config.EmailConfig.Provider` (switch/case in server.go)
- Unit tests for both providers (using httptest mocks)

**Configuration:**
Set `EMAIL_PROVIDER=smtp|sendgrid|ses` with provider-specific credentials:
- SendGrid: `EMAIL_SENDGRID_API_KEY`
- SES: `EMAIL_SES_REGION`, `EMAIL_SES_ACCESS_KEY`, `EMAIL_SES_SECRET_KEY`, `EMAIL_SES_FROM_EMAIL`

---

### 1.4 Notification System

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/notification.go`, `internal/domain/notification_preference.go` |
| Repository | `internal/repository/notification.go`, `internal/repository/notification_preference.go` |
| Service | `internal/service/notification.go` |
| Handler | `internal/http/handler/notification.go` |
| Request DTOs | `internal/http/request/notification.go` |
| Migrations | `migrations/000012_notifications.up.sql` |
| Tests | `tests/integration/notification_handler_test.go` |

**Notification Types:** `mention`, `assignment`, `system`, `invoice.created`, `news.published`

**Endpoints:**
- `GET /api/v1/notifications` - List
- `GET /api/v1/notifications/count-unread` - CountUnread
- `PUT /api/v1/notifications/:id/read` - MarkAsRead
- `PUT /api/v1/notifications/read-all` - MarkAllAsRead
- `PUT /api/v1/notifications/:id/archive` - Archive
- `GET /api/v1/notifications/preferences` - GetPreferences
- `PUT /api/v1/notifications/preferences` - UpdatePreference

**Design Notes:**
- Notifications use `archived_at` flag (no soft delete) for permanent audit trail
- Preferences use soft delete
- Rate limiting: 100 notifications/hour per user
- Email queuing based on user preferences

---

### 1.5 Webhook System

**Status:** ✅ FULLY IMPLEMENTED (per AGENTS.md documentation)

**Key Components:**
- `internal/domain/webhook.go` - Webhook + WebhookDelivery entities
- `internal/domain/webhook_events.go` - EventBus with Go channels pub/sub
- `internal/repository/webhook.go` - Repository + `FindForEventDispatch(orgID)`
- `internal/service/webhook.go` - WebhookService CRUD + DispatchEvent + SubscribeToEventBus
- `internal/service/webhook_worker.go` - Background delivery processor
- `internal/service/webhook_queue.go` + `webhook_queue_redis.go` - Queue interface + Redis impl
- `internal/service/webhook_rate_limiter.go` - Sliding window rate limiter
- `internal/service/webhook_sign.go` - HMAC-SHA256 signature
- `internal/http/handler/webhook.go` - 8 endpoints

**Endpoints:**
- `POST /api/v1/webhooks` - Create
- `GET /api/v1/webhooks` - List
- `GET /api/v1/webhooks/:id` - Get
- `PUT /api/v1/webhooks/:id` - Update
- `DELETE /api/v1/webhooks/:id` - Delete
- `GET /api/v1/webhooks/:id/deliveries` - List deliveries
- `GET /api/v1/webhooks/deliveries/:id` - Get delivery
- `POST /api/v1/webhooks/deliveries/:id/replay` - Replay

**Migration:** `migrations/000010_webhooks.up.sql`

---

## Priority 2: Enhanced Features

### 2.1 Two-Factor Authentication (2FA)

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/two_factor.go`, `internal/domain/two_factor_recovery_code.go` |
| Service | `internal/service/two_factor.go`, `internal/service/two_factor_crypto.go`, `internal/service/two_factor_recovery.go` |
| Handler | `internal/http/handler/two_factor.go` |
| Repository | `internal/repository/two_factor_recovery_code.go` |
| Request DTOs | `internal/http/request/two_factor.go` |
| Migration | `migrations/000015_two_factor_auth.up.sql` |
| Tests | `tests/integration/two_factor_test.go`, `tests/unit/two_factor_service_test.go` |

---

### 2.3 Search & Filtering

**Status:** ✅ FULLY IMPLEMENTED (verified 2026-05-08)

**What Exists:**
| Component | Location | Status |
|-----------|----------|--------|
| Domain entity | `internal/domain/saved_search.go` | ✅ SavedSearch + Response DTO |
| Repository | `internal/repository/search.go` | ✅ SavedSearchRepository interface + GORM impl |
| Service | `internal/service/search.go` | ✅ SearchService (6 methods) |
| Handler | `internal/http/handler/search.go` | ✅ All audit calls fixed to 9-param signature |
| Request DTOs | `internal/http/request/search.go` | ✅ |
| Response DTOs | `internal/http/response/search.go` | ✅ |
| Tests | `tests/unit/search_service_test.go` | ✅ Unit tests |
| Migration (news FTS) | `migrations/000016_add_news_search.up.sql` | ✅ |
| Migration (saved searches) | `migrations/000017_saved_searches.up.sql` | ✅ |
| Route registration | `internal/http/server.go:678-708` | ✅ |

**What Was Fixed (2026-05-07):**
- `search.go:235` - CreateSavedSearch: LogAction struct → 9 params ✅
- `search.go:494` - DeleteSavedSearch: LogAction struct → 9 params ✅
- `search.go:431-436` - UpdateSavedSearch: Added audit logging ✅

**Endpoints (6 fully wired):**
- `GET /api/v1/search` - Full-text search with ranking/highlighting
- `POST /api/v1/saved-searches` - Create saved search
- `GET /api/v1/saved-searches` - List saved searches
- `GET /api/v1/saved-searches/:id` - Get saved search
- `PUT /api/v1/saved-searches/:id` - Update saved search
- `DELETE /api/v1/saved-searches/:id` - Soft delete saved search

---

### 2.4 Comment System

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/comment.go` (Comment struct, CommentResponse, CommentableTypes) |
| Repository (interface + impl) | `internal/repository/comment.go` (CommentRepository: Create, FindByID, FindByCommentable, FindReplies, Update, SoftDelete, CountByCommentable, CountReplies) |
| Service | `internal/service/comment.go` (CommentService: Create, GetByID, ListByCommentable, ListReplies, Update, Delete, Pin, Unpin, mention parsing) |
| Handler | `internal/http/handler/comment.go` (CommentHandler: Create, GetByID, ListByCommentable, ListReplies, Update, Delete, Pin, Unpin) |
| Request DTOs | `internal/http/request/comment.go` (CreateCommentRequest, UpdateCommentRequest with validation) |
| Response DTOs | `internal/http/response/comment.go` (CommentListResponse) |
| DI wiring | `internal/http/server.go:475-477, 510, 680-681, 1154-1191` |
| Permissions seed | `cmd/api/main.go:581-585` (5 permissions: view, create, delete, delete_any, manage) |
| Migration | `migrations/000022_create_comments.up.sql`, `.down.sql` |
| Unit tests | `tests/unit/comment_service_test.go` (54 test cases) |

**Key Features:**
- Polymorphic commentable types: `news`, `invoice`, `media` (validated via `IsValidCommentableType`)
- Threaded comments with `parent_id` for replies
- `@mention` support: `parseMentions` extracts mentioned user IDs from content
- Pin/Unpin comments for admin moderation
- Organization-scoped: all comments tied to `organization_id`
- Soft delete with `deleted_at`
- Partial unique indexes for consistency
- RBAC enforcement: 5 permission levels (view, create, delete own, delete any, manage)
- Audit logging on all mutations

**Endpoints:**
- `POST /api/v1/:type/:id/comments` - Create comment on commentable entity
- `GET /api/v1/:type/:id/comments` - List comments for commentable entity
- `GET /api/v1/comments/:id` - Get comment by ID
- `GET /api/v1/comments/:id/replies` - List replies to a comment
- `PUT /api/v1/comments/:id` - Update comment
- `DELETE /api/v1/comments/:id` - Soft-delete comment
- `POST /api/v1/comments/:id/pin` - Pin comment
- `POST /api/v1/comments/:id/unpin` - Unpin comment

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `comment:view` | comment | view | View comments |
| `comment:create` | comment | create | Create and edit own comments |
| `comment:delete` | comment | delete | Delete own comments |
| `comment:delete_any` | comment | delete_any | Delete any comment (admin) |
| `comment:manage` | comment | manage | Pin, unpin, and manage comments |

---

### 2.2 File Versioning

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/media_version.go` (MediaVersion entity, DTOs, business methods) |
| Extended media domain | `internal/domain/media.go` (CurrentVersion, Versions relation) |
| Repository | `internal/repository/media_version.go` (9-method interface + GORM impl: Create, FindByMediaID, FindByID, FindByMediaAndVersion, UpdateCurrentVersion, SoftDelete, FindLatestVersion, GetVersionCount, FindVersionByChecksum) |
| Service | `internal/service/media_version.go` (VersionService: UploadVersion, ListVersions, GetVersion, DownloadVersion, GetVersionSignedURL, RestoreVersion, DeleteVersion) |
| Checksum utility | `internal/service/media_version.go:ComputeSHA256Checksum` (SHA-256 deduplication) |
| Handler | `internal/http/handler/media_version.go` (7 endpoints, handler-level permission middleware) |
| DI wiring | `internal/http/server.go:467-479, 547-549` |
| Permissions seed | `cmd/api/main.go:513-517, 523` (5 permissions: upload, view, download, restore, delete) |
| Migration | `migrations/000019_create_media_versions.up.sql` (media_versions table + current_version column) |
| Unit tests | `tests/unit/media_version_service_test.go` |
| Integration tests | `tests/integration/media_version_test.go` (scaffold) |

**Endpoints (7 fully wired):**
- `POST /api/v1/media/:media_id/versions` - Upload new version (multipart form)
- `GET /api/v1/media/:media_id/versions` - List all versions
- `GET /api/v1/media/:media_id/versions/:version` - Get version metadata
- `GET /api/v1/media/:media_id/versions/:version/download` - Download version file
- `GET /api/v1/media/:media_id/versions/:version/url` - Get signed URL for version
- `POST /api/v1/media/:media_id/versions/:version/restore` - Restore version as current
- `DELETE /api/v1/media/:media_id/versions/:version` - Soft-delete version

**Key Design Decisions:**
- Retroactive v1 creation on first upload (no separate create endpoint)
- SHA-256 checksum deduplication (409 Conflict on identical content)
- Optimistic locking via `media.current_version` field
- Restore = pointer update only (no new version record)
- Delete = soft-delete record + physical file removal
- Current version cannot be deleted (protected)
- Handler-level permission checks (not service-level Enforce calls)
- No EventBus integration (synchronous only)
- Versioned storage path: `{base_path}/{media_id}/v{version}/{uuid_filename}.{ext}`

---

### 3.2 Background Job Queue

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/job.go` (Job, JobStatus enum, JobResponse) |
| Repository | `internal/repository/job.go` (JobRepository interface) |
| Service | `internal/service/job.go` (JobService: submit, get, list, cancel, resubmit) |
| Handler Service | `internal/service/job_handler.go` (JobHandler interface, NoopHandler, EchoHandler, EmailJobHandler) |
| Worker Pool | `internal/service/job_worker.go` (BZPOPMIN pattern, N pollers + M workers) |
| Job Reaper | `internal/service/job_reaper.go` (stuck job recovery) |
| Callback | `internal/service/job_callback.go` (HTTP callback delivery) |
| Handler | `internal/http/handler/job.go` (Submit, List, GetByID, Cancel, Resubmit) |
| Request DTO | `internal/http/handler/request/job.go` (SubmitJobRequest) |
| Config | `internal/config/job.go` (JobConfig with all env vars) |
| Unit Tests | `tests/unit/job_service_test.go`, `tests/unit/job_worker_test.go` |

**Endpoints:**
- `POST /api/v1/jobs` - Submit job
- `GET /api/v1/jobs` - List jobs
- `GET /api/v1/jobs/:id` - Get job by ID
- `DELETE /api/v1/jobs/:id` - Cancel job
- `POST /api/v1/jobs/:id/resubmit` - Resubmit dead job

**Architecture:**
- Redis sorted set for queue (`jobs:queue`)
- Redis hashes for job data (`jobs:data:{id}`)
- Redis sorted sets for user indexes (`jobs:user:{id}`)
- BZPOPMIN blocking pop with ticker fallback
- Worker pool (configurable N workers)
- Automatic retry with exponential backoff (1m → 5m → 30m cap)
- Stuck job reaper (runs every 60s, recovers jobs stuck > 5min)
- HTTP callbacks on job completion/failure

**Commit:** This feature on branch `009-background-job-queue`

---

### 3.4 Data Import/Export System

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/data_portability.go`, `internal/domain/data_portability_events.go`, `internal/domain/export_job.go`, `internal/domain/import_job.go`, `internal/domain/import_id_map.go` |
| Repository | `internal/repository/export_job.go`, `internal/repository/import_job.go`, `internal/repository/import_id_map.go` |
| Export service | `internal/service/export_service.go` (526 lines) |
| Import service | `internal/service/import_service.go` (715 lines) |
| Export worker | `internal/service/export_worker.go` (271 lines) |
| Import worker | `internal/service/import_worker.go` (237 lines) |
| Format: CSV | `internal/service/format_csv.go` |
| Format: JSON | `internal/service/format_json.go` |
| Rate limiter | `internal/service/data_portability_rate_limiter.go`, `internal/service/data_portability_rate_limiter_iface.go` |
| Validator | `internal/service/data_portability_validator.go` |
| Signing | `internal/service/data_portability_sign.go` (HMAC-SHA256) |
| Cleanup | `internal/service/export_cleanup.go` |
| Handler | `internal/http/handler/data_portability.go` (362 lines, 11 endpoints) |
| Middleware | `internal/http/middleware/data_portability.go` (rate limiting, file size limits) |
| Config | `internal/config/data_portability.go` |
| EventBus | `internal/domain/data_portability_events.go` (Go channels pub/sub) |
| DI wiring | `internal/http/server.go:525-547, 659+`, `cmd/api/main.go:231-271` |
| Permissions seed | `cmd/api/main.go:574-578` (5 permissions) |
| Migration | `migrations/000020_create_data_portability.up.sql` |
| Unit tests | 152 test functions across 10 test files |

**Endpoints (11 fully wired):**

Export:
- `POST /api/v1/exports` - Create export job (with rate limiting)
- `GET /api/v1/exports` - List export jobs
- `GET /api/v1/exports/:id` - Get export job
- `GET /api/v1/exports/:id/download` - Download export file
- `DELETE /api/v1/exports/:id` - Cancel export job

Import:
- `POST /api/v1/imports/preview` - Preview import (with rate limiting + file size limit)
- `POST /api/v1/imports` - Create import job (with rate limiting + file size limit)
- `GET /api/v1/imports` - List import jobs
- `GET /api/v1/imports/:id` - Get import job
- `POST /api/v1/imports/:id/cancel` - Cancel import job

**Architecture:**
- Export pipeline: Create job → Worker picks up → Generates file (CSV/JSON) → Available for download
- Import pipeline: Upload file → Preview (dry run) → Create job → Worker processes → ID mapping for cross-references
- EventBus integration: Services emit events via `SetEventBus()` after successful operations
- HMAC-SHA256 signed payloads for data integrity verification
- Rate limiting (sliding window) for both export and import endpoints
- Optimistic locking for job claim (`ClaimJob` with concurrent safety)
- Stuck job recovery via reaper goroutines
- Import ID mapping for cross-referencing between old and new entity IDs
- Entity registry pattern for extensible export/import of different resource types

**Permissions:**
| Permission | Resource | Action |
|-----------|----------|--------|
| `data_portability:export_create` | data_portability | export:create |
| `data_portability:export_download` | data_portability | export:download |
| `data_portability:import_create` | data_portability | import:create |
| `data_portability:import_view` | data_portability | import:view |
| `data_portability:import_cancel` | data_portability | import:cancel |

**Unit Tests (152 functions):**
- `export_service_test.go` (30 tests)
- `import_service_test.go` (30 tests)
- `format_csv_test.go` (30 tests)
- `format_json_test.go` (32 tests)
- `data_portability_rate_limiter_test.go` (12 tests)
- `data_portability_sign_test.go` (10 tests)
- `data_portability_validator_test.go` (5 tests)
- `data_portability_test.go` (11 tests)
- `export_job_test.go` (4 tests)
- `import_job_test.go` (8 tests)
- `import_id_map_test.go` (2 tests)

---

### 3.5 Settings & Configuration System

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/user_settings.go`, `internal/domain/system_settings.go` |
| Repository | `internal/repository/user_settings.go`, `internal/repository/system_settings.go` |
| Service | `internal/service/settings.go` (merge-before-upsert, audit logging, allowlist validation) |
| Handler | `internal/http/handler/settings.go` (5 endpoints, RBAC enforcement) |
| Request DTOs | `internal/http/request/settings.go` (key allowlist, type validation) |
| Middleware | `internal/http/middleware/organization.go` (X-Organization-ID context) |
| Migrations | `migrations/000013_user_settings.up.sql`, `migrations/000014_system_settings.up.sql` |
| Unit tests | `tests/unit/settings_service_test.go` (23 test cases) |
| Integration tests | `tests/integration/settings_test.go` (9 tests), `tests/integration/settings_handler_test.go` (7 tests) |

**Endpoints:**
- `GET /api/v1/settings/user` - Get user settings (requires `settings:view_user`)
- `PUT /api/v1/settings/user` - Update user settings (requires `settings:manage_user`)
- `GET /api/v1/settings/effective` - Get merged effective settings
- `GET /api/v1/settings/system` - Get system settings (requires `settings:view_system`)
- `PUT /api/v1/settings/system` - Update system settings (requires `settings:manage_system`)

**Key Design Decisions:**
- Shallow merge on update: existing keys preserved, new keys override (no data loss)
- RBAC enforcement via Casbin `Enforce(userID, orgDomain, "settings", action)`
- Handler-level permission checks (not service-level)
- Audit logging: `LogAction` called after updates with before/after state
- Allowlist validation: only known keys with type checking for system settings
- `GetEffectiveSettings` requires authentication only (no Enforce call — users can view own merged settings)
- `UpdateSystemSettings` filters updates to allowed fields only

**Permissions:**
| Permission | Resource | Action |
|-----------|----------|--------|
| `settings:view_user` | settings | view_user |
| `settings:manage_user` | settings | manage_user |
| `settings:view_system` | settings | view_system |
| `settings:manage_system` | settings | manage_system |

### 3.6 Feature Flags System

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/feature_flag.go` |
| Repository (interface + impl) | `internal/repository/feature_flag.go` |
| Service | `internal/service/feature_flag.go` (635 lines, CRUD + evaluation + RBAC + audit) |
| Handler | `internal/http/handler/feature_flag.go` (7 endpoints) |
| Request DTOs | `internal/http/request/feature_flag.go` |
| Response DTOs | `internal/http/response/feature_flag.go` |
| Migration | `migrations/000021_create_feature_flags.up.sql`, `.down.sql` |
| Tests | `tests/unit/feature_flag_service_test.go` (42 cases), `tests/integration/feature_flag_test.go` (6 cases) |

**Key Features:**
- FNV-1a hash evaluation for percentage-based rollout (`userID + key` modulo 100)
- JSONB conditions (`user_ids`, `org_ids`, `envs`) as post-rollout filter
- System flag protection (`is_system=true` flags cannot be deleted)
- RBAC: `feature_flag:view` for read, `feature_flag:manage` for CRUD
- Evaluate endpoints authenticated-only (no RBAC) — any logged-in user can check flags
- Audit logging for Create, Update, Delete operations
- Soft delete with partial unique index (`WHERE deleted_at IS NULL`)
- Key format validation: `^[a-z][a-z0-9_]*$`
- Rollout percentage: 0-100 with CHECK constraint

**Endpoints:**
- `POST /api/v1/feature-flags` - Create
- `GET /api/v1/feature-flags` - List (paginated)
- `GET /api/v1/feature-flags/:id` - GetByID
- `PUT /api/v1/feature-flags/:id` - Update
- `DELETE /api/v1/feature-flags/:id` - Delete (soft)
- `GET /api/v1/feature-flags/:key/evaluate` - Evaluate single flag
- `GET /api/v1/feature-flags/evaluate` - Evaluate all flags

**Note:** Organization-scoped flags deferred to future iteration. Current implementation provides global flags only.

---

## Priority 3: Advanced Features (Not Implemented)

### 3.1 Real-time Communication (WebSocket)

**Status:** ✅ FULLY IMPLEMENTED

**Key Components:**
- `internal/domain/websocket.go` — WsMessage, constants, room validation
- `internal/domain/websocket_events.go` — EventBus ↔ WebSocket bridge types
- `internal/service/websocket_hub.go` — Hub: rooms, clients, broadcast, Start/Stop
- `internal/service/websocket_client.go` — Client: read/write pumps, message handling
- `internal/service/websocket_presence.go` — PresenceService: Redis SET + TTL, reference counting
- `internal/service/websocket_redis.go` — RedisSubscriber: pub/sub bridge
- `internal/http/handler/websocket.go` — Upgrade handler + REST endpoints
- `internal/http/middleware/websocket_auth.go` — JWT extraction for WS handshake
- `internal/config/websocket.go` — WsConfig with env vars
- `migrations/000024_websocket.up.sql` — Reserved (all state in Redis)

**Room Format**: `org:{orgID}` (org-wide) or `org:{orgID}:{entityType}:{entityID}` (entity-specific)
**Auth**: JWT token in query param `/ws?token=<jwt>`
**Scaling**: Redis pub/sub channels `ws:msg:{roomID}` + `ws:presence:{orgID}`
**Close Codes**: 4001 (auth expired), 4002 (server shutdown), 4003 (policy violation), 4004 (room limit)
**Library**: `github.com/coder/websocket`

**Endpoints:**
- `GET /ws` — WebSocket upgrade (JWT auth, org-scoped rooms)
- `GET /api/v1/ws/rooms` — List available rooms
- `GET /api/v1/ws/presence/:orgID` — Get online users for org
- `GET /api/v1/ws/rooms/:roomID/users` — Get users in room

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `websocket:connect` | websocket | connect | Connect to WebSocket endpoints |
| `websocket:view_presence` | websocket | view_presence | View online presence for organizations |
| `websocket:view_rooms` | websocket | view_rooms | View available WebSocket rooms |

---

### 3.3 Analytics Dashboard

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/metric_event.go`, `internal/domain/dashboard_metric.go`, `internal/domain/dashboard_preference.go`, `internal/domain/analytics_events.go` |
| Repository | `internal/repository/metric_event.go`, `internal/repository/dashboard_metric.go`, `internal/repository/dashboard_preference.go` |
| Service | `internal/service/analytics.go`, `internal/service/aggregation_worker.go`, `internal/service/analytics_reaper.go` |
| Handler | `internal/http/handler/analytics.go` (5 endpoints) |
| Request DTOs | `internal/http/request/analytics.go` |
| Config | `internal/config/analytics.go` (AnalyticsConfig with defaults) |
| Migrations | `migrations/000025_create_analytics.up.sql`, `.down.sql` |
| Unit tests | `tests/unit/analytics_domain_test.go`, `tests/unit/analytics_service_test.go`, `tests/unit/analytics_reaper_worker_test.go` |

**Endpoints:**
- `GET /api/v1/analytics/dashboard` — Dashboard data (4 metric categories)
- `GET /api/v1/analytics/metrics` — Time-series metrics with zero-fill
- `GET /api/v1/analytics/dashboard/preferences` — Dashboard preferences
- `PUT /api/v1/analytics/dashboard/preferences` — Update preferences (analytics:manage)
- `POST /api/v1/analytics/aggregate` — Trigger aggregation (202 Accepted, analytics:manage)

**Key Features:**
- Event-driven ingestion: 11 EventBus events → MetricEvent via AnalyticsMapping registry
- Idempotent ingestion: UNIQUE constraint `(event_type, resource_id, date, hour)` with ON CONFLICT DO NOTHING
- Pre-computed dashboard metrics via AggregationWorker (configurable interval, watermark resume)
- Time-series metrics with zero-filled intervals (hourly/daily/weekly/monthly)
- Organization-scoped dashboard preferences (default: all 4 categories visible)
- 90-day archival via AnalyticsReaper background goroutine
- Organization-scoped multi-tenant data isolation
- Buffered channel (256) EventBus subscription pattern (matches ActivityService)

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `analytics:view` | analytics | view | View dashboard, metrics, preferences |
| `analytics:manage` | analytics | manage | Update preferences, trigger aggregation |

**Architecture:**
```
Domain Services → EventBus.Publish(event) → AnalyticsService.SubscribeToEventBus()
                                                    → handleEvent() → GetAnalyticsMapping → Create MetricEvent
AggregationWorker.Start() → ticker → aggregatePeriod() → per-org metrics → Upsert DashboardMetric
AnalyticsReaper.Start() → ticker → ArchiveOlderThan(retentionDays)
```

---

## Priority 2: Enhanced Features (Implemented)

### 2.5 Tagging System

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/tag.go` (Tag, EntityTag structs, GenerateSlug, IsValidTaggableType) |
| Repository (interface + impl) | `internal/repository/tag.go` (TagRepository + EntityTagRepository: 9+ methods) |
| Service | `internal/service/tag.go` (TagService: CRUD, bulk attach/detach, autocomplete, RBAC, audit) |
| Handler | `internal/http/handler/tag.go` (11 endpoints) |
| Request DTOs | `internal/http/request/tag.go` (hex color regex validation `^#[0-9A-Fa-f]{6}$`) |
| Response DTOs | `internal/http/response/tag.go` |
| DI wiring | `internal/http/server.go` (RegisterTagRoutes: autocomplete/slug before /:id) |
| Permissions seed | `cmd/api/main.go` (5 permissions: view, create, update, delete, manage) |
| Migration | `migrations/000024_create_tags.up.sql`, `.down.sql` |
| Unit tests | `tests/unit/tag_service_test.go` (27 test cases including cross-org security) |
| Spec | `specs/015-tagging-system/spec.md`, `plan.md`, `tasks.md` |

**Key Features:**
- Polymorphic entity tagging: `news`, `invoice`, `media` (validated via `IsValidTaggableType`)
- Bulk attach/detach with partial success semantics (attached/skipped/errors per tag)
- Autocomplete with ILIKE search and popularity fallback
- Organization-scoped: all CRUD and attach/detach verify tag belongs to requester's org
- Cross-org security: service verifies `tag.OrganizationID == orgID` after FindByID
- Slug auto-generation with conflict resolution and empty-string fallback to UUID
- Hex color validation: `^#[0-9A-Fa-f]{6}$` regex in request DTOs
- Max limit enforcement (100) for autocomplete and list endpoints
- RBAC enforcement: 5 permission levels (view, create, update, delete, manage)
- Audit logging on all mutations
- Soft delete with partial unique indexes (`WHERE deleted_at IS NULL`)

**Endpoints (11 fully wired):**
- `POST /api/v1/tags` - Create tag
- `GET /api/v1/tags` - List tags (paginated, max limit 100)
- `GET /api/v1/tags/autocomplete` - Autocomplete search (must register before `/:id`)
- `GET /api/v1/tags/slug/:slug` - Get tag by slug (must register before `/:id`)
- `GET /api/v1/tags/:id` - Get tag by ID
- `PUT /api/v1/tags/:id` - Update tag
- `DELETE /api/v1/tags/:id` - Soft-delete tag
- `POST /api/v1/tags/:id/attach` - Bulk attach tags to entity
- `POST /api/v1/tags/:id/detach` - Bulk detach tags from entity
- `GET /api/v1/:type/:id/tags` - List tags for entity
- `GET /api/v1/tags/:id/entities` - List entities tagged with tag

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `tag:view` | tag | view | View tags and tag listings |
| `tag:create` | tag | create | Create new tags |
| `tag:update` | tag | update | Update tag properties |
| `tag:delete` | tag | delete | Soft-delete tags |
| `tag:manage` | tag | manage | Attach/detach tags to entities |

---

### 2.6 Activity Feed / Timeline

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/activity.go`, `internal/domain/activity_events.go` |
| Repository | `internal/repository/activity.go` (3 interfaces + GORM impl) |
| Service | `internal/service/activity.go`, `internal/service/activity_follow.go`, `internal/service/activity_reaper.go` |
| Handler | `internal/http/handler/activity.go` (8 endpoints) |
| Request DTOs | `internal/http/request/activity.go` |
| Response DTOs | `internal/http/response/activity.go` |
| Config | `internal/config/activity.go` |
| Migrations | `migrations/000023_create_activities.up.sql`, `migrations/000023_create_activities.down.sql` |

**Endpoints:**
- `GET /api/v1/activities` — List activities (paginated, filtered, org-scoped)
- `GET /api/v1/activities/count-unread` — Count unread activities
- `PUT /api/v1/activities/read-all` — Mark all as read
- `PUT /api/v1/activities/:id/read` — Mark single activity as read
- `POST /api/v1/activities/follow` — Follow a resource
- `DELETE /api/v1/activities/follow/:id` — Unfollow a resource
- `GET /api/v1/activities/follows` — List followed resources
- `DELETE /api/v1/activities/:id` — Soft-delete activity (admin, activity:manage)

**Key Features:**
- Per-user read tracking via `activity_reads` table
- Per-user resource following via `activity_follows` table
- Batch `is_read`/`is_following` computation (no LEFT JOIN)
- 90-day auto-archival via ActivityReaper background goroutine
- EventBus integration for automatic activity creation from domain events
- `archived_at` for archival (separate from `deleted_at` for admin DELETE)
- Organization-scoped multi-tenant feed

---

## Recommended Next Steps

Based on the analysis, the following should be prioritized:

1. **Analytics Dashboard** (4-5 days, P3, no deps) — largest remaining effort

---

## Summary: Implemented vs Not Implemented

### ✅ Implemented (16 features verified complete)

| Feature | Priority | Status |
|---------|----------|--------|
| Organization/Team | P1 | ✅ Complete |
| API Key Auth | P1 | ✅ Complete |
| Email Service (SMTP + SendGrid + SES) | P1 | ✅ Complete |
| Notification System | P1 | ✅ Complete |
| Webhook System | P1 | ✅ Complete |
| 2FA | P2 | ✅ Complete |
| Search & Filtering | P2 | ✅ Complete |
| File Versioning | P2 | ✅ Complete |
| Comment System | P2 | ✅ Complete |
| Tagging System | P2 | ✅ Complete |
| Background Job Queue | P3 | ✅ Complete |
| Data Import/Export System | P3 | ✅ Complete |
| Settings & Configuration | P3 | ✅ Complete |
| Feature Flags | P3 | ✅ Complete |
| Activity Feed / Timeline | P2 | ✅ Complete |
| Analytics Dashboard | P3 | ✅ Complete |

### ❌ Not Implemented (1 feature remaining)

These features from `FEATURE_RECOMMENDATIONS.md` have been verified as NOT present in the codebase:

| Feature | Priority | Complexity | Est. Effort | Dependencies |
|---------|----------|------------|-------------|--------------|
*(All features now implemented)*

---

## Files Referenced

- Source: `docs/FEATURE_RECOMMENDATIONS.md`
- This doc: `docs/FEATURE_STATUS.md`