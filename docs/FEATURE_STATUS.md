# Feature Implementation Status

**Generated:** 2026-05-08
**Last Updated:** 2026-05-18 — V1 complete (17/17), V2: SSRF + idempotency complete
**Build Status:** `go build ./...` ✅ PASSES

---

## Overview

This document tracks which features from `FEATURE_RECOMMENDATIONS.md` (V1) and `FEATURE_RECOMMENDATIONS_V2.md` (V2) have been implemented vs. what still needs to be built.

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

## Priority 4: Security & Compliance (V2 — Not Implemented)

### 4.1 Idempotency Keys

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Medium | **Effort:** 2 days | **Dependencies:** Redis (existing)

**Description:**
Prevents duplicate operations from retry mechanisms, network failures, and double-clicks. Every payment API (Stripe, PayPal) requires this. RFC 9110 standard.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/idempotency_record.go` |
| Middleware | `internal/http/middleware/idempotency.go` |
| Store (Redis) | `internal/service/idempotency_store_redis.go` |
| Config | `internal/config/idempotency.go` |
| Reaper | `internal/service/idempotency_reaper.go` |

**Key Design:**
- Redis-backed key store with configurable TTL (default 24h, max 72h)
- `Idempotency-Key` header (RFC 9110)
- SHA-256 request body hash guards against key reuse with different payloads
- Return `409 Conflict` if key exists with different request hash
- Automatic cleanup via background reaper
- No RBAC needed — pure middleware

**Endpoints:** None (pure middleware, applies to all POST/PUT/PATCH endpoints automatically)

---

### 4.2 SSRF Protection

**Status:** ✅ FULLY IMPLEMENTED

**Complexity:** Low | **Effort:** 1-2 days | **Dependencies:** None

**Description:**
Webhook worker makes outbound HTTP requests. Without URL validation, attackers can force the server to access internal services (Redis, database, cloud metadata endpoints). OWASP API Security Top 10 #6.

**What Exists:**
| Component | Location |
|-----------|----------|
| SSRF package | `internal/ssrf/` (6 files: validator, transport, config, + 3 test files) |
| Validator | `internal/ssrf/validator.go` (187 lines — URL validation, IP blocking, host blocking) |
| Transport | `internal/ssrf/transport.go` (115 lines — SSRF-safe HTTP client with dial-time IP validation) |
| Config | `internal/ssrf/config.go` (158 lines — SSRFConfig with defaults, CIDR parsing) |
| Config wiring | `internal/config/ssrf.go` (134 lines — env var → internal config mapping) |
| Config integration | `internal/config/config.go` (SSRFConfig field) |
| Webhook service | `internal/service/webhook.go` (SSRF validator for URL validation at webhook creation) |
| Webhook worker | `internal/service/webhook_worker.go` (SSRF-safe HTTP client for outbound delivery) |
| SendGrid provider | `internal/service/email_sendgrid_provider.go` (SSRF-safe HTTP client with 15s timeout) |
| Startup wiring | `cmd/api/main.go` (SSRF config wired into webhook worker + SendGrid provider) |
| Unit tests | `internal/ssrf/validator_test.go` (531 lines), `internal/ssrf/transport_test.go` (422 lines), `internal/ssrf/config_test.go` (309 lines) |
| Integration tests | `tests/unit/webhook_worker_test.go` (SSRF client test updates), `tests/unit/email_sendgrid_provider_test.go` (SSRF client test updates) |
| Spec | `specs/020-ssrf-protection/spec.md`, `plan.md`, `tasks.md` |

**Key Features:**
- **Two-layer defense**: URL validation at registration time + dial-time IP validation in custom HTTP Transport (DNS rebinding prevention)
- **Blocked CIDRs**: RFC1918 private ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16), loopback (127.0.0.0/8, ::1/128), link-local (169.254.0.0/16, fe80::/10), cloud metadata (169.254.169.254/32), CGNAT (100.64.0.0/10), unspecified (0.0.0.0/8, ::/128)
- **Blocked hosts**: metadata.internal, metadata.google.internal, instance-data.ec2.internal (with subdomain matching)
- **DNS rebinding prevention**: `ssrfDialContext` resolves hostname, validates ALL resolved IPs before connecting
- **Redirect protection**: CheckRedirect policy limits to 10 redirects with logging on each
- **IPv6-mapped IPv4 normalization**: `::ffff:127.0.0.1` → `127.0.0.1` before checking against blocked ranges
- **AllowedCIDRs override**: Whitelisting specific IPs within otherwise-blocked ranges (checked first, overrides ALL blocking)
- **Development mode**: `AllowPrivateIPs=true` with logged warning for local testing
- **Fail-closed**: DNS failure = blocked request (no fallback to connecting)
- **Nil config fallback**: Uses `DefaultSSRFConfig()` with warning (never creates unprotected client)

**Integration Points:**
- `WebhookService.Create()` / `WebhookService.Update()` — validates callback URLs at registration time
- `WebhookWorker` — uses SSRF-safe HTTP client for all outbound webhook deliveries
- `SendGridProvider` — uses SSRF-safe HTTP client for outbound email delivery
- `cmd/api/main.go` — `ssrfCfg := cfg.SSRF.ToInternal()` wired into service constructors

**Configuration (Environment Variables):**
| Variable | Default | Description |
|----------|---------|-------------|
| `SSRF_ALLOW_PRIVATE_IPS` | false | Development mode: allow private/internal IPs |
| `SSRF_ALLOWED_SCHEMES` | https | Comma-separated URL schemes allowed |
| `SSRF_BLOCKED_HOSTS` | metadata.internal,... | Comma-separated blocked hostnames |
| `SSRF_BLOCKED_CIDRS` | 127.0.0.0/8,... | Comma-separated blocked CIDR ranges |
| `SSRF_ALLOWED_CIDRS` | (empty) | Comma-separated CIDR overrides (whitelist) |
| `SSRF_TIMEOUT` | 30s | HTTP client timeout |

**Security Review (Oracle, all addressed):**
- CRIT-001: WebhookWorker uses DefaultSSRFConfig() fallback instead of nil/unprotected client ✅
- CRIT-002: AllowedCIDRs override ALL blocking including Go built-in checks (checked first) ✅
- MED-001: IsBlockedHost matches subdomains (e.g., sub.metadata.internal) ✅
- MED-002: CheckRedirect policy limits to 10 redirects with logging ✅
- CORR-001: SendGrid provider preserves 15s timeout with SSRF protection ✅
- CORR-002: Eliminated double client creation in WebhookWorker ✅

**Commit:** `5cccb27` — feat(ssrf): add centralized SSRF protection package with security hardening

**Endpoints:** None (internal security layer, applied in webhook worker and HTTP clients)

---

### 4.3 OAuth 2.0 Social Login

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Medium-High | **Effort:** 3-4 days | **Dependencies:** Existing JWT auth system

**Description:**
"Sign in with Google/GitHub/Microsoft" — expected by every SaaS user in 2026. Also enables enterprise SSO via SAML/OIDC bridges.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/oauth_provider.go`, `internal/domain/oauth_account.go` |
| Repository | `internal/repository/oauth_provider.go`, `internal/repository/oauth_account.go` |
| Service | `internal/service/oauth.go` |
| Handler | `internal/http/handler/oauth.go` |
| Request DTOs | `internal/http/request/oauth.go` |
| Config | `internal/config/oauth.go` |
| Migrations | `migrations/000027_oauth_providers.up.sql`, `000028_oauth_accounts.up.sql` |

