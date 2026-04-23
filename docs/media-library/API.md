# Media Library API Reference

**Version**: 1.0 | **Base URL**: `/api/v1` | **Date**: 2026-04-22

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Response Format](#response-format)
4. [Endpoints](#endpoints)
5. [Error Codes](#error-codes)
6. [Rate Limiting](#rate-limiting)
7. [Pagination](#pagination)
8. [Signed URLs](#signed-urls)
9. [Content Types](#content-types)

---

## Overview

The Media Library API provides polymorphic file management capabilities. Any domain model (News, User, Invoice, etc.) can associate media files through a unified interface.

**Key Features**:
- Polymorphic associations (attach media to any model)
- Automatic image conversions (thumbnails, previews)
- Signed URL support for secure downloads
- Comprehensive audit logging
- RBAC permission control

---

## Authentication

All endpoints require JWT authentication via the `Authorization` header:

```http
Authorization: Bearer {access_token}
```

### Obtaining a Token

```bash
# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "your-password"
  }'
```

**Response**:
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "user": { ... }
  },
  "meta": { "request_id": "uuid", "timestamp": "2026-04-22T..." }
}
```

---

## Response Format

### Success Response

```json
{
  "data": { ... },
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2026-04-22T18:30:00Z",
    "pagination": {
      "total": 100,
      "limit": 20,
      "offset": 0,
      "has_more": true
    }
  }
}
```

### Error Response

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": "File size exceeds maximum allowed size"
  },
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440001",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

## Endpoints

### 1. Upload Media

Upload a file and associate it with a model.

**Endpoint**: `POST /models/{model_type}/{model_id}/media`

**Authorization**: `media:upload` or owner of the model

**Request**:
```http
POST /api/v1/models/news/550e8400-e29b-41d4-a716-446655440000/media
Authorization: Bearer {token}
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="file"; filename="beach.jpg"
Content-Type: image/jpeg

[binary file data]
--boundary
Content-Disposition: form-data; name="collection"

images
--boundary
Content-Disposition: form-data; name="custom_properties"

{"alt_text": "Beach sunset", "source": "user-upload"}
--boundary--
```

**Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `model_type` | string | Yes | Model type: `news`, `user`, `invoice`, etc. |
| `model_id` | uuid | Yes | UUID of the model instance |
| `file` | binary | Yes | File to upload (max 100MB) |
| `collection` | string | No | Collection name (default: `default`) |
| `custom_properties` | JSON | No | Custom metadata as JSON string |

**Response** (201 Created):
```json
{
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "model_type": "news",
    "collection_name": "images",
    "filename": "beach.jpg",
    "original_filename": "beach.jpg",
    "mime_type": "image/jpeg",
    "size": 1048576,
    "disk": "local",
    "uploaded_by_id": "550e8400-e29b-41d4-a716-446655440000",
    "metadata": {
      "width": 1920,
      "height": 1080,
      "format": "jpeg"
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
      }
    ],
    "url": "/media/660e8400-e29b-41d4-a716-446655440001/download",
    "created_at": "2026-04-22T18:30:00Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

**cURL Example**:
```bash
curl -X POST http://localhost:8080/api/v1/models/news/$NEWS_ID/media \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/image.jpg" \
  -F "collection=images" \
  -F "custom_properties={\"alt_text\":\"My Image\"}"
```

---

### 2. List Media for Model

List all media associated with a specific model.

**Endpoint**: `GET /models/{model_type}/{model_id}/media`

**Authorization**: `media:view` or owner of the model

**Query Parameters**:

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `collection` | string | No | — | Filter by collection name |
| `mime_type` | string | No | — | Filter by MIME type |
| `limit` | integer | No | 20 | Items per page (max 100) |
| `offset` | integer | No | 0 | Pagination offset |
| `sort` | string | No | `-created_at` | Sort field (`created_at`, `size`, `filename`) |

**Request**:
```http
GET /api/v1/models/news/550e8400-e29b-41d4-a716-446655440000/media?collection=images&limit=10&offset=0
Authorization: Bearer {token}
```

**Response** (200 OK):
```json
{
  "data": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "filename": "beach.jpg",
      "original_filename": "beach.jpg",
      "mime_type": "image/jpeg",
      "size": 1048576,
      "collection": "images",
      "url": "/media/660e8400-e29b-41d4-a716-446655440001/download",
      "conversions": [
        {
          "name": "thumbnail",
          "size": 8192
        }
      ],
      "created_at": "2026-04-22T18:30:00Z"
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

**cURL Example**:
```bash
curl "http://localhost:8080/api/v1/models/news/$NEWS_ID/media?collection=images&limit=20" \
  -H "Authorization: Bearer $TOKEN"
```

---

### 3. Get Media by ID

Retrieve details of a specific media file.

**Endpoint**: `GET /media/{media_id}`

**Authorization**: `media:view` or owner/uploader

**Request**:
```http
GET /api/v1/media/660e8400-e29b-41d4-a716-446655440001
Authorization: Bearer {token}
```

**Response** (200 OK):
```json
{
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "model_type": "news",
    "model_id": "550e8400-e29b-41d4-a716-446655440000",
    "collection_name": "images",
    "filename": "beach.jpg",
    "original_filename": "beach.jpg",
    "mime_type": "image/jpeg",
    "size": 1048576,
    "disk": "local",
    "path": "news/550e8400-e29b-41d4-a716-446655440000/660e8400-e29b-41d4-a716-446655440001.jpg",
    "metadata": {
      "width": 1920,
      "height": 1080
    },
    "custom_properties": {
      "alt_text": "Beach sunset"
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
    "url": "/media/660e8400-e29b-41d4-a716-446655440001/download",
    "created_at": "2026-04-22T18:30:00Z",
    "updated_at": "2026-04-22T18:30:00Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

### 4. Download Media

Download a media file directly. Supports signed URLs for unauthenticated access.

**Endpoint**: `GET /media/{media_id}/download`

**Authorization**: One of:
- JWT Bearer token with `media:download` permission
- Valid signed URL parameters (`sig` + `expires`)
- Owner/uploader of the media

**Query Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `conversion` | string | No | Download specific conversion (`thumbnail`, `preview`) |
| `sig` | string | No | HMAC-SHA256 signature (for signed URLs) |
| `expires` | integer | No | Unix timestamp (for signed URLs) |

**Request with JWT**:
```http
GET /api/v1/media/660e8400-e29b-41d4-a716-446655440001/download?conversion=thumbnail
Authorization: Bearer {token}
```

**Request with Signed URL**:
```http
GET /api/v1/media/660e8400-e29b-41d4-a716-446655440001/download?sig=abc123def456&expires=1703341200
```

**Response** (200 OK):
```http
Content-Type: image/jpeg
Content-Length: 8192
Content-Disposition: attachment; filename="beach_thumbnail.jpg"

[binary file data]
```

**cURL Example**:
```bash
# Download original
curl http://localhost:8080/api/v1/media/$MEDIA_ID/download \
  -H "Authorization: Bearer $TOKEN" \
  -o downloaded.jpg

# Download thumbnail
curl "http://localhost:8080/api/v1/media/$MEDIA_ID/download?conversion=thumbnail" \
  -H "Authorization: Bearer $TOKEN" \
  -o thumbnail.jpg
```

---

### 5. Generate Signed URL

Generate a time-limited signed URL for downloading media without authentication.

**Endpoint**: `GET /media/{media_id}/url`

**Authorization**: `media:download` or owner/uploader

**Query Parameters**:

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `conversion` | string | No | — | Generate URL for specific conversion |
| `expires_in` | integer | No | 3600 | URL expiry in seconds (max 86400 = 24h) |

**Request**:
```http
GET /api/v1/media/660e8400-e29b-41d4-a716-446655440001/url?conversion=thumbnail&expires_in=7200
Authorization: Bearer {token}
```

**Response** (200 OK):
```json
{
  "data": {
    "url": "http://localhost:8080/api/v1/media/660e8400-e29b-41d4-a716-446655440001/download?conversion=thumbnail&sig=abc123def456&expires=1703348400",
    "expires_at": "2026-04-22T20:30:00Z",
    "expires_in": 7200
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

**cURL Example**:
```bash
curl "http://localhost:8080/api/v1/media/$MEDIA_ID/url?expires_in=3600" \
  -H "Authorization: Bearer $TOKEN"
```

---

### 6. Update Media Metadata

Update custom properties for a media file.

**Endpoint**: `PATCH /media/{media_id}`

**Authorization**: Owner/uploader only

**Request**:
```http
PATCH /api/v1/media/660e8400-e29b-41d4-a716-446655440001
Authorization: Bearer {token}
Content-Type: application/json

{
  "custom_properties": {
    "alt_text": "Updated beach sunset",
    "tags": ["beach", "sunset", "vacation"]
  }
}
```

**Response** (200 OK):
```json
{
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "custom_properties": {
      "alt_text": "Updated beach sunset",
      "tags": ["beach", "sunset", "vacation"]
    },
    "updated_at": "2026-04-22T18:35:00Z"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:35:00Z"
  }
}
```

---

### 7. Delete Media

Soft delete a media file. Actual file cleanup happens via background job.

**Endpoint**: `DELETE /media/{media_id}`

**Authorization**: `media:delete` or owner/uploader

**Query Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `permanent` | boolean | No | Hard delete (cascade files). Default: false |

**Request**:
```http
DELETE /api/v1/media/660e8400-e29b-41d4-a716-446655440001
Authorization: Bearer {token}
```

**Response** (200 OK):
```json
{
  "data": {
    "message": "Media deleted successfully"
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

### 8. Admin: List All Media

List all media files with admin filtering (admin only).

**Endpoint**: `GET /admin/media`

**Authorization**: `media:manage` permission required

**Query Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `model_type` | string | No | Filter by model type |
| `model_id` | uuid | No | Filter by model ID |
| `deleted` | boolean | No | Include soft-deleted (default: false) |
| `limit` | integer | No | Pagination limit (default: 20, max: 100) |
| `offset` | integer | No | Pagination offset |

**Request**:
```http
GET /api/v1/admin/media?model_type=news&limit=50
Authorization: Bearer {admin_token}
```

**Response** (200 OK):
```json
{
  "data": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "filename": "beach.jpg",
      "model_type": "news",
      "model_id": "550e8400-e29b-41d4-a716-446655440000",
      "uploaded_by_id": "550e8400-e29b-41d4-a716-446655440000",
      "size": 1048576,
      "disk": "local",
      "created_at": "2026-04-22T18:30:00Z",
      "deleted_at": null
    }
  ],
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z",
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

### 9. Admin: Storage Statistics

Get storage statistics (admin only).

**Endpoint**: `GET /admin/media/stats`

**Authorization**: `media:manage` permission required

**Query Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `model_type` | string | No | Filter by model type |
| `groupby` | string | No | Group results: `model_type`, `disk`, `mime_type` |

**Request**:
```http
GET /api/v1/admin/media/stats?groupby=model_type
Authorization: Bearer {admin_token}
```

**Response** (200 OK):
```json
{
  "data": {
    "total_files": 1250,
    "total_size": 53687091200,
    "by_model_type": [
      {
        "model_type": "news",
        "count": 600,
        "size": 26843545600
      },
      {
        "model_type": "user",
        "count": 650,
        "size": 26843545600
      }
    ],
    "by_mime_type": [
      {
        "mime_type": "image/jpeg",
        "count": 850,
        "size": 40737418240
      }
    ],
    "by_disk": [
      {
        "disk": "s3",
        "count": 1250,
        "size": 53687091200
      }
    ]
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

### 10. Admin: Cleanup Orphaned Media

Trigger cleanup of orphaned media files (admin only).

**Endpoint**: `POST /admin/media/cleanup`

**Authorization**: `media:manage` permission required

**Query Parameters**:

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `cutoff_hours` | integer | No | 24 | Delete media orphaned longer than this |
| `dry_run` | boolean | No | true | Preview without actual deletion |

**Request**:
```http
POST /api/v1/admin/media/cleanup?cutoff_hours=24&dry_run=true
Authorization: Bearer {admin_token}
```

**Response** (200 OK):
```json
{
  "data": {
    "orphans_found": 42,
    "deleted_count": 0,
    "dry_run": true,
    "message": "Found 42 orphaned media files. Set dry_run=false to delete."
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T18:30:00Z"
  }
}
```

---

## Error Codes

| Code | HTTP | Description | Resolution |
|------|------|-------------|------------|
| `UNAUTHORIZED` | 401 | Missing or invalid JWT | Authenticate with valid token |
| `FORBIDDEN` | 403 | Permission denied | Check RBAC permissions |
| `NOT_FOUND` | 404 | Resource not found | Verify IDs are correct |
| `GONE` | 410 | Media deleted (soft delete) | Cannot access deleted media |
| `VALIDATION_ERROR` | 422 | Invalid request parameters | Check request format |
| `CONFLICT` | 409 | Quota exceeded | Free storage or upgrade plan |
| `STORAGE_ERROR` | 503 | Storage backend unavailable | Check storage configuration |
| `INTERNAL_ERROR` | 500 | Server error | Contact administrator |

---

## Rate Limiting

Rate limits are applied per IP address and user:

| Endpoint Type | Limit | Window |
|---------------|-------|--------|
| Public (download with signature) | 10 | per minute |
| Authenticated | 100 | per minute |
| Admin | 1000 | per minute |

**Response Headers**:
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1703341200
```

**Rate Limit Exceeded** (429):
```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests, please try again later"
  },
  "meta": { ... }
}
```

---

## Pagination

List endpoints support offset-based pagination:

**Request**:
```http
GET /api/v1/models/news/{id}/media?limit=20&offset=40
```

**Response Meta**:
```json
{
  "meta": {
    "pagination": {
      "total": 100,
      "limit": 20,
      "offset": 40,
      "has_more": true
    }
  }
}
```

**Navigation**:
- First page: `offset=0`
- Next page: `offset=current_offset + limit`
- Previous page: `offset=current_offset - limit` (if > 0)
- Last page: `offset=total - (total % limit)`

---

## Signed URLs

Signed URLs provide temporary access to media without requiring authentication.

### Generating Signed URLs

```bash
# Get signed URL valid for 1 hour
curl "http://localhost:8080/api/v1/media/$MEDIA_ID/url?expires_in=3600" \
  -H "Authorization: Bearer $TOKEN"

# Response
{
  "data": {
    "url": "http://localhost:8080/api/v1/media/xxx/download?sig=abc123&expires=1703341200",
    "expires_at": "2026-04-22T20:30:00Z",
    "expires_in": 3600
  }
}
```

### Using Signed URLs

```bash
# Download using signed URL (no auth required)
curl "http://localhost:8080/api/v1/media/xxx/download?sig=abc123&expires=1703341200" \
  -o file.jpg
```

### Security

- URLs expire after specified duration (max 24 hours)
- Signed with HMAC-SHA256 using `MEDIA_SIGNING_SECRET`
- Cannot be modified without invalidating signature
- One-time use not enforced (use short expiry for sensitive content)

---

## Content Types

### Allowed MIME Types

**Images**:
- `image/jpeg`
- `image/png`
- `image/webp`
- `image/gif`
- `image/svg+xml`

**Documents**:
- `application/pdf`
- `application/msword`
- `application/vnd.openxmlformats-officedocument.wordprocessingml.document`
- `text/plain`
- `text/csv`

**Archives**:
- `application/zip`
- `application/x-rar-compressed`
- `application/x-7z-compressed`

### File Size Limits

- **Single upload**: Maximum 100MB
- **Multipart upload**: Maximum 100MB (v2 will support larger)

### Blocked Extensions

These extensions are blocked regardless of MIME type:
- `.exe`, `.dll`, `.bat`, `.cmd`
- `.sh`, `.php`, `.jsp`, `.asp`, `.aspx`
- `.py`, `.rb`, `.pl`, `.cgi`

---

## Common Workflows

### Complete Upload Flow

```bash
# 1. Authenticate
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password"}' \
  | jq -r '.data.access_token')

# 2. Upload media to a news article
MEDIA_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/api/v1/models/news/$NEWS_ID/media" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@image.jpg" \
  -F "collection=featured")

MEDIA_ID=$(echo $MEDIA_RESPONSE | jq -r '.data.id')

# 3. Get media details
curl "http://localhost:8080/api/v1/media/$MEDIA_ID" \
  -H "Authorization: Bearer $TOKEN"

# 4. Download original
curl "http://localhost:8080/api/v1/media/$MEDIA_ID/download" \
  -H "Authorization: Bearer $TOKEN" \
  -o downloaded.jpg

# 5. Get signed URL for external sharing
SIGNED_URL=$(curl -s \
  "http://localhost:8080/api/v1/media/$MEDIA_ID/url?conversion=thumbnail&expires_in=7200" \
  -H "Authorization: Bearer $TOKEN" \
  | jq -r '.data.url')

# 6. Download using signed URL (no auth needed)
curl "$SIGNED_URL" -o thumbnail.jpg
```

### Batch Upload

```bash
# Upload multiple files
curl -X POST "http://localhost:8080/api/v1/models/news/$NEWS_ID/media" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@image1.jpg" \
  -F "collection=gallery"

curl -X POST "http://localhost:8080/api/v1/models/news/$NEWS_ID/media" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@image2.jpg" \
  -F "collection=gallery"
```

---

## Related Documentation

- [Architecture](./ARCHITECTURE.md) - System design and components
- [Configuration](./CONFIG.md) - Environment variables reference
- [Deployment](./DEPLOYMENT.md) - Production deployment guide
- [Security](./SECURITY.md) - Security features and hardening
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions
