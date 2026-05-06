# Search & Filtering - Quickstart

**Feature**: 008-search-filter  
**Created**: 2026-05-06

---

## Overview

This feature adds full-text search capabilities to the Go API using PostgreSQL's built-in `tsvector`/`tsquery` full-text search. It provides:

- **Full-text search** across news articles with relevance ranking
- **Faceted filtering** by status, author, date range, and tags
- **Saved searches** - users can save and reuse search queries
- **Autocomplete suggestions** for search queries

---

## Prerequisites

- PostgreSQL 12+ (for generated columns)
- Go 1.22+
- Existing news table (will be extended with search_vector column)

---

## Installation Steps

### 1. Run Migrations

```bash
# Run the search migrations
make migrate

# Or manually:
psql $DATABASE_URL < migrations/000016_add_news_search.up.sql
psql $DATABASE_URL < migrations/000017_saved_searches.up.sql
```

### 2. Create Permissions

```bash
# Sync permissions to database
go run ./cmd/api permission:sync
```

This adds the following permissions:
- `search:read` - Full-text search
- `saved_search:read` - View saved searches
- `saved_search:create` - Create saved searches
- `saved_search:delete` - Delete saved searches

---

## Configuration

No new environment variables required. Search uses existing database connection.

---

## Usage Examples

### 1. Basic Search

```bash
curl -X GET "http://localhost:8080/api/v1/search?q=database" \
  -H "Authorization: Bearer <your_jwt_token>"
```

Response:
```json
{
  "data": {
    "results": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "type": "news",
        "title": "Introduction to PostgreSQL",
        "excerpt": "...accelerate your <mark>database</mark> queries...",
        "rank": 0.95,
        "created_at": "2026-05-01T10:30:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "per_page": 20,
      "total": 1
    }
  }
}
```

### 2. Search with Filters

```bash
curl -X GET "http://localhost:8080/api/v1/search?q=postgresql&status=published&author_id=<uuid>" \
  -H "Authorization: Bearer <your_jwt_token>"
```

### 3. Save a Search

```bash
curl -X POST "http://localhost:8080/api/v1/saved-searches" \
  -H "Authorization: Bearer <your_jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "PostgreSQL articles",
    "query_text": "postgresql database",
    "filters": {
      "status": ["published"]
    }
  }'
```

### 4. List Saved Searches

```bash
curl -X GET "http://localhost:8080/api/v1/saved-searches" \
  -H "Authorization: Bearer <your_jwt_token>"
```

### 5. Delete Saved Search

```bash
curl -X DELETE "http://localhost:8080/api/v1/saved-searches/<id>" \
  -H "Authorization: Bearer <your_jwt_token>"
```

---

## Testing

```bash
# Unit tests
go test ./internal/service/... -v -run Search

# Integration tests (requires Docker)
go test -tags=integration ./tests/integration/... -v -run Search -timeout 5m
```

---

## Architecture

```
Handler (internal/http/handler/search.go)
    │
    ▼
Service (internal/service/search.go)
    │
    ├──► Repository (internal/repository/search.go) - Full-text queries
    │
    └──► SavedSearchRepository - Saved search CRUD
```

---

## Key Implementation Details

### Search Vector (Generated Column)

The `search_vector` column is automatically maintained by PostgreSQL:

```sql
ALTER TABLE news ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(content, '')), 'B')
) STORED;
```

- Title matches weighted 'A' (higher priority)
- Content matches weighted 'B' (lower priority)
- Updates happen automatically on INSERT/UPDATE

### Relevance Ranking

Results are ranked using `ts_rank_cd()` which considers:
- Term frequency
- Document length
- Cover density

### Soft Deletes

Search results automatically exclude soft-deleted content via GORM scope.

---

## Troubleshooting

### No search results

1. Check the migration ran successfully:
   ```sql
   SELECT search_vector FROM news LIMIT 1;
   ```

2. Verify the GIN index exists:
   ```sql
   SELECT indexname FROM pg_indexes WHERE tablename = 'news';
   ```

### Slow search queries

1. Check for missing indexes
2. Verify `statement_timeout` isn't too aggressive
3. Consider partial indexes for filtered searches

### Permission denied errors

Run `go run ./cmd/api permission:sync` to ensure permissions are in the database.
