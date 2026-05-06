# Research: Search & Filtering Feature

**Feature**: 008-search-filter  
**Created**: 2026-05-06  
**Status**: Complete

---

## PostgreSQL Full-Text Search Architecture

### 1. tsvector Column Design

**Decision**: Use PostgreSQL 12+ generated columns for automatic tsvector maintenance.

```sql
-- Add search_vector as a generated column
ALTER TABLE news ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(content, '')), 'B')
) STORED;

-- GIN index for fast full-text search
CREATE INDEX idx_news_search ON news USING GIN (search_vector);
```

**Rationale**: Generated columns provide zero sync lag, ACID consistency, and require no application code. Triggers are fallback for PostgreSQL < 12.

**Alternatives considered**:
- Application-level tsvector computation: Rejected due to race conditions and sync bugs
- Trigger-based: Rejected in favor of generated columns (simpler, more reliable)

---

### 2. Search Query Parsing

**Decision**: Use `websearch_to_tsquery()` for user-facing search input.

```go
// Safe for user input - handles special characters automatically
query := "database postgresql"
tsQuery := "websearch_to_tsquery('english', $1)"
```

**Rationale**: `websearch_to_tsquery()` handles user input safely (no injection risk), supports Google-like syntax (+, -, quotes), and is more forgiving than `plainto_tsquery()`.

**Alternatives considered**:
- `plainto_tsquery()`: Rejected - too strict, doesn't support phrase matching
- `phraseto_tsquery()`: Rejected - requires exact phrase, poor for general search
- Raw tsquery: Rejected - injection risk without sanitization

---

### 3. Prefix Matching (Autocomplete)

**Decision**: Use `:*` suffix operator in tsquery for prefix matching.

```sql
-- Match "post", "postgres", "postgresql", etc.
SELECT * FROM news WHERE search_vector @@ to_tsquery('english', 'postgr:*');
```

**Rationale**: PostgreSQL prefix matching via `:*` is efficient with GIN indexes and handles partial word autocomplete.

**For autocomplete fallback (trigram)**:
```sql
-- Enable pg_trgm extension for fuzzy prefix matching
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_news_title_trgm ON news USING GIN (title gin_trgm_ops);

-- Autocomplete query
SELECT title FROM news WHERE title ILIKE 'post%' LIMIT 10;
```

---

### 4. Relevance Ranking

**Decision**: Use `ts_rank_cd()` (cover density) with normalization for ranking.

```sql
SELECT id, title, 
    ts_rank_cd(search_vector, query, 32) AS rank
FROM news, websearch_to_tsquery('english', 'database') query
WHERE search_vector @@ query
ORDER BY rank DESC;
```

**Normalization factor 32**: Produces values in 0-1 range (rank/(rank+1)).

**Rationale**: `ts_rank_cd` considers term density and proximity, producing more intuitive rankings than basic `ts_rank`.

---

### 5. Search Vector Updates

**Decision**: Database-level maintenance (generated columns), NOT application-level.

**Rationale**:
- ACID guaranteed consistency
- Zero sync lag between data and index
- No application code complexity
- Bulk updates work automatically

**For PostgreSQL < 12 (triggers)**:
```sql
CREATE TRIGGER update_news_search_vector
BEFORE INSERT OR UPDATE ON news
FOR EACH ROW EXECUTE FUNCTION news_search_trigger();
```

---

## Go Project Patterns (from codebase exploration)

### Service Layer Pattern

```go
type SearchService struct {
    db          *gorm.DB
    newsRepo    repository.NewsRepository
    enforcer    *permission.Enforcer
    auditService *service.AuditService
}

func NewSearchService(db *gorm.DB, newsRepo repository.NewsRepository, ...) *SearchService {
    return &SearchService{db: db, newsRepo: newsRepo, ...}
}

// Context-first methods
func (s *SearchService) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
    // Implementation
}
```

### Domain Entity Pattern (from User, News entities)

```go
type News struct {
    ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Title           string         `gorm:"size:500;not null"`
    Content         string         `gorm:"type:text"`
    Status          string         `gorm:"size:20;default:'draft'"`
    AuthorID        uuid.UUID      `gorm:"type:uuid;not null"`
    SearchVector    pgtype.Text    `gorm:"type:tsvector" json:"-"`  // Not exposed in JSON
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`  // Soft delete
}

func (News) TableName() string { return "news" }
```

### Repository Pattern

```go
type NewsRepository interface {
    Create(ctx context.Context, news *News) error
    FindByID(ctx context.Context, id uuid.UUID) (*News, error)
    FindAll(ctx context.Context, limit, offset int) ([]News, int64, error)
    Search(ctx context.Context, query string, filters SearchFilters) (*SearchResult, error)
}

// Implementation uses r.db.WithContext(ctx) for all operations
```

### Handler Response Pattern

```go
// Success
return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))

// Error  
return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "No results"))
```

---

## Key Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Search vector storage | Generated columns | Zero sync lag, ACID, no app code |
| Index type | GIN | 2-10ms queries on 5M rows |
| Query parser | websearch_to_tsquery | User-safe, Google-like syntax |
| Ranking | ts_rank_cd with norm 32 | 0-1 range, cover density |
| Prefix matching | `:*` suffix | Efficient with GIN |
| Soft deletes | GORM DeletedAt scope | Automatic exclusion |
| Autocomplete | pg_trgm fallback | Trigram similarity for typos |

---

## Migration Strategy

### Phase 1: Add search_vector column to news

```sql
-- migrations/000016_add_news_search.up.sql
ALTER TABLE news ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(content, '')), 'B')
) STORED;

CREATE INDEX idx_news_search ON news USING GIN (search_vector);

-- Trigger for updated_at (already exists)
-- No trigger needed for search_vector (generated)
```

### Phase 2: Saved searches table

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
    
    INDEX idx_saved_searches_user_id (user_id)
);
```

---

## Performance Considerations

1. **Connection pooling**: Use database/sql pool settings (already configured)
2. **Statement timeout**: Set `statement_timeout = '5s'` for search queries
3. **Partial indexes**: Index only published content for filtered searches
4. **Covering indexes**: Include title, created_at in index for common results
5. **ts_headline placement**: Apply AFTER LIMIT, never before

---

## Open Questions Resolved

1. **Transaction consistency**: Generated columns are transaction-safe ✅
2. **Index maintenance**: GIN indexes don't need frequent reindex (unlike B-tree) ✅
3. **Update performance**: Generated columns add ~5% insert/update overhead ✅
4. **Soft delete handling**: Exclude soft-deleted via GORM scope `db.Where("deleted_at IS NULL")` ✅
