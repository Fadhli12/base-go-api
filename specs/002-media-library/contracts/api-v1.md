# API Contracts: Media Library v1

**Version**: 1.0  
**Date**: 2026-04-22  
**Base Path**: `/api/v1`  
**Content-Type**: `application/json`

---

## Overview

RESTful API for polymorphic media management. All endpoints require JWT authentication via `Authorization: Bearer <token>` header. Responses use a standard envelope format. File operations are atomic (upload + DB commit together).

---

## Response Envelope

All successful responses return:
```json
{
  "data": { ... },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

Error responses:
```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message",
    "details": { ... }
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

## Endpoints

### 1. Upload Media

**Endpoint**: `POST /models/{model_type}/{model_id}/media`

**Authorization**: `media:upload` on `{model_type}` domain (or owner exemption)

**Parameters**:

| Name | Type | Location | Required | Description |
|------|------|----------|----------|-------------|
| `model_type` | string | path | Yes | Model domain: `news`, `user`, `invoice`, etc. |
| `model_id` | uuid | path | Yes | Target model instance ID |
| `file` | binary | form | Yes | File to upload (multipart/form-data) |
| `collection` | string | form | No | Collection name (default: `default`) |
| `custom_properties` | object | form | No | JSONB metadata (URL-encoded JSON) |

**Request**:
```http
POST /api/v1/models/news/550e8400-e29b-41d4-a716-446655440000/media
Authorization: Bearer <token>
Content-Type: multipart/form-data

file=<binary>
collection=images
custom_properties={"alt_text":"Featured image","source":"user-upload"}
```

**Response** (201 Created):
```json
{
  "data": {
    "id": "uuid",
    "filename": "550e8400-e29b-41d4-a716-446655440001.jpg",
    "original_filename": "beach.jpg",
    "mime_type": "image/jpeg",
    "size": 1048576,
    "collection": "images",
    "disk": "s3",
    "model_type": "news",
    "model_id": "550e8400-e29b-41d4-a716-446655440000",
    "uploaded_by_id": "550e8400-e29b-41d4-a716-446655440099",
    "metadata": {
      "width": 1920,
      "height": 1080,
      "exif": {
        "camera": "Canon EOS 5D",
        "iso": 400,
        "shutter_speed": "1/250"
      }
    },
    "custom_properties": {
      "alt_text": "Featured image",
      "source": "user-upload"
    },
    "conversions": [
      {
        "name": "thumbnail",
        "size": 8192,
        "path": "conversions/550e8400-e29b-41d4-a716-446655440001/thumbnail.jpg",
        "metadata": {
          "width": 300,
          "height": 300
        },
        "created_at": "2026-04-22T18:30:01Z"
      }
    ],
    "url": "/media/550e8400-e29b-41d4-a716-446655440001/download",
    "created_at": "2026-04-22T18:30:00Z",
    "updated_at": "2026-04-22T18:30:00Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

**Error Cases**:
- `400 Bad Request`: Invalid file type, file too large (>100MB)
- `401 Unauthorized`: Missing/invalid JWT
- `403 Forbidden`: No `media:upload` permission on model type
- `404 Not Found`: Model instance not found
- `409 Conflict`: Storage quota exceeded

---

### 2. List Media for Model

**Endpoint**: `GET /models/{model_type}/{model_id}/media`

**Authorization**: `media:view` on `{model_type}` domain (or owner exemption)

**Query Parameters**:

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `collection` | string | No | — | Filter by collection name |
| `mime_type` | string | No | — | Filter by MIME type (e.g., `image/jpeg`) |
| `limit` | integer | No | 20 | Pagination limit (max 100) |
| `offset` | integer | No | 0 | Pagination offset |
| `sort` | string | No | `-created_at` | Sort field: `created_at`, `size`, `filename` (prefix `-` for DESC) |

**Request**:
```http
GET /api/v1/models/news/550e8400-e29b-41d4-a716-446655440000/media?collection=images&limit=10&offset=0
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "data": [
    {
      "id": "uuid",
      "filename": "550e8400-e29b-41d4-a716-446655440001.jpg",
      "original_filename": "beach.jpg",
      "mime_type": "image/jpeg",
      "size": 1048576,
      "collection": "images",
      "disk": "s3",
      "model_type": "news",
      "model_id": "550e8400-e29b-41d4-a716-446655440000",
      "uploaded_by_id": "550e8400-e29b-41d4-a716-446655440099",
      "metadata": {
        "width": 1920,
        "height": 1080
      },
      "custom_properties": {},
      "conversions": [
        {
          "name": "thumbnail",
          "size": 8192
        }
      ],
      "url": "/media/550e8400-e29b-41d4-a716-446655440001/download",
      "created_at": "2026-04-22T18:30:00Z",
      "updated_at": "2026-04-22T18:30:00Z"
    }
  ],
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z",
    "pagination": {
      "total": 25,
      "limit": 10,
      "offset": 0,
      "has_more": true
    }
  }
}
```

**Error Cases**:
- `401 Unauthorized`: Missing/invalid JWT
- `403 Forbidden`: No `media:view` permission
- `404 Not Found`: Model not found

---

### 3. Get Media Details

**Endpoint**: `GET /media/{media_id}`

**Authorization**: `media:view` (or owner/uploader exemption)

**Parameters**:

| Name | Type | Location | Required | Description |
|------|------|----------|----------|-------------|
| `media_id` | uuid | path | Yes | Media instance ID |

**Request**:
```http
GET /api/v1/media/550e8400-e29b-41d4-a716-446655440001
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "filename": "550e8400-e29b-41d4-a716-446655440001.jpg",
    "original_filename": "beach.jpg",
    "mime_type": "image/jpeg",
    "size": 1048576,
    "collection": "images",
    "disk": "s3",
    "model_type": "news",
    "model_id": "550e8400-e29b-41d4-a716-446655440000",
    "uploaded_by_id": "550e8400-e29b-41d4-a716-446655440099",
    "metadata": {
      "width": 1920,
      "height": 1080
    },
    "custom_properties": {
      "alt_text": "Beach sunset",
      "source": "user-upload"
    },
    "conversions": [
      {
        "name": "thumbnail",
        "size": 8192,
        "metadata": {
          "width": 300,
          "height": 300
        }
      },
      {
        "name": "preview",
        "size": 32768,
        "metadata": {
          "width": 800,
          "height": 600
        }
      }
    ],
    "url": "/media/550e8400-e29b-41d4-a716-446655440001/download",
    "created_at": "2026-04-22T18:30:00Z",
    "updated_at": "2026-04-22T18:30:00Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