**Endpoints:**
- `GET /api/v1/auth/oauth/:provider` — Redirect to OAuth provider
- `GET /api/v1/auth/oauth/:provider/callback` — OAuth callback (exchanges code)
- `POST /api/v1/auth/oauth/:provider/link` — Link provider to existing account
- `DELETE /api/v1/auth/oauth/:provider/unlink` — Unlink provider from account
- `GET /api/v1/auth/oauth/providers` — List available OAuth providers
- `GET /api/v1/auth/oauth/accounts` — List current user's linked accounts

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `oauth:manage` | oauth | manage | Configure OAuth providers (admin) |
| `oauth:link` | oauth | link | Link/unlink own social accounts |

**EventBus Events:**
- `auth.oauth.linked` — when user links a social account
- `auth.oauth.unlinked` — when user unlinks a social account

---

## Priority 5: SaaS Essentials (V2 — Not Implemented)

### 5.1 Session Management

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Low-Medium | **Effort:** 2 days | **Dependencies:** Existing refresh token system

**Description:**
Users need "log out of all devices," view active sessions, and revoke compromised tokens. Security best practice that complements existing JWT auth.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/session.go` |
| Repository | `internal/repository/session.go` |
| Service | `internal/service/session.go` |
| Handler | `internal/http/handler/session.go` |
| Request DTOs | `internal/http/request/session.go` |
| Config | `internal/config/session.go` |
| Migration | `migrations/000027_sessions.up.sql` |
| Reaper | `internal/service/session_reaper.go` |

**Endpoints:**
- `GET /api/v1/sessions` — List current user's sessions
- `DELETE /api/v1/sessions/:id` — Revoke specific session
- `DELETE /api/v1/sessions` — Revoke all sessions ("log out everywhere")

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `session:view` | session | view | View own sessions |
| `session:manage` | session | manage | Revoke sessions (self or others, admin) |
| `session:revoke_all` | session | revoke_all | Revoke all sessions for any user (admin) |

**EventBus Events:**
- `auth.session.created` — new session created
- `auth.session.revoked` — session revoked

---

### 5.2 Rate Limiting Tiers (Per-Plan)

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Low | **Effort:** 1-2 days | **Dependencies:** Existing rate limiter + organization system

**Description:**
Current rate limiting is flat. SaaS needs different limits per plan (free: 100/hr, pro: 1000/hr, enterprise: unlimited). Enables monetization.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/rate_limit_plan.go` |
| Repository | `internal/repository/rate_limit_plan.go` |
| Service | `internal/service/rate_limit_plan.go` |
| Handler | `internal/http/handler/rate_limit_plan.go` |
| Request DTOs | `internal/http/request/rate_limit_plan.go` |
| Response DTOs | `internal/http/response/rate_limit_plan.go` |
| Config | `internal/config/rate_limit_plan.go` |
| Migration | `migrations/000028_rate_limit_plans.up.sql` |
| Middleware update | `internal/http/middleware/rate_limit.go` (extend for per-plan) |

