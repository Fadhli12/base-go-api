# HTTP API Contracts: Webhook System v1

**Version**: 1.0 | **Date**: 2026-05-02 | **Status**: Phase 1 Design

---

## Overview

All webhook endpoints follow existing API conventions: JWT authentication, standardized envelope responses, audit middleware on mutations.

**Base URL**: `/api/v1/webhooks`

**Authentication**: JWT Bearer token required for all endpoints.

**Permissions**: `webhooks:view` for read, `webhooks:manage` for write.

**Organization Context**: All webhook endpoints respect the `X-Organization-ID` header. The existing organization middleware extracts this header and injects the organization ID into the request context. If the header is absent, webhooks are scoped globally (`organization_id = NULL`). Requests with an organization ID require the user to be a member of that organization (enforced at handler layer).

---

## Endpoints

### POST /webhooks

**Purpose**: Create a new webhook subscription.

**Authentication**: Required. Permission: `webhooks:manage`

**Request**:
```json
{
  "name": "Invoice Notifications",
  "url": "https://example.com/webhooks/invoices",
  "events": ["invoice.created", "invoice.paid"],
  "active": true,
  "rate_limit": 100
}
```

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `name` | string | yes | 1-255 chars |
| `url` | string | yes | valid URL, HTTPS required (unless `WEBHOOK_ALLOW_HTTP=true`), max 500 chars, no private IPs |
| `events` | []string | yes | non-empty, each must be a valid event constant |
| `active` | bool | no | defaults to true |
| `rate_limit` | int | no | 1-1000, defaults to 100 |