**Error Cases**:
- `401 Unauthorized`: Missing/invalid JWT
- `403 Forbidden`: No permission to view this media
- `404 Not Found`: Media not found

---

### 4. Download Media

**Endpoint**: `GET /media/{media_id}/download`

**Authorization**: `media:download` (or owner exemption); supports signed URLs without JWT

**Query Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `conversion` | string | No | Download specific conversion (e.g., `thumbnail`). If omitted, serves original. |
| `sig` | string | No | HMAC-SHA256 signature (for unsigned URLs) |
| `expires` | integer | No | Unix timestamp. Required if `sig` provided. |

**Request** (with JWT):
```http
GET /api/v1/media/550e8400-e29b-41d4-a716-446655440001/download?conversion=thumbnail
Authorization: Bearer <token>
```

**Request** (with signature):
```http
GET /api/v1/media/550e8400-e29b-41d4-a716-446655440001/download?sig=abc123def456&expires=1703341200
```

**Response** (200 OK):
```
Content-Type: image/jpeg
Content-Length: 8192
Content-Disposition: attachment; filename="beach.jpg"

<binary file data>
```

**Error Cases**:
- `401 Unauthorized`: No JWT and no valid signature
- `403 Forbidden`: No permission and invalid signature
- `404 Not Found`: Media or conversion not found
- `410 Gone`: Media deleted (soft delete)

---

### 5. Get Download URL (with Signature)

**Endpoint**: `GET /media/{media_id}/url`

**Authorization**: `media:download` (or owner exemption)

**Query Parameters**:

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `conversion` | string | No | — | Generate URL for specific conversion (e.g., `thumbnail`) |
| `expires_in` | integer | No | 3600 | URL expiry in seconds (max 86400 = 24h) |

