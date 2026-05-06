# Implementation Plan: Background Job Queue

**Branch**: `009-background-job-queue` | **Date**: 2026-05-06 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/009-background-job-queue/spec.md`

## Summary

Implement a Redis-based background job queue system that allows async task processing with retry logic, status tracking, and webhook callbacks. Jobs are submitted via HTTP API, queued in Redis using sorted sets, and processed by worker goroutines. Failed jobs automatically retry with exponential backoff, and dead letter jobs are preserved for manual review.

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: Redis (existing), Go stdlib `net/http`, `encoding/json`, `log/slog`  
**Storage**: Redis sorted sets for queue, Redis hashes for job data  
**Testing**: `go test`, testcontainers-go for integration tests, mocks for unit tests  
**Target Platform**: Linux server (API service)  
**Project Type**: Web service (Go REST API)  
**Performance Goals**: 500 job submissions/min, <100ms submission latency, <50ms status API  
**Constraints**: <200ms p99 latency, job timeout 30s default, results retained 7 days  
**Scale/Scope**: Multi-instance deployment, Redis pub/sub for coordination  

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready | ✅ | Error handling, graceful shutdown, structured logging all covered |
| II. RBAC Mandatory | ✅ | Job submission requires JWT auth; status API requires user or admin |
| III. Soft Deletes | ⚠️ | Jobs are immutable (status changes only); deleted via expiration not soft delete |
| IV. Stateless JWT | ✅ | JWT auth for API; job ownership checked via user_id claim |
| V. PostgreSQL + Migrations | N/A | Using Redis, not PostgreSQL for job storage |
| VI. Multi-Instance Permission | N/A | No permission changes in job queue |
| VII. Audit Logging | ✅ | Job lifecycle (submit, start, complete, fail, retry) logged via slog |

**Constitution Violations**: None significant. Jobs use Redis expiration (TTL) for retention rather than soft deletes - justified because job results are temporary and should auto-expire.

## Project Structure

### Documentation (this feature)

```text
specs/009-background-job-queue/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── job-api.md       # HTTP API contract
└── tasks.md             # Phase 2 output (/speckit.tasks - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   └── job.go           # Job entity, JobStatus enum
├── config/
│   └── job.go           # Job queue config (worker count, retry backoff, etc.)
├── repository/
│   └── job.go           # JobRepository interface + Redis implementation
├── service/
│   ├── job.go           # JobService (submit, status, retry)
│   ├── job_handler.go   # Job handler interface + built-in handlers
│   ├── job_worker.go    # Worker pool with embedded queue poller
│   ├── job_callback.go   # Webhook callback delivery
│   └── job_reaper.go    # Stuck job recovery
└── http/
    ├── handler/
    │   └── job.go       # JobHandler (submit, status, list, resubmit)
    └── middleware/
        └── job.go       # Job ownership middleware

tests/
├── unit/
│   ├── job_service_test.go
│   └── job_worker_test.go
└── integration/
    └── job_test.go
```

**Structure Decision**: Jobs use Redis as primary storage (sorted set queue + hash for job data). No PostgreSQL tables needed. Worker is part of service layer. Queue poller is embedded in the worker pool for efficient Redis-to-channel bridging.

**Structure Decision**: Jobs use Redis as primary storage (sorted set queue + hash for job data). No PostgreSQL tables needed. Worker is part of service layer.

### Queue Poller Component

To bridge Redis sorted sets and worker goroutines, a **Queue Poller** runs in dedicated goroutines that continuously poll Redis for available jobs:

```
┌─────────────────────────────────────────────────────────────┐
│  jobs:queue (Redis sorted set, score = next_retry_at)       │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │ ZRANGE 0 0 WITHSCORES
                              │ (peek oldest job without removing)
                              │
                    ┌─────────┴──────────┐
                    │   Queue Poller     │ (N goroutines, configurable)
                    │  BZPOPMIN 5s      │ (blocking pop with timeout)
                    └─────────┬──────────┘
                              │ job claimed from Redis
                              ▼
                    ┌─────────────────┐
                    │  Job Channel    │ (buffered Go channel)
                    └────────┬────────┘
                             │ <- worker goroutines consume
                             ▼
                    ┌─────────────────┐
                    │  Worker Pool   │ (fixed goroutines)
                    └─────────────────┘
```

**Poller-to-Worker Flow**:
1. Poller uses `BZPOPMIN jobs:queue 5` (blocking, 5s timeout)
2. When job found (score <= now), poller claims it: `ZREM jobs:queue {job_id}` + update status to "processing" in hash
3. Poller pushes job onto buffered channel (capacity = worker count × 2)
4. Worker receives from channel, processes job, updates result

**Why this pattern**:
- `BZPOPMIN` atomically removes and returns the job - no separate ZREM needed
- Channel buffering prevents workers from starving during Redis round-trip latency
- Blocking pop with timeout enables graceful shutdown without extra polling loops

**Graceful Shutdown**:
1. Cancel poller context → `BZPOPMIN` returns immediately with ctx.Done()
2. Wait for in-flight Redis calls to complete (with timeout)
3. Drain buffered channel (workers finish in-progress jobs)
4. Workers exit after channel drains

## Phase 0: Research

### Research Findings

**1. Redis Sorted Set Queue Pattern**
- Use ZADD with score = timestamp for FIFO + priority ordering
- Jobs stored as Redis HASH with all attributes
- Use ZRANGE to dequeue (oldest first)
- Score-based retry: ZADD job back with next_retry_at timestamp

**2. Job State Machine**
```
pending → processing → completed
                   ↘ failed (if handler errors)
                              │
                              │ (if retries remain) ──→ ZADD with next_retry_at
                              │                        (status stays "failed")
                              │
                              ▼ (retries exhausted)
                             dead
