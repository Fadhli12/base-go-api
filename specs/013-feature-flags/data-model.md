# Data Model: Feature Flags System

**Feature ID:** 013-feature-flags | **Date:** 2026-05-11

## Entity Relationship Diagram

```
FeatureFlag (1)
     │
     └── (evaluated by) ── FeatureFlagService.IsEnabled(key, userID)
                              │
                              ├── Hash(userID%key) → rollout check
                              └── Conditions JSONB → user/org/env check
```

No foreign key relationships to other entities. Feature flags are self-contained.

## Tables

### feature_flags

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | UUID | PK, DEFAULT gen_random_uuid() | Primary key |
| `key` | VARCHAR(100) | UNIQUE, NOT NULL | Feature flag key (lowercase + underscores) |
| `name` | VARCHAR(255) | NOT NULL | Human-readable name |
| `description` | TEXT | NULL | Optional description |
| `enabled` | BOOLEAN | NOT NULL, DEFAULT FALSE | Global on/off switch |
| `rollout` | INTEGER | NOT NULL, DEFAULT 100, CHECK (0-100) | Rollout percentage |
| `conditions` | JSONB | NULL | Conditions JSON: `{"user_ids":[...], "org_ids":[...], "envs":[...]}` |
| `is_system` | BOOLEAN | NOT NULL, DEFAULT FALSE | System flags cannot be deleted |
| `created_at` | TIMESTAMP | NOT NULL, DEFAULT NOW() | Creation timestamp |
| `updated_at` | TIMESTAMP | NOT NULL, DEFAULT NOW() | Last update timestamp |
| `deleted_at` | TIMESTAMP | NULL | Soft delete timestamp |

### Indexes

```sql
CREATE UNIQUE INDEX idx_feature_flags_key ON feature_flags(key) WHERE deleted_at IS NULL;
CREATE INDEX idx_feature_flags_deleted_at ON feature_flags(deleted_at);
```

**Note**: The unique index on `key` is partial (WHERE deleted_at IS NULL) to allow soft-deleted keys to be re-created.

## Conditions JSONB Schema

```json
{
  "user_ids": ["uuid1", "uuid2"],    // Explicit user allowlist
  "org_ids": ["org-uuid1"],           // Explicit org allowlist
  "envs": ["staging", "development"]  // Environment allowlist
}
```

All fields are optional. Unknown keys are rejected during validation.

## Evaluation Logic (Pseudocode)

```
IsEnabled(key, userID):
  flag = FindByKey(key)
  if flag == nil OR flag.enabled == false:
    return EvalResult(enabled=false, reason="disabled")
  if flag.rollout == 100:
    return EvalResult(enabled=true, reason="global_enabled")
  if flag.rollout == 0:
    return EvalResult(enabled=false, reason="rollout_excluded")
  hash = FNV1a(key + userID) % 100
  if hash < flag.rollout:
    return EvalResult(enabled=true, reason="rollout")
  if conditions.user_ids contains userID:
    return EvalResult(enabled=true, reason="condition_match")
  // org_ids and envs checks would require org context (future)
  return EvalResult(enabled=false, reason="no_match")
```