**Request**:
```http
GET /api/v1/media/550e8400-e29b-41d4-a716-446655440001/url?conversion=thumbnail&expires_in=7200
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "data": {
    "url": "https://cdn.example.com/media/550e8400-e29b-41d4-a716-446655440001/download?conversion=thumbnail&sig=abc123def456&expires=1703348400",
    "expires_at": "2026-04-22T20:30:00Z",
    "expires_in": 7200
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

**Error Cases**:
- `400 Bad Request`: Invalid `expires_in` (>86400 or <60)
- `401 Unauthorized`: Missing/invalid JWT
- `403 Forbidden`: No permission to download
- `404 Not Found`: Media or conversion not found

---

### 6. Delete Media

**Endpoint**: `DELETE /media/{media_id}`

**Authorization**: `media:delete` (or uploader exemption)

**Parameters**:

| Name | Type | Location | Required | Description |
|------|------|----------|----------|-------------|
| `media_id` | uuid | path | Yes | Media instance ID |
| `permanent` | boolean | query | No | Hard-delete (cascade files). Default: soft-delete only. |

**Request**:
```http
DELETE /api/v1/media/550e8400-e29b-41d4-a716-446655440001
Authorization: Bearer <token>
```

**Response** (204 No Content):
```
(empty body)
```

**Request** (with permanent deletion):
```http
DELETE /api/v1/media/550e8400-e29b-41d4-a716-446655440001?permanent=true
Authorization: Bearer <token>
```

**Error Cases**:
- `401 Unauthorized`: Missing/invalid JWT
- `403 Forbidden`: No `media:delete` permission
- `404 Not Found`: Media not found or already deleted

---

### 7. Update Media Metadata

**Endpoint**: `PATCH /media/{media_id}`

**Authorization**: Uploader only (owner exemption)

**Request Body**:
```json
{
  "custom_properties": {
    "alt_text": "Updated alt text",
    "tags": ["beach", "sunset"]
  }
}
```

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "filename": "550e8400-e29b-41d4-a716-446655440001.jpg",
    "original_filename": "beach.jpg",
    "mime_type": "image/jpeg",
    "size": 1048576,
    "collection": "images",
    "custom_properties": {
      "alt_text": "Updated alt text",
      "tags": ["beach", "sunset"]
    },
    "updated_at": "2026-04-22T18:35:00Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:35:00Z"
  }
}
```

**Error Cases**:
- `401 Unauthorized`: Missing/invalid JWT
- `403 Forbidden`: Not the uploader
- `404 Not Found`: Media not found
- `422 Unprocessable Entity`: Invalid custom_properties format

---

### 8. Publish Media (Mark as Ready)

**Endpoint**: `POST /media/{media_id}/publish`

**Authorization**: Uploader or `media:publish` permission

**Request**:
```http
POST /api/v1/media/550e8400-e29b-41d4-a716-446655440001/publish
Authorization: Bearer <token>
Content-Type: application/json

{
  "status": "published"
}
```

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "status": "published",
    "published_at": "2026-04-22T18:30:00Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

### 9. Admin: List All Media

**Endpoint**: `GET /admin/media`

**Authorization**: `media:admin` permission

**Query Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `model_type` | string | No | Filter by model type |
| `model_id` | uuid | No | Filter by model ID |
| `deleted` | boolean | No | Include soft-deleted media (default: false) |
| `limit` | integer | No | Pagination limit (default: 20, max: 100) |
| `offset` | integer | No | Pagination offset (default: 0) |

**Request**:
```http
GET /api/v1/admin/media?model_type=news&deleted=false&limit=50
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "data": [
    {
      "id": "uuid",
      "filename": "...",
      "model_type": "news",
      "model_id": "...",
      "uploaded_by_id": "...",
      "size": 1048576,
      "disk": "s3",
      "created_at": "2026-04-22T18:30:00Z",
      "deleted_at": null
    }
  ],
  "meta": {
    "pagination": {
      "total": 150,
      "limit": 50,
      "offset": 0,
      "has_more": true
    }
  }
}
```

---

### 10. Admin: Media Cleanup

**Endpoint**: `POST /admin/media/cleanup`

**Authorization**: `media:admin` permission

