# Search & Filtering API Contracts

**Feature**: 008-search-filter  
**Created**: 2026-05-06

---

## Endpoints Overview

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/search` | Full-text search with filters |
| GET | `/api/v1/search/suggestions` | Autocomplete suggestions |
| GET | `/api/v1/saved-searches` | List user's saved searches |
| POST | `/api/v1/saved-searches` | Save a search |
| GET | `/api/v1/saved-searches/:id` | Get a saved search |
| DELETE | `/api/v1/saved-searches/:id` | Delete a saved search |

**Base URL**: `/api/v1`  
**Authentication**: JWT Bearer token (all endpoints)

---

## Common Types

### SearchQuery

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| q | string | Yes | Search query text (1-500 chars) |
| entity | string | No | Entity type to search (default: "news") |
| status | string[] | No | Filter by status: draft, published, archived |
| author_id | string (UUID) | No | Filter by author |
| date_from | string (ISO 8601) | No | Filter from date |
| date_to | string (ISO 8601) | No | Filter to date |
| tags | string[] | No | Filter by tags |
| sort | string | No | Sort field: relevance, created_at, updated_at, title (default: relevance) |
| order | string | No | Sort order: asc, desc (default: desc) |
| page | integer | No | Page number ≥ 1 (default: 1) |
| per_page | integer | No | Results per page 1-100 (default: 20) |

### SearchResponse

```json
{
  "data": {
    "results": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "type": "news",
        "title": "Introduction to <mark>PostgreSQL</mark> Full-Text Search",
        "excerpt": "...accelerate your <mark>database</mark> queries with proper indexing...",
        "rank": 0.95,
        "created_at": "2026-05-01T10:30:00Z",
        "metadata": {
          "status": "published",
          "author": "John Doe"
        }
      }
    ],
    "pagination": {
      "page": 1,
      "per_page": 20,
      "total": 150,
      "total_pages": 8
    },
    "facets": {
      "status": [
        {"value": "published", "count": 120},
        {"value": "draft", "count": 30}
      ],
      "author": [
        {"value": "John Doe", "count": 80},
        {"value": "Jane Smith", "count": 70}
      ]
    }
  },
  "meta": {
    "request_id": "req-1234",
    "took_ms": 45
  }
}
```

### SavedSearchResponse

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "name": "PostgreSQL articles",
    "query_text": "postgresql database",
    "filters": {
      "status": ["published"],
      "date_from": "2026-01-01T00:00:00Z"
    },
    "created_at": "2026-05-06T10:00:00Z"
  }
}
```

---

## GET /api/v1/search

Full-text search with faceted filtering.

### Request

```
GET /api/v1/search?q=postgresql&status=published&page=1&per_page=20
Authorization: Bearer <jwt_token>
```

### Response 200 OK

```json
{
  "data": {
    "results": [...],
    "pagination": {...},
    "facets": {...}
  },
  "meta": {
    "request_id": "req-1234",
    "took_ms": 45
  }
}
```

### Response 400 Bad Request

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Search query is required",
    "details": "q parameter cannot be empty"
  },
  "meta": {
    "request_id": "req-1234"
  }
}
```

### Response 401 Unauthorized

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Authentication required"
  }
}
```

---

## GET /api/v1/search/suggestions

Autocomplete suggestions for search queries.

### Request

```
GET /api/v1/search/suggestions?q=post&limit=5
Authorization: Bearer <jwt_token>
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| q | string | Yes | Partial query (min 2 chars) |
| limit | integer | No | Max suggestions (default: 5, max: 10) |

### Response 200 OK

```json
{
  "data": {
    "suggestions": [
      "postgresql",
      "postgres",
      "postman"
    ]
  },
  "meta": {
    "request_id": "req-1234"
  }
}
```

---

## GET /api/v1/saved-searches

List the authenticated user's saved searches.

### Request

```
GET /api/v1/saved-searches
Authorization: Bearer <jwt_token>
```

### Response 200 OK

```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440001",
      "name": "PostgreSQL articles",
      "query_text": "postgresql database",
      "filters": {"status": ["published"]},
      "created_at": "2026-05-06T10:00:00Z"
    },
    {
      "id": "550e8400-e29b-41d4-a716-446655440002",
      "name": "Draft tutorials",
      "query_text": "tutorial",
      "filters": {"status": ["draft"]},
      "created_at": "2026-05-05T15:30:00Z"
    }
  ],
  "meta": {
    "request_id": "req-1234",
    "total": 2
  }
}
```

---

## POST /api/v1/saved-searches

Save a search query for later use.

### Request

```json
{
  "name": "PostgreSQL articles",
  "query_text": "postgresql database",
  "filters": {
    "status": ["published"],
    "date_from": "2026-01-01T00:00:00Z"
  }
}
```

| Field | Type | Required | Validation |
|-------|------|----------|-------------|
| name | string | Yes | 1-255 characters |
| query_text | string | Yes | 1-500 characters |
| filters | object | No | Valid filter structure |

### Response 201 Created

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "name": "PostgreSQL articles",
    "query_text": "postgresql database",
    "filters": {"status": ["published"]},
    "created_at": "2026-05-06T10:00:00Z"
  }
}
```

### Response 400 Bad Request

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Name is required",
    "details": "name must be between 1 and 255 characters"
  }
}
```

### Response 409 Conflict (max saved searches reached)

```json
{
  "error": {
    "code": "LIMIT_EXCEEDED",
    "message": "Maximum saved searches (50) reached",
    "details": "Delete existing saved searches to add more"
  }
}
```

---

## GET /api/v1/saved-searches/:id

Get a specific saved search by ID.

### Request

```
GET /api/v1/saved-searches/550e8400-e29b-41d4-a716-446655440001
Authorization: Bearer <jwt_token>
```

### Response 200 OK

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "name": "PostgreSQL articles",
    "query_text": "postgresql database",
    "filters": {"status": ["published"]},
    "created_at": "2026-05-06T10:00:00Z"
  }
}
```

### Response 404 Not Found

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Saved search not found"
  }
}
```

---

## DELETE /api/v1/saved-searches/:id

Delete a saved search.

### Request

```
DELETE /api/v1/saved-searches/550e8400-e29b-41d4-a716-446655440001
Authorization: Bearer <jwt_token>
```

### Response 204 No Content

No body returned on successful deletion.

### Response 404 Not Found

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Saved search not found"
  }
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| VALIDATION_ERROR | 400 | Invalid request parameters |
| UNAUTHORIZED | 401 | Missing or invalid authentication |
| FORBIDDEN | 403 | User lacks permission for this resource |
| NOT_FOUND | 404 | Resource not found |
| LIMIT_EXCEEDED | 409 | User has reached maximum limit |
| INTERNAL_ERROR | 500 | Unexpected server error |

---

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| GET /api/v1/search | 100 requests/minute per user |
| GET /api/v1/search/suggestions | 30 requests/minute per user |
| POST /api/v1/saved-searches | 20 requests/minute per user |

---

## Permissions

| Action | Permission |
|--------|------------|
| Search | `search:read` |
| View saved searches | `saved_search:read` |
| Create saved search | `saved_search:create` |
| Delete saved search | `saved_search:delete` |

Note: Users can only access their own saved searches (ownership enforced at service layer).
