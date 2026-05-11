# Quickstart: Feature Flags System

**Feature ID:** 013-feature-flags | **Date:** 2026-05-11

## Setup

1. Run migrations:
```bash
make migrate
```

2. Sync permissions:
```bash
go run ./cmd/api permission:sync
```

3. Assign `feature_flag:manage` to admin role (via API or seed)

## Creating a Feature Flag

```bash
curl -X POST http://localhost:8080/api/v1/feature-flags \
  -H "Authorization: Bearer <admin-token>" \
  -H "X-Organization-ID: <org-id>" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "new_dashboard_v2",
    "name": "New Dashboard V2",
    "description": "Enable the redesigned dashboard experience",
    "enabled": true,
    "rollout": 50,
    "conditions": {
      "user_ids": ["550e8400-e29b-41d4-a716-446655440000"],
      "envs": ["staging"]
    }
  }'
```

**Response** (201 Created):
```json
{
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "key": "new_dashboard_v2",
    "name": "New Dashboard V2",
    "description": "Enable the redesigned dashboard experience",
    "enabled": true,
    "rollout": 50,
    "conditions": {"user_ids": ["550e8400-e29b-41d4-a716-446655440000"], "envs": ["staging"]},
    "is_system": false,
    "created_at": "2026-05-11T10:00:00Z",
    "updated_at": "2026-05-11T10:00:00Z"
  }
}
```

## Evaluating a Flag

```bash
curl http://localhost:8080/api/v1/feature-flags/new_dashboard_v2/evaluate \
  -H "Authorization: Bearer <user-token>"
```

**Response** (200 OK):
```json
{
  "data": {
    "key": "new_dashboard_v2",
    "enabled": true,
    "reason": "rollout",
    "rollout": 50
  }
}
```

## Evaluating All Flags

```bash
curl http://localhost:8080/api/v1/feature-flags/evaluate \
  -H "Authorization: Bearer <user-token>"
```

**Response** (200 OK):
```json
{
  "data": {
    "flags": [
      {"key": "new_dashboard_v2", "enabled": true, "reason": "rollout", "rollout": 50},
      {"key": "dark_mode", "enabled": false, "reason": "disabled", "rollout": 0}
    ]
  }
}
```

## Updating a Flag

```bash
curl -X PUT http://localhost:8080/api/v1/feature-flags/660e8400-e29b-41d4-a716-446655440001 \
  -H "Authorization: Bearer <admin-token>" \
  -H "X-Organization-ID: <org-id>" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "rollout": 100
  }'
```

## Deleting a Flag (Soft Delete)

```bash
curl -X DELETE http://localhost:8080/api/v1/feature-flags/660e8400-e29b-41d4-a716-446655440001 \
  -H "Authorization: Bearer <admin-token>" \
  -H "X-Organization-ID: <org-id>"
```

**Note**: System flags (`is_system: true`) cannot be deleted. Returns 403 Forbidden.

## Programmatic Usage (Go)

```go
// In application code, check if a feature is enabled:
if featureFlagService.IsEnabled(ctx, "new_dashboard_v2", userID) {
    // Show new dashboard
} else {
    // Show legacy dashboard
}

// With reason for analytics/debugging:
result, err := featureFlagService.IsEnabledWithReason(ctx, "new_dashboard_v2", userID)
if result.Enabled {
    log.Info("feature enabled", "reason", result.Reason)
}
```

## Key Format Rules

- Lowercase letters and digits only
- Must start with a letter
- Underscores allowed (no spaces, hyphens, or uppercase)
- Maximum 100 characters
- Examples: `new_dashboard`, `api_v2_endpoint`, `beta_features`

## Rollout Behavior

| Rollout | Behavior |
|---------|----------|
| 0 | No users see the feature |
| 50 | ~50% of users see it (deterministic by userID hash) |
| 100 | All users see the feature |

The rollout percentage uses a deterministic hash (`FNV-1a(userID + key) % 100`), so the same user always gets the same result for a given flag.