**Response** (201 Created):
```json
{
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "organization_id": null,
    "name": "Invoice Notifications",
    "url": "https://example.com/webhooks/invoices",
    "secret": "whsec_32byteshexencodedstring",
    "events": ["invoice.created", "invoice.paid"],
    "active": true,
    "rate_limit": 100,
    "created_at": "2026-05-02T10:00:00Z",
    "updated_at": "2026-05-02T10:00:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Note**: The `secret` field is ONLY returned on creation. Subsequent GET requests exclude it.

**Errors**:
- 400: Invalid URL, non-HTTPS URL, private IP, invalid event types, empty events
- 409: Duplicate URL for same organization
- 403: User lacks `webhooks:manage`

---

### GET /webhooks

**Purpose**: List webhooks for the authenticated user's organization.

**Authentication**: Required. Permission: `webhooks:view`

**Query Params**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Results per page (max 100) |
| `offset` | int | 0 | Pagination offset |
| `active` | bool | - | Filter by active status |
| `event` | string | - | Filter by subscribed event type |

**Response** (200 OK):
```json
{
  "data": {
    "webhooks": [
      {
        "id": "a1b2c3d4-...",
        "name": "Invoice Notifications",
        "url": "https://example.com/webhooks/invoices",
        "events": ["invoice.created", "invoice.paid"],
        "active": true,
        "rate_limit": 100,
        "created_at": "2026-05-02T10:00:00Z",
        "updated_at": "2026-05-02T10:00:00Z"
      }
    ],
    "total": 5,
    "limit": 20,
    "offset": 0
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 403: User lacks `webhooks:view`

---

### GET /webhooks/:id

**Purpose**: Get a specific webhook by ID.

**Authentication**: Required. Permission: `webhooks:view`

**Response** (200 OK):
```json
{
  "data": {
    "id": "a1b2c3d4-...",
    "organization_id": null,
    "name": "Invoice Notifications",
    "url": "https://example.com/webhooks/invoices",
    "events": ["invoice.created", "invoice.paid"],
    "active": true,
    "rate_limit": 100,
    "created_at": "2026-05-02T10:00:00Z",
    "updated_at": "2026-05-02T10:00:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 404: Webhook not found
- 403: User not in organization / lacks view permission

---

### PUT /webhooks/:id

**Purpose**: Update webhook configuration.

**Authentication**: Required. Permission: `webhooks:manage`

**Request**:
```json
{
  "name": "Invoice & Payment Notifications",
  "url": "https://example.com/webhooks/v2/invoices",
  "events": ["invoice.created", "invoice.paid", "invoice.cancelled"],
  "active": true,
  "rate_limit": 200
}
```

All fields optional — only provided fields are updated.

**Response** (200 OK): Same as GET /webhooks/:id

**Errors**:
- 400: Invalid URL or events
- 404: Webhook not found
- 409: Duplicate URL for same organization
- 403: User lacks `webhooks:manage`

---

### DELETE /webhooks/:id

**Purpose**: Soft-delete a webhook. Stops deliveries immediately.

**Authentication**: Required. Permission: `webhooks:manage`

**Response** (204 No Content)

**Errors**:
- 404: Webhook not found
- 403: User lacks `webhooks:manage`

---

### GET /webhooks/:id/deliveries

**Purpose**: List delivery history for a webhook.

**Authentication**: Required. Permission: `webhooks:view`

**Query Params**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Results per page (max 100) |
| `offset` | int | 0 | Pagination offset |
| `status` | string | - | Filter: queued, processing, delivered, failed, rate_limited |
| `event` | string | - | Filter by event type |

**Response** (200 OK):
```json
{
  "data": {
    "deliveries": [
      {
        "id": "d1e2f3g4-...",
        "webhook_id": "a1b2c3d4-...",
        "event": "invoice.created",
        "status": "delivered",
        "response_code": 200,
        "duration_ms": 245,
        "attempt_number": 1,
        "created_at": "2026-05-02T10:01:00Z",
        "delivered_at": "2026-05-02T10:01:00Z"
      }
    ],
    "total": 42,
    "limit": 20,
    "offset": 0
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 404: Webhook not found
- 403: User lacks `webhooks:view`

---

### GET /webhooks/:id/deliveries/:delivery_id

**Purpose**: Get full delivery details including payload and response body.

**Authentication**: Required. Permission: `webhooks:view`

**Response** (200 OK):
```json
{
  "data": {
    "id": "d1e2f3g4-...",
    "webhook_id": "a1b2c3d4-...",
    "event": "invoice.created",
    "payload": { "id": "inv-uuid", "amount": 15000 },
    "status": "delivered",
    "response_code": 200,
    "response_body": "{\"ok\":true}",
    "duration_ms": 245,
    "attempt_number": 1,
    "max_attempts": 3,
    "last_error": null,
    "next_retry_at": null,
    "created_at": "2026-05-02T10:01:00Z",
    "delivered_at": "2026-05-02T10:01:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

### POST /webhooks/:id/deliveries/:delivery_id/replay

**Purpose**: Manually replay a delivery. Resets the delivery to `queued` with `attempt_number=1` for immediate pickup by the worker. Any terminal or paused delivery (`delivered`, `failed`, `rate_limited`) can be replayed.

**Authentication**: Required. Permission: `webhooks:manage`

**Response** (200 OK):
```json
{
  "data": {
    "id": "d1e2f3g4-...",
    "status": "queued",
    "attempt_number": 1,
    "next_retry_at": null
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Note**: Replay resets `attempt_number` to 1, clears response fields (`response_code`, `response_body`, `duration_ms`, `last_error`, `delivered_at`), and sets `next_retry_at` to null for immediate processing.

**Errors**:
- 404: Webhook or delivery not found
- 409: Delivery is already in `queued` or `processing` state
- 403: User lacks `webhooks:manage`

---

## Webhook Delivery Payload (Outbound)

When an event triggers, the API sends a POST to the webhook URL:

**Request Headers**:
```
Content-Type: application/json
X-Webhook-ID: a1b2c3d4-...
X-Webhook-Event: invoice.created
X-Webhook-Signature: t=1683456789,v1=abcdef1234567890abcdef1234567890
X-Webhook-Attempt: 1
User-Agent: Go-API-Webhook/1.0
```

**Request Body**:
```json
{
  "id": "d1e2f3g4-e5f6-7890-abcd-ef1234567890",
  "event": "invoice.created",
  "timestamp": "2026-05-02T10:01:00Z",
  "data": {
    "id": "inv-uuid-...",
    "user_id": "user-uuid-...",
    "amount": 15000,
    "status": "draft",
    "created_at": "2026-05-02T10:00:00Z"
  }
}
```

**Receiver must respond with 2xx** within 10 seconds to be considered successful.

---

**Version**: 1.0 | **Created**: 2026-05-02 | **Status**: Phase 1 Design