**Endpoints:**
- `POST /api/v1/rate-limit-plans` — Create plan (admin)
- `GET /api/v1/rate-limit-plans` — List plans
- `GET /api/v1/rate-limit-plans/:id` — Get plan
- `PUT /api/v1/rate-limit-plans/:id` — Update plan (admin)
- `DELETE /api/v1/rate-limit-plans/:id` — Delete plan (admin, non-system only)
- `GET /api/v1/rate-limit/usage` — Current user's rate limit usage

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `rate_limit_plan:view` | rate_limit_plan | view | View rate limit plans |
| `rate_limit_plan:manage` | rate_limit_plan | manage | Create, update, delete plans |

---

### 5.3 API Versioning

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Low | **Effort:** 1 day | **Dependencies:** None (Echo router groups)

**Description:**
With 17+ features and growing, breaking changes are inevitable. API versioning provides a clean deprecation path.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Middleware | `internal/http/middleware/api_version.go` |
| Config | `internal/config/api_version.go` |

**Key Design:**
- URL versioning only: `/api/v1/`, `/api/v2/`
- Deprecation headers per RFC 8594 (`Deprecation`, `Sunset`, `Link`)
- Echo route group per version
- No database migration needed

**Endpoints:** None (infrastructure middleware, no RBAC)

---

### 5.4 GDPR Data Export (Right of Portability)

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Low | **Effort:** 1 day | **Dependencies:** Existing background job queue + export service