**Request Body**:
```json
{
  "dry_run": true,
  "orphaned_days": 30,
  "force": false
}
```

**Response** (200 OK):
```json
{
  "data": {
    "deleted_count": 42,
    "freed_bytes": 5368709120,
    "dry_run": true,
    "started_at": "2026-04-22T18:30:00Z",
    "completed_at": "2026-04-22T18:31:15Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:31:15Z"
  }
}
```

---

### 11. Admin: Storage Stats

**Endpoint**: `GET /admin/media/stats`

**Authorization**: `media:admin` permission

**Query Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `model_type` | string | No | Filter by model type |
| `groupby` | string | No | Group results: `model_type`, `disk`, `mime_type`. Default: none. |

**Request**:
```http
GET /api/v1/admin/media/stats?groupby=model_type
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "data": {
    "total_files": 1250,
    "total_bytes": 53687091200,
    "average_file_size": 42949672.96,
    "by_mime_type": {
      "image/jpeg": {
        "count": 850,
        "bytes": 40737418240
      },
      "application/pdf": {
        "count": 300,
        "bytes": 10737418240
      }
    },
    "by_model_type": {
      "news": {
        "count": 600,
        "bytes": 26843545600
      },
      "user": {
        "count": 650,
        "bytes": 26843545600
      }
    },
    "by_disk": {
      "s3": {
        "count": 1250,
        "bytes": 53687091200
      }
    }
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

## Error Response Codes

| Code | HTTP | Description |
|------|------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid JWT |
| `FORBIDDEN` | 403 | User lacks required permission |
| `NOT_FOUND` | 404 | Resource does not exist |
| `CONFLICT` | 409 | Quota exceeded or state conflict |
| `VALIDATION_ERROR` | 422 | Invalid request parameters |
| `INTERNAL_ERROR` | 500 | Server error |
| `STORAGE_ERROR` | 503 | Storage backend unavailable |

---

## Content Type Validation

**Allowed MIME types** (configurable per deployment):

### Images
- `image/jpeg`
- `image/png`
- `image/webp`
- `image/gif`
- `image/svg+xml`

### Documents
- `application/pdf`
- `application/msword`
- `application/vnd.openxmlformats-officedocument.wordprocessingml.document`
- `text/plain`
- `text/csv`

### Archives
- `application/zip`
- `application/x-rar-compressed`
- `application/x-7z-compressed`

**File size limits**:
- Single-part upload: max 100MB
- Multipart upload (v2): max 5GB

---

## Conversion Definitions

**Built-in conversions** (automatically generated on image upload):

| Name | Trigger | Output | Use Case |
|------|---------|--------|----------|
| `thumbnail` | image MIME type | 300×300px JPEG 85% quality | UI list views, thumbnails |
| `preview` | image MIME type | 800×600px JPEG 90% quality | Preview modals, lightbox |
| `medium` | `image/jpeg` (optional) | 1200×900px JPEG 92% quality | Content display |

**Custom conversions** can be defined per collection via configuration.

---

## Rate Limiting

- **Unauthenticated**: 10 req/min per IP
- **Authenticated**: 100 req/min per user
- **Admin**: 1000 req/min per user

Headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1703341200
```

---

## Pagination

All list endpoints support cursor and offset pagination:

**Offset pagination** (default):
```
GET /api/v1/models/news/550e8400.../media?limit=20&offset=0
```

**Response meta**:
```json
"meta": {
  "pagination": {
    "total": 125,
    "limit": 20,
    "offset": 0,
    "has_more": true
  }
}
```

---

## Versioning

**API Version**: 1.0  
**Supported Versions**: `/api/v1` (v0 deprecated)  
**Deprecation Policy**: Minor version changes backward-compatible. Major version (v2+) requires explicit migration.

Version negotiation via header:
```
Accept-Version: 1.0
```

---

## Security & Compliance

1. **All endpoints require authentication** except signed URLs (with valid HMAC + expiry)
2. **HTTPS required** in production (HTTP rejected)
3. **CORS enabled** for CDN domains only (configurable whitelist)
4. **Request signing** via HMAC-SHA256 for download URLs
5. **Audit logging** on all file operations (immutable audit_logs table)
6. **Soft deletes** preserve audit trail (can restore within 30 days)
