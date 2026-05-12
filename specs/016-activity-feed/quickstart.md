# Activity Feed — Quickstart Guide

**Branch**: `016-activity-feed` | **Date**: 2026-05-12

---

## Overview

The Activity Feed provides a chronological stream of activities across the system. It integrates with the existing EventBus to automatically record domain events, supports per-user read tracking and resource following, and auto-archives activities older than 90 days.

---

## Prerequisites

- PostgreSQL database with migrations applied (migration `000023_create_activities`)
- EventBus initialized and running (same instance used by WebhookService)
- Casbin enforcer with `activity:view` and `activity:manage` permissions seeded
- Redis running (required for other services, not required specifically for activities)

---

## Quick Start

### 1. Run Migrations

```bash
make migrate
# Applies migrations/000023_create_activities.up.sql
# Creates: activities, activity_reads, activity_follows tables
```

### 2. Seed Permissions

```bash
make seed
# or: go run ./cmd/api permission:sync
# Adds: activity:view, activity:manage
```

### 3. Start the Server

```bash
make serve
```

The ActivityService will:
- Subscribe to EventBus events on startup
- Start the background reaper goroutine (90-day archival)
- Register activity feed routes under `/api/v1/activities`

---

## API Usage

### List Activities (Main Feed)

```bash
# Get activities for your organization
curl -H "Authorization: Bearer <token>" \
     -H "X-Organization-ID: <org-id>" \
     "http://localhost:8080/api/v1/activities?limit=20&offset=0"

# Filter by resource type
curl "http://localhost:8080/api/v1/activities?resource_type=invoice"

# Filter by action type
curl "http://localhost:8080/api/v1/activities?action_type=created"

# Show only unread activities
curl "http://localhost:8080/api/v1/activities?unread=true"

# Show only followed resources
curl "http://localhost:8080/api/v1/activities?following=true"

# Include archived activities (older than 90 days)
curl "http://localhost:8080/api/v1/activities?since=2026-01-01T00:00:00Z"
```

Response:
```json
{
  "data": {
    "activities": [
      {
        "id": "...",
        "actor_id": "...",
        "actor_name": "John Doe",
        "action_type": "created",
        "resource_type": "invoice",
        "resource_id": "...",
        "organization_id": "...",
        "metadata": {
          "description": "John Doe created invoice INV-001",
          "invoice_number": "INV-001"
        },
        "is_read": false,
        "is_following": true,
        "created_at": "2026-05-12T10:00:00Z"
      }
    ],
    "total": 42,
    "unread_count": 15,
    "limit": 20,
    "offset": 0
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

### Get Unread Count

```bash
curl -H "Authorization: Bearer <token>" \
     -H "X-Organization-ID: <org-id>" \
     "http://localhost:8080/api/v1/activities/count-unread"
```

### Mark All as Read

```bash
curl -X PUT -H "Authorization: Bearer <token>" \
     -H "X-Organization-ID: <org-id>" \
     "http://localhost:8080/api/v1/activities/read-all"
```

### Mark Single Activity as Read

```bash
curl -X PUT -H "Authorization: Bearer <token>" \
     "http://localhost:8080/api/v1/activities/<activity-id>/read"
```

### Follow a Resource

```bash
curl -X POST -H "Authorization: Bearer <token>" \
     -H "Content-Type: application/json" \
     -d '{"resource_type": "invoice", "resource_id": "<invoice-id>"}' \
     "http://localhost:8080/api/v1/activities/follow"
```

### List Follows

```bash
curl -H "Authorization: Bearer <token>" \
     "http://localhost:8080/api/v1/activities/follows"
```

### Unfollow a Resource

```bash
curl -X DELETE -H "Authorization: Bearer <token>" \
     "http://localhost:8080/api/v1/activities/follow/<follow-id>"
```

### Delete an Activity (Admin)

```bash
curl -X DELETE -H "Authorization: Bearer <token>" \
     "http://localhost:8080/api/v1/activities/<activity-id>"