**Description:**
GDPR Article 20 requires users to request "all my data." Different from the admin-facing Data Import/Export — this is a user-facing self-service export.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/data_export_request.go` |
| Repository | `internal/repository/data_export_request.go` |
| Service | `internal/service/data_export_user.go` |
| Handler | `internal/http/handler/data_export_user.go` |
| Request DTOs | `internal/http/request/data_export_user.go` |
| Config | `internal/config/data_export.go` (extend existing) |
| Migration | `migrations/000029_data_export_requests.up.sql` |

**Endpoints:**
- `POST /api/v1/me/data-export` — Request data export (authenticated user)
- `GET /api/v1/me/data-export` — Get export status
- `GET /api/v1/me/data-export/:id/download` — Download export file

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `data_export:request` | data_export | request | Request own data export |
| `data_export:download` | data_export | download | Download own export file |
| `data_export:manage` | data_export | manage | Manage all exports (admin) |

---

## Priority 6: Production Hardening (V2 — Not Implemented)

### 6.1 Circuit Breaker

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Medium | **Effort:** 2 days | **Dependencies:** None (wraps existing HTTP clients)

**Description:**
When external services (SendGrid, S3, SES) go down, circuit breaker prevents cascading failures. Opens circuit after N failures, half-opens to test recovery, closes when healthy.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Service | `internal/service/circuit_breaker.go` |
| Config | `internal/config/circuit_breaker.go` |
| Handler | `internal/http/handler/circuit_breaker.go` |

**Key Design:**
- Redis-backed state for multi-instance coordination
- Wraps existing HTTP clients in email/storage providers
- State transitions: Closed → Open → HalfOpen → Closed
- Metrics exposed via existing `/healthz` endpoint

**Endpoints:**
- `GET /api/v1/circuit-breakers` — List all circuit breaker states
- `GET /api/v1/circuit-breakers/:name` — Get specific circuit breaker state
- `POST /api/v1/circuit-breakers/:name/reset` — Reset circuit breaker (admin)

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `circuit_breaker:view` | circuit_breaker | view | View circuit breaker states |
| `circuit_breaker:manage` | circuit_breaker | manage | Reset circuit breakers (admin) |

---

### 6.2 OpenTelemetry Integration

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Medium | **Effort:** 2-3 days | **Dependencies:** None (complements existing structured logging)

**Description:**
Production debugging requires distributed tracing. Already have structured logging + request IDs — OpenTelemetry extends this to traces + metrics across services.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Middleware | `internal/http/middleware/otel.go` |
| Config | `internal/config/otel.go` |

**Key Design:**
- Integration with existing `internal/http/middleware/structured_logging.go`
- Exports to OTLP (vendor-neutral), Jaeger (traces), or Prometheus (metrics)
- Minimal overhead when disabled (`otel_enabled=false`)
- Replaces `X-Trace-ID` header with proper span context
- No database migration needed

**Endpoints:** None (infrastructure-level, admin dashboard only)

---

### 6.3 Enhanced Health Checks

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Low | **Effort:** 0.5 days | **Dependencies:** None

**Description:**
Current `/healthz` and `/readyz` are basic. Kubernetes needs component-level health with detailed status for each dependency.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Handler update | `internal/http/handler/health.go` (extend) |
| Config | `internal/config/health.go` |

**Key Design:**
- Component checks: PostgreSQL latency, Redis latency, storage driver, email provider, worker status
- `degraded` = latency above threshold but still functional
- `unhealthy` = connection failure or critical component down
- Existing `/healthz` and `/readyz` unchanged (Kubernetes compatibility)

**Endpoints:**
- `GET /healthz` — Liveness (always 200, unchanged)
- `GET /readyz` — Readiness (DB + Redis check, unchanged)
- `GET /api/v1/health/detailed` — Detailed component health (admin)

**Permissions:** Basic health = no auth; detailed health = `health:detailed` or admin

---

## Priority 7: Advanced Features (V2 — Not Implemented)

### 7.1 Inbound Webhook Processing

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Medium-High | **Effort:** 3 days | **Dependencies:** Existing webhook system + job queue

**Description:**
Current webhooks are outbound only. Inbound handles incoming events from Stripe, GitHub, Slack, etc. Need signature verification, routing, and processing.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entities | `internal/domain/inbound_webhook.go` |
| Repository | `internal/repository/inbound_webhook.go` |
| Service | `internal/service/inbound_webhook.go` |
| Handler | `internal/http/handler/inbound_webhook.go` |
| Request DTOs | `internal/http/request/inbound_webhook.go` |
| Config | `internal/config/inbound_webhook.go` |
| Migration | `migrations/000030_inbound_webhooks.up.sql` |

**Endpoints:**
- `POST /webhooks/inbound/:path` — Receive inbound webhook (no auth, signature verified)
- `GET /api/v1/inbound-webhooks` — List inbound webhook configs
- `POST /api/v1/inbound-webhooks` — Create inbound webhook config
- `GET /api/v1/inbound-webhooks/:id` — Get config
- `PUT /api/v1/inbound-webhooks/:id` — Update config
- `DELETE /api/v1/inbound-webhooks/:id` — Delete config
- `GET /api/v1/inbound-webhooks/:id/deliveries` — List deliveries
- `POST /api/v1/inbound-webhooks/deliveries/:id/replay` — Replay delivery

**EventBus Events:**
- `inbound_webhook.received.<provider>` — when a webhook is received
- `inbound_webhook.processed.<provider>` — after successful processing

---

### 7.2 Scheduled Tasks / Cron

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Medium | **Effort:** 2 days | **Dependencies:** Existing job queue

**Description:**
Job queue is fire-and-forget. Cron adds "run this every night at 2am" (analytics aggregation, cleanup, reports). Natural complement to existing job system.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/scheduled_task.go` |
| Repository | `internal/repository/scheduled_task.go` |
| Service | `internal/service/scheduler.go` |
| Handler | `internal/http/handler/scheduler.go` |
| Request DTOs | `internal/http/request/scheduler.go` |
| Config | `internal/config/scheduler.go` |
| Migration | `migrations/000031_scheduled_tasks.up.sql` |

