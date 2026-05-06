# Data Model: Search & Filtering

**Feature**: 008-search-filter  
**Created**: 2026-05-06

---

## Entities

### 1. News (Extended)

The existing `news` table is extended with a `search_vector` column.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| id | UUID | PRIMARY KEY, default gen_random_uuid() | Existing |
| title | VARCHAR(500) | NOT NULL | Existing |
| content | TEXT | - | Existing |
| status | VARCHAR(20) | DEFAULT 'draft' | Existing |
| author_id | UUID | NOT NULL, REFERENCES users(id) | Existing |
| **search_vector** | **tsvector** | **STORED, GENERATED** | **NEW** |
| created_at | TIMESTAMPTZ | DEFAULT CURRENT_TIMESTAMP | Existing |
| updated_at | TIMESTAMPTZ | DEFAULT CURRENT_TIMESTAMP | Existing |
| deleted_at | TIMESTAMPTZ | NULL, indexed | Existing (soft delete) |

**Relationship**: Many news → One author (user)

**State Transitions**: N/A (status field has existing workflow)

---

### 2. SavedSearch (NEW)

Stores user's saved search queries.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| id | UUID | PRIMARY KEY, default gen_random_uuid() | |
| user_id | UUID | NOT NULL, REFERENCES users(id) ON DELETE CASCADE | |
| name | VARCHAR(255) | NOT NULL | User-defined label |
| query_text | TEXT | NOT NULL | Search query string |
| filters | JSONB | DEFAULT '{}' | Serialized filter criteria |
| created_at | TIMESTAMPTZ | DEFAULT CURRENT_TIMESTAMP | |
| updated_at | TIMESTAMPTZ | DEFAULT CURRENT_TIMESTAMP | |

**Relationship**: Many saved_searches → One user

**Validation Rules**:
- `name`: 1-255 characters, required
- `query_text`: 1-500 characters, required
- `filters`: Valid JSON object with optional keys: status, author_id, date_from, date_to, tags
- Max 50 saved searches per user (enforced at service level)

---

### 3. SearchQuery (Value Object - not persisted)

Represents the search request parameters.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| text | string | 1-500 chars | Search query |
| entity_type | string | 'news', 'invoice', 'user' | Currently only 'news' supported |
| filters | SearchFilters | - | Faceted filter criteria |
| sort | string | 'relevance', 'created_at', 'updated_at', 'title' | Default: 'relevance' |
| sort_order | string | 'asc', 'desc' | Default: 'desc' |
| page | int | ≥ 1 | Default: 1 |
| per_page | int | 1-100 | Default: 20 |

---

### 4. SearchFilters (Value Object - embedded in SearchQuery)

Faceted filter criteria.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| status | string[] | Valid statuses: 'draft', 'published', 'archived' | Optional |
| author_id | UUID | Valid user UUID | Optional |
| date_from | TIMESTAMPTZ | ≤ date_to if both set | Optional |
| date_to | TIMESTAMPTZ | ≥ date_from if both set | Optional |
| tags | string[] | Valid tag names | Optional |

---

### 5. SearchResult (Value Object - not persisted)

Represents a single search result.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| id | UUID | - | Entity ID |
| entity_type | string | - | 'news', etc. |
| title | string | - | Entity title |
| excerpt | string | - | Highlighted matching content |
| rank | float32 | - | Relevance score 0-1 |
| created_at | TIMESTAMPTZ | - | Entity creation time |
| metadata | map[string]any | - | Additional entity-specific data |

---

## Relationships

```
User (1) ──────< (many) SavedSearch
  │                    │
  │                    └── query_text, filters, name
  │
  └──< (many) News ────< (implicit via search) SavedSearch
       │
       └── search_vector (generated tsvector)
```

---

## Database Indexes

### news table (existing + new)

| Index | Columns | Type | Purpose |
|-------|---------|------|---------|
| idx_news_pkey | id | btree | Primary key |
| idx_news_author_id | author_id | btree | Author lookups |
| idx_news_status | status | btree | Status filtering |
| idx_news_deleted_at | deleted_at | btree | Soft delete scope |
| idx_news_search | search_vector | **GIN** | **Full-text search** |

### saved_searches table (new)

| Index | Columns | Type | Purpose |
|-------|---------|------|---------|
| idx_saved_searches_pkey | id | btree | Primary key |
| idx_saved_searches_user_id | user_id | btree | User's saved searches |

---

## Migration Files

### Migration 016: Add search_vector to news

```sql
-- migrations/000016_add_news_search.up.sql
ALTER TABLE news ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(content, '')), 'B')
) STORED;

CREATE INDEX idx_news_search ON news USING GIN (search_vector);
```

### Migration 017: Create saved_searches table

```sql
-- migrations/000017_saved_searches.up.sql
CREATE TABLE saved_searches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    query_text TEXT NOT NULL,
    filters JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX idx_saved_searches_user_id ON saved_searches(user_id);
CREATE INDEX idx_saved_searches_deleted_at ON saved_searches(deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TRIGGER update_saved_searches_updated_at
BEFORE UPDATE ON saved_searches
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**Note on soft delete**: SavedSearch uses soft delete per Constitution III principle. While saved searches are user-generated data, they are user preferences (not audit-relevant like users/roles/permissions). Hard delete is acceptable for non-audit-relevant records. Soft delete is applied here for consistency and to enable potential "undo" functionality.

---

## Go Domain Models

### internal/domain/news.go (extended)

```go
type News struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Title     string         `gorm:"size:500;not null"`
    Content   string         `gorm:"type:text"`
    Status    string         `gorm:"size:20;default:'draft'"`
    AuthorID  uuid.UUID      `gorm:"type:uuid;not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`  // Soft delete - excluded from queries via GORM scope
    // Note: search_vector is a PostgreSQL GENERATED ALWAYS AS (...) STORED column
    // It is NOT mapped in Go - full-text search uses raw SQL via repository
}

func (News) TableName() string { return "news" }
```

### internal/domain/saved_search.go (NEW)

```go
type SavedSearch struct {
    ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID    uuid.UUID       `gorm:"type:uuid;not null;index"`
    Name      string          `gorm:"size:255;not null"`
    QueryText string          `gorm:"type:text;not null"`
    Filters   datatype.JSONB  `gorm:"type:jsonb;default:'{}'"`
    CreatedAt time.Time
    UpdatedAt time.Time
    
    // Relationships
    User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (SavedSearch) TableName() string { return "saved_searches" }

// SearchFiltersJSON is a helper for JSON marshal/unmarshal
type SearchFiltersJSON struct {
    Status    []string    `json:"status,omitempty"`
    AuthorID  *string     `json:"author_id,omitempty"`
    DateFrom  *time.Time  `json:"date_from,omitempty"`
    DateTo    *time.Time  `json:"date_to,omitempty"`
    Tags      []string    `json:"tags,omitempty"`
}
```
