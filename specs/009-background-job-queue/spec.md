# Feature Specification: Background Job Queue

**Feature Branch**: `[009-background-job-queue]`
**Created**: 2026-05-06
**Status**: Draft
**Input**: User description: "Background job queue plan, execute use speckit.specify"

## User Scenarios & Testing

### User Story 1 - Submit Background Jobs (Priority: P1)

A developer or system component needs to submit a task for asynchronous processing without blocking the main request. The job should be queued immediately and processed in the background.

**Why this priority**: Core functionality - if jobs can't be submitted, nothing else matters. This is the fundamental capability that all other features build upon.

**Independent Test**: Can submit a job via API and verify it's queued in Redis within 100ms. Returns job ID immediately for status tracking.

**Acceptance Scenarios**:

1. **Given** the API is running and Redis is connected, **When** a POST request is made to submit a job with type and payload, **Then** the job is stored in Redis queue and a job ID is returned within 100ms

2. **Given** a job is queued, **When** the worker picks it up, **Then** the job status changes from "pending" to "processing"

3. **Given** a job completes successfully, **When** the worker finishes, **Then** the job status changes to "completed" with result data

4. **Given** a job fails, **When** the worker records the error, **Then** the job status changes to "failed" with error message and attempt count incremented

---

### User Story 2 - Monitor Job Status (Priority: P2)

Users and systems need to track the progress of submitted jobs to know when results are available and diagnose failures.

**Why this priority**: Without status visibility, users can't trust the system or debug issues. Critical for production reliability.

**Independent Test**: Can retrieve job status and result by job ID within 50ms.

**Acceptance Scenarios**:

1. **Given** a job has been submitted, **When** a GET request is made with the job ID, **Then** the current status (pending/processing/completed/failed) is returned with timestamps

2. **Given** a completed job exists, **When** status is retrieved, **Then** the result payload is included in the response

3. **Given** a failed job with retries remaining exists, **When** status is retrieved, **Then** the next retry time is included

---

### User Story 3 - Retry Failed Jobs (Priority: P3)

When jobs fail, the system should automatically retry them with configurable backoff to handle transient failures without manual intervention.

**Why this priority**: Reliability - transient failures (network timeouts, temporary unavailability) shouldn't cause permanent job failure.

**Independent Test**: Can configure retry policy and verify failed jobs are automatically re-queued according to backoff schedule.

**Acceptance Scenarios**:

1. **Given** a job fails and retries are configured (max 3, backoff 1m/5m/30m), **When** the worker records failure, **Then** the job is re-queued with next_retry_at set according to backoff

2. **Given** a job has exhausted all retries (3 attempts), **When** it fails again, **Then** status changes to "dead" (no more retries)

3. **Given** a dead letter job exists, **When** an admin resubmits it, **Then** the job is re-queued with fresh attempt count

---

### User Story 4 - Webhook Notifications on Job Completion (Priority: P4)

External systems need to be notified when jobs complete so they can proceed with downstream workflows without polling.

**Why this priority**: Enables integration patterns where the caller doesn't wait for job completion but gets notified via callback.

**Independent Test**: Can register a callback URL when submitting job and verify it's called with result when job completes.

**Acceptance Scenarios**:

1. **Given** a job is submitted with a callback_url, **When** the job completes or fails, **Then** an HTTP POST is sent to the callback_url with job result/status

2. **Given** the callback endpoint is unreachable, **When** notification is attempted, **Then** the failure is logged but job status remains completed (callback is best-effort)

---

### Edge Cases

| Scenario | Resolution |
|----------|------------|
| Redis unavailable during submission | Return 503 Service Unavailable with `Retry-After` header |
| Job exceeds timeout (default 30s) | Mark as failed, record timeout error, schedule retry if attempts remain |
| Worker crashes mid-processing | Stuck job reaper detects jobs in "processing" > 5 min, resets to retry |
| Malformed payload | Return 400 Bad Request with validation details before queueing |
| Queue depth exceeds memory limits | Redis eviction policy (maxmemory-policy) handles this; no explicit cap in application |
| Priority when backed up | Jobs processed in next_retry_at order (FIFO within same timestamp); urgent jobs can use lower next_retry_at score |

## Requirements

### Functional Requirements

- **FR-001**: System MUST accept job submissions via HTTP API with type identifier and JSON payload
- **FR-002**: System MUST store jobs in Redis with unique ID, status, attempts, and timestamps
- **FR-003**: System MUST process jobs asynchronously using worker goroutines that consume from Redis queue
- **FR-004**: System MUST support multiple job types with type-specific handlers
- **FR-005**: System MUST automatically retry failed jobs with configurable backoff (exponential or fixed)
- **FR-006**: System MUST limit retry attempts (configurable max retries, default 3)
- **FR-007**: System MUST provide job status API to query current state and results
- **FR-008**: System MUST support optional webhook callbacks on job completion/failure
- **FR-009**: System MUST record job execution time and result for auditing
- **FR-010**: System MUST handle worker crashes gracefully - mark stuck jobs as failed after timeout
- **FR-011**: System MUST use FIFO ordering with ability to prioritize urgent jobs
- **FR-012**: System MUST allow manual job resubmission from dead letter state

### Key Entities

- **Job**: Represents a unit of work. Attributes: ID (UUID), Type (string), Payload (JSON), Status (pending/processing/completed/failed/dead), AttemptCount, MaxRetries, LastError, Result (JSON), CreatedAt, StartedAt, CompletedAt, NextRetryAt, CallbackURL
- **Queue**: Redis-based queue using sorted set with score as timestamp for ordering. Jobs stored as hash with all attributes.
- **Worker**: Background goroutine pool that dequeues jobs and dispatches to type-specific handlers.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Job submission returns job ID within 100ms under normal load
- **SC-002**: Status API returns job state within 50ms
- **SC-003**: System handles 500 job submissions per minute without queuing delay
- **SC-004**: Failed jobs are retried automatically with backoff - 95% of network errors and temporary service unavailability recover within retry budget (defined as: connection timeout, 5xx responses, rate limit errors). Permanent failures (4xx client errors, validation errors, business logic errors) do not retry.
- **SC-005**: Dead letter jobs (exhausted retries) are stored separately for manual review
- **SC-006**: Workers process jobs within defined timeout (default 30 seconds)
- **SC-007**: Stuck jobs (processing >5 min without completion) are automatically recovered
- **SC-008**: Callback webhooks are delivered within 5 seconds of job completion

## Assumptions

- Redis is already available in the infrastructure (used for email queue, webhooks, etc.)
- Job payload sizes are reasonable (<1MB) - large payloads should use object storage references
- Worker concurrency can be configured - default of 5 concurrent workers
- Job timeouts are configurable per job type - default 30 seconds
- System uses existing structured logging (slog) for job lifecycle events
- Authentication: job submission requires authenticated user (JWT), status API requires same user or admin
- Retry backoff follows pattern: 1 minute, 5 minutes, 30 minutes (exponential capped)
- Job results are stored for 7 days then purged (configurable retention)