# Requires activity:manage permission
```

---

## Event Subscription

The ActivityService automatically creates Activity records when domain events are published via the EventBus. No manual activity creation is needed — it's all event-driven.

**Supported events (mapped to Activity records):**

| EventBus Event | Activity ActionType |
|---------------|-------------------|
| `user.created` | `created` (resource: user) |
| `user.deleted` | `deleted` (resource: user) |
| `invoice.created` | `created` (resource: invoice) |
| `invoice.paid` | `paid` (resource: invoice) |
| `news.published` | `published` (resource: news) |
| `news.deleted` | `deleted` (resource: news) |
| `comment.created` | `created` (resource: comment) |

**To add a new event mapping:**
1. Add the mapping to `eventMapping` in `internal/service/activity.go`
2. (Optional) Add a description template for richer metadata

---

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `ACTIVITY_RETENTION_DAYS` | 90 | Days before activities are auto-archived |
| `ACTIVITY_DEFAULT_PAGE_SIZE` | 20 | Default pagination page size |
| `ACTIVITY_MAX_PAGE_SIZE` | 100 | Maximum pagination page size |
| `ACTIVITY_REAPER_INTERVAL` | 60s | How often the reaper checks for aged activities |

---

## Architecture

```
┌─────────────────────┐     ┌──────────────────┐
│  Existing Services  │────▶│    EventBus      │
│  (User, Invoice,   │     │  (Publish/       │
│   News, Comment)    │     │   Subscribe)    │
└─────────────────────┘     └────────┬─────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                │
                    ▼                ▼                ▼
            ┌──────────┐    ┌──────────────┐    ┌──────────┐
            │ Activity │    │   Webhook    │    │  Future  │
            │ Service  │    │   Service    │    │ Subs     │
            └────┬─────┘    └──────────────┘    └──────────┘
                 │
    ┌────────────┼────────────┐
    │            │            │
    ▼            ▼            ▼
┌────────┐ ┌──────────┐ ┌──────────┐
│Activity│ │Activity │ │Activity │
│  Repo  │ │ ReadRepo│ │FollowRep│
└────────┘ └──────────┘ └──────────┘

    ┌─────────────────┐
    │ Activity Reaper │ (background goroutine)
    │  (90-day archive)│
    └─────────────────┘
```

---

## Permissions

| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `activity:view` | activity | view | View feed, mark read, follow/unfollow |
| `activity:manage` | activity | manage | Delete activities (admin) |

Assign via role:
```bash
# Via API
POST /api/v1/roles/<role-id>/permissions
{ "permission_id": "<activity:view-permission-id>" }

# Or seed with other permissions
go run ./cmd/api permission:sync
```

---

## Testing

```bash
# Unit tests (no Docker required)
go test -v -race ./tests/unit/... -run Activity

# Integration tests (Docker required)
go test -v -tags=integration ./tests/integration/... -run Activity -timeout 5m
```

---

## 90-Day Archival

The Activity Reaper is a background goroutine that runs every 60 seconds:

1. Queries activities where `created_at < NOW() - INTERVAL '<retention_days> days'` AND `archived_at IS NULL` AND `deleted_at IS NULL`
2. Sets `archived_at = NOW()` (NOT `deleted_at` — this avoids GORM soft-delete confusion)
3. Batch processes in chunks of 1000

Archived activities are:
- Excluded from default feed queries (WHERE archived_at IS NULL)
- Accessible via `?since=<timestamp>` parameter (uses Unscoped() with archived_at IS NOT NULL filter)
- Still subject to GORM soft-delete filter (deleted_at IS NULL)

**Important distinction**:
- `archived_at` — set by the reaper for 90-day auto-archival. Does NOT trigger GORM soft-delete filter.
- `deleted_at` — set by explicit admin DELETE request. DOES trigger GORM soft-delete filter.

---

**Version**: 1.0 | **Created**: 2026-05-12