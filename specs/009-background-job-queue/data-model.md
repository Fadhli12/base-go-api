# Data Model: Background Job Queue

**Feature**: Background Job Queue
**Date**: 2026-05-06
**Source**: Phase 1 of `/speckit.plan`

---

## Entity: Job

**Purpose**: Represents a unit of background work to be processed asynchronously.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | Primary Key, auto-generated | Unique job identifier |
| type | string | Required, max 64 chars | Job type identifier (e.g., "email", "export") |
| payload | JSON | Required, max 1MB | Job payload as JSON bytes |
| status | enum | Required | One of: pending, processing, completed, failed, dead |
| attempt_count | int | Default 1, max 100 | Number of processing attempts (1-based: 1 = first try) |
| max_retries | int | Default 3, range 0-10 | Maximum retry attempts before dead |
| last_error | string | Optional, max 1024 chars | Last error message if failed |
| result | JSON | Optional, max 1MB | Job result as JSON bytes |
| callback_url | string | Optional, valid URL | Webhook URL to notify on completion |
| user_id | UUID | Required | Owner (from JWT auth) |
| created_at | timestamp | Auto-generated | Job creation time |
| started_at | timestamp | Set when processing | Processing start time |
| completed_at | timestamp | Set when completed | Processing completion time |
| next_retry_at | timestamp | Set on retry | Scheduled retry time |

---

## Redis Storage Schema

### Sorted Set (Queue)
```
Key: jobs:queue
Type: Sorted Set
Score: next_retry_at timestamp (Unix milliseconds)
Member: job_id (UUID string)
```

**Operations**:
- `ZADD jobs:queue {timestamp} {job_id}` - enqueue
- `ZRANGE jobs:queue 0 0` - peek oldest
- `ZREM jobs:queue {job_id}` - dequeue
- `ZCOUNT jobs:queue -inf +inf` - pending count

### Hash (Job Data)
```
Key: jobs:data:{job_id}
Type: Hash
Fields: id, type, payload, status, attempt_count, max_retries, last_error, result, callback_url, user_id, created_at, started_at, completed_at, next_retry_at
TTL: 7 days (604800 seconds)
```

---

## Job Status State Machine

```
                         ┌─────────────────────────────────────┐
                         │                                     │
                         ▼                                     │
  ┌─────────┐   ┌────────────┐   ┌───────────┐   ┌───────┐   │
  │ pending │──▶│ processing │──▶│ completed │   │  dead │   │
  └─────────┘   └────────────┘   └───────────┘   └───────┘   │
                    │     │                          ▲        │
                    │     │         ┌────────────────┘        │
                    │     │         │                         │
                    ▼     ▼         │                         │
               ┌─────────┐          │                         │
               │  failed │◀─────────┘                         │
               └─────────┘  (if retries remain: re-queued      │
                              with next_retry_at, status stays  │
                              "failed" until picked up)        │
```

**Status During Retry Wait**: Job status remains `"failed"` with `next_retry_at` set. When a poller picks up the job (BZPOPMIN), status transitions directly to `"processing"`.

**Transitions**:
| From | To | Trigger |
|------|-----|---------|
| pending | processing | Poller claims job via BZPOPMIN |
| processing | completed | Handler succeeds |
| processing | failed | Handler returns error (with next_retry_at if retries remain) |
| failed | processing | Poller picks up retry job (skips pending state) |
| failed | dead | Retries exhausted (attempt_count >= max_retries) |

---

## Validation Rules

| Rule | Constraint |
|------|------------|
| type | Required, non-empty, max 64 chars |
| payload | Required, valid JSON, max 1MB |
| status | Must be valid enum value |
| attempt_count | Cannot exceed max_retries |
| callback_url | If provided, must be valid URL format |
| max_retries | Range 0-10, default 3 |

---

## Relationships

- **User → Job**: One-to-many (user owns jobs)
- **Job → Handler**: Many-to-one (handler registered by type)

---

## Redis Secondary Index (for list queries)

Since Redis sorted sets only support score-based ordering and hashes don't support secondary indexes, job listings (GET /api/v1/jobs?status=failed) require a separate index structure:

```
Key: jobs:user:{user_id}
Type: Sorted Set
Score: created_at timestamp
Member: job_id
```

Additional indexes for filtering:
- `jobs:status:{status}` - all jobs by status (for admin listing)
- `jobs:type:{type}` - all jobs by type

**Note**: All secondary indexes are optional. User job listings can use Redis SCAN + filtering if index complexity is a concern. Indexes improve list query performance but add operational overhead.