**Endpoints:**
- `POST /api/v1/scheduled-tasks` — Create scheduled task
- `GET /api/v1/scheduled-tasks` — List scheduled tasks
- `GET /api/v1/scheduled-tasks/:id` — Get task
- `PUT /api/v1/scheduled-tasks/:id` — Update task
- `DELETE /api/v1/scheduled-tasks/:id` — Delete task
- `POST /api/v1/scheduled-tasks/:id/trigger` — Manual trigger

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `scheduler:view` | scheduler | view | View scheduled tasks |
| `scheduler:manage` | scheduler | manage | Create, update, delete, enable/disable tasks |
| `scheduler:trigger` | scheduler | trigger | Manually trigger a scheduled task |

---

### 7.3 Bulk Operations API

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Medium | **Effort:** 2 days | **Dependencies:** Existing job queue

**Description:**
SaaS APIs need batch create/update/delete (create 100 invoices at once, bulk delete media). Uses existing job queue for async processing.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Domain entity | `internal/domain/bulk_operation.go` |
| Repository | `internal/repository/bulk_operation.go` |
| Service | `internal/service/bulk_operation.go` |
| Handler | `internal/http/handler/bulk_operation.go` |
| Request DTOs | `internal/http/request/bulk_operation.go` |
| Response DTOs | `internal/http/response/bulk_operation.go` |
| Config | `internal/config/bulk.go` |

**Endpoints:**
- `POST /api/v1/bulk/:resource_type` — Create bulk operation
- `GET /api/v1/bulk` — List user's bulk operations
- `GET /api/v1/bulk/:id` — Get operation status
- `POST /api/v1/bulk/:id/cancel` — Cancel operation

**Key Design:**
- Follows existing job queue pattern for async processing
- Entity registry pattern (extensible): register handlers per resource type
- Partial success semantics: individual item failures don't stop the batch
- Max batch size 100 (configurable)

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `bulk:create` | bulk | create | Create bulk operations |
| `bulk:view` | bulk | view | View bulk operation status |
| `bulk:cancel` | bulk | cancel | Cancel bulk operations |
| `bulk:manage` | bulk | manage | Manage all bulk operations (admin) |

---

### 7.4 Soft Delete Recovery

**Status:** ❌ NOT IMPLEMENTED

**Complexity:** Low | **Effort:** 1 day | **Dependencies:** Soft delete pattern (all entities)

**Description:**
Everything uses soft delete but there's no "undo" endpoint. Admins need to recover accidentally deleted resources.

