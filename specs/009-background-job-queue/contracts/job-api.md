# Job Queue API Contract

**Feature**: Background Job Queue  
**Date**: 2026-05-06  
**Source**: Phase 1 of `/speckit.plan`

---

## Overview

HTTP API for job queue operations. All endpoints require JWT authentication unless noted.

**Base URL**: `/api/v1/jobs`

---

## Endpoints

### POST /api/v1/jobs

Submit a new job for background processing.

**Authentication**: Required (JWT)

**Request**:
```json
{
  "type": "email",
  "payload": {"to": "user@example.com", "template": "welcome"},
  "callback_url": "https://example.com/webhook/job-complete",
  "max_retries": 3
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | string | Yes | Job type identifier |
| payload | object | Yes | Job payload (JSON object) |
| callback_url | string | No | Webhook URL for completion notification |
| max_retries | integer | No | Max retry attempts (default: 3, max: 10) |

**Response** (202 Accepted):
```json
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2026-05-06T10:00:00Z"
  },
  "meta": {
    "request_id": "req-123"
  }
}
```

**Errors**:
- 400: Invalid payload (missing type, invalid JSON, > 1MB)
- 401: Unauthorized

---

### GET /api/v1/jobs/:id

Get job status and result.

**Authentication**: Required (JWT) - must be job owner or admin

**Response** (200 OK):
```json
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "email",
    "status": "completed",
    "result": {"message_id": "abc123"},
    "attempt_count": 1,
    "max_retries": 3,
    "created_at": "2026-05-06T10:00:00Z",
    "started_at": "2026-05-06T10:00:01Z",
    "completed_at": "2026-05-06T10:00:05Z"
  },
  "meta": {
    "request_id": "req-123"
  }
}
```

**Response** (job still processing):
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

**Response** (job failed, retry scheduled):
```json
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "email",
    "status": "failed",
    "attempt_count": 2,
    "max_retries": 3,
    "last_error": "connection refused",
    "next_retry_at": "2026-05-06T10:05:00Z"
  }
}
```

**Note**: During retry wait period, status remains `"failed"` with `next_retry_at` set. When the job is picked up for retry, status transitions directly to `"processing"` (no intermediate `"pending"` state).

**Errors**:
- 401: Unauthorized
- 403: Forbidden (not job owner)
- 404: Job not found

---

### GET /api/v1/jobs

List jobs for current user.

**Authentication**: Required (JWT)

**Query Parameters**:
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| status | string | - | Filter by status (pending, processing, completed, failed, dead) |
| type | string | - | Filter by job type |
| limit | integer | 20 | Max results (1-100) |
| offset | integer | 0 | Pagination offset |

**Response** (200 OK):
```json
{
  "data": [
    {"job_id": "...", "type": "email", "status": "completed", ...},
    {"job_id": "...", "type": "export", "status": "pending", ...}
  ],
  "meta": {
    "total": 42,
    "limit": 20,
    "offset": 0,
    "request_id": "req-123"
  }
}
```

---

### POST /api/v1/jobs/:id/resubmit

Resubmit a dead job for processing.

**Authentication**: Required (JWT) - must be job owner or admin

**Constraints**:
- Job status must be `dead`

**Response** (202 Accepted):
```json
{
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "attempt_count": 1,
    "created_at": "2026-05-06T10:00:00Z"
  }
}
```

**Note**: `attempt_count` resets to 1 (first attempt after resubmit), not 0.

**Errors**:
- 400: Job not in dead status
- 401: Unauthorized
- 403: Forbidden
- 404: Job not found

---

## Webhook Callback

When `callback_url` is provided, the system sends an HTTP POST when job completes or fails.

**Timing**: Within 5 seconds of job completion/failure

**Request**:
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

**Headers**:
```
Content-Type: application/json
X-Job-ID: 550e8400-e29b-41d4-a716-446655440000
X-Job-Status: completed
```

**Behavior**:
- Best-effort delivery (failures logged but not retried)
- 5-second timeout
- Response status ignored

---

## Error Response Format

All errors follow standard envelope:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "type is required",
    "details": {"field": "type", "reason": "required"}
  },
  "meta": {
    "request_id": "req-123"
  }
}
```

| HTTP Code | Error Code | Description |
|-----------|------------|-------------|
| 400 | VALIDATION_ERROR | Invalid request |
| 400 | PAYLOAD_TOO_LARGE | Payload exceeds 1MB |
| 401 | UNAUTHORIZED | Missing/invalid JWT |
| 403 | FORBIDDEN | Not job owner |
| 404 | NOT_FOUND | Job not found |
| 500 | INTERNAL_ERROR | Server error |