```

**Status During Retry Wait**: When a job fails but retries remain, it stays in `"failed"` status with `next_retry_at` set. The job is re-queued in Redis sorted set with the next retry timestamp. When a worker picks it up, status transitions directly from `"failed"` to `"processing"` (no intermediate `"pending"` state).

**3. Worker Pool Pattern (Go)**
- N poller goroutines (configurable, default 5) run `BZPOPMIN` blocking on Redis sorted set
- Each poller claims job atomically (BZPOPMIN removes + returns in one operation)
- Claimed job pushed to buffered Go channel (size = workers × 2)
- Worker goroutines (configurable, default 5) consume from channel, process, update status
- Channel-based graceful shutdown with drain
- Re-queue on failure: ZADD job back with next_retry_at timestamp

**4. Retry Backoff Calculation**
- Attempt 1: 1 minute
- Attempt 2: 5 minutes  
- Attempt 3+: 30 minutes (capped)
- Formula: `delay = min(30min, base * 5^(attempt-1))` where base=1min

**5. Stuck Job Recovery**
- Jobs in "processing" state > 5 minutes considered stuck
- Reaper goroutine runs every 60s
- Resets stuck jobs to "pending" with incremented attempt_count

### Resolved Unknowns

- **Job ID**: UUID v4 generated on submit
- **Payload size**: Max 1MB (enforced at submission)
- **Callback delivery**: Best-effort HTTP POST, timeout 5s, no retry on callback failure
- **Worker count**: Configurable via env var, default 5
- **Queue key**: `jobs:queue` (sorted set), `jobs:data:{id}` (hash)

## Phase 1: Design & Contracts

### Data Model

**Job Entity** (`internal/domain/job.go`):
```go
type Job struct {
    ID           uuid.UUID  // Primary key
    Type         string     // Job type identifier (e.g., "email", "export")
    Payload      []byte     // JSON payload (max 1MB)
    Status       JobStatus  // pending/processing/completed/failed/dead
    AttemptCount int        // Current attempt number (1-based: 1 = first try)
    MaxRetries   int        // Maximum retry attempts (default 3)
    LastError    string     // Last error message if failed
    Result       []byte    // JSON result data if completed
    CreatedAt    time.Time // Job creation timestamp
    StartedAt    *time.Time // Processing start timestamp
    CompletedAt  *time.Time // Processing completion timestamp
    NextRetryAt  *time.Time // Scheduled retry timestamp (set during failed state for retry wait)
    CallbackURL  string     // Optional webhook URL
    UserID       uuid.UUID  // Owner (from JWT)
}

type JobStatus string
const (
    JobStatusPending    JobStatus = "pending"
    JobStatusProcessing JobStatus = "processing"
    JobStatusCompleted  JobStatus = "completed"
    JobStatusFailed     JobStatus = "failed"
    JobStatusDead       JobStatus = "dead"
)
```

**Job Handler Interface** (`internal/service/job_handler.go`):
```go
type JobHandler interface {
    JobType() string
    Handle(ctx context.Context, payload []byte) (result []byte, err error)
}
```

**Built-in Handlers** (extensible):
- `EmailJobHandler` - sends email via existing email service
- `ExportJobHandler` - generates CSV/PDF export

### API Contracts

**Submit Job** (`POST /api/v1/jobs`)
```json
Request:
{
  "type": "email",
  "payload": {"to": "user@example.com", "template": "welcome"},
  "callback_url": "https://example.com/webhook/job-complete",  // optional
  "max_retries": 3  // optional, default 3
}

Response (202 Accepted):
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2026-05-06T10:00:00Z"
  }
}
```

**Get Job Status** (`GET /api/v1/jobs/:id`)
```json
Response (200 OK):
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "email",
    "status": "completed",
    "result": {"message_id": "abc123"},
    "attempt_count": 1,
    "created_at": "2026-05-06T10:00:00Z",
    "started_at": "2026-05-06T10:00:01Z",
    "completed_at": "2026-05-06T10:00:05Z"
  }
}
```

**List User Jobs** (`GET /api/v1/jobs?status=failed&limit=20`)
```json
Response (200 OK):
{
  "data": [...jobs],
  "meta": {"total": 42, "limit": 20, "offset": 0}
}
```

**Resubmit Dead Job** (`POST /api/v1/jobs/:id/resubmit`)
```json
Response (202 Accepted):
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "attempt_count": 0  // reset
  }
}
```

### Agent Context Update

`<!-- SPECKIT START -->` updated:
```
Current Feature: [specs/009-background-job-queue/plan.md](specs/009-background-job-queue/plan.md) - Background Job Queue
Latest Feature: [specs/007-email-providers/spec.md](specs/007-email-providers/spec.md) - Email Providers (SendGrid + SES) ✅
```

## Complexity Tracking

> Not applicable - no constitution violations requiring justification.

## Implementation Phases

| Phase | Description | Key Files |
|-------|-------------|-----------|
| 1 | Domain entities + config | `internal/domain/job.go`, `internal/config/job.go` |
| 2 | Repository (Redis) | `internal/repository/job.go` |
| 3 | Service layer (submit, status, retry) | `internal/service/job.go`, `job_handler.go` |
| 4 | Worker pool | `internal/service/job_worker.go`, `job_callback.go` |
| 5 | HTTP handler + routes | `internal/http/handler/job.go`, `internal/http/middleware/job.go` |
| 6 | Stuck job reaper | `internal/service/job_reaper.go` |
| 7 | Unit tests | `tests/unit/job_service_test.go`, `job_worker_test.go` |
| 8 | Integration tests | `tests/integration/job_test.go` |