**Planned Components:**
| Component | Location |
|-----------|----------|
| Service | `internal/service/recovery.go` |
| Handler | `internal/http/handler/recovery.go` |
| Request DTOs | `internal/http/request/recovery.go` |

**Key Design:**
- No database migration needed — clears `deleted_at` on existing entities
- Registry pattern: map resource type strings to repository interfaces
- Supports all entities with `deleted_at`: users, organizations, webhooks, comments, tags, etc.
- Audit logging on recovery (before/after state)
- 30-day recovery window recommended (configurable)

**Endpoints:**
- `GET /api/v1/recovery/:resource_type` — List soft-deleted resources
- `POST /api/v1/recovery/:resource_type/:id/restore` — Restore soft-deleted resource

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `recovery:view` | recovery | view | View deleted resources |
| `recovery:manage` | recovery | manage | Recover deleted resources (admin) |

---

## Recommended Next Steps

Based on `FEATURE_RECOMMENDATIONS_V2.md`, the following implementation order is recommended:

### Sprint 1 (Week 1): Security Foundation
1. ~~**SSRF Protection**~~ — ✅ COMPLETE (commit `5cccb27`)
2. **Idempotency Keys** — prevents double-operations across all POST/PUT/PATCH endpoints

### Sprint 2 (Week 2): Auth Completion
3. **OAuth Social Login** — expected by every modern SaaS user
4. **Session Management** — "log out everywhere" security feature

### Sprint 3 (Week 3): SaaS Readiness
5. **Rate Limiting Tiers** — enables pricing differentiation
6. **API Versioning** — future-proofs all 17+ endpoints
7. **GDPR Data Export** — compliance requirement for EU users

### Sprint 4+ (As Needed)
8. Circuit Breaker → OpenTelemetry → Enhanced Health → Inbound Webhooks → Scheduled Tasks → Bulk Operations → Soft Delete Recovery

---

---

### P2: Idempotency Keys

**Status:** ✅ FULLY IMPLEMENTED

| Component | Location |
|-----------|----------|
| Domain entity | internal/domain/idempotency.go (IdempotencyKey entity, DTOs, status constants) |
| Cache extension | internal/cache/cache.go (SetNX method on Driver interface), internal/cache/redis.go, internal/cache/memory.go, internal/cache/noop.go |
| Repository | internal/repository/idempotency.go (IdempotencyKeyRepository interface + GORM impl) |
| Service | internal/service/idempotency.go (IdempotencyService: CRUD + embedded reaper) |
| Middleware | internal/http/middleware/idempotency.go (request/response capture, Redis-first guard, header replay) |
| Handler | internal/http/handler/idempotency.go (5 endpoints: List, GetByID, Delete, TriggerCleanup + audit logging) |
| Config | internal/config/idempotency.go (IdempotencyConfig with defaults) |
| Migration | migrations/000027_create_idempotency_keys.up.sql + .down.sql |
| Unit tests | tests/unit/idempotency_service_test.go (20 tests), internal/domain/idempotency_test.go (11 tests), internal/http/middleware/idempotency_test.go (25 tests) |
| Integration tests | tests/integration/idempotency_handler_test.go (16 tests) |

**Key Features:**
- Redis-first idempotency: SETNX for in-flight guard, async goroutine for DB write
- Two Redis keys per request: :lock (5min TTL) and :record (24h TTL)
- Key scoping: idem:{orgID}:{userID}:{method}:{path}:{key}
- 409 Conflict + Retry-After for in-flight duplicates
- 4KB Redis cache threshold; larger responses store reference only
- Fail-open on Redis errors
- Response header replay via allowlist (Content-Type, Content-Encoding, Content-Length, Location, X-Request-Id, Retry-After)
- Background reaper with soft-delete via deleted_at
- Key format validation: max 128 chars, ^[a-zA-Z0-9_-]+$

**Endpoints (admin only):**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | /api/v1/idempotency/keys | idempotency:view | List keys (paginated) |
| GET | /api/v1/idempotency/keys/:id | idempotency:view | Get key by ID |
| DELETE | /api/v1/idempotency/keys/:id | idempotency:manage | Delete key (soft-delete) |
| POST | /api/v1/idempotency/cleanup | idempotency:manage | Trigger cleanup (accepted async) |

