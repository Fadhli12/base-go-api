# Quickstart: Background Job Queue

**Feature**: Background Job Queue
**Date**: 2026-05-06

---

## Overview

The job queue allows you to submit tasks for asynchronous background processing. Jobs are processed by workers and you can track their status via API.

---

## Submitting a Job

```bash
# Submit a job with JWT token
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Authorization: Bearer <your-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "payload": {"to": "user@example.com", "template": "welcome"},
    "callback_url": "https://example.com/webhook/job-complete"
  }'
```

**Response:**
```json
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2026-05-06T10:00:00Z"
  }
}
```

---

## Checking Job Status

```bash
curl http://localhost:8080/api/v1/jobs/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer <your-jwt>"
```

**Pending/Processing:**
```json
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "email",
    "status": "processing",
    "attempt_count": 1,
    "created_at": "2026-05-06T10:00:00Z",
    "started_at": "2026-05-06T10:00:01Z"
  }
}
```

**Completed:**
```json
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

**Failed with retry scheduled:**
```json
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "email",
    "status": "failed",
    "attempt_count": 1,
    "max_retries": 3,
    "last_error": "connection refused",
    "next_retry_at": "2026-05-06T10:01:00Z"
  }
}
```

---

## Listing Your Jobs

```bash
# List all jobs
curl http://localhost:8080/api/v1/jobs \
  -H "Authorization: Bearer <your-jwt>"

# Filter by status
curl "http://localhost:8080/api/v1/jobs?status=failed" \
  -H "Authorization: Bearer <your-jwt>"

# Pagination
curl "http://localhost:8080/api/v1/jobs?limit=10&offset=20" \
  -H "Authorization: Bearer <your-jwt>"
```

---

## Resubmitting Dead Jobs

```bash
curl -X POST http://localhost:8080/api/v1/jobs/550e8400-e29b-41d4-a716-446655440000/resubmit \
  -H "Authorization: Bearer <your-jwt>"
```

---

## Webhook Callbacks

When you provide a `callback_url` when submitting a job, the system sends an HTTP POST to that URL when the job completes or fails:

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "email",
  "status": "completed",
  "result": {"message_id": "abc123"},
  "attempt_count": 1,
  "completed_at": "2026-05-06T10:00:05Z"
}
```

---

## Retry Behavior

| Attempt | Delay |
|---------|-------|
| 1 | 1 minute |
| 2 | 5 minutes |
| 3 | 30 minutes (capped) |

After 3 failed attempts, the job is marked as `dead` and no further automatic retries occur.

---

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| JOB_WORKER_COUNT | 5 | Number of concurrent workers |
| JOB_TIMEOUT | 30s | Max job processing time |
| JOB_MAX_RETRIES | 3 | Default max retries |
| JOB_RESULT_TTL | 7d | How long to retain results |