**Middleware Integration:**
`
Request → Recover → RequestID → CORS → RateLimit → JWT → Idempotency → Handler
                                                              │
                                                              ▼
                                                    Check Idempotency-Key header
                                                    ├─ No key → pass through
                                                    ├─ Redis hit → replay cached response
                                                    └─ Redis miss → SETNX lock → process → cache result
`

**Configuration (Environment Variables):**
| Variable | Default | Description |
|----------|---------|-------------|
| IDEMPOTENCY_ENABLED | true | Enable/disable idempotency middleware |
| IDEMPOTENCY_LOCK_TTL | 300s | Lock TTL for in-flight requests |
| IDEMPOTENCY_RECORD_TTL | 86400s | Record TTL for cached responses |
| IDEMPOTENCY_REAPER_INTERVAL | 60s | Cleanup interval for expired keys |
| IDEMPOTENCY_REAPER_RETENTION_DAYS | 7 | Days before soft-deleting records |
| IDEMPOTENCY_MAX_KEY_LENGTH | 128 | Max characters for idempotency key |

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| idempotency:view | idempotency | view | List and view keys |
| idempotency:manage | idempotency | manage | Delete keys; trigger cleanup |

**Startup Wiring:**
`go
// Idempotency service + middleware (cmd/api/main.go)
idempotencyRepo := repository.NewIdempotencyKeyRepository(db)
idempotencyService := service.NewIdempotencyService(
    idempotencyRepo, cacheDriver, auditService, enforcer, slog.Default(), cfg.Idempotency,
)
idempotencyHandler := handler.NewIdempotencyHandler(idempotencyService, auditService, enforcer)
idempotencyHandler.RegisterRoutes(v1, jwtSecret)
idempotencyService.StartReaper(ctx)

// Graceful shutdown order:
// server → eventBus → activityReaper → analyticsReaper → idempotencyService → workers → enforcer → db → redis
`

**Design Decisions:**
- cache.Driver extended with SetNX method (all 3 backends: Redis, Memory, Noop)
- Middleware wraps cho.Response to capture status, body, and replay headers
- 
eplayHeaders allowlist: Content-Type, Content-Encoding, Location, X-Request-Id, Retry-After
- Goroutine for DB write after Redis cache (fire-and-forget, fail-open)
- Panic recovery in storeResult goroutine
- Audit logging on Delete and TriggerCleanup (handler-level)
- Content-Length added to replayHeaders allowlist
- CleanupExpired uses soft-delete via deleted_at (not hard delete)
- RequirePermission(enforcer, resource, action) returns cho.MiddlewareFunc
- Handler-level permission enforcement only — IdempotencyService does NOT call enforcer.Enforce()
- Idempotency-Key header is validated before processing: max 128 chars, alphanumeric + hyphens/underscores
- Fail-open: Redis errors do NOT block requests; middleware proceeds without idempotency guarantee


## Summary: Implemented vs Not Implemented

### ✅ Implemented (18 features — V1 Complete + Idempotency Keys)

| Feature | Priority | Status | Source |
|---------|----------|--------|--------|
| Organization/Team | P1 | ✅ Complete | V1 |
| API Key Auth | P1 | ✅ Complete | V1 |
| Email Service (SMTP + SendGrid + SES) | P1 | ✅ Complete | V1 |
| Notification System | P1 | ✅ Complete | V1 |
| Webhook System | P1 | ✅ Complete | V1 |
| 2FA | P2 | ✅ Complete | V1 |
| Search & Filtering | P2 | ✅ Complete | V1 |
| File Versioning | P2 | ✅ Complete | V1 |
| Comment System | P2 | ✅ Complete | V1 |
| Tagging System | P2 | ✅ Complete | V1 |
| Activity Feed / Timeline | P2 | ✅ Complete | V1 |
| Background Job Queue | P3 | ✅ Complete | V1 |
| Data Import/Export System | P3 | ✅ Complete | V1 |
| Settings & Configuration | P3 | ✅ Complete | V1 |
| Feature Flags | P3 | ✅ Complete | V1 |
| WebSocket Real-time | P3 | ✅ Complete | V1 |
| Analytics Dashboard | P3 | ✅ Complete | V1 |

### ✅ Implemented (V2 — 1 feature complete)

| Feature | Priority | Status | Source |
|---------|----------|--------|--------|
| SSRF Protection | P1-Security | ✅ Complete | V2 §1.2 |

### ❌ Not Implemented (13 features — V2 remaining)

| Feature | Priority | Complexity | Est. Effort | Dependencies | Source |
|---------|----------|------------|-------------|--------------|--------|
| Idempotency Keys | P1-Security | Medium | 2d | Redis + middleware | V2 §1.1 |
| OAuth Social Login | P1-Security | Medium-High | 3-4d | JWT auth system | V2 §1.3 |
| Session Management | P2-SaaS | Low-Medium | 2d | Refresh tokens | V2 §2.1 |
| Rate Limit Tiers | P2-SaaS | Low | 1-2d | Rate limiter + orgs | V2 §2.2 |
| API Versioning | P2-SaaS | Low | 1d | Echo router groups | V2 §2.3 |
| GDPR Data Export | P2-SaaS | Low | 1d | Export service + job queue | V2 §2.4 |
| Circuit Breaker | P3-Production | Medium | 2d | HTTP clients | V2 §3.1 |
| OpenTelemetry | P3-Production | Medium | 2-3d | Structured logging | V2 §3.2 |
| Enhanced Health | P3-Production | Low | 0.5d | Health handler | V2 §3.3 |
| Inbound Webhooks | P4-Advanced | Medium-High | 3d | Webhook system + job queue | V2 §4.1 |
| Scheduled Tasks | P4-Advanced | Medium | 2d | Job queue | V2 §4.2 |
| Bulk Operations | P4-Advanced | Medium | 2d | Job queue | V2 §4.3 |
| Soft Delete Recovery | P4-Advanced | Low | 1d | Soft delete pattern | V2 §4.4 |

### Implementation Priority Matrix

| Feature | Priority | Complexity | Effort | Business Value | Builds On | Recommend |
|---------|----------|------------|--------|----------------|-----------|-----------|
| Idempotency Keys | P1-Security | Medium | 2d | Critical | Redis + middleware | ✅ Start Here |
| SSRF Protection | P1-Security | Low | 1-2d | Critical | Webhook worker | ✅ COMPLETE |
| OAuth Social Login | P1-Security | Medium-High | 3-4d | High | JWT auth system | ✅ User Expectation |
| Session Management | P2-SaaS | Low-Medium | 2d | High | Refresh tokens | ⚡ Quick Win |
| Rate Limit Tiers | P2-SaaS | Low | 1-2d | High | Rate limiter + orgs | ⚡ Monetization |
| API Versioning | P2-SaaS | Low | 1d | Medium | Echo router groups | ⚡ Future-proof |
| GDPR Data Export | P2-SaaS | Low | 1d | Medium | Export service + job queue | ⚡ Compliance |
| Circuit Breaker | P3-Production | Medium | 2d | Medium | HTTP clients | 🔧 Resilience |
| OpenTelemetry | P3-Production | Medium | 2-3d | Medium | Structured logging | 🔧 Observability |
| Enhanced Health | P3-Production | Low | 0.5d | Low | Health handler | 🔧 K8s-ready |
| Inbound Webhooks | P4-Advanced | Medium-High | 3d | Medium | Webhook system + job queue | 📡 Integration |
| Scheduled Tasks | P4-Advanced | Medium | 2d | Medium | Job queue | 📡 Automation |
| Bulk Operations | P4-Advanced | Medium | 2d | Medium | Job queue | 📡 Efficiency |
| Soft Delete Recovery | P4-Advanced | Low | 1d | Low-Medium | Soft delete pattern | 🔧 Safety Net |

---

## Files Referenced

- Source (V1): `docs/FEATURE_RECOMMENDATIONS.md`
- Source (V2): `docs/FEATURE_RECOMMENDATIONS_V2.md`
- This doc: `docs/FEATURE_STATUS.md`